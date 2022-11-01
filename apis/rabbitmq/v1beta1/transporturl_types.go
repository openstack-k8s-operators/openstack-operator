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

// TransportURLSpec defines the desired state of TransportURL
type TransportURLSpec struct {
	// +kubebuilder:validation:Required
	// RabbitmqClusterName the name of the Rabbitmq cluster which to configure the transport URL
	RabbitmqClusterName string `json:"rabbitmqClusterName,omitempty"`
}

// TransportURLStatus defines the observed state of TransportURL
type TransportURLStatus struct {

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// SecretName - name of the secret containing the rabbitmq transport URL
	SecretName string `json:"secretName,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// TransportURL is the Schema for the transporturls API
type TransportURL struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TransportURLSpec   `json:"spec,omitempty"`
	Status TransportURLStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TransportURLList contains a list of TransportURL
type TransportURLList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TransportURL `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TransportURL{}, &TransportURLList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance TransportURL) IsReady() bool {
	return instance.Status.Conditions.IsTrue(TransportURLReadyCondition)
}
