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
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	Log := GetLogger(ctx)

	if !instance.Spec.Manila.Enabled {
		if res, err := EnsureDeleted(ctx, helper, manila); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneManilaReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeManilaReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Manila.Template.ManilaAPI.Override.Service == nil {
			instance.Spec.Manila.Template.ManilaAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Manila.Template.ManilaAPI.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Manila.Template.ManilaAPI.Override.Service[endpointType],
				manila.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "manila", Namespace: instance.Namespace}, manila); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs, set CA cert
	if instance.Spec.TLS.Enabled(service.EndpointInternal) {
		instance.Spec.Manila.Template.ManilaAPI.TLS = manila.Spec.ManilaAPI.TLS
	}
	instance.Spec.Manila.Template.ManilaAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	// When component services got created check if there is the need to create a route
	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(manila.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Manila.Template.ManilaAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			manila,
			svcs,
			instance.Spec.Manila.Template.ManilaAPI.Override.Service,
			instance.Spec.Manila.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeManilaReadyCondition,
			false, // TODO: (mschuppert) could be removed when all integrated service support TLS
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Manila.Template.ManilaAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Manila.Template.ManilaAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Manila.Template.ManilaAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	Log.Info("Reconciling Manila", "Manila.Namespace", instance.Namespace, "Manila.Name", "manila")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), manila, func() error {
		instance.Spec.Manila.Template.DeepCopyInto(&manila.Spec)

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
		Log.Info(fmt.Sprintf("Manila %s - %s", manila.Name, op))
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

	return ctrl.Result{}, nil
}
