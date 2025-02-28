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

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	routev1 "github.com/openshift/api/route/v1"
	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "k8s.io/api/core/v1"

	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"

	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"

	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	rabbitmqv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// OpenStackControlPlaneReconciler reconciles a OpenStackControlPlane object
type OpenStackControlPlaneReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
}

// GetLog returns a logger object with a prefix of "conroller.name" and aditional controller context fields
func (r *OpenStackControlPlaneReconciler) GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenStackControlPlane")
}

// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/finalizers,verbs=update;patch
// +kubebuilder:rbac:groups=core.openstack.org,resources=openstackversions,verbs=get;list;create
// +kubebuilder:rbac:groups=ironic.openstack.org,resources=ironics,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=client.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=placement.openstack.org,resources=placementapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=glance.openstack.org,resources=glances,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=heat.openstack.org,resources=heats,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=cinder.openstack.org,resources=cinders,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=manila.openstack.org,resources=manilas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nova.openstack.org,resources=nova,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=mariadb.openstack.org,resources=galeras,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=neutron.openstack.org,resources=neutronapis,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ovn.openstack.org,resources=ovndbclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ovn.openstack.org,resources=ovnnorthds,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ovn.openstack.org,resources=ovncontrollers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=network.openstack.org,resources=dnsmasqs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=telemetry.openstack.org,resources=telemetries,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=swift.openstack.org,resources=swifts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=octavia.openstack.org,resources=octavias,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=barbican.openstack.org,resources=barbicans,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=designate.openstack.org,resources=designates,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=redis.openstack.org,resources=redises,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=route.openshift.io,resources=routes/custom-host,verbs=create;update;patch
// +kubebuilder:rbac:groups=core,resources=services,verbs=get;list;
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=cert-manager.io,resources=issuers,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=cert-manager.io,resources=certificates,verbs=get;list;watch;create;update;patch;delete;
// +kubebuilder:rbac:groups=config.openshift.io,resources=networks,verbs=get;list;watch;
// +kubebuilder:rbac:groups=topology.openstack.org,resources=topologies,verbs=get;list;watch;update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	Log := r.GetLogger(ctx)
	// Fetch the OpenStackControlPlane instance
	instance := &corev1beta1.OpenStackControlPlane{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			Log.Info("OpenStackControlPlane instance is not found, probably deleted. Nothing to do.")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	helper, err := common_helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		// helper might be nil, so can't use util.LogErrorForObject since it requires helper as first arg
		Log.Error(err, "unable to acquire helper for OpenStackControlPlane")
		return ctrl.Result{}, err
	}
	//
	// initialize Conditions
	//
	isNewInstance := instance.Status.Conditions == nil
	if isNewInstance {
		instance.Status.Conditions = condition.Conditions{}
	}

	// Save a copy of the conditions so that we can restore the LastTransitionTime
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

		condition.RestoreLastTransitionTimes(&instance.Status.Conditions, savedConditions)

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	instance.InitConditions()
	instance.Status.ObservedGeneration = instance.Generation

	// If we're not deleting this and the service object doesn't have our finalizer, add it.
	if instance.DeletionTimestamp.IsZero() && controllerutil.AddFinalizer(instance, helper.GetFinalizer()) || isNewInstance {
		return ctrl.Result{}, nil
	}

	Log.Info("Looking up the current OpenStackVersion")
	ctrlResult, version, err := openstack.ReconcileVersion(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	versionHelper, err := common_helper.NewHelper(
		version,
		r.Client,
		r.Kclient,
		r.Scheme,
		Log,
	)
	if err != nil {
		Log.Error(err, "unable to create helper")
		return ctrl.Result{}, err
	}

	// Handle version delete
	if !instance.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, instance, version, helper, versionHelper)
	}

	// wait until the version is initialized so we have images on the version.Status
	if !version.Status.Conditions.IsTrue(corev1beta1.OpenStackVersionInitialized) {
		return ctrlResult, nil
	}

	if instance.Status.DeployedVersion == nil || version.Spec.TargetVersion == *instance.Status.DeployedVersion { //revive:disable:indent-error-flow
		// green field deployment or no minor update in progress
		ctrlResult, err := r.reconcileNormal(ctx, instance, version, helper)
		if err != nil {
			Log.Info("Error reconciling normal", "error", err)
			return ctrl.Result{}, err
		} else if (ctrlResult != ctrl.Result{}) {
			Log.Info("Reconciling normal")
			return ctrlResult, nil
		}
		// this will allow reconcileNormal to proceed in subsequent reconciles
		instance.Status.DeployedVersion = &version.Spec.TargetVersion
		return ctrl.Result{}, nil
	} else {
		if !version.Status.Conditions.IsTrue(corev1beta1.OpenStackVersionMinorUpdateOVNControlplane) {
			Log.Info("Minor update OVN on the ControlPlane")
			ctrlResult, err := r.reconcileOVNControllers(ctx, instance, version, helper)
			if err != nil {
				return ctrl.Result{}, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}
			instance.Status.DeployedOVNVersion = &version.Spec.TargetVersion
			return ctrl.Result{}, nil
		} else if version.Status.Conditions.IsTrue(corev1beta1.OpenStackVersionMinorUpdateOVNDataplane) &&
			!version.Status.Conditions.IsTrue(corev1beta1.OpenStackVersionMinorUpdateControlplane) {

			Log.Info("Minor update on the ControlPlane")
			ctrlResult, err := r.reconcileNormal(ctx, instance, version, helper)
			if err != nil {
				return ctrl.Result{}, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}
			// this will allow reconcileNormal to proceed in subsequent reconciles
			instance.Status.DeployedVersion = &version.Spec.TargetVersion
			return ctrl.Result{}, nil
		} else {
			Log.Info("Skipping reconcile. Waiting on minor update to proceed.")
			return ctrl.Result{}, nil
		}
	}
}

