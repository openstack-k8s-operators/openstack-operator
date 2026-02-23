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

package dataplane

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/go-logr/logr"
	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/rolebinding"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/serviceaccount"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	deployment "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane/util"

	machineconfig "github.com/openshift/api/machineconfiguration/v1"
)

const (
	// AnsibleSSHPrivateKey ssh private key
	AnsibleSSHPrivateKey = "ssh-privatekey"
	// AnsibleSSHAuthorizedKeys authorized keys
	AnsibleSSHAuthorizedKeys = "authorized_keys"
)

// OpenStackDataPlaneNodeSetReconciler reconciles a OpenStackDataPlaneNodeSet object
type OpenStackDataPlaneNodeSetReconciler struct {
	client.Client
	APIReader  client.Reader // Direct API reader that bypasses cache
	Kclient    kubernetes.Interface
	Scheme     *runtime.Scheme
	Controller controller.Controller
	Cache      cache.Cache
	Watching   map[string]bool
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackDataPlaneNodeSetReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackDataPlaneNodeSet")
}

// +kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplaneservices,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplaneservices/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=baremetal.openstack.org,resources=openstackbaremetalsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=baremetal.openstack.org,resources=openstackbaremetalsets/status,verbs=get
// +kubebuilder:rbac:groups=baremetal.openstack.org,resources=openstackbaremetalsets/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=k8s.cni.cncf.io,resources=network-attachment-definitions,verbs=get;list;watch
// +kubebuilder:rbac:groups=network.openstack.org,resources=ipsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.openstack.org,resources=ipsets/status,verbs=get
// +kubebuilder:rbac:groups=network.openstack.org,resources=ipsets/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=network.openstack.org,resources=netconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=network.openstack.org,resources=dnsmasqs,verbs=get;list;watch
// +kubebuilder:rbac:groups=network.openstack.org,resources=dnsmasqs/status,verbs=get
// +kubebuilder:rbac:groups=network.openstack.org,resources=dnsdata,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.openstack.org,resources=dnsdata/status,verbs=get
// +kubebuilder:rbac:groups=network.openstack.org,resources=dnsdata/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions,verbs=get;list;watch

// RBAC for the ServiceAccount for the internal image registry
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=get;list;watch;create;update;patch
// +kubebuilder:rbac:groups="security.openshift.io",resourceNames=anyuid,resources=securitycontextconstraints,verbs=use
// +kubebuilder:rbac:groups="",resources=pods,verbs=create;delete;get;list;patch;update;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get
// +kubebuilder:rbac:groups="",resources=projects,verbs=get
// +kubebuilder:rbac:groups="project.openshift.io",resources=projects,verbs=get
// +kubebuilder:rbac:groups="",resources=imagestreamimages,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=imagestreammappings,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=imagestreams,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=imagestreams/layers,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=imagestreamtags,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=imagetags,verbs=get;list;watch
// +kubebuilder:rbac:groups="image.openshift.io",resources=imagestreamimages,verbs=get;list;watch
// +kubebuilder:rbac:groups="image.openshift.io",resources=imagestreammappings,verbs=get;list;watch
// +kubebuilder:rbac:groups="image.openshift.io",resources=imagestreams,verbs=get;list;watch
// +kubebuilder:rbac:groups="image.openshift.io",resources=imagestreams/layers,verbs=get
// +kubebuilder:rbac:groups="image.openshift.io",resources=imagetags,verbs=get;list;watch
// +kubebuilder:rbac:groups="image.openshift.io",resources=imagestreamtags,verbs=get;list;watch

