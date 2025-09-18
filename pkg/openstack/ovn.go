package openstack

import (
	"context"
	"fmt"
	"reflect"

	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileOVN -
func ReconcileOVN(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
	Log := GetLogger(ctx)
	setOVNReadyError := func(instance *corev1beta1.OpenStackControlPlane, err error) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneOVNReadyErrorMessage,
			err.Error()))
	}

	if instance.Spec.Ovn.Template == nil {
		instance.Spec.Ovn.Template = &corev1beta1.OvnResources{}
	}

	// Create TLS certificate for OVN metrics services when TLS is enabled
	if instance.Spec.Ovn.Enabled && instance.Spec.TLS.PodLevel.Enabled {
		if err := EnsureOVNMetricsCert(ctx, instance, helper); err != nil {
			Log.Error(err, "Failed to ensure OVN metrics certificate")
			setOVNReadyError(instance, err)
			return ctrl.Result{}, err
		}
	}

	OVNDBClustersReady, OVNDBClustersConditions, err := ReconcileOVNDbClusters(ctx, instance, version, helper)
	if err != nil {
		Log.Error(err, "Failed to reconcile OVNDBClusters")
		setOVNReadyError(instance, err)
	}

	OVNNorthdReady, OVNNorthdConditions, err := ReconcileOVNNorthd(ctx, instance, version, helper)
	if err != nil {
		Log.Error(err, "Failed to reconcile OVNNorthd")
		setOVNReadyError(instance, err)
	}

	OVNControllerReady, OVNControllerConditions, err := ReconcileOVNController(ctx, instance, version, helper)
	if err != nil {
		Log.Error(err, "Failed to reconcile OVNController")
		setOVNReadyError(instance, err)
	}

	Log.Info("Reconciling OVN", "OVNDBClustersReady", OVNDBClustersReady, "OVNNorthdReady", OVNNorthdReady, "OVNControllerReady", OVNControllerReady)

	// Expect all services (dbclusters, northd, ovn-controller) ready
	if OVNDBClustersReady && OVNNorthdReady && OVNControllerReady {
		Log.Info("OVN ready condition is true")
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneOVNReadyCondition, corev1beta1.OpenStackControlPlaneOVNReadyMessage)
	} else if !instance.Spec.Ovn.Enabled {
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
	} else {
		// We want to mirror the condition of the highest priority from the OVN resources into the instance
		// under the condition of type OpenStackControlPlaneOVNReadyCondition, but only if the sub-resources
		// currently have any conditions (which won't be true for the initial creation of the sub-resources, since
		// they have not gone through a reconcile loop yet to have any conditions).  If this condition ends up being
		// the highest priority condition in the OpenStackControlPlane, it will appear in the OpenStackControlPlane's
		// "Ready" condition at the end of the reconciliation loop, clearly surfacing the condition to the user in
		// the "oc get oscontrolplane -n <namespace>" output.
		// Note: OVN aggregates multiple resources, so we collect conditions from all sub-resources

		highestPriorityConditions := condition.Conditions{}

		// We first mirror to an OpenStackControlPlaneOVNReadyCondition as a placeholder for the highest
		// priority condition, which will later be replicated into the OpenStackControlPlaneOVNReadyCondition
		// via the call to MirrorSubResourceCondition if it is indeed the highest priority condition.  This
		// same pattern is used for all OVN sub-resources.  We take this approach to ensure that we can
		// specify exactly which type of OVN sub-resource the condition is associated with.
		if len(OVNDBClustersConditions) > 0 {
			highestPriorityOVNDBClustersCondition := OVNDBClustersConditions.Mirror(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
			highestPriorityOVNDBClustersCondition.Message = fmt.Sprintf("%s: %s", reflect.TypeOf(ovnv1.OVNDBCluster{}).Name(), highestPriorityOVNDBClustersCondition.Message)
			highestPriorityConditions = append(highestPriorityConditions, *highestPriorityOVNDBClustersCondition)
		}
		if len(OVNNorthdConditions) > 0 {
			highestPriorityOVNNorthdCondition := OVNNorthdConditions.Mirror(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
			highestPriorityOVNNorthdCondition.Message = fmt.Sprintf("%s: %s", reflect.TypeOf(ovnv1.OVNNorthd{}).Name(), highestPriorityOVNNorthdCondition.Message)
			highestPriorityConditions = append(highestPriorityConditions, *highestPriorityOVNNorthdCondition)
		}
		if len(OVNControllerConditions) > 0 {
			highestPriorityOVNControllerCondition := OVNControllerConditions.Mirror(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
			highestPriorityOVNControllerCondition.Message = fmt.Sprintf("%s: %s", reflect.TypeOf(ovnv1.OVNController{}).Name(), highestPriorityOVNControllerCondition.Message)
			highestPriorityConditions = append(highestPriorityConditions, *highestPriorityOVNControllerCondition)
		}

		if len(highestPriorityConditions) > 0 {
			MirrorSubResourceCondition(highestPriorityConditions, corev1beta1.OpenStackControlPlaneOVNReadyCondition, instance, "")
		} else {
			// Default to the associated "running" condition message for the sub-resources if they currently lack any conditions for mirroring
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneOVNReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				corev1beta1.OpenStackControlPlaneOVNReadyRunningMessage))
		}
	}
	return ctrl.Result{}, nil
}

