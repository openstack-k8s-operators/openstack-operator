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

// Package assistant contains the OpenStackAssistant controller implementation
package assistant

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

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/env"
	helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	common_rbac "github.com/openstack-k8s-operators/lib-common/modules/common/rbac"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/internal/openstackassistant"
)

const assistantFinalizer = "assistant.openstack.org/finalizer"

// OpenStackAssistantReconciler reconciles a OpenStackAssistant object
type OpenStackAssistantReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackAssistantReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackAssistant")
}

// +kubebuilder:rbac:groups=assistant.openstack.org,resources=openstackassistants,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=assistant.openstack.org,resources=openstackassistants/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=assistant.openstack.org,resources=openstackassistants/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete

// Reconcile -
func (r *OpenStackAssistantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)

	instance := &assistantv1.OpenStackAssistant{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			Log.Info("OpenStackAssistant CR not found")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	Log.Info("OpenStackAssistant CR values", "Name", instance.Name, "Namespace", instance.Namespace, "Image", instance.Spec.ContainerImage)

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

	// initialize status
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	savedConditions := instance.Status.Conditions.DeepCopy()

	defer func() {
		if r := recover(); r != nil {
			Log.Info(fmt.Sprintf("panic during reconcile %v\n", r))
			panic(r)
		}
		condition.RestoreLastTransitionTimes(&instance.Status.Conditions, savedConditions)
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, condition.ReadyMessage)
		} else {
			instance.Status.Conditions.MarkUnknown(
				condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage)
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	// Handle finalizer for ClusterRole cleanup
	if instance.DeletionTimestamp != nil {
		if controllerutil.ContainsFinalizer(instance, assistantFinalizer) {
			clusterRoleName := fmt.Sprintf("openstackassistant-%s-%s", instance.Namespace, instance.Name)
			if err := r.deleteClusterRBAC(ctx, clusterRoleName); err != nil {
				return ctrl.Result{}, err
			}
			controllerutil.RemoveFinalizer(instance, assistantFinalizer)
			if err := r.Update(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
			Log.Info("Finalizer removed, ClusterRole and ClusterRoleBinding cleaned up")
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(instance, assistantFinalizer) {
		controllerutil.AddFinalizer(instance, assistantFinalizer)
		if err := r.Update(ctx, instance); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	cl := condition.CreateList(
		condition.UnknownCondition(assistantv1.OpenStackAssistantReadyCondition, condition.InitReason, assistantv1.OpenStackAssistantReadyInitMessage),
		condition.UnknownCondition(condition.ServiceAccountReadyCondition, condition.InitReason, condition.ServiceAccountReadyInitMessage),
		condition.UnknownCondition(condition.RoleReadyCondition, condition.InitReason, condition.RoleReadyInitMessage),
		condition.UnknownCondition(condition.RoleBindingReadyCondition, condition.InitReason, condition.RoleBindingReadyInitMessage),
	)
	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	// Namespace RBAC
	rbacRules := namespacedRbacRules()
	rbacResult, err := common_rbac.ReconcileRbac(ctx, helper, instance, rbacRules)
	if err != nil {
		return rbacResult, err
	} else if (rbacResult != ctrl.Result{}) {
		return rbacResult, nil
	}

	// ClusterRole and ClusterRoleBinding
	clusterRoleName := fmt.Sprintf("openstackassistant-%s-%s", instance.Namespace, instance.Name)
	if err := r.reconcileClusterRBAC(ctx, instance, clusterRoleName); err != nil {
		return ctrl.Result{}, err
	}

	assistantLabels := map[string]string{
		common.AppSelector: "openstackassistant",
	}

	configVars := make(map[string]env.Setter)

	// Validate lightspeed ProviderSecret
	_, providerSecretHash, err := secret.GetSecret(ctx, helper, instance.Spec.LightspeedStack.ProviderSecret, instance.Namespace)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			instance.Status.Conditions.Set(condition.FalseCondition(
				assistantv1.OpenStackAssistantReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				assistantv1.OpenStackAssistantProviderSecretWaitingMessage))
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
		}
		instance.Status.Conditions.Set(condition.FalseCondition(
			assistantv1.OpenStackAssistantReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			assistantv1.OpenStackAssistantReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	configVars[instance.Spec.LightspeedStack.ProviderSecret] = env.SetValue(providerSecretHash)

	// Validate optional CaBundleSecret
	if instance.Spec.LightspeedStack.CaBundleSecretName != "" {
		_, caBundleHash, err := secret.GetSecret(ctx, helper, instance.Spec.LightspeedStack.CaBundleSecretName, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				instance.Status.Conditions.Set(condition.FalseCondition(
					assistantv1.OpenStackAssistantReadyCondition,
					condition.ErrorReason,
					condition.SeverityWarning,
					assistantv1.OpenStackAssistantReadyErrorMessage,
					fmt.Sprintf("CA bundle secret %s not found", instance.Spec.LightspeedStack.CaBundleSecretName)))
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, err
		}
		configVars[instance.Spec.LightspeedStack.CaBundleSecretName] = env.SetValue(caBundleHash)
	}

	// Validate optional Recipes ConfigMap
	if instance.Spec.Goose != nil && instance.Spec.Goose.Recipes != nil {
		_, recipesHash, err := configmap.GetConfigMapAndHashWithName(ctx, helper, *instance.Spec.Goose.Recipes, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				instance.Status.Conditions.Set(condition.FalseCondition(
					assistantv1.OpenStackAssistantReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					assistantv1.OpenStackAssistantRecipesWaitingMessage))
				return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
			}
			return ctrl.Result{}, err
		}
		configVars[*instance.Spec.Goose.Recipes] = env.SetValue(recipesHash)
	}

	// Validate optional Hints ConfigMap
	if instance.Spec.Goose != nil && instance.Spec.Goose.Hints != nil {
		_, hintsHash, err := configmap.GetConfigMapAndHashWithName(ctx, helper, *instance.Spec.Goose.Hints, instance.Namespace)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				instance.Status.Conditions.Set(condition.FalseCondition(
					assistantv1.OpenStackAssistantReadyCondition,
					condition.RequestedReason,
					condition.SeverityInfo,
					assistantv1.OpenStackAssistantHintsWaitingMessage))
				return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
			}
			return ctrl.Result{}, err
		}
		configVars[*instance.Spec.Goose.Hints] = env.SetValue(hintsHash)
	}

	// Create/update entrypoint ConfigMap
	entrypointCM := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-entrypoint",
			Namespace: instance.Namespace,
		},
	}
	_, err = controllerutil.CreateOrPatch(ctx, r.Client, entrypointCM, func() error {
		entrypointCM.Data = map[string]string{
			"entrypoint.sh": openstackassistant.EntrypointScript(),
		}
		return controllerutil.SetControllerReference(instance, entrypointCM, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("error creating entrypoint ConfigMap: %w", err)
	}

	// Compute composite config hash
	configVarsHash, err := util.HashOfInputHashes(configVars)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Build PodSpec
	spec := openstackassistant.AssistantPodSpec(instance, configVarsHash)

	podSpecHash, err := util.ObjectHash(spec)
	if err != nil {
		return ctrl.Result{}, err
	}

	podSpecHashName := "podSpec"

	// Create/update Pod
	assistantPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, r.Client, assistantPod, func() error {
		isPodUpdate := !assistantPod.CreationTimestamp.IsZero()
		currentPodSpecHash := instance.Status.Hash[podSpecHashName]
		if !isPodUpdate || currentPodSpecHash != podSpecHash {
			assistantPod.Spec = spec
		}
		assistantPod.Labels = util.MergeStringMaps(assistantPod.Labels, assistantLabels)

		return controllerutil.SetControllerReference(instance, assistantPod, r.Scheme)
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
			if err := r.Delete(ctx, assistantPod); err != nil && !k8s_errors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("error deleting OpenStackAssistant pod %s: %w", assistantPod.Name, err)
			}
			Log.Info(fmt.Sprintf("OpenStackAssistant pod deleted due to change %s", err.Error()))

			return ctrl.Result{Requeue: true}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to create or update pod %s: %w", assistantPod.Name, err)
	}

	instance.Status.Hash, _ = util.SetHash(instance.Status.Hash, podSpecHashName, podSpecHash)
	instance.Status.PodName = assistantPod.Name

	if op != controllerutil.OperationResultNone {
		util.LogForObject(
			helper,
			fmt.Sprintf("Pod %s successfully reconciled - operation: %s", assistantPod.Name, string(op)),
			instance,
		)
	}

	// Force-delete pods stuck in Terminating >3 minutes
	if assistantPod.DeletionTimestamp != nil {
		terminatingDuration := time.Since(assistantPod.DeletionTimestamp.Time)
		if terminatingDuration > time.Minute*3 {
			err := r.Delete(ctx, assistantPod, client.GracePeriodSeconds(0))
			if err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to force delete pod: %w", err)
			}
		}
	}

	// Check pod readiness
	podReady := false
	for _, cond := range assistantPod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			podReady = true
			break
		}
	}

	if podReady {
		instance.Status.Conditions.MarkTrue(
			assistantv1.OpenStackAssistantReadyCondition,
			assistantv1.OpenStackAssistantReadyMessage,
		)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			assistantv1.OpenStackAssistantReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			assistantv1.OpenStackAssistantReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackAssistantReconciler) reconcileClusterRBAC(ctx context.Context, instance *assistantv1.OpenStackAssistant, clusterRoleName string) error {
	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
	}
	_, err := controllerutil.CreateOrPatch(ctx, r.Client, clusterRole, func() error {
		clusterRole.Rules = clusterRoleRules()
		return nil
	})
	if err != nil {
		return fmt.Errorf("error reconciling ClusterRole %s: %w", clusterRoleName, err)
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleName,
		},
	}
	_, err = controllerutil.CreateOrPatch(ctx, r.Client, clusterRoleBinding, func() error {
		clusterRoleBinding.RoleRef = rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     clusterRoleName,
		}
		clusterRoleBinding.Subjects = []rbacv1.Subject{{
			Kind:      "ServiceAccount",
			Name:      instance.RbacResourceName(),
			Namespace: instance.Namespace,
		}}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error reconciling ClusterRoleBinding %s: %w", clusterRoleName, err)
	}

	return nil
}

