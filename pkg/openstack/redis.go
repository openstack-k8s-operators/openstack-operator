package openstack

import (
	"context"
	"fmt"
	"strings"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
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
	version *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
) (ctrl.Result, error) {
	var failures = []string{}
	var inprogress = []string{}

	// NOTE(beagles): This performs quite a bit of processing to purge
	// even if the controller is meant to be disabled and might set an
	// error condition somewhere along the way. I suspect this is
	// actually desirable because it indicates that the cluster is
	// somehow in a state that the operator doesn't know how to
	// reconcile. The implementation will need to careful about the
	// disabled from the start scenario when it likely should *not*
	// start appearing in conditions. It also means that the situation
	// where sample template data in a config where redis is disabled
	// will require that the controller iterator over the
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

	if instance.Spec.Redis.Templates == nil {
		instance.Spec.Redis.Templates = ptr.To(map[string]redisv1.RedisSpecCore{})
	}

	for _, redis := range redises.Items {
		for _, ref := range redis.GetOwnerReferences() {
			// Check owner UID to ensure the redis instance is owned by this OpenStackControlPlane instance
			if ref.UID == instance.GetUID() {
				owned := false

				// Check whether the name appears in spec
				for name := range *instance.Spec.Redis.Templates {
					if name == redis.GetName() {
						owned = true
						break
					}
				}

				// The instance name is no longer part of the spec. Let's delete it
				if !owned {
					mc := &redisv1.Redis{
						ObjectMeta: metav1.ObjectMeta{
							Name:      redis.GetName(),
							Namespace: redis.GetNamespace(),
						},
					}
					_, err := EnsureDeleted(ctx, helper, mc)
					if err != nil {
						failures = append(failures, fmt.Sprintf("%s(deleted)(%v)", redis.GetName(), err.Error()))
					}
				}
			}
		}
	}

	// then reconcile ones listed in spec
	var ctrlResult ctrl.Result
	var err error
	var status redisStatus

	for name, spec := range *instance.Spec.Redis.Templates {
		status, ctrlResult, err = reconcileRedis(ctx, instance, version, helper, name, &spec)

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

		return ctrlResult, fmt.Errorf("%s", errors)

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

	return ctrlResult, nil
}

// reconcileRedis -
func reconcileRedis(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	version *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
	name string,
	spec *redisv1.RedisSpecCore,
) (redisStatus, ctrl.Result, error) {
	redis := &redisv1.Redis{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Redis.Enabled {
		if _, err := EnsureDeleted(ctx, helper, redis); err != nil {
			return redisFailed, ctrl.Result{}, err
		}
		return redisReady, ctrl.Result{}, nil
	}

	helper.GetLogger().Info("Reconciling Redis", "Redis.Namespace", instance.Namespace, "Redis.Name", name)

	tlsCert := ""
	if instance.Spec.TLS.PodLevel.Enabled {
		clusterDomain := clusterdns.GetDNSClusterDomain()
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetInternalIssuer(),
			CertName:   fmt.Sprintf("%s-svc", redis.Name),
			Hostnames: []string{
				fmt.Sprintf("redis-%s.%s.svc", name, instance.Namespace),
				fmt.Sprintf("*.redis-%s.%s.svc", name, instance.Namespace),
				fmt.Sprintf("redis-%s.%s.svc.%s", name, instance.Namespace, clusterDomain),
				fmt.Sprintf("*.redis-%s.%s.svc.%s", name, instance.Namespace, clusterDomain),
			},
			Subject: &certmgrv1.X509Subject{
				Organizations: []string{fmt.Sprintf("%s.%s", instance.Namespace, clusterDomain)},
			},
			Usages: []certmgrv1.KeyUsage{
				"key encipherment",
				"digital signature",
				"server auth",
				"client auth",
			},
		}
		if instance.Spec.TLS.PodLevel.Internal.Cert.Duration != nil {
			certRequest.Duration = &instance.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration
		}
		if instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore != nil {
			certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore.Duration
		}
		certSecret, ctrlResult, err := certmanager.EnsureCert(
			ctx,
			helper,
			certRequest,
			nil)
		if err != nil {
			return redisFailed, ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return redisCreating, ctrlResult, nil
		}

		tlsCert = certSecret.Name
	}

	if spec.NodeSelector == nil {
		spec.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if spec.TopologyRef == nil {
		spec.TopologyRef = instance.Spec.TopologyRef
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), redis, func() error {
		spec.DeepCopyInto(&redis.Spec.RedisSpecCore)
		if tlsCert != "" {

			redis.Spec.TLS.SecretName = ptr.To(tlsCert)
		}
		redis.Spec.TLS.CaBundleSecretName = tls.CABundleSecret

		redis.Spec.ContainerImage = *version.Status.ContainerImages.InfraRedisImage
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), redis, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return redisFailed, ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Redis %s - %s", redis.Name, op))
	}

	if redis.IsReady() { //FIXME ObserverdGeneration
		instance.Status.ContainerImages.InfraRedisImage = version.Status.ContainerImages.InfraRedisImage
		return redisReady, ctrl.Result{}, nil
	}

	return redisCreating, ctrl.Result{}, nil
}

// RedisImageMatch - return true if the redis images match on the ControlPlane and Version, or if Redis is not enabled
func RedisImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Redis.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.InfraRedisImage, version.Status.ContainerImages.InfraRedisImage) {
			Log.Info("Redis images do not match", "controlPlane.Status.ContainerImages.InfraRedisImage", controlPlane.Status.ContainerImages.InfraRedisImage, "version.Status.ContainerImages.InfraRedisImage", version.Status.ContainerImages.InfraRedisImage)
			return false
		}
	}

	return true
}
