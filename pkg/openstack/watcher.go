package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	watcherv1 "github.com/openstack-k8s-operators/watcher-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileWatcher -
func ReconcileWatcher(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	watcher := &watcherv1.Watcher{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "watcher",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Watcher.Enabled {
		if res, err := EnsureDeleted(ctx, helper, watcher); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneWatcherReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeWatcherReadyCondition)
		instance.Status.ContainerImages.WatcherAPIImage = nil
		instance.Status.ContainerImages.WatcherApplierImage = nil
		instance.Status.ContainerImages.WatcherDecisionEngineImage = nil
		return ctrl.Result{}, nil
	}

	if instance.Spec.Watcher.Template == nil {
		instance.Spec.Watcher.Template = &watcherv1.WatcherSpecCore{}
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Watcher.Template.APIServiceTemplate.Override.Service == nil {
			instance.Spec.Watcher.Template.APIServiceTemplate.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Watcher.Template.APIServiceTemplate.Override.Service[endpointType] = AddServiceOpenStackOperatorLabel(
			instance.Spec.Watcher.Template.APIServiceTemplate.Override.Service[endpointType],
			watcher.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "watcher", Namespace: instance.Namespace}, watcher); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs, set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Watcher.Template.APIServiceTemplate.TLS = watcher.Spec.APIServiceTemplate.TLS
	}
	instance.Spec.Watcher.Template.APIServiceTemplate.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(watcher.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Watcher.Template.APIServiceTemplate.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			watcher,
			svcs,
			instance.Spec.Watcher.Template.APIServiceTemplate.Override.Service,
			instance.Spec.Watcher.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeWatcherReadyCondition,
			false, // TODO: (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Watcher.Template.APIServiceTemplate.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Watcher.Template.APIServiceTemplate.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Watcher.Template.APIServiceTemplate.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Watcher.Template.APIServiceTemplate.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	if instance.Spec.Watcher.Template.NodeSelector == nil {
		instance.Spec.Watcher.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Watcher.Template.TopologyRef == nil {
		instance.Spec.Watcher.Template.TopologyRef = instance.Spec.TopologyRef
	}

	helper.GetLogger().Info("Reconciling Watcher", "Watcher.Namespace", instance.Namespace, "Watcher.Name", "watcher")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), watcher, func() error {
		instance.Spec.Watcher.Template.DeepCopyInto(&watcher.Spec.WatcherSpecCore)

		if version.Status.ContainerImages.WatcherAPIImage == nil ||
			version.Status.ContainerImages.WatcherApplierImage == nil ||
			version.Status.ContainerImages.WatcherDecisionEngineImage == nil {
			return fmt.Errorf("no Watcher images found in the OpenStackVersion")
		}
		watcher.Spec.APIContainerImageURL = *version.Status.ContainerImages.WatcherAPIImage
		watcher.Spec.ApplierContainerImageURL = *version.Status.ContainerImages.WatcherApplierImage
		watcher.Spec.DecisionEngineContainerImageURL = *version.Status.ContainerImages.WatcherDecisionEngineImage

		if *watcher.Spec.Secret == "" {
			*watcher.Spec.Secret = instance.Spec.Secret
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), watcher, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneWatcherReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneWatcherReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("watcher %s - %s", watcher.Name, op))
	}

	if watcher.Status.ObservedGeneration == watcher.Generation && watcher.IsReady() {
		instance.Status.ContainerImages.WatcherAPIImage = version.Status.ContainerImages.WatcherAPIImage
		instance.Status.ContainerImages.WatcherApplierImage = version.Status.ContainerImages.WatcherApplierImage
		instance.Status.ContainerImages.WatcherDecisionEngineImage = version.Status.ContainerImages.WatcherApplierImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneWatcherReadyCondition, corev1beta1.OpenStackControlPlaneWatcherReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneWatcherReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneWatcherReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}

// WatcherImageMatch - return true if the Watcher images match on the ControlPlane and Version, or if Watcher is not enabled
func WatcherImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.Watcher.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.WatcherAPIImage, version.Status.ContainerImages.WatcherAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.WatcherApplierImage, version.Status.ContainerImages.WatcherApplierImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.WatcherDecisionEngineImage, version.Status.ContainerImages.WatcherDecisionEngineImage) {
			Log.Info("Watcher images do not match")
			return false
		}
	}

	return true
}
