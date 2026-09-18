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
	"fmt"
	"strings"
	"unicode"

	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// ValidateMCPServerRefs validates MCP server names and literal URLs before
// they are exported to the assistant pod.
func ValidateMCPServerRefs(mcpServers []assistantv1.MCPServerRef) error {
	seen := make(map[string]string, len(mcpServers))
	for _, mcp := range mcpServers {
		canonicalName := strings.ToLower(mcp.Name)
		if previousName, exists := seen[canonicalName]; exists {
			return fmt.Errorf("MCP server name %q conflicts with %q when compared case-insensitively", mcp.Name, previousName)
		}
		seen[canonicalName] = mcp.Name
		if err := ValidateMCPServerURL(mcp.Name, mcp.URL); err != nil {
			return err
		}
	}
	return nil
}

// ValidateMCPServerURL rejects values that cannot safely be passed through an
// environment variable and serialized into the Goose YAML configuration.
func ValidateMCPServerURL(name string, url string) error {
	for _, char := range url {
		if unicode.IsControl(char) {
			return fmt.Errorf("MCP server %q URL contains control character %U", name, char)
		}
	}
	return nil
}

// ValidateCustomMCPServerURLs validates literal MCP server URLs supplied
// through the custom pod environment. ValueFrom entries are validated by the
// entrypoint after Kubernetes resolves them.
func ValidateCustomMCPServerURLs(envVars []corev1.EnvVar) error {
	for _, envVar := range envVars {
		if strings.HasPrefix(envVar.Name, "MCP_SERVER_") && envVar.ValueFrom == nil {
			if err := ValidateMCPServerURL(envVar.Name, envVar.Value); err != nil {
				return err
			}
		}
	}
	return nil
}

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
  skills:
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

