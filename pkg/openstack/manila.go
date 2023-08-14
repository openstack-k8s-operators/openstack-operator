package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileManila -
func ReconcileManila(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	manila := &manilav1.Manila{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manila",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Manila.Enabled {
		if res, err := EnsureDeleted(ctx, helper, manila); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneManilaReadyCondition)
		return ctrl.Result{}, nil
	}

	// Create service overrides to pass into the service CR
	// and expose the public endpoint using a route per default.
	// Any trailing path will be added on the service-operator level.
	serviceOverrides := map[string]service.OverrideSpec{}
	serviceDetails := []ServiceDetails{}
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {

		sd := ServiceDetails{
			ServiceName:         manila.Name,
			Namespace:           instance.Namespace,
			Endpoint:            endpointType,
			ServiceOverrideSpec: instance.Spec.Manila.Template.ManilaAPI.Override.Service,
			RouteOverrideSpec:   instance.Spec.Manila.APIOverride.Route,
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

	helper.GetLogger().Info("Reconciling Manila", "Manila.Namespace", instance.Namespace, "Manila.Name", "manila")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), manila, func() error {
		instance.Spec.Manila.Template.DeepCopyInto(&manila.Spec)
		manila.Spec.ManilaAPI.Override.Service = serviceOverrides

		if manila.Spec.Secret == "" {
			manila.Spec.Secret = instance.Spec.Secret
		}
		if manila.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			manila.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if manila.Spec.DatabaseInstance == "" {
			//manila.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			manila.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		// if already defined at service level (template section), we don't merge
		// with the global defined extra volumes
		if len(manila.Spec.ExtraMounts) == 0 {

			var manilaVolumes []manilav1.ManilaExtraVolMounts

			for _, ev := range instance.Spec.ExtraMounts {
				manilaVolumes = append(manilaVolumes, manilav1.ManilaExtraVolMounts{
					Name:      ev.Name,
					Region:    ev.Region,
					VolMounts: ev.VolMounts,
				})
			}
			manila.Spec.ExtraMounts = manilaVolumes
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), manila, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneManilaReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneManilaReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Manila %s - %s", manila.Name, op))
	}

	if manila.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneManilaReadyCondition, corev1beta1.OpenStackControlPlaneManilaReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneManilaReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneManilaReadyRunningMessage))
	}

	for _, sd := range serviceDetails {
		// Add the service CR to the ownerRef list of the route to prevent the route being deleted
		// before the service is deleted. Otherwise this can result cleanup issues which require
		// the endpoint to be reachable.
		// If ALL objects in the list have been deleted, this object will be garbage collected.
		// https://github.com/kubernetes/apimachinery/blob/15d95c0b2af3f4fcf46dce24105e5fbb9379af5a/pkg/apis/meta/v1/types.go#L240-L247
		err = sd.AddOwnerRef(ctx, helper, manila)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
