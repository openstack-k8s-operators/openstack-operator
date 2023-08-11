package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileHeat -
func ReconcileHeat(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	heat := &heatv1.Heat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "heat",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Heat.Enabled {
		if res, err := EnsureDeleted(ctx, helper, heat); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneHeatReadyCondition)
		return ctrl.Result{}, nil
	}

	// Create service overrides to pass into the service CR
	// and expose the public endpoint using a route per default.
	// Any trailing path will be added on the service-operator level.
	serviceDetails := []ServiceDetails{}

	// HeatAPI
	apiServiceOverrides := map[string]service.OverrideSpec{}
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {

		sd := ServiceDetails{
			ServiceName:         heat.Name + "-api",
			Namespace:           instance.Namespace,
			Endpoint:            endpointType,
			ServiceOverrideSpec: instance.Spec.Heat.Template.HeatAPI.Override.Service,
			RouteOverrideSpec:   instance.Spec.Heat.APIOverride.Route,
		}

		svcOverride, ctrlResult, err := sd.CreateRouteAndServiceOverride(ctx, instance, helper)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		serviceDetails = append(
			serviceDetails,
			sd,
		)

		apiServiceOverrides[string(endpointType)] = *svcOverride
	}

	// HeatCfnAPI
	cfnAPIServiceOverrides := map[string]service.OverrideSpec{}
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {

		sd := ServiceDetails{
			ServiceName:         heat.Name + "-cfnapi",
			Namespace:           instance.Namespace,
			Endpoint:            endpointType,
			ServiceOverrideSpec: instance.Spec.Heat.Template.HeatAPI.Override.Service,
			RouteOverrideSpec:   instance.Spec.Heat.APIOverride.Route,
		}

		svcOverride, ctrlResult, err := sd.CreateRouteAndServiceOverride(ctx, instance, helper)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		serviceDetails = append(
			serviceDetails,
			sd,
		)

		cfnAPIServiceOverrides[string(endpointType)] = *svcOverride
	}
	instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneServiceOverrideReadyCondition, corev1beta1.OpenStackControlPlaneServiceOverrideReadyMessage)

	helper.GetLogger().Info("Reconcile heat", "heat.Namespace", instance.Namespace, "heat.Name", "heat")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), heat, func() error {
		instance.Spec.Heat.Template.DeepCopyInto(&heat.Spec)
		heat.Spec.HeatAPI.Override.Service = apiServiceOverrides
		heat.Spec.HeatCfnAPI.Override.Service = cfnAPIServiceOverrides

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
		helper.GetLogger().Info(fmt.Sprintf("heat %s - %s", heat.Name, op))
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

	for _, sd := range serviceDetails {
		// Add the service CR to the ownerRef list of the route to prevent the route being deleted
		// before the service is deleted. Otherwise this can result cleanup issues which require
		// the endpoint to be reachable.
		// If ALL objects in the list have been deleted, this object will be garbage collected.
		// https://github.com/kubernetes/apimachinery/blob/15d95c0b2af3f4fcf46dce24105e5fbb9379af5a/pkg/apis/meta/v1/types.go#L240-L247
		err = sd.AddOwnerRef(ctx, helper, heat)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
