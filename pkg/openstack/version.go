package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ReconcileGlance -
func ReconcileVersion(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	version := &corev1beta1.OpenStackVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	Log := GetLogger(ctx)

	// return if OpenStackVersion CR already exists
	if err := helper.GetClient().Get(ctx, types.NamespacedName{
		Name:      instance.Name,
		Namespace: instance.Namespace,
	},
		version); err == nil {
		return ctrl.Result{}, nil
	} else if err != nil {
		Log.Info(fmt.Sprintf("OpenStackVersion does not exist. Creating: %s", version.Name))
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), version, func() error {
		version.Spec.OpenStackControlPlaneName = instance.Name

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), version, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("OpenStackVersion %s - %s", version.Name, op))
	}

	return ctrl.Result{}, nil
}
