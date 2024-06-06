/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileOctavia -
func ReconcileOctavia(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	octavia := &octaviav1.Octavia{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "octavia",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Octavia.Enabled {
		if res, err := EnsureDeleted(ctx, helper, octavia); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneOctaviaReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeOctaviaReadyCondition)
		instance.Status.ContainerImages.OctaviaAPIImage = nil
		instance.Status.ContainerImages.OctaviaWorkerImage = nil
		instance.Status.ContainerImages.OctaviaHealthmanagerImage = nil
		instance.Status.ContainerImages.OctaviaHousekeepingImage = nil
		//FIXME: (dprince) Octavia should have its own parameter for the apache image (it can share the same image in OpenStackVersion though)
		instance.Status.ContainerImages.ApacheImage = nil
		return ctrl.Result{}, nil
	}

	if instance.Spec.Octavia.Template == nil {
		instance.Spec.Octavia.Template = &octaviav1.OctaviaSpecCore{}
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Octavia.Template.OctaviaAPI.Override.Service == nil {
			instance.Spec.Octavia.Template.OctaviaAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Octavia.Template.OctaviaAPI.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Octavia.Template.OctaviaAPI.Override.Service[endpointType],
				octavia.Name)
	}

	// preserve any previously set TLS certs, set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Octavia.Template.OctaviaAPI.TLS = octavia.Spec.OctaviaAPI.TLS

		serviceName := "octavia"
		// create ovndb client certificate for octavia
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetOvnIssuer(),
			CertName:   fmt.Sprintf("%s-ovndbs", serviceName),
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", serviceName, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", serviceName, instance.Namespace, ClusterInternalDomain),
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

		instance.Spec.Octavia.Template.OctaviaAPI.TLS.Ovn.SecretName = &certSecret.Name
	}
	instance.Spec.Octavia.Template.OctaviaAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "octavia", Namespace: instance.Namespace}, octavia); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(octavia.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Octavia.Template.OctaviaAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			octavia,
			svcs,
			instance.Spec.Octavia.Template.OctaviaAPI.Override.Service,
			instance.Spec.Octavia.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeOctaviaReadyCondition,
			false, // TODO: (mschuppert) could be removed when all integrated service support TLS
			tls.API{},
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Octavia.Template.OctaviaAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()

		// update TLS settings with cert secret
		instance.Spec.Octavia.Template.OctaviaAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Octavia.Template.OctaviaAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	helper.GetLogger().Info("Reconciling Octavia", "Octavia.Namespace", instance.Namespace, "Octavia.Name", octavia.Name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), octavia, func() error {
		instance.Spec.Octavia.Template.OctaviaSpecBase.DeepCopyInto(&octavia.Spec.OctaviaSpecBase)
		instance.Spec.Octavia.Template.OctaviaAPI.DeepCopyInto(&octavia.Spec.OctaviaAPI.OctaviaAPISpecCore)
		instance.Spec.Octavia.Template.OctaviaHousekeeping.DeepCopyInto(&octavia.Spec.OctaviaHousekeeping.OctaviaAmphoraControllerSpecCore)
		instance.Spec.Octavia.Template.OctaviaHealthManager.DeepCopyInto(&octavia.Spec.OctaviaHealthManager.OctaviaAmphoraControllerSpecCore)
		instance.Spec.Octavia.Template.OctaviaWorker.DeepCopyInto(&octavia.Spec.OctaviaWorker.OctaviaAmphoraControllerSpecCore)

		octavia.Spec.OctaviaAPI.ContainerImage = *version.Status.ContainerImages.OctaviaAPIImage
		octavia.Spec.OctaviaWorker.ContainerImage = *version.Status.ContainerImages.OctaviaWorkerImage
		octavia.Spec.OctaviaHealthManager.ContainerImage = *version.Status.ContainerImages.OctaviaHealthmanagerImage
		octavia.Spec.OctaviaHousekeeping.ContainerImage = *version.Status.ContainerImages.OctaviaHousekeepingImage
		octavia.Spec.ApacheContainerImage = *version.Status.ContainerImages.ApacheImage

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), octavia, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOctaviaReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneOctaviaReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Octavia %s - %s", octavia.Name, op))
	}

	if octavia.Status.ObservedGeneration == octavia.Generation && octavia.IsReady() {
		instance.Status.ContainerImages.OctaviaAPIImage = version.Status.ContainerImages.OctaviaAPIImage
		instance.Status.ContainerImages.OctaviaWorkerImage = version.Status.ContainerImages.OctaviaWorkerImage
		instance.Status.ContainerImages.OctaviaHealthmanagerImage = version.Status.ContainerImages.OctaviaHealthmanagerImage
		instance.Status.ContainerImages.OctaviaHousekeepingImage = version.Status.ContainerImages.OctaviaHousekeepingImage
		instance.Status.ContainerImages.ApacheImage = version.Status.ContainerImages.ApacheImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneOctaviaReadyCondition, corev1beta1.OpenStackControlPlaneOctaviaReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOctaviaReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneOctaviaReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}

// OctaviaImageMatch - return true if the octavia images match on the ControlPlane and Version, or if Octavia is not enabled
func OctaviaImageMatch(controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {

	if controlPlane.Spec.Octavia.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.OctaviaAPIImage, version.Status.ContainerImages.OctaviaAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.OctaviaWorkerImage, version.Status.ContainerImages.OctaviaWorkerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.OctaviaHealthmanagerImage, version.Status.ContainerImages.OctaviaHealthmanagerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.OctaviaHousekeepingImage, version.Status.ContainerImages.OctaviaHousekeepingImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.ApacheImage, version.Status.ContainerImages.ApacheImage) {
			return false
		}
	}

	return true
}
