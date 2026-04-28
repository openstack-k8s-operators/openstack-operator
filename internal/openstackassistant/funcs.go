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

// Package openstackassistant provides functionality for managing OpenStack assistant resources
package openstackassistant

import (
	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// EntrypointScript returns the entrypoint shell script for the goose provider
func EntrypointScript() string {
	return `#!/bin/sh
set -eu

# Create goose config directory
mkdir -p ~/.goose/config/profiles/default/custom_providers

# Write goose config.yaml
cat > ~/.goose/config/profiles/default/config.yaml <<'GOOSE_CONFIG'
extensions:
  developer:
    enabled: true
    type: builtin
  computercontroller:
    enabled: false
    type: builtin
  summarize:
    enabled: true
    type: builtin
  summon:
    enabled: true
    type: builtin
  apps:
    enabled: false
    type: builtin
  analyze:
    enabled: false
    type: builtin
  todo:
    enabled: false
    type: builtin
  extensionmanager:
    enabled: false
    type: builtin
  chatrecall:
    enabled: false
    type: builtin
GOOSE_CONFIG

# Discover and register recipe files as slash commands
if [ -d /tmp/recipes ]; then
  for recipe in /tmp/recipes/*.yaml /tmp/recipes/*.yml; do
    [ -f "$recipe" ] || continue
    basename=$(basename "$recipe")
    # Strip extension to get the command name
    cmdname="${basename%.*}"
    echo "  ${cmdname}:" >> ~/.goose/config/profiles/default/config.yaml
    echo "    type: recipe" >> ~/.goose/config/profiles/default/config.yaml
    echo "    enabled: true" >> ~/.goose/config/profiles/default/config.yaml
    echo "    recipe_source: ${recipe}" >> ~/.goose/config/profiles/default/config.yaml
  done
fi

# Copy hints if present
if [ -f /tmp/hints/hints ]; then
  cp /tmp/hints/hints ~/.goosehints
fi

# Copy lightspeed provider config
if [ -f /tmp/lightspeed-provider/lightspeed.json ]; then
  cp /tmp/lightspeed-provider/lightspeed.json ~/.goose/config/profiles/default/custom_providers/lightspeed.json
fi

exec sleep infinity
`
}

// AssistantPodSpec returns the PodSpec for the assistant pod
func AssistantPodSpec(
	instance *assistantv1.OpenStackAssistant,
	configHash string,
) corev1.PodSpec {
	envVars := map[string]env.Setter{}
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["GOOSE_PROVIDER"] = env.SetValue("lightspeed")
	envVars["GOOSE_TELEMETRY_ENABLED"] = env.SetValue("false")
	envVars["GOOSE_DISABLE_KEYRING"] = env.SetValue("1")

	if instance.Spec.Env != nil {
		for idx := range instance.Spec.Env {
			e := instance.Spec.Env[idx]
			envVars[e.Name] = func(env *corev1.EnvVar) {
				env.Value = e.Value
				env.ValueFrom = e.ValueFrom
			}
		}
	}

	volumes := assistantPodVolumes(instance)
	volumeMounts := assistantPodVolumeMounts(instance)

	containerName := "goose"
	if instance.Spec.Provider != "" {
		containerName = string(instance.Spec.Provider)
	}

	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: ptr.To[int64](0),
		ServiceAccountName:            instance.RbacResourceName(),
		Volumes:                       volumes,
		Containers: []corev1.Container{
			{
				Name:    containerName,
				Image:   instance.Spec.ContainerImage,
				Command: []string{"/bin/sh"},
				Args:    []string{"/tmp/entrypoint/entrypoint.sh"},
				SecurityContext: &corev1.SecurityContext{
					RunAsNonRoot:             ptr.To(true),
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{
							"ALL",
						},
					},
				},
				Env:          env.MergeEnvs([]corev1.EnvVar{}, envVars),
				VolumeMounts: volumeMounts,
			},
		},
	}

	if instance.Spec.NodeSelector != nil {
		podSpec.NodeSelector = *instance.Spec.NodeSelector
	}

	return podSpec
}

func assistantPodVolumeMounts(instance *assistantv1.OpenStackAssistant) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "entrypoint",
			MountPath: "/tmp/entrypoint",
			ReadOnly:  true,
		},
		{
			Name:      "lightspeed-provider",
			MountPath: "/tmp/lightspeed-provider",
			ReadOnly:  true,
		},
	}

	if instance.Spec.Goose != nil {
		if instance.Spec.Goose.Recipes != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      "recipes",
				MountPath: "/tmp/recipes",
				ReadOnly:  true,
			})
		}
		if instance.Spec.Goose.Hints != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      "hints",
				MountPath: "/tmp/hints",
				ReadOnly:  true,
			})
		}
	}

	if instance.Spec.LightspeedStack.CaBundleSecretName != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "ca-bundle",
			MountPath: "/etc/ssl/certs/ca-certificates.crt",
			SubPath:   "ca-bundle.crt",
			ReadOnly:  true,
		})
	}

	return mounts
}

func assistantPodVolumes(instance *assistantv1.OpenStackAssistant) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "entrypoint",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Name + "-entrypoint",
					},
					DefaultMode: ptr.To[int32](0755),
				},
			},
		},
		{
			Name: "lightspeed-provider",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: instance.Spec.LightspeedStack.ProviderSecret,
				},
			},
		},
	}

	if instance.Spec.Goose != nil {
		if instance.Spec.Goose.Recipes != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "recipes",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: *instance.Spec.Goose.Recipes,
						},
					},
				},
			})
		}
		if instance.Spec.Goose.Hints != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "hints",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: *instance.Spec.Goose.Hints,
						},
					},
				},
			})
		}
	}

	if instance.Spec.LightspeedStack.CaBundleSecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "ca-bundle",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: instance.Spec.LightspeedStack.CaBundleSecretName,
				},
			},
		})
	}

	return volumes
}
