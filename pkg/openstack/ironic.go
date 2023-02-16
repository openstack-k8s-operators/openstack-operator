package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileIronic -
func ReconcileIronic(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	ironic := &ironicv1.Ironic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ironic",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ironic.Enabled {
		if res, err := EnsureDeleted(ctx, helper, ironic); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneIronicReadyCondition)
		return ctrl.Result{}, nil
	}

	helper.GetLogger().Info("Reconciling ironic", "ironic.Namespace", instance.Namespace, "ironic.Name", "ironic")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), ironic, func() error {
		instance.Spec.Ironic.Template.DeepCopyInto(&ironic.Spec)
		if ironic.Spec.Secret == "" {
			ironic.Spec.Secret = instance.Spec.Secret
		}
		if ironic.Spec.DatabaseInstance == "" {
			ironic.Spec.DatabaseInstance = "openstack"
		}
		for _, c := range ironic.Spec.IronicConductors {
			if c.StorageClass == "" {
				c.StorageClass = instance.Spec.StorageClass
			}
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), ironic, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneIronicReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneIronicReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("ironic %s - %s", ironic.Name, op))
	}

	if ironic.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneIronicReadyCondition, corev1beta1.OpenStackControlPlaneIronicReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneIronicReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneIronicReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}
