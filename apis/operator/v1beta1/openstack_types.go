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

// OpenStackSpec defines the desired state of OpenStack
type OpenStackSpec struct {
}

// OpenStackStatus defines the observed state of OpenStack
type OpenStackStatus struct {

	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// DeployedOperatorCount - the number of operators deployed
	DeployedOperatorCount *int `json:"deployedOperatorCount,omitempty"`

	// ObservedGeneration - the most recent generation observed for this object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"` // no spec yet so maybe we don't need this

	// ContainerImage - the container image that has been successfully deployed
	ContainerImage *string `json:"containerImage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack"
// +kubebuilder:printcolumn:name="Deployed Operator Count",type=integer,JSONPath=`.status.deployedOperatorCount`
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
// OpenStack is the Schema for the openstacks API
type OpenStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackSpec   `json:"spec,omitempty"`
	Status OpenStackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackList contains a list of OpenStack
type OpenStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStack{}, &OpenStackList{})
}