// ReconcileOVNDbClusters reconciles the OVN database clusters for the OpenStack control plane
func ReconcileOVNDbClusters(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (bool, condition.Conditions, error) {
	Log := GetLogger(ctx)
	dnsSuffix := clusterdns.GetDNSClusterDomain()
	conditions := condition.Conditions{}

	OVNDBClustersReady := len(instance.Spec.Ovn.Template.OVNDBCluster) != 0
	for name, dbcluster := range instance.Spec.Ovn.Template.OVNDBCluster {
		OVNDBCluster := &ovnv1.OVNDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: instance.Namespace,
			},
		}

		if !instance.Spec.Ovn.Enabled {
			instance.Status.ContainerImages.OvnNbDbclusterImage = nil
			instance.Status.ContainerImages.OvnSbDbclusterImage = nil
			instance.Status.ContainerImages.OpenstackNetworkExporterImage = nil
			if _, err := EnsureDeleted(ctx, helper, OVNDBCluster); err != nil {
				return false, conditions, err
			}
			continue
		}

		// preserve any previously set TLS certs, set CA cert
		if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, OVNDBCluster); err != nil {
			if !k8s_errors.IsNotFound(err) {
				return false, conditions, err
			}
		}

		// Add the conditions to the list of conditions to consider later for mirroring
		// It doesn't matter if the conditions are already in the list, they will be
		// deduplicated later during a MirrorSubResourceCondition call.
		conditions = append(conditions, OVNDBCluster.Status.Conditions...)
		if instance.Spec.TLS.PodLevel.Enabled {
			dbcluster.TLS = OVNDBCluster.Spec.TLS
		}
		dbcluster.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

		if instance.Spec.TLS.PodLevel.Enabled {
			// create certificate for ovndbclusters
			certRequest := certmanager.CertificateRequest{
				IssuerName: instance.GetOvnIssuer(),
				CertName:   fmt.Sprintf("%s-ovndbs", name),
				// Cert needs to be valid for the individual pods in the statefulset so make this a wildcard cert
				Hostnames: []string{
					fmt.Sprintf("*.%s.svc", instance.Namespace),
					fmt.Sprintf("*.%s.svc.%s", instance.Namespace, dnsSuffix),
				},
				Ips: nil,
				Usages: []certmgrv1.KeyUsage{
					certmgrv1.UsageKeyEncipherment,
					certmgrv1.UsageDigitalSignature,
					certmgrv1.UsageServerAuth,
					certmgrv1.UsageClientAuth,
				},
				Labels: map[string]string{serviceCertSelector: ""},
			}
			if instance.Spec.TLS.PodLevel.Ovn.Cert.Duration != nil {
				certRequest.Duration = &instance.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration
			}
			if instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore != nil {
				certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore.Duration
			}
			certSecret, ctrlResult, err := certmanager.EnsureCert(
				ctx,
				helper,
				certRequest,
				nil)
			if err != nil {
				return false, conditions, err
			} else if (ctrlResult != ctrl.Result{}) {
				return false, conditions, nil
			}

			dbcluster.TLS.SecretName = &certSecret.Name
		}

		if dbcluster.NodeSelector == nil {
			dbcluster.NodeSelector = &instance.Spec.NodeSelector
		}

		// When there's no Topology referenced in the Service Template, inject the
		// top-level one
		// NOTE: This does not check the Service subCRs: by default the generated
		// subCRs inherit the top-level TopologyRef unless an override is present
		if dbcluster.TopologyRef == nil {
			dbcluster.TopologyRef = instance.Spec.TopologyRef
		}

		Log.Info("Reconciling OVNDBCluster", "OVNDBCluster.Namespace", instance.Namespace, "OVNDBCluster.Name", name)
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNDBCluster, func() error {

			dbcluster.DeepCopyInto(&OVNDBCluster.Spec.OVNDBClusterSpecCore)

			// we always set these to match OpenStackVersion
			switch dbcluster.DBType {
			case ovnv1.NBDBType:
				OVNDBCluster.Spec.ContainerImage = *version.Status.ContainerImages.OvnNbDbclusterImage
			case ovnv1.SBDBType:
				OVNDBCluster.Spec.ContainerImage = *version.Status.ContainerImages.OvnSbDbclusterImage
			}

			OVNDBCluster.Spec.ExporterImage = *getImg(version.Status.ContainerImages.OpenstackNetworkExporterImage, &missingImageDefault)

			if OVNDBCluster.Spec.StorageClass == "" {
				OVNDBCluster.Spec.StorageClass = instance.Spec.StorageClass
			}

			err := controllerutil.SetControllerReference(helper.GetBeforeObject(), OVNDBCluster, helper.GetScheme())
			if err != nil {
				return err
			}
			return nil
		})

		if err != nil {
			Log.Error(err, "Failed to reconcile OVNDBCluster")
			return false, conditions, err
		}
		if op != controllerutil.OperationResultNone {
			Log.Info(fmt.Sprintf("OVNDBCluster %s - %s", OVNDBCluster.Name, op))
		}

		OVNDBClustersReady = OVNDBClustersReady && (OVNDBCluster.Status.ObservedGeneration == OVNDBCluster.Generation) && OVNDBCluster.IsReady()

	}
	if OVNDBClustersReady {
		instance.Status.ContainerImages.OvnNbDbclusterImage = version.Status.ContainerImages.OvnNbDbclusterImage
		instance.Status.ContainerImages.OvnSbDbclusterImage = version.Status.ContainerImages.OvnSbDbclusterImage
		instance.Status.ContainerImages.OpenstackNetworkExporterImage = version.Status.ContainerImages.OpenstackNetworkExporterImage
	}

	return OVNDBClustersReady, conditions, nil

}

