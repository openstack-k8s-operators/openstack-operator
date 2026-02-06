package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileSwift -
func ReconcileSwift(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	swift := &swiftv1.Swift{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "swift",
			Namespace: instance.Namespace,
		},
	}

	Log := GetLogger(ctx)

	if !instance.Spec.Swift.Enabled {
		if res, err := EnsureDeleted(ctx, helper, swift); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneSwiftReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeSwiftReadyCondition)
		instance.Status.ContainerImages.SwiftAccountImage = nil
		instance.Status.ContainerImages.SwiftContainerImage = nil
		instance.Status.ContainerImages.SwiftObjectImage = nil
		instance.Status.ContainerImages.SwiftProxyImage = nil
		return ctrl.Result{}, nil
	}

	if instance.Spec.Swift.Template == nil {
		instance.Spec.Swift.Template = &swiftv1.SwiftSpecCore{}
	}

	if instance.Spec.Swift.Template.NodeSelector == nil {
		instance.Spec.Swift.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Swift.Template.TopologyRef == nil {
		instance.Spec.Swift.Template.TopologyRef = instance.Spec.TopologyRef
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Swift.Template.SwiftProxy.Override.Service == nil {
			instance.Spec.Swift.Template.SwiftProxy.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Swift.Template.SwiftProxy.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Swift.Template.SwiftProxy.Override.Service[endpointType],
				swift.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "swift", Namespace: instance.Namespace}, swift); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Application Credential Management (Day-2 operation)
	swiftReady := swift.Status.ObservedGeneration == swift.GetGeneration() && swift.IsReady()

	// Apply same fallback logic as in CreateOrPatch to avoid passing empty values to AC
	swiftSecret := instance.Spec.Swift.Template.SwiftProxy.Secret
	if swiftSecret == "" {
		swiftSecret = instance.Spec.Secret
	}

	// Only call if AC enabled or currently configured
	if isACEnabled(instance.Spec.ApplicationCredential, instance.Spec.Swift.ApplicationCredential) ||
		instance.Spec.Swift.Template.SwiftProxy.Auth.ApplicationCredentialSecret != "" {

		acSecretName, acResult, err := EnsureApplicationCredentialForService(
			ctx,
			helper,
			instance,
			swift.Name,
			swiftReady,
			swiftSecret,
			instance.Spec.Swift.Template.SwiftProxy.PasswordSelectors.Service,
			instance.Spec.Swift.Template.SwiftProxy.ServiceUser,
			instance.Spec.Swift.ApplicationCredential,
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		// If AC is not ready, return immediately without updating the service CR
		if (acResult != ctrl.Result{}) {
			return acResult, nil
		}

		// Set ApplicationCredentialSecret based on what the helper returned:
		// - If AC disabled: returns ""
		// - If AC enabled and ready: returns the AC secret name
		instance.Spec.Swift.Template.SwiftProxy.Auth.ApplicationCredentialSecret = acSecretName
	}

	// preserve any previously set TLS certs,set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Swift.Template.SwiftProxy.TLS = swift.Spec.SwiftProxy.TLS
	}
	instance.Spec.Swift.Template.SwiftProxy.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(swift.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Swift.Template.SwiftProxy.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			swift,
			svcs,
			instance.Spec.Swift.Template.SwiftProxy.Override.Service,
			instance.Spec.Swift.ProxyOverride,
			corev1beta1.OpenStackControlPlaneExposeSwiftReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Swift.Template.SwiftProxy.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Swift.Template.SwiftProxy.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Swift.Template.SwiftProxy.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Swift.Template.SwiftProxy.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	Log.Info("Reconciling Swift", "Swift.Namespace", instance.Namespace, "Swift.Name", "swift")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), swift, func() error {
		instance.Spec.Swift.Template.SwiftSpecBase.DeepCopyInto(&swift.Spec.SwiftSpecBase)
		instance.Spec.Swift.Template.SwiftProxy.DeepCopyInto(&swift.Spec.SwiftProxy.SwiftProxySpecCore)
		instance.Spec.Swift.Template.SwiftStorage.DeepCopyInto(&swift.Spec.SwiftStorage.SwiftStorageSpecCore)
		instance.Spec.Swift.Template.SwiftRing.DeepCopyInto(&swift.Spec.SwiftRing.SwiftRingSpecCore)

		swift.Spec.SwiftRing.ContainerImage = *version.Status.ContainerImages.SwiftProxyImage
		swift.Spec.SwiftStorage.ContainerImageAccount = *version.Status.ContainerImages.SwiftAccountImage
		swift.Spec.SwiftStorage.ContainerImageContainer = *version.Status.ContainerImages.SwiftContainerImage
		swift.Spec.SwiftStorage.ContainerImageObject = *version.Status.ContainerImages.SwiftObjectImage
		swift.Spec.SwiftStorage.ContainerImageProxy = *version.Status.ContainerImages.SwiftProxyImage
		swift.Spec.SwiftProxy.ContainerImageProxy = *version.Status.ContainerImages.SwiftProxyImage

		if swift.Spec.SwiftProxy.Secret == "" {
			swift.Spec.SwiftProxy.Secret = instance.Spec.Secret
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), swift, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneSwiftReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneSwiftReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("Swift %s - %s", swift.Name, op))
	}

	if swift.Status.ObservedGeneration == swift.GetGeneration() && swift.IsReady() {
		Log.Info("Swift ready condition is true")
		instance.Status.ContainerImages.SwiftAccountImage = version.Status.ContainerImages.SwiftAccountImage
		instance.Status.ContainerImages.SwiftContainerImage = version.Status.ContainerImages.SwiftContainerImage
		instance.Status.ContainerImages.SwiftObjectImage = version.Status.ContainerImages.SwiftObjectImage
		instance.Status.ContainerImages.SwiftProxyImage = version.Status.ContainerImages.SwiftProxyImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneSwiftReadyCondition, corev1beta1.OpenStackControlPlaneSwiftReadyMessage)
	} else {
		// We want to mirror the condition of the highest priority from the Swift resource into the instance
		// under the condition of type OpenStackControlPlaneSwiftReadyCondition, but only if the sub-resource
		// currently has any conditions (which won't be true for the initial creation of the sub-resource, since
		// it has not gone through a reconcile loop yet to have any conditions).  If this condition ends up being
		// the highest priority condition in the OpenStackControlPlane, it will appear in the OpenStackControlPlane's
		// "Ready" condition at the end of the reconciliation loop, clearly surfacing the condition to the user in
		// the "oc get oscontrolplane -n <namespace>" output.
		if len(swift.Status.Conditions) > 0 {
			MirrorSubResourceCondition(swift.Status.Conditions, corev1beta1.OpenStackControlPlaneSwiftReadyCondition, instance, swift.Kind)
		} else {
			// Default to the associated "running" condition message for the sub-resource if it currently lacks any conditions for mirroring
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneSwiftReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneSwiftReadyRunningMessage))
		}
	}

	return ctrl.Result{}, nil
}

// SwiftImageMatch - return true if the swift images match on the ControlPlane and Version, or if Swift is not enabled
func SwiftImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Swift.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.SwiftAccountImage, version.Status.ContainerImages.SwiftAccountImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.SwiftContainerImage, version.Status.ContainerImages.SwiftContainerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.SwiftObjectImage, version.Status.ContainerImages.SwiftObjectImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.SwiftProxyImage, version.Status.ContainerImages.SwiftProxyImage) {
			Log.Info("Swift images do not match")
			return false
		}
	}

	return true
}