// RBAC for ImageContentSourcePolicy and MachineConfig
// +kubebuilder:rbac:groups="operator.openshift.io",resources=imagecontentsourcepolicies,verbs=get;list;watch
// +kubebuilder:rbac:groups="config.openshift.io",resources=imagedigestmirrorsets,verbs=get;list;watch
// +kubebuilder:rbac:groups="config.openshift.io",resources=images,verbs=get;list;watch
// +kubebuilder:rbac:groups="machineconfiguration.openshift.io",resources=machineconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=rabbitmq.openstack.org,resources=rabbitmqusers,verbs=get;list;watch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the OpenStackDataPlaneNodeSet object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *OpenStackDataPlaneNodeSetReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)
	Log.Info("Reconciling NodeSet")

	// Try to set up MachineConfig watch if not already done
	// This is done conditionally because MachineConfig CRD may not exist on all clusters
	r.ensureMachineConfigWatch(ctx)

	validate := validator.New()

	// Fetch the OpenStackDataPlaneNodeSet instance
	instance := &dataplanev1.OpenStackDataPlaneNodeSet{}
	err := r.Get(ctx, req.NamespacedName, instance)
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

	helper, _ := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)

	// initialize status if Conditions is nil, but do not reset if it already
	// exists
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the conditions so that we can restore the LastTransitionTime
	// when a condition's state doesn't change.
	savedConditions := instance.Status.Conditions.DeepCopy()

	// Reset all conditions to Unknown as the state is not yet known for
	// this reconcile loop.
	instance.InitConditions()
	// Set ObservedGeneration since we've reset conditions
	instance.Status.ObservedGeneration = instance.Generation

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() { // update the Ready condition based on the sub conditions
		// Don't update the status, if reconciler Panics
		if r := recover(); r != nil {
			Log.Info(fmt.Sprintf("panic during reconcile %v\n", r))
			panic(r)
		}
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, dataplanev1.NodeSetReadyMessage)
		} else if instance.Status.Conditions.IsUnknown(condition.ReadyCondition) {
			// Recalculate ReadyCondition based on the state of the rest of the conditions
			instance.Status.Conditions.Set(
				instance.Status.Conditions.Mirror(condition.ReadyCondition))
		}
		condition.RestoreLastTransitionTimes(
			&instance.Status.Conditions, savedConditions)

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			Log.Error(err, "Error updating instance status conditions")
			_err = err
			return
		}
	}()

	if instance.Status.ConfigMapHashes == nil {
		instance.Status.ConfigMapHashes = make(map[string]string)
	}
	if instance.Status.SecretHashes == nil {
		instance.Status.SecretHashes = make(map[string]string)
	}
	if instance.Status.ContainerImages == nil {
		instance.Status.ContainerImages = make(map[string]string)
	}

	instance.Status.Conditions.MarkFalse(dataplanev1.SetupReadyCondition, condition.RequestedReason, condition.SeverityInfo, condition.ReadyInitMessage)

	// Detect config changes and set Status ConfigHash
	configHash, err := r.GetSpecConfigHash(instance)
	if err != nil {
		return ctrl.Result{}, err
	}

	if configHash != instance.Status.DeployedConfigHash {
		instance.Status.ConfigHash = configHash
	}

	// Ensure Services
	err = deployment.EnsureServices(ctx, helper, instance, validate)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			dataplanev1.SetupReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			dataplanev1.DataPlaneNodeSetErrorMessage,
			err.Error())
		return ctrl.Result{}, err
	}

	// Ensure IPSets Required for Nodes
	allIPSets, netServiceNetMap, isReady, err := deployment.EnsureIPSets(ctx, helper, instance)
	if err != nil || !isReady {
		return ctrl.Result{}, err
	}

	// Ensure DNSData Required for Nodes
	dnsDetails, err := deployment.EnsureDNSData(
		ctx, helper,
		instance, allIPSets)
	if err != nil || !dnsDetails.IsReady {
		return ctrl.Result{}, err
	}
	instance.Status.DNSClusterAddresses = dnsDetails.ClusterAddresses
	instance.Status.CtlplaneSearchDomain = dnsDetails.CtlplaneSearchDomain
	instance.Status.AllHostnames = dnsDetails.Hostnames
	instance.Status.AllIPs = dnsDetails.AllIPs

	ansibleSSHPrivateKeySecret := instance.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret

	secretKeys := []string{}
	secretKeys = append(secretKeys, AnsibleSSHPrivateKey)
	if !instance.Spec.PreProvisioned {
		secretKeys = append(secretKeys, AnsibleSSHAuthorizedKeys)
	}
	_, result, err = secret.VerifySecret(
		ctx,
		types.NamespacedName{
			Namespace: instance.Namespace,
			Name:      ansibleSSHPrivateKeySecret,
		},
		secretKeys,
		helper.GetClient(),
		time.Second*5,
	)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			"%s", err.Error())
		return result, err
	} else if (result != ctrl.Result{}) {
		// Since the the private key secret should have been manually created by the user when provided in the spec,
		// we treat this as a warning because it means that reconciliation will not be able to continue.
		instance.Status.Conditions.MarkFalse(
			condition.InputReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			dataplanev1.InputReadyWaitingMessage,
			"secret/"+ansibleSSHPrivateKeySecret)
		return result, nil
	}

	// all our input checks out so report InputReady
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)

	// Reconcile ServiceAccount
	nodeSetServiceAccount := serviceaccount.NewServiceAccount(
		&corev1.ServiceAccount{
			ObjectMeta: v1.ObjectMeta{
				Namespace: instance.Namespace,
				Name:      instance.Name,
			},
		},
		time.Duration(10),
	)
	saResult, err := nodeSetServiceAccount.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceAccountReadyErrorMessage,
			err.Error())
		return saResult, err
	} else if (saResult != ctrl.Result{}) {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ServiceAccountCreatingMessage)
		return saResult, nil
	}

	regViewerRoleBinding := rolebinding.NewRoleBinding(
		&rbacv1.RoleBinding{
			ObjectMeta: v1.ObjectMeta{
				Namespace: instance.Namespace,
				Name:      instance.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "registry-viewer",
			},
		},
		time.Duration(10),
	)
	rbResult, err := regViewerRoleBinding.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			condition.ServiceAccountReadyErrorMessage,
			err.Error())
		return rbResult, err
	} else if (rbResult != ctrl.Result{}) {
		instance.Status.Conditions.MarkFalse(
			condition.ServiceAccountReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.ServiceAccountCreatingMessage)
		return rbResult, nil
	}

	instance.Status.Conditions.MarkTrue(
		condition.ServiceAccountReadyCondition,
		condition.ServiceAccountReadyMessage)

	version, err := dataplaneutil.GetVersion(ctx, helper, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}
	containerImages := dataplaneutil.GetContainerImages(version)
	var provResult deployment.ProvisionResult
	// Reconcile BaremetalSet if required
	if !instance.Spec.PreProvisioned {
		// Reset the NodeSetBareMetalProvisionReadyCondition to unknown
		instance.Status.Conditions.MarkUnknown(dataplanev1.NodeSetBareMetalProvisionReadyCondition,
			condition.InitReason, condition.InitReason)

		provResult, err = deployment.DeployBaremetalSet(ctx, helper, instance,
			allIPSets, dnsDetails.ServerAddresses, containerImages)
		if err != nil || !provResult.IsProvisioned {
			return ctrl.Result{}, err
		}
		instance.Status.BmhRefHash = provResult.BmhRefHash
	}

	isDeploymentReady, isDeploymentRunning, isDeploymentFailed, failedDeployment, err := checkDeployment(
		ctx, helper, instance, r)
	if !isDeploymentFailed && err != nil {
		instance.Status.Conditions.MarkFalse(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			condition.DeploymentReadyErrorMessage,
			err.Error())
		Log.Error(err, "Unable to get deployed OpenStackDataPlaneDeployments.")
		return ctrl.Result{}, err
	}

	if !isDeploymentRunning {
		// Generate NodeSet Inventory
		_, errInventory := deployment.GenerateNodeSetInventory(ctx, helper, instance,
			allIPSets, dnsDetails.ServerAddresses, containerImages, netServiceNetMap)
		if errInventory != nil {
			errorMsg := fmt.Sprintf("Unable to generate inventory for %s", instance.Name)
			util.LogErrorForObject(helper, errInventory, errorMsg, instance)
			instance.Status.Conditions.MarkFalse(
				dataplanev1.SetupReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				dataplanev1.DataPlaneNodeSetErrorMessage,
				errorMsg)
			return ctrl.Result{}, errInventory
		}
	}
	// all setup tasks complete, mark SetupReadyCondition True
	instance.Status.Conditions.MarkTrue(dataplanev1.SetupReadyCondition, condition.ReadyMessage)

	// Set DeploymentReadyCondition to False if it was unknown.
	// Handles the case where the NodeSet is created, but not yet deployed.
	if instance.Status.Conditions.IsUnknown(condition.DeploymentReadyCondition) {
		Log.Info("Set NodeSet DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			dataplanev1.NodeSetDeploymentReadyWaitingMessage)
	}

	if isDeploymentReady {
		Log.Info("Set NodeSet DeploymentReadyCondition true")
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition,
			condition.DeploymentReadyMessage)
		instance.Status.DeployedBmhHash = instance.Status.BmhRefHash
	} else if isDeploymentRunning {
		Log.Info("Deployment still running...", "instance", instance)
		Log.Info("Set NodeSet DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(
			condition.DeploymentReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			condition.DeploymentReadyRunningMessage)
	} else if isDeploymentFailed {
		podsInterface := r.Kclient.CoreV1().Pods(instance.Namespace)
		podsList, _err := podsInterface.List(ctx, v1.ListOptions{
			LabelSelector: fmt.Sprintf("openstackdataplanedeployment=%s", failedDeployment),
			FieldSelector: "status.phase=Failed",
		})

		if _err != nil {
			Log.Error(err, "unable to retrieve list of pods for dataplane diagnostic")
		} else {
			for _, pod := range podsList.Items {
				Log.Info(fmt.Sprintf("openstackansibleee job %s failed due to %s with message: %s", pod.Name, pod.Status.Reason, pod.Status.Message))
			}
		}
		Log.Info("Set NodeSet DeploymentReadyCondition false")
		deployErrorMsg := ""
		if err != nil {
			deployErrorMsg = err.Error()
		}
		instance.Status.Conditions.MarkFalse(
			condition.DeploymentReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			"%s", deployErrorMsg)
	}

	return ctrl.Result{}, err
}

