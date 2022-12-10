/*
Copyright 2022.

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
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	labels "github.com/openstack-k8s-operators/lib-common/modules/common/labels"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"
)

// OpenStackClientReconciler reconciles a OpenStackClient object
type OpenStackClientReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
	Log     logr.Logger
}

//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackclients/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackclients/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=pods,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *OpenStackClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	_ = r.Log.WithValues("openstackclient", req.NamespacedName)

	instance := &corev1beta1.OpenStackClient{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			r.Log.Info("OpenStackClient CR not found", "Name", instance.Name, "Namespace", instance.Namespace)
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	r.Log.Info("OpenStackClient CR values", "Name", instance.Name, "Namespace", instance.Namespace, "Secret", instance.Spec.OpenStackConfigSecret, "Image", instance.Spec.ContainerImage)

	instance.Status.Conditions = condition.Conditions{}
	cl := condition.CreateList(
		condition.UnknownCondition(
			corev1beta1.OpenStackClientReadyCondition,
			condition.InitReason,
			corev1beta1.OpenStackClientReadyInitMessage,
		),
	)
	instance.Status.Conditions.Init(&cl)

	h, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		if instance.Status.Conditions.IsTrue(corev1beta1.OpenStackClientReadyCondition) {
			instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		}

		err := h.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	//
	// Validate that keystoneAPI is up
	//
	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, h, instance.Namespace, map[string]string{})
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackClientReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackClientKeystoneWaitingMessage))
			r.Log.Info("KeystoneAPI not found!")
			return ctrl.Result{RequeueAfter: time.Second * 5}, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if !keystoneAPI.IsReady() {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackClientReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackClientKeystoneWaitingMessage))
		r.Log.Info("KeystoneAPI not yet ready")
		return ctrl.Result{RequeueAfter: time.Second * 5}, nil
	}

	_, configMapHash, err := configmap.GetConfigMapAndHashWithName(ctx, h, instance.Spec.OpenStackConfigMap, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackClientReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackClientConfigMapWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	_, secretHash, err := secret.GetSecret(ctx, h, instance.Spec.OpenStackConfigSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackClientReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackClientSecretWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Second * 10}, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	instance.Status.Conditions.Set(condition.FalseCondition(
		corev1beta1.OpenStackClientReadyCondition,
		condition.RequestedReason,
		condition.SeverityInfo,
		corev1beta1.OpenStackClientInputReady))

	//
	// create cm holding deployment script and render deployment script.
	//
	cmLabels := labels.GetLabels(instance, labels.GetGroupLabel("openstackclient"), map[string]string{})
	envVars := make(map[string]env.Setter)

	cms := []util.Template{
		// ScriptsConfigMap
		{
			Name:               "openstackclient-sh",
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeScripts,
			InstanceType:       instance.Kind,
			AdditionalTemplate: map[string]string{},
			Labels:             cmLabels,
		},
	}
	err = configmap.EnsureConfigMaps(ctx, h, instance, cms, &envVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	clientLabels := map[string]string{
		"app": "openstackclient",
	}
	pod := openstack.ClientPod(instance, clientLabels, configMapHash, secretHash)

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, pod, func() error {
		pod.Spec.Containers[0].Image = instance.Spec.ContainerImage
		err := controllerutil.SetControllerReference(instance, pod, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	if op != controllerutil.OperationResultNone {
		util.LogForObject(
			h,
			fmt.Sprintf("Pod %s successfully reconciled - operation: %s", pod.Name, string(op)),
			instance,
		)
	}

	instance.Status.Conditions.MarkTrue(
		corev1beta1.OpenStackClientReadyCondition,
		corev1beta1.OpenStackClientReadyMessage,
	)

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackClientReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.OpenStackClient{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ConfigMap{}).
		Complete(r)
}
