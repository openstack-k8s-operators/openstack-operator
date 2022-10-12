package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileCinder -
func ReconcileCinder(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	cinder := &cinderv1.Cinder{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cinder",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling Cinder", "Cinder.Namespace", instance.Namespace, "Cinder.Name", "cinder")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), cinder, func() error {
		instance.Spec.CinderTemplate.DeepCopyInto(&cinder.Spec)
		if cinder.Spec.Secret == "" {
			cinder.Spec.Secret = instance.Spec.Secret
		}
		//if cinder.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
		//cinder.Spec.NodeSelector = instance.Spec.NodeSelector
		//}
		if cinder.Spec.DatabaseInstance == "" {
			//cinder.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			cinder.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), cinder, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Cinder %s - %s", cinder.Name, op))
	}

	return ctrl.Result{}, nil

}
