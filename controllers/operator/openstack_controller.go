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

package operator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/go-logr/logr"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	operatorv1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/operator/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/operator/bindata"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	OperatorCount = 22
)

// OpenStackReconciler reconciles a OpenStack object
type OpenStackReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLog returns a logger object with a prefix of "controller.name" and aditional controller context fields
func (r *OpenStackReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackControlPlane")
}

var (
	envRelatedOperatorImages (map[string]*string) // operatorName -> image
	rabbitmqImage            string
	operatorImage            string
	openstackReleaseVersion  string
)

// SetupEnv -
func SetupEnv() {
	envRelatedOperatorImages = make(map[string]*string)
	for _, name := range os.Environ() {
		envArr := strings.Split(name, "=")

		if strings.HasSuffix(envArr[0], "_OPERATOR_MANAGER_IMAGE_URL") {
			operatorName := strings.TrimPrefix(envArr[0], "RELATED_IMAGE_")
			operatorName = strings.TrimSuffix(operatorName, "_OPERATOR_MANAGER_IMAGE_URL")
			operatorName = strings.ToLower(operatorName)
			operatorName = strings.ReplaceAll(operatorName, "_", "-")
			// rabbitmq-cluster is a special case with an alternate deployment template
			if operatorName == "rabbitmq-cluster" {
				rabbitmqImage = envArr[1]
			} else {
				envRelatedOperatorImages[operatorName] = &envArr[1]
			}
			log.Log.Info("Found operator related image", "operator", operatorName, "image", envArr[1])
		} else if envArr[0] == "OPERATOR_IMAGE_URL" {
			operatorImage = envArr[1]
		} else if envArr[0] == "OPENSTACK_RELEASE_VERSION" {
			openstackReleaseVersion = envArr[1]
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
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups="monitoring.coreos.com",resources=servicemonitors,verbs=list;get;watch;update;create

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *OpenStackReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)

	// Fetch the OpenStack instance
	instanceList := &operatorv1beta1.OpenStackList{}
	err := r.Client.List(ctx, instanceList, &client.ListOptions{})
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed listing all OpenStack instances")
	}
	instance := &operatorv1beta1.OpenStack{}
	err = r.Client.Get(ctx, req.NamespacedName, instance)
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

	versionHelper, err := helper.NewHelper(
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

		err := versionHelper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	cl := condition.CreateList(
		condition.UnknownCondition(operatorv1beta1.OpenStackOperatorReadyCondition, condition.InitReason, string(operatorv1beta1.OpenStackOperatorReadyInitMessage)),
	)
	instance.Status.Conditions.Init(&cl)
	instance.Status.ObservedGeneration = instance.Generation

	instance.Status.Conditions.Set(condition.FalseCondition(
		operatorv1beta1.OpenStackOperatorReadyCondition,
		condition.RequestedReason,
		condition.SeverityInfo,
		operatorv1beta1.OpenStackOperatorReadyRunningMessage))

	// We only want one instance of OpenStack. Ignore anything after that.
	if len(instanceList.Items) > 0 {
		if len(instanceList.Items) > 1 {
			sort.Slice(instanceList.Items, func(i, j int) bool {
				return instanceList.Items[j].CreationTimestamp.After(instanceList.Items[i].CreationTimestamp.Time)
			})
		}
		if instanceList.Items[0].Name != req.Name {
			Log.Info("Ignoring OpenStack.operator.openstack.org because one already exists and does not match existing name")
			err = r.Client.Delete(ctx, instance, &client.DeleteOptions{})
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

	// TODO: cleanup obsolete resources here (remove old CSVs, etc)
	/*
	   if err := r.cleanupObsoleteResources(ctx); err != nil {
	           return ctrl.Result{}, err
	   }
	*/

	if err := r.applyManifests(ctx, instance); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			operatorv1beta1.OpenStackOperatorReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			operatorv1beta1.OpenStackOperatorErrorMessage,
			err))
		return ctrl.Result{}, err
	}

	// Check if all deployments are running
	deploymentsRunning, err := r.countDeployments(ctx, instance)
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
	if deploymentsRunning < OperatorCount {
		Log.Info("Waiting for all deployments to be running", "current", deploymentsRunning, "expected", OperatorCount)
		return ctrl.Result{}, nil
	}

	instance.Status.Conditions.MarkTrue(
		operatorv1beta1.OpenStackOperatorReadyCondition,
		operatorv1beta1.OpenStackOperatorReadyMessage)

	Log.Info("Reconcile complete.")
	return ctrl.Result{}, nil

}

// countDeployments -
func (r *OpenStackReconciler) countDeployments(ctx context.Context, instance *operatorv1beta1.OpenStack) (int, error) {
	deployments := &appsv1.DeploymentList{}
	err := r.Client.List(ctx, deployments, &client.ListOptions{Namespace: instance.Namespace})
	if err != nil {
		return 0, err
	}

	count := 0
	for _, deployment := range deployments.Items {
		if metav1.IsControlledBy(&deployment, instance) {
			if deployment.Status.ReadyReplicas > 0 {
				count++
			}
		}
	}
	return count, nil
}

func (r *OpenStackReconciler) applyManifests(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	if err := r.applyCRDs(ctx, instance); err != nil {
		log.Log.Error(err, "failed applying CRD manifests")
		return err
	}

	if err := r.applyRBAC(ctx, instance); err != nil {
		log.Log.Error(err, "failed applying RBAC manifests")
		return err
	}

	if err := r.applyOperator(ctx, instance); err != nil {
		log.Log.Error(err, "failed applying Operator manifests")
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
	return r.renderAndApply(ctx, instance, data, "rbac", false)
}

func (r *OpenStackReconciler) applyOperator(ctx context.Context, instance *operatorv1beta1.OpenStack) error {
	data := bindata.MakeRenderData()
	data.Data["OperatorNamespace"] = instance.Namespace
	data.Data["OperatorImages"] = envRelatedOperatorImages
	data.Data["RabbitmqImage"] = rabbitmqImage
	data.Data["OperatorImage"] = operatorImage
	data.Data["OpenstackReleaseVersion"] = openstackReleaseVersion
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
				log.Log.Info("Setting controller reference", "object", obj.GetName(), "controller", instance.Name)
				err = controllerutil.SetControllerReference(instance, obj, r.Scheme)
				if err != nil {
					return errors.Wrap(err, "failed to set owner reference")
				}
			} else {
				log.Log.Info("skipping controller reference (cluster scoped)", "object", obj.GetName(), "controller", instance.Name)
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

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackReconciler) SetupWithManager(mgr ctrl.Manager) error {

	deploymentFunc := handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		Log := r.GetLogger(ctx)

		instanceList := &operatorv1beta1.OpenStackList{}
		err := r.Client.List(ctx, instanceList)
		if err != nil {
			Log.Error(err, "Unable to retrieve OpenStack instances")
			return nil
		}

		if len(instanceList.Items) == 0 {
			return nil
		}

		instance := &instanceList.Items[0]
		if metav1.IsControlledBy(o, instance) {
			Log.Info("Reconcile request for OpenStack instance", "instance", instance.Name)
			return []reconcile.Request{
				{
					NamespacedName: client.ObjectKey{
						Namespace: instance.Namespace,
						Name:      instance.Name,
					},
				},
			}
		}

		return nil
	})

	return ctrl.NewControllerManagedBy(mgr).
		Watches(&appsv1.Deployment{}, deploymentFunc).
		For(&operatorv1beta1.OpenStack{}).
		Complete(r)
}