func checkDeployment(ctx context.Context, helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	r *OpenStackDataPlaneNodeSetReconciler) (
	isNodeSetDeploymentReady bool, isNodeSetDeploymentRunning bool,
	isNodeSetDeploymentFailed bool, failedDeploymentName string, err error) {

	// Get all completed deployments
	deployments := &dataplanev1.OpenStackDataPlaneDeploymentList{}
	opts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	err = helper.GetClient().List(ctx, deployments, opts...)
	if err != nil {
		helper.GetLogger().Error(err, "Unable to retrieve OpenStackDataPlaneDeployment CRs %v")
		return isNodeSetDeploymentReady, isNodeSetDeploymentRunning, isNodeSetDeploymentFailed, failedDeploymentName, err
	}

	// Collect deployments that target this nodeset (excluding deleted ones)
	var relevantDeployments []*dataplanev1.OpenStackDataPlaneDeployment
	for i := range deployments.Items {
		deployment := &deployments.Items[i]
		if !deployment.DeletionTimestamp.IsZero() {
			continue
		}
		if slices.Contains(deployment.Spec.NodeSets, instance.Name) {
			relevantDeployments = append(relevantDeployments, deployment)
		}
	}

	// Sort relevant deployments from oldest to newest, then take the last one
	var latestRelevantDeployment *dataplanev1.OpenStackDataPlaneDeployment
	if len(relevantDeployments) > 0 {
		slices.SortFunc(relevantDeployments, func(a, b *dataplanev1.OpenStackDataPlaneDeployment) int {
			aReady := a.Status.Conditions.Get(condition.DeploymentReadyCondition)
			bReady := b.Status.Conditions.Get(condition.DeploymentReadyCondition)
			if aReady != nil && bReady != nil {
				if aReady.LastTransitionTime.Before(&bReady.LastTransitionTime) {
					return -1
				}
			}
			return 1
		})
		latestRelevantDeployment = relevantDeployments[len(relevantDeployments)-1]
	}

	for _, deployment := range relevantDeployments {
		// Always add to DeploymentStatuses (for visibility)
		deploymentConditions := deployment.Status.NodeSetConditions[instance.Name]
		if instance.Status.DeploymentStatuses == nil {
			instance.Status.DeploymentStatuses = make(map[string]condition.Conditions)
		}
		instance.Status.DeploymentStatuses[deployment.Name] = deploymentConditions

		// Apply filtering for overall nodeset deployment state logic
		isLatestDeployment := latestRelevantDeployment != nil && deployment.Name == latestRelevantDeployment.Name
		deploymentCondition := deploymentConditions.Get(dataplanev1.NodeSetDeploymentReadyCondition)

		// Skip failed/error deployments that aren't the latest
		// All running and completed deployments are processed
		isCurrentDeploymentFailed := condition.IsError(deploymentCondition)
		if isCurrentDeploymentFailed && !isLatestDeployment {
			continue
		}

		isCurrentDeploymentRunning := deploymentConditions.IsFalse(dataplanev1.NodeSetDeploymentReadyCondition) && !isCurrentDeploymentFailed
		isCurrentDeploymentReady := deploymentConditions.IsTrue(dataplanev1.NodeSetDeploymentReadyCondition)

		// Reset the vars for every deployment that affects overall state
		isNodeSetDeploymentReady = false
		isNodeSetDeploymentRunning = false
		isNodeSetDeploymentFailed = false

		if isCurrentDeploymentFailed {
			err = fmt.Errorf("%s", deploymentCondition.Message)
			failedDeploymentName = deployment.Name
			isNodeSetDeploymentFailed = true
			break
		}
		if isCurrentDeploymentRunning {
			isNodeSetDeploymentRunning = true
		}

		if isCurrentDeploymentReady {
			// If the nodeset configHash does not match with what's in the deployment or
			// deployedBmhHash is different from current bmhRefHash.
			if (deployment.Status.NodeSetHashes[instance.Name] != instance.Status.ConfigHash) ||
				(!instance.Spec.PreProvisioned &&
					deployment.Status.BmhRefHashes[instance.Name] != instance.Status.BmhRefHash) {
				continue
			}

			hasAnsibleVarsFromChanged, err := checkAnsibleVarsFromChanged(ctx, helper, instance, deployment.Status.ConfigMapHashes, deployment.Status.SecretHashes)

			if err != nil {
				return isNodeSetDeploymentReady, isNodeSetDeploymentRunning, isNodeSetDeploymentFailed, failedDeploymentName, err
			}

			if hasAnsibleVarsFromChanged {
				continue
			}

			isNodeSetDeploymentReady = true

			// Track if this deployment is actually changing the nodeset config
			// IMPORTANT: Check BEFORE copying hashes, since multiple deployments process in the same reconcile
			newDeployedConfigHash, hasNodeSetHash := deployment.Status.NodeSetHashes[instance.Name]
			if !hasNodeSetHash {
				// Deployment doesn't have a hash for this nodeset, skip credential tracking
				helper.GetLogger().Info("Deployment missing NodeSetHash, skipping credential tracking",
					"deployment", deployment.Name,
					"nodeset", instance.Name)
				newDeployedConfigHash = ""
			}
			configHashChanged := instance.Status.DeployedConfigHash != newDeployedConfigHash

			// Check if any secrets changed (for credential rotation detection)
			// This must be done BEFORE copying the hashes below
			secretsChanged := false
			for k, newHash := range deployment.Status.SecretHashes {
				if oldHash, exists := instance.Status.SecretHashes[k]; !exists || oldHash != newHash {
					secretsChanged = true
					break
				}
			}

			// Update secret deployment tracking BEFORE copying hashes
			// Track secrets when:
			// 1. Config or secrets changed (normal case)
			// 2. Status is empty (first-time tracking)
			// 3. ConfigMap is missing but status exists (recovery from deletion)
			// 4. Deployment covers some nodes (via AnsibleLimit) - needed for gradual rollouts
			//    where multiple deployments have same hashes but target different nodes
			hasAnsibleLimit := deployment.Spec.AnsibleLimit != "" && deployment.Spec.AnsibleLimit != "*"
			needsTracking := configHashChanged || secretsChanged || instance.Status.SecretDeployment == nil || hasAnsibleLimit

			// Check if ConfigMap exists when status exists (handle manual deletion)
			if !needsTracking && instance.Status.SecretDeployment != nil {
				configMapName := getSecretTrackingConfigMapName(instance.Name)
				cm := &corev1.ConfigMap{}
				err := helper.GetClient().Get(ctx, types.NamespacedName{
					Name:      configMapName,
					Namespace: instance.Namespace,
				}, cm)
				if k8s_errors.IsNotFound(err) {
					helper.GetLogger().Info("Tracking ConfigMap missing but status exists, forcing recreation",
						"configMapName", configMapName)
					needsTracking = true
				}
			}

			if needsTracking {
				if err := r.updateSecretDeploymentTracking(ctx, helper, instance, deployment); err != nil {
					helper.GetLogger().Error(err, "Failed to update secret deployment tracking")
					return false, false, false, "", err
				}
			}

			// Now copy the hashes to nodeset status
			for k, v := range deployment.Status.ConfigMapHashes {
				instance.Status.ConfigMapHashes[k] = v
			}
			for k, v := range deployment.Status.SecretHashes {
				instance.Status.SecretHashes[k] = v
			}
			for k, v := range deployment.Status.ContainerImages {
				instance.Status.ContainerImages[k] = v
			}
			instance.Status.DeployedConfigHash = newDeployedConfigHash

			// Get list of services by name, either from ServicesOverride or
			// the NodeSet.
			var services []string
			if len(deployment.Spec.ServicesOverride) != 0 {
				services = deployment.Spec.ServicesOverride
			} else {
				services = instance.Spec.Services
			}

			// For each service, check if EDPMServiceType is "update" or "update-services", and
			// if so, copy Deployment.Status.DeployedVersion to
			// NodeSet.Status.DeployedVersion
			for _, serviceName := range services {
				service := &dataplanev1.OpenStackDataPlaneService{}
				name := types.NamespacedName{
					Namespace: instance.Namespace,
					Name:      serviceName,
				}
				err := helper.GetClient().Get(ctx, name, service)
				if err != nil {
					helper.GetLogger().Error(err, "Unable to retrieve OpenStackDataPlaneService %v")
					return isNodeSetDeploymentReady, isNodeSetDeploymentRunning, isNodeSetDeploymentFailed, failedDeploymentName, err
				}

				if service.Spec.EDPMServiceType != "update" && service.Spec.EDPMServiceType != "update-services" {
					continue
				}

				// An "update" or "update-services" service Deployment has been completed, so
				// set the NodeSet's DeployedVersion to the Deployment's
				// DeployedVersion.
				instance.Status.DeployedVersion = deployment.Status.DeployedVersion
			}
		}
	}

	// Detect secret drift to prevent race conditions during credential rotation.
	// Runs on every reconciliation to catch when secrets change in the cluster
	// but status still shows AllNodesUpdated=true (e.g., new RabbitMQUser created
	// before deployment runs).
	//
	// Always run drift detection - even if tracking was just updated, secrets may have
	// changed during the deployment (e.g., cert-manager rotating certs), and we should
	// detect this as drift requiring a new deployment.
	//
	// IMPORTANT: We reload trackingData here to check current cluster state.
	// However, Kubernetes client caching can cause stale reads if deployments were
	// just processed above. To work around this, we fetch the ConfigMap directly
	// with a fresh read to bypass the cache.
	if instance.Status.SecretDeployment != nil {
		// Fetch ConfigMap using APIReader to bypass client cache
		// The regular client caches reads, causing drift detection to see stale data
		// even after deployment processing just updated the ConfigMap.
		configMapName := getSecretTrackingConfigMapName(instance.Name)
		cm := &corev1.ConfigMap{}
		err := r.APIReader.Get(ctx, types.NamespacedName{
			Name:      configMapName,
			Namespace: instance.Namespace,
		}, cm)

		var trackingData *SecretTrackingData
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				helper.GetLogger().Info("Tracking ConfigMap not found for drift detection, skipping")
				trackingData = nil
			} else {
				helper.GetLogger().Error(err, "Failed to load tracking ConfigMap for drift detection")
				trackingData = nil
			}
		} else {
			// Parse tracking data from ConfigMap
			trackingJSON := cm.Data["tracking.json"]
			if trackingJSON == "" {
				helper.GetLogger().Info("Empty tracking data in ConfigMap for drift detection")
				trackingData = &SecretTrackingData{
					Secrets:    make(map[string]SecretVersionInfo),
					NodeStatus: make(map[string]NodeSecretStatus),
				}
			} else {
				trackingData = &SecretTrackingData{}
				if unmarshalErr := json.Unmarshal([]byte(trackingJSON), trackingData); unmarshalErr != nil {
					helper.GetLogger().Error(unmarshalErr, "Failed to unmarshal tracking data for drift detection")
					trackingData = nil
				}
			}
		}

		if trackingData != nil {
			// Check for drift (also updates Expected fields in tracking)
			driftDetected, err := r.detectSecretDrift(ctx, instance, trackingData)
			if err != nil {
				helper.GetLogger().Error(err, "Error during secret drift detection")
				// Don't fail reconciliation, but log the error
			} else {
				// Save tracking data with updated Expected fields
				if saveErr := r.saveSecretTrackingData(ctx, helper, instance, trackingData); saveErr != nil {
					helper.GetLogger().Error(saveErr, "Failed to save tracking data after drift detection")
				}

				if driftDetected {
					// Drift detected - secrets in cluster differ from what's deployed on nodes
					helper.GetLogger().Info("Secret drift detected, updating status to block credential deletion")

					// Set AllNodesUpdated = false to prevent credential deletion
					// Set UpdatedNodes = 0 because nodes don't have the Expected (cluster) version
					// This matches the logic in computeDeploymentSummary and prevents confusing
					// status like "updatedNodes: 2, allNodesUpdated: false"
					instance.Status.SecretDeployment.AllNodesUpdated = false
					instance.Status.SecretDeployment.UpdatedNodes = 0
					now := v1.Now()
					instance.Status.SecretDeployment.LastUpdateTime = &now

					helper.GetLogger().Info("Status updated after drift detection",
						"allNodesUpdated", false,
						"updatedNodes", 0,
						"totalNodes", instance.Status.SecretDeployment.TotalNodes)
				}
			}
		}
	}

	return isNodeSetDeploymentReady, isNodeSetDeploymentRunning, isNodeSetDeploymentFailed, failedDeploymentName, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackDataPlaneNodeSetReconciler) SetupWithManager(
	ctx context.Context, mgr ctrl.Manager,
) error {
	// index for ConfigMaps listed on ansibleVarsFrom
	if err := mgr.GetFieldIndexer().IndexField(ctx,
		&dataplanev1.OpenStackDataPlaneNodeSet{}, "spec.ansibleVarsFrom.ansible.configMaps",
		func(rawObj client.Object) []string {
			nodeSet := rawObj.(*dataplanev1.OpenStackDataPlaneNodeSet)
			configMaps := make([]string, 0)

			appendConfigMaps := func(varsFrom []dataplanev1.DataSource) {
				for _, ref := range varsFrom {
					if ref.ConfigMapRef != nil {
						configMaps = append(configMaps, ref.ConfigMapRef.Name)
					}
				}
			}

			appendConfigMaps(nodeSet.Spec.NodeTemplate.Ansible.AnsibleVarsFrom)
			for _, node := range nodeSet.Spec.Nodes {
				appendConfigMaps(node.Ansible.AnsibleVarsFrom)
			}
			return configMaps
		}); err != nil {
		return err
	}

	// index for Secrets listed on ansibleVarsFrom
	if err := mgr.GetFieldIndexer().IndexField(ctx,
		&dataplanev1.OpenStackDataPlaneNodeSet{}, "spec.ansibleVarsFrom.ansible.secrets",
		func(rawObj client.Object) []string {
			nodeSet := rawObj.(*dataplanev1.OpenStackDataPlaneNodeSet)
			secrets := make([]string, 0, len(nodeSet.Spec.Nodes)+1)
			if nodeSet.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret != "" {
				secrets = append(secrets, nodeSet.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret)
			}

			appendSecrets := func(varsFrom []dataplanev1.DataSource) {
				for _, ref := range varsFrom {
					if ref.SecretRef != nil {
						secrets = append(secrets, ref.SecretRef.Name)
					}
				}
			}

			appendSecrets(nodeSet.Spec.NodeTemplate.Ansible.AnsibleVarsFrom)
			for _, node := range nodeSet.Spec.Nodes {
				appendSecrets(node.Ansible.AnsibleVarsFrom)
			}
			return secrets
		}); err != nil {
		return err
	}
	// Initialize the Watching map for conditional CRD watches
	r.Watching = make(map[string]bool)
	r.Cache = mgr.GetCache()

	// Build the controller without MachineConfig watch (added conditionally later)
	c, err := ctrl.NewControllerManagedBy(mgr).
		For(&dataplanev1.OpenStackDataPlaneNodeSet{},
			builder.WithPredicates(predicate.Or(
				predicate.GenerationChangedPredicate{},
				predicate.AnnotationChangedPredicate{},
				predicate.LabelChangedPredicate{}))).
		Owns(&batchv1.Job{}).
		Owns(&baremetalv1.OpenStackBaremetalSet{}).
		Owns(&infranetworkv1.IPSet{}).
		Owns(&infranetworkv1.DNSData{}).
		Owns(&corev1.Secret{}).
		Watches(&infranetworkv1.DNSMasq{},
			handler.EnqueueRequestsFromMapFunc(r.genericWatcherFn)).
		Watches(&dataplanev1.OpenStackDataPlaneDeployment{},
			handler.EnqueueRequestsFromMapFunc(r.deploymentWatcherFn)).
		Watches(&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.secretWatcherFn),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.secretWatcherFn)).
		Watches(&openstackv1.OpenStackVersion{},
			handler.EnqueueRequestsFromMapFunc(r.genericWatcherFn)).
		// NOTE: MachineConfig watch is added conditionally during reconciliation
		// to avoid failures when the MachineConfig CRD doesn't exist
		Build(r)

	if err != nil {
		return err
	}
	r.Controller = c
	return nil
}

