/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deployment

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/slices"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
)

// Generates an organized data structure that is leveraged to create the secrets.
func createSecretsDataStructure(secretMaxSize int,
	certsData map[string][]byte,
) []map[string][]byte {
	ci := []map[string][]byte{}

	keys := []string{}
	for k := range certsData {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	totalSize := secretMaxSize
	var cur *map[string][]byte
	// Going 3 by 3 to include CA, crt and key, in the same secret.
	for k := 0; k < len(keys)-1; k += 3 {
		szCa := len(certsData[keys[k]]) + len(keys[k])
		szCrt := len(certsData[keys[k+1]]) + len(keys[k+1])
		szKey := len(certsData[keys[k+2]]) + len(keys[k+2])
		sz := szCa + szCrt + szKey
		if (totalSize + sz) > secretMaxSize {
			i := len(ci)
			ci = append(ci, make(map[string][]byte))
			cur = &ci[i]
			totalSize = 0
		}
		totalSize += sz
		(*cur)[keys[k]] = certsData[keys[k]]
		(*cur)[keys[k+1]] = certsData[keys[k+1]]
		(*cur)[keys[k+2]] = certsData[keys[k+2]]
	}

	return ci
}

// EnsureTLSCerts generates secrets containing all the certificates for the relevant service
// These secrets will be mounted by the ansibleEE pod as an extra mount when the service is deployed.
func EnsureTLSCerts(ctx context.Context, helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	allHostnames map[string]map[infranetworkv1.NetNameStr]string,
	allIPs map[string]map[infranetworkv1.NetNameStr]string,
	service dataplanev1.OpenStackDataPlaneService,
	certKey string,
) (*ctrl.Result, error) {
	certsData := map[string][]byte{}
	secretMaxSize := instance.Spec.SecretMaxSize

	// for each node in the nodeset, issue all the TLS certs needed based on the
	// ips or DNS Names
	for nodeName, node := range instance.Spec.Nodes {
		var dnsNames map[infranetworkv1.NetNameStr]string
		var ipsMap map[infranetworkv1.NetNameStr]string
		var hosts []string
		var ips []string
		var issuer *certmgrv1.Issuer
		var issuerLabelSelector map[string]string
		var certName string
		var certSecret *corev1.Secret
		var err error
		var result ctrl.Result

		// TODO(alee) decide if we want to use other labels
		// For now we just add the hostname so we can select all the certs on one node
		hostName := node.HostName
		labels := map[string]string{
			HostnameLabel:   hostName,
			ServiceLabel:    service.Name,
			ServiceKeyLabel: certKey,
			NodeSetLabel:    instance.Name,
		}
		certName = service.Name + "-" + certKey + "-" + hostName

		dnsNames = allHostnames[hostName]
		ipsMap = allIPs[hostName]

		dnsNamesInCert := slices.Contains(service.Spec.TLSCerts[certKey].Contents, DNSNamesStr)
		ipValuesInCert := slices.Contains(service.Spec.TLSCerts[certKey].Contents, IPValuesStr)

		// Create the hosts and ips lists
		if dnsNamesInCert {
			if len(service.Spec.TLSCerts[certKey].Networks) == 0 {
				hosts = make([]string, 0, len(dnsNames))
				for _, host := range dnsNames {
					hosts = append(hosts, host)
				}
			} else {
				hosts = make([]string, 0, len(service.Spec.TLSCerts[certKey].Networks))
				for _, network := range service.Spec.TLSCerts[certKey].Networks {
					certNetwork := strings.ToLower(string(network))
					hosts = append(hosts, dnsNames[infranetworkv1.NetNameStr(certNetwork)])
				}
			}
		}
		if ipValuesInCert {
			if len(service.Spec.TLSCerts[certKey].Networks) == 0 {
				ips = make([]string, 0, len(ipsMap))
				for _, ip := range ipsMap {
					ips = append(ips, ip)
				}
			} else {
				ips = make([]string, 0, len(service.Spec.TLSCerts[certKey].Networks))
				for _, network := range service.Spec.TLSCerts[certKey].Networks {
					certNetwork := strings.ToLower(string(network))
					ips = append(ips, ipsMap[infranetworkv1.NetNameStr(certNetwork)])
				}
			}
		}

		if service.Spec.TLSCerts[certKey].Issuer == "" {
			// by default, use the internal root CA
			issuerLabelSelector = map[string]string{certmanager.RootCAIssuerInternalLabel: ""}
		} else {
			issuerLabelSelector = map[string]string{service.Spec.TLSCerts[certKey].Issuer: ""}
		}

		issuer, err = certmanager.GetIssuerByLabels(ctx, helper, instance.Namespace, issuerLabelSelector)
		if err != nil {
			helper.GetLogger().Info("Error retrieving issuer by label", "issuerLabelSelector", issuerLabelSelector)
			return &result, err
		}

		// NOTE: we are assuming that there will always be a ctlplane network
		// that means if you are not using network isolation with multiple networks
		// you should still need to have a ctlplane network at a minimum to use tls-e
		baseName, ok := dnsNames[CtlPlaneNetwork]
		if !ok {
			return &result, fmt.Errorf(
				"control plane network not found for node %s , tls-e requires a control plane network to be present",
				nodeName)
		}

		commonName := strings.Split(baseName, ".")[0]

		certSecret, result, err = GetTLSNodeCert(ctx, helper, instance, certName,
			issuer, labels, commonName, hosts, ips, service.Spec.TLSCerts[certKey].KeyUsages)

		// handle cert request errors
		if (err != nil) || (result != ctrl.Result{}) {
			return &result, err
		}

		// TODO(alee) Add an owner reference to the secret so it can be monitored
		// We'll do this once stuggi adds a function to do this in libcommon

		// To use this cert, add it to the relevant service data
		certsData[baseName+"-tls.key"] = certSecret.Data["tls.key"]
		certsData[baseName+"-tls.crt"] = certSecret.Data["tls.crt"]
		certsData[baseName+"-ca.crt"] = certSecret.Data["ca.crt"]
	}

	// Calculate number of secrets to create
	ci := createSecretsDataStructure(secretMaxSize, certsData)

	labels := map[string]string{
		"numberOfSecrets": strconv.Itoa(len(ci)),
	}
	// create secrets to hold the certs for the services
	for i := range ci {
		labels["secretNumber"] = strconv.Itoa(i)
		serviceCertsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      GetServiceCertsSecretName(instance, service.Name, certKey, i),
				Namespace: instance.Namespace,
				Labels:    labels,
			},
			Data: ci[i],
		}
		_, result, err := secret.CreateOrPatchSecret(ctx, helper, instance, serviceCertsSecret)
		if err != nil {
			err = fmt.Errorf("error creating certs secret for %s - %w", service.Name, err)
			return &ctrl.Result{}, err
		} else if result != controllerutil.OperationResultNone {
			return &ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
	}

	return &ctrl.Result{}, nil
}