// ReconcileOVNNorthd reconciles the OVN Northd service for the OpenStack control plane
func ReconcileOVNNorthd(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (bool, condition.Conditions, error) {
	Log := GetLogger(ctx)
	conditions := condition.Conditions{}

	OVNNorthd := &ovnv1.OVNNorthd{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovnnorthd",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ovn.Enabled {
		instance.Status.ContainerImages.OvnNorthdImage = nil
		if _, err := EnsureDeleted(ctx, helper, OVNNorthd); err != nil {
			return false, conditions, err
		}
		return false, conditions, nil
	}

	ovnNorthdSpec := &instance.Spec.Ovn.Template.OVNNorthd

	// preserve any previously set TLS certs, set CA cert
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "ovnnorthd", Namespace: instance.Namespace}, OVNNorthd); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return false, conditions, err
		}
	}
	// Add the conditions to the list of conditions to consider later for mirroring
	// It doesn't matter if the conditions are already in the list, they will be
	// deduplicated later during a MirrorSubResourceCondition call.
	conditions = append(conditions, OVNNorthd.Status.Conditions...)
	if instance.Spec.TLS.PodLevel.Enabled {
		ovnNorthdSpec.TLS = OVNNorthd.Spec.TLS
		dnsSuffix := clusterdns.GetDNSClusterDomain()

		serviceName := ovnv1.ServiceNameOvnNorthd
		// create certificate for ovnnorthd
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetOvnIssuer(),
			CertName:   fmt.Sprintf("%s-ovndbs", "ovnnorthd"),
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", serviceName, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", serviceName, instance.Namespace, dnsSuffix),
			},
			Ips: nil,
			Usages: []certmgrv1.KeyUsage{
				certmgrv1.UsageKeyEncipherment,
				certmgrv1.UsageDigitalSignature,
				certmgrv1.UsageServerAuth,
				certmgrv1.UsageClientAuth,
			},
			Labels: map[string]string{serviceCertSelector: ""},
		}
		if instance.Spec.TLS.PodLevel.Ovn.Cert.Duration != nil {
			certRequest.Duration = &instance.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration
		}
		if instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore != nil {
			certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore.Duration
		}
		certSecret, ctrlResult, err := certmanager.EnsureCert(
			ctx,
			helper,
			certRequest,
			nil)
		if err != nil {
			return false, conditions, err
		} else if (ctrlResult != ctrl.Result{}) {
			return false, conditions, nil
		}

		ovnNorthdSpec.TLS.SecretName = &certSecret.Name
	}
	ovnNorthdSpec.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	if ovnNorthdSpec.NodeSelector == nil {
		ovnNorthdSpec.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if ovnNorthdSpec.TopologyRef == nil {
		ovnNorthdSpec.TopologyRef = instance.Spec.TopologyRef
	}

	Log.Info("Reconciling OVNNorthd", "OVNNorthd.Namespace", instance.Namespace, "OVNNorthd.Name", "ovnnorthd")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNNorthd, func() error {

		instance.Spec.Ovn.Template.OVNNorthd.DeepCopyInto(&OVNNorthd.Spec.OVNNorthdSpecCore)

		OVNNorthd.Spec.ContainerImage = *version.Status.ContainerImages.OvnNorthdImage

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), OVNNorthd, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneOVNReadyErrorMessage,
			err.Error()))
		Log.Error(err, "Failed to reconcile OVNNorthd")
		return false, conditions, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OVNNorthd %s - %s", OVNNorthd.Name, op))
	}

	if OVNNorthd.Status.ObservedGeneration == OVNNorthd.Generation && OVNNorthd.IsReady() { //revive:disable:indent-error-flow
		instance.Status.ContainerImages.OvnNorthdImage = version.Status.ContainerImages.OvnNorthdImage
		return true, conditions, nil
	} else {
		return false, conditions, nil
	}

}

