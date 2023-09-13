package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeIronicReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Ironic.Template.IronicAPI.Override.Service == nil {
			instance.Spec.Ironic.Template.IronicAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Ironic.Template.IronicAPI.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Ironic.Template.IronicAPI.Override.Service[endpointType],
				ironic.Name+"-api")

		if instance.Spec.Ironic.Template.IronicInspector.Override.Service == nil {
			instance.Spec.Ironic.Template.IronicInspector.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Ironic.Template.IronicInspector.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Ironic.Template.IronicInspector.Override.Service[endpointType],
				ironic.Name+"-inspector")
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "ironic", Namespace: instance.Namespace}, ironic); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Ironic API
	if ironic.Status.Conditions.IsTrue(ironicv1.IronicAPIReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: ironic.Name + "-api"},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		instance.Spec.Ironic.Template.IronicAPI.Override.Service, ctrlResult, err = EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			ironic,
			svcs,
			instance.Spec.Ironic.Template.IronicAPI.Override.Service,
			instance.Spec.Ironic.APIOverride.Route,
			corev1beta1.OpenStackControlPlaneExposeIronicReadyCondition,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	// Ironic Inspector
	if ironic.Status.Conditions.IsTrue(ironicv1.IronicInspectorReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: ironic.Name + "-inspector"},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		instance.Spec.Ironic.Template.IronicInspector.Override.Service, ctrlResult, err = EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			ironic,
			svcs,
			instance.Spec.Ironic.Template.IronicInspector.Override.Service,
			instance.Spec.Ironic.InspectorOverride.Route,
			corev1beta1.OpenStackControlPlaneExposeIronicReadyCondition,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	helper.GetLogger().Info("Reconciling Ironic", "Ironic.Namespace", instance.Namespace, "Ironic.Name", "ironic")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), ironic, func() error {
		instance.Spec.Ironic.Template.DeepCopyInto(&ironic.Spec)
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
