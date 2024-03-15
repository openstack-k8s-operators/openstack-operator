package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	telemetryNamespaceLabel = "Telemetry.Namespace"
	telemetryNameLabel      = "Telemetry.Name"
	telemetryName           = "telemetry"
)

// ReconcileTelemetry puts telemetry resources to required state
func ReconcileTelemetry(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	telemetry := &telemetryv1.Telemetry{
		ObjectMeta: metav1.ObjectMeta{
			Name:      telemetryName,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Telemetry.Enabled {
		if res, err := EnsureDeleted(ctx, helper, telemetry); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneTelemetryReadyCondition)
		return ctrl.Result{}, nil
	}

	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "telemetry", Namespace: instance.Namespace}, telemetry); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service == nil {
			instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service = make(map[service.Endpoint]service.RoutedOverrideSpec)
		}
		instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service[endpointType],
				telemetry.Name)
	}

	// preserve any previously set TLS certs, set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Telemetry.Template.Autoscaling.Aodh.TLS = telemetry.Spec.Autoscaling.Aodh.TLS
	}
	instance.Spec.Telemetry.Template.Autoscaling.Aodh.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(telemetry.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			telemetry,
			svcs,
			instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service,
			instance.Spec.Telemetry.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeTelemetryReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Telemetry.Template.Autoscaling.Aodh.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Telemetry.Template.Autoscaling.Aodh.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Telemetry.Template.Autoscaling.Aodh.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	helper.GetLogger().Info("Reconciling Telemetry", telemetryNamespaceLabel, instance.Namespace, telemetryNameLabel, telemetryName)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), telemetry, func() error {
		instance.Spec.Telemetry.Template.DeepCopyInto(&telemetry.Spec)

		if telemetry.Spec.Ceilometer.Secret == "" {
			telemetry.Spec.Ceilometer.Secret = instance.Spec.Secret
		}

		if telemetry.Spec.Autoscaling.Aodh.DatabaseInstance == "" {
			// TODO(mmagr): Fix once this is not hardcoded in rest of the operator
			telemetry.Spec.Autoscaling.Aodh.DatabaseInstance = "openstack"
		}
		if telemetry.Spec.Autoscaling.Aodh.Secret == "" {
			telemetry.Spec.Autoscaling.Aodh.Secret = instance.Spec.Secret
		}
		if telemetry.Spec.Autoscaling.HeatInstance == "" {
			telemetry.Spec.Autoscaling.HeatInstance = heatName
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), telemetry, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneTelemetryReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneTelemetryReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("%s %s - %s", telemetryName, telemetry.Name, op))
	}

	if telemetry.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneTelemetryReadyCondition, corev1beta1.OpenStackControlPlaneTelemetryReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneTelemetryReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneTelemetryReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
