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
	"time"

	"github.com/go-playground/validator/v10"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	ansibleeev1 "github.com/openstack-k8s-operators/openstack-ansibleee-operator/api/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	deployment "github.com/openstack-k8s-operators/openstack-operator/pkg/dataplane"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/pkg/dataplane/util"
)

// OpenStackDataPlaneDeploymentReconciler reconciles a OpenStackDataPlaneDeployment object
type OpenStackDataPlaneDeploymentReconciler struct {
	client.Client
	Kclient kubernetes.Interface
	Scheme  *runtime.Scheme
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackDataPlaneDeploymentReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackDataPlaneDeployment")
}

//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanedeployments,verbs=get;list;watch;create;delete
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanedeployments/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanedeployments/finalizers,verbs=update;patch
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplanenodesets,verbs=get;list;watch
//+kubebuilder:rbac:groups=dataplane.openstack.org,resources=openstackdataplaneservices,verbs=get;list;watch
//+kubebuilder:rbac:groups=ansibleee.openstack.org,resources=openstackansibleees,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;create;update;patch;delete;
//+kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;
//+kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
func (r *OpenStackDataPlaneDeploymentReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)
	Log.Info("Reconciling Deployment")

	// Check if deployment name matches RFC1123 for use in labels
	validate := validator.New()
	if err := validate.Var(req.Name, "hostname_rfc1123"); err != nil {
		Log.Error(err, "error validating OpenStackDataPlaneDeployment name, the name must follow RFC1123")
		return ctrl.Result{}, err
	}
	// Fetch the OpenStackDataPlaneDeployment instance
	instance := &dataplanev1.OpenStackDataPlaneDeployment{}
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

	// If the deploy is already done, return immediately.
	if instance.Status.Deployed {
		Log.Info("Already deployed", "instance.Status.Deployed", instance.Status.Deployed)
		return ctrl.Result{}, nil
	}

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
	instance.InitHashesAndImages()

	// Set ObservedGeneration since we've reset conditions
	instance.Status.ObservedGeneration = instance.Generation

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() { // update the Ready condition based on the sub conditions
		if instance.Status.Conditions.AllSubConditionIsTrue() {
			instance.Status.Conditions.MarkTrue(
				condition.ReadyCondition, condition.ReadyMessage)
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

	// Ensure NodeSets
	nodeSets := dataplanev1.OpenStackDataPlaneNodeSetList{}
	for _, nodeSet := range instance.Spec.NodeSets {

		// Fetch the OpenStackDataPlaneNodeSet instance
		nodeSetInstance := &dataplanev1.OpenStackDataPlaneNodeSet{}
		err := r.Client.Get(
			ctx,
			types.NamespacedName{
				Namespace: instance.GetNamespace(),
				Name:      nodeSet,
			},
			nodeSetInstance)
		if err != nil {
			// NodeSet not found, force a requeue
			if k8s_errors.IsNotFound(err) {
				Log.Info("NodeSet not found", "NodeSet", nodeSet)
				return ctrl.Result{RequeueAfter: time.Second * time.Duration(instance.Spec.DeploymentRequeueTime)}, nil
			}
			instance.Status.Conditions.MarkFalse(
				dataplanev1.SetupReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				dataplanev1.DataPlaneNodeSetErrorMessage,
				err.Error())
			// Error reading the object - requeue the request.
			return ctrl.Result{}, err
		}
		nodeSets.Items = append(nodeSets.Items, *nodeSetInstance)
	}

	// Check that all nodeSets are SetupReady
	for _, nodeSet := range nodeSets.Items {
		if !nodeSet.Status.Conditions.IsTrue(dataplanev1.SetupReadyCondition) {
			Log.Info("NodeSet SetupReadyCondition is not True", "NodeSet", nodeSet.Name)
			return ctrl.Result{RequeueAfter: time.Second * time.Duration(instance.Spec.DeploymentRequeueTime)}, nil
		}
	}

	// get TLS certs
	for _, nodeSet := range nodeSets.Items {
		if nodeSet.Spec.TLSEnabled {
			var services []string
			if len(instance.Spec.ServicesOverride) != 0 {
				services = instance.Spec.ServicesOverride
			} else {
				services = nodeSet.Spec.Services
			}
			nsConditions := instance.Status.NodeSetConditions[nodeSet.Name]

			for _, serviceName := range services {
				service, err := deployment.GetService(ctx, helper, serviceName)
				if err != nil {
					instance.Status.Conditions.MarkFalse(
						condition.InputReadyCondition,
						condition.ErrorReason,
						condition.SeverityError,
						dataplanev1.ServiceErrorMessage,
						err.Error())
					nsConditions.MarkFalse(
						dataplanev1.NodeSetDeploymentReadyCondition,
						condition.ErrorReason,
						condition.SeverityError,
						dataplanev1.ServiceErrorMessage,
						err.Error())
					return ctrl.Result{}, err
				}
				if service.Spec.TLSCerts != nil {
					for certKey := range service.Spec.TLSCerts {
						result, err := deployment.EnsureTLSCerts(ctx, helper, &nodeSet,
							nodeSet.Status.AllHostnames, nodeSet.Status.AllIPs, service, certKey)
						if err != nil {
							instance.Status.Conditions.MarkFalse(
								condition.InputReadyCondition,
								condition.ErrorReason,
								condition.SeverityError,
								condition.TLSInputErrorMessage,
								err.Error())
							nsConditions.MarkFalse(
								dataplanev1.NodeSetDeploymentReadyCondition,
								condition.ErrorReason,
								condition.SeverityError,
								condition.TLSInputErrorMessage,
								err.Error())
							return ctrl.Result{}, err
						} else if (*result != ctrl.Result{}) {
							return *result, nil // requeue here
						}
					}
				}
			}
		}
	}

	// All nodeSets successfully fetched.
	// Mark InputReadyCondition=True
	instance.Status.Conditions.MarkTrue(condition.InputReadyCondition, condition.InputReadyMessage)
	shouldRequeue := false
	haveError := false
	deploymentErrMsg := ""
	var nodesetServiceMap map[string][]string
	backoffLimitReached := false

	globalInventorySecrets := map[string]string{}
	globalSSHKeySecrets := map[string]string{}

	// Gathering individual inventory and ssh secrets for later use
	for _, nodeSet := range nodeSets.Items {
		// Add inventory secret to list of inventories for global services
		globalInventorySecrets[nodeSet.Name] = fmt.Sprintf("dataplanenodeset-%s", nodeSet.Name)
		globalSSHKeySecrets[nodeSet.Name] = nodeSet.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret
	}

	if instance.Spec.ServicesOverride == nil {
		if nodesetServiceMap, err = deployment.DedupServices(ctx, helper, nodeSets.Items); err != nil {
			util.LogErrorForObject(helper, err, "OpenStackDeployment error for deployment", instance)
			instance.Status.Conditions.MarkFalse(
				condition.DeploymentReadyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				dataplanev1.ServiceErrorMessage,
				err.Error())
			return ctrl.Result{}, err
		}
	}

	version, err := dataplaneutil.GetVersion(ctx, helper, instance.Namespace)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Deploy each nodeSet
	// The loop starts and checks NodeSet deployments sequentially. However, after they
	// are started, they are running in parallel, since the loop does not wait
	// for the first started NodeSet to finish before starting the next.
	for _, nodeSet := range nodeSets.Items {

		Log.Info(fmt.Sprintf("Deploying NodeSet: %s", nodeSet.Name))
		Log.Info("Set Status.Deployed to false", "instance", instance)
		instance.Status.Deployed = false
		Log.Info("Set DeploymentReadyCondition false")
		instance.Status.Conditions.MarkFalse(
			condition.DeploymentReadyCondition, condition.RequestedReason,
			condition.SeverityInfo, condition.DeploymentReadyRunningMessage)
		ansibleEESpec := nodeSet.GetAnsibleEESpec()
		ansibleEESpec.AnsibleTags = instance.Spec.AnsibleTags
		ansibleEESpec.AnsibleSkipTags = instance.Spec.AnsibleSkipTags
		ansibleEESpec.AnsibleLimit = instance.Spec.AnsibleLimit
		ansibleEESpec.ExtraVars = instance.Spec.AnsibleExtraVars

		if nodeSet.Status.DNSClusterAddresses != nil && nodeSet.Status.CtlplaneSearchDomain != "" {
			ansibleEESpec.DNSConfig = &corev1.PodDNSConfig{
				Nameservers: nodeSet.Status.DNSClusterAddresses,
				Searches:    []string{nodeSet.Status.CtlplaneSearchDomain},
			}
		}

		deployer := deployment.Deployer{
			Ctx:                         ctx,
			Helper:                      helper,
			NodeSet:                     &nodeSet,
			Deployment:                  instance,
			Status:                      &instance.Status,
			AeeSpec:                     &ansibleEESpec,
			InventorySecrets:            globalInventorySecrets,
			AnsibleSSHPrivateKeySecrets: globalSSHKeySecrets,
			Version:                     version,
		}

		// When ServicesOverride is set on the OpenStackDataPlaneDeployment,
		// deploy those services for each OpenStackDataPlaneNodeSet. Otherwise,
		// deploy with the OpenStackDataPlaneNodeSet's Services.
		var deployResult *ctrl.Result
		if len(instance.Spec.ServicesOverride) != 0 {
			deployResult, err = deployer.Deploy(instance.Spec.ServicesOverride)
		} else {
			deployResult, err = deployer.Deploy(nodesetServiceMap[nodeSet.Name])
		}

		nsConditions := instance.Status.NodeSetConditions[nodeSet.Name]
		nsConditions.Set(nsConditions.Mirror(dataplanev1.NodeSetDeploymentReadyCondition))

		if err != nil {
			util.LogErrorForObject(helper, err, fmt.Sprintf("OpenStackDeployment error for NodeSet %s", nodeSet.Name), instance)
			Log.Info("Set NodeSetDeploymentReadyCondition false", "nodeSet", nodeSet.Name)
			haveError = true
			errMsg := fmt.Sprintf("nodeSet: %s error: %s", nodeSet.Name, err.Error())
			if len(deploymentErrMsg) == 0 {
				deploymentErrMsg = errMsg
			} else {
				deploymentErrMsg = fmt.Sprintf("%s & %s", deploymentErrMsg, errMsg)
			}
			errorReason := nsConditions.Get(dataplanev1.NodeSetDeploymentReadyCondition).Reason
			backoffLimitReached = errorReason == condition.JobReasonBackoffLimitExceeded
		}

		if deployResult != nil {
			shouldRequeue = true
		} else {
			Log.Info("OpenStackDeployment succeeded for NodeSet", "NodeSet", nodeSet.Name)
			Log.Info("Set NodeSetDeploymentReadyCondition true", "nodeSet", nodeSet.Name)
			nsConditions.MarkTrue(
				dataplanev1.NodeSetDeploymentReadyCondition,
				condition.DeploymentReadyMessage)
			instance.Status.NodeSetHashes[nodeSet.Name] = nodeSet.Status.ConfigHash
		}
	}

	if haveError {
		var reason condition.Reason
		reason = condition.ErrorReason
		severity := condition.SeverityWarning
		if backoffLimitReached {
			reason = condition.JobReasonBackoffLimitExceeded
			severity = condition.SeverityError
		}
		instance.Status.Conditions.MarkFalse(
			condition.DeploymentReadyCondition,
			reason,
			severity,
			condition.DeploymentReadyErrorMessage,
			deploymentErrMsg)
		return ctrl.Result{}, fmt.Errorf(deploymentErrMsg)
	}

	if shouldRequeue {
		Log.Info("Not all NodeSets done for OpenStackDeployment")
		return ctrl.Result{}, nil
	}

	Log.Info("Set DeploymentReadyCondition true")
	instance.Status.Conditions.MarkTrue(condition.DeploymentReadyCondition, condition.DeploymentReadyMessage)
	Log.Info("Set Status.Deployed to true", "instance", instance)
	instance.Status.Deployed = true
	if version != nil {
		instance.Status.DeployedVersion = version.Spec.TargetVersion
	}
	err = r.setHashes(ctx, helper, instance, nodeSets)
	if err != nil {
		Log.Error(err, "Error setting service hashes")
	}
	return ctrl.Result{}, nil
}

func (r *OpenStackDataPlaneDeploymentReconciler) setHashes(
	ctx context.Context,
	helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneDeployment,
	nodeSets dataplanev1.OpenStackDataPlaneNodeSetList,
) error {
	var err error
	services := []string{}

	if len(instance.Spec.ServicesOverride) > 0 {
		services = instance.Spec.ServicesOverride
	} else {
		// get the union of services across nodesets
		type void struct{}
		var member void
		s := make(map[string]void)
		for _, nodeSet := range nodeSets.Items {
			for _, serviceName := range nodeSet.Spec.Services {
				s[serviceName] = member
			}
		}
		for service := range s {
			services = append(services, service)
		}
	}

	for _, serviceName := range services {
		err = deployment.GetDeploymentHashesForService(
			ctx,
			helper,
			instance.Namespace,
			serviceName,
			instance.Status.ConfigMapHashes,
			instance.Status.SecretHashes,
			nodeSets)
		if err != nil {
			return err
		}
	}

	for _, nodeSet := range nodeSets.Items {
		instance.Status.NodeSetHashes[nodeSet.Name] = nodeSet.Status.ConfigHash
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackDataPlaneDeploymentReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// watch for changes in certificates
	certFn := func(ctx context.Context, obj client.Object) []reconcile.Request {
		Log := r.GetLogger(ctx)
		result := []reconcile.Request{}

		objectLabelValue, ok := obj.GetLabels()[deployment.NodeSetLabel]
		if !ok {
			// cert doesn't have a nodeset label
			return nil
		}

		// get all deployments in namespace
		deployments := &dataplanev1.OpenStackDataPlaneDeploymentList{}
		listOpts := []client.ListOption{
			client.InNamespace(obj.GetNamespace()),
		}
		if err := r.Client.List(context.Background(), deployments, listOpts...); err != nil {
			Log.Error(err, "Unable to retrieve deployments %w")
			return nil
		}

		for _, dep := range deployments.Items {
			if dep.Status.Deployed {
				continue
			}
			if util.StringInSlice(objectLabelValue, dep.Spec.NodeSets) {
				name := client.ObjectKey{
					Namespace: dep.GetNamespace(),
					Name:      dep.GetName(),
				}
				Log.Info(fmt.Sprintf("Cert %s is used by deployment %s", obj.GetName(), dep.GetName()))
				result = append(result, reconcile.Request{NamespacedName: name})
			}
		}

		if len(result) > 0 {
			return result
		}
		return nil
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&dataplanev1.OpenStackDataPlaneDeployment{}).
		Owns(&ansibleeev1.OpenStackAnsibleEE{}).
		Watches(&certmgrv1.Certificate{},
			handler.EnqueueRequestsFromMapFunc(certFn)).
		Complete(r)
}
