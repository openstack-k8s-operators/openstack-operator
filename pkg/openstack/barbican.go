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
func ReconcileBarbican(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
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
		return ctrl.Result{}, nil
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

	helper.GetLogger().Info("Reconciling Barbican", "Barbican.Namespace", instance.Namespace, "Barbican.Name", "barbican")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), barbican, func() error {
		instance.Spec.Barbican.Template.DeepCopyInto(&barbican.Spec)

		if barbican.Spec.Secret == "" {
			barbican.Spec.Secret = instance.Spec.Secret
		}
		if barbican.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			barbican.Spec.NodeSelector = instance.Spec.NodeSelector
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

	return ctrl.Result{}, nil
}
