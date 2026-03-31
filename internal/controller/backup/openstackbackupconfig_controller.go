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

// Package backup contains the controller for OpenStackBackupConfig resources.
package backup

import (
	"context"
	stderrors "errors"
	"fmt"

	"github.com/go-logr/logr"
	backupv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/backup/v1beta1"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	"github.com/openstack-k8s-operators/lib-common/modules/common/backup"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"k8s.io/client-go/kubernetes"

	k8s_networkingv1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// OpenStackBackupConfigReconciler reconciles a OpenStackBackupConfig object
type OpenStackBackupConfigReconciler struct {
	client.Client
	Kclient       kubernetes.Interface
	Scheme        *runtime.Scheme
	CRDLabelCache backup.CRDLabelCache
}

// getGVKFromCRD looks up a CRD by name and returns its GVK
func (r *OpenStackBackupConfigReconciler) getGVKFromCRD(ctx context.Context, crdName string) (schema.GroupVersionKind, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := r.Get(ctx, types.NamespacedName{Name: crdName}, crd); err != nil {
		return schema.GroupVersionKind{}, err
	}

	// Find the served version (prefer storage version, fall back to first served)
	var version string
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			version = v.Name
			break
		}
		if v.Served && version == "" {
			version = v.Name
		}
	}

	return schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: version,
		Kind:    crd.Spec.Names.Kind,
	}, nil
}

// shouldLabelResource checks if a resource should be labeled based on ownerReferences and config
func shouldLabelResource(obj client.Object, config backupv1beta1.ResourceBackupConfig) bool {
	// Check if labeling is enabled (nil treated as enabled for backward compatibility)
	if config.Labeling != nil && *config.Labeling == backupv1beta1.BackupLabelingDisabled {
		return false
	}

	// Only label resources without ownerReferences (user-provided)
	if len(obj.GetOwnerReferences()) > 0 {
		return false
	}

	labels := obj.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	// Check exclude label keys
	for _, excludeKey := range config.ExcludeLabelKeys {
		if _, exists := labels[excludeKey]; exists {
			return false
		}
	}

	// Check exclude names
	for _, excludeName := range config.ExcludeNames {
		if obj.GetName() == excludeName {
			return false
		}
	}

	// Check include label selector (if specified, resource must match)
	if len(config.IncludeLabelSelector) > 0 {
		for key, value := range config.IncludeLabelSelector {
			if labels[key] != value {
				return false
			}
		}
	}

	return true
}

// getRestoreOrder returns the per-type restore order if set, otherwise the global default
func getRestoreOrder(config backupv1beta1.ResourceBackupConfig, defaultOrder string) string {
	if config.RestoreOrder != "" {
		return config.RestoreOrder
	}
	return defaultOrder
}

// hasBackupAnnotations returns true if the resource has any backup-related annotations
func hasBackupAnnotations(obj client.Object) bool {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return false
	}
	for _, key := range backup.LabelKeys() {
		if _, has := annotations[key]; has {
			return true
		}
	}
	return false
}

// labelResourceItems labels a list of resources with backup labels.
// Resources with ownerReferences are skipped unless they have annotation overrides.
// Resources that already have a restore label (set by operators at creation time,
// e.g. cert-manager secrets) are skipped unless they have annotation overrides.
func (r *OpenStackBackupConfigReconciler) labelResourceItems(
	ctx context.Context,
	log logr.Logger,
	items []client.Object,
	config backupv1beta1.ResourceBackupConfig,
	defaultLabels map[string]string,
) (int, error) {
	var errs []error
	count := 0
	for _, obj := range items {
		// Annotation overrides bypass all filtering
		if !hasBackupAnnotations(obj) {
			// Skip resources that already have a restore label (set by operators or previous reconcile)
			if restoreVal, hasRestoreLabel := obj.GetLabels()[backup.BackupRestoreLabel]; hasRestoreLabel {
				if restoreVal == "true" {
					count++
				}
				continue
			}
			if !shouldLabelResource(obj, config) {
				continue
			}
		}

		if _, err := backup.EnsureBackupLabels(ctx, r.Client, obj, defaultLabels); err != nil {
			log.Error(err, "Failed to label resource", "name", obj.GetName())
			errs = append(errs, fmt.Errorf("%s: %w", obj.GetName(), err))
			continue
		}
		count++
	}
	return count, stderrors.Join(errs...)
}

