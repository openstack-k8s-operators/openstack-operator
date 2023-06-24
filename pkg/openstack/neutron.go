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

	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	Log := GetLogger(ctx)

	if !instance.Spec.Neutron.Enabled {
		if res, err := EnsureDeleted(ctx, helper, neutronAPI); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneNeutronReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeNeutronReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Neutron.Template.Override.Service == nil {
			instance.Spec.Neutron.Template.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Neutron.Template.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Neutron.Template.Override.Service[endpointType],
				neutronAPI.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "neutron", Namespace: instance.Namespace}, neutronAPI); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	if neutronAPI.Status.Conditions.IsTrue(condition.ExposeServiceReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: neutronAPI.Name},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		instance.Spec.Neutron.Template.Override.Service, ctrlResult, err = EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			neutronAPI,
			svcs,
			instance.Spec.Neutron.Template.Override.Service,
			instance.Spec.Neutron.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeNeutronReadyCondition,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	Log.Info("Reconciling NeutronAPI", "NeutronAPI.Namespace", instance.Namespace, "NeutronAPI.Name", "neutron")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), neutronAPI, func() error {
		instance.Spec.Neutron.Template.DeepCopyInto(&neutronAPI.Spec)

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
		Log.Info(fmt.Sprintf("neutronAPI %s - %s", neutronAPI.Name, op))
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

	return ctrl.Result{}, nil

}
