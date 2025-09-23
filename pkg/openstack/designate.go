package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileDesignate -
func ReconcileDesignate(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	designate := &designatev1.Designate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "designate",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Designate.Enabled {
		if res, err := EnsureDeleted(ctx, helper, designate); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneDesignateReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeDesignateReadyCondition)
		instance.Status.ContainerImages.DesignateAPIImage = nil
		instance.Status.ContainerImages.DesignateCentralImage = nil
		instance.Status.ContainerImages.DesignateMdnsImage = nil
		instance.Status.ContainerImages.DesignateProducerImage = nil
		instance.Status.ContainerImages.DesignateWorkerImage = nil
		instance.Status.ContainerImages.DesignateBackendbind9Image = nil
		instance.Status.ContainerImages.DesignateUnboundImage = nil
		instance.Status.ContainerImages.NetUtilsImage = nil
		return ctrl.Result{}, nil
	}

	if instance.Spec.Designate.Template == nil {
		instance.Spec.Designate.Template = &designatev1.DesignateSpecCore{}
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Designate.Template.DesignateAPI.Override.Service == nil {
			instance.Spec.Designate.Template.DesignateAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Designate.Template.DesignateAPI.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Designate.Template.DesignateAPI.Override.Service[endpointType],
				designate.Name)
	}

	// preserve any previously set TLS certs, set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Designate.Template.DesignateAPI.TLS = designate.Spec.DesignateAPI.TLS
	}

	instance.Spec.Designate.Template.DesignateAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "designate", Namespace: instance.Namespace}, designate); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(designate.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Designate.Template.DesignateAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			designate,
			svcs,
			instance.Spec.Designate.Template.DesignateAPI.Override.Service,
			instance.Spec.Designate.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeDesignateReadyCondition,
			false, // TODO: (oschwart) could be removed when all integrated service support TLS
			instance.Spec.Designate.Template.DesignateAPI.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Designate.Template.DesignateAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()

		// update TLS settings with cert secret
		instance.Spec.Designate.Template.DesignateAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Designate.Template.DesignateAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	if instance.Spec.Designate.Template.NodeSelector == nil {
		instance.Spec.Designate.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Designate.Template.TopologyRef == nil {
		instance.Spec.Designate.Template.TopologyRef = instance.Spec.TopologyRef
	}

	helper.GetLogger().Info("Reconciling Designate", "Designate.Namespace", instance.Namespace, "Designate.Name", "designate")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), designate, func() error {
		// FIXME: the designate structs need some rework (images should be at the top level, not in the sub structs)
		instance.Spec.Designate.Template.DesignateSpecBase.DeepCopyInto(&designate.Spec.DesignateSpecBase)
		// API
		instance.Spec.Designate.Template.DesignateAPI.DesignateAPISpecBase.DeepCopyInto(&designate.Spec.DesignateAPI.DesignateAPISpecBase)
		instance.Spec.Designate.Template.DesignateAPI.DesignateServiceTemplateCore.DeepCopyInto(&designate.Spec.DesignateAPI.DesignateServiceTemplateCore)
		// Central
		instance.Spec.Designate.Template.DesignateCentral.DesignateCentralSpecBase.DeepCopyInto(&designate.Spec.DesignateCentral.DesignateCentralSpecBase)
		instance.Spec.Designate.Template.DesignateCentral.DesignateServiceTemplateCore.DeepCopyInto(&designate.Spec.DesignateCentral.DesignateServiceTemplateCore)
		// Worker
		instance.Spec.Designate.Template.DesignateWorker.DesignateWorkerSpecBase.DeepCopyInto(&designate.Spec.DesignateWorker.DesignateWorkerSpecBase)
		instance.Spec.Designate.Template.DesignateWorker.DesignateServiceTemplateCore.DeepCopyInto(&designate.Spec.DesignateWorker.DesignateServiceTemplateCore)
		// Mdns
		instance.Spec.Designate.Template.DesignateMdns.DesignateMdnsSpecBase.DeepCopyInto(&designate.Spec.DesignateMdns.DesignateMdnsSpecBase)
		instance.Spec.Designate.Template.DesignateMdns.DesignateServiceTemplateCore.DeepCopyInto(&designate.Spec.DesignateMdns.DesignateServiceTemplateCore)
		// Producer
		instance.Spec.Designate.Template.DesignateProducer.DesignateProducerSpecBase.DeepCopyInto(&designate.Spec.DesignateProducer.DesignateProducerSpecBase)
		instance.Spec.Designate.Template.DesignateProducer.DesignateServiceTemplateCore.DeepCopyInto(&designate.Spec.DesignateProducer.DesignateServiceTemplateCore)
		// Bind9
		instance.Spec.Designate.Template.DesignateBackendbind9.DesignateBackendbind9SpecBase.DeepCopyInto(&designate.Spec.DesignateBackendbind9.DesignateBackendbind9SpecBase)
		instance.Spec.Designate.Template.DesignateBackendbind9.DesignateServiceTemplateCore.DeepCopyInto(&designate.Spec.DesignateBackendbind9.DesignateServiceTemplateCore)
		// Unbound
		instance.Spec.Designate.Template.DesignateUnbound.DesignateUnboundSpecBase.DeepCopyInto(&designate.Spec.DesignateUnbound.DesignateUnboundSpecBase)
		instance.Spec.Designate.Template.DesignateUnbound.DesignateServiceTemplateCore.DeepCopyInto(&designate.Spec.DesignateUnbound.DesignateServiceTemplateCore)

		designate.Spec.DesignateAPI.ContainerImage = *version.Status.ContainerImages.DesignateAPIImage
		designate.Spec.DesignateCentral.ContainerImage = *version.Status.ContainerImages.DesignateCentralImage
		designate.Spec.DesignateMdns.ContainerImage = *version.Status.ContainerImages.DesignateMdnsImage
		designate.Spec.DesignateProducer.ContainerImage = *version.Status.ContainerImages.DesignateProducerImage
		designate.Spec.DesignateWorker.ContainerImage = *version.Status.ContainerImages.DesignateWorkerImage
		designate.Spec.DesignateBackendbind9.ContainerImage = *version.Status.ContainerImages.DesignateBackendbind9Image
		designate.Spec.DesignateUnbound.ContainerImage = *version.Status.ContainerImages.DesignateUnboundImage
		designate.Spec.DesignateBackendbind9.NetUtilsImage = *version.Status.ContainerImages.NetUtilsImage
		designate.Spec.DesignateMdns.NetUtilsImage = *version.Status.ContainerImages.NetUtilsImage

		if designate.Spec.Secret == "" {
			designate.Spec.Secret = instance.Spec.Secret
		}
		if designate.Spec.DatabaseInstance == "" {
			//designate.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			designate.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), designate, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneDesignateReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneDesignateReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Designate %s - %s", designate.Name, op))
	}

	if designate.Status.ObservedGeneration == designate.Generation && designate.IsReady() {
		helper.GetLogger().Info("Designate ready condition is true")
		instance.Status.ContainerImages.DesignateAPIImage = version.Status.ContainerImages.DesignateAPIImage
		instance.Status.ContainerImages.DesignateCentralImage = version.Status.ContainerImages.DesignateCentralImage
		instance.Status.ContainerImages.DesignateMdnsImage = version.Status.ContainerImages.DesignateMdnsImage
		instance.Status.ContainerImages.DesignateProducerImage = version.Status.ContainerImages.DesignateProducerImage
		instance.Status.ContainerImages.DesignateWorkerImage = version.Status.ContainerImages.DesignateWorkerImage
		instance.Status.ContainerImages.DesignateBackendbind9Image = version.Status.ContainerImages.DesignateBackendbind9Image
		instance.Status.ContainerImages.DesignateUnboundImage = version.Status.ContainerImages.DesignateUnboundImage
		instance.Status.ContainerImages.NetUtilsImage = version.Status.ContainerImages.NetUtilsImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneDesignateReadyCondition, corev1beta1.OpenStackControlPlaneDesignateReadyMessage)
	} else {
		// We want to mirror the condition of the highest priority from the Designate resource into the instance
		// under the condition of type OpenStackControlPlaneDesignateReadyCondition, but only if the sub-resource
		// currently has any conditions (which won't be true for the initial creation of the sub-resource, since
		// it has not gone through a reconcile loop yet to have any conditions).  If this condition ends up being
		// the highest priority condition in the OpenStackControlPlane, it will appear in the OpenStackControlPlane's
		// "Ready" condition at the end of the reconciliation loop, clearly surfacing the condition to the user in
		// the "oc get oscontrolplane -n <namespace>" output.
		if len(designate.Status.Conditions) > 0 {
			MirrorSubResourceCondition(designate.Status.Conditions, corev1beta1.OpenStackControlPlaneDesignateReadyCondition, instance, designate.Kind)
		} else {
			// Default to the associated "running" condition message for the sub-resource if it currently lacks any conditions for mirroring
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneDesignateReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneDesignateReadyRunningMessage))
		}
	}

	return ctrl.Result{}, nil

}

// DesignateImageMatch - return true if the Designate images match on the ControlPlane and Version, or if Designate is not enabled
func DesignateImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.Designate.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.DesignateAPIImage, version.Status.ContainerImages.DesignateAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.DesignateCentralImage, version.Status.ContainerImages.DesignateCentralImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.DesignateMdnsImage, version.Status.ContainerImages.DesignateMdnsImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.DesignateProducerImage, version.Status.ContainerImages.DesignateProducerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.DesignateWorkerImage, version.Status.ContainerImages.DesignateWorkerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.DesignateBackendbind9Image, version.Status.ContainerImages.DesignateBackendbind9Image) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.DesignateUnboundImage, version.Status.ContainerImages.DesignateUnboundImage) {
			Log.Info("Designate images do not match")
			return false
		}
	}

	return true
}
