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
	"strings"
	"testing"

	"github.com/onsi/gomega"

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
			ContainerImage: "quay.io/dprince/goose:oc-fedora",
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
	g.Expect(script).To(gomega.ContainSubstring("enabled: true"))
	g.Expect(script).To(gomega.ContainSubstring("/tmp/recipes/"))
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

func TestAssistantPodSpec_BasicFields(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "testhash123", nil, false)

	g.Expect(spec.ServiceAccountName).To(gomega.Equal("openstackassistant-test-assistant"))
	g.Expect(*spec.TerminationGracePeriodSeconds).To(gomega.Equal(int64(0)))
	g.Expect(spec.Containers).To(gomega.HaveLen(1))

	container := spec.Containers[0]
	g.Expect(container.Name).To(gomega.Equal("goose"))
	g.Expect(container.Image).To(gomega.Equal("quay.io/dprince/goose:oc-fedora"))
	g.Expect(container.Command).To(gomega.Equal([]string{"/bin/sh"}))
	g.Expect(container.Args).To(gomega.Equal([]string{"/tmp/entrypoint/entrypoint.sh"}))
}

func TestAssistantPodSpec_SecurityContext(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil, false)
	sc := spec.Containers[0].SecurityContext

	g.Expect(*sc.RunAsNonRoot).To(gomega.BeTrue())
	g.Expect(*sc.AllowPrivilegeEscalation).To(gomega.BeFalse())
	g.Expect(sc.Capabilities.Drop).To(gomega.ContainElement(corev1.Capability("ALL")))
}

func TestAssistantPodSpec_DefaultEnvVars(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "somehash", nil, false)
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

	spec := AssistantPodSpec(instance, "hash", nil, false)
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
	}

	spec := AssistantPodSpec(instance, "hash", nil, false)
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(gomega.HaveKeyWithValue("MY_CUSTOM_VAR", "myvalue"))
	g.Expect(envMap).To(gomega.HaveKeyWithValue("GOOSE_PROVIDER", "lightspeed"))
}

func TestAssistantPodSpec_MinimalVolumes(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil, false)

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

	spec := AssistantPodSpec(instance, "hash", nil, false)

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
	instance.Spec.LightspeedStack.CaBundleSecretName = "lightspeed-ca-bundle"

	spec := AssistantPodSpec(instance, "hash", nil, false)

	g.Expect(spec.Volumes).To(gomega.HaveLen(3))

	var caBundleVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "ca-bundle" {
			caBundleVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(caBundleVolume).NotTo(gomega.BeNil())
	g.Expect(caBundleVolume.Secret.SecretName).To(gomega.Equal("lightspeed-ca-bundle"))

	var caBundleMount *corev1.VolumeMount
	for i := range spec.Containers[0].VolumeMounts {
		if spec.Containers[0].VolumeMounts[i].Name == "ca-bundle" {
			caBundleMount = &spec.Containers[0].VolumeMounts[i]
			break
		}
	}
	g.Expect(caBundleMount).NotTo(gomega.BeNil())
	g.Expect(caBundleMount.MountPath).To(gomega.Equal("/etc/ssl/certs/ca-certificates.crt"))
	g.Expect(caBundleMount.SubPath).To(gomega.Equal("ca-bundle.crt"))
	g.Expect(caBundleMount.ReadOnly).To(gomega.BeTrue())

	envMap := map[string]string{}
	for _, e := range spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	g.Expect(envMap).To(gomega.HaveKeyWithValue("SSL_CERT_FILE", "/etc/ssl/certs/ca-certificates.crt"))
}

func TestAssistantPodSpec_WithNodeSelector(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.NodeSelector = &map[string]string{
		"node-role.kubernetes.io/worker": "",
	}

	spec := AssistantPodSpec(instance, "hash", nil, false)

	g.Expect(spec.NodeSelector).To(gomega.HaveKeyWithValue("node-role.kubernetes.io/worker", ""))
}

func TestAssistantPodSpec_WithoutNodeSelector(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil, false)

	g.Expect(spec.NodeSelector).To(gomega.BeNil())
}

func TestAssistantPodSpec_EntrypointConfigMapName(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash", nil, false)

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

	spec := AssistantPodSpec(instance, "hash", nil, false)

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
		Hints:   ptr.To("hints-cm"),
	}
	instance.Spec.LightspeedStack.CaBundleSecretName = "ca-secret"

	spec := AssistantPodSpec(instance, "hash", nil, false)

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

	spec := AssistantPodSpec(instance, "hash", nil, false)

	g.Expect(spec.Volumes).To(gomega.HaveLen(3))
	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(gomega.ContainElement("recipes"))
	g.Expect(volumeNames).NotTo(gomega.ContainElement("hints"))
}

func TestAssistantPodSpec_MCPServers(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()

	resolvedMCPServers := map[string]string{
		"openstack": "http://openstackclient-mcp.openstack.svc:8080/openstack/",
	}

	spec := AssistantPodSpec(instance, "hash", resolvedMCPServers, false)
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

	spec := AssistantPodSpec(instance, "hash", resolvedMCPServers, false)
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(gomega.HaveKeyWithValue("MCP_SERVER_openstack", "https://openstackclient-mcp.openstack.svc:8080/openstack/"))
}

func TestAssistantPodSpec_CombinedCA(t *testing.T) {
	g := gomega.NewWithT(t)
	instance := newTestInstance()
	instance.Spec.LightspeedStack.CaBundleSecretName = "lightspeed-ca"

	spec := AssistantPodSpec(instance, "hash", nil, true)

	// Combined CA should be mounted from the entrypoint ConfigMap
	var combinedMount *corev1.VolumeMount
	for i := range spec.Containers[0].VolumeMounts {
		if spec.Containers[0].VolumeMounts[i].SubPath == "combined-ca.crt" {
			combinedMount = &spec.Containers[0].VolumeMounts[i]
			break
		}
	}
	g.Expect(combinedMount).NotTo(gomega.BeNil())
	g.Expect(combinedMount.Name).To(gomega.Equal("entrypoint"))
	g.Expect(combinedMount.MountPath).To(gomega.Equal("/etc/ssl/certs/combined-ca.crt"))
	g.Expect(combinedMount.ReadOnly).To(gomega.BeTrue())

	// SSL_CERT_FILE should point to combined CA
	envMap := map[string]string{}
	for _, e := range spec.Containers[0].Env {
		envMap[e.Name] = e.Value
	}
	g.Expect(envMap).To(gomega.HaveKeyWithValue("SSL_CERT_FILE", "/etc/ssl/certs/combined-ca.crt"))

	// No separate ca-bundle volume when combined CA is used
	for _, v := range spec.Volumes {
		g.Expect(v.Name).NotTo(gomega.Equal("ca-bundle"))
	}
}

func TestEntrypointScript_MCPServerDiscovery(t *testing.T) {
	g := gomega.NewWithT(t)

	script := EntrypointScript()

	g.Expect(script).To(gomega.ContainSubstring("MCP_SERVER_"))
	g.Expect(script).To(gomega.ContainSubstring("streamable_http"))
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