// machineConfigWatcherFn - watches for changes to the registries MachineConfig resource and queues
// a reconcile of each NodeSet if the MachineConfig is changed.
func (r *OpenStackDataPlaneNodeSetReconciler) machineConfigWatcherFn(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	Log := r.GetLogger(ctx)
	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	const registryMachineConfigName string = "99-master-generated-registries"

	if obj.GetName() != registryMachineConfigName {
		return nil
	}

	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
	}
	if err := r.List(ctx, nodeSets, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve OpenStackDataPlaneNodeSetList")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeSets.Items))
	for _, nodeSet := range nodeSets.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      nodeSet.Name,
			},
		})
		Log.Info(fmt.Sprintf("reconcile loop for openstackdataplanenodeset %s triggered by %s %s",
			nodeSet.Name, kind, obj.GetName()))
	}
	return requests
}

// machineConfigWatcherFnTyped - typed version of machineConfigWatcherFn for use with source.Kind
func (r *OpenStackDataPlaneNodeSetReconciler) machineConfigWatcherFnTyped(
	ctx context.Context, obj *machineconfig.MachineConfig,
) []reconcile.Request {
	return r.machineConfigWatcherFn(ctx, obj)
}

const machineConfigCRDName = "machineconfigs.machineconfiguration.openshift.io"

// ensureMachineConfigWatch attempts to set up a watch for MachineConfig resources.
// This is done conditionally because the MachineConfig CRD may not exist on all clusters
// (e.g., non-OpenShift Kubernetes clusters or clusters without the Machine Config Operator).
// Returns true if the CRD is available (watch was set up or already exists), false otherwise.
func (r *OpenStackDataPlaneNodeSetReconciler) ensureMachineConfigWatch(ctx context.Context) bool {
	Log := r.GetLogger(ctx)

	// Check if we're already watching
	if r.Watching[machineConfigCRDName] {
		return true
	}

	// Check if the MachineConfig CRD exists
	crd := &unstructured.Unstructured{}
	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Kind:    "CustomResourceDefinition",
		Version: "v1",
	})

	err := r.Get(ctx, client.ObjectKey{Name: machineConfigCRDName}, crd)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			Log.Info("MachineConfig CRD not found, disconnected environment features disabled")
		} else {
			Log.Error(err, "Error checking for MachineConfig CRD")
		}
		return false
	}

	// CRD exists, set up the watch
	Log.Info("MachineConfig CRD found, enabling watch for disconnected environment support")
	err = r.Controller.Watch(
		source.Kind(
			r.Cache,
			&machineconfig.MachineConfig{},
			handler.TypedEnqueueRequestsFromMapFunc(r.machineConfigWatcherFnTyped),
			predicate.TypedResourceVersionChangedPredicate[*machineconfig.MachineConfig]{},
		),
	)
	if err != nil {
		Log.Error(err, "Failed to set up MachineConfig watch")
		return false
	}

	r.Watching[machineConfigCRDName] = true
	Log.Info("Successfully set up MachineConfig watch")
	return true
}

