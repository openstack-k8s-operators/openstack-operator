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

package functional_test

import (
	"context"
	"encoding/base64"

	"github.com/google/uuid"
	. "github.com/onsi/gomega" //revive:disable:dot-imports

	k8s_corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	infrav1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	openstackclientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	watcherv1 "github.com/openstack-k8s-operators/watcher-operator/api/v1beta1"
)

type Names struct {
	Namespace                          string
	OpenStackControlplaneName          types.NamespacedName
	OpenStackVersionName               types.NamespacedName
	OpenStackVersionName2              types.NamespacedName
	KeystoneAPIName                    types.NamespacedName
	MemcachedName                      types.NamespacedName
	MemcachedCertName                  types.NamespacedName
	CinderName                         types.NamespacedName
	ManilaName                         types.NamespacedName
	GlanceName                         types.NamespacedName
	NeutronName                        types.NamespacedName
	HorizonName                        types.NamespacedName
	HeatName                           types.NamespacedName
	NovaName                           types.NamespacedName
	TelemetryName                      types.NamespacedName
	WatcherName                        types.NamespacedName
	DBName                             types.NamespacedName
	DBCertName                         types.NamespacedName
	DBCell1Name                        types.NamespacedName
	DBCell1CertName                    types.NamespacedName
	RabbitMQName                       types.NamespacedName
	RabbitMQCertName                   types.NamespacedName
	RabbitMQCell1Name                  types.NamespacedName
	RabbitMQCell1CertName              types.NamespacedName
	NoVNCProxyCell1CertPublicRouteName types.NamespacedName
	NoVNCProxyCell1CertPublicSvcName   types.NamespacedName
	NoVNCProxyCell1CertVencryptName    types.NamespacedName
	ServiceAccountName                 types.NamespacedName
	RoleName                           types.NamespacedName
	RoleBindingName                    types.NamespacedName
	RootCAPublicName                   types.NamespacedName
	RootCAInternalName                 types.NamespacedName
	RootCAOvnName                      types.NamespacedName
	RootCALibvirtName                  types.NamespacedName
	SelfSignedIssuerName               types.NamespacedName
	CustomIssuerName                   types.NamespacedName
	CustomServiceCertSecretName        types.NamespacedName
	CABundleName                       types.NamespacedName
	OpenStackClientName                types.NamespacedName
	OVNNorthdName                      types.NamespacedName
	OVNNorthdCertName                  types.NamespacedName
	OVNControllerName                  types.NamespacedName
	OVNControllerCertName              types.NamespacedName
	OVNDbServerNBName                  types.NamespacedName
	OVNDbServerSBName                  types.NamespacedName
	OVNMetricsCertName                 types.NamespacedName
	NeutronOVNCertName                 types.NamespacedName
	OpenStackTopology                  []types.NamespacedName
	WatcherCertPublicRouteName         types.NamespacedName
	WatcherCertPublicSvcName           types.NamespacedName
	WatcherCertInternalName            types.NamespacedName
}

