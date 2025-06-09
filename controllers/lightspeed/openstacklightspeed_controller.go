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
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs/status,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ols.openshift.io,resources=olsconfigs/finalizers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions,verbs=get;list;

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

	OLSOperatorInstalled, err := lightspeed.IsOLSOperatorInstalled(ctx, helper)
	if !OLSOperatorInstalled || err != nil {
		return ctrl.Result{}, fmt.Errorf("installation of OpenShift LightSpeed not detected: %w", err)
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

		err = lightspeed.PatchOLSConfig(&olsConfig, instance, helper)
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
	if ok := controllerutil.RemoveFinalizer(instance, helper.GetFinalizer()); !ok {
		return ctrl.Result{}, fmt.Errorf("remove finalizer failed")
	}

	olsConfig, err := lightspeed.GetOLSConfig(ctx, helper)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	} else if err != nil {
		return ctrl.Result{}, nil
	}

	ownerLabel := olsConfig.GetLabels()[lightspeed.OpenStackLightspeedOwnerIDLabel]
	if ownerLabel == "" {
		return ctrl.Result{}, fmt.Errorf("OLSConfig is not managed by OpenStackLightspeed instance")
	} else if ownerLabel != string(instance.GetObjectMeta().GetUID()) {
		return ctrl.Result{}, fmt.Errorf("OLSConfig is managed by different OpenStackLightspeed instance")
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

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackLightspeedReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&lightspeedv1beta1.OpenStackLightspeed{}).
		Complete(r)
}
