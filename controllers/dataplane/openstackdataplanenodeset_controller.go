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
	"fmt"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"golang.org/x/exp/slices"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/rolebinding"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/serviceaccount"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	deployment "github.com/openstack-k8s-operators/openstack-operator/pkg/dataplane"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/pkg/dataplane/util"
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
	Kclient kubernetes.Interface
	Scheme  *runtime.Scheme
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

	validate := validator.New()

	// Fetch the OpenStackDataPlaneNodeSet instance
	instance := &dataplanev1.OpenStackDataPlaneNodeSet{}
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
	allIPSets, isReady, err := deployment.EnsureIPSets(ctx, helper, instance)
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
			condition.RequestedReason,
			condition.SeverityError,
			err.Error())
		return result, err
	} else if (result != ctrl.Result{}) {
		instance.Status.Conditions.MarkFalse(
			condition.InputReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
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

	// Reconcile BaremetalSet if required
	if !instance.Spec.PreProvisioned {
		// Reset the NodeSetBareMetalProvisionReadyCondition to unknown
		instance.Status.Conditions.MarkUnknown(dataplanev1.NodeSetBareMetalProvisionReadyCondition,
			condition.InitReason, condition.InitReason)

		isReady, err := deployment.DeployBaremetalSet(ctx, helper, instance,
			allIPSets, dnsDetails.ServerAddresses, containerImages)
		if err != nil || !isReady {
			return ctrl.Result{}, err
		}
	}

	isDeploymentReady, isDeploymentRunning, isDeploymentFailed, failedDeployment, err := checkDeployment(
		ctx, helper, instance)
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

	if !isDeploymentRunning && !isDeploymentFailed {
		// Generate NodeSet Inventory
		_, err = deployment.GenerateNodeSetInventory(ctx, helper, instance,
			allIPSets, dnsDetails.ServerAddresses, containerImages)
		if err != nil {
			errorMsg := fmt.Sprintf("Unable to generate inventory for %s", instance.Name)
			util.LogErrorForObject(helper, err, errorMsg, instance)
			instance.Status.Conditions.MarkFalse(
				dataplanev1.SetupReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				dataplanev1.DataPlaneNodeSetErrorMessage,
				errorMsg)
			return ctrl.Result{}, err
		}
	}
	// all setup tasks complete, mark SetupReadyCondition True
	instance.Status.Conditions.MarkTrue(dataplanev1.SetupReadyCondition, condition.ReadyMessage)

	// Set DeploymentReadyCondition to False if it was unknown.
	// Handles the case where the NodeSet is created, but not yet deployed.
	if instance.Status.Conditions.IsUnknown(condition.DeploymentReadyCondition) {
		Log.Info("Set NodeSet DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(condition.DeploymentReadyCondition,
			condition.RequestedReason, condition.SeverityInfo,
			condition.DeploymentReadyInitMessage)
	}

	if isDeploymentReady {
		Log.Info("Set NodeSet DeploymentReadyCondition true")
		instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition,
			condition.DeploymentReadyMessage)
	} else if isDeploymentRunning {
		Log.Info("Deployment still running...", "instance", instance)
		Log.Info("Set NodeSet DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(condition.DeploymentReadyCondition,
			condition.RequestedReason, condition.SeverityInfo,
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
		instance.Status.Conditions.MarkFalse(condition.DeploymentReadyCondition,
			condition.ErrorReason, condition.SeverityError,
			deployErrorMsg)
	}

	return ctrl.Result{}, err
}

func checkDeployment(ctx context.Context, helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
) (bool, bool, bool, string, error) {
	// Get all completed deployments
	var failedDeployment string
	deployments := &dataplanev1.OpenStackDataPlaneDeploymentList{}
	opts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	err := helper.GetClient().List(ctx, deployments, opts...)
	if err != nil {
		helper.GetLogger().Error(err, "Unable to retrieve OpenStackDataPlaneDeployment CRs %v")
		return false, false, false, failedDeployment, err
	}

	var isDeploymentReady bool
	var isDeploymentRunning bool
	var isDeploymentFailed bool

	// Sort deployments from oldest to newest by the LastTransitionTime of
	// their DeploymentReadyCondition
	slices.SortFunc(deployments.Items, func(a, b dataplanev1.OpenStackDataPlaneDeployment) int {
		aReady := a.Status.Conditions.Get(condition.DeploymentReadyCondition)
		bReady := b.Status.Conditions.Get(condition.DeploymentReadyCondition)
		if aReady != nil && bReady != nil {
			if aReady.LastTransitionTime.Before(&bReady.LastTransitionTime) {
				return -1
			}
		}
		return 1
	})

	for _, deployment := range deployments.Items {
		if !deployment.DeletionTimestamp.IsZero() {
			continue
		}
		if slices.Contains(
			deployment.Spec.NodeSets, instance.Name) {

			// Reset the vars for every deployment
			isDeploymentReady = false
			isDeploymentRunning = false
			deploymentConditions := deployment.Status.NodeSetConditions[instance.Name]
			if instance.Status.DeploymentStatuses == nil {
				instance.Status.DeploymentStatuses = make(map[string]condition.Conditions)
			}
			instance.Status.DeploymentStatuses[deployment.Name] = deploymentConditions
			deploymentCondition := deploymentConditions.Get(dataplanev1.NodeSetDeploymentReadyCondition)
			if condition.IsError(deploymentCondition) {
				err = fmt.Errorf(deploymentCondition.Message)
				isDeploymentFailed = true
				failedDeployment = deployment.Name
				break
			} else if deploymentConditions.IsFalse(dataplanev1.NodeSetDeploymentReadyCondition) {
				isDeploymentRunning = true
			} else if deploymentConditions.IsTrue(dataplanev1.NodeSetDeploymentReadyCondition) {
				if deployment.Status.NodeSetHashes[instance.Name] != instance.Status.ConfigHash {
					continue
				}
				isDeploymentReady = true
				for k, v := range deployment.Status.ConfigMapHashes {
					instance.Status.ConfigMapHashes[k] = v
				}
				for k, v := range deployment.Status.SecretHashes {
					instance.Status.SecretHashes[k] = v
				}
				for k, v := range deployment.Status.ContainerImages {
					instance.Status.ContainerImages[k] = v
				}
				instance.Status.DeployedConfigHash = deployment.Status.NodeSetHashes[instance.Name]

				// Get list of services by name, either from ServicesOverride or
				// the NodeSet.
				var services []string
				if len(deployment.Spec.ServicesOverride) != 0 {
					services = deployment.Spec.ServicesOverride
				} else {
					services = instance.Spec.Services
				}

				// For each service, check if EDPMServiceType is "update", and
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
						return false, false, false, failedDeployment, err
					}

					if service.Spec.EDPMServiceType != "update" {
						continue
					}

					// An "update" service Deployment has been completed, so
					// set the NodeSet's DeployedVersion to the Deployment's
					// DeployedVersion.
					instance.Status.DeployedVersion = deployment.Status.DeployedVersion
				}
			}
		}
	}

	return isDeploymentReady, isDeploymentRunning, isDeploymentFailed, failedDeployment, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackDataPlaneNodeSetReconciler) SetupWithManager(
	ctx context.Context, mgr ctrl.Manager) error {
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
	return ctrl.NewControllerManagedBy(mgr).
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
			handler.EnqueueRequestsFromMapFunc(r.secretWatcherFn),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{})).
		Watches(&openstackv1.OpenStackVersion{},
			handler.EnqueueRequestsFromMapFunc(r.genericWatcherFn)).
		Complete(r)
}

func (r *OpenStackDataPlaneNodeSetReconciler) secretWatcherFn(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	Log := r.GetLogger(ctx)
	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	kind := strings.ToLower(obj.GetObjectKind().GroupVersionKind().Kind)
	selector := "spec.ansibleVarsFrom.ansible.configMaps"
	if kind == "secret" {
		selector = "spec.ansibleVarsFrom.ansible.secrets"
	}

	listOpts := &client.ListOptions{
		FieldSelector: fields.OneTermEqualSelector(selector, obj.GetName()),
		Namespace:     obj.GetNamespace(),
	}

	if err := r.List(ctx, nodeSets, listOpts); err != nil {
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

func (r *OpenStackDataPlaneNodeSetReconciler) genericWatcherFn(
	ctx context.Context, obj client.Object,
) []reconcile.Request {
	Log := r.GetLogger(ctx)
	nodeSets := &dataplanev1.OpenStackDataPlaneNodeSetList{}
	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
	}
	if err := r.Client.List(ctx, nodeSets, listOpts...); err != nil {
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