// ReconcileOVNController reconciles the OVN Controller service for the OpenStack control plane
func ReconcileOVNController(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (bool, condition.Conditions, error) {
	Log := GetLogger(ctx)
	conditions := condition.Conditions{}

	OVNController := &ovnv1.OVNController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovncontroller",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ovn.Enabled {
		instance.Status.ContainerImages.OvnControllerImage = nil
		instance.Status.ContainerImages.OvnControllerOvsImage = nil
		if _, err := EnsureDeleted(ctx, helper, OVNController); err != nil {
			return false, conditions, err
		}
		return false, conditions, nil
	} else if len(instance.Spec.Ovn.Template.OVNController.NicMappings) == 0 {
		// If nicMappings is not configured there's no need to start ovn-controller
		Log.Info("OVN Controller has no nicMappings configured, forcing ready condition to true.")
		if _, err := EnsureDeleted(ctx, helper, OVNController); err != nil {
			return false, conditions, err
		}

		// for minor updates, update the ovn controller images to the one from the version
		instance.Status.ContainerImages.OvnControllerImage = version.Status.ContainerImages.OvnControllerImage
		instance.Status.ContainerImages.OvnControllerOvsImage = version.Status.ContainerImages.OvnControllerOvsImage

		return true, conditions, nil
	}

	ovnControllerSpec := &instance.Spec.Ovn.Template.OVNController

	// preserve any previously set TLS certs, set CA cert
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "ovncontroller", Namespace: instance.Namespace}, OVNController); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return false, conditions, err
		}
	}
	// Add the conditions to the list of conditions to consider later for mirroring
	// It doesn't matter if the conditions are already in the list, they will be
	// deduplicated later during a MirrorSubResourceCondition call.
	conditions = append(conditions, OVNController.Status.Conditions...)
	if instance.Spec.TLS.PodLevel.Enabled {
		dnsSuffix := clusterdns.GetDNSClusterDomain()
		ovnControllerSpec.TLS = OVNController.Spec.TLS

		serviceName := ovnv1.ServiceNameOvnController
		// create certificate for ovncontroller
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetOvnIssuer(),
			CertName:   fmt.Sprintf("%s-ovndbs", "ovncontroller"),
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", serviceName, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", serviceName, instance.Namespace, dnsSuffix),
			},
			Ips: nil,
			Usages: []certmgrv1.KeyUsage{
				certmgrv1.UsageKeyEncipherment,
				certmgrv1.UsageDigitalSignature,
				certmgrv1.UsageServerAuth,
				certmgrv1.UsageClientAuth,
			},
			Labels: map[string]string{serviceCertSelector: ""},
		}
		if instance.Spec.TLS.PodLevel.Ovn.Cert.Duration != nil {
			certRequest.Duration = &instance.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration
		}
		if instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore != nil {
			certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore.Duration
		}
		certSecret, ctrlResult, err := certmanager.EnsureCert(
			ctx,
			helper,
			certRequest,
			nil)
		if err != nil {
			return false, conditions, err
		} else if (ctrlResult != ctrl.Result{}) {
			return false, conditions, nil
		}

		ovnControllerSpec.TLS.SecretName = &certSecret.Name
	}
	ovnControllerSpec.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	if ovnControllerSpec.NodeSelector == nil {
		ovnControllerSpec.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	// NOTE: This does not check the Service subCRs: by default the generated
	// subCRs inherit the top-level TopologyRef unless an override is present
	if ovnControllerSpec.TopologyRef == nil {
		ovnControllerSpec.TopologyRef = instance.Spec.TopologyRef
	}

	Log.Info("Reconciling OVNController", "OVNController.Namespace", instance.Namespace, "OVNController.Name", "ovncontroller")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNController, func() error {

		instance.Spec.Ovn.Template.OVNController.DeepCopyInto(&OVNController.Spec.OVNControllerSpecCore)

		OVNController.Spec.OvnContainerImage = *version.Status.ContainerImages.OvnControllerImage
		OVNController.Spec.OvsContainerImage = *version.Status.ContainerImages.OvnControllerOvsImage
		OVNController.Spec.ExporterImage = *getImg(version.Status.ContainerImages.OpenstackNetworkExporterImage, &missingImageDefault)

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), OVNController, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneOVNReadyErrorMessage,
			err.Error()))
		Log.Error(err, "Failed to reconcile OVNController")
		return false, conditions, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OVNController %s - %s", OVNController.Name, op))
	}

	if OVNController.Status.ObservedGeneration == OVNController.Generation && OVNController.IsReady() {
		Log.Info("OVN Controller ready condition is true")
		instance.Status.ContainerImages.OvnControllerImage = version.Status.ContainerImages.OvnControllerImage
		instance.Status.ContainerImages.OvnControllerOvsImage = version.Status.ContainerImages.OvnControllerOvsImage
		instance.Status.ContainerImages.OpenstackNetworkExporterImage = version.Status.ContainerImages.OpenstackNetworkExporterImage
		return true, conditions, nil
	} else {
		return false, conditions, nil
	}
}

