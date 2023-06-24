package openstack

import (
	"context"
	"fmt"
	"strings"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type mariadbStatus int

const (
	mariadbFailed   mariadbStatus = iota
	mariadbCreating mariadbStatus = iota
	mariadbReady    mariadbStatus = iota
)

// ReconcileMariaDBs -
func ReconcileMariaDBs(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
) (ctrl.Result, error) {
	if !instance.Spec.Mariadb.Enabled {
		return ctrl.Result{}, nil
	}

	var failures []string = []string{}
	var inprogress []string = []string{}

	for name, spec := range instance.Spec.Mariadb.Templates {
		status, err := reconcileMariaDB(ctx, instance, helper, name, &spec)

		switch status {
		case mariadbFailed:
			failures = append(failures, fmt.Sprintf("%s(%v)", name, err.Error()))
		case mariadbCreating:
			inprogress = append(inprogress, name)
		case mariadbReady:
		default:
			return ctrl.Result{}, fmt.Errorf("Invalid mariadbStatus from reconcileMariaDB: %d for MariaDB %s", status, name)
		}
	}

	if len(failures) > 0 {
		errors := strings.Join(failures, ",")

		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneMariaDBReadyErrorMessage,
			errors))

		return ctrl.Result{}, fmt.Errorf(errors)

	} else if len(inprogress) > 0 {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneMariaDBReadyRunningMessage))
	} else {
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			corev1beta1.OpenStackControlPlaneMariaDBReadyMessage,
		)
	}

	return ctrl.Result{}, nil
}

// reconcileMariaDB -
func reconcileMariaDB(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
	name string,
	spec *mariadbv1.MariaDBSpec,
) (mariadbStatus, error) {
	mariadb := &mariadbv1.MariaDB{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	Log := GetLogger(ctx)

	if !instance.Spec.Mariadb.Enabled {
		if _, err := EnsureDeleted(ctx, helper, mariadb); err != nil {
			return mariadbFailed, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneMariaDBReadyCondition)
		return mariadbReady, nil
	}

	Log.Info("Reconciling MariaDB", "MariaDB.Namespace", instance.Namespace, "Mariadb.Name", name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), mariadb, func() error {
		spec.DeepCopyInto(&mariadb.Spec)
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), mariadb, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return mariadbFailed, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("MariaDB %s - %s", mariadb.Name, op))
	}

	if mariadb.IsReady() {
		return mariadbReady, nil
	}

	return mariadbCreating, nil
}
