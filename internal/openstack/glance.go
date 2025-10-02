package openstack

import (
	"context"
	"fmt"

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/common/webhook"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// svcSelector is used as selector to get the list of "Services" associated
	// to a specific glanceAPI instance. It must be different from glanceAPI
	// label set by the service operator as in case of split glanceAPI type the
	// the label on public svc gets set to -external and internal instance svc
	// to -internal instead of the glance top level glanceType split
	svcSelector = "tlGlanceAPI"
)

// ReconcileGlance -
func ReconcileGlance(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	Log := GetLogger(ctx)

	// Trigger webhook to cache service name if UniquePodNames is enabled and not yet cached
	// This handles operator upgrade scenario where existing CRs don't have ServiceName set
	if instance.Spec.Glance.UniquePodNames && instance.Spec.Glance.ServiceName == "" {
		return webhook.EnsureWebhookTrigger(ctx, instance, corev1beta1.ReconcileTriggerAnnotation, "Glance service name caching", Log, 0)
	}

	glanceName, altGlanceName := instance.GetServiceNameCached(corev1beta1.GlanceName, instance.Spec.Glance.UniquePodNames, instance.Spec.Glance.ServiceName)
	// Ensure the alternate glance CR doesn't exist, as the randomPodNames flag may have been toggled
	glance := &glancev1.Glance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      altGlanceName,
			Namespace: instance.Namespace,
		},
	}
	if res, err := EnsureDeleted(ctx, helper, glance); err != nil {
		return res, err
	}

	glance = &glancev1.Glance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      glanceName,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Glance.Enabled {
		if res, err := EnsureDeleted(ctx, helper, glance); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneGlanceReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeGlanceReadyCondition)
		instance.Status.ContainerImages.GlanceAPIImage = nil
		return ctrl.Result{}, nil
	}

	if instance.Spec.Glance.Template == nil {
		instance.Spec.Glance.Template = &glancev1.GlanceSpecCore{}
	}

	if instance.Spec.Glance.Template.NodeSelector == nil {
		instance.Spec.Glance.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Glance.Template.TopologyRef == nil {
		instance.Spec.Glance.Template.TopologyRef = instance.Spec.TopologyRef
	}

	// When no NotificationsBusInstance is referenced in the subCR (override)
	// try to inject the top-level one if defined
	if instance.Spec.Glance.Template.NotificationBusInstance == nil {
		instance.Spec.Glance.Template.NotificationBusInstance = instance.Spec.NotificationsBusInstance
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: glanceName, Namespace: instance.Namespace}, glance); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Application Credential Management (Day-2 operation)
	// Check if AC should be enabled and manage it accordingly
	glanceReady := glance.Status.ObservedGeneration == glance.Generation && glance.IsReady()

	// Apply same fallback logic as in CreateOrPatch to avoid passing empty values to AC
	glanceSecret := instance.Spec.Glance.Template.Secret
	if glanceSecret == "" {
		glanceSecret = instance.Spec.Secret
	}

	// Check if any GlanceAPI has AC configured
	hasACConfigured := false
	for _, glanceAPI := range instance.Spec.Glance.Template.GlanceAPIs {
		if glanceAPI.Auth.ApplicationCredentialSecret != "" {
			hasACConfigured = true
			break
		}
	}

	// Only call if AC enabled or currently configured
	if isACEnabled(instance.Spec.ApplicationCredential, instance.Spec.Glance.ApplicationCredential) || hasACConfigured {

		acSecretName, acResult, err := EnsureApplicationCredentialForService(
			ctx,
			helper,
			instance,
			glance.Name,
			glanceReady,
			glanceSecret,
			instance.Spec.Glance.Template.PasswordSelectors.Service,
			instance.Spec.Glance.Template.ServiceUser,
			instance.Spec.Glance.ApplicationCredential,
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		// If AC is not ready, return immediately without updating the service CR
		if (acResult != ctrl.Result{}) {
			return acResult, nil
		}

		// Set ApplicationCredentialSecret for all GlanceAPIs based on what the helper returned:
		// - If AC disabled: returns ""
		// - If AC enabled and ready: returns the AC secret name
		for name, glanceAPI := range instance.Spec.Glance.Template.GlanceAPIs {
			glanceAPI.Auth.ApplicationCredentialSecret = acSecretName
			instance.Spec.Glance.Template.GlanceAPIs[name] = glanceAPI
		}
	}

	// add selector to service overrides
	for name, glanceAPI := range instance.Spec.Glance.Template.GlanceAPIs {
		eps := []service.Endpoint{service.EndpointPublic, service.EndpointInternal}
		// An Edge glanceAPI has an internal endpoint only
		if glanceAPI.Type == glancev1.APIEdge {
			eps = []service.Endpoint{service.EndpointInternal}
		}
		for _, endpointType := range eps {
			if glanceAPI.Override.Service == nil {
				glanceAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
			}
			glanceAPI.Override.Service[endpointType] = AddServiceOpenStackOperatorLabel(
				glanceAPI.Override.Service[endpointType], glance.Name)

			svcOverride := glanceAPI.Override.Service[endpointType]
			svcOverride.AddLabel(getGlanceAPILabelMap(glance.Name, name, glanceAPI.Type))
			glanceAPI.Override.Service[endpointType] = svcOverride
		}

		// preserve any previously set TLS certs,set CA cert
		if instance.Spec.TLS.PodLevel.Enabled {
			glanceAPI.TLS.API = glance.Spec.GlanceAPIs[name].TLS.API
		}
		glanceAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
		instance.Spec.Glance.Template.GlanceAPIs[name] = glanceAPI
	}

	var changed = false
	for name, glanceAPI := range instance.Spec.Glance.Template.GlanceAPIs {
		// Retrieve the services by Label and filter on glanceAPI: for
		// each instance we should get **only** the associated `SVCs`
		// and not the whole list. As per the Glance design doc we know
		// that a given instance name is made in the form: "<service>
		// <apiName> <apiType>", so we build the filter accordingly
		// to resolve the label as <service>-<apiName>
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			util.MergeMaps(
				GetServiceOpenStackOperatorLabel(glance.Name),
				getGlanceAPILabelMap(glance.Name, name, glanceAPI.Type),
			),
		)
		if err != nil {
			return ctrl.Result{}, err
		}
		// make sure to get to EndpointConfig when all service got created
		// Webhook initializes APIOverride and always has at least the timeout override
		if len(svcs.Items) == len(glanceAPI.Override.Service) {
			endpointDetails, ctrlResult, err := EnsureEndpointConfig(
				ctx,
				instance,
				helper,
				glance,
				svcs,
				glanceAPI.Override.Service,
				instance.Spec.Glance.APIOverride[name],
				corev1beta1.OpenStackControlPlaneExposeGlanceReadyCondition,
				false, // TODO (mschuppert) could be removed when all integrated service support TLS
				glanceAPI.TLS,
			)
			if err != nil {
				return ctrlResult, err
			}
			// set service overrides
			glanceAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
			// update TLS cert secret, but skip Public endpoint for Edge
			// instances
			if glanceAPI.Type != glancev1.APIEdge {
				glanceAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
			}
			glanceAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)

			// let's keep track of changes for any instance, but return
			// only when the iteration on the whole APIList is over
			if (ctrlResult != ctrl.Result{}) {
				changed = true
			}
		}
		instance.Spec.Glance.Template.GlanceAPIs[name] = glanceAPI
	}
	if changed {
		return ctrl.Result{}, nil
	}

	Log.Info("Reconciling Glance", "Glance.Namespace", instance.Namespace, "Glance.Name", glanceName)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), glance, func() error {
		instance.Spec.Glance.Template.DeepCopyInto(&glance.Spec.GlanceSpecCore)
		glance.Spec.ContainerImage = *version.Status.ContainerImages.GlanceAPIImage
		if glance.Spec.Secret == "" {
			glance.Spec.Secret = instance.Spec.Secret
		}
		if glance.Spec.DatabaseInstance == "" {
			glance.Spec.DatabaseInstance = "openstack"
		}
		if glance.Spec.Storage.StorageClass == "" {
			glance.Spec.Storage.StorageClass = instance.Spec.StorageClass
		}

		if glance.GetAnnotations() == nil {
			glance.SetAnnotations(make(map[string]string))
		}
		if version.Status.ServiceDefaults.GlanceWsgi != nil && *version.Status.ServiceDefaults.GlanceWsgi == "true" {
			glance.GetAnnotations()[glancev1.GlanceWSGILabel] = "true"
		} else {
			glance.GetAnnotations()[glancev1.GlanceWSGILabel] = "false"
		}

		// Append globally defined extraMounts to the service's own list.
		for _, ev := range instance.Spec.ExtraMounts {
			glance.Spec.ExtraMounts = append(glance.Spec.ExtraMounts, glancev1.GlanceExtraVolMounts{
				Name:      ev.Name,
				Region:    ev.Region,
				VolMounts: ev.VolMounts,
			})
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), glance, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneGlanceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneGlanceReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("glance %s - %s", glance.Name, op))
	}

	if glance.Status.ObservedGeneration == glance.Generation && glance.IsReady() {
		Log.Info("Glance ready condition is true")
		instance.Status.ContainerImages.GlanceAPIImage = version.Status.ContainerImages.GlanceAPIImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneGlanceReadyCondition, corev1beta1.OpenStackControlPlaneGlanceReadyMessage)
	} else {
		// We want to mirror the condition of the highest priority from the Glance resource into the instance
		// under the condition of type OpenStackControlPlaneGlanceReadyCondition, but only if the sub-resource
		// currently has any conditions (which won't be true for the initial creation of the sub-resource, since
		// it has not gone through a reconcile loop yet to have any conditions).  If this condition ends up being
		// the highest priority condition in the OpenStackControlPlane, it will appear in the OpenStackControlPlane's
		// "Ready" condition at the end of the reconciliation loop, clearly surfacing the condition to the user in
		// the "oc get oscontrolplane -n <namespace>" output.
		if len(glance.Status.Conditions) > 0 {
			MirrorSubResourceCondition(glance.Status.Conditions, corev1beta1.OpenStackControlPlaneGlanceReadyCondition, instance, glance.Kind)
		} else {
			// Default to the associated "running" condition message for the sub-resource if it currently lacks any conditions for mirroring
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneGlanceReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneGlanceReadyRunningMessage))
		}
	}

	return ctrl.Result{}, nil
}

func getGlanceAPILabelMap(name string, apiName string, apiType string) map[string]string {
	apiFilter := fmt.Sprintf("%s-%s-%s", name, apiName, apiType)

	return map[string]string{
		svcSelector: apiFilter,
	}
}

// GlanceImageMatch - return true if the glance images match on the ControlPlane and Version, or if Glance is not enabled
func GlanceImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Glance.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.GlanceAPIImage, version.Status.ContainerImages.GlanceAPIImage) {
			Log.Info("Glance API image mismatch", "controlPlane.Status.ContainerImages.GlanceAPIImage", controlPlane.Status.ContainerImages.GlanceAPIImage, "version.Status.ContainerImages.GlanceAPIImage", version.Status.ContainerImages.GlanceAPIImage)
			return false
		}
	}

	return true
}
