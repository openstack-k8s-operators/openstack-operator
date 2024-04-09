package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileKeystoneAPI -
func ReconcileKeystoneAPI(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	keystoneAPI := &keystonev1.KeystoneAPI{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone", //FIXME (keystone doesn't seem to work unless named "keystone")
			Namespace: instance.Namespace,
		},
	}

	Log := GetLogger(ctx)

	if !instance.Spec.Keystone.Enabled {
		if res, err := EnsureDeleted(ctx, helper, keystoneAPI); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeKeystoneAPIReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Keystone.Template.Override.Service == nil {
			instance.Spec.Keystone.Template.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Keystone.Template.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Keystone.Template.Override.Service[endpointType],
				keystoneAPI.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "keystone", Namespace: instance.Namespace}, keystoneAPI); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs,set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Keystone.Template.TLS = keystoneAPI.Spec.TLS
	}
	instance.Spec.Keystone.Template.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(keystoneAPI.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Keystone.Template.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			keystoneAPI,
			svcs,
			instance.Spec.Keystone.Template.Override.Service,
			instance.Spec.Keystone.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeKeystoneAPIReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Keystone.Template.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Keystone.Template.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Keystone.Template.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Keystone.Template.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	Log.Info("Reconciling KeystoneAPI", "KeystoneAPI.Namespace", instance.Namespace, "KeystoneAPI.Name", "keystone")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), keystoneAPI, func() error {
		instance.Spec.Keystone.Template.DeepCopyInto(&keystoneAPI.Spec.KeystoneAPISpecCore)

		keystoneAPI.Spec.ContainerImage = *version.Status.ContainerImages.KeystoneAPIImage
		if keystoneAPI.Spec.Secret == "" {
			keystoneAPI.Spec.Secret = instance.Spec.Secret
		}
		if keystoneAPI.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			keystoneAPI.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if keystoneAPI.Spec.DatabaseInstance == "" {
			//keystoneAPI.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			keystoneAPI.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), keystoneAPI, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("KeystoneAPI %s - %s", keystoneAPI.Name, op))
	}

	if keystoneAPI.Status.ObservedGeneration == keystoneAPI.Generation && keystoneAPI.IsReady() {
		instance.Status.ContainerImages.KeystoneAPIImage = version.Status.ContainerImages.KeystoneAPIImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition, corev1beta1.OpenStackControlPlaneKeystoneAPIReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneKeystoneAPIReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
