package openstack

import (
	"context"
	"errors"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileCinder -
func ReconcileCinder(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	cinderName, altCinderName := instance.GetServiceName(corev1beta1.CinderName, instance.Spec.Cinder.UniquePodNames)
	// Ensure the alternate cinder CR doesn't exist, as the ramdomPodNames flag may have been toggled
	cinder := &cinderv1.Cinder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      altCinderName,
			Namespace: instance.Namespace,
		},
	}
	if res, err := EnsureDeleted(ctx, helper, cinder); err != nil {
		return res, err
	}

	cinder = &cinderv1.Cinder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cinderName,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Cinder.Enabled {
		if res, err := EnsureDeleted(ctx, helper, cinder); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneCinderReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeCinderReadyCondition)
		instance.Status.ContainerImages.CinderAPIImage = nil
		instance.Status.ContainerImages.CinderSchedulerImage = nil
		instance.Status.ContainerImages.CinderBackupImage = nil
		instance.Status.ContainerImages.CinderVolumeImages = make(map[string]*string)
		return ctrl.Result{}, nil
	}
	Log := GetLogger(ctx)

	if instance.Spec.Cinder.Template == nil {
		instance.Spec.Cinder.Template = &cinderv1.CinderSpecCore{}
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Cinder.Template.CinderAPI.Override.Service == nil {
			instance.Spec.Cinder.Template.CinderAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Cinder.Template.CinderAPI.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Cinder.Template.CinderAPI.Override.Service[endpointType],
				cinder.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: cinderName, Namespace: instance.Namespace}, cinder); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// preserve any previously set TLS certs,set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Cinder.Template.CinderAPI.TLS = cinder.Spec.CinderAPI.TLS
	}
	instance.Spec.Cinder.Template.CinderAPI.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(cinder.Name),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Cinder.Template.CinderAPI.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			cinder,
			svcs,
			instance.Spec.Cinder.Template.CinderAPI.Override.Service,
			instance.Spec.Cinder.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeCinderReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Cinder.Template.CinderAPI.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Cinder.Template.CinderAPI.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// update TLS settings with cert secret
		instance.Spec.Cinder.Template.CinderAPI.TLS.API.Public.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Cinder.Template.CinderAPI.TLS.API.Internal.SecretName = endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	if instance.Spec.Cinder.Template.NodeSelector == nil {
		instance.Spec.Cinder.Template.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if instance.Spec.Cinder.Template.TopologyRef == nil {
		instance.Spec.Cinder.Template.TopologyRef = instance.Spec.TopologyRef
	}

	Log.Info("Reconciling Cinder", "Cinder.Namespace", instance.Namespace, "Cinder.Name", cinderName)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), cinder, func() error {
		instance.Spec.Cinder.Template.CinderSpecBase.DeepCopyInto(&cinder.Spec.CinderSpecBase)
		instance.Spec.Cinder.Template.CinderAPI.DeepCopyInto(&cinder.Spec.CinderAPI.CinderAPITemplateCore)
		instance.Spec.Cinder.Template.CinderScheduler.DeepCopyInto(&cinder.Spec.CinderScheduler.CinderSchedulerTemplateCore)
		instance.Spec.Cinder.Template.CinderBackup.DeepCopyInto(&cinder.Spec.CinderBackup.CinderBackupTemplateCore)

		cinder.Spec.CinderAPI.ContainerImage = *version.Status.ContainerImages.CinderAPIImage
		cinder.Spec.CinderScheduler.ContainerImage = *version.Status.ContainerImages.CinderSchedulerImage
		cinder.Spec.CinderBackup.ContainerImage = *version.Status.ContainerImages.CinderBackupImage

		defaultVolumeImg := version.Status.ContainerImages.CinderVolumeImages["default"]
		if defaultVolumeImg == nil {
			return errors.New("default Cinder Volume images is unset")
		}

		// Discard old list of volume services and rebuild it
		cinder.Spec.CinderVolumes = make(map[string]cinderv1.CinderVolumeTemplate)

		for name, volume := range instance.Spec.Cinder.Template.CinderVolumes {
			cinderCore := cinderv1.CinderVolumeTemplate{}
			volume.DeepCopyInto(&cinderCore.CinderVolumeTemplateCore)
			if volVal, ok := version.Status.ContainerImages.CinderVolumeImages[name]; ok {
				cinderCore.ContainerImage = *volVal
			} else {
				cinderCore.ContainerImage = *defaultVolumeImg
			}
			cinder.Spec.CinderVolumes[name] = cinderCore
		}

		if cinder.Spec.Secret == "" {
			cinder.Spec.Secret = instance.Spec.Secret
		}
		if cinder.Spec.DatabaseInstance == "" {
			//cinder.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			cinder.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		// Append globally defined extraMounts to the service's own list.
		for _, ev := range instance.Spec.ExtraMounts {
			cinder.Spec.ExtraMounts = append(cinder.Spec.ExtraMounts, cinderv1.CinderExtraVolMounts{
				Name:      ev.Name,
				Region:    ev.Region,
				VolMounts: ev.VolMounts,
			})
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), cinder, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCinderReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneCinderReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("Cinder %s - %s", cinder.Name, op))
	}

	if cinder.Status.ObservedGeneration == cinder.Generation && cinder.IsReady() {
		instance.Status.ContainerImages.CinderAPIImage = version.Status.ContainerImages.CinderAPIImage
		instance.Status.ContainerImages.CinderSchedulerImage = version.Status.ContainerImages.CinderSchedulerImage
		instance.Status.ContainerImages.CinderBackupImage = version.Status.ContainerImages.CinderBackupImage
		instance.Status.ContainerImages.CinderVolumeImages = version.Status.ContainerImages.DeepCopy().CinderVolumeImages
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneCinderReadyCondition, corev1beta1.OpenStackControlPlaneCinderReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneCinderReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneCinderReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}

// CinderImageMatch - return true if the Cinder images match on the ControlPlane and Version, or if Cinder is not enabled
func CinderImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Cinder.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.CinderAPIImage, version.Status.ContainerImages.CinderAPIImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CinderSchedulerImage, version.Status.ContainerImages.CinderSchedulerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.CinderBackupImage, version.Status.ContainerImages.CinderBackupImage) {
			Log.Info("Cinder images do not match")
			return false
		}
		for name, img := range version.Status.ContainerImages.CinderVolumeImages {
			if !stringPointersEqual(controlPlane.Status.ContainerImages.CinderVolumeImages[name], img) {
				Log.Info("Cinder Volume images do not match")
				return false
			}
		}
	}

	return true
}
