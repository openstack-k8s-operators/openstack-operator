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
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/onsi/gomega"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"gopkg.in/yaml.v3"

	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func newTestInstance() *assistantv1.OpenStackAssistant {
	return &assistantv1.OpenStackAssistant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-assistant",
			Namespace: "openstack",
		},
		Spec: assistantv1.OpenStackAssistantSpec{
			ContainerImage: "quay.io/openstack-s2i-containers/openstack-goose:master-latest",
			Provider:       assistantv1.ProviderGoose,
			LightspeedStack: assistantv1.LightspeedStackSpec{
				ProviderSecret: "lightspeed-provider-config",
			},
		},
	}
}

func TestEntrypointScript(t *testing.T) {
	g := gomega.NewWithT(t)

	script := EntrypointScript()

	g.Expect(script).To(gomega.ContainSubstring("#!/bin/sh"))
	g.Expect(script).To(gomega.ContainSubstring("mkdir -p $HOME/.config/goose/custom_providers"))
	g.Expect(script).To(gomega.ContainSubstring("config.yaml"))
	g.Expect(script).To(gomega.ContainSubstring("developer:"))
	g.Expect(script).To(gomega.ContainSubstring("skills:"))
	g.Expect(script).To(gomega.ContainSubstring("enabled: true"))
	g.Expect(script).NotTo(gomega.ContainSubstring("recipe_source:"))
	g.Expect(script).To(gomega.ContainSubstring("slash_commands:"))
	g.Expect(script).To(gomega.ContainSubstring("/tmp/goose-recipes"))
	g.Expect(script).To(gomega.ContainSubstring("/tmp/hints/hints"))
	g.Expect(script).To(gomega.ContainSubstring("/tmp/lightspeed-provider/lightspeed.json"))
	g.Expect(script).To(gomega.ContainSubstring("sleep infinity"))

	g.Expect(script).To(gomega.ContainSubstring("ca-bundle.crt"))
	g.Expect(script).To(gomega.ContainSubstring("service-ca.crt"))
	g.Expect(script).NotTo(gomega.ContainSubstring("update-ca-trust"))
	g.Expect(script).To(gomega.ContainSubstring(`export LIGHTSPEED_API_KEY="`))
	g.Expect(script).To(gomega.ContainSubstring(`export SSL_CERT_FILE="`))
	g.Expect(script).NotTo(gomega.ContainSubstring("/etc/profile.d/goose.sh"))
	g.Expect(script).To(gomega.ContainSubstring("/tmp/assistant-env.sh"))
	g.Expect(script).To(gomega.ContainSubstring(".bashrc"))
	g.Expect(script).To(gomega.ContainSubstring(".profile"))
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
	g.Expect(container.Name).To(gomega.Equal("goose"))
	g.Expect(container.Image).To(gomega.Equal("quay.io/openstack-s2i-containers/openstack-goose:master-latest"))
	g.Expect(container.Command).To(gomega.Equal([]string{"/bin/sh"}))
	g.Expect(container.Args).To(gomega.Equal([]string{"/tmp/entrypoint/entrypoint.sh"}))
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

func TestAssistantPodSpec_DefaultEnvVars(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "somehash", nil)
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(gomega.HaveKeyWithValue("CONFIG_HASH", "somehash"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_PROVIDER", "lightspeed"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_TELEMETRY_ENABLED", "false"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_DISABLE_KEYRING", "1"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("ENV", "/tmp/assistant-env.sh"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("BASH_ENV", "/tmp/assistant-env.sh"))
}

func TestAssistantPodSpec_GooseModel(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Model: "gemini/models/gemini-2.5-flash",
	}

	spec := AssistantPodSpec(instance, "hash", nil)
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_MODEL", "gemini/models/gemini-2.5-flash"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_PROVIDER", "lightspeed"))
}

