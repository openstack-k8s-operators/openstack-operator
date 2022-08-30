package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileRabbitMQ -
func ReconcileRabbitMQ(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {

	rabbitmq := &rabbitmqv1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rabbitmq", //FIXME
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling Rabbitmq", "RabbitMQ.Namespace", instance.Namespace, "rabbitMQ.Name", "rabbitmq")

	defaultStatefulSet := rabbitmqv1.StatefulSet{
		Spec: &rabbitmqv1.StatefulSetSpec{
			Template: &rabbitmqv1.PodTemplateSpec{
				Spec: &corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{},
					Containers:      []corev1.Container{},
					InitContainers: []corev1.Container{
						{Name: "setup-container", SecurityContext: &corev1.SecurityContext{}}},
				},
			},
		},
	}
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), rabbitmq, func() error {

		instance.Spec.RabbitmqTemplate.DeepCopyInto(&rabbitmq.Spec)

		//FIXME: We shouldn't have to set this here but not setting it causes the rabbitmq
		// operator to continuously mutate the CR when setting it:
		// https://github.com/rabbitmq/cluster-operator/blob/main/controllers/reconcile_operator_defaults.go#L19
		if rabbitmq.Spec.Image == "" {
			rabbitmq.Spec.Image = "rabbitmq:3.10.2-management"
		}

		if rabbitmq.Spec.Persistence.StorageClassName == nil {
			helper.GetLogger().Info("Setting StorageClassName: " + instance.Spec.StorageClass)
			rabbitmq.Spec.Persistence.StorageClassName = &instance.Spec.StorageClass
		}

		if rabbitmq.Spec.Override.StatefulSet == nil {
			helper.GetLogger().Info("Setting StatefulSet")
			rabbitmq.Spec.Override.StatefulSet = &defaultStatefulSet
		}

		// overrides
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), rabbitmq, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("RabbitMQ %s - %s", rabbitmq.Name, op))
	}

	return ctrl.Result{}, nil

}
