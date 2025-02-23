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
func ReconcileDNSMasqs(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
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
		instance.Status.ContainerImages.InfraDnsmasqImage = nil
		return ctrl.Result{}, nil
	}

	Log := GetLogger(ctx)

	if instance.Spec.DNS.Template == nil {
		instance.Spec.DNS.Template = &networkv1.DNSMasqSpecCore{}
	}

	if instance.Spec.DNS.Template.NodeSelector == nil {
		instance.Spec.DNS.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	Log.Info("Reconciling DNSMasq", "DNSMasq.Namespace", instance.Namespace, "DNSMasq.Name", "dnsmasq")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), dnsmasq, func() error {
		instance.Spec.DNS.Template.DeepCopyInto(&dnsmasq.Spec.DNSMasqSpecCore)
		dnsmasq.Spec.ContainerImage = *version.Status.ContainerImages.InfraDnsmasqImage
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
		Log.Info(fmt.Sprintf("dnsmasq %s - %s", dnsmasq.Name, op))
	}

	if dnsmasq.Status.ObservedGeneration == dnsmasq.Generation && dnsmasq.IsReady() {
		instance.Status.ContainerImages.InfraDnsmasqImage = version.Status.ContainerImages.InfraDnsmasqImage
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

// DnsmasqImageMatch - return true if the Dnsmasq images match on the ControlPlane and Version, or if Dnsmasq is not enabled
func DnsmasqImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.DNS.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.InfraDnsmasqImage, version.Status.ContainerImages.InfraDnsmasqImage) {
			Log.Info("Dnsmasq images do not match", "controlPlane.Status.ContainerImages.InfraDnsmasqImage", controlPlane.Status.ContainerImages.InfraDnsmasqImage, "version.Status.ContainerImages.InfraDnsmasqImage", version.Status.ContainerImages.InfraDnsmasqImage)
			return false
		}
	}
	return true
}
