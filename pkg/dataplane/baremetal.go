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

// Package deployment provides functionality for OpenStack dataplane baremetal deployment operations
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
	"github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	utils "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
)

// ProvisionResult represents the result of a baremetal provisioning operation
type ProvisionResult struct {
	IsProvisioned bool
	BmhRefHash    string
}

// DeployBaremetalSet Deploy OpenStackBaremetalSet
func DeployBaremetalSet(
	ctx context.Context, helper *helper.Helper, instance *dataplanev1.OpenStackDataPlaneNodeSet,
	ipSets map[string]infranetworkv1.IPSet,
	dnsAddresses []string,
	containerImages openstackv1.ContainerImages,
) (ProvisionResult, error) {
	baremetalSet := &baremetalv1.OpenStackBaremetalSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	utils.LogForObject(helper, "Reconciling BaremetalSet", instance)
	_, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), baremetalSet, func() error {
		ownerLabels := labels.GetLabels(instance, labels.GetGroupLabel(NodeSetLabel), map[string]string{})
		baremetalSet.Labels = utils.MergeStringMaps(baremetalSet.GetLabels(), ownerLabels)
		baremetalSet.Spec.BaremetalHosts = make(map[string]baremetalv1.InstanceSpec)
		instance.Spec.BaremetalSetTemplate.DeepCopyInto(&baremetalSet.Spec.OpenStackBaremetalSetTemplateSpec)
		// Set Images
		if containerImages.OsContainerImage != nil && instance.Spec.BaremetalSetTemplate.OSContainerImageURL == "" {
			baremetalSet.Spec.OSContainerImageURL = *containerImages.OsContainerImage
		}
		if containerImages.AgentImage != nil && instance.Spec.BaremetalSetTemplate.AgentImageURL == "" {
			baremetalSet.Spec.AgentImageURL = *containerImages.AgentImage
		}
		if containerImages.ApacheImage != nil && instance.Spec.BaremetalSetTemplate.ApacheImageURL == "" {
			baremetalSet.Spec.ApacheImageURL = *containerImages.ApacheImage
		}

		for _, node := range instance.Spec.Nodes {
			hostName := node.HostName
			ipSet, ok := ipSets[hostName]
			if !ok {
				err := fmt.Errorf("no IPSet found for host: %s", hostName)
				return err
			}
			instanceSpec := baremetalv1.InstanceSpec{}
			instanceSpec.BmhLabelSelector = node.BmhLabelSelector
			instanceSpec.UserData = node.UserData
			instanceSpec.NetworkData = node.NetworkData
			instanceSpec.CtlplaneInterface = node.CtlplaneInterface
			for _, res := range ipSet.Status.Reservation {
				if strings.ToLower(string(res.Network)) == dataplanev1.CtlPlaneNetwork {
					_, ipNet, err := net.ParseCIDR(res.Cidr)
					if err != nil {
						return err
					}
					ipPrefix, _ := ipNet.Mask.Size()
					instanceSpec.CtlPlaneIP = fmt.Sprintf("%s/%d", res.Address, ipPrefix)
					if res.Gateway == nil {
						return fmt.Errorf("%s gateway is missing", dataplanev1.CtlPlaneNetwork)
					}
					instanceSpec.CtlplaneGateway = *res.Gateway
					instanceSpec.CtlplaneVlan = res.Vlan
					baremetalSet.Spec.BootstrapDNS = dnsAddresses
					baremetalSet.Spec.DNSSearchDomains = []string{res.DNSDomain}
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
		return ProvisionResult{}, err
	}

	// Check if baremetalSet is ready
	if !baremetalSet.IsReady() {
		utils.LogForObject(helper, "BaremetalSet not ready, waiting...", instance)
		instance.Status.Conditions.MarkFalse(
			dataplanev1.NodeSetBareMetalProvisionReadyCondition,
			condition.RequestedReason, condition.SeverityInfo,
			dataplanev1.NodeSetBaremetalProvisionReadyWaitingMessage)
		return ProvisionResult{}, nil
	}

	bmhRefHash, err := getBMHRefHash(baremetalSet)
	if err != nil {
		return ProvisionResult{}, err
	}

	instance.Status.Conditions.MarkTrue(
		dataplanev1.NodeSetBareMetalProvisionReadyCondition,
		dataplanev1.NodeSetBaremetalProvisionReadyMessage)
	return ProvisionResult{IsProvisioned: true, BmhRefHash: bmhRefHash}, nil
}

func getBMHRefHash(bmSet *baremetalv1.OpenStackBaremetalSet) (string, error) {
	bmhRefHash, err := utils.ObjectHash(bmSet.Status.BaremetalHosts)
	if err != nil {
		return "", err
	}
	return bmhRefHash, nil

}
