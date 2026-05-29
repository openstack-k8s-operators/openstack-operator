package openstack

import (
	"context"
	"fmt"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// InstanceHaConfigMap is the name of the ConfigMap used for instance HA configuration
	InstanceHaConfigMap = "infra-instanceha-config"
	// InstanceHaImageKey is the key used for the instance HA image in the ConfigMap
	InstanceHaImageKey = "instanceha-image"
)

// ReconcileInstanceHa reconciles the instance HA configuration for the OpenStack control plane
func ReconcileInstanceHa(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	Log := GetLogger(ctx)

	if instance.Spec.TLS.PodLevel.Enabled {
		_, err := EnsureInstanceHAMetricsCert(ctx, instance, helper)
		if err != nil {
			Log.Error(err, "Failed to ensure InstanceHA metrics certificate")
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneInstanceHaTLSReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1beta1.OpenStackControlPlaneInstanceHaTLSReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		instance.Status.Conditions.Set(condition.TrueCondition(
			corev1beta1.OpenStackControlPlaneInstanceHaTLSReadyCondition,
			corev1beta1.OpenStackControlPlaneInstanceHaTLSReadyMessage,
		))
	}

	customData := map[string]string{
		InstanceHaImageKey: *getImg(version.Status.ContainerImages.OpenstackClientImage, &missingImageDefault),
	}

	cms := []util.Template{
		{
			Name:          InstanceHaConfigMap,
			Namespace:     instance.Namespace,
			InstanceType:  instance.Kind,
			Labels:        nil,
			ConfigOptions: nil,
			CustomData:    customData,
		},
	}

	if err := configmap.EnsureConfigMaps(ctx, helper, instance, cms, nil); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneInstanceHaCMReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneInstanceHaCMReadyErrorMessage,
			err.Error()))

		return ctrl.Result{}, err
	}

	instance.Status.Conditions.Set(condition.TrueCondition(
		corev1beta1.OpenStackControlPlaneInstanceHaCMReadyCondition,
		corev1beta1.OpenStackControlPlaneInstanceHaCMReadyMessage,
	))

	return ctrl.Result{}, nil
}

// EnsureInstanceHAMetricsCert creates a TLS certificate for InstanceHA metrics services
func EnsureInstanceHAMetricsCert(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (string, error) {
	Log := GetLogger(ctx)

	dnsSuffix := clusterdns.GetDNSClusterDomain()

	certRequest := certmanager.CertificateRequest{
		IssuerName: instance.GetInternalIssuer(),
		CertName:   "instanceha-metrics",
		Hostnames: []string{
			fmt.Sprintf("*.%s.svc", instance.Namespace),
			fmt.Sprintf("*.%s.svc.%s", instance.Namespace, dnsSuffix),
		},
		Ips: nil,
		Usages: []certmgrv1.KeyUsage{
			certmgrv1.UsageKeyEncipherment,
			certmgrv1.UsageDigitalSignature,
			certmgrv1.UsageServerAuth,
		},
		Labels: map[string]string{ServiceCertSelector: ""},
	}

	if instance.Spec.TLS.PodLevel.Internal.Cert.Duration != nil {
		certRequest.Duration = &instance.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration
	}
	if instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore != nil {
		certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore.Duration
	}

	certSecret, ctrlResult, err := certmanager.EnsureCert(
		ctx,
		helper,
		certRequest,
		nil)
	if err != nil {
		return "", err
	} else if (ctrlResult != ctrl.Result{}) {
		Log.Info("InstanceHA metrics certificate creation in progress", "certificate", certRequest.CertName)
		return "", fmt.Errorf("InstanceHA metrics certificate creation in progress")
	}

	Log.Info("InstanceHA metrics certificate ensured", "secret", certSecret.Name, "certificate", certRequest.CertName)
	return certSecret.Name, nil
}
