/*
Copyright 2026.

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

package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// OpenStackAssistantContainerImage is the fall-back container image for OpenStackAssistant
	OpenStackAssistantContainerImage = "quay.io/openstack-s2i-containers/openstack-goose:master-latest"

	// AssistantDefaultWorkingDir is the default writable working directory (also
	// used as HOME) for the assistant's configuration, sessions, and runtime state.
	AssistantDefaultWorkingDir = "/home/goose"
)

// ModelRef describes an additional LLM model the assistant may select, e.g. a
// heavier model for adversarial cross-review or a cheaper model for simple
// subagent tasks. Exposed to the container so the image entrypoint can render
// it into the harness config.
type ModelRef struct {
	// Name is the model identifier, e.g. "gemini/models/gemini-2.5-pro".
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// BaseURL overrides the LightspeedStack BaseURL for this model. When empty
	// the LightspeedStack BaseURL is used.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^https?://[^\p{Cc}]+$`
	BaseURL string `json:"baseURL,omitempty"`
}

// StorageSpec configures the writable working directory used by the agent for
// configuration, sessions, and runtime state. A writable directory is required
// because the pod runs as a non-root, arbitrary UID with only read-only
// ConfigMap mounts otherwise.
type StorageSpec struct {
	// MountPath is the writable working directory, also exported as HOME.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=/home/goose
	// +kubebuilder:validation:Pattern=`^/[^\p{Cc}]*$`
	MountPath string `json:"mountPath,omitempty"`

	// PVCName, when set, backs the working directory with an existing
	// PersistentVolumeClaim instead of an ephemeral emptyDir, persisting state
	// across pod replacement and node reboots.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	PVCName string `json:"pvcName,omitempty"`
}

// LightspeedStackSpec defines connectivity to the Lightspeed Stack (LLM backend).
// The API key is derived at runtime from the pod's service-account token by the
// image entrypoint; no provider secret is required.
type LightspeedStackSpec struct {
	// BaseURL of the Lightspeed Stack OpenAI-compatible endpoint,
	// e.g. "https://lightspeed-app-server.openstack.svc:8443/v1".
	// Exposed to the assistant container as the LIGHTSPEED_URL env var.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^https?://[^\p{Cc}]+$`
	BaseURL string `json:"baseURL"`

	// Model identifier, e.g. "gemini/models/gemini-2.5-flash".
	// Exposed to the assistant container as the LIGHTSPEED_MODEL env var.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Model string `json:"model"`

	// AdditionalModels are extra models the agent may select per subagent, e.g.
	// a heavier model for adversarial cross-review or a cheaper model for simple
	// tasks. Exposed to the container as the LIGHTSPEED_ADDITIONAL_MODELS env var
	// (JSON) for the image entrypoint to render into the harness config.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	AdditionalModels []ModelRef `json:"additionalModels,omitempty"`
}

// MCPServerRef references an MCP server endpoint to expose to the assistant.
// Exactly one of URL or OpenStackClientRef must be specified.
// +kubebuilder:validation:XValidation:rule="has(self.url) != has(self.openstackClientRef)",message="exactly one of url or openstackClientRef must be set"
type MCPServerRef struct {
	// Name is the MCP server name. It is used to derive the
	// MCP_SERVER_<name> environment variable, so it must be a valid
	// environment-variable name: start with a letter or underscore and
	// contain only letters, digits, and underscores (no dashes).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=63
	// +kubebuilder:validation:Pattern=`^[a-zA-Z_][a-zA-Z0-9_]*$`
	Name string `json:"name"`

	// URL is the MCP server's Streamable HTTP endpoint.
	// Mutually exclusive with OpenStackClientRef.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	// ensures that a string contains no control characters
	// +kubebuilder:validation:Pattern=`^[^\p{Cc}]*$`
	URL string `json:"url,omitempty"`

	// OpenStackClientRef is the name of an OpenStackClient CR in the same
	// namespace that has MCP enabled. The controller reads the service URL
	// published in the OpenStackClient status.
	// Mutually exclusive with URL.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinLength=1
	OpenStackClientRef string `json:"openstackClientRef,omitempty"`
}

// ConfigMount projects a ConfigMap into the assistant pod at a caller-chosen
// path. The meaning of the mounted files is defined entirely by the image
// entrypoint (e.g. a Goose image may read recipes, skills or hints from the
// mounted paths). The operator only mounts the data and tracks it so that
// content changes trigger a pod restart.
type ConfigMount struct {
	// Name of a ConfigMap in the same namespace.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// MountPath is the absolute directory the ConfigMap keys are projected to.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Pattern=`^/[^\p{Cc}]*$`
	MountPath string `json:"mountPath"`
}

// OpenStackAssistantSpec defines the desired state of OpenStackAssistant
type OpenStackAssistantSpec struct {
	// ContainerImage is the agent/harness image (will be set to the
	// environmental default if empty). Its baked-in entrypoint is responsible
	// for rendering harness config from the env vars and mounts the operator
	// provides.
	// +kubebuilder:validation:Required
	ContainerImage string `json:"containerImage"`

	// LightspeedStack configuration for the AI backend (base URL + model).
	// +kubebuilder:validation:Required
	LightspeedStack LightspeedStackSpec `json:"lightspeedStack"`

	// ExtraConfig is a list of ConfigMaps to project into the pod at chosen
	// paths for the image entrypoint to consume (recipes, skills, hints,
	// system prompts, etc. - all harness-defined).
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	ExtraConfig []ConfigMount `json:"extraConfig,omitempty"`

	// MCPServers lists MCP server endpoints to expose to the assistant. Each
	// entry is exported to the pod as an MCP_SERVER_<name> environment variable.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	MCPServers []MCPServerRef `json:"mcpServers,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Secret containing any CA certificates which should be added to the assistant pod.
	tls.Ca `json:",inline"`

	// Storage configures the writable working directory (also HOME) used for the
	// agent's configuration, sessions, and runtime state. Defaults to an emptyDir
	// mounted at /home/goose; an existing PVC can be used to persist its contents.
	// +kubebuilder:validation:Optional
	Storage *StorageSpec `json:"storage,omitempty"`

	// Resources defines the compute resource requirements for the assistant
	// container. This is useful for agent workloads that run concurrent processes.
	// +kubebuilder:validation:Optional
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`

	// NodeSelector to target subset of worker nodes for pod scheduling.
	// +kubebuilder:validation:Optional
	NodeSelector *map[string]string `json:"nodeSelector,omitempty"`

	// Env is a list of additional environment variables for the container.
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	Env []corev1.EnvVar `json:"env,omitempty"`
}

// OpenStackAssistantStatus defines the observed state of OpenStackAssistant
type OpenStackAssistantStatus struct {
	// PodName is the name of the running assistant pod
	PodName string `json:"podName,omitempty"`

	// Conditions tracks the state of each sub-resource
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// ObservedGeneration - the most recent generation observed
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Hash tracks input hashes to detect changes
	Hash map[string]string `json:"hash,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Assistant"
// +kubebuilder:resource:shortName=osassistant;osassistants
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackAssistant is the Schema for the openstackassistants API
type OpenStackAssistant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackAssistantSpec   `json:"spec,omitempty"`
	Status OpenStackAssistantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackAssistantList contains a list of OpenStackAssistant
type OpenStackAssistantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackAssistant `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackAssistant{}, &OpenStackAssistantList{})
}

// IsReady - returns true if OpenStackAssistant is reconciled successfully
func (instance OpenStackAssistant) IsReady() bool {
	return instance.Status.Conditions.IsTrue(OpenStackAssistantReadyCondition)
}

// RbacConditionsSet - set the conditions for the rbac object
func (instance *OpenStackAssistant) RbacConditionsSet(c *condition.Condition) {
	instance.Status.Conditions.Set(c)
}

// RbacNamespace - return the namespace
func (instance OpenStackAssistant) RbacNamespace() string {
	return instance.Namespace
}

// RbacResourceName - return the name to be used for rbac objects (serviceaccount, role, rolebinding)
func (instance OpenStackAssistant) RbacResourceName() string {
	return "openstackassistant-" + instance.Name
}

// WorkingDir returns the writable working directory (also used as HOME) for the
// assistant, honoring spec.storage.mountPath and falling back to the default.
func (instance OpenStackAssistant) WorkingDir() string {
	if instance.Spec.Storage != nil && instance.Spec.Storage.MountPath != "" {
		return instance.Spec.Storage.MountPath
	}
	return AssistantDefaultWorkingDir
}

// OpenStackAssistantDefaults holds defaults for the assistant
type OpenStackAssistantDefaults struct {
	ContainerImageURL string
}

var openStackAssistantDefaults OpenStackAssistantDefaults

// SetupOpenStackAssistantDefaults - initialize OpenStackAssistant spec defaults
func SetupOpenStackAssistantDefaults(defaults OpenStackAssistantDefaults) {
	openStackAssistantDefaults = defaults
}

// SetupDefaults - initializes any CRD field defaults based on environment variables
func SetupDefaults() {
	openStackAssistantDefaults := OpenStackAssistantDefaults{
		ContainerImageURL: util.GetEnvVar("RELATED_IMAGE_OPENSTACK_ASSISTANT_IMAGE_URL_DEFAULT", OpenStackAssistantContainerImage),
	}

	SetupOpenStackAssistantDefaults(openStackAssistantDefaults)
}

// Default implements webhook.Defaulter
func (r *OpenStackAssistant) Default() {
	if r.Spec.ContainerImage == "" {
		r.Spec.ContainerImage = openStackAssistantDefaults.ContainerImageURL
	}
	if r.Spec.CaBundleSecretName == "" {
		r.Spec.CaBundleSecretName = tls.CABundleSecret
	}
}