// IsMachineConfigAvailable returns true if the MachineConfig CRD is available and being watched
func (r *OpenStackDataPlaneNodeSetReconciler) IsMachineConfigAvailable() bool {
	return r.Watching[machineConfigCRDName]
}

func (r *OpenStackDataPlaneNodeSetReconciler) secretWatcherFn(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	Log := r.GetLogger(ctx)

	// Determine kind based on object type (GVK may not be populated in watch events)
	var kind string
	switch obj.(type) {
	case *corev1.Secret:
		kind = "secret"
	case *corev1.ConfigMap:
		kind = "configmap"
	default:
		// Fallback to GVK if available
		kind = strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	}

	Log.Info("secretWatcherFn called",
		"kind", kind,
		"name", obj.GetName(),
		"namespace", obj.GetNamespace())

	// Track which nodesets we've already added to avoid duplicates
	requestedNodeSets := make(map[string]bool)
	requests := make([]reconcile.Request, 0)

	// 1. Check for nodesets that reference this secret/configmap in ansibleVarsFrom
	selector := "spec.ansibleVarsFrom.ansible.configMaps"
	if kind == "secret" {
		selector = "spec.ansibleVarsFrom.ansible.secrets"
	}

	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	listOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(selector, obj.GetName()),
		Namespace:     obj.GetNamespace(),
	}

	if err := r.List(ctx, nodeSets, listOpts); err != nil {
		Log.Error(err, "Unable to retrieve OpenStackDataPlaneNodeSetList for ansibleVarsFrom")
		return nil
	}

	for _, nodeSet := range nodeSets.Items {
		key := fmt.Sprintf("%s/%s", nodeSet.Namespace, nodeSet.Name)
		if !requestedNodeSets[key] {
			requests = append(requests, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: obj.GetNamespace(),
					Name:      nodeSet.Name,
				},
			})
			requestedNodeSets[key] = true
			Log.Info(fmt.Sprintf("reconcile loop for openstackdataplanenodeset %s triggered by %s %s (ansibleVarsFrom)",
				nodeSet.Name, kind, obj.GetName()))
		}
	}

	// 2. Check for nodesets that have this secret tracked in status.SecretHashes
	// This handles secrets like RabbitMQUser credentials that aren't in ansibleVarsFrom
	// but are tracked for drift detection
	if kind == "secret" {
		allNodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
		if err := r.List(ctx, allNodeSets, client.InNamespace(obj.GetNamespace())); err != nil {
			Log.Error(err, "Unable to retrieve OpenStackDataPlaneNodeSetList for secret tracking")
			return requests
		}

		for _, nodeSet := range allNodeSets.Items {
			// Check if secret is in nodeset.Status.SecretHashes
			if nodeSet.Status.SecretHashes != nil {
				if _, exists := nodeSet.Status.SecretHashes[obj.GetName()]; exists {
					key := fmt.Sprintf("%s/%s", nodeSet.Namespace, nodeSet.Name)
					if !requestedNodeSets[key] {
						requests = append(requests, reconcile.Request{
							NamespacedName: types.NamespacedName{
								Namespace: nodeSet.Namespace,
								Name:      nodeSet.Name,
							},
						})
						requestedNodeSets[key] = true
						Log.Info(fmt.Sprintf("reconcile loop for openstackdataplanenodeset %s triggered by %s %s (tracked secret)",
							nodeSet.Name, kind, obj.GetName()))
					}
				}
			}
		}
	}

	return requests
}

func (r *OpenStackDataPlaneNodeSetReconciler) genericWatcherFn(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	Log := r.GetLogger(ctx)
	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
	}
	if err := r.List(ctx, nodeSets, listOpts...); err != nil {
		Log.Error(err, "Unable to retrieve OpenStackDataPlaneNodeSetList")
		return nil
	}

	requests := make([]reconcile.Request, 0, len(nodeSets.Items))
	for _, nodeSet := range nodeSets.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: obj.GetNamespace(),
				Name:      nodeSet.Name,
			},
		})
		Log.Info(fmt.Sprintf("Reconciling NodeSet %s due to watcher on %s/%s", nodeSet.Name, obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()))
	}
	return requests
}

func (r *OpenStackDataPlaneNodeSetReconciler) deploymentWatcherFn(
	ctx context.Context, //revive:disable-line
	obj client.Object,
) []reconcile.Request {
	Log := r.GetLogger(ctx)
	namespace := obj.GetNamespace()
	deployment := obj.(*dataplanev1.OpenStackDataPlaneDeployment)

	requests := make([]reconcile.Request, 0, len(deployment.Spec.NodeSets))
	for _, nodeSet := range deployment.Spec.NodeSets {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Namespace: namespace,
				Name:      nodeSet,
			},
		})
		Log.Info(fmt.Sprintf("Reconciling NodeSet %s due to watcher on %s/%s", nodeSet, obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName()))
	}
	return requests
}

// GetSpecConfigHash initialises a new struct with only the field we want to check for variances in.
// We then hash the contents of the new struct using md5 and return the hashed string.
func (r *OpenStackDataPlaneNodeSetReconciler) GetSpecConfigHash(instance *dataplanev1.OpenStackDataPlaneNodeSet) (string, error) {
	configHash, err := util.ObjectHash(&instance.Spec)
	if err != nil {
		return "", err
	}
	return configHash, nil
}

// checkAnsibleVarsFromChanged computes current hashes for ConfigMaps/Secrets
// referenced in AnsibleVarsFrom and compares them with deployed hashes.
// Returns true if any content has changed, false otherwise.
func checkAnsibleVarsFromChanged(
	ctx context.Context,
	helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	deployedConfigMapHashes map[string]string,
	deployedSecretHashes map[string]string,
) (bool, error) {
	currentConfigMapHashes := make(map[string]string)
	currentSecretHashes := make(map[string]string)

	namespace := instance.Namespace

	// Process NodeTemplate level AnsibleVarsFrom
	if err := deployment.ProcessAnsibleVarsFrom(ctx, helper, namespace, currentConfigMapHashes, currentSecretHashes, instance.Spec.NodeTemplate.Ansible.AnsibleVarsFrom); err != nil {
		return false, err
	}

	// Process individual Node level AnsibleVarsFrom
	for _, node := range instance.Spec.Nodes {
		if err := deployment.ProcessAnsibleVarsFrom(ctx, helper, namespace, currentConfigMapHashes, currentSecretHashes, node.Ansible.AnsibleVarsFrom); err != nil {
			return false, err
		}
	}

	// Compare current ConfigMap hashes with deployed hashes
	for name, currentHash := range currentConfigMapHashes {
		if deployedHash, exists := deployedConfigMapHashes[name]; !exists || deployedHash != currentHash {
			helper.GetLogger().Info("ConfigMap content changed", "configMap", name)
			return true, nil
		}
	}

	// Compare current Secret hashes with deployed hashes
	for name, currentHash := range currentSecretHashes {
		if deployedHash, exists := deployedSecretHashes[name]; !exists || deployedHash != currentHash {
			helper.GetLogger().Info("Secret content changed", "secret", name)
			return true, nil
		}
	}

	return false, nil
}

// SecretTrackingData represents the structure stored in the secret tracking ConfigMap
type SecretTrackingData struct {
	Secrets    map[string]SecretVersionInfo `json:"secrets"`
	NodeStatus map[string]NodeSecretStatus  `json:"nodeStatus"`
}

// SecretVersionInfo tracks a secret's current and previous versions across nodes
type SecretVersionInfo struct {
	// Current: what's deployed on nodes (from last deployment)
	CurrentHash            string   `json:"currentHash"`
	CurrentResourceVersion string   `json:"currentResourceVersion"`
	CurrentGeneration      int64    `json:"currentGeneration"`
	NodesWithCurrent       []string `json:"nodesWithCurrent"`

	// Expected: what's in K8s cluster right now (fetched live during drift detection)
	ExpectedHash            string `json:"expectedHash,omitempty"`
	ExpectedResourceVersion string `json:"expectedResourceVersion,omitempty"`
	ExpectedGeneration      int64  `json:"expectedGeneration,omitempty"`

	// Previous: for rotation tracking (old version during rollout)
	PreviousHash            string   `json:"previousHash,omitempty"`
	PreviousResourceVersion string   `json:"previousResourceVersion,omitempty"`
	PreviousGeneration      int64    `json:"previousGeneration,omitempty"`
	NodesWithPrevious       []string `json:"nodesWithPrevious,omitempty"`

	LastChanged time.Time `json:"lastChanged"`
}

