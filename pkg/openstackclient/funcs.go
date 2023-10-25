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
	"context"

	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// ClientPodSpec func
func ClientPodSpec(
	ctx context.Context,
	instance *clientv1.OpenStackClient,
	helper *helper.Helper,
	labels map[string]string,
	configHash string,
) (*corev1.PodSpec, error) {
	envVars := map[string]env.Setter{}
	envVars["OS_CLOUD"] = env.SetValue("default")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	podSpec := &corev1.PodSpec{}

	podSpec.TerminationGracePeriodSeconds = ptr.To[int64](0)
	podSpec.ServiceAccountName = instance.RbacResourceName()
	clientContainer := corev1.Container{
		Name:    "openstackclient",
		Image:   instance.Spec.ContainerImage,
		Command: []string{"/bin/sleep"},
		Args:    []string{"infinity"},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:                ptr.To[int64](42401),
			RunAsGroup:               ptr.To[int64](42401),
			RunAsNonRoot:             ptr.To(true),
			AllowPrivilegeEscalation: ptr.To(false),
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{
					"ALL",
				},
			},
		},
		Env: env.MergeEnvs([]corev1.EnvVar{}, envVars),
	}

	tlsConfig, err := tls.NewTLS(ctx, helper, instance.Namespace, nil, &instance.Spec.Ca)
	if err != nil {
		return nil, err
	}
	clientContainer.VolumeMounts = clientPodVolumeMounts(tlsConfig)

	podSpec.Containers = []corev1.Container{clientContainer}

	podSpec.Volumes = clientPodVolumes(instance, tlsConfig)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		podSpec.NodeSelector = instance.Spec.NodeSelector
	}

	return podSpec, nil
}

func clientPodVolumeMounts(
	tlsConfig *tls.TLS,
) []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      "openstack-config",
			MountPath: "/home/cloud-admin/.config/openstack/clouds.yaml",
			SubPath:   "clouds.yaml",
		},
		{
			Name:      "openstack-config-secret",
			MountPath: "/home/cloud-admin/.config/openstack/secure.yaml",
			SubPath:   "secure.yaml",
		},
	}
	volumeMounts = append(volumeMounts, tlsConfig.CreateVolumeMounts()...)

	return volumeMounts
}

func clientPodVolumes(
	instance *clientv1.OpenStackClient,
	tlsConfig *tls.TLS,
) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "openstack-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *instance.Spec.OpenStackConfigMap,
					},
				},
			},
		},
		{
			Name: "openstack-config-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: *instance.Spec.OpenStackConfigSecret,
				},
			},
		},
	}
	volumes = append(volumes, tlsConfig.CreateVolumes()...)

	return volumes
}
