package openstack

import (
	"context"
	"fmt"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

var count int

// ReconcilePlacementAPI -
func ReconcilePlacementAPI(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	placementAPI := &placementv1.PlacementAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "placement",
			Namespace: instance.Namespace,
		},
	}
	Log := GetLogger(ctx)

	if !instance.Spec.Placement.Enabled {
		if res, err := EnsureDeleted(ctx, helper, placementAPI); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposePlacementAPIReadyCondition)
		return ctrl.Result{}, nil
	}

	if instance.Spec.Placement.Template == nil {
		instance.Spec.Placement.Template = &placementv1.PlacementAPISpecCore{}
	}

	if instance.Spec.Placement.Template.NodeSelector == nil {
		instance.Spec.Placement.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Placement.Template.TopologyRef == nil {
		instance.Spec.Placement.Template.TopologyRef = instance.Spec.TopologyRef
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Placement.Template.Override.Service == nil {
			instance.Spec.Placement.Template.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Placement.Template.Override.Service[endpointType] = AddServiceOpenStackOperatorLabel(
			instance.Spec.Placement.Template.Override.Service[endpointType],
			placementAPI.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "placement", Namespace: instance.Namespace}, placementAPI); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// set CA cert and preserve any previously set TLS certs
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Placement.Template.TLS = placementAPI.Spec.TLS
	}
	instance.Spec.Placement.Template.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(placementAPI.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Placement.Template.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			placementAPI,
			svcs,
			instance.Spec.Placement.Template.Override.Service,
			instance.Spec.Placement.APIOverride,
			corev1beta1.OpenStackControlPlaneExposePlacementAPIReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Placement.Template.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		if count != 10 {
			Log.Info("XXX delaying placement endpoint update")
			time.Sleep(10 * time.Second)
			count++
			return ctrl.Result{Requeue: true}, nil
		}
		Log.Info("XXX continues with placement endpoint update")
		// set service overrides
		instance.Spec.Placement.Template.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Placement.Template.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Placement.Template.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	Log.Info("Reconciling PlacementAPI", "PlacementAPI.Namespace", instance.Namespace, "PlacementAPI.Name", "placement")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), placementAPI, func() error {
		instance.Spec.Placement.Template.DeepCopyInto(&placementAPI.Spec.PlacementAPISpecCore)

		placementAPI.Spec.ContainerImage = *version.Status.ContainerImages.PlacementAPIImage
		if placementAPI.Spec.Secret == "" {
			placementAPI.Spec.Secret = instance.Spec.Secret
		}
		if placementAPI.Spec.DatabaseInstance == "" {
			placementAPI.Spec.DatabaseInstance = "openstack"
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), placementAPI, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlanePlacementAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("placementAPI %s - %s", placementAPI.Name, op))
	}

	if placementAPI.Status.ObservedGeneration == placementAPI.Generation && placementAPI.IsReady() {
		instance.Status.ContainerImages.PlacementAPIImage = version.Status.ContainerImages.PlacementAPIImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition, corev1beta1.OpenStackControlPlanePlacementAPIReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlanePlacementAPIReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlanePlacementAPIReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// PlacementImageMatch - return true if the placement images match on the ControlPlane and Version, or if Placement is not enabled
func PlacementImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Placement.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.PlacementAPIImage, version.Status.ContainerImages.PlacementAPIImage) {
			Log.Info("Placement API image mismatch", "controlPlane.Status.ContainerImages.PlacementAPIImage", controlPlane.Status.ContainerImages.PlacementAPIImage, "version.Status.ContainerImages.PlacementAPIImage", version.Status.ContainerImages.PlacementAPIImage)
			return false
		}
	}

	return true
}
