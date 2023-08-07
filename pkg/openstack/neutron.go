package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileNeutron -
func ReconcileNeutron(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	neutronAPI := &neutronv1.NeutronAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neutron",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Neutron.Enabled {
		if res, err := EnsureDeleted(ctx, helper, neutronAPI); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneNeutronReadyCondition)
		return ctrl.Result{}, nil
	}

	// Create service overrides to pass into the service CR
	// and expose the public endpoint using a route per default.
	// Any trailing path will be added on the service-operator level.
	serviceOverrides := map[string]service.OverrideSpec{}
	serviceDetails := []ServiceDetails{}

	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		sd := ServiceDetails{
			ServiceName:         neutronAPI.Name,
			Namespace:           instance.Namespace,
			Endpoint:            endpointType,
			ServiceOverrideSpec: instance.Spec.Placement.Template.Override.Service,
			RouteOverrideSpec:   instance.Spec.Placement.APIOverride.Route,
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

		serviceOverrides[string(endpointType)] = *svcOverride
	}
	instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneServiceOverrideReadyCondition, corev1beta1.OpenStackControlPlaneServiceOverrideReadyMessage)

	helper.GetLogger().Info("Reconciling NeutronAPI", "NeutronAPI.Namespace", instance.Namespace, "NeutronAPI.Name", "neutron")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), neutronAPI, func() error {
		instance.Spec.Neutron.Template.DeepCopyInto(&neutronAPI.Spec)
		neutronAPI.Spec.Override.Service = serviceOverrides
		if neutronAPI.Spec.Secret == "" {
			neutronAPI.Spec.Secret = instance.Spec.Secret
		}
		if neutronAPI.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			neutronAPI.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if neutronAPI.Spec.DatabaseInstance == "" {
			neutronAPI.Spec.DatabaseInstance = "openstack"
		}

		// if already defined at service level (template section), we don't merge
		// with the global defined extra volumes
		if len(neutronAPI.Spec.ExtraMounts) == 0 {

			var neutronVolumes []neutronv1.NeutronExtraVolMounts

			for _, ev := range instance.Spec.ExtraMounts {
				neutronVolumes = append(neutronVolumes, neutronv1.NeutronExtraVolMounts{
					Name:      ev.Name,
					Region:    ev.Region,
					VolMounts: ev.VolMounts,
				})
			}
			neutronAPI.Spec.ExtraMounts = neutronVolumes
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), neutronAPI, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNeutronReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneNeutronReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("neutronAPI %s - %s", neutronAPI.Name, op))
	}

	if neutronAPI.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneNeutronReadyCondition, corev1beta1.OpenStackControlPlaneNeutronReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNeutronReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneNeutronReadyRunningMessage))
	}

	for _, sd := range serviceDetails {
		// Add the service CR to the ownerRef list of the route to prevent the route being deleted
		// before the service is deleted. Otherwise this can result cleanup issues which require
		// the endpoint to be reachable.
		// If ALL objects in the list have been deleted, this object will be garbage collected.
		// https://github.com/kubernetes/apimachinery/blob/15d95c0b2af3f4fcf46dce24105e5fbb9379af5a/pkg/apis/meta/v1/types.go#L240-L247
		err = sd.AddOwnerRef(ctx, helper, neutronAPI)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil

}
