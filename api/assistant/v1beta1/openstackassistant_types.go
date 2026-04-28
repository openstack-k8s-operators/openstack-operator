/*
Copyright 2022.

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
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// OpenStackAssistantContainerImage is the fall-back container image for OpenStackAssistant
	OpenStackAssistantContainerImage = "quay.io/dprince/goose:oc-fedora"
)

// ProviderType defines the AI agent provider
// +kubebuilder:validation:Enum=goose
type ProviderType string

const (
	// ProviderGoose is the Goose AI agent provider
	ProviderGoose ProviderType = "goose"
)

// LightspeedStackSpec defines connectivity to the Lightspeed Stack AI backend
type LightspeedStackSpec struct {
	// ProviderSecret is the name of a Secret containing the lightspeed
	// provider config JSON (custom_providers/lightspeed.json content).
	// Must contain key "lightspeed.json".
	// +kubebuilder:validation:Required
	ProviderSecret string `json:"providerSecret"`

	// CaBundleSecretName is the name of a Secret containing CA certs
	// to trust for TLS connections to the lightspeed-stack endpoint.
	// The Secret must contain a key "ca-bundle.crt" with PEM-encoded certs.
	// +kubebuilder:validation:Optional
	CaBundleSecretName string `json:"caBundleSecretName,omitempty"`
}

// GooseConfig defines Goose-specific provider configuration
type GooseConfig struct {
	// Recipes is a ConfigMap name containing Goose recipe YAML files.
	// Each key in the ConfigMap becomes a recipe file registered as a
	// Goose slash command (e.g., /cluster-health).
	// +kubebuilder:validation:Optional
	Recipes *string `json:"recipes,omitempty"`

	// Hints is a ConfigMap name containing Goose hints/context.
	// The ConfigMap must have a key "hints" with the content that
	// will be written to ~/.goosehints in the pod.
	// +kubebuilder:validation:Optional
	Hints *string `json:"hints,omitempty"`
}

// OpenStackAssistantSpec defines the desired state of OpenStackAssistant
type OpenStackAssistantSpec struct {
	// ContainerImage for the assistant container.
	// +kubebuilder:validation:Required
	ContainerImage string `json:"containerImage"`

	// Provider is the AI agent provider type. Currently only "goose" is supported.
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=goose
	Provider ProviderType `json:"provider,omitempty"`

	// LightspeedStack configuration for the AI backend.
	// +kubebuilder:validation:Required
	LightspeedStack LightspeedStackSpec `json:"lightspeedStack"`

	// Goose contains Goose-specific provider configuration.
	// Only applicable when provider is "goose".
	// +kubebuilder:validation:Optional
	Goose *GooseConfig `json:"goose,omitempty"`

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
func (instance OpenStackAssistant) RbacConditionsSet(c *condition.Condition) {
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
}
