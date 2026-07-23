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
mkdir -p $HOME/.config/goose/custom_providers

# Write goose config.yaml
cat > $HOME/.config/goose/config.yaml <<'GOOSE_CONFIG'
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
    echo "  ${cmdname}:" >> $HOME/.config/goose/config.yaml
    echo "    type: recipe" >> $HOME/.config/goose/config.yaml
    echo "    enabled: true" >> $HOME/.config/goose/config.yaml
    echo "    recipe_source: ${recipe}" >> $HOME/.config/goose/config.yaml
  done
fi

# Discover and register MCP servers from environment variables
# MCP_SERVER_<name>=<url> entries are set by the controller
env | grep '^MCP_SERVER_' | while IFS='=' read -r varname url; do
  name="${varname#MCP_SERVER_}"
  # Convert to lowercase for the extension key
  name=$(echo "$name" | tr '[:upper:]' '[:lower:]')
  cat >> $HOME/.config/goose/config.yaml <<MCPEOF
  ${name}:
    type: streamable_http
    name: ${name}
    uri: ${url}
    description: "${name} MCP server"
    enabled: true
    timeout: 300
MCPEOF
done

# Copy hints if present
if [ -f /tmp/hints/hints ]; then
  cp /tmp/hints/hints ~/.goosehints
fi

# Copy lightspeed provider config
if [ -f /tmp/lightspeed-provider/lightspeed.json ]; then
  cp /tmp/lightspeed-provider/lightspeed.json $HOME/.config/goose/custom_providers/lightspeed.json
fi

# Trust the OpenShift service-serving CA so TLS connections to
# in-cluster services (e.g. lightspeed-app-server) verify successfully.
# We cannot write to /etc/pki (read-only as non-root), so we build a
# merged bundle in $HOME and point SSL_CERT_FILE at it.
SERVICE_CA="/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"
if [ -f "$SERVICE_CA" ]; then
  MERGED_CA="$HOME/ca-bundle.crt"
  # Start from the system bundle if SSL_CERT_FILE is already set
  # (e.g. combined-ca.crt mounted by the controller), otherwise
  # fall back to the default system CA bundle.
  BASE_CA="${SSL_CERT_FILE:-/etc/pki/tls/certs/ca-bundle.crt}"
  cat "$BASE_CA" "$SERVICE_CA" > "$MERGED_CA"
  export SSL_CERT_FILE="$MERGED_CA"
fi

# Set the API key in the current process environment so it propagates
# to the sleep process and is visible to oc exec/rsh sessions.
export LIGHTSPEED_API_KEY="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"

# Write env snippets so oc rsh / exec sessions pick up the API key,
# SSL trust, and any other assistant-specific env vars.
# The container spec sets ENV and BASH_ENV to this path so every
# shell (sh and bash, interactive or not) sources it automatically.
GOOSE_ENV='export LIGHTSPEED_API_KEY="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token 2>/dev/null)"'
if [ -n "$SSL_CERT_FILE" ]; then
  GOOSE_ENV="${GOOSE_ENV}
export SSL_CERT_FILE=\"${SSL_CERT_FILE}\""
fi
echo "$GOOSE_ENV" > /tmp/assistant-env.sh
echo "$GOOSE_ENV" >> "$HOME/.bashrc"
echo "$GOOSE_ENV" >> "$HOME/.profile"

exec sleep infinity
`
}

const combinedCAMountPath = "/etc/ssl/certs/combined-ca.crt"

// AssistantPodSpec returns the PodSpec for the assistant pod.
// resolvedMCPServers maps extension name to URL for all MCP servers
// (both manually specified and auto-resolved from OpenStackClientRef).
// hasCombinedCA indicates the entrypoint ConfigMap contains a combined-ca.crt
// key with pre-merged CA bundles from both lightspeed and MCP sources.
func AssistantPodSpec(
	instance *assistantv1.OpenStackAssistant,
	configHash string,
	resolvedMCPServers map[string]string,
	hasCombinedCA bool,
) corev1.PodSpec {
	envVars := map[string]env.Setter{}
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["GOOSE_PROVIDER"] = env.SetValue("lightspeed")
	envVars["GOOSE_TELEMETRY_ENABLED"] = env.SetValue("false")
	envVars["GOOSE_DISABLE_KEYRING"] = env.SetValue("1")
	envVars["ENV"] = env.SetValue("/tmp/assistant-env.sh")
	envVars["BASH_ENV"] = env.SetValue("/tmp/assistant-env.sh")

	if hasCombinedCA {
		envVars["SSL_CERT_FILE"] = env.SetValue(combinedCAMountPath)
	} else if instance.Spec.LightspeedStack.CaBundleSecretName != "" {
		envVars["SSL_CERT_FILE"] = env.SetValue("/etc/ssl/certs/ca-certificates.crt")
	}

	if instance.Spec.Goose != nil && instance.Spec.Goose.Model != "" {
		envVars["GOOSE_MODEL"] = env.SetValue(instance.Spec.Goose.Model)
	}

	for name, url := range resolvedMCPServers {
		envVars["MCP_SERVER_"+name] = env.SetValue(url)
	}

	if instance.Spec.Env != nil {
		for idx := range instance.Spec.Env {
			e := instance.Spec.Env[idx]
			envVars[e.Name] = func(env *corev1.EnvVar) {
				env.Value = e.Value
				env.ValueFrom = e.ValueFrom
			}
		}
	}

	volumes := assistantPodVolumes(instance, hasCombinedCA)
	volumeMounts := assistantPodVolumeMounts(instance, hasCombinedCA)

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

func assistantPodVolumeMounts(instance *assistantv1.OpenStackAssistant, hasCombinedCA bool) []corev1.VolumeMount {
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

	if hasCombinedCA {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "entrypoint",
			MountPath: combinedCAMountPath,
			SubPath:   "combined-ca.crt",
			ReadOnly:  true,
		})
	} else if instance.Spec.LightspeedStack.CaBundleSecretName != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "ca-bundle",
			MountPath: "/etc/ssl/certs/ca-certificates.crt",
			SubPath:   "ca-bundle.crt",
			ReadOnly:  true,
		})
	}

	return mounts
}

func assistantPodVolumes(instance *assistantv1.OpenStackAssistant, hasCombinedCA bool) []corev1.Volume {
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

	if !hasCombinedCA && instance.Spec.LightspeedStack.CaBundleSecretName != "" {
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
