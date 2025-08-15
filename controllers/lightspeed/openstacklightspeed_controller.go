/*
Copyright 2025.

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

package lightspeed

import (
	"context"
	"fmt"
	"time"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	lightspeedv1 "github.com/openstack-k8s-operators/openstack-operator/apis/lightspeed/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	lightspeedv1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/lightspeed/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/lightspeed"
)

// OpenStackLightspeedReconciler reconciles a OpenStackLightspeed object
type OpenStackLightspeedReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackLightspeedReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackLightspeed")
}

// +kubebuilder:rbac:groups=lightspeed.openstack.org,resources=openstacklightspeeds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=lightspeed.openstack.org,resources=openstacklightspeeds/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=lightspeed.openstack.org,resources=openstacklightspeeds/finalizers,verbs=update
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,verbs=get;list;
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *OpenStackLightspeedReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)

	instance := &lightspeedv1beta1.OpenStackLightspeed{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			Log.Info("OpenStackLightspeed CR not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	helper, err := common_helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)

	// Save a copy of the conditions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// Don't update the status, if reconciler Panics
		if r := recover(); r != nil {
			Log.Info(fmt.Sprintf("panic during reconcile %v\n", r))
			panic(r)
		}

		condition.RestoreLastTransitionTimes(&instance.Status.Conditions, savedConditions)
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

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			return
		}

	}()

	cl := condition.CreateList(
		condition.UnknownCondition(
			lightspeedv1.OpenStackLightspeedReadyCondition,
			condition.InitReason,
			lightspeedv1.OpenStackLightspeedReadyInitMessage,
		),
	)

	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, helper, instance)
	}

	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) {
		return ctrl.Result{}, nil
	}

	if instance.Spec.RAGImage == "" {
		instance.Spec.RAGImage = lightspeedv1beta1.OpenStackLightspeedDefaultValues.RAGImageURL
	}

	OLSOperatorInstalled, err := lightspeed.IsOLSOperatorInstalled(ctx, helper)
	if !OLSOperatorInstalled || err != nil {
		errMsg := fmt.Errorf("installation of OpenShift LightSpeed not detected")
		instance.Status.Conditions.Set(condition.FalseCondition(
			lightspeedv1.OpenStackLightspeedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			errMsg))
		return ctrl.Result{}, errMsg
	}

	// TODO(lpiwowar): Remove ResolveIndexID once OpenShift Lightspeed supports auto discovery of the indexID directly
	// from the vector db image.
	indexID, result, err := lightspeed.ResolveIndexID(ctx, helper, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			lightspeedv1.OpenStackLightspeedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return result, err
	} else if (result != ctrl.Result{}) {
		return result, nil
	}

	// NOTE: We cannot consume the OLSConfig definition directly from the OLS operator's code due to
	// a conflict in Go versions. When this comment was written, the min. required Go version for
	// openstack-operator was 1.21 whereas OLS operator required at least Go version 1.23. Once the
	// Go versions catch up with each other we should consider consuming OLSConfig directly from OLS
	// operator and updating this code and any subsequent code that consumes this structure.
	olsConfig := uns.Unstructured{}
	olsConfigGVK := schema.GroupVersionKind{
		Group:   "ols.openshift.io",
		Version: "v1alpha1",
		Kind:    "OLSConfig",
	}

	olsConfig.SetGroupVersionKind(olsConfigGVK)
	olsConfig.SetName(lightspeed.OLSConfigName)

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, &olsConfig, func() error {
		// Check if the OpenStackLightspeed instance that is being processed owns the OLSConfig. If
		// it is owned by other OpenStackLightspeed instance stop the reconciliation.
		olsConfigLabels := olsConfig.GetLabels()
		ownerLabel := ""
		if val, ok := olsConfigLabels[lightspeed.OpenStackLightspeedOwnerIDLabel]; ok {
			ownerLabel = val
		}

		if ownerLabel != "" && ownerLabel != string(instance.GetObjectMeta().GetUID()) {
			return fmt.Errorf("OLSConfig is managed by different OpenStackLightspeed instance")
		}

		err = lightspeed.PatchOLSConfig(helper, instance, &olsConfig, indexID)
		if err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			lightspeedv1.OpenStackLightspeedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	OLSConfigReady, err := lightspeed.IsOLSConfigReady(ctx, helper)
	if err != nil {
		return ctrl.Result{}, err
	}

	if OLSConfigReady {
		instance.Status.Conditions.MarkTrue(
			lightspeedv1.OpenStackLightspeedReadyCondition,
			lightspeedv1.OpenStackLightspeedReadyMessage,
		)
	} else {
		Log.Info("OLSConfig is not ready yet. Waiting.")
		return ctrl.Result{RequeueAfter: time.Second * time.Duration(5)}, nil
	}

	return ctrl.Result{}, nil
}

// reconcileDelete reconciles the deletion of OpenStackLightspeed instance
func (r *OpenStackLightspeedReconciler) reconcileDelete(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *lightspeedv1beta1.OpenStackLightspeed,
) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)

	olsConfig, err := lightspeed.GetOLSConfig(ctx, helper)
	if err != nil && k8s_errors.IsNotFound(err) {
		controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, err
	}

	ownerLabel := olsConfig.GetLabels()[lightspeed.OpenStackLightspeedOwnerIDLabel]
	if ownerLabel == "" || ownerLabel != string(instance.GetObjectMeta().GetUID()) {
		Log.Info("Skipping OLSConfig deletion as it is not managed by the OpenStackLightspeed instance")
		controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
		return ctrl.Result{}, nil
	}

	_, err = controllerutil.CreateOrPatch(ctx, r.Client, &olsConfig, func() error {
		if ok := controllerutil.RemoveFinalizer(&olsConfig, helper.GetFinalizer()); !ok {
			return fmt.Errorf("remove finalizer failed")
		}

		return nil
	})
	if err != nil {
		return ctrl.Result{}, err
	}

	err = r.Client.Delete(ctx, &olsConfig)
	if err != nil {
		return ctrl.Result{}, err
	}

	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackLightspeedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	versionFunc := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		Log := r.GetLogger(ctx)
		versionList := &corev1beta1.OpenStackVersionList{}

		var result []reconcile.Request

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
		For(&lightspeedv1beta1.OpenStackLightspeed{}).
		Watches(&corev1beta1.OpenStackVersion{}, versionFunc).
		Complete(r)
}
