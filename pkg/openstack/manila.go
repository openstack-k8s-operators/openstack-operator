package openstack

import (
	"context"
	"errors"
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
func ReconcileManila(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
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
		instance.Status.ContainerImages.ManilaAPIImage = nil
		instance.Status.ContainerImages.ManilaSchedulerImage = nil
		instance.Status.ContainerImages.ManilaShareImages = make(map[string]*string)
		return ctrl.Result{}, nil
	}

	if instance.Spec.Manila.Template == nil {
		instance.Spec.Manila.Template = &manilav1.ManilaSpecCore{}
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
	if instance.Spec.TLS.PodLevel.Enabled {
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
			instance.Spec.Manila.Template.ManilaAPI.TLS,
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

	if instance.Spec.Manila.Template.NodeSelector == nil {
		instance.Spec.Manila.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// There's no Topology referenced in Manila Template, inject the top-level
	// one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Manila.Template.TopologyRef == nil {
		instance.Spec.Manila.Template.TopologyRef = instance.Spec.TopologyRef
	}

	// When no NotificationsBusInstance is referenced in the subCR (override)
	// try to inject the top-level one if defined
	if instance.Spec.Manila.Template.NotificationsBusInstance == nil {
		instance.Spec.Manila.Template.NotificationsBusInstance = instance.Spec.NotificationsBusInstance
	}

	Log.Info("Reconciling Manila", "Manila.Namespace", instance.Namespace, "Manila.Name", "manila")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), manila, func() error {
		instance.Spec.Manila.Template.ManilaSpecBase.DeepCopyInto(&manila.Spec.ManilaSpecBase)
		instance.Spec.Manila.Template.ManilaAPI.DeepCopyInto(&manila.Spec.ManilaAPI.ManilaAPITemplateCore)
		instance.Spec.Manila.Template.ManilaScheduler.DeepCopyInto(&manila.Spec.ManilaScheduler.ManilaSchedulerTemplateCore)
		manila.Spec.NodeSelector = instance.Spec.Manila.Template.NodeSelector

		manila.Spec.ManilaAPI.ContainerImage = *version.Status.ContainerImages.ManilaAPIImage
		manila.Spec.ManilaScheduler.ContainerImage = *version.Status.ContainerImages.ManilaSchedulerImage

		defaultShareImg := version.Status.ContainerImages.ManilaShareImages["default"]
		if defaultShareImg == nil {
			return errors.New("default Manila Share images is unset")
		}

		// Discard old list of share services and rebuild it
		manila.Spec.ManilaShares = make(map[string]manilav1.ManilaShareTemplate)

		for name, share := range instance.Spec.Manila.Template.ManilaShares {
			manilaCore := manilav1.ManilaShareTemplate{}
			share.DeepCopyInto(&manilaCore.ManilaShareTemplateCore)
			if volVal, ok := version.Status.ContainerImages.ManilaShareImages[name]; ok {
				manilaCore.ContainerImage = *volVal
			} else {
				manilaCore.ContainerImage = *defaultShareImg
			}
			manila.Spec.ManilaShares[name] = manilaCore
		}

		if manila.Spec.Secret == "" {
			manila.Spec.Secret = instance.Spec.Secret
		}
		if manila.Spec.DatabaseInstance == "" {
			//manila.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			manila.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		// Append globally defined extraMounts to the service's own list.
		for _, ev := range instance.Spec.ExtraMounts {
			manila.Spec.ExtraMounts = append(manila.Spec.ExtraMounts, manilav1.ManilaExtraVolMounts{
				Name:      ev.Name,
				Region:    ev.Region,
				VolMounts: ev.VolMounts,
			})
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

	if manila.Status.ObservedGeneration == manila.Generation && manila.IsReady() {
		Log.Info("Manila ready condition is true")
		instance.Status.ContainerImages.ManilaAPIImage = version.Status.ContainerImages.ManilaAPIImage
		instance.Status.ContainerImages.ManilaSchedulerImage = version.Status.ContainerImages.ManilaSchedulerImage
		instance.Status.ContainerImages.ManilaShareImages = version.Status.ContainerImages.ManilaShareImages
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneManilaReadyCondition, corev1beta1.OpenStackControlPlaneManilaReadyMessage)
	} else {
		// We want to mirror the condition of the highest priority from the Manila resource into the instance
		// under the condition of type OpenStackControlPlaneManilaReadyCondition, but only if the sub-resource
		// currently has any conditions (which won't be true for the initial creation of the sub-resource, since
		// it has not gone through a reconcile loop yet to have any conditions).  If this condition ends up being
		// the highest priority condition in the OpenStackControlPlane, it will appear in the OpenStackControlPlane's
		// "Ready" condition at the end of the reconciliation loop, clearly surfacing the condition to the user in
		// the "oc get oscontrolplane -n <namespace>" output.
		if len(manila.Status.Conditions) > 0 {
			MirrorSubResourceCondition(manila.Status.Conditions, corev1beta1.OpenStackControlPlaneManilaReadyCondition, instance, manila.Kind)
		} else {
			// Default to the associated "running" condition message for the sub-resource if it currently lacks any conditions for mirroring
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneManilaReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneManilaReadyRunningMessage))
		}
	}

	return ctrl.Result{}, nil
}

// ManilaImageMatch - return true if the Manila images match on the ControlPlane and Version, or if Manila is not enabled
func ManilaImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Manila.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.ManilaAPIImage, version.Status.ContainerImages.ManilaAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.ManilaSchedulerImage, version.Status.ContainerImages.ManilaSchedulerImage) {
			Log.Info("Manila images do not match")
			return false
		}
		for name, img := range version.Status.ContainerImages.ManilaShareImages {
			if !stringPointersEqual(controlPlane.Status.ContainerImages.ManilaShareImages[name], img) {
				Log.Info("Manila share images do not match")
				return false
			}
		}
	}

	return true
}
