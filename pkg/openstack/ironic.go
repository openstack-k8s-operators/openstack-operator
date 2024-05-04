package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileIronic -
func ReconcileIronic(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	ironic := &ironicv1.Ironic{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ironic",
			Namespace: instance.Namespace,
		},
	}
	Log := GetLogger(ctx)

	if !instance.Spec.Ironic.Enabled {
		if res, err := EnsureDeleted(ctx, helper, ironic); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneIronicReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeIronicReadyCondition)
		instance.Status.ContainerImages.IronicAPIImage = nil
		instance.Status.ContainerImages.IronicConductorImage = nil
		instance.Status.ContainerImages.IronicInspectorImage = nil
		instance.Status.ContainerImages.IronicNeutronAgentImage = nil
		instance.Status.ContainerImages.IronicPxeImage = nil
		instance.Status.ContainerImages.IronicPythonAgentImage = nil
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Ironic.Template.IronicAPI.Override.Service == nil {
			instance.Spec.Ironic.Template.IronicAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Ironic.Template.IronicAPI.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Ironic.Template.IronicAPI.Override.Service[endpointType],
				ironic.Name+"-api")

		if instance.Spec.Ironic.Template.IronicInspector.Override.Service == nil {
			instance.Spec.Ironic.Template.IronicInspector.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Ironic.Template.IronicInspector.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Ironic.Template.IronicInspector.Override.Service[endpointType],
				ironic.Name+"-inspector")
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "ironic", Namespace: instance.Namespace}, ironic); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs,set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Ironic.Template.IronicAPI.TLS = ironic.Spec.IronicAPI.TLS
		instance.Spec.Ironic.Template.IronicInspector.TLS = ironic.Spec.IronicInspector.TLS
	}
	instance.Spec.Ironic.Template.IronicAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
	instance.Spec.Ironic.Template.IronicInspector.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	// Ironic API
	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(ironic.Name+"-api"),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Ironic.Template.IronicAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			ironic,
			svcs,
			instance.Spec.Ironic.Template.IronicAPI.Override.Service,
			instance.Spec.Ironic.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeIronicReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Ironic.Template.IronicAPI.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		instance.Spec.Ironic.Template.IronicAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Ironic.Template.IronicAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Ironic.Template.IronicAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	// Ironic Inspector
	svcs, err = service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(ironic.Name+"-inspector"),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Ironic.Template.IronicInspector.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			ironic,
			svcs,
			instance.Spec.Ironic.Template.IronicInspector.Override.Service,
			instance.Spec.Ironic.InspectorOverride,
			corev1beta1.OpenStackControlPlaneExposeIronicReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Ironic.Template.IronicInspector.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Ironic.Template.IronicInspector.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Ironic.Template.IronicInspector.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Ironic.Template.IronicInspector.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	Log.Info("Reconciling Ironic", "Ironic.Namespace", instance.Namespace, "Ironic.Name", "ironic")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), ironic, func() error {
		instance.Spec.Ironic.Template.DeepCopyInto(&ironic.Spec.IronicSpecCore)

		ironic.Spec.Images.API = *version.Status.ContainerImages.IronicAPIImage
		ironic.Spec.Images.Conductor = *version.Status.ContainerImages.IronicConductorImage
		ironic.Spec.Images.Inspector = *version.Status.ContainerImages.IronicInspectorImage
		ironic.Spec.Images.NeutronAgent = *version.Status.ContainerImages.IronicNeutronAgentImage
		ironic.Spec.Images.Pxe = *version.Status.ContainerImages.IronicPxeImage
		ironic.Spec.Images.IronicPythonAgent = *version.Status.ContainerImages.IronicPythonAgentImage

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), ironic, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneIronicReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneIronicReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("ironic %s - %s", ironic.Name, op))
	}

	if ironic.Status.ObservedGeneration == ironic.Generation && ironic.IsReady() {
		instance.Status.ContainerImages.IronicAPIImage = version.Status.ContainerImages.IronicAPIImage
		instance.Status.ContainerImages.IronicConductorImage = version.Status.ContainerImages.IronicConductorImage
		instance.Status.ContainerImages.IronicInspectorImage = version.Status.ContainerImages.IronicInspectorImage
		instance.Status.ContainerImages.IronicNeutronAgentImage = version.Status.ContainerImages.IronicNeutronAgentImage
		instance.Status.ContainerImages.IronicPxeImage = version.Status.ContainerImages.IronicPxeImage
		instance.Status.ContainerImages.IronicPythonAgentImage = version.Status.ContainerImages.IronicPythonAgentImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneIronicReadyCondition, corev1beta1.OpenStackControlPlaneIronicReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneIronicReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneIronicReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// IronicImagesCheck - return true if the ironic images match on the ControlPlane and Version, or if Ironic is not enabled
func IronicImageCheck(controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {

	if controlPlane.Spec.Ironic.Enabled {
		if !compareStringPointers(controlPlane.Status.ContainerImages.IronicAPIImage, version.Status.ContainerImages.IronicAPIImage) ||
			!compareStringPointers(controlPlane.Status.ContainerImages.IronicConductorImage, version.Status.ContainerImages.IronicConductorImage) ||
			!compareStringPointers(controlPlane.Status.ContainerImages.IronicInspectorImage, version.Status.ContainerImages.IronicInspectorImage) ||
			!compareStringPointers(controlPlane.Status.ContainerImages.IronicNeutronAgentImage, version.Status.ContainerImages.IronicNeutronAgentImage) ||
			!compareStringPointers(controlPlane.Status.ContainerImages.IronicPxeImage, version.Status.ContainerImages.IronicPxeImage) ||
			!compareStringPointers(controlPlane.Status.ContainerImages.IronicPythonAgentImage, version.Status.ContainerImages.IronicPythonAgentImage) {
			return false
		}
	}

	return true
}
