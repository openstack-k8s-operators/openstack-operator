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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackClientSpec defines the desired state of OpenStackClient
type OpenStackClientSpec struct {
	// +kubebuilder:validation:Required
	// ContainerImage for the the OpenstackClient container
	ContainerImage string `json:"containerImage"`
	// +kubebuilder:validation:Required
	// OpenStackConfigMap is the name of the ConfigMap containing the clouds.yaml
	OpenStackConfigMap string `json:"openStackConfigMap"`
	// +kubebuilder:validation:Required
	// OpenStackConfigSecret is the name of the Secret containing the secure.yaml
	OpenStackConfigSecret string `json:"openStackConfigSecret"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running control plane services (currently only applies to KeystoneAPI and PlacementAPI)
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
}

// OpenStackClientStatus defines the observed state of OpenStackClient
type OpenStackClientStatus struct {
	// PodName
	PodName string `json:"podName,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackClient is the Schema for the openstackclients API
type OpenStackClient struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackClientSpec   `json:"spec,omitempty"`
	Status OpenStackClientStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackClientList contains a list of OpenStackClient
type OpenStackClientList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackClient `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackClient{}, &OpenStackClientList{})
}
