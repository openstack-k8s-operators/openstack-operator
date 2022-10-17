package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileGlance -
func ReconcileGlance(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	glance := &glancev1.Glance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "glance",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling glance", "glance.Namespace", instance.Namespace, "glance.Name", "glance")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), glance, func() error {
		instance.Spec.GlanceTemplate.DeepCopyInto(&glance.Spec)
		if glance.Spec.Secret == "" {
			glance.Spec.Secret = instance.Spec.Secret
		}
		if glance.Spec.DatabaseInstance == "" {
			glance.Spec.DatabaseInstance = "openstack"
		}
		if glance.Spec.StorageClass == "" {
			glance.Spec.StorageClass = instance.Spec.StorageClass
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), glance, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneGlanceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneGlanceReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("glance %s - %s", glance.Name, op))
	}

	if glance.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneGlanceReadyCondition, corev1beta1.OpenStackControlPlaneGlanceReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneGlanceReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneGlanceReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}
