// Package openstack provides OpenStack service reconciliation and management functionality
package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileBarbican -
func ReconcileBarbican(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	barbican := &barbicanv1.Barbican{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "barbican",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Barbican.Enabled {
		if res, err := EnsureDeleted(ctx, helper, barbican); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneBarbicanReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeBarbicanReadyCondition)
		instance.Status.ContainerImages.BarbicanAPIImage = nil
		instance.Status.ContainerImages.BarbicanWorkerImage = nil
		instance.Status.ContainerImages.BarbicanKeystoneListenerImage = nil
		return ctrl.Result{}, nil
	}

	if instance.Spec.Barbican.Template == nil {
		instance.Spec.Barbican.Template = &barbicanv1.BarbicanSpecCore{}
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Barbican.Template.BarbicanAPI.Override.Service == nil {
			instance.Spec.Barbican.Template.BarbicanAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Barbican.Template.BarbicanAPI.Override.Service[endpointType] = AddServiceOpenStackOperatorLabel(
			instance.Spec.Barbican.Template.BarbicanAPI.Override.Service[endpointType],
			barbican.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "barbican", Namespace: instance.Namespace}, barbican); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs, set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Barbican.Template.BarbicanAPI.TLS = barbican.Spec.BarbicanAPI.TLS
	}
	instance.Spec.Barbican.Template.BarbicanAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(barbican.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Barbican.Template.BarbicanAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			barbican,
			svcs,
			instance.Spec.Barbican.Template.BarbicanAPI.Override.Service,
			instance.Spec.Barbican.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeBarbicanReadyCondition,
			false, // TODO: (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Barbican.Template.BarbicanAPI.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Barbican.Template.BarbicanAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Barbican.Template.BarbicanAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Barbican.Template.BarbicanAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	if instance.Spec.Barbican.Template.NodeSelector == nil {
		instance.Spec.Barbican.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Barbican.Template.TopologyRef == nil {
		instance.Spec.Barbican.Template.TopologyRef = instance.Spec.TopologyRef
	}

	helper.GetLogger().Info("Reconciling Barbican", "Barbican.Namespace", instance.Namespace, "Barbican.Name", "barbican")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), barbican, func() error {
		instance.Spec.Barbican.Template.BarbicanSpecBase.DeepCopyInto(&barbican.Spec.BarbicanSpecBase)
		instance.Spec.Barbican.Template.BarbicanAPI.DeepCopyInto(&barbican.Spec.BarbicanAPI.BarbicanAPITemplateCore)
		instance.Spec.Barbican.Template.BarbicanWorker.DeepCopyInto(&barbican.Spec.BarbicanWorker.BarbicanWorkerTemplateCore)
		instance.Spec.Barbican.Template.BarbicanKeystoneListener.DeepCopyInto(&barbican.Spec.BarbicanKeystoneListener.BarbicanKeystoneListenerTemplateCore)

		barbican.Spec.BarbicanAPI.ContainerImage = *version.Status.ContainerImages.BarbicanAPIImage
		barbican.Spec.BarbicanWorker.ContainerImage = *version.Status.ContainerImages.BarbicanWorkerImage
		barbican.Spec.BarbicanKeystoneListener.ContainerImage = *version.Status.ContainerImages.BarbicanKeystoneListenerImage

		// FIXME: barbican webhooks are not setting this correctly yet
		if barbican.Spec.DatabaseAccount == "" {
			barbican.Spec.DatabaseAccount = "barbican"
		}
		if barbican.Spec.Secret == "" {
			barbican.Spec.Secret = instance.Spec.Secret
		}
		if barbican.Spec.DatabaseInstance == "" {
			// barbican.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			barbican.Spec.DatabaseInstance = "openstack" // FIXME: see above
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), barbican, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneBarbicanReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneBarbicanReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("barbican %s - %s", barbican.Name, op))
	}

	if barbican.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneBarbicanReadyCondition, corev1beta1.OpenStackControlPlaneBarbicanReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneBarbicanReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneBarbicanReadyRunningMessage))
	}
	instance.Status.ContainerImages.BarbicanAPIImage = version.Status.ContainerImages.BarbicanAPIImage
	instance.Status.ContainerImages.BarbicanWorkerImage = version.Status.ContainerImages.BarbicanWorkerImage
	instance.Status.ContainerImages.BarbicanKeystoneListenerImage = version.Status.ContainerImages.BarbicanKeystoneListenerImage

	return ctrl.Result{}, nil
}

// BarbicanImageMatch - return true if the Barbican images match on the ControlPlane and Version, or if Barbican is not enabled
func BarbicanImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.Barbican.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.BarbicanAPIImage, version.Status.ContainerImages.BarbicanAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.BarbicanWorkerImage, version.Status.ContainerImages.BarbicanWorkerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.BarbicanKeystoneListenerImage, version.Status.ContainerImages.BarbicanKeystoneListenerImage) {
			Log.Info("Barbican images do not match")
			return false
		}
	}

	return true
}
