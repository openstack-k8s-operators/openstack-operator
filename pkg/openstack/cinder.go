package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileCinder -
func ReconcileCinder(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	cinder := &cinderv1.Cinder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cinder",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Cinder.Enabled {
		if res, err := EnsureDeleted(ctx, helper, cinder); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneCinderReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeCinderReadyCondition)
		return ctrl.Result{}, nil
	}
	Log := GetLogger(ctx)

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Cinder.Template.CinderAPI.Override.Service == nil {
			instance.Spec.Cinder.Template.CinderAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Cinder.Template.CinderAPI.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Cinder.Template.CinderAPI.Override.Service[endpointType],
				cinder.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "cinder", Namespace: instance.Namespace}, cinder); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	if cinder.Status.Conditions.IsTrue(cinderv1.CinderAPIReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: cinder.Name},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			cinder,
			svcs,
			instance.Spec.Cinder.Template.CinderAPI.Override.Service,
			instance.Spec.Cinder.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeCinderReadyCondition,
			true, // TODO: (mschuppert) disable TLS for now until implemented
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		instance.Spec.Cinder.Template.CinderAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
	}

	Log.Info("Reconciling Cinder", "Cinder.Namespace", instance.Namespace, "Cinder.Name", "cinder")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), cinder, func() error {
		instance.Spec.Cinder.Template.DeepCopyInto(&cinder.Spec)

		if cinder.Spec.Secret == "" {
			cinder.Spec.Secret = instance.Spec.Secret
		}
		if cinder.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			cinder.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if cinder.Spec.DatabaseInstance == "" {
			//cinder.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			cinder.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		// if already defined at service level (template section), we don't merge
		// with the global defined extra volumes
		if len(cinder.Spec.ExtraMounts) == 0 {

			var cinderVolumes []cinderv1.CinderExtraVolMounts

			for _, ev := range instance.Spec.ExtraMounts {
				cinderVolumes = append(cinderVolumes, cinderv1.CinderExtraVolMounts{
					Name:      ev.Name,
					Region:    ev.Region,
					VolMounts: ev.VolMounts,
				})
			}
			cinder.Spec.ExtraMounts = cinderVolumes
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), cinder, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCinderReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneCinderReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("Cinder %s - %s", cinder.Name, op))
	}

	if cinder.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneCinderReadyCondition, corev1beta1.OpenStackControlPlaneCinderReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCinderReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneCinderReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}
