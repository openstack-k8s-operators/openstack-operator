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

	. "github.com/onsi/gomega"

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
	g := NewWithT(t)

	script := EntrypointScript()

	g.Expect(script).To(ContainSubstring("#!/bin/sh"))
	g.Expect(script).To(ContainSubstring("mkdir -p ~/.goose/config/profiles/default/custom_providers"))
	g.Expect(script).To(ContainSubstring("config.yaml"))
	g.Expect(script).To(ContainSubstring("developer:"))
	g.Expect(script).To(ContainSubstring("enabled: true"))
	g.Expect(script).To(ContainSubstring("/tmp/recipes/"))
	g.Expect(script).To(ContainSubstring("/tmp/hints/hints"))
	g.Expect(script).To(ContainSubstring("/tmp/lightspeed-provider/lightspeed.json"))
	g.Expect(script).To(ContainSubstring("sleep infinity"))
}

func TestAssistantPodSpec_BasicFields(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "testhash123")

	g.Expect(spec.ServiceAccountName).To(Equal("openstackassistant-test-assistant"))
	g.Expect(*spec.TerminationGracePeriodSeconds).To(Equal(int64(0)))
	g.Expect(spec.Containers).To(HaveLen(1))

	container := spec.Containers[0]
	g.Expect(container.Name).To(Equal("goose"))
	g.Expect(container.Image).To(Equal("quay.io/dprince/goose:oc-fedora"))
	g.Expect(container.Command).To(Equal([]string{"/bin/sh"}))
	g.Expect(container.Args).To(Equal([]string{"/tmp/entrypoint/entrypoint.sh"}))
}

func TestAssistantPodSpec_SecurityContext(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash")
	sc := spec.Containers[0].SecurityContext

	g.Expect(*sc.RunAsNonRoot).To(BeTrue())
	g.Expect(*sc.AllowPrivilegeEscalation).To(BeFalse())
	g.Expect(sc.Capabilities.Drop).To(ContainElement(corev1.Capability("ALL")))
}

func TestAssistantPodSpec_DefaultEnvVars(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "somehash")
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(HaveKeyWithValue("CONFIG_HASH", "somehash"))
	g.Expect(envMap).To(HaveKeyWithValue("GOOSE_PROVIDER", "lightspeed"))
	g.Expect(envMap).To(HaveKeyWithValue("GOOSE_TELEMETRY_ENABLED", "false"))
	g.Expect(envMap).To(HaveKeyWithValue("GOOSE_DISABLE_KEYRING", "1"))
}

func TestAssistantPodSpec_CustomEnvVars(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Env = []corev1.EnvVar{
		{Name: "GOOSE_MODEL", Value: "gemini/models/gemini-2.5-flash"},
		{Name: "LIGHTSPEED_API_KEY", Value: "dummy"},
	}

	spec := AssistantPodSpec(instance, "hash")
	envVars := spec.Containers[0].Env

	envMap := make(map[string]string)
	for _, e := range envVars {
		envMap[e.Name] = e.Value
	}

	g.Expect(envMap).To(HaveKeyWithValue("GOOSE_MODEL", "gemini/models/gemini-2.5-flash"))
	g.Expect(envMap).To(HaveKeyWithValue("LIGHTSPEED_API_KEY", "dummy"))
	g.Expect(envMap).To(HaveKeyWithValue("GOOSE_PROVIDER", "lightspeed"))
}

func TestAssistantPodSpec_MinimalVolumes(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash")

	g.Expect(spec.Volumes).To(HaveLen(2))

	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(ContainElements("entrypoint", "lightspeed-provider"))

	mountNames := make([]string, len(spec.Containers[0].VolumeMounts))
	for i, m := range spec.Containers[0].VolumeMounts {
		mountNames[i] = m.Name
	}
	g.Expect(mountNames).To(ContainElements("entrypoint", "lightspeed-provider"))
}

func TestAssistantPodSpec_WithRecipesAndHints(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Recipes: ptr.To("assistant-recipes"),
		Hints:   ptr.To("assistant-hints"),
	}

	spec := AssistantPodSpec(instance, "hash")

	g.Expect(spec.Volumes).To(HaveLen(4))
	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(ContainElements("entrypoint", "lightspeed-provider", "recipes", "hints"))

	mountNames := make([]string, len(spec.Containers[0].VolumeMounts))
	for i, m := range spec.Containers[0].VolumeMounts {
		mountNames[i] = m.Name
	}
	g.Expect(mountNames).To(ContainElements("entrypoint", "lightspeed-provider", "recipes", "hints"))
}

