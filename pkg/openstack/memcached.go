package openstack

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
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
	var conditions = condition.Conditions{}

	// We first remove memcacheds no longer owned
	memcacheds := &memcachedv1.MemcachedList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.Namespace),
	}
	Log := GetLogger(ctx)
	if err := helper.GetClient().List(ctx, memcacheds, listOpts...); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMemcachedReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneMemcachedReadyErrorMessage,
			err))
		return ctrl.Result{}, err
	}

	if instance.Spec.Memcached.Templates == nil {
		instance.Spec.Memcached.Templates = ptr.To(map[string]memcachedv1.MemcachedSpecCore{})
	}

	for _, memcached := range memcacheds.Items {
		for _, ref := range memcached.GetOwnerReferences() {
			// Check owner UID to ensure the memcached instance is owned by this OpenStackControlPlane instance
			if ref.UID == instance.GetUID() {
				owned := false

				// Check whether the name appears in spec
				for name := range *instance.Spec.Memcached.Templates {
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
	for name, spec := range *instance.Spec.Memcached.Templates {
		var memcached *memcachedv1.Memcached

		status, memcached, ctrlResult, err = reconcileMemcached(ctx, instance, version, helper, name, &spec)

		// Add the conditions to the list of conditions to consider later for mirroring.
		// It doesn't matter if the conditions are already in the list, they will be
		// deduplicated later during the MirrorSubResourceCondition call.
		if memcached != nil && memcached.Status.Conditions != nil {
			conditions = append(conditions, memcached.Status.Conditions...)
		}

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

		return ctrlResult, fmt.Errorf("%s", errors)

	} else if len(inprogress) > 0 {
		// We want to mirror the condition of the highest priority from the Memcached resources into the instance
		// under the condition of type OpenStackControlPlaneMemcachedReadyCondition, but only if the sub-resources
		// currently have any conditions (which won't be true for the initial creation of the sub-resources, since
		// they have not gone through a reconcile loop yet to have any conditions).  If this condition ends up being
		// the highest priority condition in the OpenStackControlPlane, it will appear in the OpenStackControlPlane's
		// "Ready" condition at the end of the reconciliation loop, clearly surfacing the condition to the user in
		// the "oc get oscontrolplane -n <namespace>" output.
		if len(conditions) > 0 {
			MirrorSubResourceCondition(conditions, corev1beta1.OpenStackControlPlaneMemcachedReadyCondition, instance, reflect.TypeOf(memcachedv1.Memcached{}).Name())
		} else {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneMemcachedReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneMemcachedReadyRunningMessage))
		}
	} else {
		Log.Info("Memcached ready condition is true")
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
) (memcachedStatus, *memcachedv1.Memcached, ctrl.Result, error) {
	memcached := &memcachedv1.Memcached{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	Log := GetLogger(ctx)

	if !instance.Spec.Memcached.Enabled {
		if _, err := EnsureDeleted(ctx, helper, memcached); err != nil {
			return memcachedFailed, memcached, ctrl.Result{}, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneMemcachedReadyCondition)
		instance.Status.ContainerImages.InfraMemcachedImage = nil
		return memcachedReady, memcached, ctrl.Result{}, nil
	}

	Log.Info("Reconciling Memcached", "Memcached.Namespace", instance.Namespace, "Memcached.Name", name)

	tlsCert := ""
	mtlsCert := ""
	if instance.Spec.TLS.PodLevel.Enabled {
		Log.Info("Reconciling Memcached TLS", "Memcached.Namespace", instance.Namespace, "Memcached.Name", name)
		clusterDomain := clusterdns.GetDNSClusterDomain()
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetInternalIssuer(),
			CertName:   fmt.Sprintf("%s-svc", memcached.Name),
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", name, instance.Namespace),
				fmt.Sprintf("*.%s.%s.svc", name, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", name, instance.Namespace, clusterDomain),
				fmt.Sprintf("*.%s.%s.svc.%s", name, instance.Namespace, clusterDomain),
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
			return memcachedFailed, memcached, ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return memcachedCreating, memcached, ctrlResult, nil
		}

		tlsCert = certSecret.Name

		// mTLS cert
		if spec.TLS.MTLS.SslVerifyMode == "Request" || spec.TLS.MTLS.SslVerifyMode == "Require" {
			Log.Info("Reconciling Memcached mTLS", "Memcached.Namespace", instance.Namespace, "Memcached.Name", name)
			clusterDomain = clusterdns.GetDNSClusterDomain()
			certRequest = certmanager.CertificateRequest{
				IssuerName: instance.GetInternalIssuer(),
				CertName:   fmt.Sprintf("%s-mtls", memcached.Name),
				Hostnames: []string{
					fmt.Sprintf("*.%s.svc", instance.Namespace),
					fmt.Sprintf("*.%s.svc.%s", instance.Namespace, clusterDomain),
				},
				Labels: map[string]string{serviceCertSelector: ""},
				Usages: []certmgrv1.KeyUsage{
					certmgrv1.UsageKeyEncipherment,
					certmgrv1.UsageDigitalSignature,
					certmgrv1.UsageClientAuth,
				},
			}
			if instance.Spec.TLS.PodLevel.Internal.Cert.Duration != nil {
				certRequest.Duration = &instance.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration
			}
			if instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore != nil {
				certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore.Duration
			}
			certSecret, ctrlResult, err = certmanager.EnsureCert(
				ctx,
				helper,
				certRequest,
				nil)
			if err != nil {
				return memcachedFailed, memcached, ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return memcachedCreating, memcached, ctrlResult, nil
			}

			mtlsCert = certSecret.Name
		}
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

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), memcached, func() error {
		spec.DeepCopyInto(&memcached.Spec.MemcachedSpecCore)

		if tlsCert != "" {
			memcached.Spec.TLS.SecretName = ptr.To(tlsCert)
		}
		if mtlsCert != "" {
			memcached.Spec.TLS.MTLS.AuthCertSecret.SecretName = ptr.To(mtlsCert)
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
		return memcachedFailed, memcached, ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("Memcached %s - %s", memcached.Name, op))
	}

	if memcached.Status.ObservedGeneration == memcached.Generation && memcached.IsReady() {
		instance.Status.ContainerImages.InfraMemcachedImage = version.Status.ContainerImages.InfraMemcachedImage
		return memcachedReady, memcached, ctrl.Result{}, nil
	}

	return memcachedCreating, memcached, ctrl.Result{}, nil
}

// MemcachedImageMatch - return true if the memcached images match on the ControlPlane and Version, or if Memcached is not enabled
func MemcachedImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Memcached.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.InfraMemcachedImage, version.Status.ContainerImages.InfraMemcachedImage) {
			Log.Info("Memcached images do not match", "controlPlane.Status.ContainerImages.InfraMemcachedImage", controlPlane.Status.ContainerImages.InfraMemcachedImage, "version.Status.ContainerImages.InfraMemcachedImage", version.Status.ContainerImages.InfraMemcachedImage)
			return false
		}
	}

	return true
}
