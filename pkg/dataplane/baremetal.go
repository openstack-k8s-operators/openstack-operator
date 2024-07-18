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
	"net"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	utils "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
)

// DeployBaremetalSet Deploy OpenStackBaremetalSet
func DeployBaremetalSet(
	ctx context.Context, helper *helper.Helper, instance *dataplanev1.OpenStackDataPlaneNodeSet,
	ipSets map[string]infranetworkv1.IPSet,
	dnsAddresses []string,
) (bool, error) {
	baremetalSet := &baremetalv1.OpenStackBaremetalSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	if instance.Spec.BaremetalSetTemplate.BaremetalHosts == nil {
		return false, fmt.Errorf("no baremetal hosts set in baremetalSetTemplate")
	}
	utils.LogForObject(helper, "Reconciling BaremetalSet", instance)
	_, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), baremetalSet, func() error {
		instance.Spec.BaremetalSetTemplate.DeepCopyInto(&baremetalSet.Spec)
		for _, node := range instance.Spec.Nodes {
			hostName := node.HostName
			ipSet, ok := ipSets[hostName]
			instanceSpec := baremetalSet.Spec.BaremetalHosts[hostName]
			if !ok {
				// TODO: Change this to raise an error instead.
				// NOTE(hjensas): Hardcode /24 here, this used to rely on
				// baremetalSet.Spec.CtlplaneNetmask's default value ("255.255.255.0").
				utils.LogForObject(helper, "IPAM Not configured for use, skipping", instance)
				instanceSpec.CtlPlaneIP = fmt.Sprintf("%s/24", node.Ansible.AnsibleHost)
			} else {
				for _, res := range ipSet.Status.Reservation {
					if strings.ToLower(string(res.Network)) == CtlPlaneNetwork {
						_, ipNet, err := net.ParseCIDR(res.Cidr)
						if err != nil {
							return err
						}
						ipPrefix, _ := ipNet.Mask.Size()
						instanceSpec.CtlPlaneIP = fmt.Sprintf("%s/%d", res.Address, ipPrefix)
						if res.Gateway == nil {
							return fmt.Errorf("%s gateway is missing", CtlPlaneNetwork)
						}
						baremetalSet.Spec.CtlplaneGateway = *res.Gateway
						baremetalSet.Spec.BootstrapDNS = dnsAddresses
						baremetalSet.Spec.DNSSearchDomains = []string{res.DNSDomain}
					}
				}
			}
			baremetalSet.Spec.BaremetalHosts[hostName] = instanceSpec

		}
		err := controllerutil.SetControllerReference(
			helper.GetBeforeObject(), baremetalSet, helper.GetScheme())
		return err
	})

	if err != nil {
		instance.Status.Conditions.MarkFalse(
			dataplanev1.NodeSetBareMetalProvisionReadyCondition,
			condition.ErrorReason, condition.SeverityError,
			dataplanev1.NodeSetBaremetalProvisionErrorMessage,
			err.Error())
		return false, err
	}

	// Check if baremetalSet is ready
	if !baremetalSet.IsReady() {
		utils.LogForObject(helper, "BaremetalSet not ready, waiting...", instance)
		instance.Status.Conditions.MarkFalse(
			dataplanev1.NodeSetBareMetalProvisionReadyCondition,
			condition.RequestedReason, condition.SeverityInfo,
			dataplanev1.NodeSetBaremetalProvisionReadyWaitingMessage)
		return false, nil
	}
	instance.Status.Conditions.MarkTrue(
		dataplanev1.NodeSetBareMetalProvisionReadyCondition,
		dataplanev1.NodeSetBaremetalProvisionReadyMessage)
	return true, nil
}
