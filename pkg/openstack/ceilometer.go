package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	ceilometerNamespaceLabel = "CeilometerCentral.Namespace"
	ceilometerNameLabel      = "CeilometerCentral.Name"
	ceilometerName           = "ceilometercentral"
)

// ReconcileCeilometer ...
func ReconcileCeilometer(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	ceilometer := &telemetryv1.CeilometerCentral{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ceilometerName,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ceilometer.Enabled {
		if res, err := EnsureDeleted(ctx, helper, ceilometer); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneCeilometerReadyCondition)
		return ctrl.Result{}, nil
	}

	helper.GetLogger().Info("Reconciling Ceilometer", ceilometerNamespaceLabel, instance.Namespace, ceilometerNameLabel, ceilometerName)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), ceilometer, func() error {
		instance.Spec.Ceilometer.Template.DeepCopyInto(&ceilometer.Spec)

		if ceilometer.Spec.Secret == "" {
			ceilometer.Spec.Secret = instance.Spec.Secret
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), ceilometer, helper.GetScheme())
		if err != nil {
			return err
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
		helper.GetLogger().Info(fmt.Sprintf("%s %s - %s", ceilometerName, ceilometer.Name, op))
	}

	if ceilometer.IsReady() {
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
