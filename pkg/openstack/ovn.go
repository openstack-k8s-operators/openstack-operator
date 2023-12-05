package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileOVN -
func ReconcileOVN(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {

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
			if res, err := EnsureDeleted(ctx, helper, OVNDBCluster); err != nil {
				return res, err
			}
			instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
			continue
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
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneOVNReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1beta1.OpenStackControlPlaneOVNReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			Log.Info(fmt.Sprintf("OVNDBCluster %s - %s", OVNDBCluster.Name, op))
		}
		OVNDBClustersReady = OVNDBClustersReady && OVNDBCluster.IsReady()
	}

	OVNNorthd := &ovnv1.OVNNorthd{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovnnorthd",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ovn.Enabled {
		if res, err := EnsureDeleted(ctx, helper, OVNNorthd); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
		return ctrl.Result{}, nil
	}

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
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OVNNorthd %s - %s", OVNNorthd.Name, op))
	}

	OVNController := &ovnv1.OVNController{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ovncontroller",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Ovn.Enabled {
		if res, err := EnsureDeleted(ctx, helper, OVNController); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneOVNReadyCondition)
		return ctrl.Result{}, nil
	}

	Log.Info("Reconciling OVNController", "OVNController.Namespace", instance.Namespace, "OVNController.Name", "ovncontroller")
	op, err = controllerutil.CreateOrPatch(ctx, helper.GetClient(), OVNController, func() error {

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
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OVNController %s - %s", OVNController.Name, op))
	}

	// Expect all services (dbclusters, northd, ovn-controller) ready
	if OVNDBClustersReady && OVNNorthd.IsReady() && OVNController.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneOVNReadyCondition, corev1beta1.OpenStackControlPlaneOVNReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneOVNReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneOVNReadyRunningMessage))
	}
	return ctrl.Result{}, nil
}