// GetTLSNodeCert creates or retrieves the cert for a node for a given service
func GetTLSNodeCert(ctx context.Context, helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	certName string, issuer *certmgrv1.Issuer,
	labels map[string]string,
	commonName string,
	hostnames []string, ips []string, usages []certmgrv1.KeyUsage,
) (*corev1.Secret, ctrl.Result, error) {
	// use cert duration and renewBefore from annotations set on issuer
	// - if no duration annotation is set, use the default from certmanager lib-common module,
	// - if no renewBefore annotation is set, the cert-manager default is used.
	durationString := certmanager.CertDefaultDuration
	if d, ok := issuer.Annotations[certmanager.CertDurationAnnotation]; ok && d != "" {
		durationString = d
	}
	duration, err := time.ParseDuration(durationString)
	if err != nil {
		err = fmt.Errorf("error parsing duration annotation %s - %w", certmanager.CertDurationAnnotation, err)
		return nil, ctrl.Result{}, err
	}

	var renewBefore *time.Duration
	if r, ok := issuer.Annotations[certmanager.CertRenewBeforeAnnotation]; ok && r != "" {
		rb, err := time.ParseDuration(r)
		if err != nil {
			err = fmt.Errorf("error parsing renewBefore annotation %s - %w", certmanager.CertRenewBeforeAnnotation, err)
			return nil, ctrl.Result{}, err
		}

		renewBefore = &rb
	}

	request := certmanager.CertificateRequest{
		CommonName:  &commonName,
		IssuerName:  issuer.Name,
		CertName:    certName,
		Duration:    &duration,
		RenewBefore: renewBefore,
		Hostnames:   hostnames,
		Ips:         ips,
		Annotations: nil,
		Labels:      labels,
		Usages:      usages,
		Subject: &certmgrv1.X509Subject{
			// NOTE(owalsh): For libvirt/QEMU this should match issuer CN
			Organizations: []string{issuer.Name},
		},
	}

	certSecret, result, err := certmanager.EnsureCert(ctx, helper, request, instance)
	if err != nil {
		return nil, ctrl.Result{}, err
	} else if (result != ctrl.Result{}) {
		return nil, result, nil
	}

	return certSecret, ctrl.Result{}, nil
}

// GetServiceCertsSecretName - return name of secret to be mounted in ansibleEE which contains
// all the TLS certs that fit in a secret for the relevant service. The index variable is used
// to make the secret name unique.
// The convention we use here is "<nodeset.name>-<service>-<certkey>-certs-<index>", for example,
// openstack-epdm-nova-default-certs-0.
func GetServiceCertsSecretName(instance *dataplanev1.OpenStackDataPlaneNodeSet, serviceName string,
	certKey string, index int) string {
	return fmt.Sprintf("%s-%s-%s-certs-%s", instance.Name, serviceName, certKey, strconv.Itoa(index))
}
