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

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	clientv1beta1 "github.com/openstack-k8s-operators/infra-operator/apis/client/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

// OpenStackControlPlaneReconciler reconciles a OpenStackControlPlane object
type OpenStackControlPlaneReconciler struct {
	client.Client
	Scheme                        *runtime.Scheme
	Kclient                       kubernetes.Interface
	Log                           logr.Logger
	OpenStackClientContainerImage string
}

//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=client.openstack.org,resources=openstackclients,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ironic.openstack.org,resources=ironics,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=horizon.openstack.org,resources=horizons,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=placement.openstack.org,resources=placementapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=heat.openstack.org,resources=heats,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=cinder.openstack.org,resources=cinders,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=manila.openstack.org,resources=manilas,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=nova.openstack.org,resources=nova,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.openstack.org,resources=galeras,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=memcached.openstack.org,resources=memcacheds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=neutron.openstack.org,resources=neutronapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ovn.openstack.org,resources=ovndbclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ovn.openstack.org,resources=ovnnorthds,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ovn.openstack.org,resources=ovncontrollers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=network.openstack.org,resources=dnsmasqs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=telemetry.openstack.org,resources=ceilometercentrals,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, _err error) {
	_ = log.FromContext(ctx)

	// Fetch the OpenStackControlPlane instance
	instance := &corev1beta1.OpenStackControlPlane{}
	err := r.Client.Get(ctx, req.NamespacedName, instance)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected.
			// For additional cleanup logic use finalizers. Return and don't requeue.
			r.Log.Info("OpenStackControlPlane instance is not found, probaby deleted. Nothing to do.")
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		// helper might be nil, so can't use util.LogErrorForObject since it requires helper as first arg
		r.Log.Error(err, fmt.Sprintf("unable to acquire helper for OpenStackControlPlane %s", instance.Name))
		return ctrl.Result{}, err
	}

	// Always patch the instance status when exiting this function so we can persist any changes.
	defer func() {
		// update the overall status condition if service is ready
		if instance.IsReady() {
			instance.Status.Conditions.MarkTrue(condition.ReadyCondition, condition.ReadyMessage)
		}

		err := helper.PatchInstance(ctx, instance)
		if err != nil {
			_err = err
			return
		}
	}()

	if instance.Status.Conditions == nil {
		instance.InitConditions()
		// Register overall status immediately to have an early feedback e.g. in the cli
		return ctrl.Result{}, nil
	}

	// Reset all ReadyConditons to 'Unknown'
	instance.InitConditions()

	return r.reconcileNormal(ctx, instance, helper)
}

func (r *OpenStackControlPlaneReconciler) reconcileNormal(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {

	ctrlResult, err := openstack.ReconcileDNSMasqs(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileRabbitMQs(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileMariaDBs(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileGaleras(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileMemcacheds(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileKeystoneAPI(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcilePlacementAPI(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileGlance(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileCinder(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileOVN(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileNeutron(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileNova(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileHeat(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileIronic(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileOpenStackClient(ctx, instance, helper, r.OpenStackClientContainerImage)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileManila(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileHorizon(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileCeilometer(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.OpenStackControlPlane{}).
		Owns(&mariadbv1.MariaDB{}).
		Owns(&mariadbv1.Galera{}).
		Owns(&memcachedv1.Memcached{}).
		Owns(&keystonev1.KeystoneAPI{}).
		Owns(&placementv1.PlacementAPI{}).
		Owns(&glancev1.Glance{}).
		Owns(&cinderv1.Cinder{}).
		Owns(&manilav1.Manila{}).
		Owns(&rabbitmqv1.RabbitmqCluster{}).
		Owns(&ovnv1.OVNDBCluster{}).
		Owns(&ovnv1.OVNNorthd{}).
		Owns(&ovnv1.OVNController{}).
		Owns(&neutronv1.NeutronAPI{}).
		Owns(&novav1.Nova{}).
		Owns(&heatv1.Heat{}).
		Owns(&ironicv1.Ironic{}).
		Owns(&clientv1beta1.OpenStackClient{}).
		Owns(&horizonv1.Horizon{}).
		Owns(&telemetryv1.CeilometerCentral{}).
		Complete(r)
}
