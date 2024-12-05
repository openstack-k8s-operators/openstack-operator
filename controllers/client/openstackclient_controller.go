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

package client

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstackclient"
)

// OpenStackClientReconciler reconciles a OpenStackClient object
type OpenStackClientReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLog returns a logger object with a prefix of "conroller.name" and aditional controller context fields
func (r *OpenStackClientReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackClient")
}

// +kubebuilder:rbac:groups=client.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=client.openstack.org,resources=openstackclients/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=client.openstack.org,resources=openstackclients/finalizers,verbs=update
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch
// +kubebuilder:rbac:groups=telemetry.openstack.org,resources=metricstorages,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;
// service account, role, rolebinding
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch
// service account permissions that are needed to grant permission to the above
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch;patch

// Reconcile -
func (r *OpenStackClientReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)

	instance := &clientv1.OpenStackClient{}
	err := r.Client.Get(context.TODO(), req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			Log.Info("OpenStackClient CR not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	Log.Info("OpenStackClient CR values", "Name", instance.Name, "Namespace", instance.Namespace, "Secret", instance.Spec.OpenStackConfigSecret, "Image", instance.Spec.ContainerImage)

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	//
	// initialize status
	//
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the condtions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
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
			_err = err
			return
		}
	}()

	cl := condition.CreateList(
		condition.UnknownCondition(clientv1.OpenStackClientReadyCondition, condition.InitReason, clientv1.OpenStackClientReadyInitMessage),
		// service account, role, rolebinding conditions
		condition.UnknownCondition(condition.ServiceAccountReadyCondition, condition.InitReason, condition.ServiceAccountReadyInitMessage),
		condition.UnknownCondition(condition.RoleReadyCondition, condition.InitReason, condition.RoleReadyInitMessage),
		condition.UnknownCondition(condition.RoleBindingReadyCondition, condition.InitReason, condition.RoleBindingReadyInitMessage),
	)
	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	// Service account, role, binding
	rbacRules := []rbacv1.PolicyRule{
		{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{"anyuid"},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"create", "get", "list", "watch", "update", "patch", "delete"},
		},
	}
	rbacResult, err := common_rbac.ReconcileRbac(ctx, helper, instance, rbacRules)
	if err != nil {
		return rbacResult, err
	} else if (rbacResult != ctrl.Result{}) {
		return rbacResult, nil
	}

	//
	// Validate that keystoneAPI is up
	//
	keystoneAPI, err := keystonev1.GetKeystoneAPI(ctx, helper, instance.Namespace, map[string]string{})
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				clientv1.OpenStackClientReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				clientv1.OpenStackClientKeystoneWaitingMessage))
			Log.Info("KeystoneAPI not found!")
			return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			clientv1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			clientv1.OpenStackClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if !keystoneAPI.IsReady() {
		instance.Status.Conditions.Set(condition.FalseCondition(
			clientv1.OpenStackClientReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			clientv1.OpenStackClientKeystoneWaitingMessage))
		Log.Info("KeystoneAPI not yet ready")
		return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
	}

	clientLabels := map[string]string{
		common.AppSelector: "openstackclient",
	}

	configVars := make(map[string]env.Setter)

	_, configMapHash, err := configmap.GetConfigMapAndHashWithName(ctx, helper, *instance.Spec.OpenStackConfigMap, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				clientv1.OpenStackClientReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				clientv1.OpenStackClientConfigMapWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			clientv1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			clientv1.OpenStackClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	configVars[*instance.Spec.OpenStackConfigMap] = env.SetValue(configMapHash)

	_, secretHash, err := secret.GetSecret(ctx, helper, *instance.Spec.OpenStackConfigSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				clientv1.OpenStackClientReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				clientv1.OpenStackClientSecretWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			clientv1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			clientv1.OpenStackClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	configVars[*instance.Spec.OpenStackConfigSecret] = env.SetValue(secretHash)

	if instance.Spec.CaBundleSecretName != "" {
		secretHash, err := tls.ValidateCACertSecret(
			ctx,
			helper.GetClient(),
			types.NamespacedName{
				Name:      instance.Spec.CaBundleSecretName,
				Namespace: instance.Namespace,
			},
		)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				instance.Status.Conditions.Set(condition.FalseCondition(
					condition.TLSInputReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					fmt.Sprintf(condition.TLSInputReadyWaitingMessage, instance.Spec.CaBundleSecretName)))
				return ctrl.Result{}, nil
			}
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.TLSInputReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.TLSInputErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}

		configVars[instance.Spec.CaBundleSecretName] = env.SetValue(secretHash)
	}

	metricStorage := &telemetryv1.MetricStorage{}
	err = helper.GetClient().Get(ctx, client.ObjectKey{
		Namespace: instance.Namespace,
		Name:      telemetryv1.DefaultServiceName,
	}, metricStorage)
	if err == nil {
		configVars["PrometheusTls"] = env.SetValue(fmt.Sprint(metricStorage.Spec.PrometheusTLS.Enabled()))
	}

	// all cert input checks out so report InputReady
	instance.Status.Conditions.MarkTrue(condition.TLSInputReadyCondition, condition.InputReadyMessage)

	configVarsHash, err := util.HashOfInputHashes(configVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	osclient := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	spec := openstackclient.ClientPodSpec(ctx, instance, helper, configVarsHash)

	podSpecHash, err := util.ObjectHash(spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	podSpecHashName := "podSpec"

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, osclient, func() error {
		isPodUpdate := !osclient.ObjectMeta.CreationTimestamp.IsZero()
		currentPodSpecHash := instance.Status.Hash[podSpecHashName]
		if !isPodUpdate || currentPodSpecHash != podSpecHash {
			osclient.Spec = spec
		}
		osclient.Labels = util.MergeStringMaps(osclient.Labels, clientLabels)

		err = controllerutil.SetControllerReference(instance, osclient, r.Scheme)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		var forbiddenPodSpecChangeErr *k8s_errors.StatusError

		forbiddenPodSpec := false
		if errors.As(err, &forbiddenPodSpecChangeErr) {
			if forbiddenPodSpecChangeErr.ErrStatus.Reason == metav1.StatusReasonForbidden {
				forbiddenPodSpec = true
			}
		}

		if forbiddenPodSpec || k8s_errors.IsInvalid(err) {
			// Delete pod when its config changed. In this case we just re-create the
			// openstackclient pod
			if err := r.Delete(ctx, osclient); err != nil && !k8s_errors.IsNotFound(err) {
				// Error deleting the object
				return ctrl.Result{}, fmt.Errorf("Error deleting OpenStackClient pod %s: %w", osclient.Name, err)
			}
			Log.Info(fmt.Sprintf("OpenStackClient pod deleted due to change %s", err.Error()))

			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, fmt.Errorf("Failed to create or update pod %s: %w", osclient.Name, err)
	}

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			clientv1.OpenStackClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.DeploymentReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	instance.Status.Hash, _ = util.SetHash(instance.Status.Hash, podSpecHashName, podSpecHash)

	if op != controllerutil.OperationResultNone {
		util.LogForObject(
			helper,
			fmt.Sprintf("Pod %s successfully reconciled - operation: %s", osclient.Name, string(op)),
			instance,
		)
	}

	podReady := false

	for _, condition := range osclient.Status.Conditions {
		if condition.Type == corev1.PodReady && condition.Status == corev1.ConditionTrue {
			podReady = true
			break
		}
	}

	if podReady {
		instance.Status.Conditions.MarkTrue(
			clientv1.OpenStackClientReadyCondition,
			clientv1.OpenStackClientReadyMessage,
		)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			clientv1.OpenStackClientReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			clientv1.OpenStackClientReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}

// fields to index to reconcile when change
const (
	caBundleSecretNameField    = ".spec.caBundleSecretName"
	openStackConfigMapField    = ".spec.openStackConfigMap"
	openStackConfigSecretField = ".spec.openStackConfigSecret"
)

var allWatchFields = []string{
	caBundleSecretNameField,
	openStackConfigMapField,
	openStackConfigSecretField,
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackClientReconciler) SetupWithManager(
	ctx context.Context, mgr ctrl.Manager) error {
	// index caBundleSecretNameField
	if err := mgr.GetFieldIndexer().IndexField(ctx, &clientv1.OpenStackClient{}, caBundleSecretNameField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*clientv1.OpenStackClient)
		if cr.Spec.CaBundleSecretName == "" {
			return nil
		}
		return []string{cr.Spec.CaBundleSecretName}
	}); err != nil {
		return err
	}
	// index openStackConfigMap
	if err := mgr.GetFieldIndexer().IndexField(ctx, &clientv1.OpenStackClient{}, openStackConfigMapField, func(rawObj client.Object) []string {
		// Extract the configmap name from the spec, if one is provided
		cr := rawObj.(*clientv1.OpenStackClient)
		if cr.Spec.OpenStackConfigMap == nil {
			return nil
		}
		if *cr.Spec.OpenStackConfigMap == "" {
			return nil
		}
		return []string{*cr.Spec.OpenStackConfigMap}
	}); err != nil {
		return err
	}
	// index openStackConfigSecret
	if err := mgr.GetFieldIndexer().IndexField(ctx, &clientv1.OpenStackClient{}, openStackConfigSecretField, func(rawObj client.Object) []string {
		// Extract the configmap name from the spec, if one is provided
		cr := rawObj.(*clientv1.OpenStackClient)
		if cr.Spec.OpenStackConfigSecret == nil {
			return nil
		}
		if *cr.Spec.OpenStackConfigSecret == "" {
			return nil
		}
		return []string{*cr.Spec.OpenStackConfigSecret}
	}); err != nil {
		return err
	}

	Log := r.GetLogger(ctx)
	metricStorageFn := func(ctx context.Context, o client.Object) []reconcile.Request {
		result := []reconcile.Request{}

		// get all openstackclient CRs
		openstackclients := &clientv1.OpenStackClientList{}
		listOpts := []client.ListOption{
			client.InNamespace(o.GetNamespace()),
		}
		if err := r.Client.List(ctx, openstackclients, listOpts...); err != nil {
			Log.Error(err, "Unable to retrieve OpenstackClient CRs %v")
			return nil
		}
		for _, cr := range openstackclients.Items {
			name := client.ObjectKey{
				Namespace: o.GetNamespace(),
				Name:      cr.Name,
			}
			Log.Info(fmt.Sprintf("OpenStackClient %s will be reconciled, because a MetricStorage %s changed", cr.Name, o.GetName()))
			result = append(result, reconcile.Request{NamespacedName: name})
		}
		if len(result) > 0 {
			return result
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&clientv1.OpenStackClient{}).
		Owns(&corev1.Pod{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&rbacv1.Role{}).
		Owns(&rbacv1.RoleBinding{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSrc),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSrc),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			// Reconcile all openstackclients when a MetricStorage changes.
			// This is needed to ensure the observability client is
			// configured correctly when tls is enabled or disabled.
			&telemetryv1.MetricStorage{},
			handler.EnqueueRequestsFromMapFunc(metricStorageFn),
		).
		Complete(r)
}

func (r *OpenStackClientReconciler) findObjectsForSrc(ctx context.Context, src client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	l := log.FromContext(context.Background()).WithName("Controllers").WithName("OpenStackClient")

	for _, field := range allWatchFields {
		crList := &clientv1.OpenStackClientList{}
		listOps := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(field, src.GetName()),
			Namespace:     src.GetNamespace(),
		}
		err := r.List(ctx, crList, listOps)
		if err != nil {
			l.Error(err, fmt.Sprintf("listing %s for field: %s - %s", crList.GroupVersionKind().Kind, field, src.GetNamespace()))
			return requests
		}

		for _, item := range crList.Items {
			l.Info(fmt.Sprintf("input source %s changed, reconcile: %s - %s", src.GetName(), item.GetName(), item.GetNamespace()))

			requests = append(requests,
				reconcile.Request{
					NamespacedName: types.NamespacedName{
						Name:      item.GetName(),
						Namespace: item.GetNamespace(),
					},
				},
			)
		}
	}

	return requests
}
