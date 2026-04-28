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

package openstackassistant

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"

	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newTestInstance() *assistantv1.OpenStackAssistant {
	return &assistantv1.OpenStackAssistant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-assistant",
			Namespace: "openstack",
		},
		Spec: assistantv1.OpenStackAssistantSpec{
			ContainerImage: "quay.io/openstack-k8s-operators/openstack-goose:current-podified",
			LightspeedStack: assistantv1.LightspeedStackSpec{
				BaseURL: "https://lightspeed-stack.openstack.svc:8443",
				Model:   "default",
			},
		},
	}
}

func envMapOf(spec corev1.PodSpec) map[string]string {
	envMap := make(map[string]string)
	for _, e := range spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	return envMap
}

func TestValidateMCPServerRefs(t *testing.T) {
	g := gomega.NewWithT(t)

	g.Expect(ValidateMCPServerRefs([]assistantv1.MCPServerRef{
		{Name: "Foo"},
		{Name: "foo"},
	})).To(gomega.MatchError(`MCP server name "foo" conflicts with "Foo" when compared case-insensitively`))

	g.Expect(ValidateMCPServerRefs([]assistantv1.MCPServerRef{
		{Name: "Foo"},
		{Name: "bar"},
	})).To(gomega.Succeed())

	g.Expect(ValidateMCPServerRefs([]assistantv1.MCPServerRef{
		{Name: "unsafe", URL: "https://mcp.example.test/\nother"},
	})).To(gomega.MatchError(`MCP server "unsafe" URL contains control character U+000A`))
}

func TestValidateMCPServerURL(t *testing.T) {
	g := gomega.NewWithT(t)

	g.Expect(ValidateMCPServerURL("safe", "https://mcp.example.test/a#fragment")).To(gomega.Succeed())
	g.Expect(ValidateMCPServerURL("unsafe", "https://mcp.example.test/\nother: value")).To(
		gomega.MatchError(`MCP server "unsafe" URL contains control character U+000A`))
	g.Expect(ValidateMCPServerURL("unsafe", "https://mcp.example.test/\u0085other")).To(
		gomega.MatchError(`MCP server "unsafe" URL contains control character U+0085`))
}

func TestValidateCustomMCPServerURLs(t *testing.T) {
	g := gomega.NewWithT(t)

	g.Expect(ValidateCustomMCPServerURLs([]corev1.EnvVar{
		{Name: "OTHER", Value: "line one\nline two"},
		{Name: "MCP_SERVER_safe", Value: "https://mcp.example.test/path"},
		{Name: "MCP_SERVER_from", ValueFrom: &corev1.EnvVarSource{}},
	})).To(gomega.Succeed())
	g.Expect(ValidateCustomMCPServerURLs([]corev1.EnvVar{
		{Name: "MCP_SERVER_unsafe", Value: "https://mcp.example.test/\tother"},
	})).To(gomega.MatchError(`MCP server "MCP_SERVER_unsafe" URL contains control character U+0009`))
}

func TestAssistantPodSpec_BasicFields(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "testhash123", nil)

	g.Expect(spec.ServiceAccountName).To(gomega.Equal("openstackassistant-test-assistant"))
	g.Expect(*spec.TerminationGracePeriodSeconds).To(gomega.Equal(int64(0)))
	g.Expect(spec.Containers).To(gomega.HaveLen(1))

	container := spec.Containers[0]
	g.Expect(container.Name).To(gomega.Equal("assistant"))
	g.Expect(container.Image).To(gomega.Equal("quay.io/openstack-k8s-operators/openstack-goose:current-podified"))
	// The operator is agent-agnostic: it must not override the image entrypoint.
	g.Expect(container.Command).To(gomega.BeEmpty())
	g.Expect(container.Args).To(gomega.BeEmpty())
}

func TestAssistantPodSpec_SecurityContext(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil)
	sc := spec.Containers[0].SecurityContext

	g.Expect(*sc.RunAsNonRoot).To(gomega.BeTrue())
	g.Expect(*sc.AllowPrivilegeEscalation).To(gomega.BeFalse())
	g.Expect(sc.Capabilities.Drop).To(gomega.ContainElement(corev1.Capability("ALL")))
}