// OVNControllerImageMatch - return true if the OVN Controller images match on the ControlPlane and Version, or if OVN is not enabled
func OVNControllerImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.Ovn.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.OvnControllerImage, version.Status.ContainerImages.OvnControllerImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.OvnControllerOvsImage, version.Status.ContainerImages.OvnControllerOvsImage) {
			Log.Info("OVN Controller images do not match")
			return false
		}
	}
	return true
}

// OVNDbClusterImageMatch - return true if the OVN DbCluster images match on the ControlPlane and Version, or if OVN is not enabled
func OVNDbClusterImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.Ovn.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.OvnNbDbclusterImage, version.Status.ContainerImages.OvnNbDbclusterImage) ||
			!stringPointersEqual(controlPlane.Status.ContainerImages.OvnSbDbclusterImage, version.Status.ContainerImages.OvnSbDbclusterImage) {
			Log.Info("OVN Cluster images do not match")
			return false
		}
	}
	return true
}

// OVNNorthImageMatch - return true if the OVN North images match on the ControlPlane and Version, or if OVN is not enabled
func OVNNorthImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)

	if controlPlane.Spec.Ovn.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.OvnNorthdImage, version.Status.ContainerImages.OvnNorthdImage) {
			Log.Info("OVN North images do not match", "controlPlane.Status.ContainerImages.OvnNorthdImage", controlPlane.Status.ContainerImages.OvnNorthdImage, "version.Status.ContainerImages.OvnNorthdImage", version.Status.ContainerImages.OvnNorthdImage)
			return false
		}
	}
	return true
}

