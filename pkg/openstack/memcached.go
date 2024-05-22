package openstack

import (
	"context"
	"fmt"
	"strings"

	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
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
	version *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
) (ctrl.Result, error) {
	var failures = []string{}
	var inprogress = []string{}

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
	var ctrlResult ctrl.Result
	var err error
	var status memcachedStatus
	for name, spec := range instance.Spec.Memcached.Templates {
		status, ctrlResult, err = reconcileMemcached(ctx, instance, version, helper, name, &spec)

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

		return ctrlResult, fmt.Errorf(errors)

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

	return ctrlResult, nil
}

// reconcileMemcached -
func reconcileMemcached(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	version *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
	name string,
	spec *memcachedv1.MemcachedSpecCore,
) (memcachedStatus, ctrl.Result, error) {
	memcached := &memcachedv1.Memcached{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	Log := GetLogger(ctx)

	if !instance.Spec.Memcached.Enabled {
		if _, err := EnsureDeleted(ctx, helper, memcached); err != nil {
			return memcachedFailed, ctrl.Result{}, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneMemcachedReadyCondition)
		return memcachedReady, ctrl.Result{}, nil
	}

	Log.Info("Reconciling Memcached", "Memcached.Namespace", instance.Namespace, "Memcached.Name", name)

	tlsCert := ""
	if instance.Spec.TLS.PodLevel.Enabled {
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetInternalIssuer(),
			CertName:   fmt.Sprintf("%s-svc", memcached.Name),
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", name, instance.Namespace),
				fmt.Sprintf("*.%s.%s.svc", name, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", name, instance.Namespace, ClusterInternalDomain),
				fmt.Sprintf("*.%s.%s.svc.%s", name, instance.Namespace, ClusterInternalDomain),
			},
			Labels: map[string]string{serviceCertSelector: ""},
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
			return memcachedFailed, ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return memcachedCreating, ctrlResult, nil
		}

		tlsCert = certSecret.Name
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), memcached, func() error {
		spec.DeepCopyInto(&memcached.Spec.MemcachedSpecCore)

		if tlsCert != "" {
			memcached.Spec.TLS.SecretName = ptr.To(tlsCert)
		}
		memcached.Spec.TLS.CaBundleSecretName = tls.CABundleSecret
		memcached.Spec.ContainerImage = *version.Status.ContainerImages.InfraMemcachedImage
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), memcached, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return memcachedFailed, ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("Memcached %s - %s", memcached.Name, op))
	}

	if memcached.Status.ObservedGeneration == memcached.Generation && memcached.IsReady() {
		instance.Status.ContainerImages.InfraMemcachedImage = version.Status.ContainerImages.InfraMemcachedImage
		return memcachedReady, ctrl.Result{}, nil
	}

	return memcachedCreating, ctrl.Result{}, nil
}
