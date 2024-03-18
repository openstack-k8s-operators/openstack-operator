package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
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
func ReconcileOVN(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {

	setOVNReadyError := func(instance *corev1beta1.OpenStackControlPlane, err error) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneOVNReadyErrorMessage,
			err.Error()))
	}

	OVNDBClustersReady, err := ReconcileOVNDbClusters(ctx, instance, helper)
	if err != nil {
		setOVNReadyError(instance, err)
	}

	OVNNorthdReady, err := ReconcileOVNNorthd(ctx, instance, helper)
	if err != nil {
		setOVNReadyError(instance, err)
	}

	OVNControllerReady, err := ReconcileOVNController(ctx, instance, helper)
	if err != nil {
		setOVNReadyError(instance, err)
	}

	// Expect all services (dbclusters, northd, ovn-controller) ready
	if OVNDBClustersReady && OVNNorthdReady && OVNControllerReady {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneOVNReadyCondition, corev1beta1.OpenStackControlPlaneOVNReadyMessage)
	} else if !instance.Spec.Ovn.Enabled {
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneOVNReadyRunningMessage))
	}
	return ctrl.Result{}, nil
}

func ReconcileOVNDbClusters(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (bool, error) {
	Log := GetLogger(ctx)

	OVNDBClustersReady := len(instance.Spec.Ovn.Template.OVNDBCluster) != 0
	for name, dbcluster := range instance.Spec.Ovn.Template.OVNDBCluster {
		OVNDBCluster := &ovnv1.OVNDBCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: instance.Namespace,
			},
		}

		if !instance.Spec.Ovn.Enabled {
			if _, err := EnsureDeleted(ctx, helper, OVNDBCluster); err != nil {
				return false, err
			}
			continue
		}

		// preserve any previously set TLS certs, set CA cert
		if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: name, Namespace: instance.Namespace}, OVNDBCluster); err != nil {
			if !k8s_errors.IsNotFound(err) {
				return false, err
			}
		}
		if instance.Spec.TLS.PodLevel.Enabled {
			dbcluster.TLS = OVNDBCluster.Spec.TLS
		}
		dbcluster.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

		if OVNDBCluster.Status.Conditions.IsTrue(condition.ExposeServiceReadyCondition) {
			// create certificate for ovndbclusters
			certRequest := certmanager.CertificateRequest{
				IssuerName: instance.GetOvnIssuer(),
				CertName:   fmt.Sprintf("%s-ovndbs", name),
				Duration:   nil,
				// Cert needs to be valid for the individual pods in the statefulset so make this a wildcard cert
				Hostnames: []string{
					fmt.Sprintf("*.%s.svc", instance.Namespace),
					fmt.Sprintf("*.%s.svc.%s", instance.Namespace, ovnv1.DNSSuffix),
				},
				Ips: nil,
				Usages: []certmgrv1.KeyUsage{
					certmgrv1.UsageKeyEncipherment,
					certmgrv1.UsageDigitalSignature,
					certmgrv1.UsageServerAuth,
					certmgrv1.UsageClientAuth,
				},
			}
			certSecret, ctrlResult, err := certmanager.EnsureCert(
				ctx,
				helper,
				certRequest,
				nil)
			if err != nil {
				return false, err
			} else if (ctrlResult != ctrl.Result{}) {
				return false, nil
			}

			if instance.Spec.TLS.PodLevel.Enabled {
				dbcluster.TLS.SecretName = &certSecret.Name
			}
		}

		Log.Info("Reconciling OVNDBCluster", "OVNDBCluster.Namespace", instance.Namespace, "OVNDBCluster.Name", name)
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNDBCluster, func() error {

			dbcluster.DeepCopyInto(&OVNDBCluster.Spec)

			if OVNDBCluster.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
				OVNDBCluster.Spec.NodeSelector = instance.Spec.NodeSelector
			}
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
			return false, err
		}
		if op != controllerutil.OperationResultNone {
			Log.Info(fmt.Sprintf("OVNDBCluster %s - %s", OVNDBCluster.Name, op))
		}
		OVNDBClustersReady = OVNDBClustersReady && OVNDBCluster.IsReady()
	}
	return OVNDBClustersReady, nil
}

