package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	heatName = "heat"
)

// ReconcileHeat -
func ReconcileHeat(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	heat := &heatv1.Heat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      heatName,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Heat.Enabled {
		if res, err := EnsureDeleted(ctx, helper, heat); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneHeatReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeHeatReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Heat.Template.HeatAPI.Override.Service == nil {
			instance.Spec.Heat.Template.HeatAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Heat.Template.HeatAPI.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Heat.Template.HeatAPI.Override.Service[endpointType],
				heat.Name+"-api")

		if instance.Spec.Heat.Template.HeatCfnAPI.Override.Service == nil {
			instance.Spec.Heat.Template.HeatCfnAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Heat.Template.HeatCfnAPI.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Heat.Template.HeatCfnAPI.Override.Service[endpointType],
				heat.Name+"-cfn")
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "heat", Namespace: instance.Namespace}, heat); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Heat API
	if heat.Status.Conditions.IsTrue(heatv1.HeatAPIReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: heat.Name + "-api"},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			heat,
			svcs,
			instance.Spec.Heat.Template.HeatAPI.Override.Service,
			instance.Spec.Heat.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeHeatReadyCondition,
			true, // TODO: (mschuppert) disable TLS for now until implemented
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		instance.Spec.Heat.Template.HeatAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
	}

	// Heat CFNAPI
	if heat.Status.Conditions.IsTrue(heatv1.HeatCfnAPIReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: heat.Name + "-cfn"},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			heat,
			svcs,
			instance.Spec.Heat.Template.HeatCfnAPI.Override.Service,
			instance.Spec.Heat.CnfAPIOverride,
			corev1beta1.OpenStackControlPlaneExposeHeatReadyCondition,
			true, // TODO: (mschuppert) disable TLS for now until implemented
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		instance.Spec.Heat.Template.HeatCfnAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
	}

	Log := GetLogger(ctx)

	Log.Info("Reconcile heat", "heat.Namespace", instance.Namespace, "heat.Name", "heat")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), heat, func() error {
		instance.Spec.Heat.Template.DeepCopyInto(&heat.Spec)

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), heat, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHeatReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneHeatReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("heat %s - %s", heat.Name, op))
	}

	if heat.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneHeatReadyCondition, corev1beta1.OpenStackControlPlaneHeatReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHeatReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneHeatReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
