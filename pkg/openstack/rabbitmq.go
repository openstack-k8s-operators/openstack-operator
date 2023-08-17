package openstack

import (
	"context"
	"fmt"
	"strings"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"

	// Cannot use the following import due to linting error:
	// Error: 	pkg/openstack/rabbitmq.go:10:2: use of internal package github.com/rabbitmq/cluster-operator/internal/status not allowed
	//rabbitstatus "github.com/rabbitmq/cluster-operator/internal/status"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

type mqStatus int

const (
	mqFailed   mqStatus = iota
	mqCreating mqStatus = iota
	mqReady    mqStatus = iota
)

// ReconcileRabbitMQs -
func ReconcileRabbitMQs(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
) (ctrl.Result, error) {
	var failures []string = []string{}
	var inprogress []string = []string{}

	for name, spec := range instance.Spec.Rabbitmq.Templates {
		status, err := reconcileRabbitMQ(ctx, instance, helper, name, spec)

		switch status {
		case mqFailed:
			failures = append(failures, fmt.Sprintf("%s(%v)", name, err.Error()))
		case mqCreating:
			inprogress = append(inprogress, name)
		case mqReady:
		default:
			return ctrl.Result{}, fmt.Errorf("invalid mqStatus from reconcileRabbitMQ: %d for RAbbitMQ %s", status, name)
		}
	}

	if len(failures) > 0 {
		errors := strings.Join(failures, ",")

		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneRabbitMQReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneRabbitMQReadyErrorMessage,
			errors))

		return ctrl.Result{}, fmt.Errorf(errors)

	} else if len(inprogress) > 0 {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneRabbitMQReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneRabbitMQReadyRunningMessage))
	} else {
		instance.Status.Conditions.MarkTrue(
			corev1beta1.OpenStackControlPlaneRabbitMQReadyCondition,
			corev1beta1.OpenStackControlPlaneRabbitMQReadyMessage,
		)
	}

	return ctrl.Result{}, nil
}

func reconcileRabbitMQ(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
	name string,
	spec corev1beta1.RabbitmqTemplate,
) (mqStatus, error) {
	rabbitmq := &rabbitmqv1.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling RabbitMQ", "RabbitMQ.Namespace", instance.Namespace, "RabbitMQ.Name", name)
	if !instance.Spec.Rabbitmq.Enabled {
		if _, err := EnsureDeleted(ctx, helper, rabbitmq); err != nil {
			return mqFailed, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneRabbitMQReadyCondition)
		return mqReady, nil
	}

	defaultStatefulSet := rabbitmqv1.StatefulSet{
		Spec: &rabbitmqv1.StatefulSetSpec{
			Template: &rabbitmqv1.PodTemplateSpec{
				EmbeddedObjectMeta: &rabbitmqv1.EmbeddedObjectMeta{},
				Spec: &corev1.PodSpec{
					SecurityContext: &corev1.PodSecurityContext{},
					Containers: []corev1.Container{
						{
							// NOTE(gibi): if this is set according to the
							// RabbitMQCluster name the the Pod will crash
							Name: "rabbitmq",
							// NOTE(gibi): without this the second RabbitMqCluster
							// will fail as the Pod will have no image.
							Image: spec.Image,
							Env: []corev1.EnvVar{
								{
									// The upstream rabbitmq image has /var/log/rabbitmq mode 777, so when
									// openshift runs the rabbitmq container as a random uid it can still write
									// the logs there.  The OSP image however has the directory more constrained,
									// so the random uid cannot write the logs there.  Force it into /var/lib
									// where it can create the file without crashing.
									Name:  "RABBITMQ_UPGRADE_LOG",
									Value: "/var/lib/rabbitmq/rabbitmq_upgrade.log",
								},
								{
									// For some reason HOME needs to be explictly set here even though the entry
									// for the random user in /etc/passwd has the correct homedir set.
									Name:  "HOME",
									Value: "/var/lib/rabbitmq",
								},
								{
									// The various /usr/sbin/rabbitmq* scripts are really all the same
									// wrapper shell-script that performs some "sanity checks" and then
									// invokes the corresponding "real" program in
									// /usr/lib/rabbitmq/bin.  The main "sanity check" is to ensure that
									// the user running the command is either root or rabbitmq.  Inside
									// of an openshift pod, however, the user is neither of these, so
									// the wrapper script will always fail.

									// By putting the real programs ahead of the wrapper in PATH we can
									// avoid the unnecessary check and just run things directly as
									// whatever user the pod has graciously generated for us.
									Name:  "PATH",
									Value: "/usr/lib/rabbitmq/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
								},
							},
							Args: []string{
								// OSP17 runs kolla_start here, instead just run rabbitmq-server directly
								"/usr/lib/rabbitmq/bin/rabbitmq-server",
							},
						},
					},
					InitContainers: []corev1.Container{
						{Name: "setup-container", SecurityContext: &corev1.SecurityContext{}}},
				},
			},
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), rabbitmq, func() error {

		spec.RabbitmqClusterSpec.DeepCopyInto(&rabbitmq.Spec)

		//FIXME: We shouldn't have to set this here but not setting it causes the rabbitmq
		// operator to continuously mutate the CR when setting it:
		// https://github.com/rabbitmq/cluster-operator/blob/main/controllers/reconcile_operator_defaults.go#L19
		if rabbitmq.Spec.Image == "" {
			rabbitmq.Spec.Image = "registry.redhat.io/rhosp-rhel9/openstack-rabbitmq:17.0"
		}

		if rabbitmq.Spec.Persistence.StorageClassName == nil {
			helper.GetLogger().Info("Setting StorageClassName: " + instance.Spec.StorageClass)
			rabbitmq.Spec.Persistence.StorageClassName = &instance.Spec.StorageClass
		}

		if rabbitmq.Spec.Override.StatefulSet == nil {
			helper.GetLogger().Info("Setting StatefulSet")
			rabbitmq.Spec.Override.StatefulSet = &defaultStatefulSet
		}

		if rabbitmq.Spec.Rabbitmq.AdditionalConfig == "" {
			helper.GetLogger().Info("Setting AdditionalConfig")
			// This is the same situation as RABBITMQ_UPGRADE_LOG above,
			// except for the "main" rabbitmq log we can just force it to use the console.
			rabbitmq.Spec.Rabbitmq.AdditionalConfig = "log.console = true"
		}

		// overrides
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), rabbitmq, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return mqFailed, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("RabbitMQ %s - %s", rabbitmq.Name, op))
	}

	for _, oldCond := range rabbitmq.Status.Conditions {
		// Forced to hardcode "ClusterAvailable" here because linter will not allow
		// us to import "github.com/rabbitmq/cluster-operator/internal/status"
		if string(oldCond.Type) == "ClusterAvailable" && oldCond.Status == corev1.ConditionTrue {
			return mqReady, nil
		}
	}

	return mqCreating, nil
}