// NodeSecretStatus tracks which secrets a node has (current version)
type NodeSecretStatus struct {
	AllSecretsUpdated   bool     `json:"allSecretsUpdated"`
	SecretsWithCurrent  []string `json:"secretsWithCurrent"`
	SecretsWithPrevious []string `json:"secretsWithPrevious,omitempty"`
}

// getSecretTrackingConfigMapName returns the name of the tracking ConfigMap for a nodeset
func getSecretTrackingConfigMapName(nodesetName string) string {
	return fmt.Sprintf("%s-secret-tracking", nodesetName)
}

// getSecretTrackingData reads and parses the secret tracking data from the ConfigMap
func (r *OpenStackDataPlaneNodeSetReconciler) getSecretTrackingData(
	ctx context.Context,
	helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
) (*SecretTrackingData, error) {
	configMapName := getSecretTrackingConfigMapName(instance.Name)

	cm := &corev1.ConfigMap{}
	err := helper.GetClient().Get(ctx, types.NamespacedName{
		Name:      configMapName,
		Namespace: instance.Namespace,
	}, cm)

	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// ConfigMap doesn't exist - bootstrap from nodeset status if available
			return r.bootstrapTrackingFromStatus(ctx, instance)
		}
		return nil, err
	}

	// Parse JSON from tracking.json key
	trackingJSON, exists := cm.Data["tracking.json"]
	if !exists || trackingJSON == "" {
		// Empty ConfigMap, return empty structure
		return &SecretTrackingData{
			Secrets:    make(map[string]SecretVersionInfo),
			NodeStatus: make(map[string]NodeSecretStatus),
		}, nil
	}

	var data SecretTrackingData
	if err := json.Unmarshal([]byte(trackingJSON), &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal tracking data: %w", err)
	}

	// Ensure maps are initialized
	if data.Secrets == nil {
		data.Secrets = make(map[string]SecretVersionInfo)
	}
	if data.NodeStatus == nil {
		data.NodeStatus = make(map[string]NodeSecretStatus)
	}

	return &data, nil
}

// bootstrapTrackingFromStatus initializes tracking from nodeset status when ConfigMap doesn't exist.
// Uses instance.Status.SecretHashes as "Current" (what was last deployed to some/all nodes)
// and fetches cluster secrets as "Expected" (what exists in cluster now).
// We don't know which specific nodes have which secrets until a deployment runs.
func (r *OpenStackDataPlaneNodeSetReconciler) bootstrapTrackingFromStatus(
	ctx context.Context,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
) (*SecretTrackingData, error) {
	Log := r.GetLogger(ctx)

	trackingData := &SecretTrackingData{
		Secrets:    make(map[string]SecretVersionInfo),
		NodeStatus: make(map[string]NodeSecretStatus),
	}

	// If no status yet, return empty tracking
	if instance.Status.SecretHashes == nil || len(instance.Status.SecretHashes) == 0 {
		Log.Info("No ConfigMap and no status.SecretHashes, returning empty tracking")
		return trackingData, nil
	}

	Log.Info("Bootstrapping tracking from nodeset status", "secretCount", len(instance.Status.SecretHashes))

	// For each secret in status, create tracking entry
	// Current = from status (what was deployed)
	// Expected = from cluster (what exists now)
	for secretName, statusHash := range instance.Status.SecretHashes {
		secret := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: instance.Namespace,
		}, secret)

		var expectedHash string
		var expectedResourceVersion string
		var expectedGeneration int64

		if err != nil {
			if k8s_errors.IsNotFound(err) {
				Log.Info("Secret in status not found in cluster (deleted?)",
					"secret", secretName)
				// Current exists (in status), Expected doesn't exist → drift
				expectedHash = ""
				expectedResourceVersion = ""
				expectedGeneration = 0
			} else {
				Log.Error(err, "Failed to fetch secret for bootstrap", "secret", secretName)
				return nil, err
			}
		} else {
			// Compute hash of current cluster secret
			var hashErr error
			expectedHash, hashErr = util.ObjectHash(secret.Data)
			if hashErr != nil {
				Log.Error(hashErr, "Failed to hash secret data", "secret", secretName)
				return nil, hashErr
			}
			expectedResourceVersion = secret.ResourceVersion
			expectedGeneration = secret.Generation
		}

		// Initialize tracking entry
		// Current = from status (unknown which nodes have it)
		// Expected = from cluster
		// NodesWithCurrent = empty (we don't know until deployment runs)
		secretInfo := SecretVersionInfo{
			CurrentHash:             statusHash,
			CurrentResourceVersion:  "", // Don't know ResourceVersion of what's deployed
			CurrentGeneration:       0,  // Don't know Generation of what's deployed
			ExpectedHash:            expectedHash,
			ExpectedResourceVersion: expectedResourceVersion,
			ExpectedGeneration:      expectedGeneration,
			NodesWithCurrent:        []string{}, // Unknown which nodes have it
			LastChanged:             time.Now(),
		}

		trackingData.Secrets[secretName] = secretInfo

		Log.Info("Bootstrapped secret tracking",
			"secret", secretName,
			"currentHash", statusHash,
			"expectedHash", expectedHash,
			"drift", statusHash != expectedHash)
	}

	// Don't initialize node status - we don't know which nodes have which secrets
	// First deployment will establish per-node tracking

	Log.Info("Bootstrapping complete - waiting for deployment to establish node tracking",
		"secrets", len(trackingData.Secrets))

	return trackingData, nil
}

// saveSecretTrackingData marshals and saves the tracking data to the ConfigMap
func (r *OpenStackDataPlaneNodeSetReconciler) saveSecretTrackingData(
	ctx context.Context,
	helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	data *SecretTrackingData,
) error {
	configMapName := getSecretTrackingConfigMapName(instance.Name)

	// Marshal to JSON
	trackingJSON, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tracking data: %w", err)
	}

	customData := map[string]string{
		"tracking.json": string(trackingJSON),
	}

	cms := []util.Template{
		{
			Name:         configMapName,
			Namespace:    instance.Namespace,
			InstanceType: instance.Kind,
			CustomData:   customData,
		},
	}

	return configmap.EnsureConfigMaps(ctx, helper, instance, cms, nil)
}

// computeDeploymentSummary calculates summary status from detailed tracking data.
// AllNodesUpdated is true only when:
// 1. All nodes have all secrets (per-node tracking)
// 2. All secrets have Current == Expected (no drift)
func computeDeploymentSummary(
	data *SecretTrackingData,
	totalNodes int,
	configMapName string,
) *dataplanev1.SecretDeploymentStatus {
	// Check if any secret has drift (Current != Expected)
	// Use hash comparison - secret.Hash() from lib-common is deterministic
	hasDrift := false
	for _, secretInfo := range data.Secrets {
		if secretInfo.CurrentHash != secretInfo.ExpectedHash {
			hasDrift = true
			break
		}
	}

	// Count nodes where all secrets are updated
	// If there's drift, no nodes have the expected (cluster) version, so updatedNodes = 0
	// This prevents confusing status like "updatedNodes: 2" while "allNodesUpdated: false"
	updatedNodes := 0
	if !hasDrift {
		// No drift - count nodes that have all secrets in current version
		for _, nodeStatus := range data.NodeStatus {
			if nodeStatus.AllSecretsUpdated {
				updatedNodes++
			}
		}
	}
	// else: drift detected - all nodes are out of date relative to cluster, so updatedNodes = 0

	// AllNodesUpdated = all nodes have all secrets AND no drift
	allNodesUpdated := updatedNodes == totalNodes && totalNodes > 0 && !hasDrift

	now := v1.Now()
	return &dataplanev1.SecretDeploymentStatus{
		AllNodesUpdated: allNodesUpdated,
		TotalNodes:      totalNodes,
		UpdatedNodes:    updatedNodes,
		ConfigMapName:   configMapName,
		LastUpdateTime:  &now,
	}
}

