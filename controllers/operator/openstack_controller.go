/*
Copyright 2024.

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

// Package operator contains the OpenStack operator controller implementation
package operator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	operatorv1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/operator/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/operator"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/operator/bindata"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// OpenStackReconciler reconciles a OpenStack object
type OpenStackReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func (r *OpenStackReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackOperator")
}

var (
	envRelatedOperatorImages         (map[string]*string) // operatorName -> image
	envRelatedOpenStackServiceImages (map[string]*string) // full_related_image_name -> image
	operatorImage                    string
	kubeRbacProxyImage               string
	openstackReleaseVersion          string
	leaseDuration                    string
	renewDeadline                    string
	retryPeriod                      string
)

// SetupEnv -
func SetupEnv() {
	envRelatedOperatorImages = make(map[string]*string)
	envRelatedOpenStackServiceImages = make(map[string]*string)
	for _, name := range os.Environ() {
		envArr := strings.Split(name, "=")

		if strings.HasSuffix(envArr[0], "_OPERATOR_MANAGER_IMAGE_URL") {
			operatorName := strings.TrimPrefix(envArr[0], "RELATED_IMAGE_")
			operatorName = strings.TrimSuffix(operatorName, "_OPERATOR_MANAGER_IMAGE_URL")
			operatorName = strings.ToLower(operatorName)
			operatorName = strings.ReplaceAll(operatorName, "_", "-")
			envRelatedOperatorImages[operatorName] = &envArr[1]
			log.Log.Info("Found operator related image", "operator", operatorName, "image", envArr[1])
		} else if strings.HasPrefix(envArr[0], "RELATED_IMAGE_") {
			envRelatedOpenStackServiceImages[envArr[0]] = &envArr[1]
		} else if envArr[0] == "KUBE_RBAC_PROXY" {
			kubeRbacProxyImage = envArr[1]
		} else if envArr[0] == "OPERATOR_IMAGE_URL" {
			operatorImage = envArr[1]
			envRelatedOperatorImages[operatorv1beta1.OpenStackOperatorName] = &operatorImage
		} else if envArr[0] == "OPENSTACK_RELEASE_VERSION" {
			openstackReleaseVersion = envArr[1]
		} else if envArr[0] == "LEASE_DURATION" {
			leaseDuration = envArr[1]
		} else if envArr[0] == "RENEW_DEADLINE" {
			renewDeadline = envArr[1]
		} else if envArr[0] == "RETRY_PERIOD" {
			retryPeriod = envArr[1]
		}
	}
}

//+kubebuilder:rbac:groups=operator.openstack.org,resources=openstacks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.openstack.org,resources=openstacks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.openstack.org,resources=openstacks/finalizers,verbs=update
// +kubebuilder:rbac:groups=admissionregistration.k8s.io,resources=mutatingwebhookconfigurations;validatingwebhookconfigurations,verbs="*"
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles;clusterrolebindings;rolebindings;roles,verbs="*"
// +kubebuilder:rbac:groups=apiextensions.k8s.io,resources="*",verbs="*"
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups="",resources=serviceaccounts;configmaps;namespaces,verbs="*"
// +kubebuilder:rbac:groups=core,resources=services,verbs="*";
// +kubebuilder:rbac:groups=discovery.k8s.io,resources=endpointslices,verbs=get;list;watch;
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors,verbs=list;get;watch;update;create
// +kubebuilder:rbac:groups=operators.coreos.com,resources=clusterserviceversions;subscriptions;installplans;operators,verbs=get;list;delete;

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *OpenStackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)

	// Fetch the OpenStack instance
	instanceList := &operatorv1beta1.OpenStackList{}
	err := r.List(ctx, instanceList, &client.ListOptions{})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed listing all OpenStack instances")
	}
	instance := &operatorv1beta1.OpenStack{}
	err = r.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile req.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the req.
		return ctrl.Result{}, err
	}

	openstackHelper, err := helper.NewHelper(
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
		// Don't update the status, if reconciler Panics
		if r := recover(); r != nil {
			Log.Info(fmt.Sprintf("panic during reconcile %v\n", r))
			panic(r)
		}
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

		err := openstackHelper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	cl := condition.CreateList(
		condition.UnknownCondition(operatorv1beta1.OpenStackOperatorReadyCondition, condition.InitReason, string(operatorv1beta1.OpenStackOperatorReadyInitMessage)),
		condition.UnknownCondition(operatorv1beta1.OpenStackOperatorDeploymentsReadyCondition, condition.InitReason, string(operatorv1beta1.OpenStackOperatorDeploymentsReadyInitMessage)),
	)
	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation
	instance.Status.TotalOperatorCount = ptr.To(len(operatorv1beta1.OperatorList))

	instance.Status.Conditions.Set(condition.FalseCondition(
		operatorv1beta1.OpenStackOperatorReadyCondition,
		condition.RequestedReason,
		condition.SeverityInfo,
		operatorv1beta1.OpenStackOperatorReadyRunningMessage))

	// If we're not deleting this and the object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, openstackHelper.GetFinalizer()) || isNewInstance {
		return ctrl.Result{}, err
	}

	// We only want one instance of OpenStack. Ignore anything after that.
	if len(instanceList.Items) > 0 {
		if len(instanceList.Items) > 1 {
			sort.Slice(instanceList.Items, func(i, j int) bool {
				return instanceList.Items[j].CreationTimestamp.After(instanceList.Items[i].CreationTimestamp.Time)
			})
		}
		if instanceList.Items[0].Name != req.Name {
			Log.Info("Ignoring OpenStack.operator.openstack.org because one already exists and does not match existing name")
			err = r.Delete(ctx, instance, &client.DeleteOptions{})
			if err != nil {
				instance.Status.Conditions.Set(condition.FalseCondition(
					operatorv1beta1.OpenStackOperatorReadyCondition,
					condition.ErrorReason,
					condition.SeverityWarning,
					operatorv1beta1.OpenStackOperatorErrorMessage,
					err))
				Log.Error(err, "failed to remove OpenStack.operator.openstack.org instance")
			}
			return ctrl.Result{}, nil
		}
	}

	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, openstackHelper)
	}

	// cleanup obsolete resources here (remove old CSVs, etc)
	if err := r.cleanupObsoleteResources(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	if err := r.applyManifests(ctx, instance); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			operatorv1beta1.OpenStackOperatorReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			operatorv1beta1.OpenStackOperatorErrorMessage,
			err))
		return ctrl.Result{}, err
	}

	// now that CRDs have been updated (with old olm.managed references removed)
	// we can finally cleanup the old operators
	if err := r.postCleanupObsoleteResources(ctx, instance); err != nil {
		return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, err
	}

	// Check if all deployments are running
	deploymentsRunning, deploymentsPending, err := r.countDeployments(ctx, instance)
	instance.Status.DeployedOperatorCount = &deploymentsRunning
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			operatorv1beta1.OpenStackOperatorReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			operatorv1beta1.OpenStackOperatorErrorMessage,
			err))
		return ctrl.Result{}, err
	}
	if deploymentsRunning < *instance.Status.EnabledOperatorCount {
		Log.Info("Waiting for all deployments to be running", "current", deploymentsRunning, "expected", *instance.Status.EnabledOperatorCount, "pending", deploymentsPending)
		instance.Status.Conditions.Set(condition.FalseCondition(
			operatorv1beta1.OpenStackOperatorDeploymentsReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			operatorv1beta1.OpenStackOperatorDeploymentsReadyRunningMessage,
			deploymentsPending))
		return ctrl.Result{}, nil
	}

	instance.Status.Conditions.MarkTrue(
		operatorv1beta1.OpenStackOperatorDeploymentsReadyCondition,
		operatorv1beta1.OpenStackOperatorDeploymentsReadyMessage)

	// Check if Services are running and have an endpoint
	ctrlResult, err := r.checkServiceEndpoints(ctx, instance)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			operatorv1beta1.OpenStackOperatorReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			operatorv1beta1.OpenStackOperatorErrorMessage,
			err))
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	instance.Status.Conditions.MarkTrue(
		operatorv1beta1.OpenStackOperatorReadyCondition,
		operatorv1beta1.OpenStackOperatorReadyMessage)

	Log.Info("Reconcile complete.")
	return ctrl.Result{}, nil

}

func (r *OpenStackReconciler) reconcileDelete(ctx context.Context, instance *operatorv1beta1.OpenStack, helper *helper.Helper) (ctrl.Result, error) {
	Log := r.GetLogger(ctx)
	Log.Info("Reconciling OpenStack initialization resource delete")

	// validating webhook cleanup
	valWebhooks, err := r.Kclient.AdmissionregistrationV1().ValidatingWebhookConfigurations().List(ctx, metav1.ListOptions{
		LabelSelector: "openstack.openstack.org/managed=true",
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed listing validating webhook configurations")
	}
	for _, webhook := range valWebhooks.Items {
		err := r.Kclient.AdmissionregistrationV1().ValidatingWebhookConfigurations().Delete(ctx, webhook.Name, metav1.DeleteOptions{})
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to cleanup webhook")
		}
		fmt.Println("Found ValidatingWebhookConfiguration:", webhook.Name)

	}

	// mutating webhook cleanup
	mutWebhooks, err := r.Kclient.AdmissionregistrationV1().MutatingWebhookConfigurations().List(ctx, metav1.ListOptions{
		LabelSelector: "openstack.openstack.org/managed=true",
	})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed listing validating webhook configurations")
	}
	for _, webhook := range mutWebhooks.Items {
		err := r.Kclient.AdmissionregistrationV1().MutatingWebhookConfigurations().Delete(ctx, webhook.Name, metav1.DeleteOptions{})
		if err != nil {
			return ctrl.Result{}, errors.Wrap(err, "failed to cleanup webhook")
		}
		fmt.Println("Found MutatingWebhookConfiguration:", webhook.Name)

	}

	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())

	return ctrl.Result{}, nil
}

// countDeployments -
func (r *OpenStackReconciler) countDeployments(ctx context.Context, instance *operatorv1beta1.OpenStack) (int, []string, error) {
	deployments := &appsv1.DeploymentList{}
	pending := []string{}
	err := r.List(ctx, deployments, &client.ListOptions{Namespace: instance.Namespace})
	if err != nil {
		return 0, pending, err
	}

	count := 0
	for _, deployment := range deployments.Items {
		if metav1.IsControlledBy(&deployment, instance) {
			if deployment.Status.ReadyReplicas > 0 {
				count++
			} else {
				name := strings.Replace(deployment.Name, "-operator-controller-manager", "", 1)
				name = strings.Replace(name, "-cluster-operator-manager", "", 1)
				pending = append(pending, name)
			}
		}
	}
	sort.Strings(pending)
	return count, pending, nil
}

// containerImageMatch - returns true if the deployedContainerImage matches the operatorImage
func containerImageMatch(instance *operatorv1beta1.OpenStack) bool {
	if instance.Status.ContainerImage != nil && *instance.Status.ContainerImage == operatorImage {
		return true
	}
	return false
}

func isWebhookEndpoint(name string) bool {
	// NOTE: this is a static list for all operators with webhooks enabled
	endpointNames := []string{"openstack-operator-webhook-service", "infra-operator-webhook-service", "openstack-baremetal-operator-webhook-service"}
	for _, prefix := range endpointNames {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// checkServiceEndpoints -
func (r *OpenStackReconciler) checkServiceEndpoints(ctx context.Context, instance *operatorv1beta1.OpenStack) (ctrl.Result, error) {

	Log := r.GetLogger(ctx)

	endpointSliceList := &discoveryv1.EndpointSliceList{}
	err := r.List(ctx, endpointSliceList, &client.ListOptions{Namespace: instance.Namespace})
	if err != nil {
		if apierrors.IsNotFound(err) {
			Log.Info("Webhook endpoint not found. Requeuing...")
			return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
		}
		return ctrl.Result{}, err
	}

	for _, endpointSlice := range endpointSliceList.Items {
		endpointSliceName := endpointSlice.GetName()

		if isWebhookEndpoint(endpointSliceName) {
			// is deployment disabled ?
			disabled := false
			for _, op := range instance.Spec.OperatorOverrides {
				if strings.HasPrefix(endpointSliceName, op.Name+"-operator") &&
					op.Replicas != nil && *op.Replicas == 0 {

					disabled = true
					Log.Info(fmt.Sprintf("Webhook %s disabled, skipping endpoint slice check", endpointSliceName), "name", endpointSlice.GetName())
					break
				}
			}
			if disabled {
				continue
			}

			if len(endpointSlice.Endpoints) == 0 {
				Log.Info("Webhook endpoint not configured. Requeuing...", "name", endpointSlice.GetName())
				return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
			}
			for _, endpoint := range endpointSlice.Endpoints {
				if len(endpoint.Addresses) == 0 {
					Log.Info("Webhook endpoint addresses aren't healthy. Requeuing...", "name", endpointSlice.GetName())
					return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
				}
				bFalse := false
				if endpoint.Conditions.Ready == &bFalse || endpoint.Conditions.Serving == &bFalse {
					Log.Info("Webhook endpoint addresses aren't serving. Requeuing...", "name", endpointSlice.GetName())
					return ctrl.Result{RequeueAfter: time.Duration(5) * time.Second}, nil
				}
			}
		}
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackReconciler) applyManifests(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	Log := r.GetLogger(ctx)
	// only apply CRDs and RBAC once per each containerImage change
	if !containerImageMatch(instance) {
		if err := r.applyCRDs(ctx, instance); err != nil {
			Log.Error(err, "failed applying CRD manifests")
			return err
		}

		if err := r.applyRBAC(ctx, instance); err != nil {
			Log.Error(err, "failed applying RBAC manifests")
			return err
		}
	}
	instance.Status.ContainerImage = &operatorImage

	if err := r.applyOperator(ctx, instance); err != nil {
		Log.Error(err, "failed applying Operator manifests")
		return err
	}

	return nil
}

func (r *OpenStackReconciler) applyCRDs(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	data := bindata.MakeRenderData()
	return r.renderAndApply(ctx, instance, data, "crds", false)
}

func (r *OpenStackReconciler) applyRBAC(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	data := bindata.MakeRenderData()
	data.Data["OperatorNamespace"] = instance.Namespace
	return r.renderAndApply(ctx, instance, data, "rbac", true)
}

func (r *OpenStackReconciler) applyOperator(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	kubeRbacProxyContainer := operator.Container{
		Image: kubeRbacProxyImage,
		Resources: operator.Resource{
			Limits: &operator.ResourceList{
				CPU:    operatorv1beta1.DefaultRbacProxyCPULimit.String(),
				Memory: operatorv1beta1.DefaultRbacProxyMemoryLimit.String(),
			},
			Requests: &operator.ResourceList{
				CPU:    operatorv1beta1.DefaultRbacProxyCPURequests.String(),
				Memory: operatorv1beta1.DefaultRbacProxyMemoryRequests.String(),
			},
		},
	}
	defaultEnv := []corev1.EnvVar{
		{
			Name:  "LEASE_DURATION",
			Value: leaseDuration,
		},
		{
			Name:  "RENEW_DEADLINE",
			Value: renewDeadline,
		},
		{
			Name:  "RETRY_PERIOD",
			Value: retryPeriod,
		}}

	relatedImagesEnv := []corev1.EnvVar{}
	// maps are not sorted in go, make sure to add the images in a sorted way,
	// otherwise the env vars content will change regularly
	keys := make([]string, 0, len(envRelatedOpenStackServiceImages))
	for k := range envRelatedOpenStackServiceImages {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, name := range keys {
		if img, ok := envRelatedOpenStackServiceImages[name]; ok && img != nil {
			relatedImagesEnv = append(relatedImagesEnv,
				corev1.EnvVar{
					Name:  name,
					Value: *img,
				})
		}
	}

	// all othere serviceOperators
	operators := []operator.Operator{} // operator details
	disabledOperators := 0
	enabledOperators := 0
	for _, op := range operatorv1beta1.OperatorList {
		if img, ok := envRelatedOperatorImages[op.Name]; ok && img != nil {
			// set global defaults
			serviceOp := operator.Operator{
				Name:      op.Name,
				Namespace: instance.Namespace,
				Deployment: operator.Deployment{
					Replicas: ptr.To(operatorv1beta1.ReplicasEnabled),
					Manager: operator.Container{
						Image: *img,
						Env:   defaultEnv,
						Resources: operator.Resource{
							Limits: &operator.ResourceList{
								CPU:    operatorv1beta1.DefaultManagerCPULimit.String(),
								Memory: operatorv1beta1.DefaultManagerMemoryLimit.String(),
							},
							Requests: &operator.ResourceList{
								CPU:    operatorv1beta1.DefaultManagerCPURequests.String(),
								Memory: operatorv1beta1.DefaultManagerMemoryRequests.String(),
							},
						},
					},
					KubeRbacProxy: kubeRbacProxyContainer,
					Tolerations:   operatorv1beta1.DefaultTolerations,
				},
			}

			switch op.Name {
			case operatorv1beta1.OpenStackOperatorName:
				// set the release version on the openstack-operator
				serviceOp.Deployment.Manager.Env = append(serviceOp.Deployment.Manager.Env,
					corev1.EnvVar{
						Name:  "OPENSTACK_RELEASE_VERSION",
						Value: openstackReleaseVersion,
					})
				// set related images on the openstack-operator
				serviceOp.Deployment.Manager.Env = append(serviceOp.Deployment.Manager.Env,
					relatedImagesEnv...)
			case operatorv1beta1.OpenStackBaremetalOperatorName:
				// enable webhooks on the openstack-operator
				serviceOp.Deployment.Manager.Env = append(serviceOp.Deployment.Manager.Env,
					corev1.EnvVar{
						Name:  "ENABLE_WEBHOOKS",
						Value: "true",
					})
				// set related images on the openstack-baremetal-operator
				serviceOp.Deployment.Manager.Env = append(serviceOp.Deployment.Manager.Env,
					relatedImagesEnv...)
			case operatorv1beta1.InfraOperatorName:
				// enable webhooks on the infra-operator
				serviceOp.Deployment.Manager.Env = append(serviceOp.Deployment.Manager.Env,
					corev1.EnvVar{
						Name:  "ENABLE_WEBHOOKS",
						Value: "true",
					})
			default:
				// disable webhooks per default
				serviceOp.Deployment.Manager.Env = append(serviceOp.Deployment.Manager.Env,
					corev1.EnvVar{
						Name:  "ENABLE_WEBHOOKS",
						Value: "false",
					})
			}

			// set operator defaults if defined
			operator.SetOverrides(op, &serviceOp)

			// set user overrides
			opOvr := operator.HasOverrides(instance.Spec.OperatorOverrides, op.Name)
			if opOvr != nil {
				operator.SetOverrides(*opOvr, &serviceOp)
			}

			operators = append(operators, serviceOp)

			if *serviceOp.Deployment.Replicas == 0 {
				disabledOperators++
			} else {
				enabledOperators++
			}
		}
	}
	instance.Status.DisabledOperatorCount = &disabledOperators
	instance.Status.EnabledOperatorCount = &enabledOperators

	// serviceOperators a copy of operators details, without openstack-operator and rabbitmq-cluster-operator
	// which get removed bellow when creating openstackOperator and rabbitmqOperator, since they use dedicated
	// templates
	serviceOperators := operators

	// openstack-operator-controller-manager
	openstackOperator := operator.Operator{}
	idx, op := operator.GetOperator(serviceOperators, operatorv1beta1.OpenStackOperatorName)
	if idx >= 0 {
		openstackOperator = op
		// remove openstackOperator from serviceOperators
		serviceOperators = append(serviceOperators[:idx], serviceOperators[idx+1:]...)
	}

	// rabbitmq-cluster-operator
	rabbitmqOperator := operator.Operator{}
	idx, op = operator.GetOperator(serviceOperators, operatorv1beta1.RabbitMQOperatorName)
	if idx >= 0 {
		rabbitmqOperator = op
		// remove rabbitmq-cluster-operator from serviceOperators
		serviceOperators = append(serviceOperators[:idx], serviceOperators[idx+1:]...)
	}

	data := bindata.MakeRenderData()

	// global stuff
	data.Data["OperatorNamespace"] = instance.Namespace

	// rabbitmaq-cluster-operator-manager image rabbit.yaml
	data.Data["RabbitmqOperator"] = rabbitmqOperator

	// openstack-operator-controller-manager image operator.yaml
	data.Data["OpenStackOperator"] = openstackOperator

	// service operators
	data.Data["ServiceOperators"] = serviceOperators // -> service operator images managers.yaml

	return r.renderAndApply(ctx, instance, data, "operator", true)
}

func (r *OpenStackReconciler) renderAndApply(
	ctx context.Context,
	instance *operatorv1beta1.OpenStack,
	data bindata.RenderData,
	sourceDirectory string,
	setControllerReference bool,
) error {
	var err error

	Log := r.GetLogger(ctx)
	bindir := util.GetEnvVar("BASE_BINDATA", "/bindata")

	sourceFullDirectory := filepath.Join(bindir, sourceDirectory)
	objs, err := bindata.RenderDir(sourceFullDirectory, &data)
	if err != nil {
		return errors.Wrapf(err, "failed to render openstack-operator - %s", sourceDirectory)
	}

	// If no file found in directory - return error
	if len(objs) == 0 {
		return fmt.Errorf("no manifests rendered from %s", sourceFullDirectory)
	}

	for _, obj := range objs {
		// RenderDir seems to add an extra null entry to the list. It appears to be because of the
		// nested templates. This just makes sure we don't try to apply an empty obj.
		if obj.GetName() == "" {
			continue
		}
		if setControllerReference {
			// Set the controller reference.
			if obj.GetNamespace() != "" {
				Log.Info("Setting controller reference", "object", obj.GetName(), "controller", instance.Name)
				err = controllerutil.SetControllerReference(instance, obj, r.Scheme)
				if err != nil {
					return errors.Wrap(err, "failed to set owner reference")
				}
			} else {
				Log.Info("skipping controller reference (cluster scoped)", "object", obj.GetName(), "controller", instance.Name)
			}
		}

		// Now apply the object
		err = bindata.ApplyObject(ctx, r.Client, obj)
		if err != nil {
			return errors.Wrapf(err, "failed to apply object %v", obj)
		}
	}
	return nil
}

func isServiceOperatorResource(name string) bool {
	for _, item := range operatorv1beta1.OperatorList {
		if item.Name == operatorv1beta1.OpenStackOperatorName {
			continue
		}
		if strings.Index(name, item.Name) == 0 {
			return true
		}
	}
	return false
}

// cleanupObsoleteResources - deletes CSVs and subscriptions
func (r *OpenStackReconciler) cleanupObsoleteResources(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	Log := r.GetLogger(ctx)

	csvGVR := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "clusterserviceversions",
	}

	subscriptionGVR := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}

	installPlanGVR := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "installplans",
	}

	csvList := &uns.UnstructuredList{}
	csvList.SetGroupVersionKind(csvGVR.GroupVersion().WithKind("ClusterServiceVersion"))
	err := r.List(ctx, csvList, &client.ListOptions{Namespace: instance.Namespace})
	if err != nil {
		return err
	}
	for _, csv := range csvList.Items {
		Log.Info("Found CSV", "name", csv.GetName())
		if isServiceOperatorResource(csv.GetName()) {
			err = r.Delete(ctx, &csv)
			if err != nil {
				if apierrors.IsNotFound(err) {
					Log.Info("CSV not found on delete. Continuing...", "name", csv.GetName())
					continue
				}
				return err
			}
			Log.Info("CSV deleted successfully", "name", csv.GetName())
		}
	}

	subscriptionList := &uns.UnstructuredList{}
	subscriptionList.SetGroupVersionKind(subscriptionGVR.GroupVersion().WithKind("Subscription"))
	err = r.List(ctx, subscriptionList, &client.ListOptions{Namespace: instance.Namespace})
	if err != nil {
		return err
	}
	for _, subscription := range subscriptionList.Items {
		Log.Info("Found Subscription", "name", subscription.GetName())
		if isServiceOperatorResource(subscription.GetName()) {
			err = r.Delete(ctx, &subscription)
			if err != nil {
				if apierrors.IsNotFound(err) {
					Log.Info("Subscription not found on delete. Continuing...", "name", subscription.GetName())
					continue
				}
				return err
			}
			Log.Info("Subscription deleted successfully", "name", subscription.GetName())
		}
	}

	// lookup the installplan which has the clusterServiceVersionNames we removed above
	// there will be just a single installPlan that has all of them referenced
	installPlanList := &uns.UnstructuredList{}
	installPlanList.SetGroupVersionKind(installPlanGVR.GroupVersion().WithKind("InstallPlan"))

	err = r.List(ctx, installPlanList, &client.ListOptions{Namespace: instance.Namespace})
	if err != nil {
		return err
	}
	for _, installPlan := range installPlanList.Items {
		Log.Info("Found installPlan", "name", installPlan.GetName())
		// this should have a list containing the CSV names of all the old/legacy service operator CSVs
		csvNames, found, err := uns.NestedSlice(installPlan.Object, "spec", "clusterServiceVersionNames")
		if err != nil {
			return err
		}
		if found {
			// just checking for the first one should be sufficient
			if isServiceOperatorResource(csvNames[0].(string)) {
				err = r.Delete(ctx, &installPlan)
				if err != nil {
					if apierrors.IsNotFound(err) {
						Log.Info("Installplane not found on delete. Continuing...", "name", installPlan.GetName())
						continue
					}
					return err
				}
				Log.Info("Installplan deleted successfully", "name", installPlan.GetName())
			}
		}
	}

	return nil

}

// postCleanupObsoleteResources - deletes CSVs for old service operator bundles
func (r *OpenStackReconciler) postCleanupObsoleteResources(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	Log := r.GetLogger(ctx)

	operatorGVR := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1",
		Resource: "operators",
	}

	// finally we can remove operator objects as all the refs have been cleaned up:
	// 1) CSVs
	// 2) Subscriptions
	// 3) CRD olm.managed references removed
	// 4) installPlan from old service operators removed
	operatorList := &uns.UnstructuredList{}
	operatorList.SetGroupVersionKind(operatorGVR.GroupVersion().WithKind("Operator"))
	err := r.List(ctx, operatorList, &client.ListOptions{Namespace: instance.Namespace})
	if err != nil {
		return err
	}
	for _, operator := range operatorList.Items {
		Log.Info("Found Operator", "name", operator.GetName())
		if isServiceOperatorResource(operator.GetName()) {

			refs, found, err := uns.NestedSlice(operator.Object, "status", "components", "refs")
			if err != nil {
				return err
			}
			if found {

				// The horizon-operator.openstack-operators has references to old roles/bindings
				// the code below will delete those references before continuing
				for _, ref := range refs {
					refData := ref.(map[string]interface{})
					Log.Info("Deleting operator reference", "Reference", ref)
					obj := uns.Unstructured{}
					obj.SetName(refData["name"].(string))

					// Some of the references are not namespaced, so we need to check if the namespace is present
					if namespace, ok := refData["namespace"]; ok {
						obj.SetNamespace(namespace.(string))
					}

					apiVersion := refData["apiVersion"].(string)
					apiParts := strings.Split(apiVersion, "/")
					objGvk := schema.GroupVersionResource{
						Resource: refData["kind"].(string),
					}
					// Some of the references have an apiVersion that lacks a group prefix
					if len(apiParts) > 1 {
						objGvk.Group = apiParts[0]
						objGvk.Version = apiParts[1]
						Log.Info("postCleanupObsoleteResources: Found apiVersion with group prefix", "apiVersion", apiVersion)
					} else {
						objGvk.Version = apiParts[0]
						Log.Info("postCleanupObsoleteResources: Found apiVersion without group prefix", "apiVersion", apiVersion)
					}
					obj.SetGroupVersionKind(objGvk.GroupVersion().WithKind(refData["kind"].(string)))

					// references from CRD's should be removed before this function is called
					// but this is a safeguard as we do not want to delete them
					if refData["kind"].(string) != "CustomResourceDefinition" {
						err = r.Delete(ctx, &obj)
						if err != nil {
							if apierrors.IsNotFound(err) {
								Log.Info("Object not found on delete. Continuing...", "name", obj.GetName())
								continue
							}
							return err
						}
					}
				}

				return fmt.Errorf("Requeuing/Found references for operator name: %s, refs: %v", operator.GetName(), refs)
			}
			// no refs found so we should be able to successfully delete the operator
			err = r.Delete(ctx, &operator)
			if err != nil {
				return err
			}
			Log.Info("Operator deleted successfully", "name", operator.GetName())
		}
	}

	return nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackReconciler) SetupWithManager(mgr ctrl.Manager) error {

	return ctrl.NewControllerManagedBy(mgr).
		Owns(&appsv1.Deployment{}).
		For(&operatorv1beta1.OpenStack{}).
		Complete(r)
}
