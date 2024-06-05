package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	heatName = "heat"
)

// ReconcileHeat -
func ReconcileHeat(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	heat := &heatv1.Heat{
		ObjectMeta: metav1.ObjectMeta{
			Name:      heatName,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Heat.Enabled {
		if res, err := EnsureDeleted(ctx, helper, heat); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneHeatReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeHeatReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Heat.Template.HeatAPI.Override.Service == nil {
			instance.Spec.Heat.Template.HeatAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Heat.Template.HeatAPI.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Heat.Template.HeatAPI.Override.Service[endpointType],
				heat.Name+"-api")

		if instance.Spec.Heat.Template.HeatCfnAPI.Override.Service == nil {
			instance.Spec.Heat.Template.HeatCfnAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Heat.Template.HeatCfnAPI.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Heat.Template.HeatCfnAPI.Override.Service[endpointType],
				heat.Name+"-cfn")
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "heat", Namespace: instance.Namespace}, heat); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs,set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Heat.Template.HeatAPI.TLS = heat.Spec.HeatAPI.TLS
		instance.Spec.Heat.Template.HeatCfnAPI.TLS = heat.Spec.HeatCfnAPI.TLS
	}
	instance.Spec.Heat.Template.HeatAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
	instance.Spec.Heat.Template.HeatCfnAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	// Heat API
	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(heat.Name+"-api"),
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Heat.Template.HeatAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			heat,
			svcs,
			instance.Spec.Heat.Template.HeatAPI.Override.Service,
			instance.Spec.Heat.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeHeatReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Heat.Template.HeatAPI.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Heat.Template.HeatAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Heat.Template.HeatAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Heat.Template.HeatAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	// Heat CFNAPI
	svcs, err = service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(heat.Name+"-cfn"),
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Heat.Template.HeatCfnAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			heat,
			svcs,
			instance.Spec.Heat.Template.HeatCfnAPI.Override.Service,
			instance.Spec.Heat.CnfAPIOverride,
			corev1beta1.OpenStackControlPlaneExposeHeatReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Heat.Template.HeatCfnAPI.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Heat.Template.HeatCfnAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Heat.Template.HeatCfnAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Heat.Template.HeatCfnAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}
	Log := GetLogger(ctx)

	Log.Info("Reconcile heat", "heat.Namespace", instance.Namespace, "heat.Name", "heat")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), heat, func() error {
		instance.Spec.Heat.Template.HeatSpecBase.DeepCopyInto(&heat.Spec.HeatSpecBase)
		instance.Spec.Heat.Template.HeatAPI.DeepCopyInto(&heat.Spec.HeatAPI.HeatAPITemplateCore)
		instance.Spec.Heat.Template.HeatCfnAPI.DeepCopyInto(&heat.Spec.HeatCfnAPI.HeatCfnAPITemplateCore)
		instance.Spec.Heat.Template.HeatEngine.DeepCopyInto(&heat.Spec.HeatEngine.HeatEngineTemplateCore)

		if heat.Spec.DatabaseInstance == "" {
			//heat.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			heat.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}

		heat.Spec.HeatAPI.ContainerImage = *version.Status.ContainerImages.HeatAPIImage
		heat.Spec.HeatCfnAPI.ContainerImage = *version.Status.ContainerImages.HeatCfnapiImage
		heat.Spec.HeatEngine.ContainerImage = *version.Status.ContainerImages.HeatEngineImage

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), heat, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHeatReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneHeatReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("heat %s - %s", heat.Name, op))
	}

	if heat.Status.ObservedGeneration == heat.Generation && heat.IsReady() {
		instance.Status.ContainerImages.HeatAPIImage = version.Status.ContainerImages.HeatAPIImage
		instance.Status.ContainerImages.HeatCfnapiImage = version.Status.ContainerImages.HeatCfnapiImage
		instance.Status.ContainerImages.HeatEngineImage = version.Status.ContainerImages.HeatEngineImage
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneHeatReadyCondition, corev1beta1.OpenStackControlPlaneHeatReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneHeatReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneHeatReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