func TestAssistantPodSpec_WithCaBundle(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()
	instance.Spec.LightspeedStack.CaBundleSecretName = "lightspeed-ca-bundle"

	spec := AssistantPodSpec(instance, "hash")

	g.Expect(spec.Volumes).To(HaveLen(3))

	var caBundleVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "ca-bundle" {
			caBundleVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(caBundleVolume).NotTo(BeNil())
	g.Expect(caBundleVolume.Secret.SecretName).To(Equal("lightspeed-ca-bundle"))

	var caBundleMount *corev1.VolumeMount
	for i := range spec.Containers[0].VolumeMounts {
		if spec.Containers[0].VolumeMounts[i].Name == "ca-bundle" {
			caBundleMount = &spec.Containers[0].VolumeMounts[i]
			break
		}
	}
	g.Expect(caBundleMount).NotTo(BeNil())
	g.Expect(caBundleMount.MountPath).To(Equal("/etc/ssl/certs/ca-certificates.crt"))
	g.Expect(caBundleMount.SubPath).To(Equal("ca-bundle.crt"))
	g.Expect(caBundleMount.ReadOnly).To(BeTrue())
}

func TestAssistantPodSpec_WithNodeSelector(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()
	instance.Spec.NodeSelector = &map[string]string{
		"node-role.kubernetes.io/worker": "",
	}

	spec := AssistantPodSpec(instance, "hash")

	g.Expect(spec.NodeSelector).To(HaveKeyWithValue("node-role.kubernetes.io/worker", ""))
}

func TestAssistantPodSpec_WithoutNodeSelector(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash")

	g.Expect(spec.NodeSelector).To(BeNil())
}

func TestAssistantPodSpec_EntrypointConfigMapName(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash")

	var entrypointVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "entrypoint" {
			entrypointVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(entrypointVolume).NotTo(BeNil())
	g.Expect(entrypointVolume.ConfigMap.Name).To(Equal("test-assistant-entrypoint"))
	g.Expect(*entrypointVolume.ConfigMap.DefaultMode).To(Equal(int32(0755)))
}

func TestAssistantPodSpec_LightspeedProviderSecretName(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()

	spec := AssistantPodSpec(instance, "hash")

	var providerVolume *corev1.Volume
	for i := range spec.Volumes {
		if spec.Volumes[i].Name == "lightspeed-provider" {
			providerVolume = &spec.Volumes[i]
			break
		}
	}
	g.Expect(providerVolume).NotTo(BeNil())
	g.Expect(providerVolume.Secret.SecretName).To(Equal("lightspeed-provider-config"))
}

func TestAssistantPodSpec_AllVolumeMountsReadOnly(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Recipes: ptr.To("recipes-cm"),
		Hints:   ptr.To("hints-cm"),
	}
	instance.Spec.LightspeedStack.CaBundleSecretName = "ca-secret"

	spec := AssistantPodSpec(instance, "hash")

	for _, mount := range spec.Containers[0].VolumeMounts {
		g.Expect(mount.ReadOnly).To(BeTrue(), "VolumeMount %s should be read-only", mount.Name)
	}
}

func TestAssistantPodSpec_RecipesOnlyNoHints(t *testing.T) {
	g := NewWithT(t)
	instance := newTestInstance()
	instance.Spec.Goose = &assistantv1.GooseConfig{
		Recipes: ptr.To("recipes-cm"),
	}

	spec := AssistantPodSpec(instance, "hash")

	g.Expect(spec.Volumes).To(HaveLen(3))
	volumeNames := make([]string, len(spec.Volumes))
	for i, v := range spec.Volumes {
		volumeNames[i] = v.Name
	}
	g.Expect(volumeNames).To(ContainElement("recipes"))
	g.Expect(volumeNames).NotTo(ContainElement("hints"))
}

func TestEntrypointScript_DisabledExtensions(t *testing.T) {
	g := NewWithT(t)

	script := EntrypointScript()

	disabledExtensions := []string{"computercontroller", "apps", "analyze", "todo", "extensionmanager", "chatrecall"}
	for _, ext := range disabledExtensions {
		idx := strings.Index(script, ext+":")
		g.Expect(idx).To(BeNumerically(">", 0), "should contain %s", ext)
		enabledLine := script[idx:]
		enabledLine = enabledLine[:strings.Index(enabledLine, "\n")]
		g.Expect(script).To(ContainSubstring(ext))
	}

	enabledExtensions := []string{"developer", "summarize", "summon"}
	for _, ext := range enabledExtensions {
		g.Expect(script).To(ContainSubstring(ext))
	}
}
