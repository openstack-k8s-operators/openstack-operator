/*
Copyright 2025.

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
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Container image fall-back defaults

	// OpenStackLightspeedContainerImage is the fall-back container image for OpenStackLightspeed
	OpenStackLightspeedContainerImage = "quay.io/openstack-lightspeed/rag-content:os-docs-2024.2"
)

// OpenStackLightspeedSpec defines the desired state of OpenStackLightspeed
type OpenStackLightspeedSpec struct {
	OpenStackLightspeedCore `json:",inline"`

	// +kubebuilder:validation:Optional
	// ContainerImage for the Openstack Lightspeed RAG container (will be set to environmental default if empty)
	RAGImage string `json:"ragImage"`
}

// OpenStackLightspeedCore defines the desired state of OpenStackLightspeed
type OpenStackLightspeedCore struct {
	// +kubebuilder:validation:Required
	// URL pointing to the LLM
	LLMEndpoint string `json:"llmEndpoint"`

	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=azure_openai;bam;openai;watsonx;rhoai_vllm;rhelai_vllm;fake_provider
	// +operator-sdk:csv:customresourcedefinitions:type=spec,displayName="Provider Type"
	// Type of the provider serving the LLM
	LLMEndpointType string `json:"llmEndpointType"`

	// +kubebuilder:validation:Required
	// Name of the model to use at the API endpoint provided in LLMEndpoint
	ModelName string `json:"modelName"`

	// +kubebuilder:validation:Required
	// Secret name containing API token for the LLMEndpoint. The key for the field
	// in the secret that holds the token should be "apitoken".
	LLMCredentials string `json:"llmCredentials"`

	// +kubebuilder:validation:Optional
	// Configmap name containing a CA Certificates bundle
	TLSCACertBundle string `json:"tlsCACertBundle"`
}

// OpenStackLightspeedStatus defines the observed state of OpenStackLightspeed
type OpenStackLightspeedStatus struct {
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// ObservedGeneration - the most recent generation observed for this object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Lightspeed"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackLightspeed is the Schema for the openstacklightspeeds API
type OpenStackLightspeed struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackLightspeedSpec   `json:"spec,omitempty"`
	Status OpenStackLightspeedStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackLightspeedList contains a list of OpenStackLightspeed
type OpenStackLightspeedList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackLightspeed `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackLightspeed{}, &OpenStackLightspeedList{})
}

// IsReady - returns true if OpenStackLightspeed is reconciled successfully
func (instance OpenStackLightspeed) IsReady() bool {
	return instance.Status.Conditions.IsTrue(OpenStackLightspeedReadyCondition)
}

type OpenStackLightspeedDefaults struct {
	RAGImageURL string
}

var OpenStackLightspeedDefaultValues OpenStackLightspeedDefaults

// SetupDefaults - initializes OpenStackLightspeedDefaultValues with default values from env vars
func SetupDefaults() {
	// Acquire environmental defaults and initialize OpenStackLightspeed defaults with them
	openStackLightspeedDefaults := OpenStackLightspeedDefaults{
		RAGImageURL: util.GetEnvVar(
			"RELATED_IMAGE_OPENSTACK_LIGHTSPEED_IMAGE_URL_DEFAULT", OpenStackLightspeedContainerImage),
	}

	OpenStackLightspeedDefaultValues = openStackLightspeedDefaults
}
