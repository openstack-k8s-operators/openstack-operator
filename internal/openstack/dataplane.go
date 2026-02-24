package openstack

import (
	"context"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"

	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetDataplaneNodesets - returns the dataplanenodesets in the namespace of the controlplane
func GetDataplaneNodesets(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (*dataplanev1.OpenStackDataPlaneNodeSetList, error) {
	// Get the dataplane nodesets
	dataplaneNodesets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	opts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	err := helper.GetClient().List(ctx, dataplaneNodesets, opts...)
	if err != nil {
		return nil, err
	}
	return dataplaneNodesets, nil
}

// DataplaneNodesetsDeployedVersionIsSet checks if deployed version is set for all dataplane nodesets
func DataplaneNodesetsDeployedVersionIsSet(dataplaneNodesets *dataplanev1.OpenStackDataPlaneNodeSetList) bool {
	for _, nodeset := range dataplaneNodesets.Items {
		// FIXME: DeployedVersion on the DataplaneNodeset should be a string pointer to match how Controlplane implements this
		if nodeset.Status.DeployedVersion == "" {
			return false
		}
	}
	return true
}

// DataplaneNodesetsOVNControllerImagesMatch returns true if OVNControllers are deployed on all nodesets
func DataplaneNodesetsOVNControllerImagesMatch(version *corev1beta1.OpenStackVersion, dataplaneNodesets *dataplanev1.OpenStackDataPlaneNodeSetList) bool {
	for _, nodeset := range dataplaneNodesets.Items {
		if nodeset.Generation != nodeset.Status.ObservedGeneration {
			return false
		}
		// After considering generation (to make sure reconciliation has quiesced for
		// the current nodeset spec), we only check nodesets if they have any nodes
		// and have deployed OVN
		if len(nodeset.Spec.Nodes) > 0 && nodeset.Status.ContainerImages["OvnControllerImage"] != "" {
			// Check if OVN controller image matches the target version.
			// Note: We don't check nodeset.IsReady() here because this is an intermediate
			// step in the minor update workflow. The nodeset might be not-Ready due to
			// subsequent deployments running (e.g. edpm-update), but if the OVN image matches,
			// it means the OVN update deployment already completed.
			if nodeset.Status.ContainerImages["OvnControllerImage"] != *version.Status.ContainerImages.OvnControllerImage {
				return false
			}
		}
	}
	return true
}

// DataplaneNodesetsDeployed returns true if all nodesets are deployed with the latest version
func DataplaneNodesetsDeployed(version *corev1beta1.OpenStackVersion, dataplaneNodesets *dataplanev1.OpenStackDataPlaneNodeSetList) bool {
	for _, nodeset := range dataplaneNodesets.Items {
		if nodeset.Generation != nodeset.Status.ObservedGeneration {
			return false
		}
		// After considering generation (to make sure reconciliation has quiesced for
		// the current nodeset spec), we only care about deployed status if the nodeset
		// has nodes
		if len(nodeset.Spec.Nodes) == 0 {
			continue
		}
		if !nodeset.IsReady() {
			return false
		}
		if nodeset.Status.DeployedVersion != version.Spec.TargetVersion {
			return false
		}

	}
	return true
}
