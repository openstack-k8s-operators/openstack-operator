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

	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileHorizon -
func ReconcileHorizon(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	horizon := &horizonv1.Horizon{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "horizon",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Horizon.Enabled {
		if res, err := EnsureDeleted(ctx, helper, horizon); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneHorizonReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeHorizonReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	/*
		for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
			if instance.Spec.Horizon.Template.Override.Service == nil {
				instance.Spec.Horizon.Template.Override.Service = map[string]service.RoutedOverrideSpec{}
			}
			instance.Spec.Horizon.Template.Override.Service[string(endpointType)] =
				AddServiceComponentLabel(
					ptr.To(instance.Spec.Horizon.Template.Override.Service[string(endpointType)]),
					horizon.Name)
	*/
	serviceOverrides := map[service.Endpoint]service.RoutedOverrideSpec{}
	if instance.Spec.Horizon.Template.Override.Service != nil {
		serviceOverrides[service.EndpointPublic] = *instance.Spec.Horizon.Template.Override.Service
	}

	// add selector to service overrides
	serviceOverrides[service.EndpointPublic] = AddServiceComponentLabel(
		serviceOverrides[service.EndpointPublic],
		horizon.Name)

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "horizon", Namespace: instance.Namespace}, horizon); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	if horizon.Status.Conditions.IsTrue(condition.ExposeServiceReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: horizon.Name},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		serviceOverrides, ctrlResult, err = EnsureRoute(
			ctx,
			instance,
			helper,
			horizon,
			svcs,
			serviceOverrides,
			instance.Spec.Horizon.APIOverride.Route,
			corev1beta1.OpenStackControlPlaneExposeHorizonReadyCondition,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	helper.GetLogger().Info("Reconcile Horizon", "horizon.Namespace", instance.Namespace, "horizon.Name", "horizon")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), horizon, func() error {
		instance.Spec.Horizon.Template.DeepCopyInto(&horizon.Spec)
		horizon.Spec.Override.Service = ptr.To(serviceOverrides[service.EndpointPublic])

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), horizon, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHorizonReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneHorizonReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Horizon %s - %s", horizon.Name, op))
	}

	if horizon.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneHorizonReadyCondition, corev1beta1.OpenStackControlPlaneHorizonReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHorizonReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneHorizonReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
