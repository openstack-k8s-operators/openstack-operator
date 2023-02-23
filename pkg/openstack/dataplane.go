package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	dataplanev1beta1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileDataplaneNode -
func ReconcileDataplaneNode(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	if !instance.Spec.DataPlaneNode.Enabled {
		return ctrl.Result{}, nil
	}

	dataPlaneNode := &dataplanev1beta1.OpenStackDataPlaneNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dataplanenode", //FIXME
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling OpenStackDataPlaneNode", "dataPlaneNode.Namespace", instance.Namespace, "dataPlaneNode.Name", "dataplanenode")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), dataPlaneNode, func() error {
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), dataPlaneNode, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackDataPlaneNodeReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackDataPlaneNodeReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("DataPlaneNode %s - %s", dataPlaneNode.Name, op))
	}

	if dataPlaneNode.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackDataPlaneNodeReadyCondition, corev1beta1.OpenStackDataPlaneNodeReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackDataPlaneNodeReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackDataPlaneNodeReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
