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

package core

import (
	"context"
	"fmt"
	"os"
	"strings"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"golang.org/x/exp/slices"
)

var envContainerImages (map[string]string)
var envAvailableVersion string

// SetupVersionDefaults -
func SetupVersionDefaults() {
	fmt.Println("SetupVersionDefaults")
	localVars := make(map[string]string)
	for _, name := range os.Environ() {
		envArr := strings.Split(name, "=")
		if envArr[0] == "OPENSTACK_VERSION" {
			envAvailableVersion = envArr[1]
		}
		if strings.HasPrefix(envArr[0], "RELATED_IMAGE_") {
			localVars[envArr[0]] = envArr[1]
		}
	}
	envContainerImages = localVars
}

// OpenStackVersionReconciler reconciles a OpenStackVersion object
type OpenStackVersionReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Scheme  *runtime.Scheme
	Log     logr.Logger
}

// func to append corev1beta1.OpenStackService if not already there
func appendServiceIfMissing(slice []corev1beta1.OpenStackService, i corev1beta1.OpenStackService) []corev1beta1.OpenStackService {
	for _, ele := range slice {
		if ele.ServiceName == i.ServiceName {
			return slice
		}
	}
	return append(slice, i)
}

// func to remove corev1beta1.OpenStackService from slice
func removeService(slice []corev1beta1.OpenStackService, i corev1beta1.OpenStackService) []corev1beta1.OpenStackService {
	for index, ele := range slice {
		if ele.ServiceName == i.ServiceName {
			return append(slice[:index], slice[index+1:]...)
		}
	}
	return slice
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackVersionReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackVersion")
}

