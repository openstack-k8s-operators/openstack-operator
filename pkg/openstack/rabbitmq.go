package openstack

import (
	"context"
	"fmt"
	"strings"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"

	// Cannot use the following import due to linting error:
	// Error: 	pkg/openstack/rabbitmq.go:10:2: use of internal package github.com/rabbitmq/cluster-operator/internal/status not allowed
	//rabbitstatus "github.com/rabbitmq/cluster-operator/internal/status"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"k8s.io/utils/ptr"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"

	rabbitmqv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
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
		instance.Spec.Rabbitmq.Templates = ptr.To(map[string]rabbitmqv1.RabbitMqSpecCore{})
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
	spec rabbitmqv1.RabbitMqSpecCore,
) (mqStatus, ctrl.Result, error) {
	rabbitmq := &rabbitmqv1.RabbitMq{
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

	clusterDomain := clusterdns.GetDNSClusterDomain()
	hostname := fmt.Sprintf("%s.%s.svc", name, instance.Namespace)
	hostnameHeadless := fmt.Sprintf("%s-nodes.%s.svc", name, instance.Namespace)
	hostnames := []string{
		hostname,
		fmt.Sprintf("%s.%s", hostname, clusterDomain),
		hostnameHeadless,
		fmt.Sprintf("%s.%s", hostnameHeadless, clusterDomain),
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
				Organizations: []string{fmt.Sprintf("%s.%s", rabbitmq.Namespace, clusterDomain)},
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

	if spec.NodeSelector == nil {
		spec.NodeSelector = &instance.Spec.NodeSelector
	}

	// When there's no Topology referenced in the Service Template, inject the
	// top-level one
	if spec.TopologyRef == nil {
		spec.TopologyRef = instance.Spec.TopologyRef
	}

	// infra operator is now the controller
	err := removeRabbitmqClusterControllerReference(ctx, helper, instance, name)
	if err != nil {
		return mqFailed, ctrl.Result{}, err
	}
	// infra operator is now the controller
	err = removeConfigMapControllerReference(ctx, helper, instance, name)
	if err != nil {
		return mqFailed, ctrl.Result{}, err
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), rabbitmq, func() error {
		spec.DeepCopyInto(&rabbitmq.Spec.RabbitMqSpecCore)
		if rabbitmq.Spec.Persistence.StorageClassName == nil {
			Log.Info(fmt.Sprintf("Setting StorageClassName: " + instance.Spec.StorageClass))
			rabbitmq.Spec.Persistence.StorageClassName = &instance.Spec.StorageClass
		}
		if tlsCert != "" {

			rabbitmq.Spec.TLS.SecretName = tlsCert
			rabbitmq.Spec.TLS.CaSecretName = tlsCert
			rabbitmq.Spec.TLS.DisableNonTLSListeners = true
		}
		rabbitmq.Spec.ContainerImage = *version.Status.ContainerImages.RabbitmqImage
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

	if rabbitmq.Status.ObservedGeneration == rabbitmq.Generation && rabbitmq.IsReady() {
		instance.Status.ContainerImages.InfraMemcachedImage = version.Status.ContainerImages.InfraMemcachedImage
		return mqReady, ctrl.Result{}, nil
	}

	return mqCreating, ctrl.Result{}, nil
}

func removeRabbitmqClusterControllerReference(
	ctx context.Context,
	helper *helper.Helper,
	instance *corev1beta1.OpenStackControlPlane,
	name string,
) error {
	rabbitmqCluster := &rabbitmqv2.RabbitmqCluster{}
	namespacedName := types.NamespacedName{
		Name:      name,
		Namespace: instance.Namespace,
	}
	if err := helper.GetClient().Get(ctx, namespacedName, rabbitmqCluster); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if metav1.IsControlledBy(rabbitmqCluster, instance) {
		_, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), rabbitmqCluster, func() error {
			return controllerutil.RemoveControllerReference(helper.GetBeforeObject(), rabbitmqCluster, helper.GetScheme())
		})
		return err
	}
	return nil
}

func removeConfigMapControllerReference(
	ctx context.Context,
	helper *helper.Helper,
	instance *corev1beta1.OpenStackControlPlane,
	name string,
) error {
	configMap := &corev1.ConfigMap{}
	namespacedName := types.NamespacedName{
		Name:      fmt.Sprintf("%s-config-data", name),
		Namespace: instance.Namespace,
	}
	if err := helper.GetClient().Get(ctx, namespacedName, configMap); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if metav1.IsControlledBy(configMap, instance) {
		_, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), configMap, func() error {
			return controllerutil.RemoveControllerReference(helper.GetBeforeObject(), configMap, helper.GetScheme())
		})
		return err
	}
	return nil
}

// RabbitmqImageMatch - return true if the rabbitmq images match on the ControlPlane and Version, or if Rabbitmq is not enabled
func RabbitmqImageMatch(ctx context.Context, controlPlane *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion) bool {
	Log := GetLogger(ctx)
	if controlPlane.Spec.Rabbitmq.Enabled {
		if !stringPointersEqual(controlPlane.Status.ContainerImages.RabbitmqImage, version.Status.ContainerImages.RabbitmqImage) {
			Log.Info("RabbitMQ image mismatch", "controlPlane.Status.ContainerImages.RabbitmqImage", controlPlane.Status.ContainerImages.RabbitmqImage, "version.Status.ContainerImages.RabbitmqImage", version.Status.ContainerImages.RabbitmqImage)
			return false
		}
	}

	return true
}
