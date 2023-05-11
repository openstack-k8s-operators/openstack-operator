package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileHorizon -
func ReconcileHorizon(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	horizon := &horizonv1.Horizon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "horizon",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Horizon.Enabled {
		if res, err := EnsureDeleted(ctx, helper, horizon); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneHorizonReadyCondition)
		return ctrl.Result{}, nil
	}
	helper.GetLogger().Info("Reconcile Horizon", "horizon.Namespace", instance.Namespace, "horizon.Name", "horizon")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), horizon, func() error {
		instance.Spec.Horizon.Template.DeepCopyInto(&horizon.Spec)
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), horizon, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHorizonReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneHorizonReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Horizon %s - %s", horizon.Name, op))
	}

	if horizon.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneHorizonReadyCondition, corev1beta1.OpenStackControlPlaneHorizonReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHorizonReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneHorizonReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
