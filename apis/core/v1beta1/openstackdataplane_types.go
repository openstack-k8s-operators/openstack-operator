/*
Copyright 2023.

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
	dataplanev1beta1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OpenStackDataPlaneSpec defines the desired state of OpenStackDataPlane
type OpenStackDataPlaneSpec struct {

	// +kubebuilder:validation:Optional
	// Nodes - List of nodes
	Nodes map[string]dataplanev1beta1.OpenStackDataPlaneNodeSpec `json:"nodes,omitempty"`
	// +kubebuilder:validation:Optional
	// Roles - List of roles
	Roles map[string]dataplanev1beta1.OpenStackDataPlaneRoleSpec `json:"roles,omitempty"`
}

// OpenStackDataPlaneStatus defines the observed state of OpenStackDataPlane
type OpenStackDataPlaneStatus struct {

	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack DataPlane"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackDataPlane is the Schema for the openstackdataplanes API
type OpenStackDataPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackDataPlaneSpec   `json:"spec,omitempty"`
	Status OpenStackDataPlaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackDataPlaneList contains a list of OpenStackDataPlane
type OpenStackDataPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackDataPlane `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackDataPlane{}, &OpenStackDataPlaneList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance OpenStackDataPlane) IsReady() bool {
	for _, c := range instance.Status.Conditions {
		if c.Type == condition.ReadyCondition {
			continue
		}
		if c.Status != corev1.ConditionTrue {
			return false
		}
	}
	return true
}

// InitConditions - Initializes Status Conditons
func (instance OpenStackDataPlane) InitConditions() {
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
	}
	cl := condition.CreateList(
		// Add the overall status condition as Unknown
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
	)
	if len(instance.Spec.Nodes) > 0 {
		node := condition.UnknownCondition(OpenStackDataPlaneNodeReadyCondition, condition.InitReason, OpenStackDataPlaneNodeReadyInitMessage)
		cl = append(cl, *node)
	}
	if len(instance.Spec.Roles) > 0 {
		role := condition.UnknownCondition(OpenStackDataPlaneRoleReadyCondition, condition.InitReason, OpenStackDataPlaneRoleReadyInitMessage)
		cl = append(cl, *role)
	}

	// initialize conditions used later as Status=Unknown
	instance.Status.Conditions.Init(&cl)
}