func CreateNames(openstackControlplaneName types.NamespacedName) Names {
	return Names{
		Namespace:                 openstackControlplaneName.Namespace,
		OpenStackControlplaneName: openstackControlplaneName,
		OpenStackVersionName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      openstackControlplaneName.Name, // same name as controlplane
		},
		OpenStackVersionName2: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "arbitrary-openstackversion", // different name than controlplane
		},
		RootCAPublicName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rootca-public"},
		RootCAInternalName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rootca-internal"},
		RootCAOvnName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rootca-ovn"},
		RootCALibvirtName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rootca-libvirt"},
		SelfSignedIssuerName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "selfsigned-issuer"},
		CustomIssuerName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "custom-issuer"},
		CABundleName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "combined-ca-bundle"},
		CustomServiceCertSecretName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "custom-service-cert"},
		ServiceAccountName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      openstackControlplaneName.Name},
		RoleName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      openstackControlplaneName.Name + "-role"},
		RoleBindingName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      openstackControlplaneName.Name + "-rolebinding"},
		KeystoneAPIName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "keystone",
		},
		MemcachedName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "memcached",
		},
		MemcachedCertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-memcached-svc",
		},
		CinderName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cinder",
		},
		GlanceName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "glance",
		},
		ManilaName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "manila",
		},
		NeutronName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "neutron",
		},
		HorizonName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "horizon",
		},
		HeatName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "heat",
		},
		TelemetryName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "telemetry",
		},
		NovaName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "nova",
		},
		WatcherName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "watcher",
		},
		DBName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "openstack",
		},
		DBCertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-galera-openstack-svc",
		},
		DBCell1Name: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "openstack-cell1",
		},
		DBCell1CertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-galera-openstack-cell1-svc",
		},
		RabbitMQName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rabbitmq",
		},
		RabbitMQCertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-rabbitmq-svc",
		},
		RabbitMQCell1Name: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rabbitmq-cell1",
		},
		RabbitMQCell1CertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-rabbitmq-cell1-svc",
		},
		NoVNCProxyCell1CertPublicRouteName: types.NamespacedName{
			Name:      "cert-nova-novncproxy-cell1-public-route",
			Namespace: openstackControlplaneName.Namespace,
		},
		NoVNCProxyCell1CertPublicSvcName: types.NamespacedName{
			Name:      "cert-nova-novncproxy-cell1-public-svc",
			Namespace: openstackControlplaneName.Namespace,
		},
		NoVNCProxyCell1CertVencryptName: types.NamespacedName{
			Name:      "cert-nova-novncproxy-cell1-vencrypt",
			Namespace: openstackControlplaneName.Namespace,
		},
		OpenStackClientName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "openstackclient",
		},
		OVNNorthdName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "ovnnorthd",
		},
		OVNNorthdCertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-ovnnorthd-ovndbs",
		},
		OVNDbServerNBName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "ovndbcluster-nb",
		},
		OVNDbServerSBName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "ovndbcluster-sb",
		},
		OVNControllerName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "ovncontroller",
		},
		OVNControllerCertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-ovncontroller-ovndbs",
		},
		OVNMetricsCertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-ovn-metrics",
		},
		NeutronOVNCertName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "cert-neutron-ovndbs",
		},
		OpenStackTopology: []types.NamespacedName{
			{
				Namespace: openstackControlplaneName.Namespace,
				Name:      "openstack-topology",
			},
			{
				Namespace: openstackControlplaneName.Namespace,
				Name:      "openstack-topology-alt",
			},
		},
		WatcherCertPublicRouteName: types.NamespacedName{
			Name:      "cert-watcher-public-route",
			Namespace: openstackControlplaneName.Namespace,
		},
		WatcherCertPublicSvcName: types.NamespacedName{
			Name:      "cert-watcher-public-svc",
			Namespace: openstackControlplaneName.Namespace,
		},
		WatcherCertInternalName: types.NamespacedName{
			Name:      "cert-watcher-internal-svc",
			Namespace: openstackControlplaneName.Namespace,
		},
	}
}

func GetDefaultOpenStackClientSpec() map[string]interface{} {
	return map[string]interface{}{}
}