func TestAssistantPodSpec_CustomEnvVars(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Env = []corev1.EnvVar{
		{Name: "MY_CUSTOM_VAR", Value: "myvalue"},
		{Name: "GOOSE_PROVIDER", Value: "custom-provider"},
	}

	spec := AssistantPodSpec(instance, "hash", nil)
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(gomega.HaveKeyWithValue("MY_CUSTOM_VAR", "myvalue"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_PROVIDER", "custom-provider"))
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

	g.Expect(spec.Volumes).To(gomega.HaveLen(2))

	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(gomega.ContainElements("entrypoint", "lightspeed-provider"))

	mountNames := make([]string, len(spec.Containers[0].VolumeMounts))
	for i, m := range spec.Containers[0].VolumeMounts {
		mountNames[i] = m.Name
	}
	g.Expect(mountNames).To(gomega.ContainElements("entrypoint", "lightspeed-provider"))
}

func TestAssistantPodSpec_WithRecipesAndHints(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Recipes: ptr.To("assistant-recipes"),
		Hints:   ptr.To("assistant-hints"),
	}

	spec := AssistantPodSpec(instance, "hash", nil)

	g.Expect(spec.Volumes).To(gomega.HaveLen(4))
	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(gomega.ContainElements("entrypoint", "lightspeed-provider", "recipes", "hints"))

	mountNames := make([]string, len(spec.Containers[0].VolumeMounts))
	for i, m := range spec.Containers[0].VolumeMounts {
		mountNames[i] = m.Name
	}
	g.Expect(mountNames).To(gomega.ContainElements("entrypoint", "lightspeed-provider", "recipes", "hints"))
}

func TestAssistantPodSpec_WithCaBundle(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.CaBundleSecretName = "lightspeed-ca-bundle"

	spec := AssistantPodSpec(instance, "hash", nil)

	g.Expect(spec.Volumes).To(gomega.HaveLen(3))

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

	envMap := map[string]string{}
	for _, e := range spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	g.Expect(envMap).To(gomega.HaveKeyWithValue("SSL_CERT_FILE", tls.DownstreamTLSCABundlePath))
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

func TestAssistantPodSpec_EntrypointConfigMapName(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil)

	var entrypointVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "entrypoint" {
			entrypointVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(entrypointVolume).NotTo(gomega.BeNil())
	g.Expect(entrypointVolume.ConfigMap.Name).To(gomega.Equal("test-assistant-entrypoint"))
	g.Expect(*entrypointVolume.ConfigMap.DefaultMode).To(gomega.Equal(int32(0755)))
}

func TestAssistantPodSpec_LightspeedProviderSecretName(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil)

	var providerVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "lightspeed-provider" {
			providerVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(providerVolume).NotTo(gomega.BeNil())
	g.Expect(providerVolume.Secret.SecretName).To(gomega.Equal("lightspeed-provider-config"))
}

func TestAssistantPodSpec_AllVolumeMountsReadOnly(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Recipes: ptr.To("recipes-cm"),
		Skills:  ptr.To("skills-cm"),
		Hints:   ptr.To("hints-cm"),
	}
	instance.Spec.CaBundleSecretName = "ca-secret"

	spec := AssistantPodSpec(instance, "hash", nil)

	for _, mount := range spec.Containers[0].VolumeMounts {
		g.Expect(mount.ReadOnly).To(gomega.BeTrue(), "VolumeMount %s should be read-only", mount.Name)
	}
}

func TestAssistantPodSpec_RecipesOnlyNoHints(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Recipes: ptr.To("recipes-cm"),
	}

	spec := AssistantPodSpec(instance, "hash", nil)

	g.Expect(spec.Volumes).To(gomega.HaveLen(3))
	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(gomega.ContainElement("recipes"))
	g.Expect(volumeNames).NotTo(gomega.ContainElement("hints"))

	envMap := map[string]string{}
	for _, e := range spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_RECIPE_PATH", "/tmp/goose-recipes"))
}

func TestAssistantPodSpec_SkillsOnly(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Skills: ptr.To("skills-cm"),
	}

	spec := AssistantPodSpec(instance, "hash", nil)

	g.Expect(spec.Volumes).To(gomega.HaveLen(3))
	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(gomega.ContainElement("skills"))

	var skillsVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "skills" {
			skillsVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(skillsVolume).NotTo(gomega.BeNil())
	g.Expect(skillsVolume.ConfigMap.Name).To(gomega.Equal("skills-cm"))

	mountNames := make([]string, len(spec.Containers[0].VolumeMounts))
	for i, m := range spec.Containers[0].VolumeMounts {
		mountNames[i] = m.Name
	}
	g.Expect(mountNames).To(gomega.ContainElement("skills"))
}

