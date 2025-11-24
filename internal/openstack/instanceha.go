package openstack

import (
	"context"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// InstanceHaConfigMap is the name of the ConfigMap used for instance HA configuration
	InstanceHaConfigMap = "infra-instanceha-config"
	// InstanceHaImageKey is the key used for the instance HA image in the ConfigMap
	InstanceHaImageKey = "instanceha-image"
)

// ReconcileInstanceHa reconciles the instance HA configuration for the OpenStack control plane
func ReconcileInstanceHa(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	customData := map[string]string{
		InstanceHaImageKey: *getImg(version.Status.ContainerImages.OpenstackClientImage, &missingImageDefault),
	}

	cms := []util.Template{
		{
			Name:          InstanceHaConfigMap,
			Namespace:     instance.Namespace,
			InstanceType:  instance.Kind,
			Labels:        nil,
			ConfigOptions: nil,
			CustomData:    customData,
		},
	}

	if err := configmap.EnsureConfigMaps(ctx, helper, instance, cms, nil); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneInstanceHaCMReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneInstanceHaCMReadyErrorMessage,
			err.Error()))

		return ctrl.Result{}, err
	}

	instance.Status.Conditions.Set(condition.TrueCondition(
		corev1beta1.OpenStackControlPlaneInstanceHaCMReadyCondition,
		corev1beta1.OpenStackControlPlaneInstanceHaCMReadyMessage,
	))

	return ctrl.Result{}, nil
}
