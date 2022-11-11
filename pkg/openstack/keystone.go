package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileKeystoneAPI -
func ReconcileKeystoneAPI(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.Keystone.Enabled {
		return ctrl.Result{}, nil
	}

	keystoneAPI := &keystonev1.KeystoneAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone", //FIXME (keystone doesn't seem to work unless named "keystone")
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling KeystoneAPI", "KeystoneAPI.Namespace", instance.Namespace, "keystoneAPI.Name", "keystone")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), keystoneAPI, func() error {
		instance.Spec.Keystone.Template.DeepCopyInto(&keystoneAPI.Spec)
		if keystoneAPI.Spec.Secret == "" {
			keystoneAPI.Spec.Secret = instance.Spec.Secret
		}
		if keystoneAPI.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			keystoneAPI.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if keystoneAPI.Spec.DatabaseInstance == "" {
			//keystoneAPI.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			keystoneAPI.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), keystoneAPI, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("KeystoneAPI %s - %s", keystoneAPI.Name, op))
	}

	if keystoneAPI.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition, corev1beta1.OpenStackControlPlaneKeystoneAPIReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}