func TestAssistantPodSpec_MCPServers(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	resolvedMCPServers := map[string]string{
		"openstack": "http://openstackclient-mcp.openstack.svc:8080/openstack/",
	}

	spec := AssistantPodSpec(instance, "hash", resolvedMCPServers)
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(gomega.HaveKeyWithValue("MCP_SERVER_openstack", "http://openstackclient-mcp.openstack.svc:8080/openstack/"))
}

func TestAssistantPodSpec_MCPServersHTTPS(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	resolvedMCPServers := map[string]string{
		"openstack": "https://openstackclient-mcp.openstack.svc:8080/openstack/",
	}

	spec := AssistantPodSpec(instance, "hash", resolvedMCPServers)
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(gomega.HaveKeyWithValue("MCP_SERVER_openstack", "https://openstackclient-mcp.openstack.svc:8080/openstack/"))
}

func TestEntrypointScript_MCPServerDiscovery(t *testing.T) {
	g := gomega.NewWithT(t)

	script := EntrypointScript()

	g.Expect(script).To(gomega.ContainSubstring("MCP_SERVER_"))
	g.Expect(script).To(gomega.ContainSubstring("streamable_http"))
	g.Expect(script).To(gomega.ContainSubstring(`*[[:cntrl:]]*`))
	g.Expect(script).To(gomega.ContainSubstring(`uri: '${yaml_url}'`))
}

func TestEntrypointScript_InstallsSkills(t *testing.T) {
	g := gomega.NewWithT(t)
	home := t.TempDir()
	skillsMount := t.TempDir()
	skillContent := "---\nname: cluster-health\ndescription: Diagnose cluster health\n---\n\n# Cluster Health\n"
	projectedSkill := t.TempDir() + "/cluster-health.md"
	g.Expect(os.WriteFile(projectedSkill, []byte(skillContent), 0o600)).To(gomega.Succeed())
	g.Expect(os.Symlink(projectedSkill, skillsMount+"/cluster-health.md")).To(gomega.Succeed())

	script, _, found := strings.Cut(EntrypointScript(), "# Discover and register MCP servers")
	g.Expect(found).To(gomega.BeTrue())
	script = strings.ReplaceAll(script, "/tmp/skills", skillsMount)

	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH")}
	output, err := cmd.CombinedOutput()
	g.Expect(err).NotTo(gomega.HaveOccurred(), string(output))

	installedSkill := home + "/.config/goose/skills/cluster-health/SKILL.md"
	info, err := os.Lstat(installedSkill)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(info.Mode() & os.ModeSymlink).To(gomega.BeZero())
	content, err := os.ReadFile(installedSkill)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(string(content)).To(gomega.Equal(skillContent))

	config, err := os.ReadFile(home + "/.config/goose/config.yaml")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	parsed := struct {
		Extensions map[string]struct {
			Enabled bool   `yaml:"enabled"`
			Type    string `yaml:"type"`
		} `yaml:"extensions"`
	}{}
	g.Expect(yaml.Unmarshal(config, &parsed)).To(gomega.Succeed())
	g.Expect(parsed.Extensions).To(gomega.HaveKey("skills"))
	g.Expect(parsed.Extensions["skills"].Enabled).To(gomega.BeTrue())
	g.Expect(parsed.Extensions["skills"].Type).To(gomega.Equal("builtin"))
}

func TestEntrypointScript_MCPServerURLHandling(t *testing.T) {
	g := gomega.NewWithT(t)

	runMCPConfig := func(url string) ([]byte, string, error) {
		home := t.TempDir()
		script, _, found := strings.Cut(EntrypointScript(), "# Copy hints if present")
		g.Expect(found).To(gomega.BeTrue())

		cmd := exec.Command("/bin/sh", "-c", script)
		cmd.Env = []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "MCP_SERVER_Test=" + url}
		output, err := cmd.CombinedOutput()
		config, readErr := os.ReadFile(home + "/.config/goose/config.yaml")
		g.Expect(readErr).NotTo(gomega.HaveOccurred())
		return config, string(output), err
	}

	url := `https://mcp.example.test/a: b#c?quote='yes'&path=\server`
	config, _, err := runMCPConfig(url)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	parsed := struct {
		Extensions map[string]struct {
			URI string `yaml:"uri"`
		} `yaml:"extensions"`
	}{}
	g.Expect(yaml.Unmarshal(config, &parsed)).To(gomega.Succeed())
	g.Expect(parsed.Extensions).To(gomega.HaveKey("test"))
	g.Expect(parsed.Extensions["test"].URI).To(gomega.Equal(url))

	for _, url := range []string{
		"https://mcp.example.test/\nnext: value",
		"https://mcp.example.test/\n",
		"https://mcp.example.test/\tvalue",
		"https://mcp.example.test/\u0085value",
	} {
		config, output, err := runMCPConfig(url)
		g.Expect(err).To(gomega.HaveOccurred())
		g.Expect(output).To(gomega.ContainSubstring("control characters are not allowed"))
		g.Expect(string(config)).NotTo(gomega.ContainSubstring("streamable_http"))
	}
}

