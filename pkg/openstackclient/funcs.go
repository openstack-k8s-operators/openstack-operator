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

package openstackclient

import (
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClientPod func
func ClientPod(
	instance *clientv1.OpenStackClient,
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
	clientPod.Spec.ServiceAccountName = instance.RbacResourceName()
	clientPod.Spec.Containers = []corev1.Container{
		{
			Name:    "openstackclient",
			Image:   instance.Spec.ContainerImage,
			Command: []string{"/bin/sleep"},
			Args:    []string{"infinity"},
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
	instance *clientv1.OpenStackClient,
	labels map[string]string,
	configHash string,
	secretHash string,
) []corev1.Volume {

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
	}

}
