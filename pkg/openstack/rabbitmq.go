package openstack

import (
	"context"
	"fmt"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"

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

		//FIXME: need to tease out which of the RabbitMQ fields can be updated
		if rabbitmq.ObjectMeta.CreationTimestamp.IsZero() {
			instance.Spec.RabbitmqTemplate.DeepCopyInto(&rabbitmq.Spec)
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
		if k8s_errors.IsNotFound(err) {
			helper.GetLogger().Info("RabbitMQ %s not found, reconcile in 10s")
			return ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
		}
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("RabbitMQ %s - %s", rabbitmq.Name, op))
	}

	return ctrl.Result{}, nil

}
