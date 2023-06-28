package openstack

import (
	"context"
	"fmt"
	"strings"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type memcachedStatus int

const (
	memcachedFailed   memcachedStatus = iota
	memcachedCreating memcachedStatus = iota
	memcachedReady    memcachedStatus = iota
)

// ReconcileMemcacheds -
func ReconcileMemcacheds(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
) (ctrl.Result, error) {
	var failures []string = []string{}
	var inprogress []string = []string{}

	// We first remove memcacheds no longer owned
	memcacheds := &memcachedv1.MemcachedList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := helper.GetClient().List(ctx, memcacheds, listOpts...); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneMemcachedReadyErrorMessage,
			err))
		return ctrl.Result{}, err
	}

	for _, memcached := range memcacheds.Items {
		for _, ref := range memcached.GetOwnerReferences() {
			// Check owner UID to ensure the memcached instance is owned by this OpenStackControlPlane instance
			if ref.UID == instance.GetUID() {
				owned := false

				// Check whether the name appears in spec
				for name := range instance.Spec.Memcached.Templates {
					if name == memcached.GetName() {
						owned = true
						break
					}
				}

				// The instance name is no longer part of the spec. Let's delete it
				if !owned {
					mc := &memcachedv1.Memcached{
						ObjectMeta: metav1.ObjectMeta{
							Name:      memcached.GetName(),
							Namespace: memcached.GetNamespace(),
						},
					}
					_, err := EnsureDeleted(ctx, helper, mc)
					if err != nil {
						failures = append(failures, fmt.Sprintf("%s(deleted)(%v)", memcached.GetName(), err.Error()))
					}
				}
			}
		}
	}

	// then reconcile ones listed in spec
	for name, spec := range instance.Spec.Memcached.Templates {
		status, err := reconcileMemcached(ctx, instance, helper, name, &spec)

		switch status {
		case memcachedFailed:
			failures = append(failures, fmt.Sprintf("%s(%v)", name, err.Error()))
		case memcachedCreating:
			inprogress = append(inprogress, name)
		case memcachedReady:
		default:
			failures = append(failures, fmt.Sprintf("Invalid memcachedStatus from reconcileMemcached: %d for Memcached %s", status, name))
		}
	}

	if len(failures) > 0 {
		errors := strings.Join(failures, ",")

		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneMemcachedReadyErrorMessage,
			errors))

		return ctrl.Result{}, fmt.Errorf(errors)

	} else if len(inprogress) > 0 {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMemcachedReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneMemcachedReadyRunningMessage))
	} else {
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackControlPlaneMemcachedReadyCondition,
			corev1beta1.OpenStackControlPlaneMemcachedReadyMessage,
		)
	}

	return ctrl.Result{}, nil
}

// reconcileMemcached -
func reconcileMemcached(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
	name string,
	spec *memcachedv1.MemcachedSpec,
) (memcachedStatus, error) {
	memcached := &memcachedv1.Memcached{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Memcached.Enabled {
		if _, err := EnsureDeleted(ctx, helper, memcached); err != nil {
			return memcachedFailed, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneMemcachedReadyCondition)
		return memcachedReady, nil
	}

	helper.GetLogger().Info("Reconciling Memcached", "Memcached.Namespace", instance.Namespace, "Memcached.Name", name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), memcached, func() error {
		spec.DeepCopyInto(&memcached.Spec)
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), memcached, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return memcachedFailed, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Memcached %s - %s", memcached.Name, op))
	}

	if memcached.IsReady() {
		return memcachedReady, nil
	}

	return memcachedCreating, nil
}
