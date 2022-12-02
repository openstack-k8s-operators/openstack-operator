package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ovsv1 "github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileOVS -
func ReconcileOVS(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.Ovs.Enabled {
		return ctrl.Result{}, nil
	}
	name := "ovs"
	ovs := &ovsv1.OVS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling OVS", "OVS.Namespace", instance.Namespace, "OVS.Name", name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), ovs, func() error {
		instance.Spec.Ovs.Template.DeepCopyInto(&ovs.Spec)

		if ovs.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			ovs.Spec.NodeSelector = instance.Spec.NodeSelector
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), ovs, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVSReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneOVSReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("OVS %s - %s", ovs.Name, op))
	}

	if ovs.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneOVSReadyCondition, corev1beta1.OpenStackControlPlaneOVSReadyMessage)
	} else {
		instance.Status.Conditions.Set(
			condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneOVSReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneOVSReadyRunningMessage))
	}
	return ctrl.Result{}, nil
}