// EnsureOVNMetricsCert creates TLS certificate for OVN metrics services
func EnsureOVNMetricsCert(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) error {
	Log := GetLogger(ctx)

	dnsSuffix := clusterdns.GetDNSClusterDomain()

	certRequest := certmanager.CertificateRequest{
		IssuerName: instance.GetOvnIssuer(),
		CertName:   "ovn-metrics",
		Hostnames: []string{
			// Cert needs to be valid for the individual pods services so make this a wildcard cert
			fmt.Sprintf("*.%s.svc", instance.Namespace),
			fmt.Sprintf("*.%s.svc.%s", instance.Namespace, dnsSuffix),
		},
		Ips: nil,
		Usages: []certmgrv1.KeyUsage{
			certmgrv1.UsageKeyEncipherment,
			certmgrv1.UsageDigitalSignature,
			certmgrv1.UsageServerAuth,
			certmgrv1.UsageClientAuth,
		},
		Labels: map[string]string{serviceCertSelector: ""},
	}

	// Apply certificate duration settings if configured
	if instance.Spec.TLS.PodLevel.Ovn.Cert.Duration != nil {
		certRequest.Duration = &instance.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration
	}
	if instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore != nil {
		certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Ovn.Cert.RenewBefore.Duration
	}

	// Create or update the certificate
	certSecret, ctrlResult, err := certmanager.EnsureCert(
		ctx,
		helper,
		certRequest,
		nil)
	if err != nil {
		return err
	} else if (ctrlResult != ctrl.Result{}) {
		Log.Info("OVN metrics certificate creation in progress", "certificate", certRequest.CertName)
		return fmt.Errorf("OVN metrics certificate creation in progress")
	}

	Log.Info("OVN metrics certificate ensured", "secret", certSecret.Name, "certificate", certRequest.CertName)
	return nil
}
