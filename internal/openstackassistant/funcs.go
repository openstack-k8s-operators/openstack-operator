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
	"encoding/json"
	"fmt"
	"strings"
	"unicode"

	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// workingDirVolumeName is the name of the writable working-directory volume.
const workingDirVolumeName = "working-dir"

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
// environment variable to the assistant container.
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

// AssistantPodSpec returns the PodSpec for the assistant pod.
// resolvedMCPServers maps MCP server name to URL for all MCP servers
// (both manually specified and auto-resolved from OpenStackClientRef).
//
// The operator is agent-agnostic: it injects a generic set of environment
// variables and mounts the CA bundle plus any ExtraConfig ConfigMaps. The
// image's own entrypoint is responsible for rendering the harness-specific
// configuration from these inputs (and for deriving the LLM API key from the
// pod service-account token).
func AssistantPodSpec(
	instance *assistantv1.OpenStackAssistant,
	configHash string,
	resolvedMCPServers map[string]string,
) corev1.PodSpec {
	return AssistantPodSpecWithCABundle(
		instance,
		configHash,
		resolvedMCPServers,
		instance.Spec.CaBundleSecretName,
	)
}

// AssistantPodSpecWithCABundle returns the assistant PodSpec using the
// effective CA bundle Secret reconciled by the controller. Keeping the source
// Secret from the API separate from the mounted Secret lets the controller add
// cluster-provided trust roots without modifying user-owned data.
func AssistantPodSpecWithCABundle(
	instance *assistantv1.OpenStackAssistant,
	configHash string,
	resolvedMCPServers map[string]string,
	caBundleSecretName string,
) corev1.PodSpec {
	envVars := map[string]env.Setter{}
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["LIGHTSPEED_URL"] = env.SetValue(instance.Spec.LightspeedStack.BaseURL)
	envVars["LIGHTSPEED_MODEL"] = env.SetValue(instance.Spec.LightspeedStack.Model)
	envVars["HOME"] = env.SetValue(instance.WorkingDir())

	// Additional models are passed as a JSON array for the image entrypoint to
	// render into the harness config.
	if len(instance.Spec.LightspeedStack.AdditionalModels) > 0 {
		if encoded, err := json.Marshal(instance.Spec.LightspeedStack.AdditionalModels); err == nil {
			envVars["LIGHTSPEED_ADDITIONAL_MODELS"] = env.SetValue(string(encoded))
		}
	}

	if caBundleSecretName != "" {
		envVars["SSL_CERT_FILE"] = env.SetValue(tls.DownstreamTLSCABundlePath)
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

	volumes := assistantPodVolumes(instance, caBundleSecretName)
	volumeMounts := assistantPodVolumeMounts(instance, caBundleSecretName)

	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: ptr.To[int64](0),
		ServiceAccountName:            instance.RbacResourceName(),
		Volumes:                       volumes,
		Containers: []corev1.Container{
			{
				Name:  "assistant",
				Image: instance.Spec.ContainerImage,
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
				Resources:    instance.Spec.Resources,
			},
		},
	}

	if instance.Spec.NodeSelector != nil {
		podSpec.NodeSelector = *instance.Spec.NodeSelector
	}

	return podSpec
}

// extraConfigVolumeName returns a deterministic, RFC1123-valid volume name for
// the ExtraConfig entry at the given index. The index guarantees uniqueness
// regardless of the ConfigMap name characters.
func extraConfigVolumeName(idx int) string {
	return fmt.Sprintf("extra-config-%d", idx)
}

func assistantPodVolumeMounts(
	instance *assistantv1.OpenStackAssistant,
	caBundleSecretName string,
) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{}

	// Writable working directory (also HOME) for the agent's configuration,
	// sessions, and runtime state, since all other mounts are read-only.
	mounts = append(mounts, corev1.VolumeMount{
		Name:      workingDirVolumeName,
		MountPath: instance.WorkingDir(),
	})

	for idx, cfg := range instance.Spec.ExtraConfig {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      extraConfigVolumeName(idx),
			MountPath: cfg.MountPath,
			ReadOnly:  true,
		})
	}

	ca := tls.Ca{CaBundleSecretName: caBundleSecretName}
	mounts = append(mounts, ca.CreateVolumeMounts(nil)...)

	return mounts
}

func assistantPodVolumes(
	instance *assistantv1.OpenStackAssistant,
	caBundleSecretName string,
) []corev1.Volume {
	volumes := []corev1.Volume{}

	// Writable working directory: an existing PVC when requested, otherwise an
	// ephemeral emptyDir.
	workingDirVolume := corev1.Volume{Name: workingDirVolumeName}
	if instance.Spec.Storage != nil && instance.Spec.Storage.PVCName != "" {
		workingDirVolume.VolumeSource = corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: instance.Spec.Storage.PVCName,
			},
		}
	} else {
		workingDirVolume.VolumeSource = corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	}
	volumes = append(volumes, workingDirVolume)

	for idx, cfg := range instance.Spec.ExtraConfig {
		volumes = append(volumes, corev1.Volume{
			Name: extraConfigVolumeName(idx),
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.Name,
					},
				},
			},
		})
	}

	if caBundleSecretName != "" {
		ca := tls.Ca{CaBundleSecretName: caBundleSecretName}
		volumes = append(volumes, ca.CreateVolume())
	}

	return volumes
}