// labelSecrets labels secrets in the target namespace
func (r *OpenStackBackupConfigReconciler) labelSecrets(ctx context.Context, log logr.Logger, instance *backupv1beta1.OpenStackBackupConfig) (int, error) {
	list := &corev1.SecretList{}
	if err := r.List(ctx, list, client.InNamespace(instance.Namespace)); err != nil {
		return 0, err
	}
	items := make([]client.Object, len(list.Items))
	for i := range list.Items {
		items[i] = &list.Items[i]
	}
	defaultLabels := backup.GetRestoreLabels(getRestoreOrder(instance.Spec.Secrets, instance.Spec.DefaultRestoreOrder), "")
	return r.labelResourceItems(ctx, log, items, instance.Spec.Secrets, defaultLabels)
}

// labelConfigMaps labels configmaps in the target namespace
func (r *OpenStackBackupConfigReconciler) labelConfigMaps(ctx context.Context, log logr.Logger, instance *backupv1beta1.OpenStackBackupConfig) (int, error) {
	list := &corev1.ConfigMapList{}
	if err := r.List(ctx, list, client.InNamespace(instance.Namespace)); err != nil {
		return 0, err
	}
	items := make([]client.Object, len(list.Items))
	for i := range list.Items {
		items[i] = &list.Items[i]
	}
	defaultLabels := backup.GetRestoreLabels(getRestoreOrder(instance.Spec.ConfigMaps, instance.Spec.DefaultRestoreOrder), "")
	return r.labelResourceItems(ctx, log, items, instance.Spec.ConfigMaps, defaultLabels)
}

// labelNetworkAttachmentDefinitions labels NADs in the target namespace
func (r *OpenStackBackupConfigReconciler) labelNetworkAttachmentDefinitions(ctx context.Context, log logr.Logger, instance *backupv1beta1.OpenStackBackupConfig) (int, error) {
	list := &k8s_networkingv1.NetworkAttachmentDefinitionList{}
	if err := r.List(ctx, list, client.InNamespace(instance.Namespace)); err != nil {
		return 0, err
	}
	items := make([]client.Object, len(list.Items))
	for i := range list.Items {
		items[i] = &list.Items[i]
	}
	defaultLabels := backup.GetRestoreLabels(getRestoreOrder(instance.Spec.NetworkAttachmentDefinitions, instance.Spec.DefaultRestoreOrder), "")
	return r.labelResourceItems(ctx, log, items, instance.Spec.NetworkAttachmentDefinitions, defaultLabels)
}

// labelIssuers labels cert-manager Issuers in the target namespace
func (r *OpenStackBackupConfigReconciler) labelIssuers(ctx context.Context, log logr.Logger, instance *backupv1beta1.OpenStackBackupConfig) (int, error) {
	list := &certmgrv1.IssuerList{}
	if err := r.List(ctx, list, client.InNamespace(instance.Namespace)); err != nil {
		return 0, err
	}
	items := make([]client.Object, len(list.Items))
	for i := range list.Items {
		items[i] = &list.Items[i]
	}
	defaultLabels := backup.GetRestoreLabels(getRestoreOrder(instance.Spec.Issuers, instance.Spec.DefaultRestoreOrder), "")
	return r.labelResourceItems(ctx, log, items, instance.Spec.Issuers, defaultLabels)
}

// labelCRInstances labels CR instances based on CRD backup-restore labels
// This labels CRs like OpenStackControlPlane, OpenStackVersion, NetConfig, etc.
// based on their CRD's backup/restore configuration.
func (r *OpenStackBackupConfigReconciler) labelCRInstances(ctx context.Context, log logr.Logger, instance *backupv1beta1.OpenStackBackupConfig) (int, error) {
	// Fallback: build cache if not populated at setup time
	if len(r.CRDLabelCache) == 0 {
		cache, err := backup.BuildCRDLabelCache(ctx, r.Client)
		if err != nil {
			return 0, fmt.Errorf("failed to build CRD label cache: %w", err)
		}
		r.CRDLabelCache = cache
		log.Info("Built CRD label cache", "entries", len(cache))
	}

	count := 0

	// Iterate through all CRDs that have backup-restore enabled
	for crdName, backupConfig := range r.CRDLabelCache {
		if !backupConfig.Enabled {
			continue
		}

		// Look up the CRD to get proper group, version, and kind
		gvk, err := r.getGVKFromCRD(ctx, crdName)
		if err != nil {
			log.Error(err, "Failed to get CRD", "name", crdName)
			continue
		}

		// Create a metadata-only list for this CRD type
		list := &metav1.PartialObjectMetadataList{}
		list.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gvk.Group,
			Version: gvk.Version,
			Kind:    gvk.Kind + "List",
		})

		if err := r.List(ctx, list, client.InNamespace(instance.Namespace)); err != nil {
			log.Error(err, "Failed to list CR instances", "crd", crdName)
			continue
		}

		// Label each CR instance
		defaultLabels := backup.GetRestoreLabels(backupConfig.RestoreOrder, backupConfig.Category)
		for i := range list.Items {
			obj := &list.Items[i]

			if _, err := backup.EnsureBackupLabels(ctx, r.Client, obj, defaultLabels); err != nil {
				log.Error(err, "Failed to label CR instance", "kind", gvk.Kind, "name", obj.GetName())
				continue
			}
			count++
		}
	}

	return count, nil
}

