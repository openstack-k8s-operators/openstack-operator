package openstack

import (
	"context"
	"errors"
	"fmt"
	"strings"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/object"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

type galeraStatus int

const (
	galeraFailed   galeraStatus = iota
	galeraCreating galeraStatus = iota
	galeraReady    galeraStatus = iota
)

func deleteUndefinedGaleras(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
) (ctrl.Result, error) {

	log := GetLogger(ctx)
	// Fetch the list of Galera objects
	galeraList := &mariadbv1.GaleraList{}
	listOpts := []client.ListOption{
		client.InNamespace(instance.GetNamespace()),
	}
	err := helper.GetClient().List(ctx, galeraList, listOpts...)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not get galeras %w", err)
	}

	var delErrs []error
	for _, galeraObj := range galeraList.Items {
		// if it is not defined in the OpenStackControlPlane then delete it from k8s.
		if _, exists := (*instance.Spec.Galera.Templates)[galeraObj.Name]; !exists {
			if object.CheckOwnerRefExist(instance.GetUID(), galeraObj.OwnerReferences) {
				log.Info("Deleting Galera", "", galeraObj.Name)

				certName := fmt.Sprintf("galera-%s-svc", galeraObj.Name)
				err = DeleteCertificate(ctx, helper, instance.Namespace, certName)
				if err != nil {
					delErrs = append(delErrs, fmt.Errorf("galera cert deletion for '%s' failed, because: %w", certName, err))
					continue
				}

				if _, err := EnsureDeleted(ctx, helper, &galeraObj); err != nil {
					delErrs = append(delErrs, fmt.Errorf("galera deletion for '%s' failed, because: %w", galeraObj.Name, err))
				}

			}
		}
	}

	if len(delErrs) > 0 {
		delErrs := errors.Join(delErrs...)
		return ctrl.Result{}, delErrs
	}

	return ctrl.Result{}, nil
}

// ReconcileGaleras -
func ReconcileGaleras(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	version *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
) (ctrl.Result, error) {
	log := GetLogger(ctx)
	if !instance.Spec.Galera.Enabled {
		return ctrl.Result{}, nil
	}

	var failures = []string{}
	var inprogress = []string{}
	clusterDomain := clusterdns.GetDNSClusterDomain()

	if instance.Spec.Galera.Templates == nil {
		instance.Spec.Galera.Templates = ptr.To(map[string]mariadbv1.GaleraSpecCore{})
	}

	for name, spec := range *instance.Spec.Galera.Templates {
		hostname := fmt.Sprintf("%s.%s.svc", name, instance.Namespace)
		hostnameHeadless := fmt.Sprintf("%s-galera.%s.svc", name, instance.Namespace)

		// Galera gets always configured to support TLS connections.
		// If TLS can/must be used is a per user configuration.
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetInternalIssuer(),
			CertName:   fmt.Sprintf("galera-%s-svc", name),
			Hostnames: []string{
				hostname,
				fmt.Sprintf("%s.%s", hostname, clusterDomain),
				hostnameHeadless,
				fmt.Sprintf("%s.%s", hostnameHeadless, clusterDomain),
				fmt.Sprintf("*.%s", hostnameHeadless),
				fmt.Sprintf("*.%s.%s", hostnameHeadless, clusterDomain),
			},
			// Note (dciabrin) from https://github.com/openstack-k8s-operators/openstack-operator/pull/678#issuecomment-1952459166
			// the certificate created for galera should populate the 'organization' field,
			// otherwise this trip the SST transfer setup done by wsrep_sst_rsync. This will not show
			// at the initial deployment because there is no SST involved when the DB is bootstrapped
			// as there are no data to be transferred yet.
			Subject: &certmgrv1.X509Subject{
				Organizations: []string{fmt.Sprintf("%s.%s", instance.Namespace, clusterDomain)},
			},
			Usages: []certmgrv1.KeyUsage{
				"key encipherment",
				"digital signature",
				"server auth",
				"client auth",
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
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		spec.TLS.Ca.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
		spec.TLS.SecretName = ptr.To(certSecret.Name)

		status, err := reconcileGalera(ctx, instance, version, helper, name, &spec)

		switch status {
		case galeraFailed:
			failures = append(failures, fmt.Sprintf("%s(%v)", name, err.Error()))
		case galeraCreating:
			inprogress = append(inprogress, name)
		case galeraReady:
		default:
			return ctrl.Result{}, fmt.Errorf("invalid galeraStatus from reconcileGalera: %d for Galera %s", status, name)
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
		log.Info("Galera in progress")
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneMariaDBReadyRunningMessage))
	} else {
		log.Info("Galera ready condition is true")
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			corev1beta1.OpenStackControlPlaneMariaDBReadyMessage,
		)
	}

	_, errs := deleteUndefinedGaleras(ctx, instance, helper)
	if errs != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneMariaDBReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneMariaDBReadyErrorMessage,
			errs))
		return ctrl.Result{}, errs
	}

	return ctrl.Result{}, nil
}

// reconcileGalera -
func reconcileGalera(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	version *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
	name string,
	spec *mariadbv1.GaleraSpecCore,
) (galeraStatus, error) {
	galera := &mariadbv1.Galera{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	log := GetLogger(ctx)

	if !instance.Spec.Galera.Enabled {
		if _, err := EnsureDeleted(ctx, helper, galera); err != nil {
			return galeraFailed, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneMariaDBReadyCondition)
		instance.Status.ContainerImages.MariadbImage = nil
		return galeraReady, nil
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

	log.Info("Reconciling Galera", "Galera.Namespace", instance.Namespace, "Galera.Name", name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), galera, func() error {
		spec.DeepCopyInto(&galera.Spec.GaleraSpecCore)
		galera.Spec.ContainerImage = *version.Status.ContainerImages.MariadbImage
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
		log.Info(fmt.Sprintf("Galera %s - %s", galera.Name, op))
	}

	if galera.Status.ObservedGeneration == galera.Generation && galera.IsReady() {
		instance.Status.ContainerImages.MariadbImage = version.Status.ContainerImages.MariadbImage
		return galeraReady, nil
	}

	return galeraCreating, nil
}

// GaleraImageMatch - return true if the Galera images match on the ControlPlane and Version, or if Galera is not enabled
func GaleraImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	log := GetLogger(ctx)
	if controlPlane.Spec.Galera.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.MariadbImage, version.Status.ContainerImages.MariadbImage) {
			log.Info("Galera images do not match", "controlPlane.Status.ContainerImages.MariadbImage", controlPlane.Status.ContainerImages.MariadbImage, "version.Status.ContainerImages.MariadbImage", version.Status.ContainerImages.MariadbImage)
			return false
		}
	}

	return true
}