//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackVersion object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *OpenStackVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {

	Log := r.GetLogger(ctx)
	// Fetch the instance
	instance := &corev1beta1.OpenStackVersion{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	versionHelper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		/*
		   // update the Ready condition based on the sub conditions
		   if instance.Status.Conditions.AllSubConditionIsTrue() {
		       instance.Status.Conditions.MarkTrue(
		           condition.ReadyCondition, condition.ReadyMessage)
		   } else {
		       // something is not ready so reset the Ready condition
		       instance.Status.Conditions.MarkUnknown(
		           condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
		       // and recalculate it based on the state of the rest of the conditions
		       instance.Status.Conditions.Set(
		           instance.Status.Conditions.Mirror(condition.ReadyCondition))
		   }*/

		err := versionHelper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()
	instance.Status.AvailableVersion = envAvailableVersion
	instance.Status.AvailableServices = make([]string, 0)
	for service, _ := range envContainerImages {
		instance.Status.AvailableServices = append(instance.Status.AvailableServices, (strings.TrimPrefix(strings.TrimSuffix(service, "_IMAGE_URL_DEFAULT"), "RELATED_IMAGE_")))
	}

	if len(instance.Spec.TargetVersion) > 0 && instance.Spec.TargetVersion == instance.Status.TargetVersion {
		return ctrl.Result{}, nil
	}

	// lookup the named OpenStackControlplane
	controlPlane, err := r.lookupOpenStackControlPlane(ctx, instance)
	if err != nil {
		return ctrl.Result{}, err
	}
	controlPlaneHelper, err := helper.NewHelper(
		controlPlane,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// services
	glance := CheckGlanceImages(ctx, Log, &controlPlane.Spec.Glance.Template, instance.Spec.ServiceExcludes, envContainerImages)
	if !glance {
		instance.Status.ServicesNeedingUpdates = appendServiceIfMissing(instance.Status.ServicesNeedingUpdates, corev1beta1.OpenStackService{ServiceName: "glance"})
	}

	cinder := CheckCinderImages(ctx, Log, &controlPlane.Spec.Cinder.Template, instance.Spec.ServiceExcludes, instance.Spec.CinderVolumeExcludes, envContainerImages)
	if !cinder {
		instance.Status.ServicesNeedingUpdates = appendServiceIfMissing(instance.Status.ServicesNeedingUpdates, corev1beta1.OpenStackService{ServiceName: "cinder"})
	}

	// FIXME: move this to webhook validation when created
	if instance.Spec.TargetVersion != envAvailableVersion {
		return ctrl.Result{}, fmt.Errorf(fmt.Sprintf("Target version must equal: %s", envAvailableVersion))
	}

	if len(instance.Status.ServicesNeedingUpdates) > 0 {

		if !glance {
			UpdateGlanceImages(ctx, Log, &controlPlane.Spec.Glance.Template, instance.Spec.ServiceExcludes, envContainerImages)
			glanceService := corev1beta1.OpenStackService{ServiceName: "glance"}
			instance.Status.DeployedVersions = appendServiceIfMissing(instance.Status.DeployedVersions, glanceService)
			instance.Status.ServicesNeedingUpdates = removeService(instance.Status.ServicesNeedingUpdates, glanceService)
		}
		if !cinder {
			UpdateCinderImages(ctx, Log, &controlPlane.Spec.Cinder.Template, instance.Spec.ServiceExcludes, instance.Spec.CinderVolumeExcludes, envContainerImages)
			cinderService := corev1beta1.OpenStackService{ServiceName: "cinder"}
			instance.Status.DeployedVersions = appendServiceIfMissing(instance.Status.DeployedVersions, cinderService)
			instance.Status.ServicesNeedingUpdates = removeService(instance.Status.ServicesNeedingUpdates, cinderService)
		}

		err = controlPlaneHelper.PatchInstance(ctx, controlPlane)
		if err != nil {
			return ctrl.Result{}, err
		}

		// FIXME: do the same for the dataplane

	}
	instance.Status.TargetVersion = envAvailableVersion
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {

	// FIXME: add watch here on CSV or openstack-operator deployment so
	// we are notified of newly available updates

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.OpenStackVersion{}).
		Complete(r)
}

// function to lookup the named OpenStackControlplane
func (r *OpenStackVersionReconciler) lookupOpenStackControlPlane(ctx context.Context, instance *corev1beta1.OpenStackVersion) (*corev1beta1.OpenStackControlPlane, error) {
	// Fetch the OpenStackControlPlane instance
	controlPlane := &corev1beta1.OpenStackControlPlane{}
	err := r.Client.Get(ctx, client.ObjectKey{
		Name:      instance.Spec.OpenStackControlPlaneName,
		Namespace: instance.Namespace,
	}, controlPlane)
	if err != nil {
		return nil, err
	}
	return controlPlane, nil
}

// FIXME: the functions below can all go into the respective operators

// CheckCinderImages - return false if cinder needs an update
func CheckCinderImages(ctx context.Context, log logr.Logger, cinder *cinderv1.CinderSpec, serviceExcludes []string, cinderExcludes []string, envVars map[string]string) bool {
	log.Info(fmt.Sprintf("CheckCinderImages.RELATED_IMAGE_CINDER_API_IMAGE_URL_DEFAULT: '%s'", envVars["RELATED_IMAGE_CINDER_API_IMAGE_URL_DEFAULT"]))
	log.Info(fmt.Sprintf("CheckCinderImages.cinder.CinderAPI.ContainerImage: %s", cinder.CinderAPI.ContainerImage))
	if !slices.Contains(serviceExcludes, "CinderApi") {
		if envVars["RELATED_IMAGE_CINDER_API_IMAGE_URL_DEFAULT"] != cinder.CinderAPI.ContainerImage {
			return false
		}
	}

	if !slices.Contains(serviceExcludes, "CinderScheduler") {
		if envVars["RELATED_IMAGE_CINDER_SCHEDULER_IMAGE_URL_DEFAULT"] != cinder.CinderScheduler.ContainerImage {
			return false
		}
	}

	if !slices.Contains(serviceExcludes, "CinderBackup") {
		if envVars["RELATED_IMAGE_CINDER_BACKUP_IMAGE_URL_DEFAULT"] != cinder.CinderBackup.ContainerImage {
			return false
		}
	}

	for name, volume := range cinder.CinderVolumes {
		if !slices.Contains(cinderExcludes, name) {
			if envVars["RELATED_IMAGE_CINDER_VOLUME_IMAGE_URL_DEFAULT"] != volume.ContainerImage {
				return false
			}
		}
	}

	return true
}

// UpdateCinderImages -
func UpdateCinderImages(ctx context.Context, log logr.Logger, cinder *cinderv1.CinderSpec, serviceExcludes []string, cinderExcludes []string, envVars map[string]string) {
	log.Info("UpdateCinderImages")
	if !slices.Contains(serviceExcludes, "CinderApi") {
		cinder.CinderAPI.ContainerImage = envVars["RELATED_IMAGE_CINDER_API_IMAGE_URL_DEFAULT"]
	}
	if !slices.Contains(serviceExcludes, "CinderScheduler") {
		cinder.CinderScheduler.ContainerImage = envVars["RELATED_IMAGE_CINDER_SCHEDULER_IMAGE_URL_DEFAULT"]
	}
	if !slices.Contains(serviceExcludes, "CinderBackup") {
		cinder.CinderBackup.ContainerImage = envVars["RELATED_IMAGE_CINDER_BACKUP_IMAGE_URL_DEFAULT"]
	}
	for name, volume := range cinder.CinderVolumes {
		if !slices.Contains(cinderExcludes, name) {
			volume.CinderServiceTemplate.ContainerImage = envVars["RELATED_IMAGE_CINDER_VOLUME_IMAGE_URL_DEFAULT"]
		}
	}
}

// CheckGlanceImages - return false if glance needs an update
func CheckGlanceImages(ctx context.Context, log logr.Logger, glance *glancev1.GlanceSpec, serviceExcludes []string, envVars map[string]string) bool {
	log.Info(fmt.Sprintf("CheckGlanceImages.RELATED_IMAGE_GLANCE_API_IMAGE_URL_DEFAULT: '%s'", envVars["RELATED_IMAGE_GLANCE_API_IMAGE_URL_DEFAULT"]))
	log.Info(fmt.Sprintf("CheckGlanceImages.glance.ContainerImage: %s", glance.ContainerImage))
	if !slices.Contains(serviceExcludes, "GlanceApi") {
		if envVars["RELATED_IMAGE_GLANCE_API_IMAGE_URL_DEFAULT"] != glance.ContainerImage {
			return false
		}
	}
	return true
}

// UpdateGlanceImages -
func UpdateGlanceImages(ctx context.Context, log logr.Logger, glance *glancev1.GlanceSpec, serviceExcludes []string, envVars map[string]string) {
	log.Info("UpdateGlanceImages")
	if !slices.Contains(serviceExcludes, "GlanceApi") {
		glance.ContainerImage = envVars["RELATED_IMAGE_GLANCE_API_IMAGE_URL_NEW"]
	}
}