func ReconcileOVNNorthd(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (bool, error) {
	Log := GetLogger(ctx)

	OVNNorthd := &ovnv1.OVNNorthd{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovnnorthd",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ovn.Enabled {
		if _, err := EnsureDeleted(ctx, helper, OVNNorthd); err != nil {
			return false, err
		}
		return false, nil
	}

	ovnNorthdSpec := &instance.Spec.Ovn.Template.OVNNorthd

	// preserve any previously set TLS certs, set CA cert
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "ovnnorthd", Namespace: instance.Namespace}, OVNNorthd); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return false, err
		}
	}
	if instance.Spec.TLS.PodLevel.Enabled {
		ovnNorthdSpec.TLS = OVNNorthd.Spec.TLS

		serviceName := ovnv1.ServiceNameOvnNorthd
		// create certificate for ovnnorthd
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetOvnIssuer(),
			CertName:   fmt.Sprintf("%s-ovndbs", "ovnnorthd"),
			Duration:   nil,
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", serviceName, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", serviceName, instance.Namespace, ovnv1.DNSSuffix),
			},
			Ips: nil,
			Usages: []certmgrv1.KeyUsage{
				certmgrv1.UsageKeyEncipherment,
				certmgrv1.UsageDigitalSignature,
				certmgrv1.UsageServerAuth,
				certmgrv1.UsageClientAuth,
			},
		}
		certSecret, ctrlResult, err := certmanager.EnsureCert(
			ctx,
			helper,
			certRequest,
			nil)
		if err != nil {
			return false, err
		} else if (ctrlResult != ctrl.Result{}) {
			return false, nil
		}

		ovnNorthdSpec.TLS.SecretName = &certSecret.Name
	}
	ovnNorthdSpec.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	Log.Info("Reconciling OVNNorthd", "OVNNorthd.Namespace", instance.Namespace, "OVNNorthd.Name", "ovnnorthd")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNNorthd, func() error {

		instance.Spec.Ovn.Template.OVNNorthd.DeepCopyInto(&OVNNorthd.Spec)

		if OVNNorthd.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			OVNNorthd.Spec.NodeSelector = instance.Spec.NodeSelector
		}

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
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OVNNorthd %s - %s", OVNNorthd.Name, op))
	}
	return OVNNorthd.IsReady(), nil
}

func ReconcileOVNController(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (bool, error) {
	Log := GetLogger(ctx)

	OVNController := &ovnv1.OVNController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovncontroller",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ovn.Enabled {
		if _, err := EnsureDeleted(ctx, helper, OVNController); err != nil {
			return false, err
		}
		return false, nil
	}

	ovnControllerSpec := &instance.Spec.Ovn.Template.OVNController

	// preserve any previously set TLS certs, set CA cert
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "ovncontroller", Namespace: instance.Namespace}, OVNController); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return false, err
		}
	}
	if instance.Spec.TLS.PodLevel.Enabled {
		ovnControllerSpec.TLS = OVNController.Spec.TLS

		serviceName := ovnv1.ServiceNameOvnController
		// create certificate for ovncontroller
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetOvnIssuer(),
			CertName:   fmt.Sprintf("%s-ovndbs", "ovncontroller"),
			Duration:   nil,
			Hostnames: []string{
				fmt.Sprintf("%s.%s.svc", serviceName, instance.Namespace),
				fmt.Sprintf("%s.%s.svc.%s", serviceName, instance.Namespace, ovnv1.DNSSuffix),
			},
			Ips: nil,
			Usages: []certmgrv1.KeyUsage{
				certmgrv1.UsageKeyEncipherment,
				certmgrv1.UsageDigitalSignature,
				certmgrv1.UsageServerAuth,
				certmgrv1.UsageClientAuth,
			},
		}
		certSecret, ctrlResult, err := certmanager.EnsureCert(
			ctx,
			helper,
			certRequest,
			nil)
		if err != nil {
			return false, err
		} else if (ctrlResult != ctrl.Result{}) {
			return false, nil
		}

		ovnControllerSpec.TLS.SecretName = &certSecret.Name
	}
	ovnControllerSpec.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	Log.Info("Reconciling OVNController", "OVNController.Namespace", instance.Namespace, "OVNController.Name", "ovncontroller")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNController, func() error {

		instance.Spec.Ovn.Template.OVNController.DeepCopyInto(&OVNController.Spec)

		if OVNController.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			OVNController.Spec.NodeSelector = instance.Spec.NodeSelector
		}

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
		return false, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OVNController %s - %s", OVNController.Name, op))
	}

	return OVNController.IsReady(), nil
}