func (r *OpenStackControlPlaneReconciler) reconcileOVNControllers(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *common_helper.Helper) (ctrl.Result, error) {
	OVNControllerReady, err := openstack.ReconcileOVNController(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if !OVNControllerReady {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneOVNReadyRunningMessage))
	} else {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneOVNReadyCondition, corev1beta1.OpenStackControlPlaneOVNReadyMessage)
	}
	return ctrl.Result{}, nil
}

func (r *OpenStackControlPlaneReconciler) reconcileNormal(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *common_helper.Helper) (ctrl.Result, error) {
	if instance.Spec.TopologyRef != nil {
		if err := r.checkTopologyRef(ctx, helper,
			instance.Spec.TopologyRef, instance.Namespace); err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condition.TopologyReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				condition.TopologyReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, fmt.Errorf("waiting for Topology requirements: %w", err)
		}
		// TopologyRef != nil and exists and we're able to get it
		instance.Status.Conditions.MarkTrue(condition.TopologyReadyCondition, condition.TopologyReadyMessage)
	}

	ctrlResult, err := openstack.ReconcileCAs(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileDNSMasqs(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileRabbitMQs(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileGaleras(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileMemcacheds(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileKeystoneAPI(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcilePlacementAPI(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileGlance(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileCinder(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileOVN(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileNeutron(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileNova(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileHeat(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileIronic(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileOpenStackClient(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileManila(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileHorizon(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileTelemetry(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileBarbican(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileRedis(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileOctavia(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileDesignate(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileSwift(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileTest(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileInstanceHa(ctx, instance, version, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	return ctrl.Result{}, nil
}

func (r *OpenStackControlPlaneReconciler) reconcileDelete(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *common_helper.Helper, versionHelper *common_helper.Helper) (ctrl.Result, error) {
	helper.GetLogger().Info("reconcile delete")
	if controllerutil.RemoveFinalizer(version, versionHelper.GetFinalizer()) {
		err := versionHelper.PatchInstance(ctx, version)
		if err != nil {
			return ctrl.Result{}, err
		}
	}
	helper.GetLogger().Info(fmt.Sprintf("finalizer removed '%s' successfully", versionHelper.GetFinalizer()))

	// remove instance finalizer
	controllerutil.RemoveFinalizer(instance, helper.GetFinalizer())
	helper.GetLogger().Info(fmt.Sprintf("finalizer removed '%s' successfully", helper.GetFinalizer()))

	return ctrl.Result{}, nil
}

// fields to index to reconcile when change
const (
	passwordSecretField               = ".spec.secret"
	tlsCABundleSecretNameField        = ".spec.tls.caBundleSecretName"
	tlsIngressCACustomIssuer          = ".spec.tls.ingerss.ca.customIssuer"
	tlsPodLevelInternalCACustomIssuer = ".spec.tls.podLevel.internal.ca.customIssuer"
	tlsPodLevelOvnCACustomIssuer      = ".spec.tls.podLevel.ovn.ca.customIssuer"
)

var allWatchFields = []string{
	passwordSecretField,
	tlsCABundleSecretNameField,
	tlsIngressCACustomIssuer,
	tlsPodLevelInternalCACustomIssuer,
	tlsPodLevelOvnCACustomIssuer,
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackControlPlaneReconciler) SetupWithManager(
	ctx context.Context, mgr ctrl.Manager) error {
	// index passwordSecretField
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1beta1.OpenStackControlPlane{}, passwordSecretField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*corev1beta1.OpenStackControlPlane)
		if cr.Spec.Secret == "" {
			return nil
		}
		return []string{cr.Spec.Secret}
	}); err != nil {
		return err
	}

	// index caBundleSecretNameField
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1beta1.OpenStackControlPlane{}, tlsCABundleSecretNameField, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*corev1beta1.OpenStackControlPlane)
		if cr.Spec.TLS.CaBundleSecretName == "" {
			return nil
		}
		return []string{cr.Spec.TLS.CaBundleSecretName}
	}); err != nil {
		return err
	}

	// index tlsIngressCACustomIssuer
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1beta1.OpenStackControlPlane{}, tlsIngressCACustomIssuer, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*corev1beta1.OpenStackControlPlane)
		if cr.Spec.TLS.Ingress.Ca.CustomIssuer == nil {
			return nil
		}
		return []string{*cr.Spec.TLS.Ingress.Ca.CustomIssuer}
	}); err != nil {
		return err
	}

	// index tlsPodLevelInternalCACustomIssuer
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1beta1.OpenStackControlPlane{}, tlsPodLevelInternalCACustomIssuer, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*corev1beta1.OpenStackControlPlane)
		if cr.Spec.TLS.PodLevel.Internal.Ca.CustomIssuer == nil {
			return nil
		}
		return []string{*cr.Spec.TLS.PodLevel.Internal.Ca.CustomIssuer}
	}); err != nil {
		return err
	}

	// index tlsPodLevelOvnCACustomIssuer
	if err := mgr.GetFieldIndexer().IndexField(ctx, &corev1beta1.OpenStackControlPlane{}, tlsPodLevelOvnCACustomIssuer, func(rawObj client.Object) []string {
		// Extract the secret name from the spec, if one is provided
		cr := rawObj.(*corev1beta1.OpenStackControlPlane)
		if cr.Spec.TLS.PodLevel.Ovn.Ca.CustomIssuer == nil {
			return nil
		}
		return []string{*cr.Spec.TLS.PodLevel.Ovn.Ca.CustomIssuer}
	}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.OpenStackControlPlane{}).
		Owns(&clientv1.OpenStackClient{}).
		Owns(&networkv1.DNSMasq{}).
		Owns(&corev1.Secret{}).
		Owns(&mariadbv1.Galera{}).
		Owns(&memcachedv1.Memcached{}).
		Owns(&keystonev1.KeystoneAPI{}).
		Owns(&placementv1.PlacementAPI{}).
		Owns(&glancev1.Glance{}).
		Owns(&cinderv1.Cinder{}).
		Owns(&manilav1.Manila{}).
		Owns(&swiftv1.Swift{}).
		Owns(&rabbitmqv2.RabbitmqCluster{}).
		Owns(&ovnv1.OVNDBCluster{}).
		Owns(&ovnv1.OVNNorthd{}).
		Owns(&ovnv1.OVNController{}).
		Owns(&neutronv1.NeutronAPI{}).
		Owns(&novav1.Nova{}).
		Owns(&heatv1.Heat{}).
		Owns(&ironicv1.Ironic{}).
		Owns(&horizonv1.Horizon{}).
		Owns(&telemetryv1.Telemetry{}).
		Owns(&redisv1.Redis{}).
		Owns(&octaviav1.Octavia{}).
		Owns(&designatev1.Designate{}).
		Owns(&routev1.Route{}).
		Owns(&certmgrv1.Issuer{}).
		Owns(&certmgrv1.Certificate{}).
		Owns(&barbicanv1.Barbican{}).
		Owns(&corev1beta1.OpenStackVersion{}).
		Watches(
			&corev1.Secret{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSrc),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Watches(
			&certmgrv1.Issuer{},
			handler.EnqueueRequestsFromMapFunc(r.findObjectsForSrc),
			builder.WithPredicates(predicate.ResourceVersionChangedPredicate{}),
		).
		Complete(r)
}

func (r *OpenStackControlPlaneReconciler) findObjectsForSrc(ctx context.Context, src client.Object) []reconcile.Request {
	requests := []reconcile.Request{}

	l := log.FromContext(ctx).WithName("Controllers").WithName("OpenStackControlPlane")

	for _, field := range allWatchFields {
		crList := &corev1beta1.OpenStackControlPlaneList{}
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

// Verify the referenced topology exists
func (r *OpenStackControlPlaneReconciler) checkTopologyRef(
	ctx context.Context,
	h *helper.Helper,
	topologyRef *topologyv1.TopoRef,
	namespace string,
) error {
	if topologyRef.Namespace == "" {
		topologyRef.Namespace = namespace
	}
	_, _, err := topologyv1.GetTopologyByName(
		ctx,
		h,
		topologyRef.Name,
		topologyRef.Namespace,
	)
	if err != nil {
		return err
	}
	return nil
}