// +kubebuilder:rbac:groups=backup.openstack.org,resources=openstackbackupconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=backup.openstack.org,resources=openstackbackupconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=backup.openstack.org,resources=openstackbackupconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources=customresourcedefinitions,verbs=get;list;watch
// RBAC for labeling CR instances across all openstack.org API groups.
// Kubernetes RBAC does not support wildcard group patterns (*.openstack.org),
// so each group must be listed explicitly.
// +kubebuilder:rbac:groups=barbican.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=baremetal.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=cinder.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=client.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=dataplane.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=designate.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=glance.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=heat.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=horizon.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=instanceha.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ironic.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=manila.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=memcached.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=network.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=neutron.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=nova.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=octavia.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=ovn.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=placement.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=rabbitmq.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=redis.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=swift.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=telemetry.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=topology.openstack.org,resources=*,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=watcher.openstack.org,resources=*,verbs=get;list;watch;update;patch

// Reconcile labels user-provided resources (without ownerReferences) for backup/restore.
func (r *OpenStackBackupConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the OpenStackBackupConfig instance
	instance := &backupv1beta1.OpenStackBackupConfig{}
	err := r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("OpenStackBackupConfig resource not found, ignoring")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get OpenStackBackupConfig")
		return ctrl.Result{}, err
	}

	h, err := helper.NewHelper(instance, r.Client, r.Kclient, r.Scheme, log)
	if err != nil {
		log.Error(err, "Failed to create helper")
		return ctrl.Result{}, err
	}

	//
	// initialize Conditions
	//
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
	}

	cl := condition.CreateList(
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
		condition.UnknownCondition(backupv1beta1.OpenStackBackupConfigSecretsReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(backupv1beta1.OpenStackBackupConfigConfigMapsReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(backupv1beta1.OpenStackBackupConfigNADsReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(backupv1beta1.OpenStackBackupConfigIssuersReadyCondition, condition.InitReason, condition.InitReason),
		condition.UnknownCondition(backupv1beta1.OpenStackBackupConfigCRsReadyCondition, condition.InitReason, condition.InitReason),
	)
	instance.Status.Conditions.Init(&cl)

	// Save a copy of the conditions for LastTransitionTime restore
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Always patch the instance status when exiting this function
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

		condition.RestoreLastTransitionTimes(&instance.Status.Conditions, savedConditions)
		if err := h.PatchInstance(ctx, instance); err != nil {
			_err = err
			return
		}
	}()

	// Label resources in target namespace — process all types and collect errors
	var reconcileErrs []error

	secretCount, err := r.labelSecrets(ctx, log, instance)
	if err != nil {
		log.Error(err, "Failed to label secrets")
		instance.Status.Conditions.Set(condition.FalseCondition(
			backupv1beta1.OpenStackBackupConfigSecretsReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			"Failed to label secrets: %v", err))
		reconcileErrs = append(reconcileErrs, err)
	} else {
		instance.Status.Conditions.Set(condition.TrueCondition(
			backupv1beta1.OpenStackBackupConfigSecretsReadyCondition,
			"Labeled %d secrets", secretCount))
	}

	configMapCount, err := r.labelConfigMaps(ctx, log, instance)
	if err != nil {
		log.Error(err, "Failed to label configmaps")
		instance.Status.Conditions.Set(condition.FalseCondition(
			backupv1beta1.OpenStackBackupConfigConfigMapsReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			"Failed to label configmaps: %v", err))
		reconcileErrs = append(reconcileErrs, err)
	} else {
		instance.Status.Conditions.Set(condition.TrueCondition(
			backupv1beta1.OpenStackBackupConfigConfigMapsReadyCondition,
			"Labeled %d configmaps", configMapCount))
	}

	nadCount, err := r.labelNetworkAttachmentDefinitions(ctx, log, instance)
	if err != nil {
		log.Error(err, "Failed to label network-attachment-definitions")
		instance.Status.Conditions.Set(condition.FalseCondition(
			backupv1beta1.OpenStackBackupConfigNADsReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			"Failed to label network-attachment-definitions: %v", err))
		reconcileErrs = append(reconcileErrs, err)
	} else {
		instance.Status.Conditions.Set(condition.TrueCondition(
			backupv1beta1.OpenStackBackupConfigNADsReadyCondition,
			"Labeled %d NADs", nadCount))
	}

	issuerCount, err := r.labelIssuers(ctx, log, instance)
	if err != nil {
		log.Error(err, "Failed to label issuers")
		instance.Status.Conditions.Set(condition.FalseCondition(
			backupv1beta1.OpenStackBackupConfigIssuersReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			"Failed to label issuers: %v", err))
		reconcileErrs = append(reconcileErrs, err)
	} else {
		instance.Status.Conditions.Set(condition.TrueCondition(
			backupv1beta1.OpenStackBackupConfigIssuersReadyCondition,
			"Labeled %d issuers", issuerCount))
	}

	// Label CR instances based on CRD backup-restore labels
	crCount, err := r.labelCRInstances(ctx, log, instance)
	if err != nil {
		log.Error(err, "Failed to label CR instances")
		instance.Status.Conditions.Set(condition.FalseCondition(
			backupv1beta1.OpenStackBackupConfigCRsReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			"Failed to label CR instances: %v", err))
		reconcileErrs = append(reconcileErrs, err)
	} else {
		instance.Status.Conditions.Set(condition.TrueCondition(
			backupv1beta1.OpenStackBackupConfigCRsReadyCondition,
			"Labeled %d CRs", crCount))
	}

	// Update status counts
	instance.Status.LabeledResources.Secrets = secretCount
	instance.Status.LabeledResources.ConfigMaps = configMapCount
	instance.Status.LabeledResources.NetworkAttachmentDefinitions = nadCount
	instance.Status.LabeledResources.Issuers = issuerCount

	if len(reconcileErrs) > 0 {
		return ctrl.Result{}, stderrors.Join(reconcileErrs...)
	}

	log.Info("Successfully labeled resources", "secrets", secretCount, "configmaps", configMapCount, "nads", nadCount, "issuers", issuerCount, "crs", crCount)
	return ctrl.Result{}, nil
}

// backupLabelKeys are the label keys managed by this controller.
var backupLabelKeys = []string{
	backup.BackupLabel,
	backup.BackupRestoreLabel,
	backup.BackupRestoreOrderLabel,
	backup.BackupCategoryLabel,
}

// needsBackupLabeling returns true if a resource does not yet have backup labels.
func needsBackupLabeling(labels map[string]string) bool {
	_, hasBackup := labels[backup.BackupLabel]
	_, hasRestore := labels[backup.BackupRestoreLabel]
	return !hasBackup && !hasRestore
}

// backupAnnotationsChanged returns true if backup-related annotations differ between old and new.
func backupAnnotationsChanged(oldAnnotations, newAnnotations map[string]string) bool {
	for _, key := range backup.LabelKeys() {
		if oldAnnotations[key] != newAnnotations[key] {
			return true
		}
	}
	return false
}

// backupLabelsRemoved returns true if any backup labels were present on old but removed from new.
func backupLabelsRemoved(oldLabels, newLabels map[string]string) bool {
	for _, key := range backupLabelKeys {
		if _, hadIt := oldLabels[key]; hadIt {
			if _, hasIt := newLabels[key]; !hasIt {
				return true
			}
		}
	}
	return false
}

// backupResourcePredicate filters events to only reconcile when backup labeling is needed.
// Triggers on:
//   - Create: resource has no backup labels yet
//   - Update: backup annotations changed OR backup labels were removed
//
// Ignores deletes and generic events entirely.
var backupResourcePredicate = predicate.Funcs{
	CreateFunc: func(e event.CreateEvent) bool {
		return needsBackupLabeling(e.Object.GetLabels())
	},
	UpdateFunc: func(e event.UpdateEvent) bool {
		return backupAnnotationsChanged(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations()) ||
			backupLabelsRemoved(e.ObjectOld.GetLabels(), e.ObjectNew.GetLabels())
	},
	DeleteFunc: func(_ event.DeleteEvent) bool {
		return false
	},
	GenericFunc: func(_ event.GenericEvent) bool {
		return false
	},
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackBackupConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	Log := ctrl.Log.WithName("backup").WithName("setup")

	// findBackupConfigForSrc maps a resource back to the BackupConfig that should process it
	findBackupConfigForSrc := func(ctx context.Context, obj client.Object) []reconcile.Request {
		configList := &backupv1beta1.OpenStackBackupConfigList{}
		if err := mgr.GetClient().List(ctx, configList, client.InNamespace(obj.GetNamespace())); err != nil {
			return []reconcile.Request{}
		}

		requests := make([]reconcile.Request, len(configList.Items))
		for i, config := range configList.Items {
			requests[i] = reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      config.GetName(),
					Namespace: config.GetNamespace(),
				},
			}
		}
		return requests
	}

	bldr := ctrl.NewControllerManagedBy(mgr).
		For(&backupv1beta1.OpenStackBackupConfig{}).
		Watches(&corev1.Secret{}, handler.EnqueueRequestsFromMapFunc(findBackupConfigForSrc), builder.WithPredicates(backupResourcePredicate)).
		Watches(&corev1.ConfigMap{}, handler.EnqueueRequestsFromMapFunc(findBackupConfigForSrc), builder.WithPredicates(backupResourcePredicate)).
		Watches(&k8s_networkingv1.NetworkAttachmentDefinition{}, handler.EnqueueRequestsFromMapFunc(findBackupConfigForSrc), builder.WithPredicates(backupResourcePredicate)).
		Watches(&certmgrv1.Issuer{}, handler.EnqueueRequestsFromMapFunc(findBackupConfigForSrc), builder.WithPredicates(backupResourcePredicate))

	// Build CRD label cache and add watches for CRD instance types.
	// Uses the API reader since the manager's cache is not started yet.
	apiReader := mgr.GetAPIReader()
	cache, err := buildCRDLabelCacheFromReader(apiReader)
	if err != nil {
		Log.Error(err, "Failed to build CRD label cache, CR instances will not be watched")
	} else {
		r.CRDLabelCache = cache
		for crdName := range cache {
			gvk, err := getGVKFromCRDUsingReader(apiReader, crdName)
			if err != nil {
				Log.Error(err, "Failed to get GVK for CRD, skipping watch", "crd", crdName)
				continue
			}
			obj := &metav1.PartialObjectMetadata{}
			obj.SetGroupVersionKind(gvk)
			bldr = bldr.Watches(obj, handler.EnqueueRequestsFromMapFunc(findBackupConfigForSrc), builder.WithPredicates(backupResourcePredicate))
			Log.Info("Added watch for CRD instances", "crd", crdName, "gvk", gvk)
		}
	}

	return bldr.Named("openstackbackupconfig").Complete(r)
}