// updateSecretDeploymentTracking updates the NodeSet status with information about which
// nodes have been updated with secrets from a deployment.
//
// This tracks ALL secrets in deployment.Status.SecretHashes, storing detailed per-secret
// tracking in a ConfigMap and summary metrics in the CR status.
func (r *OpenStackDataPlaneNodeSetReconciler) updateSecretDeploymentTracking(
	ctx context.Context,
	helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	deployment *dataplanev1.OpenStackDataPlaneDeployment,
) error {
	Log := r.GetLogger(ctx)

	// Validate inputs
	if instance == nil || deployment == nil {
		return fmt.Errorf("instance and deployment must not be nil")
	}

	// Check if deployment is ready
	isDeploymentReady := deployment.Status.Conditions.IsTrue(condition.DeploymentReadyCondition)

	// Get all nodes in this nodeset
	allNodes := getAllNodeNames(instance)
	totalNodes := len(allNodes)

	// Determine which nodes were covered by this deployment (handle AnsibleLimit)
	coveredNodes := getNodesCoveredByDeployment(deployment, instance)

	Log.Info("Updating secret deployment tracking",
		"deployment", deployment.Name,
		"deploymentReady", isDeploymentReady,
		"totalNodes", totalNodes,
		"coveredNodes", len(coveredNodes),
		"secretsInDeployment", len(deployment.Status.SecretHashes))

	// Load existing tracking data
	trackingData, err := r.getSecretTrackingData(ctx, helper, instance)
	if err != nil {
		Log.Error(err, "Failed to load secret tracking data")
		return err
	}

	// Process each secret in the deployment
	// Strategy:
	// 1. Fetch cluster secret to get ResourceVersion/Generation (for logging/debugging metadata)
	// 2. Use hash from deployment.Status.SecretHashes for rotation detection
	//    - Hash is from secret.Hash() (lib-common) - deterministic and content-based
	//    - Captured by deployment controller when deployment completed
	//    - Represents what was actually deployed, not cluster state NOW
	// 3. This avoids timing issues where cluster secret changes between deployment completion and reconciliation
	// 4. Drift detection separately compares cluster state vs tracking using hash comparison
	// 5. Both rotation and drift detection use hash-based comparison for consistency

	// Defensive validation: check deployment has secret hashes
	if len(deployment.Status.SecretHashes) == 0 {
		Log.Info("Deployment has no secret hashes in status, skipping tracking update",
			"deployment", deployment.Name,
			"namespace", deployment.Namespace,
			"deploymentReady", isDeploymentReady)
		return nil
	}

	for secretName, secretHash := range deployment.Status.SecretHashes {
		// Defensive validation: check hash is not empty
		if secretHash == "" {
			Log.Error(nil, "Empty secret hash in deployment status, skipping this secret",
				"secret", secretName,
				"deployment", deployment.Name,
				"namespace", deployment.Namespace)
			continue
		}

		// Fetch cluster secret for ResourceVersion/Generation metadata (logging/debugging)
		// IMPORTANT: We use the hash from deployment.Status for rotation detection,
		// NOT the cluster secret's current state. This prevents timing issues where
		// the secret rotates between deployment completion and this reconciliation.
		// We verify the hashes match (informational log if they differ).
		clusterSecret := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: instance.Namespace,
		}, clusterSecret)
		if err != nil {
			if k8s_errors.IsNotFound(err) {
				// Secret deleted between deployment completion and now - skip it
				// Drift detection will handle this case
				Log.Info("Secret in deployment not found in cluster, skipping tracking",
					"secret", secretName,
					"deployment", deployment.Name)
				continue
			}
			// API error - return immediately to trigger reconciliation retry
			// We don't save partial tracking state to avoid inconsistency
			// Next reconciliation will retry with the same starting state
			Log.Error(err, "Failed to fetch secret for tracking, will retry on next reconciliation",
				"secret", secretName,
				"deployment", deployment.Name,
				"namespace", instance.Namespace)
			return err
		}

		resourceVersion := clusterSecret.ResourceVersion
		generation := clusterSecret.Generation

		// Check if deployment is stale (deployed hash != current cluster hash)
		// If stale, skip tracking for this secret to avoid incorrectly marking nodes as updated
		clusterSecretHash, hashErr := secret.Hash(clusterSecret)
		if hashErr != nil {
			Log.Error(hashErr, "Failed to compute cluster secret hash, skipping this secret",
				"secret", secretName,
				"deployment", deployment.Name)
			// Skip this secret - can't verify if deployment is stale
			continue
		}

		if clusterSecretHash != secretHash {
			Log.Info("Deployment is stale, skipping tracking for this secret",
				"secret", secretName,
				"deployment", deployment.Name,
				"deploymentHash", secretHash,
				"clusterHash", clusterSecretHash,
				"clusterResourceVersion", resourceVersion)
			// Skip this secret - deployment reflects old cluster state, not current
			// If we processed it, we'd incorrectly mark nodes as having the current version
			continue
		}

		// Deployment hash matches cluster hash - it's current, safe to process
		hashToStore := secretHash

		secretInfo, exists := trackingData.Secrets[secretName]

		if !exists {
			// First time seeing this secret in tracking
			nodesWithCurrent := []string{}
			if isDeploymentReady {
				nodesWithCurrent = coveredNodes
			}

			secretInfo = SecretVersionInfo{
				CurrentHash:            hashToStore,
				CurrentResourceVersion: resourceVersion,
				CurrentGeneration:      generation,
				NodesWithCurrent:       nodesWithCurrent,
				LastChanged:            time.Now(),
			}

			Log.Info("New secret detected in deployment",
				"secret", secretName,
				"deployment", deployment.Name,
				"namespace", deployment.Namespace,
				"hash", hashToStore,
				"resourceVersion", resourceVersion,
				"generation", generation,
				"deploymentReady", isDeploymentReady,
				"nodesWithCurrent", len(nodesWithCurrent))

		} else if secretInfo.CurrentHash != hashToStore {
			// SECRET ROTATION: Deployment hash different from currently tracked
			// Use current cluster hash to ensure tracking reflects current state
			Log.Info("Secret rotation detected in deployment",
				"secret", secretName,
				"deployment", deployment.Name,
				"namespace", deployment.Namespace,
				"oldCurrentHash", secretInfo.CurrentHash,
				"newHash", hashToStore,
				"oldResourceVersion", secretInfo.CurrentResourceVersion,
				"newResourceVersion", resourceVersion,
				"deploymentReady", isDeploymentReady)

			// Move Current to Previous (nodes not yet upgraded still have old version)
			secretInfo.PreviousHash = secretInfo.CurrentHash
			secretInfo.PreviousResourceVersion = secretInfo.CurrentResourceVersion
			secretInfo.PreviousGeneration = secretInfo.CurrentGeneration
			secretInfo.NodesWithPrevious = secretInfo.NodesWithCurrent

			// Update Current to new version - use cluster hash not deployment hash
			// This ensures old deployments don't flip-flop the tracking state
			secretInfo.CurrentHash = hashToStore
			secretInfo.CurrentResourceVersion = resourceVersion
			secretInfo.CurrentGeneration = generation
			secretInfo.LastChanged = time.Now()

			// Update NodesWithCurrent if deployment is ready
			if isDeploymentReady {
				secretInfo.NodesWithCurrent = coveredNodes

				// Clear previous version if all nodes now have current version
				if len(secretInfo.NodesWithCurrent) == totalNodes && secretInfo.PreviousHash != "" {
					Log.Info("All nodes updated with new secret version after rotation, clearing previous",
						"secret", secretName,
						"deployment", deployment.Name,
						"namespace", deployment.Namespace,
						"previousHash", secretInfo.PreviousHash,
						"totalNodes", totalNodes)
					secretInfo.PreviousHash = ""
					secretInfo.PreviousResourceVersion = ""
					secretInfo.PreviousGeneration = 0
					secretInfo.NodesWithPrevious = []string{}
				}
			} else {
				// Deployment not ready yet - nodes still have previous version
				secretInfo.NodesWithCurrent = []string{}
			}

		} else {
			// SAME version - accumulate nodes across deployments
			// Current hash matches deployment hash - just accumulate nodes
			// Note: We don't update ResourceVersion/Generation here to keep metadata
			// consistent with the hash. Drift detection will update Expected metadata.

			if isDeploymentReady {
				// Add newly covered nodes
				for _, node := range coveredNodes {
					if !slices.Contains(secretInfo.NodesWithCurrent, node) {
						secretInfo.NodesWithCurrent = append(secretInfo.NodesWithCurrent, node)
					}

					// Remove from previous if it was there (node upgraded)
					if secretInfo.PreviousHash != "" {
						newPrevious := []string{}
						for _, prevNode := range secretInfo.NodesWithPrevious {
							if prevNode != node {
								newPrevious = append(newPrevious, prevNode)
							}
						}
						secretInfo.NodesWithPrevious = newPrevious
					}
				}

				// Clear previous version metadata if all nodes now have current version
				if len(secretInfo.NodesWithCurrent) == totalNodes && secretInfo.PreviousHash != "" {
					Log.Info("All nodes updated with new secret version, clearing previous",
						"secret", secretName,
						"deployment", deployment.Name,
						"namespace", deployment.Namespace,
						"previousHash", secretInfo.PreviousHash,
						"totalNodes", totalNodes)
					secretInfo.PreviousHash = ""
					secretInfo.PreviousResourceVersion = ""
					secretInfo.PreviousGeneration = 0
					secretInfo.NodesWithPrevious = []string{}
				}
			}
		}

		trackingData.Secrets[secretName] = secretInfo
	}

	// Update per-node status
	for _, nodeName := range allNodes {
		nodeStatus := NodeSecretStatus{
			AllSecretsUpdated:   true,
			SecretsWithCurrent:  []string{},
			SecretsWithPrevious: []string{},
		}

		// Check which secrets this node has
		for secretName, secretInfo := range trackingData.Secrets {
			if slices.Contains(secretInfo.NodesWithCurrent, nodeName) {
				nodeStatus.SecretsWithCurrent = append(nodeStatus.SecretsWithCurrent, secretName)
			} else if slices.Contains(secretInfo.NodesWithPrevious, nodeName) {
				nodeStatus.SecretsWithPrevious = append(nodeStatus.SecretsWithPrevious, secretName)
				nodeStatus.AllSecretsUpdated = false
			} else {
				// Node doesn't have this secret at all
				nodeStatus.AllSecretsUpdated = false
			}
		}

		trackingData.NodeStatus[nodeName] = nodeStatus
	}

	// Save tracking data to ConfigMap
	if err := r.saveSecretTrackingData(ctx, helper, instance, trackingData); err != nil {
		Log.Error(err, "Failed to save secret tracking data")
		return err
	}

	// Calculate and update summary in CR status
	configMapName := getSecretTrackingConfigMapName(instance.Name)
	instance.Status.SecretDeployment = computeDeploymentSummary(trackingData, totalNodes, configMapName)

	Log.Info("Secret deployment tracking updated",
		"totalNodes", instance.Status.SecretDeployment.TotalNodes,
		"updatedNodes", instance.Status.SecretDeployment.UpdatedNodes,
		"allNodesUpdated", instance.Status.SecretDeployment.AllNodesUpdated)

	return nil
}

