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
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Container image fall-back defaults

	// OpenStackClientContainerImage is the fall-back container image for OpenStackClient
	OpenStackClientContainerImage = "quay.io/podified-antelope-centos9/openstack-openstackclient:current-podified"
)

// OpenStackClientSpec defines the desired state of OpenStackClient
type OpenStackClientSpec struct {
	OpenStackClientSpecCore `json:",inline"`

	// +kubebuilder:validation:Required
	// ContainerImage for the the OpenstackClient container (will be set to environmental default if empty)
	ContainerImage string `json:"containerImage"`
}

// OpenStackClientSpecCore defines the desired state of OpenStackClient
type OpenStackClientSpecCore struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:default=openstack-config
	// OpenStackConfigMap is the name of the ConfigMap containing the clouds.yaml
	OpenStackConfigMap *string `json:"openStackConfigMap"`

	// +kubebuilder:validation:Required
	// +kubebuilder:default=openstack-config-secret
	// OpenStackConfigSecret is the name of the Secret containing the secure.yaml
	OpenStackConfigSecret *string `json:"openStackConfigSecret"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running control plane services (currently only applies to KeystoneAPI and PlacementAPI)
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Secret containing any CA certificates which should be added to deployment pods
	tls.Ca `json:",inline"`
}

// OpenStackClientStatus defines the observed state of OpenStackClient
type OpenStackClientStatus struct {
	// PodName
	PodName string `json:"podName,omitempty"`

	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	//ObservedGeneration - the most recent generation observed for this object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
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

// IsReady - returns true if OpenStackClient is reconciled successfully
func (instance OpenStackClient) IsReady() bool {
	return instance.Status.Conditions.IsTrue(OpenStackClientReadyCondition)
}

// RbacConditionsSet - set the conditions for the rbac object
func (instance OpenStackClient) RbacConditionsSet(c *condition.Condition) {
	instance.Status.Conditions.Set(c)
}

// RbacNamespace - return the namespace
func (instance OpenStackClient) RbacNamespace() string {
	return instance.Namespace
}

// RbacResourceName - return the name to be used for rbac objects (serviceaccount, role, rolebinding)
func (instance OpenStackClient) RbacResourceName() string {
	return "openstackclient-" + instance.Name
}

// SetupDefaults - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaults() {
	// Acquire environmental defaults and initialize OpenStackClient defaults with them
	openStackClientDefaults := OpenStackClientDefaults{
		ContainerImageURL: util.GetEnvVar("RELATED_IMAGE_OPENSTACK_CLIENT_IMAGE_URL_DEFAULT", OpenStackClientContainerImage),
	}

	SetupOpenStackClientDefaults(openStackClientDefaults)
}