func TestEntrypointScript_InstallsAndRegistersRecipes(t *testing.T) {
	g := gomega.NewWithT(t)
	home := t.TempDir()
	recipeMount := t.TempDir()
	installedRecipes := t.TempDir() + "/recipes"
	recipeContent := "version: 1.0.0\ntitle: Cluster Health\ndescription: Check cluster health\nprompt: Check the cluster\n"
	projectedRecipe := t.TempDir() + "/cluster-health.yaml"
	g.Expect(os.WriteFile(projectedRecipe, []byte(recipeContent), 0o600)).To(gomega.Succeed())
	g.Expect(os.Symlink(projectedRecipe, recipeMount+"/cluster-health.yaml")).To(gomega.Succeed())
	g.Expect(os.WriteFile(recipeMount+"/notes.txt", []byte("not a recipe"), 0o600)).To(gomega.Succeed())

	script, _, found := strings.Cut(EntrypointScript(), "# Copy hints if present")
	g.Expect(found).To(gomega.BeTrue())
	script = strings.ReplaceAll(script, "/tmp/goose-recipes", installedRecipes)
	script = strings.ReplaceAll(script, "/tmp/recipes", recipeMount)

	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.Env = []string{
		"HOME=" + home,
		"PATH=" + os.Getenv("PATH"),
		"MCP_SERVER_Test=https://mcp.example.test/endpoint",
	}
	output, err := cmd.CombinedOutput()
	g.Expect(err).NotTo(gomega.HaveOccurred(), string(output))

	installedRecipe := installedRecipes + "/cluster-health.yaml"
	info, err := os.Lstat(installedRecipe)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(info.Mode() & os.ModeSymlink).To(gomega.BeZero())
	content, err := os.ReadFile(installedRecipe)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(string(content)).To(gomega.Equal(recipeContent))
	g.Expect(installedRecipes + "/notes.txt").NotTo(gomega.BeAnExistingFile())

	config, err := os.ReadFile(home + "/.config/goose/config.yaml")
	g.Expect(err).NotTo(gomega.HaveOccurred())
	parsed := struct {
		Extensions map[string]struct {
			URI string `yaml:"uri"`
		} `yaml:"extensions"`
		SlashCommands []struct {
			Command    string `yaml:"command"`
			RecipePath string `yaml:"recipe_path"`
		} `yaml:"slash_commands"`
	}{}
	g.Expect(yaml.Unmarshal(config, &parsed)).To(gomega.Succeed())
	g.Expect(parsed.Extensions["test"].URI).To(gomega.Equal("https://mcp.example.test/endpoint"))
	g.Expect(parsed.SlashCommands).To(gomega.ConsistOf(struct {
		Command    string `yaml:"command"`
		RecipePath string `yaml:"recipe_path"`
	}{
		Command:    "cluster-health",
		RecipePath: installedRecipe,
	}))
}

func TestEntrypointScript_DisabledExtensions(t *testing.T) {
	g := gomega.NewWithT(t)

	script := EntrypointScript()

	disabledExtensions := []string{"computercontroller", "apps", "analyze", "todo", "extensionmanager", "chatrecall"}
	for _, ext := range disabledExtensions {
		idx := strings.Index(script, ext+":")
		g.Expect(idx).To(gomega.BeNumerically(">", 0), "should contain %s", ext)
		g.Expect(script).To(gomega.ContainSubstring(ext))
	}

	enabledExtensions := []string{"developer", "summarize", "summon"}
	for _, ext := range enabledExtensions {
		g.Expect(script).To(gomega.ContainSubstring(ext))
	}
}
