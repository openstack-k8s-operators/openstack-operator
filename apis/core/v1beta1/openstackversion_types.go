/*
Copyright 2024.

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

const (
	// MinorUpdateOvnControlPlane -
	MinorUpdateOvnControlPlane string = "Minor Update OVN Controlplane In Progress"
	// MinorUpdateControlPlane -
	MinorUpdateControlPlane string = "Minor Update Controlplane In Progress"
	// MinorUpdateComplete -
	MinorUpdateComplete string = "Complete"
)

// OpenStackVersionSpec - defines the desired state of OpenStackVersion
type OpenStackVersionSpec struct {

	// +kubebuilder:validation:Required
	// TargetVersion is the version of OpenStack to install (based on the availableVersion in the OpenStackVersion CR status)
	TargetVersion string `json:"targetVersion"`

	// CustomContainerImages is a list of containerImages to customize for deployment
	CustomContainerImages CustomContainerImages `json:"customContainerImages,omitempty"`
}

// CustomContainerImages - struct for custom container images
type CustomContainerImages struct {
	ContainerTemplate  `json:",inline"`
	CinderVolumeImages map[string]*string `json:"cinderVolumeImages,omitempty"`
	ManilaShareImages  map[string]*string `json:"manilaShareImages,omitempty"`
}

// ContainerDefaults - struct that contains container image default URLs for each service (internal use only)
type ContainerDefaults struct {
	ContainerTemplate `json:",inline"`
	CinderVolumeImage *string `json:"cinderVolumeImage,omitempty"`
	ManilaShareImage  *string `json:"manilaShareImage,omitempty"`
}

// ContainerImages - struct acts as the source of truth for container image URLs to be deployed
type ContainerImages struct {
	ContainerTemplate `json:",inline"`
	// CinderVolumeImages custom Cinder Volume images for each backend (default Cinder volume image is stored 'default' key)
	// TODO: add validation to cinder-operator to prevent backend being named 'default'
	CinderVolumeImages map[string]*string `json:"cinderVolumeImages,omitempty"`
	// ManilaShareImages custom Manila Share images for each backend (default Manila share image is stored 'default' key)
	// TODO: add validation to cinder-operator to prevent backend being named 'default'
	ManilaShareImages map[string]*string `json:"manilaShareImages,omitempty"`
}

// ContainerTemplate - struct that contains container image URLs for each service in OpenStackControlplane
type ContainerTemplate struct {
	AgentImage                    *string `json:"agentImage,omitempty"`
	AnsibleeeImage                *string `json:"ansibleeeImage,omitempty"`
	AodhAPIImage                  *string `json:"aodhAPIImage,omitempty"`
	AodhEvaluatorImage            *string `json:"aodhEvaluatorImage,omitempty"`
	AodhListenerImage             *string `json:"aodhListenerImage,omitempty"`
	AodhNotifierImage             *string `json:"aodhNotifierImage,omitempty"`
	ApacheImage                   *string `json:"apacheImage,omitempty"`
	BarbicanAPIImage              *string `json:"barbicanAPIImage,omitempty"`
	BarbicanKeystoneListenerImage *string `json:"barbicanKeystoneListenerImage,omitempty"`
	BarbicanWorkerImage           *string `json:"barbicanWorkerImage,omitempty"`
	CeilometerCentralImage        *string `json:"ceilometerCentralImage,omitempty"`
	CeilometerComputeImage        *string `json:"ceilometerComputeImage,omitempty"`
	CeilometerIpmiImage           *string `json:"ceilometerIpmiImage,omitempty"`
	CeilometerNotificationImage   *string `json:"ceilometerNotificationImage,omitempty"`
	CeilometerSgcoreImage         *string `json:"ceilometerSgcoreImage,omitempty"`
	CinderAPIImage                *string `json:"cinderAPIImage,omitempty"`
	CinderBackupImage             *string `json:"cinderBackupImage,omitempty"`
	CinderSchedulerImage          *string `json:"cinderSchedulerImage,omitempty"`
	DesignateAPIImage             *string `json:"designateAPIImage,omitempty"`
	DesignateBackendbind9Image    *string `json:"designateBackendbind9Image,omitempty"`
	DesignateCentralImage         *string `json:"designateCentralImage,omitempty"`
	DesignateMdnsImage            *string `json:"designateMdnsImage,omitempty"`
	DesignateProducerImage        *string `json:"designateProducerImage,omitempty"`
	DesignateUnboundImage         *string `json:"designateUnboundImage,omitempty"`
	DesignateWorkerImage          *string `json:"designateWorkerImage,omitempty"`
	EdpmFrrImage                  *string `json:"edpmFrrImage,omitempty"`
	EdpmIscsidImage               *string `json:"edpmIscsidImage,omitempty"`
	EdpmLogrotateCrondImage       *string `json:"edpmLogrotateCrondImage,omitempty"`
	EdpmMultipathdImage           *string `json:"edpmMultipathdImage,omitempty"`
	EdpmNeutronMetadataAgentImage *string `json:"edpmNeutronMetadataAgentImage,omitempty"`
	EdpmNeutronSriovAgentImage    *string `json:"edpmNeutronSriovAgentImage,omitempty"`
	EdpmOvnBgpAgentImage          *string `json:"edpmOvnBgpAgentImage,omitempty"`
	EdpmNodeExporterImage         *string `json:"edpmNodeExporterImage,omitempty"`
	GlanceAPIImage                *string `json:"glanceAPIImage,omitempty"`
	HeatAPIImage                  *string `json:"heatAPIImage,omitempty"`
	HeatCfnapiImage               *string `json:"heatCfnapiImage,omitempty"`
	HeatEngineImage               *string `json:"heatEngineImage,omitempty"`
	HorizonImage                  *string `json:"horizonImage,omitempty"`
	InfraDnsmasqImage             *string `json:"infraDnsmasqImage,omitempty"`
	InfraMemcachedImage           *string `json:"infraMemcachedImage,omitempty"`
	InfraRedisImage               *string `json:"infraRedisImage,omitempty"`
	IronicAPIImage                *string `json:"ironicAPIImage,omitempty"`
	IronicConductorImage          *string `json:"ironicConductorImage,omitempty"`
	IronicInspectorImage          *string `json:"ironicInspectorImage,omitempty"`
	IronicNeutronAgentImage       *string `json:"ironicNeutronAgentImage,omitempty"`
	IronicPxeImage                *string `json:"ironicPxeImage,omitempty"`
	IronicPythonAgentImage        *string `json:"ironicPythonAgentImage,omitempty"`
	KeystoneAPIImage              *string `json:"keystoneAPIImage,omitempty"`
	ManilaAPIImage                *string `json:"manilaAPIImage,omitempty"`
	ManilaSchedulerImage          *string `json:"manilaSchedulerImage,omitempty"`
	MariadbImage                  *string `json:"mariadbImage,omitempty"`
	NeutronAPIImage               *string `json:"neutronAPIImage,omitempty"`
	NovaAPIImage                  *string `json:"novaAPIImage,omitempty"`
	NovaComputeImage              *string `json:"novaComputeImage,omitempty"`
	NovaConductorImage            *string `json:"novaConductorImage,omitempty"`
	NovaNovncImage                *string `json:"novaNovncImage,omitempty"`
	NovaSchedulerImage            *string `json:"novaSchedulerImage,omitempty"`
	OctaviaAPIImage               *string `json:"octaviaAPIImage,omitempty"`
	OctaviaHealthmanagerImage     *string `json:"octaviaHealthmanagerImage,omitempty"`
	OctaviaHousekeepingImage      *string `json:"octaviaHousekeepingImage,omitempty"`
	OctaviaWorkerImage            *string `json:"octaviaWorkerImage,omitempty"`
	OpenstackClientImage          *string `json:"openstackClientImage,omitempty"`
	OsContainerImage              *string `json:"osContainerImage,omitempty"` //fixme wire this in?
	OvnControllerImage            *string `json:"ovnControllerImage,omitempty"`
	OvnControllerOvsImage         *string `json:"ovnControllerOvsImage,omitempty"`
	OvnNbDbclusterImage           *string `json:"ovnNbDbclusterImage,omitempty"`
	OvnNorthdImage                *string `json:"ovnNorthdImage,omitempty"`
	OvnSbDbclusterImage           *string `json:"ovnSbDbclusterImage,omitempty"`
	PlacementAPIImage             *string `json:"placementAPIImage,omitempty"`
	RabbitmqImage                 *string `json:"rabbitmqImage,omitempty"`
	SwiftAccountImage             *string `json:"swiftAccountImage,omitempty"`
	SwiftContainerImage           *string `json:"swiftContainerImage,omitempty"`
	SwiftObjectImage              *string `json:"swiftObjectImage,omitempty"`
	SwiftProxyImage               *string `json:"swiftProxyImage,omitempty"`
	TelemetryNodeExporterImage    *string `json:"telemetryNodeExporterImage,omitempty"`
}

// OpenStackVersionStatus defines the observed state of OpenStackVersion
type OpenStackVersionStatus struct {
	//+operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	DeployedVersion  *string `json:"deployedVersion,omitempty"`
	AvailableVersion *string `json:"availableVersion,omitempty"`

	// This is the source of truth for the container images to be deployed.
	ContainerImages ContainerImages `json:"containerImages,omitempty"`

	// where we keep track of the container images for previous versions
	ContainerImageVersionDefaults map[string]*ContainerDefaults `json:"containerImageVersionDefaults,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Version"
// +kubebuilder:printcolumn:name="Target Version",type=string,JSONPath=`.spec.targetVersion`
// +kubebuilder:printcolumn:name="Available Version",type=string,JSONPath=`.status.availableVersion`
// +kubebuilder:printcolumn:name="Deployed Version",type=string,JSONPath=`.status.deployedVersion`

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

// IsReady - returns true if service is ready to serve requests
func (instance OpenStackVersion) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}