func CreateOpenStackClient(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "client.openstack.org/v1beta1",
		"kind":       "OpenStackClient",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func GetOpenStackClient(name types.NamespacedName) *openstackclientv1.OpenStackClient {
	instance := &openstackclientv1.OpenStackClient{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func OpenStackClientConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetOpenStackClient(name)
	return instance.Status.Conditions
}

func CreateOpenStackVersion(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "core.openstack.org/v1beta1",
		"kind":       "OpenStackVersion",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func GetDefaultOpenStackVersionSpec() map[string]interface{} {
	return map[string]interface{}{}
}

func GetOpenStackVersion(name types.NamespacedName) *corev1.OpenStackVersion {
	instance := &corev1.OpenStackVersion{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func OpenStackVersionConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetOpenStackVersion(name)
	return instance.Status.Conditions
}

func OpenStackVersionRemoveFinalizer(ctx context.Context, name types.NamespacedName) {
	Eventually(func(g Gomega) {
		instance := GetOpenStackVersion(name)
		instance.SetFinalizers([]string{})
		g.Expect(th.K8sClient.Update(ctx, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
}

func CreateOpenStackControlPlane(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "core.openstack.org/v1beta1",
		"kind":       "OpenStackControlPlane",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

// Build OpenStackDataPlaneNodeSetSpec struct with empty `Nodes` list
func DefaultDataPlaneNoNodeSetSpec(tlsEnabled bool) map[string]interface{} {
	spec := map[string]interface{}{
		"preProvisioned": true,
		"nodeTemplate": map[string]interface{}{
			"networks": []infrav1.IPSetNetwork{
				{Name: "ctlplane", SubnetName: "subnet1"},
			},
			"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
		},
		"nodes":            map[string]interface{}{},
		"servicesOverride": []string{},
	}
	if tlsEnabled {
		spec["tlsEnabled"] = true
	}
	spec["nodes"] = map[string]dataplanev1.NodeSection{"edpm-compute-node-1": {}}
	return spec
}

func GetDataplaneNodeset(name types.NamespacedName) *dataplanev1.OpenStackDataPlaneNodeSet {
	instance := &dataplanev1.OpenStackDataPlaneNodeSet{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// Build OpenStackDataPlaneNodeSet struct and fill it with preset values
func DefaultDataplaneNodeSetTemplate(name types.NamespacedName, spec map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{

		"apiVersion": "dataplane.openstack.org/v1beta1",
		"kind":       "OpenStackDataPlaneNodeSet",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
}

// Create OpenstackDataPlaneNodeSet in k8s and test that no errors occur
// func CreateDataplaneNodeSet(name types.NamespacedName, spec map[string]interface{}) *unstructured.Unstructured {
func CreateDataplaneNodeSet(name types.NamespacedName, spec map[string]interface{}) client.Object {
	instance := DefaultDataplaneNodeSetTemplate(name, spec)
	return th.CreateUnstructured(instance)
}

func GetTLSPublicSpec() map[string]interface{} {
	return map[string]interface{}{
		"podLevel": map[string]interface{}{
			"enabled": false,
		},
	}
}

func GetTLSeCustomIssuerSpec() map[string]interface{} {
	return map[string]interface{}{
		"ingress": map[string]interface{}{
			"enabled": true,

			"ca": map[string]interface{}{
				"customIssuer": names.CustomIssuerName.Name,
				"duration":     "100h",
			},
			"cert": map[string]interface{}{
				"duration": "10h",
			},
		},
		"podLevel": map[string]interface{}{
			"enabled": true,
			"internal": map[string]interface{}{
				"ca": map[string]interface{}{
					"duration": "100h",
				},
				"cert": map[string]interface{}{
					"duration": "10h",
				},
			},
			"ovn": map[string]interface{}{
				"ca": map[string]interface{}{
					"duration": "100h",
				},
				"cert": map[string]interface{}{
					"duration": "10h",
				},
			},
		},
	}
}

func GetDefaultGaleraSpec() map[string]interface{} {
	return map[string]interface{}{
		"replicas":       1,
		"logToDisk":      false,
		"secret":         "osp-secret",
		"storageClass":   "local-storage",
		"storageRequest": "100M",
		"containerImage": "",
	}
}

func CreateGaleraConfig(namespace string, spec map[string]interface{}) client.Object {
	name := uuid.New().String()

	raw := map[string]interface{}{
		"apiVersion": "mariadb.openstack.org/v1beta1",
		"kind":       "Galera",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": spec,
	}

	return th.CreateUnstructured(raw)
}

func GetDefaultRabbitMQSpec() map[string]interface{} {
	return map[string]interface{}{
		"replicas":       1,
		"containerImage": "",
	}
}

func CreateRabbitMQConfig(namespace string, spec map[string]interface{}) client.Object {
	name := uuid.New().String()

	raw := map[string]interface{}{
		"apiVersion": "rabbitmq.openstack.org/v1beta1",
		"kind":       "RabbitMq",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": spec,
	}

	return th.CreateUnstructured(raw)
}

func GetDefaultOpenStackControlPlaneSpec() map[string]interface{} {
	memcachedTemplate := map[string]interface{}{
		"memcached": map[string]interface{}{
			"replicas": 1,
		},
	}
	rabbitTemplate := map[string]interface{}{
		names.RabbitMQName.Name: map[string]interface{}{
			"replicas": 1,
		},
		names.RabbitMQCell1Name.Name: map[string]interface{}{
			"replicas": 1,
		},
	}
	galeraTemplate := map[string]interface{}{
		names.DBName.Name: map[string]interface{}{
			"storageRequest": "500M",
		},
		names.DBCell1Name.Name: map[string]interface{}{
			"storageRequest": "500M",
		},
	}
	keystoneTemplate := map[string]interface{}{
		"databaseInstance": names.KeystoneAPIName.Name,
		"secret":           "osp-secret",
	}
	ironicTemplate := map[string]interface{}{
		"ironicConductors": []interface{}{},
	}
	heatTemplate := map[string]interface{}{
		"databaseInstance": "openstack",
		"secret":           "osp-secret",
		"passwordSelectors": map[string]interface{}{
			"authEncryptionKey": "HeatAuthEncryptionKey",
		},
	}
	telemetryTemplate := map[string]interface{}{
		"ceilometer": map[string]interface{}{
			"enabled": false,
		},
		"metricStorage": map[string]interface{}{
			"enabled": false,
		},
		"logging": map[string]interface{}{
			"enabled": false,
		},
		"autoscaling": map[string]interface{}{
			"enabled": false,
		},
	}
	manilaTemplate := map[string]interface{}{
		"databaseInstance":    "openstack",
		"rabbitMqClusterName": "rabbitmq",
		"memcachedInstance":   "memcached",
		"databaseAccount":     "account",
		"manilaAPI": map[string]interface{}{
			"replicas": 1,
		},
		"manilaScheduler": map[string]interface{}{
			"replicas": 1,
		},
		"manilaShares": map[string]interface{}{
			"share1": map[string]interface{}{
				"replicas": 1,
			},
		},
	}

	return map[string]interface{}{
		"secret":       "osp-secret",
		"storageClass": "local-storage",
		"galera": map[string]interface{}{
			"enabled":   true,
			"templates": galeraTemplate,
		},
		"rabbitmq": map[string]interface{}{
			"enabled":   true,
			"templates": rabbitTemplate,
		},
		"memcached": map[string]interface{}{
			"enabled":   true,
			"templates": memcachedTemplate,
		},
		"keystone": map[string]interface{}{
			"enabled":  true,
			"template": keystoneTemplate,
		},
		"placement": map[string]interface{}{
			"enabled": false,
		},
		"glance": map[string]interface{}{
			"enabled": true,
		},
		"horizon": map[string]interface{}{
			"enabled": true,
		},
		"cinder": map[string]interface{}{
			"enabled": true,
		},
		"ovn": map[string]interface{}{
			"enabled": false,
		},
		"neutron": map[string]interface{}{
			"enabled": true,
		},
		"swift": map[string]interface{}{
			"enabled": false,
		},
		"nova": map[string]interface{}{
			"enabled": false,
		},
		"redis": map[string]interface{}{
			"enabled": false,
		},
		"ironic": map[string]interface{}{
			"enabled":  false,
			"template": ironicTemplate,
		},
		"designate": map[string]interface{}{
			"enabled": false,
		},
		"barbican": map[string]interface{}{
			"enabled": false,
		},
		"openstackclient": map[string]interface{}{},
		"manila": map[string]interface{}{
			"enabled":  true,
			"template": manilaTemplate,
		},
		"heat": map[string]interface{}{
			"enabled":  true,
			"template": heatTemplate,
		},
		"telemetry": map[string]interface{}{
			"enabled":  true,
			"template": telemetryTemplate,
		},
		"watcher": map[string]interface{}{
			"enabled": false,
		},
	}
}

func GetOpenStackControlPlane(name types.NamespacedName) *corev1.OpenStackControlPlane {
	instance := &corev1.OpenStackControlPlane{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func OpenStackControlPlaneConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetOpenStackControlPlane(name)
	return instance.Status.Conditions
}

func CreateCertSecret(name types.NamespacedName) *k8s_corev1.Secret {
	certBase64 := "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJlekNDQVNLZ0F3SUJBZ0lRTkhER1lzQnM3OThpYkREN3EvbzJsakFLQmdncWhrak9QUVFEQWpBZU1Sd3cKR2dZRFZRUURFeE55YjI5MFkyRXRhM1YwZEd3dGNIVmliR2xqTUI0WERUSTBNREV4TlRFd01UVXpObG9YRFRNMApNREV4TWpFd01UVXpObG93SGpFY01Cb0dBMVVFQXhNVGNtOXZkR05oTFd0MWRIUnNMWEIxWW14cFl6QlpNQk1HCkJ5cUdTTTQ5QWdFR0NDcUdTTTQ5QXdFSEEwSUFCRDc4YXZYcWhyaEM1dzhzOVdrZDRJcGJlRXUwM0NSK1hYVWQKa0R6T1J5eGE5d2NjSWREaXZiR0pqSkZaVFRjVm1ianExQk1Zc2pyMTJVSUU1RVQzVmxxalFqQkFNQTRHQTFVZApEd0VCL3dRRUF3SUNwREFQQmdOVkhSTUJBZjhFQlRBREFRSC9NQjBHQTFVZERnUVdCQlRLSml6V1VKOWVVS2kxCmRzMGxyNmM2c0Q3RUJEQUtCZ2dxaGtqT1BRUURBZ05IQURCRUFpQklad1lxNjFCcU1KYUI2VWNGb1JzeGVjd0gKNXovek1PZHJPeWUwbU5pOEpnSWdRTEI0d0RLcnBmOXRYMmxvTSswdVRvcEFEU1lJbnJjZlZ1NEZCdVlVM0lnPQotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg=="
	keyBase64 := "LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCk1IY0NBUUVFSUptbGNLUEl1RitFc3RhYkxnVmowZkNhdzFTK09xNnJPU3M0U3pMQkJGYVFvQW9HQ0NxR1NNNDkKQXdFSG9VUURRZ0FFUHZ4cTllcUd1RUxuRHl6MWFSM2dpbHQ0UzdUY0pINWRkUjJRUE01SExGcjNCeHdoME9LOQpzWW1Na1ZsTk54V1p1T3JVRXhpeU92WFpRZ1RrUlBkV1dnPT0KLS0tLS1FTkQgRUMgUFJJVkFURSBLRVktLS0tLQo=="

	cert, _ := base64.StdEncoding.DecodeString(certBase64)
	key, _ := base64.StdEncoding.DecodeString(keyBase64)

	s := &k8s_corev1.Secret{}
	Eventually(func(_ Gomega) {
		s = th.CreateSecret(
			name,
			map[string][]byte{
				"ca.crt":  []byte(cert),
				"tls.crt": []byte(cert),
				"tls.key": []byte(key),
			})
	}, timeout, interval).Should(Succeed())

	return s
}

func CreateClusterConfigCM() client.Object {
	var cm client.Object

	Eventually(func(_ Gomega) {
		cm = th.CreateConfigMap(
			types.NamespacedName{
				Name:      "cluster-config-v1",
				Namespace: "kube-system",
			},
			map[string]interface{}{
				"install-config": "",
			})
	}, timeout, interval).Should(Succeed())

	return cm
}

func GetRabbitMQCluster(name types.NamespacedName) *rabbitmqv1.RabbitMq {
	instance := &rabbitmqv1.RabbitMq{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func SimulateRabbitmqReady() {

	instance := GetOpenStackControlPlane(names.OpenStackControlplaneName)
	if instance.Spec.Rabbitmq.Enabled {
		// FIXME add helpers to infra-operator to simulate rabbitmq
		Eventually(func(g Gomega) {

			for rabbitName := range *instance.Spec.Rabbitmq.Templates {
				rabbitmqNamespacedName := types.NamespacedName{
					Namespace: names.Namespace,
					Name:      rabbitName,
				}

				rabbit := &rabbitmqv1.RabbitMq{}
				g.Expect(th.K8sClient.Get(th.Ctx, rabbitmqNamespacedName, rabbit)).Should(Succeed())
				rabbit.Status.ObservedGeneration = rabbit.Generation
				rabbit.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)

				g.Expect(th.K8sClient.Status().Update(th.Ctx, rabbit)).To(Succeed())

				th.Logger.Info("Simulated RabbitMq ready", "on", rabbit.Name)

			}

		}, timeout, interval).Should(Succeed())
	}

}

func SimulateGalaraReady() {

	instance := GetOpenStackControlPlane(names.OpenStackControlplaneName)
	if instance.Spec.Galera.Enabled {
		// FIXME add helpers to mariadb-operator to simulate galera!
		Eventually(func(g Gomega) {

			for dbName := range *instance.Spec.Galera.Templates {
				dbNamespacedName := types.NamespacedName{
					Namespace: names.Namespace,
					Name:      dbName,
				}

				db := &mariadbv1.Galera{}
				g.Expect(th.K8sClient.Get(th.Ctx, dbNamespacedName, db)).Should(Succeed())
				db.Status.ObservedGeneration = db.Generation
				db.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition, condition.DeploymentReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, db)).To(Succeed())
				th.Logger.Info("Simulated DB ready", "on", dbName)
			}

		}, timeout, interval).Should(Succeed())
	}

}

func SimulateMemcachedReady() {

	instance := GetOpenStackControlPlane(names.OpenStackControlplaneName)

	if instance.Spec.Memcached.Enabled {
		if instance.Spec.TLS.PodLevel.Enabled {
			infra.SimulateTLSMemcachedReady(names.MemcachedName)
		} else {
			infra.SimulateMemcachedReady(names.MemcachedName)

		}
	}

}

func SimulateControlplaneReady() {
	instance := GetOpenStackControlPlane(names.OpenStackControlplaneName)

	SimulateRabbitmqReady()
	SimulateGalaraReady()
	SimulateMemcachedReady()

	if instance.Spec.Keystone.Enabled {
		keystone.SimulateKeystoneAPIReady(names.KeystoneAPIName)
	}

	if instance.Spec.Ovn.Enabled {
		ovn.SimulateOVNNorthdReady(names.OVNNorthdName)
		ovn.SimulateOVNDBClusterReady(names.OVNDbServerNBName)
		ovn.SimulateOVNDBClusterReady(names.OVNDbServerSBName)
		ovn.SimulateOVNControllerReady(names.OVNControllerName)
	}

	// simulate pod ready for openstackclient
	Eventually(func(_ Gomega) {
		th.SimulatePodReady(names.OpenStackClientName)
		th.Logger.Info("Simulated OpenStackClient ready")

	}, timeout, interval).Should(Succeed())
}

// GetSampleTopologySpec - A sample (and opinionated) Topology Spec
func GetSampleTopologySpec() map[string]interface{} {
	// Build the topology Spec
	topologySpec := map[string]interface{}{
		"topologySpreadConstraints": []map[string]interface{}{
			{
				"maxSkew":           1,
				"topologyKey":       k8s_corev1.LabelHostname,
				"whenUnsatisfiable": "ScheduleAnyway",
			},
		},
	}
	return topologySpec
}

// CreateTopology - Creates a Topology CR based on the spec passed as input
func CreateTopology(topology types.NamespacedName, spec map[string]interface{}) client.Object {
	raw := map[string]interface{}{
		"apiVersion": "topology.openstack.org/v1beta1",
		"kind":       "Topology",
		"metadata": map[string]interface{}{
			"name":      topology.Name,
			"namespace": topology.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

// GetGlance
func GetGlance(name types.NamespacedName) *glancev1.Glance {
	instance := &glancev1.Glance{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetCinder
func GetCinder(name types.NamespacedName) *cinderv1.Cinder {
	instance := &cinderv1.Cinder{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetNeutron
func GetNeutron(name types.NamespacedName) *neutronv1.NeutronAPI {
	instance := &neutronv1.NeutronAPI{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetManila
func GetManila(name types.NamespacedName) *manilav1.Manila {
	instance := &manilav1.Manila{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetHorizon
func GetHorizon(name types.NamespacedName) *horizonv1.Horizon {
	instance := &horizonv1.Horizon{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetHeat
func GetHeat(name types.NamespacedName) *heatv1.Heat {
	instance := &heatv1.Heat{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetTelemetry
func GetTelemetry(name types.NamespacedName) *telemetryv1.Telemetry {
	instance := &telemetryv1.Telemetry{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

// GetWatcher
func GetWatcher(name types.NamespacedName) *watcherv1.Watcher {
	instance := &watcherv1.Watcher{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}
