package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	telemetryNamespaceLabel = "Telemetry.Namespace"
	telemetryNameLabel      = "Telemetry.Name"
	telemetryName           = "telemetry"
)

// ReconcileTelemetry puts telemetry resources to required state
func ReconcileTelemetry(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
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
		instance.Status.ContainerImages.CeilometerCentralImage = nil
		instance.Status.ContainerImages.CeilometerComputeImage = nil
		instance.Status.ContainerImages.CeilometerIpmiImage = nil
		instance.Status.ContainerImages.CeilometerNotificationImage = nil
		instance.Status.ContainerImages.CeilometerSgcoreImage = nil
		instance.Status.ContainerImages.CeilometerProxyImage = nil
		instance.Status.ContainerImages.CeilometerMysqldExporterImage = nil
		instance.Status.ContainerImages.AodhAPIImage = nil
		instance.Status.ContainerImages.AodhEvaluatorImage = nil
		instance.Status.ContainerImages.AodhNotifierImage = nil
		instance.Status.ContainerImages.AodhListenerImage = nil
		return ctrl.Result{}, nil
	}

	if instance.Spec.Telemetry.Template == nil {
		instance.Spec.Telemetry.Template = &telemetryv1.TelemetrySpecCore{}
	}

	if instance.Spec.Telemetry.Template.NodeSelector == nil {
		instance.Spec.Telemetry.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Telemetry.Template.TopologyRef == nil {
		instance.Spec.Telemetry.Template.TopologyRef = instance.Spec.TopologyRef
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
		instance.Spec.Telemetry.Template.MetricStorage.PrometheusTLS = telemetry.Spec.MetricStorage.PrometheusTLS
		instance.Spec.Telemetry.Template.Ceilometer.TLS = telemetry.Spec.Ceilometer.TLS
		instance.Spec.Telemetry.Template.Ceilometer.MysqldExporterTLS = telemetry.Spec.Ceilometer.MysqldExporterTLS
		instance.Spec.Telemetry.Template.Ceilometer.KSMTLS = telemetry.Spec.Ceilometer.KSMTLS
	}
	instance.Spec.Telemetry.Template.Autoscaling.Aodh.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
	instance.Spec.Telemetry.Template.Ceilometer.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
	instance.Spec.Telemetry.Template.Ceilometer.MysqldExporterTLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
	instance.Spec.Telemetry.Template.Ceilometer.KSMTLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
	instance.Spec.Telemetry.Template.MetricStorage.PrometheusTLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	aodhSvcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(telemetry.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	prometheusSvcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		map[string]string{"app.kubernetes.io/name": fmt.Sprintf("%s-prometheus", telemetryv1.DefaultServiceName)},
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	alertmanagerSvcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		map[string]string{"app.kubernetes.io/name": fmt.Sprintf("%s-alertmanager", telemetryv1.DefaultServiceName)},
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	ceilometerSvcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		map[string]string{common.AppSelector: "ceilometer"},
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	ksmSvcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		map[string]string{common.AppSelector: "kube-state-metrics"},
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	mysqldExporterSvcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		map[string]string{common.AppSelector: "mysqld-exporter"},
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(aodhSvcs.Items) == len(instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			telemetry,
			aodhSvcs,
			instance.Spec.Telemetry.Template.Autoscaling.Aodh.Override.Service,
			instance.Spec.Telemetry.AodhAPIOverride,
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

	if telemetry.Status.Conditions.IsTrue(telemetryv1.MetricStorageReadyCondition) {
		// EnsureEndpoint for prometheus
		// NOTE: We don't manage the prometheus service, it's managed by COO, we just annotate it
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			telemetry,
			prometheusSvcs,
			nil,
			instance.Spec.Telemetry.PrometheusOverride,
			corev1beta1.OpenStackControlPlaneExposeTelemetryReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			tls.API{
				API: tls.APIService{
					Public: tls.GenericService{
						SecretName: instance.Spec.Telemetry.Template.MetricStorage.PrometheusTLS.SecretName,
					},
				},
			},
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// update TLS settings with cert secret
		instance.Spec.Telemetry.Template.MetricStorage.PrometheusTLS.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)

		// TODO: rewrite this once we have TLS on alertmanager
		for _, alertmanagerSvc := range alertmanagerSvcs.Items {
			ed := EndpointDetail{
				Name:      alertmanagerSvc.Name,
				Namespace: alertmanagerSvc.Namespace,
				Type:      service.Endpoint(alertmanagerSvc.Annotations[service.AnnotationEndpointKey]),
				Service: ServiceDetails{
					Spec: &alertmanagerSvc,
				},
			}
			ed.Route.Create = alertmanagerSvc.Annotations[service.AnnotationIngressCreateKey] == "true"
			ed.Route.TLS.Enabled = false
			if instance.Spec.Telemetry.AlertmanagerOverride.Route != nil {
				ed.Route.OverrideSpec = *instance.Spec.Telemetry.AlertmanagerOverride.Route
			}
			ctrlResult, err := ed.ensureRoute(
				ctx,
				instance,
				helper,
				&alertmanagerSvc,
				telemetry,
				corev1beta1.OpenStackControlPlaneExposeTelemetryReadyCondition,
			)
			if err != nil {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}
		}
	}

	if telemetry.Status.Conditions.IsTrue(telemetryv1.CeilometerReadyCondition) {
		// NOTE: We don't have svc overrides for ceilometer objects.
		// Ceilometer
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			telemetry,
			ceilometerSvcs,
			nil,
			corev1beta1.Override{},
			corev1beta1.OpenStackControlPlaneExposeTelemetryReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			tls.API{},
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// update TLS settings with cert secret
		instance.Spec.Telemetry.Template.Ceilometer.TLS.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)

		// MysqldExporter
		if telemetry.Spec.Ceilometer.MysqldExporterEnabled != nil && *telemetry.Spec.Ceilometer.MysqldExporterEnabled {
			endpointDetails, ctrlResult, err := EnsureEndpointConfig(
				ctx,
				instance,
				helper,
				telemetry,
				mysqldExporterSvcs,
				nil,
				corev1beta1.Override{},
				corev1beta1.OpenStackControlPlaneExposeTelemetryReadyCondition,
				false, // TODO (mschuppert) could be removed when all integrated service support TLS
				tls.API{},
			)
			if err != nil {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}
			// update TLS settings with cert secret
			instance.Spec.Telemetry.Template.Ceilometer.MysqldExporterTLS.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
		}

		// NOTE: We don't have svc overrides for KSM objects too.
		ksmEpDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			telemetry,
			ksmSvcs,
			nil,
			corev1beta1.Override{},
			corev1beta1.OpenStackControlPlaneExposeTelemetryReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			tls.API{},
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// update TLS settings with cert secret
		instance.Spec.Telemetry.Template.Ceilometer.KSMTLS.SecretName = ksmEpDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	helper.GetLogger().Info("Reconciling Telemetry", telemetryNamespaceLabel, instance.Namespace, telemetryNameLabel, telemetryName)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), telemetry, func() error {
		instance.Spec.Telemetry.Template.TelemetrySpecBase.DeepCopyInto(&telemetry.Spec.TelemetrySpecBase)
		instance.Spec.Telemetry.Template.Autoscaling.AutoscalingSpecBase.DeepCopyInto(&telemetry.Spec.Autoscaling.AutoscalingSpecBase)
		instance.Spec.Telemetry.Template.Autoscaling.Aodh.DeepCopyInto(&telemetry.Spec.Autoscaling.Aodh.AodhCore)
		instance.Spec.Telemetry.Template.Ceilometer.CeilometerSpecCore.DeepCopyInto(&telemetry.Spec.Ceilometer.CeilometerSpecCore)
		instance.Spec.Telemetry.Template.Logging.DeepCopyInto(&telemetry.Spec.Logging)
		instance.Spec.Telemetry.Template.MetricStorage.DeepCopyInto(&telemetry.Spec.MetricStorage)

		telemetry.Spec.Ceilometer.Enabled = ptr.To(*instance.Spec.Telemetry.Template.Ceilometer.Enabled)
		telemetry.Spec.Autoscaling.Enabled = ptr.To(*instance.Spec.Telemetry.Template.Autoscaling.Enabled)

		telemetry.Spec.Ceilometer.CentralImage = *version.Status.ContainerImages.CeilometerCentralImage
		telemetry.Spec.Ceilometer.ComputeImage = *version.Status.ContainerImages.CeilometerComputeImage
		telemetry.Spec.Ceilometer.IpmiImage = *version.Status.ContainerImages.CeilometerIpmiImage
		telemetry.Spec.Ceilometer.NotificationImage = *version.Status.ContainerImages.CeilometerNotificationImage
		telemetry.Spec.Ceilometer.SgCoreImage = *version.Status.ContainerImages.CeilometerSgcoreImage
		telemetry.Spec.Ceilometer.ProxyImage = *version.Status.ContainerImages.CeilometerProxyImage
		telemetry.Spec.Autoscaling.Aodh.APIImage = *version.Status.ContainerImages.AodhAPIImage
		telemetry.Spec.Autoscaling.Aodh.EvaluatorImage = *version.Status.ContainerImages.AodhEvaluatorImage
		telemetry.Spec.Autoscaling.Aodh.NotifierImage = *version.Status.ContainerImages.AodhNotifierImage
		telemetry.Spec.Autoscaling.Aodh.ListenerImage = *version.Status.ContainerImages.AodhListenerImage

		telemetry.Spec.Ceilometer.KSMImage = *getImg(version.Status.ContainerImages.KsmImage, &missingImageDefault)
		telemetry.Spec.Ceilometer.MysqldExporterImage = *getImg(version.Status.ContainerImages.CeilometerMysqldExporterImage, &missingImageDefault)

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

	if telemetry.Status.ObservedGeneration == telemetry.Generation && telemetry.IsReady() {
		instance.Status.ContainerImages.CeilometerCentralImage = version.Status.ContainerImages.CeilometerCentralImage
		instance.Status.ContainerImages.CeilometerComputeImage = version.Status.ContainerImages.CeilometerComputeImage
		instance.Status.ContainerImages.CeilometerIpmiImage = version.Status.ContainerImages.CeilometerIpmiImage
		instance.Status.ContainerImages.CeilometerNotificationImage = version.Status.ContainerImages.CeilometerNotificationImage
		instance.Status.ContainerImages.CeilometerSgcoreImage = version.Status.ContainerImages.CeilometerSgcoreImage
		instance.Status.ContainerImages.CeilometerProxyImage = version.Status.ContainerImages.CeilometerProxyImage
		instance.Status.ContainerImages.CeilometerMysqldExporterImage = version.Status.ContainerImages.CeilometerMysqldExporterImage
		instance.Status.ContainerImages.KsmImage = version.Status.ContainerImages.KsmImage
		instance.Status.ContainerImages.AodhAPIImage = version.Status.ContainerImages.AodhAPIImage
		instance.Status.ContainerImages.AodhEvaluatorImage = version.Status.ContainerImages.AodhEvaluatorImage
		instance.Status.ContainerImages.AodhNotifierImage = version.Status.ContainerImages.AodhNotifierImage
		instance.Status.ContainerImages.AodhListenerImage = version.Status.ContainerImages.AodhListenerImage
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

// TelemetryImageMatch - return true if the telemetry images match on the ControlPlane and Version, or if Telemetry is not enabled
func TelemetryImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Telemetry.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.CeilometerCentralImage, version.Status.ContainerImages.CeilometerCentralImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CeilometerComputeImage, version.Status.ContainerImages.CeilometerComputeImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CeilometerIpmiImage, version.Status.ContainerImages.CeilometerIpmiImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CeilometerNotificationImage, version.Status.ContainerImages.CeilometerNotificationImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CeilometerSgcoreImage, version.Status.ContainerImages.CeilometerSgcoreImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CeilometerProxyImage, version.Status.ContainerImages.CeilometerProxyImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CeilometerMysqldExporterImage, version.Status.ContainerImages.CeilometerMysqldExporterImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.AodhAPIImage, version.Status.ContainerImages.AodhAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.AodhEvaluatorImage, version.Status.ContainerImages.AodhEvaluatorImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.AodhNotifierImage, version.Status.ContainerImages.AodhNotifierImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.AodhListenerImage, version.Status.ContainerImages.AodhListenerImage) {
			Log.Info("Telemetry images do not match")
			return false
		}
	}

	return true
}
