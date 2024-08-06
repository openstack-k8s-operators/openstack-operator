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
	"os"
	"strings"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"
)

var (
	envContainerImages  (map[string]*string)
	envAvailableVersion string
)

// SetupVersionDefaults -
func SetupVersionDefaults() {
	localVars := make(map[string]*string)
	for _, name := range os.Environ() {
		envArr := strings.Split(name, "=")
		if envArr[0] == "OPENSTACK_RELEASE_VERSION" {
			envAvailableVersion = envArr[1]
		}
		if strings.HasPrefix(envArr[0], "RELATED_IMAGE_") {
			localVars[envArr[0]] = &envArr[1]
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

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackVersionReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackVersion")
}

// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch
// +kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *OpenStackVersionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)
	Log.Info("Reconciling OpenStackVersion")
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
		Log.Error(err, "unable to create helper")
		return ctrl.Result{}, err
	}

	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the condtions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
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
		}

		condition.RestoreLastTransitionTimes(
			&instance.Status.Conditions, savedConditions)

		err := versionHelper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// greenfield deployment
	cl := condition.CreateList(
		condition.UnknownCondition(corev1beta1.OpenStackVersionInitialized, condition.InitReason, string(corev1beta1.OpenStackVersionInitializedInitMessage)),
	)
	// no minor update conditions unless we have a deployed version
	if instance.Status.DeployedVersion != nil && instance.Spec.TargetVersion != *instance.Status.DeployedVersion {
		cl = append(cl, *condition.UnknownCondition(corev1beta1.OpenStackVersionMinorUpdateOVNControlplane, condition.InitReason, string(corev1beta1.OpenStackVersionMinorUpdateInitMessage)),
			*condition.UnknownCondition(corev1beta1.OpenStackVersionMinorUpdateOVNDataplane, condition.InitReason, string(corev1beta1.OpenStackVersionMinorUpdateInitMessage)),
			*condition.UnknownCondition(corev1beta1.OpenStackVersionMinorUpdateControlplane, condition.InitReason, string(corev1beta1.OpenStackVersionMinorUpdateInitMessage)),
			*condition.UnknownCondition(corev1beta1.OpenStackVersionMinorUpdateDataplane, condition.InitReason, string(corev1beta1.OpenStackVersionMinorUpdateInitMessage)),
		)
	}
	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && isNewInstance {
		controllerutil.AddFinalizer(instance, versionHelper.GetFinalizer())
		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}

	instance.Status.Conditions.Set(condition.FalseCondition(
		corev1beta1.OpenStackVersionInitialized,
		condition.RequestedReason,
		condition.SeverityInfo,
		corev1beta1.OpenStackVersionInitializedReadyRunningMessage))

	instance.Status.AvailableVersion = &envAvailableVersion
	defaults := openstack.InitializeOpenStackVersionImageDefaults(ctx, envContainerImages)
	if instance.Status.ContainerImageVersionDefaults == nil {
		instance.Status.ContainerImageVersionDefaults = make(map[string]*corev1beta1.ContainerDefaults)
	}
	// store the defaults for the currently available version
	instance.Status.ContainerImageVersionDefaults[envAvailableVersion] = defaults

	// calculate the container images for the target version
	Log.Info("Target version: ", "targetVersion", instance.Spec.TargetVersion)
	val, ok := instance.Status.ContainerImageVersionDefaults[instance.Spec.TargetVersion]
	if !ok {
		Log.Info("Target version not found in defaults", "targetVersion", instance.Spec.TargetVersion)
		return ctrl.Result{}, nil
	}
	instance.Status.ContainerImages = openstack.GetContainerImages(val, *instance)

	instance.Status.Conditions.MarkTrue(
		corev1beta1.OpenStackVersionInitialized,
		corev1beta1.OpenStackVersionInitializedReadyMessage)
	Log.Info("OpenStackVersion Initialized")

	// lookup the current Controlplane object
	controlPlane := &corev1beta1.OpenStackControlPlane{}
	err = r.Client.Get(ctx, client.ObjectKey{
		Namespace: instance.Namespace,
		Name:      instance.Name,
	}, controlPlane)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			Log.Info("Controlplane not found:", "instance name", instance.Name)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// lookup nodesets
	dataplaneNodesets, err := openstack.GetDataplaneNodesets(ctx, controlPlane, versionHelper)
	if err != nil {
		Log.Error(err, "Failed to get dataplane nodesets")
		return ctrl.Result{}, err
	}

	// greenfield deployment
	if controlPlane.Status.DeployedVersion == nil && !openstack.DataplaneNodesetsDeployedVersionIsSet(dataplaneNodesets) {
		Log.Info("Waiting for controlplane and dataplane nodesets to be deployed.")
		return ctrl.Result{}, nil
	}

	// minor update in progress
	if instance.Status.DeployedVersion != nil && instance.Spec.TargetVersion != *instance.Status.DeployedVersion {

		if !openstack.OVNControllerImageMatch(controlPlane, instance) ||
			!controlPlane.Status.Conditions.IsTrue(corev1beta1.OpenStackControlPlaneOVNReadyCondition) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackVersionMinorUpdateOVNControlplane,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackVersionMinorUpdateReadyRunningMessage))
			Log.Info("Minor update for OVN Controlplane in progress")
			return ctrl.Result{}, nil
		}
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackVersionMinorUpdateOVNControlplane,
			corev1beta1.OpenStackVersionMinorUpdateReadyMessage)

		// minor update for Dataplane OVN
		if !openstack.DataplaneNodesetsOVNControllerImagesMatch(instance, dataplaneNodesets) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackVersionMinorUpdateOVNDataplane,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackVersionMinorUpdateReadyRunningMessage))
			Log.Info("Waiting on OVN Dataplane updates to complete")
			return ctrl.Result{}, nil
		}
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackVersionMinorUpdateOVNDataplane,
			corev1beta1.OpenStackVersionMinorUpdateReadyMessage)

		// minor update for Controlplane in progress
		if !openstack.ControlplaneContainerImageMatch(controlPlane, instance) ||
			!controlPlane.IsReady() {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackVersionMinorUpdateControlplane,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackVersionMinorUpdateReadyRunningMessage))
			Log.Info("Minor update for Controlplane in progress")
			return ctrl.Result{}, nil
		}
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackVersionMinorUpdateControlplane,
			corev1beta1.OpenStackVersionMinorUpdateReadyMessage)

		if !openstack.DataplaneNodesetsDeployed(instance, dataplaneNodesets) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackVersionMinorUpdateDataplane,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackVersionMinorUpdateReadyRunningMessage))
			Log.Info("Waiting on Dataplane update to complete")
			return ctrl.Result{}, nil
		}

		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackVersionMinorUpdateDataplane,
			corev1beta1.OpenStackVersionMinorUpdateReadyMessage)
	}

	if controlPlane.IsReady() {
		Log.Info("Setting DeployedVersion")
		instance.Status.DeployedVersion = &instance.Spec.TargetVersion
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackVersionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	versionFunc := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		Log := r.GetLogger(ctx)
		versionList := &corev1beta1.OpenStackVersionList{}

		result := []reconcile.Request{}

		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(ctx, versionList, listOpts...); err != nil {
			Log.Error(err, "Unable to retrieve OpenStackVersion")
			return nil
		}

		for _, i := range versionList.Items {
			name := client.ObjectKey{
				Namespace: o.GetNamespace(),
				Name:      i.Name,
			}
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			Log.Info("Reconcile request for:", "result", result)
			return result
		}
		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&corev1beta1.OpenStackControlPlane{}, versionFunc).
		Watches(&dataplanev1.OpenStackDataPlaneNodeSet{}, versionFunc).
		For(&corev1beta1.OpenStackVersion{}).
		Complete(r)
}
