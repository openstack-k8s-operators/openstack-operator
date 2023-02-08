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

type galeraStatus int

const (
	galeraFailed   galeraStatus = iota
	galeraCreating galeraStatus = iota
	galeraReady    galeraStatus = iota
)

// ReconcileGaleras -
func ReconcileGaleras(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
) (ctrl.Result, error) {
	if !instance.Spec.Galera.Enabled {
		return ctrl.Result{}, nil
	}

	var failures []string = []string{}
	var inprogress []string = []string{}

	for name, spec := range instance.Spec.Galera.Templates {
		status, err := reconcileGalera(ctx, instance, helper, name, &spec)

		switch status {
		case galeraFailed:
			failures = append(failures, fmt.Sprintf("%s(%v)", name, err.Error()))
		case galeraCreating:
			inprogress = append(inprogress, name)
		case galeraReady:
		default:
			return ctrl.Result{}, fmt.Errorf("Invalid galeraStatus from reconcileGalera: %d for Galera %s", status, name)
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

// reconcileGalera -
func reconcileGalera(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
	name string,
	spec *mariadbv1.GaleraSpec,
) (galeraStatus, error) {
	galera := &mariadbv1.Galera{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Galera.Enabled {
		if _, err := EnsureDeleted(ctx, helper, galera); err != nil {
			return galeraFailed, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneMariaDBReadyCondition)
		return galeraReady, nil
	}

	helper.GetLogger().Info("Reconciling Galera", "Galera.Namespace", instance.Namespace, "Galera.Name", name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), galera, func() error {
		spec.DeepCopyInto(&galera.Spec)
		if galera.Spec.Secret == "" {
			galera.Spec.Secret = instance.Spec.Secret
		}
		if galera.Spec.StorageClass == "" {
			galera.Spec.StorageClass = instance.Spec.StorageClass
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), galera, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return galeraFailed, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Galera %s - %s", galera.Name, op))
	}

	if galera.IsReady() {
		return galeraReady, nil
	}

	return galeraCreating, nil
}
