package openstack

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ceilometerv1 "github.com/openstack-k8s-operators/ceilometer-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
)

const ceilometerName = "ceilometer"

// ReconcileCeilometer makes sure CeilometerCentralAgent, CeilometerNotificationAgent and SGCore are deployed according to declaration
func ReconcileCeilometer(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	ceilo := &ceilometerv1.Ceilometer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ceilometerName,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ceilometer.Enabled {
		if res, err := EnsureDeleted(ctx, helper, ceilo); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneCeilometerReadyCondition)
		return ctrl.Result{}, nil
	}

	helper.GetLogger().Info("Reconciling Ceilometer", "Ceilometer.Namespace", instance.Namespace, "Ceilometer.Name", ceilometerName)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), ceilo, func() error {
		instance.Spec.Ceilometer.Template.DeepCopyInto(&ceilo.Spec)
		if ceilo.Spec.Secret == "" {
			ceilo.Spec.Secret = instance.Spec.Secret
		}

		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCeilometerReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneCeilometerReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Ceilometer %s - %s", ceilo.Name, op))
	}

	if ceilo.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneCeilometerReadyCondition, corev1beta1.OpenStackControlPlaneCeilometerReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCeilometerReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneCeilometerReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}
