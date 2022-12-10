/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package openstack

import (
	"context"
	"fmt"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// ServiceAccount -
	ServiceAccount = "openstack-operator-openstackclient"
)

// ReconcileOpenStackClient -
func ReconcileOpenStackClient(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper, openstackClientImage string) (ctrl.Result, error) {

	openstackclient := &corev1beta1.OpenStackClient{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "openstackclient",
			Namespace: instance.Namespace,
		},
	}

	helper.GetLogger().Info("Reconciling OpenStackClient", "OpenStackClient.Namespace", instance.Namespace, "OpenStackClient.Name", openstackclient.Name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), openstackclient, func() error {
		openstackclient.Spec.ContainerImage = openstackClientImage

		// the following are created/owned by keystoneclient
		openstackclient.Spec.OpenStackConfigMap = "openstack-config"
		openstackclient.Spec.OpenStackConfigSecret = "openstack-config-secret"
		openstackclient.Spec.NodeSelector = instance.Spec.NodeSelector

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), openstackclient, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneClientReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneClientReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("OpenStackClient %s - %s", openstackclient.Name, op))
	}

	if openstackclient.Status.Conditions.IsTrue(condition.DeploymentReadyCondition) {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneClientReadyCondition, corev1beta1.OpenStackControlPlaneClientReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneClientReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneClientReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}

// ClientPod func
func ClientPod(
	instance *corev1beta1.OpenStackClient,
	labels map[string]string,
	configHash string,
	secretHash string,
) *corev1.Pod {

	clientPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	}

	var terminationGracePeriodSeconds int64 = 0
	runAsUser := int64(42401)
	runAsGroup := int64(42401)
	clientPod.ObjectMeta = metav1.ObjectMeta{
		Name:      instance.Name,
		Namespace: instance.Namespace,
		Labels:    labels,
	}
	clientPod.Spec.TerminationGracePeriodSeconds = &terminationGracePeriodSeconds
	clientPod.Spec.ServiceAccountName = ServiceAccount
	clientPod.Spec.Containers = []corev1.Container{
		{
			Name:  "openstackclient",
			Image: instance.Spec.ContainerImage,
			SecurityContext: &corev1.SecurityContext{
				RunAsUser:  &runAsUser,
				RunAsGroup: &runAsGroup,
			},
			Env: []corev1.EnvVar{
				{
					Name:  "OS_CLOUD",
					Value: "default",
				},
				{
					Name:  "KOLLA_CONFIG_STRATEGY",
					Value: "COPY_ALWAYS",
				},
				{
					Name:  "CONFIG_HASH",
					Value: configHash,
				},
				{
					Name:  "SECRET_HASH",
					Value: secretHash,
				},
			},
			VolumeMounts: []corev1.VolumeMount{
				{
					Name:      "openstack-config",
					MountPath: "/etc/openstack/clouds.yaml",
					SubPath:   "clouds.yaml",
				},
				{
					Name:      "openstack-config-secret",
					MountPath: "/etc/openstack/secure.yaml",
					SubPath:   "secure.yaml",
				},
				{
					Name:      "kolla-config",
					MountPath: "/var/lib/kolla/config_files",
					ReadOnly:  true,
				},
			},
		},
	}
	clientPod.Spec.Volumes = clientPodVolumes(instance, labels, configHash, secretHash)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		clientPod.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	return clientPod
}

func clientPodVolumes(
	instance *corev1beta1.OpenStackClient,
	labels map[string]string,
	configHash string,
	secretHash string,
) []corev1.Volume {

	var config0644AccessMode int32 = 0644
	return []corev1.Volume{
		{
			Name: "openstack-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.OpenStackConfigMap,
					},
				},
			},
		},
		{
			Name: "openstack-config-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: instance.Spec.OpenStackConfigSecret,
				},
			},
		},
		{
			Name: "kolla-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					DefaultMode: &config0644AccessMode,
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "openstackclient-sh",
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "kolla_config.json",
							Path: "config.json",
						},
					},
				},
			},
		},
	}

}