func TestAssistantPodSpec_GenericEnvVars(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "somehash", nil)
	envMap := envMapOf(spec)

	g.Expect(envMap).To(gomega.HaveKeyWithValue("CONFIG_HASH", "somehash"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("LIGHTSPEED_URL", "https://lightspeed-stack.openstack.svc:8443"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("LIGHTSPEED_MODEL", "default"))
	// No harness-specific (GOOSE_*) env vars and no operator-set API key.
	for name := range envMap {
		g.Expect(name).NotTo(gomega.HavePrefix("GOOSE_"))
	}
	g.Expect(envMap).NotTo(gomega.HaveKey("LIGHTSPEED_API_KEY"))
}

func TestAssistantPodSpec_CustomEnvVars(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Env = []corev1.EnvVar{
		{Name: "MY_CUSTOM_VAR", Value: "myvalue"},
		{Name: "LIGHTSPEED_MODEL", Value: "override-model"},
	}

	spec := AssistantPodSpec(instance, "hash", nil)
	envMap := envMapOf(spec)

	g.Expect(envMap).To(gomega.HaveKeyWithValue("MY_CUSTOM_VAR", "myvalue"))
	// User-supplied Env overrides operator defaults.
	g.Expect(envMap).To(gomega.HaveKeyWithValue("LIGHTSPEED_MODEL", "override-model"))
}

func TestAssistantPodSpec_CustomEnvVarsPreserveDeclarationOrder(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Env = []corev1.EnvVar{
		{Name: "CUSTOM_BASE", Value: "base"},
		{Name: "CUSTOM_DEPENDENT", Value: "$(CUSTOM_BASE)/dependent"},
	}

	spec := AssistantPodSpec(instance, "hash", nil)
	customEnvIndexes := make(map[string]int)
	for i, e := range spec.Containers[0].Env {
		if e.Name == "CUSTOM_BASE" || e.Name == "CUSTOM_DEPENDENT" {
			customEnvIndexes[e.Name] = i
		}
	}

	g.Expect(customEnvIndexes["CUSTOM_BASE"]).To(gomega.BeNumerically("<", customEnvIndexes["CUSTOM_DEPENDENT"]))
}

func TestAssistantPodSpec_MinimalVolumes(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil)

	// No ExtraConfig and no CA bundle -> only the writable working-dir volume.
	g.Expect(spec.Volumes).To(gomega.HaveLen(1))
	g.Expect(spec.Volumes[0].Name).To(gomega.Equal(workingDirVolumeName))
	g.Expect(spec.Volumes[0].EmptyDir).NotTo(gomega.BeNil())

	g.Expect(spec.Containers[0].VolumeMounts).To(gomega.HaveLen(1))
	g.Expect(spec.Containers[0].VolumeMounts[0].Name).To(gomega.Equal(workingDirVolumeName))
	g.Expect(spec.Containers[0].VolumeMounts[0].MountPath).To(gomega.Equal(instance.WorkingDir()))
	g.Expect(spec.Containers[0].VolumeMounts[0].ReadOnly).To(gomega.BeFalse())
}

func TestAssistantPodSpec_WorkingDirPVC(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Storage = &assistantv1.StorageSpec{
		MountPath: "/workspace",
		PVCName:   "assistant-workspace-pvc",
	}

	spec := AssistantPodSpec(instance, "hash", nil)

	var workingDir *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == workingDirVolumeName {
			workingDir = &spec.Volumes[i]
			break
		}
	}
	g.Expect(workingDir).NotTo(gomega.BeNil())
	g.Expect(workingDir.EmptyDir).To(gomega.BeNil())
	g.Expect(workingDir.PersistentVolumeClaim).NotTo(gomega.BeNil())
	g.Expect(workingDir.PersistentVolumeClaim.ClaimName).To(gomega.Equal("assistant-workspace-pvc"))
	g.Expect(spec.Containers[0].VolumeMounts[0].MountPath).To(gomega.Equal("/workspace"))
	g.Expect(envMapOf(spec)).To(gomega.HaveKeyWithValue("HOME", "/workspace"))
}

func TestAssistantPodSpec_AdditionalModels(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.LightspeedStack.AdditionalModels = []assistantv1.ModelRef{
		{Name: "gemini/models/gemini-2.5-pro"},
	}

	spec := AssistantPodSpec(instance, "hash", nil)

	env := envMapOf(spec)
	g.Expect(env).To(gomega.HaveKey("LIGHTSPEED_ADDITIONAL_MODELS"))
	g.Expect(env["LIGHTSPEED_ADDITIONAL_MODELS"]).To(gomega.ContainSubstring("gemini/models/gemini-2.5-pro"))
}

func TestAssistantPodSpec_WithExtraConfig(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.ExtraConfig = []assistantv1.ConfigMount{
		{Name: "recipes-cm", MountPath: "/etc/openstack-goose/recipes"},
		{Name: "skills-cm", MountPath: "/etc/openstack-goose/skills"},
	}

	spec := AssistantPodSpec(instance, "hash", nil)

	// Working-dir volume is always present and prepended, followed by ExtraConfig.
	g.Expect(spec.Volumes).To(gomega.HaveLen(3))
	g.Expect(spec.Volumes[0].Name).To(gomega.Equal(workingDirVolumeName))
	g.Expect(spec.Volumes[1].Name).To(gomega.Equal("extra-config-0"))
	g.Expect(spec.Volumes[1].ConfigMap.Name).To(gomega.Equal("recipes-cm"))
	g.Expect(spec.Volumes[2].Name).To(gomega.Equal("extra-config-1"))
	g.Expect(spec.Volumes[2].ConfigMap.Name).To(gomega.Equal("skills-cm"))

	mounts := spec.Containers[0].VolumeMounts
	g.Expect(mounts).To(gomega.HaveLen(3))
	g.Expect(mounts[0].Name).To(gomega.Equal(workingDirVolumeName))
	g.Expect(mounts[1].Name).To(gomega.Equal("extra-config-0"))
	g.Expect(mounts[1].MountPath).To(gomega.Equal("/etc/openstack-goose/recipes"))
	g.Expect(mounts[1].ReadOnly).To(gomega.BeTrue())
	g.Expect(mounts[2].Name).To(gomega.Equal("extra-config-1"))
	g.Expect(mounts[2].MountPath).To(gomega.Equal("/etc/openstack-goose/skills"))
	g.Expect(mounts[2].ReadOnly).To(gomega.BeTrue())
}

