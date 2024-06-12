package openstack

import (
	"context"
	"fmt"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileNeutron -
func ReconcileNeutron(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	neutronAPI := &neutronv1.NeutronAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neutron",
			Namespace: instance.Namespace,
		},
	}
	Log := GetLogger(ctx)

	if !instance.Spec.Neutron.Enabled {
		if res, err := EnsureDeleted(ctx, helper, neutronAPI); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneNeutronReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeNeutronReadyCondition)
		instance.Status.ContainerImages.NeutronAPIImage = nil
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Neutron.Template.Override.Service == nil {
			instance.Spec.Neutron.Template.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Neutron.Template.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Neutron.Template.Override.Service[endpointType],
				neutronAPI.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "neutron", Namespace: instance.Namespace}, neutronAPI); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs,set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Neutron.Template.TLS = neutronAPI.Spec.TLS

		serviceName := "neutron"
		// create ovndb client certificate for neutron
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetOvnIssuer(),
			CertName:   fmt.Sprintf("%s-ovndbs", serviceName),
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", serviceName, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", serviceName, instance.Namespace, "cluster.local"),
			},
			Ips: nil,
			Usages: []certmgrv1.KeyUsage{
				certmgrv1.UsageKeyEncipherment,
				certmgrv1.UsageDigitalSignature,
				certmgrv1.UsageClientAuth,
			},
			Labels: map[string]string{serviceCertSelector: ""},
		}
		if instance.Spec.TLS.PodLevel.Ovn.Cert.Duration != nil {
			certRequest.Duration = &instance.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration
		}
		if instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore != nil {
			certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore.Duration
		}
		certSecret, ctrlResult, err := certmanager.EnsureCert(
			ctx,
			helper,
			certRequest,
			nil)
		if err != nil {
			return ctrl.Result{}, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrl.Result{}, nil
		}

		instance.Spec.Neutron.Template.TLS.Ovn.SecretName = &certSecret.Name
	}
	instance.Spec.Neutron.Template.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(neutronAPI.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Neutron.Template.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			neutronAPI,
			svcs,
			instance.Spec.Neutron.Template.Override.Service,
			instance.Spec.Neutron.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeNeutronReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			tls.API{API: instance.Spec.Neutron.Template.TLS.API},
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Neutron.Template.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Neutron.Template.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Neutron.Template.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	Log.Info("Reconciling NeutronAPI", "NeutronAPI.Namespace", instance.Namespace, "NeutronAPI.Name", "neutron")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), neutronAPI, func() error {
		instance.Spec.Neutron.Template.DeepCopyInto(&neutronAPI.Spec.NeutronAPISpecCore)
		neutronAPI.Spec.ContainerImage = *version.Status.ContainerImages.NeutronAPIImage
		if neutronAPI.Spec.Secret == "" {
			neutronAPI.Spec.Secret = instance.Spec.Secret
		}
		if neutronAPI.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			neutronAPI.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if neutronAPI.Spec.DatabaseInstance == "" {
			neutronAPI.Spec.DatabaseInstance = "openstack"
		}

		// Append globally defined extraMounts to the service's own list.
		for _, ev := range instance.Spec.ExtraMounts {
			neutronAPI.Spec.ExtraMounts = append(neutronAPI.Spec.ExtraMounts, neutronv1.NeutronExtraVolMounts{
				Name:      ev.Name,
				Region:    ev.Region,
				VolMounts: ev.VolMounts,
			})
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), neutronAPI, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNeutronReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneNeutronReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("neutronAPI %s - %s", neutronAPI.Name, op))
	}

	if neutronAPI.Status.ObservedGeneration == neutronAPI.Generation && neutronAPI.IsReady() {
		instance.Status.ContainerImages.NeutronAPIImage = version.Status.ContainerImages.NeutronAPIImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneNeutronReadyCondition, corev1beta1.OpenStackControlPlaneNeutronReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNeutronReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneNeutronReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// NeutronImageMatch - return true if the neutron images match on the ControlPlane and Version, or if Neutron is not enabled
func NeutronImageMatch(controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {

	if controlPlane.Spec.Neutron.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.NeutronAPIImage, version.Status.ContainerImages.NeutronAPIImage) {
			return false
		}
	}

	return true
}