func (r *OpenStackAssistantReconciler) deleteClusterRBAC(ctx context.Context, name string) error {
	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := r.Delete(ctx, clusterRoleBinding); err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("error deleting ClusterRoleBinding %s: %w", name, err)
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
	if err := r.Delete(ctx, clusterRole); err != nil && !k8s_errors.IsNotFound(err) {
		return fmt.Errorf("error deleting ClusterRole %s: %w", name, err)
	}

	return nil
}

func namespacedRbacRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{
				"pods", "pods/log", "services", "endpoints",
				"configmaps", "secrets", "events",
				"persistentvolumeclaims", "serviceaccounts",
			},
			Verbs: []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments", "statefulsets", "daemonsets", "replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"batch"},
			Resources: []string{"jobs", "cronjobs"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"route.openshift.io"},
			Resources: []string{"routes"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"k8s.cni.cncf.io"},
			Resources: []string{"network-attachment-definitions"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"cert-manager.io"},
			Resources: []string{"certificates", "issuers"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{
				"core.openstack.org",
				"dataplane.openstack.org",
				"keystone.openstack.org",
				"mariadb.openstack.org",
				"memcached.openstack.org",
				"rabbitmq.openstack.org",
				"nova.openstack.org",
				"neutron.openstack.org",
				"glance.openstack.org",
				"cinder.openstack.org",
				"heat.openstack.org",
				"octavia.openstack.org",
				"designate.openstack.org",
				"barbican.openstack.org",
				"manila.openstack.org",
				"horizon.openstack.org",
				"swift.openstack.org",
				"placement.openstack.org",
				"ovn.openstack.org",
				"ironic.openstack.org",
				"telemetry.openstack.org",
				"network.openstack.org",
			},
			Resources: []string{"*"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
}

func clusterRoleRules() []rbacv1.PolicyRule {
	return []rbacv1.PolicyRule{
		{
			APIGroups: []string{""},
			Resources: []string{"nodes", "persistentvolumes", "namespaces"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"config.openshift.io"},
			Resources: []string{"clusteroperators", "clusterversions"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"nmstate.io"},
			Resources: []string{"nodenetworkconfigurationpolicies"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"machine.openshift.io"},
			Resources: []string{"machines", "machinesets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{"storage.k8s.io"},
			Resources: []string{"storageclasses"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}
}

// fields to index to reconcile when change
const (
	providerSecretField = ".spec.lightspeedStack.providerSecret"
	caBundleSecretField = ".spec.lightspeedStack.caBundleSecretName"
	recipesField        = ".spec.goose.recipes"
	hintsField          = ".spec.goose.hints"
)

var allWatchFields = []string{
	providerSecretField,
	caBundleSecretField,
	recipesField,
	hintsField,
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackAssistantReconciler) SetupWithManager(
	ctx context.Context, mgr ctrl.Manager) error {

	if err := mgr.GetFieldIndexer().IndexField(ctx, &assistantv1.OpenStackAssistant{}, providerSecretField, func(rawObj client.Object) []string {
		cr := rawObj.(*assistantv1.OpenStackAssistant)
		if cr.Spec.LightspeedStack.ProviderSecret == "" {
			return nil
		}
		return []string{cr.Spec.LightspeedStack.ProviderSecret}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &assistantv1.OpenStackAssistant{}, caBundleSecretField, func(rawObj client.Object) []string {
		cr := rawObj.(*assistantv1.OpenStackAssistant)
		if cr.Spec.LightspeedStack.CaBundleSecretName == "" {
			return nil
		}
		return []string{cr.Spec.LightspeedStack.CaBundleSecretName}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &assistantv1.OpenStackAssistant{}, recipesField, func(rawObj client.Object) []string {
		cr := rawObj.(*assistantv1.OpenStackAssistant)
		if cr.Spec.Goose == nil || cr.Spec.Goose.Recipes == nil || *cr.Spec.Goose.Recipes == "" {
			return nil
		}
		return []string{*cr.Spec.Goose.Recipes}
	}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(ctx, &assistantv1.OpenStackAssistant{}, hintsField, func(rawObj client.Object) []string {
		cr := rawObj.(*assistantv1.OpenStackAssistant)
		if cr.Spec.Goose == nil || cr.Spec.Goose.Hints == nil || *cr.Spec.Goose.Hints == "" {
			return nil
		}
		return []string{*cr.Spec.Goose.Hints}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&assistantv1.OpenStackAssistant{}).
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
		Complete(r)
}

func (r *OpenStackAssistantReconciler) findObjectsForSrc(ctx context.Context, src client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	Log := r.GetLogger(context.Background())

	for _, field := range allWatchFields {
		crList := &assistantv1.OpenStackAssistantList{}
		listOps := &client.ListOptions{
			FieldSelector: fields.OneTermEqualSelector(field, src.GetName()),
			Namespace:     src.GetNamespace(),
		}
		err := r.List(ctx, crList, listOps)
		if err != nil {
			Log.Error(err, fmt.Sprintf("listing %s for field: %s - %s", crList.GroupVersionKind().Kind, field, src.GetNamespace()))
			return requests
		}

		for _, item := range crList.Items {
			Log.Info(fmt.Sprintf("input source %s changed, reconcile: %s - %s", src.GetName(), item.GetName(), item.GetNamespace()))

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