func TestAssistantPodSpec_WithCaBundle(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.CaBundleSecretName = "lightspeed-ca-bundle"

	spec := AssistantPodSpec(instance, "hash", nil)

	var caBundleVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == tls.CABundleLabel {
			caBundleVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(caBundleVolume).NotTo(gomega.BeNil())
	g.Expect(caBundleVolume.Secret.SecretName).To(gomega.Equal("lightspeed-ca-bundle"))

	var caBundleMount *corev1.VolumeMount
	for i := range spec.Containers[0].VolumeMounts {
		if spec.Containers[0].VolumeMounts[i].Name == tls.CABundleLabel {
			caBundleMount = &spec.Containers[0].VolumeMounts[i]
			break
		}
	}
	g.Expect(caBundleMount).NotTo(gomega.BeNil())
	g.Expect(caBundleMount.MountPath).To(gomega.Equal(tls.DownstreamTLSCABundlePath))
	g.Expect(caBundleMount.SubPath).To(gomega.Equal(tls.CABundleKey))
	g.Expect(caBundleMount.ReadOnly).To(gomega.BeTrue())

	g.Expect(envMapOf(spec)).To(gomega.HaveKeyWithValue("SSL_CERT_FILE", tls.DownstreamTLSCABundlePath))
}

func TestAssistantPodSpecWithCABundle_UsesEffectiveSecret(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.CaBundleSecretName = "source-ca-bundle"

	spec := AssistantPodSpecWithCABundle(instance, "hash", nil, "generated-ca-bundle")

	var caBundleVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == tls.CABundleLabel {
			caBundleVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(caBundleVolume).NotTo(gomega.BeNil())
	g.Expect(caBundleVolume.Secret.SecretName).To(gomega.Equal("generated-ca-bundle"))
	g.Expect(instance.Spec.CaBundleSecretName).To(gomega.Equal("source-ca-bundle"))
	g.Expect(envMapOf(spec)).To(gomega.HaveKeyWithValue("SSL_CERT_FILE", tls.DownstreamTLSCABundlePath))
}

func TestAssistantPodSpec_AllVolumeMountsReadOnly(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.ExtraConfig = []assistantv1.ConfigMount{
		{Name: "recipes-cm", MountPath: "/etc/openstack-goose/recipes"},
		{Name: "skills-cm", MountPath: "/etc/openstack-goose/skills"},
	}
	instance.Spec.CaBundleSecretName = "ca-secret"

	spec := AssistantPodSpec(instance, "hash", nil)

	g.Expect(spec.Containers[0].VolumeMounts).NotTo(gomega.BeEmpty())
	for _, mount := range spec.Containers[0].VolumeMounts {
		// The working directory is intentionally writable; everything else
		// (config/recipes/skills/CA bundle) must be read-only.
		if mount.Name == workingDirVolumeName {
			g.Expect(mount.ReadOnly).To(gomega.BeFalse(), "working dir must be writable")
			continue
		}
		g.Expect(mount.ReadOnly).To(gomega.BeTrue(), "VolumeMount %s should be read-only", mount.Name)
	}
}

func TestAssistantPodSpec_WithNodeSelector(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.NodeSelector = &map[string]string{
		"node-role.kubernetes.io/worker": "",
	}

	spec := AssistantPodSpec(instance, "hash", nil)

	g.Expect(spec.NodeSelector).To(gomega.HaveKeyWithValue("node-role.kubernetes.io/worker", ""))
}

func TestAssistantPodSpec_WithoutNodeSelector(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil)

	g.Expect(spec.NodeSelector).To(gomega.BeNil())
}

func TestAssistantPodSpec_MCPServers(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	resolvedMCPServers := map[string]string{
		"openstack": "http://openstackclient-mcp.openstack.svc:8080/openstack/",
	}

	spec := AssistantPodSpec(instance, "hash", resolvedMCPServers)

	g.Expect(envMapOf(spec)).To(gomega.HaveKeyWithValue(
		"MCP_SERVER_openstack", "http://openstackclient-mcp.openstack.svc:8080/openstack/"))
}

func TestAssistantPodSpec_MCPServersHTTPS(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	resolvedMCPServers := map[string]string{
		"openstack": "https://openstackclient-mcp.openstack.svc:8080/openstack/",
	}

	spec := AssistantPodSpec(instance, "hash", resolvedMCPServers)

	g.Expect(envMapOf(spec)).To(gomega.HaveKeyWithValue(
		"MCP_SERVER_openstack", "https://openstackclient-mcp.openstack.svc:8080/openstack/"))
}
