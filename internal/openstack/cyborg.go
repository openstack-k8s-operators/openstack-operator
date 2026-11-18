package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cyborgv1 "github.com/openstack-k8s-operators/nova-operator/api/cyborg/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileCyborg -
func ReconcileCyborg(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	cyborg := &cyborgv1.Cyborg{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cyborg",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Cyborg.Enabled {
		if res, err := EnsureDeleted(ctx, helper, cyborg); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneCyborgReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeCyborgReadyCondition)
		instance.Status.ContainerImages.CyborgAPIImage = nil
		instance.Status.ContainerImages.CyborgAgentImage = nil
		instance.Status.ContainerImages.CyborgConductorImage = nil
		// Clean up AC CRs when service is disabled
		if err := CleanupApplicationCredentialForService(ctx, helper, instance, cyborg.Name); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if instance.Spec.Cyborg.Template == nil {
		instance.Spec.Cyborg.Template = &cyborgv1.CyborgSpecCore{}
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Cyborg.Template.APIServiceTemplate.Override.Service == nil {
			instance.Spec.Cyborg.Template.APIServiceTemplate.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Cyborg.Template.APIServiceTemplate.Override.Service[endpointType] = AddServiceOpenStackOperatorLabel(
			instance.Spec.Cyborg.Template.APIServiceTemplate.Override.Service[endpointType],
			cyborg.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "cyborg", Namespace: instance.Namespace}, cyborg); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Application Credential Management (Day-2 operation)
	// Cyborg uses pointer fields, safely extract values
	cyborgReady := cyborg.Status.Conditions != nil && cyborg.Status.ObservedGeneration == cyborg.Generation && cyborg.IsReady()

	// Helper to get Cyborg values (which are pointers) with fallback logic
	getCyborgSecret := func() string {
		if instance.Spec.Cyborg.Template.Secret != nil && *instance.Spec.Cyborg.Template.Secret != "" {
			return *instance.Spec.Cyborg.Template.Secret
		}
		// Apply same fallback as in CreateOrPatch
		return instance.Spec.Secret
	}
	getCyborgServiceUser := func() string {
		if instance.Spec.Cyborg.Template.ServiceUser != nil {
			return *instance.Spec.Cyborg.Template.ServiceUser
		}
		return ""
	}
	getCyborgPasswordSelector := func() string {
		if instance.Spec.Cyborg.Template.PasswordSelectors != nil {
			return instance.Spec.Cyborg.Template.PasswordSelectors.Service
		}
		return ""
	}

	// Always reconcile AC - EnsureApplicationCredentialForService checks cluster state and handles the full AC lifecycle.
	if instance.Spec.Cyborg.ApplicationCredential != nil ||
		instance.Spec.Cyborg.Template.Auth.ApplicationCredentialSecret != "" {

		acSecretName, acResult, err := EnsureApplicationCredentialForService(
			ctx,
			helper,
			instance,
			cyborg.Name,
			cyborgReady,
			getCyborgSecret(),
			getCyborgPasswordSelector(),
			getCyborgServiceUser(),
			instance.Spec.Cyborg.ApplicationCredential,
			false,
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
		instance.Spec.Cyborg.Template.Auth.ApplicationCredentialSecret = acSecretName
	}

	// snapshot pre-restore BYO service cert secret (pure user input; see resolveServiceCert)
	cyborgUserTLS := instance.Spec.Cyborg.Template.APIServiceTemplate.TLS
	// preserve any previously set TLS certs, set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Cyborg.Template.APIServiceTemplate.TLS = cyborg.Spec.APIServiceTemplate.TLS
	}
	instance.Spec.Cyborg.Template.APIServiceTemplate.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(cyborg.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Cyborg.Template.APIServiceTemplate.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			cyborg,
			svcs,
			instance.Spec.Cyborg.Template.APIServiceTemplate.Override.Service,
			instance.Spec.Cyborg.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeCyborgReadyCondition,
			false, // TODO: (mschuppert) could be removed when all integrated service support TLS
			cyborgUserTLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Cyborg.Template.APIServiceTemplate.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Cyborg.Template.APIServiceTemplate.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Cyborg.Template.APIServiceTemplate.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	if instance.Spec.Cyborg.Template.NodeSelector == nil {
		instance.Spec.Cyborg.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Cyborg.Template.TopologyRef == nil {
		instance.Spec.Cyborg.Template.TopologyRef = instance.Spec.TopologyRef
	}

	// Propagate MessagingBus from top-level to template if not set
	// Template-level takes precedence over top-level
	if instance.Spec.MessagingBus != nil && instance.Spec.MessagingBus.Cluster != "" {
		if instance.Spec.Cyborg.Template.MessagingBus.Cluster == "" {
			instance.Spec.Cyborg.Template.MessagingBus = *instance.Spec.MessagingBus
		}
	}

	helper.GetLogger().Info("Reconciling Cyborg", "Cyborg.Namespace", instance.Namespace, "Cyborg.Name", "cyborg")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), cyborg, func() error {
		instance.Spec.Cyborg.Template.DeepCopyInto(&cyborg.Spec.CyborgSpecCore)

		if version.Status.ContainerImages.CyborgAPIImage == nil ||
			version.Status.ContainerImages.CyborgAgentImage == nil ||
			version.Status.ContainerImages.CyborgConductorImage == nil {
			return fmt.Errorf("no Cyborg images found in the OpenStackVersion")
		}
		cyborg.Spec.APIContainerImageURL = *version.Status.ContainerImages.CyborgAPIImage
		cyborg.Spec.AgentContainerImageURL = *version.Status.ContainerImages.CyborgAgentImage
		cyborg.Spec.ConductorContainerImageURL = *version.Status.ContainerImages.CyborgConductorImage

		if cyborg.Spec.Secret != nil && *cyborg.Spec.Secret == "" {
			*cyborg.Spec.Secret = instance.Spec.Secret
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), cyborg, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCyborgReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneCyborgReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("cyborg %s - %s", cyborg.Name, op))
	}

	if cyborg.Status.ObservedGeneration == cyborg.Generation && cyborg.IsReady() {
		helper.GetLogger().Info("Cyborg ready condition is true")
		instance.Status.ContainerImages.CyborgAPIImage = version.Status.ContainerImages.CyborgAPIImage
		instance.Status.ContainerImages.CyborgAgentImage = version.Status.ContainerImages.CyborgAgentImage
		instance.Status.ContainerImages.CyborgConductorImage = version.Status.ContainerImages.CyborgConductorImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneCyborgReadyCondition, corev1beta1.OpenStackControlPlaneCyborgReadyMessage)
	} else {
		// We want to mirror the condition of the highest priority from the Cyborg resource into the instance
		// under the condition of type OpenStackControlPlaneCyborgReadyCondition, but only if the sub-resource
		// currently has any conditions (which won't be true for the initial creation of the sub-resource, since
		// it has not gone through a reconcile loop yet to have any conditions).  If this condition ends up being
		// the highest priority condition in the OpenStackControlPlane, it will appear in the OpenStackControlPlane's
		// "Ready" condition at the end of the reconciliation loop, clearly surfacing the condition to the user in
		// the "oc get oscontrolplane -n <namespace>" output.
		if len(cyborg.Status.Conditions) > 0 {
			MirrorSubResourceCondition(cyborg.Status.Conditions, corev1beta1.OpenStackControlPlaneCyborgReadyCondition, instance, cyborg.Kind)
		} else {
			// Default to the associated "running" condition message for the sub-resource if it currently lacks any conditions for mirroring
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneCyborgReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneCyborgReadyRunningMessage))
		}
	}

	return ctrl.Result{}, nil
}

// CyborgImageMatch - return true if the Cyborg images match on the ControlPlane and Version, or if Cyborg is not enabled
func CyborgImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.Cyborg.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.CyborgAPIImage, version.Status.ContainerImages.CyborgAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CyborgAgentImage, version.Status.ContainerImages.CyborgAgentImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CyborgConductorImage, version.Status.ContainerImages.CyborgConductorImage) {
			Log.Info("Cyborg images do not match")
			return false
		}
	}

	return true
}