// buildCRDLabelCacheFromReader builds the CRD label cache using a client.Reader.
// Used at setup time when the manager's cache is not started.
func buildCRDLabelCacheFromReader(reader client.Reader) (backup.CRDLabelCache, error) {
	cache := make(backup.CRDLabelCache)

	crdList := &apiextensionsv1.CustomResourceDefinitionList{}
	if err := reader.List(context.Background(), crdList); err != nil {
		return nil, err
	}

	for _, crd := range crdList.Items {
		labels := crd.GetLabels()
		if labels == nil || labels[backup.BackupRestoreLabel] != "true" {
			continue
		}
		cache[crd.Name] = backup.Config{
			Enabled:      true,
			RestoreOrder: labels[backup.BackupRestoreOrderLabel],
			Category:     labels[backup.BackupCategoryLabel],
		}
	}

	return cache, nil
}

// getGVKFromCRDUsingReader looks up a CRD by name using a reader and returns its GVK.
// Used at setup time when the manager's cache is not started.
func getGVKFromCRDUsingReader(reader client.Reader, crdName string) (schema.GroupVersionKind, error) {
	crd := &apiextensionsv1.CustomResourceDefinition{}
	if err := reader.Get(context.Background(), types.NamespacedName{Name: crdName}, crd); err != nil {
		return schema.GroupVersionKind{}, err
	}

	var version string
	for _, v := range crd.Spec.Versions {
		if v.Storage {
			version = v.Name
			break
		}
		if v.Served && version == "" {
			version = v.Name
		}
	}

	return schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: version,
		Kind:    crd.Spec.Names.Kind,
	}, nil
}
