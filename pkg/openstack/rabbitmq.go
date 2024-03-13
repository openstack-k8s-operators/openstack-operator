package openstack

import (
	"context"
	"fmt"
	"strings"

	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	rabbitmqv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

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
	var ctrlResult ctrl.Result
	var err error
	var status mqStatus

	for name, spec := range instance.Spec.Rabbitmq.Templates {
		status, ctrlResult, err = reconcileRabbitMQ(ctx, instance, helper, name, spec)

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

	return ctrlResult, nil
}

func reconcileRabbitMQ(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	helper *helper.Helper,
	name string,
	spec corev1beta1.RabbitmqTemplate,
) (mqStatus, ctrl.Result, error) {
	rabbitmq := &rabbitmqv2.RabbitmqCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: instance.Namespace,
		},
	}
	Log := GetLogger(ctx)

	Log.Info("Reconciling RabbitMQ", "RabbitMQ.Namespace", instance.Namespace, "RabbitMQ.Name", name)
	if !instance.Spec.Rabbitmq.Enabled {
		if _, err := EnsureDeleted(ctx, helper, rabbitmq); err != nil {
			return mqFailed, ctrl.Result{}, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneRabbitMQReadyCondition)
		return mqReady, ctrl.Result{}, nil
	}

	defaultStatefulSet := rabbitmqv2.StatefulSet{
		Spec: &rabbitmqv2.StatefulSetSpec{
			Template: &rabbitmqv2.PodTemplateSpec{
				EmbeddedObjectMeta: &rabbitmqv2.EmbeddedObjectMeta{},
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

	hostname := fmt.Sprintf("%s.%s.svc", name, instance.Namespace)
	tlsCert := ""

	if instance.Spec.TLS.PodLevel.Enabled {
		certRequest := certmanager.CertificateRequest{
			IssuerName: tls.DefaultCAPrefix + string(service.EndpointInternal),
			CertName:   fmt.Sprintf("%s-svc", rabbitmq.Name),
			Hostnames:  []string{hostname},
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
			certRequest)
		if err != nil {
			return mqFailed, ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return mqCreating, ctrlResult, nil
		}

		tlsCert = certSecret.Name
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
			Log.Info(fmt.Sprintf("Setting StorageClassName: " + instance.Spec.StorageClass))
			rabbitmq.Spec.Persistence.StorageClassName = &instance.Spec.StorageClass
		}

		if rabbitmq.Spec.Override.StatefulSet == nil {
			Log.Info("Setting StatefulSet")
			rabbitmq.Spec.Override.StatefulSet = &defaultStatefulSet
		}

		if rabbitmq.Spec.Override.Service != nil &&
			rabbitmq.Spec.Override.Service.Spec.Type == corev1.ServiceTypeLoadBalancer {
			rabbitmq.Spec.Override.Service.Annotations =
				util.MergeStringMaps(rabbitmq.Spec.Override.Service.Annotations,
					map[string]string{networkv1.AnnotationHostnameKey: hostname})
		}

		if rabbitmq.Spec.Rabbitmq.AdditionalConfig == "" {
			Log.Info("Setting AdditionalConfig")
			// This is the same situation as RABBITMQ_UPGRADE_LOG above,
			// except for the "main" rabbitmq log we can just force it to use the console.
			rabbitmq.Spec.Rabbitmq.AdditionalConfig = "log.console = true"
		}

		if tlsCert != "" {
			rabbitmq.Spec.TLS.CaSecretName = tls.DefaultCAPrefix + string(service.EndpointInternal)
			rabbitmq.Spec.TLS.SecretName = tlsCert
			// disable non tls listeners
			rabbitmq.Spec.TLS.DisableNonTLSListeners = true
		}

		// overrides
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), rabbitmq, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return mqFailed, ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		Log.Info(fmt.Sprintf("RabbitMQ %s - %s", rabbitmq.Name, op))
	}

	for _, oldCond := range rabbitmq.Status.Conditions {
		// Forced to hardcode "ClusterAvailable" here because linter will not allow
		// us to import "github.com/rabbitmq/cluster-operator/internal/status"
		if string(oldCond.Type) == "ClusterAvailable" && oldCond.Status == corev1.ConditionTrue {
			return mqReady, ctrl.Result{}, nil
		}
	}

	return mqCreating, ctrl.Result{}, nil
}
