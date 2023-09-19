package openstack

import (
	"context"
	"fmt"
	"strings"

	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type redisStatus int

const (
	redisFailed   redisStatus = iota
	redisCreating redisStatus = iota
	redisReady    redisStatus = iota
)

// ReconcileRedis -
func ReconcileRedis(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
) (ctrl.Result, error) {
	var failures []string = []string{}
	var inprogress []string = []string{}

	// NOTE(beagles): This performs quite a bit of processing to purge
	// even if the controller is meant to be disabled and might set an
	// error condition somewhere along the way. I suspect this is
	// actually desirable because it indicates that the cluster is
	// somehow in a state that the operator doesn't know how to
	// reconcile. The implementation will need to careful about the
	// disabled from the start scenario when it likely should *not*
	// start appearing in conditions. It also means that the situation
	// where sample template data in a config where redis is disabled
	// will require require that the controller iterator over the
	// specs and make sure that existing deployed redis instances are
	// deleted. Generally means if we want the redis controller to
	// truly remain invisible it's best not to provide any example
	// templates info while the service is disabled.

	// We first remove redises no longer owned
	redises := &redisv1.RedisList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	if err := helper.GetClient().List(ctx, redises, listOpts...); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneRedisReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneRedisReadyErrorMessage,
			err))
		return ctrl.Result{}, err
	}

	// then reconcile ones listed in spec
	for name, spec := range instance.Spec.Redis.Templates {
		status, err := reconcileRedis(ctx, instance, helper, name, &spec)

		switch status {
		case redisFailed:
			failures = append(failures, fmt.Sprintf("%s(%v)", name, err.Error()))
		case redisCreating:
			inprogress = append(inprogress, name)
		case redisReady:
		default:
			failures = append(failures, fmt.Sprintf("Invalid redisStatus from reconcileRedis: %d for Redis %s", status, name))
		}
	}

	if len(failures) > 0 {
		errors := strings.Join(failures, ",")

		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneRedisReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneRedisReadyErrorMessage,
			errors))

		return ctrl.Result{}, fmt.Errorf(errors)

	} else if len(inprogress) > 0 {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneRedisReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneRedisReadyRunningMessage))
	} else if !instance.Spec.Redis.Enabled {
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneRedisReadyCondition)
	} else {
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackControlPlaneRedisReadyCondition,
			corev1beta1.OpenStackControlPlaneRedisReadyMessage,
		)
	}

	return ctrl.Result{}, nil
}

// reconcileRedis -
func reconcileRedis(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
	name string,
	spec *redisv1.RedisSpec,
) (redisStatus, error) {
	redis := &redisv1.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Redis.Enabled {
		if _, err := EnsureDeleted(ctx, helper, redis); err != nil {
			return redisFailed, err
		}
		return redisReady, nil
	}

	helper.GetLogger().Info("Reconciling Redis", "Redis.Namespace", instance.Namespace, "Redis.Name", name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), redis, func() error {
		spec.DeepCopyInto(&redis.Spec)
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), redis, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return redisFailed, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Redis %s - %s", redis.Name, op))
	}

	if redis.IsReady() {
		return redisReady, nil
	}

	return redisCreating, nil
}
