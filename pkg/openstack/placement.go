package openstack

import (
	"context"
	"fmt"

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

// ReconcilePlacementAPI -
func ReconcilePlacementAPI(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
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
		// set service overrides
		instance.Spec.Placement.Template.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Placement.Template.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Placement.Template.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	Log.Info("Reconciling PlacementAPI", "PlacementAPI.Namespace", instance.Namespace, "PlacementAPI.Name", "placement")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), placementAPI, func() error {
		instance.Spec.Placement.Template.DeepCopyInto(&placementAPI.Spec)

		if placementAPI.Spec.Secret == "" {
			placementAPI.Spec.Secret = instance.Spec.Secret
		}
		if placementAPI.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			placementAPI.Spec.NodeSelector = instance.Spec.NodeSelector
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

	if placementAPI.IsReady() {
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
