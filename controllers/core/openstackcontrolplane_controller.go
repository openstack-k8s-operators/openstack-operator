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

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/go-logr/logr"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

// GetClient -
func (r *OpenStackControlPlaneReconciler) GetClient() client.Client {
	return r.Client
}

// GetKClient -
func (r *OpenStackControlPlaneReconciler) GetKClient() kubernetes.Interface {
	return r.Kclient
}

// GetLogger -
func (r *OpenStackControlPlaneReconciler) GetLogger() logr.Logger {
	return r.Log
}

// GetScheme -
func (r *OpenStackControlPlaneReconciler) GetScheme() *runtime.Scheme {
	return r.Scheme
}

// OpenStackControlPlaneReconciler reconciles a OpenStackControlPlane object
type OpenStackControlPlaneReconciler struct {
	client.Client
	Scheme  *runtime.Scheme
	Kclient kubernetes.Interface
	Log     logr.Logger
}

//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=core.openstack.org,resources=openstackcontrolplanes/finalizers,verbs=update
//+kubebuilder:rbac:groups=keystone.openstack.org,resources=keystoneapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=placement.openstack.org,resources=placementapis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=glance.openstack.org,resources=glances,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=mariadb.openstack.org,resources=mariadbs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rabbitmq.com,resources=rabbitmqclusters,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *OpenStackControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// Fetch the OpenStackControlPlane instance
	instance := &corev1beta1.OpenStackControlPlane{}
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
	helper, err := helper.NewHelper(
		instance,
		r.Client,
		r.Kclient,
		r.Scheme,
		r.Log,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.reconcileNormal(ctx, instance, helper)

}

func (r *OpenStackControlPlaneReconciler) reconcileNormal(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {

	ctrlResult, err := openstack.ReconcileRabbitMQ(ctx, instance, helper)
	if err != nil {
		return ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	ctrlResult, err = openstack.ReconcileMariaDB(ctx, instance, helper)
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

	return ctrl.Result{}, nil

}

// SetupWithManager sets up the controller with the Manager.
func (r *OpenStackControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1beta1.OpenStackControlPlane{}).
		Owns(&mariadbv1.MariaDB{}).
		Owns(&keystonev1.KeystoneAPI{}).
		Owns(&placementv1.PlacementAPI{}).
		Owns(&glancev1.Glance{}).
		Owns(&rabbitmqv1.RabbitmqCluster{}).
		Complete(r)
}
