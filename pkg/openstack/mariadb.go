package openstack

import (
	"context"
	"fmt"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileMariaDB -
func ReconcileMariaDB(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {

	mariadb := &mariadbv1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstack", //FIXME
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling MariaDB", "MariaDB.Namespace", instance.Namespace, "mariadb.Name", "openstack")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), mariadb, func() error {
		instance.Spec.MariadbTemplate.DeepCopyInto(&mariadb.Spec)
		if mariadb.Spec.Secret == "" {
			mariadb.Spec.Secret = instance.Spec.Secret
		}
		if mariadb.Spec.StorageClass == "" {
			mariadb.Spec.StorageClass = instance.Spec.StorageClass
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), mariadb, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		if k8s_errors.IsNotFound(err) {
			helper.GetLogger().Info("MariaDB %s not found, reconcile in 10s")
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("MariaDB %s - %s", mariadb.Name, op))
	}

	return ctrl.Result{}, nil

}