// detectSecretDrift checks if cluster secrets (Expected) differ from what's deployed to nodes (Current).
// Updates Expected from cluster and compares with Current to detect drift.
//
// This function is fail-safe: if tracking ConfigMap is missing or can't be read,
// it assumes drift (blocks deletion until tracking is updated).
//
// Returns true if drift is detected (Expected != Current, meaning cluster changed but not deployed yet).
func (r *OpenStackDataPlaneNodeSetReconciler) detectSecretDrift(
	ctx context.Context,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	trackingData *SecretTrackingData,
) (bool, error) {
	Log := r.GetLogger(ctx)

	if trackingData == nil || len(trackingData.Secrets) == 0 {
		// No tracking data yet - we don't know what's on nodes
		// Fail-safe: assume drift (don't allow deletions until we have tracking)
		Log.Info("No tracking data available, assuming drift (fail-safe)")
		return true, nil
	}

	driftDetected := false

	// Check each tracked secret
	// Fetch from cluster (Expected) and compare with Current (what's on nodes)
	for secretName, secretInfo := range trackingData.Secrets {
		// Fetch current secret from cluster
		clusterSecret := &corev1.Secret{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: instance.Namespace,
		}, clusterSecret)

		var expectedResourceVersion string
		var expectedGeneration int64

		var expectedHash string

		if err != nil {
			if k8s_errors.IsNotFound(err) {
				// Secret deleted from cluster
				Log.Info("Secret drift detected: secret deleted from cluster",
					"secret", secretName,
					"currentHash", secretInfo.CurrentHash,
					"currentResourceVersion", secretInfo.CurrentResourceVersion)
				expectedHash = ""
				expectedResourceVersion = ""
				expectedGeneration = 0
				driftDetected = true

				// Update Expected in tracking to reflect deletion
				secretInfo.ExpectedHash = expectedHash
				secretInfo.ExpectedResourceVersion = expectedResourceVersion
				secretInfo.ExpectedGeneration = expectedGeneration
				trackingData.Secrets[secretName] = secretInfo
			} else {
				// Error fetching secret - fail-safe: assume drift
				Log.Error(err, "Failed to fetch secret for drift detection, assuming drift (fail-safe)",
					"secret", secretName)
				return true, err
			}
		} else {
			// Compute hash of cluster secret for comparison
			// Use secret.Hash() from lib-common (deterministic, unlike util.ObjectHash)
			expectedHash, err := secret.Hash(clusterSecret)
			if err != nil {
				// Can't compute hash - fail-safe: assume drift
				Log.Error(err, "Failed to compute secret hash for drift detection, assuming drift (fail-safe)",
					"secret", secretName)
				return true, err
			}

			// Get metadata for logging/debugging
			expectedResourceVersion = clusterSecret.ResourceVersion
			expectedGeneration = clusterSecret.Generation

			// Update Expected in tracking
			secretInfo.ExpectedHash = expectedHash
			secretInfo.ExpectedResourceVersion = expectedResourceVersion
			secretInfo.ExpectedGeneration = expectedGeneration
			trackingData.Secrets[secretName] = secretInfo

			// Compare Expected (cluster) vs Current (deployed) using hash
			// Hash represents actual secret content, avoiding ResourceVersion timing issues
			if expectedHash != secretInfo.CurrentHash {
				Log.Info("Secret drift detected: cluster secret differs from deployed version",
					"secret", secretName,
					"currentHash", secretInfo.CurrentHash,
					"expectedHash", expectedHash,
					"currentResourceVersion", secretInfo.CurrentResourceVersion,
					"expectedResourceVersion", expectedResourceVersion)
				driftDetected = true
			}
		}
	}

	if driftDetected {
		Log.Info("Secret drift detected - cluster secrets differ from what's deployed on nodes",
			"secretsChecked", len(trackingData.Secrets))
	}

	return driftDetected, nil
}

// getNodesCoveredByDeployment determines which nodes were covered by a deployment
// based on the AnsibleLimit field
func getNodesCoveredByDeployment(
	deployment *dataplanev1.OpenStackDataPlaneDeployment,
	nodeset *dataplanev1.OpenStackDataPlaneNodeSet,
) []string {
	if deployment == nil || nodeset == nil {
		return []string{}
	}

	allNodes := getAllNodeNames(nodeset)

	// Check AnsibleLimit
	ansibleLimit := deployment.Spec.AnsibleLimit
	if ansibleLimit == "" || ansibleLimit == "*" {
		// All nodes covered
		return allNodes
	}

	// Parse AnsibleLimit (comma-separated list)
	limitParts := strings.Split(ansibleLimit, ",")

	coveredNodes := make([]string, 0, len(allNodes))
	for _, node := range allNodes {
		for _, part := range limitParts {
			part = strings.TrimSpace(part)

			// Exact match
			if part == node {
				coveredNodes = append(coveredNodes, node)
				break
			}

			// Wildcard matching
			if strings.HasSuffix(part, "*") {
				prefix := strings.TrimSuffix(part, "*")
				if strings.HasPrefix(node, prefix) {
					coveredNodes = append(coveredNodes, node)
					break
				}
			}
		}
	}

	return coveredNodes
}

// getAllNodeNames returns a list of all node names in the nodeset
func getAllNodeNames(nodeset *dataplanev1.OpenStackDataPlaneNodeSet) []string {
	if nodeset == nil {
		return []string{}
	}
	nodes := make([]string, 0, len(nodeset.Spec.Nodes))
	for nodeName := range nodeset.Spec.Nodes {
		nodes = append(nodes, nodeName)
	}
	return nodes
}
