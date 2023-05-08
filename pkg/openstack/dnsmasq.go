package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileDNSMasqs -
func ReconcileDNSMasqs(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	dnsmasq := &networkv1.DNSMasq{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "dns",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.DNS.Enabled {
		if res, err := EnsureDeleted(ctx, helper, dnsmasq); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneDNSReadyCondition)
		return ctrl.Result{}, nil
	}

	helper.GetLogger().Info("Reconciling DNSMasq", "DNSMasq.Namespace", instance.Namespace, "DNSMasq.Name", "dnsmasq")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), dnsmasq, func() error {
		instance.Spec.DNS.Template.DeepCopyInto(&dnsmasq.Spec)
		if dnsmasq.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			dnsmasq.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), dnsmasq, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneDNSReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneDNSReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("dnsmasq %s - %s", dnsmasq.Name, op))
	}

	if dnsmasq.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneDNSReadyCondition, corev1beta1.OpenStackControlPlaneDNSReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneDNSReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneDNSReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}
