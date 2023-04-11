package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileHeat -
func ReconcileHeat(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	heat := &heatv1.Heat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "heat",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Heat.Enabled {
		if res, err := EnsureDeleted(ctx, helper, heat); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneHeatReadyCondition)
		return ctrl.Result{}, nil
	}
	helper.GetLogger().Info("Reconcile heat", "heat.Namespace", instance.Namespace, "heat.Name", "heat")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), heat, func() error {
		instance.Spec.Heat.Template.DeepCopyInto(&heat.Spec)
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), heat, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHeatReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneHeatReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("heat %s - %s", heat.Name, op))
	}

	if heat.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneHeatReadyCondition, corev1beta1.OpenStackControlPlaneHeatReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHeatReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneHeatReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
