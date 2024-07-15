package openstack

import (
	"context"
	"fmt"
	"strings"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/ocp"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	rabbitmqv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

	// Cannot use the following import due to linting error:
	// Error: 	pkg/openstack/rabbitmq.go:10:2: use of internal package github.com/rabbitmq/cluster-operator/internal/status not allowed
	//rabbitstatus "github.com/rabbitmq/cluster-operator/internal/status"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

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
	version *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
) (ctrl.Result, error) {
	var failures = []string{}
	var inprogress = []string{}
	var ctrlResult ctrl.Result
	var err error
	var status mqStatus

	if instance.Spec.Rabbitmq.Templates == nil {
		instance.Spec.Rabbitmq.Templates = ptr.To(map[string]corev1beta1.RabbitmqTemplate{})
	}

	for name, spec := range *instance.Spec.Rabbitmq.Templates {
		status, ctrlResult, err = reconcileRabbitMQ(ctx, instance, version, helper, name, spec)

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
	version *corev1beta1.OpenStackVersion,
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
		instance.Status.ContainerImages.RabbitmqImage = nil
		return mqReady, ctrl.Result{}, nil
	}

	envVars := []corev1.EnvVar{
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
	}

	IPv6Enabled, err := ocp.HasIPv6ClusterNetwork(ctx, helper)
	if err != nil {
		return mqFailed, ctrl.Result{}, err
	}
	inetFamily := "inet"
	inetProtocol := "tcp"
	tlsArgs := ""
	fipsArgs := ""
	if IPv6Enabled {
		inetFamily = "inet6"
	}
	erlangInetConfig := fmt.Sprintf("{%s,true}.\n", inetFamily)

	if instance.Spec.TLS.PodLevel.Enabled {
		inetProtocol = "tls"
		tlsArgs = "-ssl_dist_optfile /etc/rabbitmq/inter-node-tls.config"
		fipsEnabled, err := ocp.IsFipsCluster(ctx, helper)
		if err != nil {
			return mqFailed, ctrl.Result{}, err
		}
		if fipsEnabled {
			fipsArgs = "-crypto fips_mode true"
		}
	}
	envVars = append(envVars, corev1.EnvVar{
		Name: "RABBITMQ_SERVER_ADDITIONAL_ERL_ARGS",
		Value: fmt.Sprintf(
			"-kernel inetrc '/etc/rabbitmq/erl_inetrc' -proto_dist %s_%s %s %s",
			inetFamily,
			inetProtocol,
			tlsArgs,
			fipsArgs,
		),
	}, corev1.EnvVar{
		Name:  "RABBITMQ_CTL_ERL_ARGS",
		Value: fmt.Sprintf("-proto_dist %s_%s %s", inetFamily, inetProtocol, tlsArgs),
	})

	cms := []util.Template{
		{
			Name:         fmt.Sprintf("%s-config-data", rabbitmq.Name),
			Namespace:    rabbitmq.Namespace,
			Type:         util.TemplateTypeConfig,
			InstanceType: "rabbitmq",
			Labels:       map[string]string{},
			CustomData: map[string]string{
				"inter_node_tls.config": `[
  {server, [
    {cacertfile,"/etc/rabbitmq-tls/ca.crt"},
    {certfile,"/etc/rabbitmq-tls/tls.crt"},
    {keyfile,"/etc/rabbitmq-tls/tls.key"},
    {secure_renegotiate, true},
    {fail_if_no_peer_cert, true},
    {verify, verify_peer},
    {versions, ['tlsv1.2','tlsv1.3']}
  ]},
  {client, [
    {cacertfile,"/etc/rabbitmq-tls/ca.crt"},
    {certfile,"/etc/rabbitmq-tls/tls.crt"},
    {keyfile,"/etc/rabbitmq-tls/tls.key"},
    {secure_renegotiate, true},
    {verify, verify_peer},
    {versions, ['tlsv1.2','tlsv1.3']}
  ]}
].
`,
			},
		},
	}

	err = configmap.EnsureConfigMaps(ctx, helper, instance, cms, nil)
	if err != nil {
		Log.Error(err, "Unable to create rabbitmq config maps")
		return mqFailed, ctrl.Result{}, err
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
							Image: *version.Status.ContainerImages.RabbitmqImage,
							Env:   envVars,
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
	hostnameHeadless := fmt.Sprintf("%s-nodes.%s.svc", name, instance.Namespace)
	hostnames := []string{
		hostname,
		fmt.Sprintf("%s.%s", hostname, ClusterInternalDomain),
		hostnameHeadless,
		fmt.Sprintf("%s.%s", hostnameHeadless, ClusterInternalDomain),
	}
	for i := 0; i < int(*spec.Replicas); i++ {
		hostnames = append(hostnames, fmt.Sprintf("%s-server-%d.%s-nodes.%s", name, i, name, instance.Namespace))
	}

	tlsCert := ""
	if instance.Spec.TLS.PodLevel.Enabled {
		certRequest := certmanager.CertificateRequest{
			IssuerName: instance.GetInternalIssuer(),
			CertName:   fmt.Sprintf("%s-svc", rabbitmq.Name),
			Hostnames:  hostnames,
			Subject: &certmgrv1.X509Subject{
				Organizations: []string{fmt.Sprintf("%s.%s", rabbitmq.Namespace, ClusterInternalDomain)},
			},
			Usages: []certmgrv1.KeyUsage{
				certmgrv1.UsageKeyEncipherment,
				certmgrv1.UsageDataEncipherment,
				certmgrv1.UsageDigitalSignature,
				certmgrv1.UsageServerAuth,
				certmgrv1.UsageClientAuth,
				certmgrv1.UsageContentCommitment,
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
			return mqFailed, ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return mqCreating, ctrlResult, nil
		}

		tlsCert = certSecret.Name
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), rabbitmq, func() error {

		rabbitmq.Spec.Image = *version.Status.ContainerImages.RabbitmqImage
		rabbitmq.Spec.Replicas = spec.Replicas
		rabbitmq.Spec.Tolerations = spec.Tolerations
		rabbitmq.Spec.SkipPostDeploySteps = spec.SkipPostDeploySteps
		rabbitmq.Spec.TerminationGracePeriodSeconds = spec.TerminationGracePeriodSeconds
		rabbitmq.Spec.DelayStartSeconds = spec.DelayStartSeconds
		spec.Service.DeepCopyInto(&rabbitmq.Spec.Service)
		spec.Persistence.DeepCopyInto(&rabbitmq.Spec.Persistence)
		spec.Override.DeepCopyInto(&rabbitmq.Spec.Override)
		spec.SecretBackend.DeepCopyInto(&rabbitmq.Spec.SecretBackend)
		if spec.Resources != nil {
			rabbitmq.Spec.Resources = spec.Resources
			//spec.Resources.DeepCopyInto(rabbitmq.Spec.Resources)
		}
		if spec.Affinity != nil {
			rabbitmq.Spec.Affinity = spec.Affinity
			//spec.Affinity.DeepCopyInto(rabbitmq.Spec.Affinity)
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
			var settings []string
			settings = append(settings, "log.console = true")
			if tlsCert != "" {
				settings = append(settings, "ssl_options.verify = verify_none")
			}
			rabbitmq.Spec.Rabbitmq.AdditionalConfig = strings.Join(settings, "\n")
		}

		rabbitmq.Spec.Rabbitmq.ErlangInetConfig = erlangInetConfig
		rabbitmq.Spec.Rabbitmq.AdvancedConfig = ""
		if tlsCert != "" {
			rabbitmq.Spec.TLS.CaSecretName = tlsCert
			rabbitmq.Spec.TLS.SecretName = tlsCert
			// disable non tls listeners
			rabbitmq.Spec.TLS.DisableNonTLSListeners = true
			// NOTE(dciabrin) RabbitMQ/Erlang needs a specific TLS configuration ordering
			// in ssl_options.versions for TLS to work with FIPS. We cannot enforce the right
			// ordering with AdditionalConfig, we have to pass a specific Erlang value via
			// the AdvancedConfig field. We also add configuration flags which were known to
			// work with FIPS in previous version of Openstack.
			rabbitmq.Spec.Rabbitmq.AdvancedConfig = `[
  {rabbit, [
    {ssl_options, [
      {cacertfile,"/etc/rabbitmq-tls/ca.crt"},
      {certfile,"/etc/rabbitmq-tls/tls.crt"},
      {keyfile,"/etc/rabbitmq-tls/tls.key"},
      {depth,1},
      {secure_renegotiate,true},
      {reuse_sessions,true},
      {honor_cipher_order,false},
      {honor_ecc_order,false},
      {verify,verify_none},
      {fail_if_no_peer_cert,false},
      {versions, ['tlsv1.2','tlsv1.3']}
    ]}
  ]},
  {rabbitmq_management, [
    {ssl_config, [
      {cacertfile,"/etc/rabbitmq-tls/ca.crt"},
      {certfile,"/etc/rabbitmq-tls/tls.crt"},
      {keyfile,"/etc/rabbitmq-tls/tls.key"},
      {depth,1},
      {secure_renegotiate,true},
      {reuse_sessions,true},
      {honor_cipher_order,false},
      {honor_ecc_order,false},
      {verify,verify_none},
      {fail_if_no_peer_cert,false},
      {versions, ['tlsv1.2','tlsv1.3']}
    ]}
  ]},
  {client, [
    {cacertfile, "/etc/rabbitmq-tls/ca.crt"},
    {verify,verify_peer},
    {secure_renegotiate,true},
    {versions, ['tlsv1.2','tlsv1.3']}
  ]}
].
`

			rabbitmq.Spec.Override.StatefulSet.Spec.Template.Spec.Volumes = []corev1.Volume{
				{
					Name: "config-data",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: fmt.Sprintf("%s-config-data", rabbitmq.Name),
							},
							DefaultMode: ptr.To[int32](0o420),
							Items: []corev1.KeyToPath{
								{
									Key:  "inter_node_tls.config",
									Path: "inter_node_tls.config",
								},
							},
						},
					},
				},
			}
			rabbitmq.Spec.Override.StatefulSet.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
				{
					MountPath: "/etc/rabbitmq/inter-node-tls.config",
					ReadOnly:  true,
					Name:      "config-data",
					SubPath:   "inter_node_tls.config",
				},
			}
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

	if rabbitmq.Status.ObservedGeneration == rabbitmq.Generation {
		for _, oldCond := range rabbitmq.Status.Conditions {
			// Forced to hardcode "ClusterAvailable" here because linter will not allow
			// us to import "github.com/rabbitmq/cluster-operator/internal/status"
			if string(oldCond.Type) == "ClusterAvailable" && oldCond.Status == corev1.ConditionTrue {
				Log.Info("RabbitMQ ready condition is true")
				instance.Status.ContainerImages.RabbitmqImage = version.Status.ContainerImages.RabbitmqImage
				return mqReady, ctrl.Result{}, nil
			}
		}
	}

	return mqCreating, ctrl.Result{}, nil
}

// RabbitmqImageMatch - return true if the rabbitmq images match on the ControlPlane and Version, or if Rabbitmq is not enabled
func RabbitmqImageMatch(controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {

	if controlPlane.Spec.Rabbitmq.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.RabbitmqImage, version.Status.ContainerImages.RabbitmqImage) {
			return false
		}
	}

	return true
}