# Install Agent Skills as regular files. Kubernetes projects ConfigMap keys as
# symlinks, so copy each key into the global directory discovered by Goose's
# explicitly enabled Skills platform extension. Unlike recipes, skills are
# loaded contextually or through /skills and are not individual slash commands.
if [ -d /tmp/skills ]; then
  mkdir -p $HOME/.config/goose/skills
  skill_names=""
  for skill in /tmp/skills/*; do
    [ -f "$skill" ] || continue
    basename=$(basename "$skill")
    name="${basename%.*}"
    if [ -z "$name" ]; then
      echo "invalid Goose skill filename: ${basename}" >&2
      exit 1
    fi
    case " ${skill_names} " in
      *" ${name} "*)
        echo "duplicate Goose skill name: ${name}" >&2
        exit 1
        ;;
    esac
    skill_names="${skill_names} ${name}"
    mkdir -p "$HOME/.config/goose/skills/${name}"
    cp "$skill" "$HOME/.config/goose/skills/${name}/SKILL.md"
  done
fi

# Discover and register MCP servers from environment variables
# MCP_SERVER_<name>=<url> entries are set by the controller
LC_ALL=C
export LC_ALL
c1_control_pattern=$(printf '\302[\200-\237]')
env | grep '^MCP_SERVER_' | while IFS='=' read -r varname _; do
  name="${varname#MCP_SERVER_}"
  # Read the complete value again because env renders embedded newlines as
  # separate records. Reject all control characters before processing it.
  # Preserve trailing newlines: command substitution strips them, while
  # printenv adds one record terminator that must be removed explicitly.
  url_with_terminator=$(printenv "$varname"; printf x)
  url_with_terminator=${url_with_terminator%x}
  url=${url_with_terminator%?}
  case "$url" in
    *[[:cntrl:]]*|*$c1_control_pattern*)
      echo "invalid MCP server URL in ${varname}: control characters are not allowed" >&2
      exit 1
      ;;
  esac
  # YAML single-quoted scalars preserve the URL literally. A single quote is
  # represented by two adjacent single quotes inside the scalar.
  yaml_url=$(printf '%s' "$url" | sed "s/'/''/g")
  # Convert to lowercase for the extension key
  name=$(echo "$name" | tr '[:upper:]' '[:lower:]')
  cat >> $HOME/.config/goose/config.yaml <<MCPEOF
  ${name}:
    type: streamable_http
    name: ${name}
    uri: '${yaml_url}'
    description: "${name} MCP server"
    enabled: true
    timeout: 300
MCPEOF
done

# Kubernetes projects ConfigMap keys as symlinks, while Goose rejects
# symlinked recipe files. Copy recipes into the container filesystem so Goose
# can read them, then register each recipe as an explicit slash command.
if [ -d /tmp/recipes ]; then
  mkdir -p /tmp/goose-recipes
  recipe_commands=""
  slash_commands_written=0
  for recipe in /tmp/recipes/*; do
    [ -f "$recipe" ] || continue
    basename=$(basename "$recipe")
    case "$basename" in
      *.yaml|*.json) ;;
      *) continue ;;
    esac

    command="${basename%.*}"
    if [ -z "$command" ]; then
      echo "invalid Goose recipe filename: ${basename}" >&2
      exit 1
    fi
    case " ${recipe_commands} " in
      *" ${command} "*)
        echo "duplicate Goose recipe command: ${command}" >&2
        exit 1
        ;;
    esac
    recipe_commands="${recipe_commands} ${command}"

    installed_recipe="/tmp/goose-recipes/${basename}"
    cp "$recipe" "$installed_recipe"
    if [ "$slash_commands_written" -eq 0 ]; then
      printf '\nslash_commands:\n' >> $HOME/.config/goose/config.yaml
      slash_commands_written=1
    fi
    cat >> $HOME/.config/goose/config.yaml <<RECIPEEOF
  - command: '${command}'
    recipe_path: '${installed_recipe}'
RECIPEEOF
  done
fi

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
  # Start from the environment CA bundle if SSL_CERT_FILE is already set,
  # otherwise fall back to the default system CA bundle.
  BASE_CA="${SSL_CERT_FILE:-/etc/pki/tls/certs/ca-bundle.crt}"
  if [ -f "$BASE_CA" ]; then
    cat "$BASE_CA" "$SERVICE_CA" > "$MERGED_CA"
    export SSL_CERT_FILE="$MERGED_CA"
  fi
fi

# Set the API key in the current process environment so it propagates
# to the sleep process and is visible to oc exec/rsh sessions.
export LIGHTSPEED_API_KEY="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token)"

# Write env snippets so oc rsh / exec sessions pick up the API key,
# SSL trust, and any other assistant-specific env vars.
# The container spec sets ENV and BASH_ENV to this path so every
# shell (sh and bash, interactive or not) sources it automatically.
GOOSE_ENV='export LIGHTSPEED_API_KEY="$(cat /var/run/secrets/kubernetes.io/serviceaccount/token 2>/dev/null)"'
if [ -n "${SSL_CERT_FILE:-}" ]; then
  GOOSE_ENV="${GOOSE_ENV}
export SSL_CERT_FILE=\"${SSL_CERT_FILE}\""
fi
echo "$GOOSE_ENV" > /tmp/assistant-env.sh
echo "$GOOSE_ENV" >> "$HOME/.bashrc"
echo "$GOOSE_ENV" >> "$HOME/.profile"

exec sleep infinity
`
}

// AssistantPodSpec returns the PodSpec for the assistant pod.
// resolvedMCPServers maps extension name to URL for all MCP servers
// (both manually specified and auto-resolved from OpenStackClientRef).
func AssistantPodSpec(
	instance *assistantv1.OpenStackAssistant,
	configHash string,
	resolvedMCPServers map[string]string,
) corev1.PodSpec {
	envVars := map[string]env.Setter{}
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["GOOSE_PROVIDER"] = env.SetValue("lightspeed")
	envVars["GOOSE_TELEMETRY_ENABLED"] = env.SetValue("false")
	envVars["GOOSE_DISABLE_KEYRING"] = env.SetValue("1")
	envVars["ENV"] = env.SetValue("/tmp/assistant-env.sh")
	envVars["BASH_ENV"] = env.SetValue("/tmp/assistant-env.sh")
	if instance.Spec.Goose != nil && instance.Spec.Goose.Recipes != nil {
		// The entrypoint copies projected ConfigMap recipe symlinks into this
		// writable directory as regular files. Keep this as a pod environment
		// variable so it is also present when users exec goose directly.
		envVars["GOOSE_RECIPE_PATH"] = env.SetValue("/tmp/goose-recipes")
	}

	if instance.Spec.CaBundleSecretName != "" {
		envVars["SSL_CERT_FILE"] = env.SetValue(tls.DownstreamTLSCABundlePath)
	}

	if instance.Spec.Goose != nil && instance.Spec.Goose.Model != "" {
		envVars["GOOSE_MODEL"] = env.SetValue(instance.Spec.Goose.Model)
	}

	for name, url := range resolvedMCPServers {
		envVars["MCP_SERVER_"+name] = env.SetValue(url)
	}

	containerEnvs := env.MergeEnvs([]corev1.EnvVar{}, envVars)
	for idx := range instance.Spec.Env {
		e := instance.Spec.Env[idx]
		containerEnvs = env.MergeEnvs(containerEnvs, env.SetterMap{
			e.Name: func(env *corev1.EnvVar) {
				env.Value = e.Value
				env.ValueFrom = e.ValueFrom
			},
		})
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
				Env:          containerEnvs,
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
		if instance.Spec.Goose.Skills != nil {
			mounts = append(mounts, corev1.VolumeMount{
				Name:      "skills",
				MountPath: "/tmp/skills",
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

	mounts = append(mounts, instance.Spec.CreateVolumeMounts(nil)...)

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
		if instance.Spec.Goose.Skills != nil {
			volumes = append(volumes, corev1.Volume{
				Name: "skills",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: *instance.Spec.Goose.Skills,
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

	if instance.Spec.CaBundleSecretName != "" {
		volumes = append(volumes, instance.Spec.CreateVolume())
	}

	return volumes
}
