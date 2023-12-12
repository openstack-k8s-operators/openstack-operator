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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackVersionSpec defines the desired state of OpenStackVersion
type OpenStackVersionSpec struct {

	// Strategy -
	//Strategy UpdateStrategy `json:"strategy,omitempty"`

	// 1.0.1 --> 1.0.2
	TargetVersion string `json:"targetVersion,omitempty"`

	// +kubebuilder:default="openstack"
	OpenStackControlPlaneName string `json:"openstackControlPlaneName,omitempty"`

	// ServiceExcludes is a list of services to exclude from the update. Any named service here will be excluded from the update.
	ServiceExcludes []string `json:"serviceExcludes,omitempty"`

	// CinderVolumeExcludes is a list of cinder volumes to exclude from the update. Any named Cinder volume instance here will be excluded.
	CinderVolumeExcludes []string `json:"cinderVolumeExcludes,omitempty"`

	//FIXME: add parameters for other custom service maps (Neutron, etc)
}

// UpdateStrategy defines the strategy used to roll out updates to the OpenStack services
/*
type UpdateStrategy struct {

	// +kubebuilder:default="automatic"
	UpdateType string `json:"updateType,omitempty"`


	// Type serial or parallel
	// +kubebuilder:default="serial"
	//Type string `json:"type"`
}*/

type ServiceVersionURL struct {
	ServiceName       string `json:"name,omitempty"`
	ContainerImageUrl string `json:"containerImageUrl,omitempty"`
}

type OpenStackService struct {
	ServiceName string `json:"name,omitempty"`
}

// OpenStackVersionStatus defines the observed state of OpenStackVersion
type OpenStackVersionStatus struct {
	ServicesNeedingUpdates []OpenStackService `json:"updateNeeded,omitempty"`

	DeployedVersions []OpenStackService `json:"updateApplied,omitempty"`

	TargetVersion string `json:"targetVersion,omitempty"`

	AvailableVersion string `json:"availableVersion,omitempty"`

	AvailableServices []string `json:"availableService,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Version"
// +kubebuilder:printcolumn:name="Target Version",type=string,JSONPath=`.spec.targetVersion`
// +kubebuilder:printcolumn:name="Available Version",type=string,JSONPath=`.status.availableVersion`

// OpenStackVersion is the Schema for the openstackversionupdates API
type OpenStackVersion struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackVersionSpec   `json:"spec,omitempty"`
	Status OpenStackVersionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackVersionList contains a list of OpenStackVersion
type OpenStackVersionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackVersion `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackVersion{}, &OpenStackVersionList{})
}
