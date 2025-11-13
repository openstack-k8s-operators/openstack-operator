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
	"context"
	"reflect"
	"regexp"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	ContainerTemplate    `json:",inline"`
	OctaviaApacheImage   *string `json:"octaviaApacheImage,omitempty"`   // gets set to ApacheImage once applied
	CeilometerProxyImage *string `json:"ceilometerProxyImage,omitempty"` // gets set to ApacheImage once applied
	// CinderVolumeImages custom Cinder Volume images for each backend (default Cinder volume image is stored 'default' key)
	// TODO: add validation to cinder-operator to prevent backend being named 'default'
	CinderVolumeImages map[string]*string `json:"cinderVolumeImages,omitempty"`
	// ManilaShareImages custom Manila Share images for each backend (default Manila share image is stored 'default' key)
	// TODO: add validation to manila-operator to prevent backend being named 'default'
	ManilaShareImages map[string]*string `json:"manilaShareImages,omitempty"`
}

// ContainerTemplate - struct that contains container image URLs for each service in OpenStackControlplane
type ContainerTemplate struct {
	AgentImage         *string `json:"agentImage,omitempty"`
	AnsibleeeImage     *string `json:"ansibleeeImage,omitempty"`
	AodhAPIImage       *string `json:"aodhAPIImage,omitempty"`
	AodhEvaluatorImage *string `json:"aodhEvaluatorImage,omitempty"`
	AodhListenerImage  *string `json:"aodhListenerImage,omitempty"`
	AodhNotifierImage  *string `json:"aodhNotifierImage,omitempty"`
	// this is shared by BaremetalOperator, OctaviaOperator, and TelemetryOperator
	ApacheImage                   *string `json:"apacheImage,omitempty"`
	BarbicanAPIImage              *string `json:"barbicanAPIImage,omitempty"`
	BarbicanKeystoneListenerImage *string `json:"barbicanKeystoneListenerImage,omitempty"`
	BarbicanWorkerImage           *string `json:"barbicanWorkerImage,omitempty"`
	CeilometerCentralImage        *string `json:"ceilometerCentralImage,omitempty"`
	CeilometerComputeImage        *string `json:"ceilometerComputeImage,omitempty"`
	CeilometerIpmiImage           *string `json:"ceilometerIpmiImage,omitempty"`
	CeilometerNotificationImage   *string `json:"ceilometerNotificationImage,omitempty"`
	CeilometerSgcoreImage         *string `json:"ceilometerSgcoreImage,omitempty"`
	CeilometerMysqldExporterImage *string `json:"ceilometerMysqldExporterImage,omitempty"`
	CinderAPIImage                *string `json:"cinderAPIImage,omitempty"`
	CinderBackupImage             *string `json:"cinderBackupImage,omitempty"`
	CinderSchedulerImage          *string `json:"cinderSchedulerImage,omitempty"`
	CloudKittyAPIImage            *string `json:"cloudkittyAPIImage,omitempty"`
	CloudKittyProcImage           *string `json:"cloudkittyProcImage,omitempty"`
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
	EdpmNeutronDhcpAgentImage     *string `json:"edpmNeutronDhcpAgentImage,omitempty"`
	EdpmNeutronMetadataAgentImage *string `json:"edpmNeutronMetadataAgentImage,omitempty"`
	EdpmNeutronOvnAgentImage      *string `json:"edpmNeutronOvnAgentImage,omitempty"`
	EdpmNeutronSriovAgentImage    *string `json:"edpmNeutronSriovAgentImage,omitempty"`
	EdpmOvnBgpAgentImage          *string `json:"edpmOvnBgpAgentImage,omitempty"`
	EdpmNodeExporterImage         *string `json:"edpmNodeExporterImage,omitempty"`
	EdpmKeplerImage               *string `json:"edpmKeplerImage,omitempty"`
	EdpmPodmanExporterImage       *string `json:"edpmPodmanExporterImage,omitempty"`
	// Deprecated: Use OpenstackNetworkExporterImage instead
	EdpmOpenstackNetworkExporterImage *string `json:"edpmOpenstackNetworkExporterImage,omitempty"`
	OpenstackNetworkExporterImage     *string `json:"openstackNetworkExporterImage,omitempty"`
	GlanceAPIImage                    *string `json:"glanceAPIImage,omitempty"`
	HeatAPIImage                      *string `json:"heatAPIImage,omitempty"`
	HeatCfnapiImage                   *string `json:"heatCfnapiImage,omitempty"`
	HeatEngineImage                   *string `json:"heatEngineImage,omitempty"`
	HorizonImage                      *string `json:"horizonImage,omitempty"`
	InfraDnsmasqImage                 *string `json:"infraDnsmasqImage,omitempty"`
	InfraMemcachedImage               *string `json:"infraMemcachedImage,omitempty"`
	InfraRedisImage                   *string `json:"infraRedisImage,omitempty"`
	IronicAPIImage                    *string `json:"ironicAPIImage,omitempty"`
	IronicConductorImage              *string `json:"ironicConductorImage,omitempty"`
	IronicInspectorImage              *string `json:"ironicInspectorImage,omitempty"`
	IronicNeutronAgentImage           *string `json:"ironicNeutronAgentImage,omitempty"`
	IronicPxeImage                    *string `json:"ironicPxeImage,omitempty"`
	IronicPythonAgentImage            *string `json:"ironicPythonAgentImage,omitempty"`
	KeystoneAPIImage                  *string `json:"keystoneAPIImage,omitempty"`
	KsmImage                          *string `json:"ksmImage,omitempty"`
	ManilaAPIImage                    *string `json:"manilaAPIImage,omitempty"`
	ManilaSchedulerImage              *string `json:"manilaSchedulerImage,omitempty"`
	MariadbImage                      *string `json:"mariadbImage,omitempty"`
	NetUtilsImage                     *string `json:"netUtilsImage,omitempty"`
	NeutronAPIImage                   *string `json:"neutronAPIImage,omitempty"`
	NovaAPIImage                      *string `json:"novaAPIImage,omitempty"`
	NovaComputeImage                  *string `json:"novaComputeImage,omitempty"`
	NovaConductorImage                *string `json:"novaConductorImage,omitempty"`
	NovaNovncImage                    *string `json:"novaNovncImage,omitempty"`
	NovaSchedulerImage                *string `json:"novaSchedulerImage,omitempty"`
	OctaviaAPIImage                   *string `json:"octaviaAPIImage,omitempty"`
	OctaviaHealthmanagerImage         *string `json:"octaviaHealthmanagerImage,omitempty"`
	OctaviaHousekeepingImage          *string `json:"octaviaHousekeepingImage,omitempty"`
	OctaviaWorkerImage                *string `json:"octaviaWorkerImage,omitempty"`
	OctaviaRsyslogImage               *string `json:"octaviaRsyslogImage,omitempty"`
	OpenstackClientImage              *string `json:"openstackClientImage,omitempty"`
	OsContainerImage                  *string `json:"osContainerImage,omitempty"` //fixme wire this in?
	OvnControllerImage                *string `json:"ovnControllerImage,omitempty"`
	OvnControllerOvsImage             *string `json:"ovnControllerOvsImage,omitempty"`
	OvnNbDbclusterImage               *string `json:"ovnNbDbclusterImage,omitempty"`
	OvnNorthdImage                    *string `json:"ovnNorthdImage,omitempty"`
	OvnSbDbclusterImage               *string `json:"ovnSbDbclusterImage,omitempty"`
	PlacementAPIImage                 *string `json:"placementAPIImage,omitempty"`
	RabbitmqImage                     *string `json:"rabbitmqImage,omitempty"`
	SwiftAccountImage                 *string `json:"swiftAccountImage,omitempty"`
	SwiftContainerImage               *string `json:"swiftContainerImage,omitempty"`
	SwiftObjectImage                  *string `json:"swiftObjectImage,omitempty"`
	SwiftProxyImage                   *string `json:"swiftProxyImage,omitempty"`
	TelemetryNodeExporterImage        *string `json:"telemetryNodeExporterImage,omitempty"`
	TestTempestImage                  *string `json:"testTempestImage,omitempty"`
	TestTobikoImage                   *string `json:"testTobikoImage,omitempty"`
	TestHorizontestImage              *string `json:"testHorizontestImage,omitempty"`
	TestAnsibletestImage              *string `json:"testAnsibletestImage,omitempty"`
	WatcherAPIImage                   *string `json:"watcherAPIImage,omitempty"`
	WatcherApplierImage               *string `json:"watcherApplierImage,omitempty"`
	WatcherDecisionEngineImage        *string `json:"watcherDecisionEngineImage,omitempty"`
}

// ServiceDefaults - struct that contains defaults for OSP services that can change over time
// but are associated with a specific OpenStack release version
type ServiceDefaults struct {
	GlanceWsgi *string `json:"glanceWsgi,omitempty"`
}

// OpenStackVersionStatus defines the observed state of OpenStackVersion
type OpenStackVersionStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	DeployedVersion  *string `json:"deployedVersion,omitempty"`
	AvailableVersion *string `json:"availableVersion,omitempty"`

	// This is the source of truth for the container images to be deployed.
	ContainerImages ContainerImages `json:"containerImages,omitempty"`

	// where we keep track of the container images for previous versions
	ContainerImageVersionDefaults map[string]*ContainerDefaults `json:"containerImageVersionDefaults,omitempty"`

	// AvailableServiceDefaults - struct that contains defaults for OSP services for each discovered available version
	AvailableServiceDefaults map[string]*ServiceDefaults `json:"availableServiceDefaults,omitempty"`

	// ServiceDefaults - struct that contains current defaults for OSP services
	ServiceDefaults ServiceDefaults `json:"serviceDefaults,omitempty"`

	// TrackedCustomImages tracks CustomContainerImages used for each version to detect changes
	TrackedCustomImages map[string]CustomContainerImages `json:"trackedCustomImages,omitempty"`

	//ObservedGeneration - the most recent generation observed for this object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osv;osvs
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

// +kubebuilder:object:root=true
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

func getOpenStackReleaseVersion(openstackReleaseVersion string, releaseVersionScheme string, operatorConditionName string) string {

	/* NOTE: dprince
	 * The releaseVersionScheme can be optionally used to enable 'csvEpochAppend' behavior
	 * This is used by some downstream builds to append the version to the CSV version suffix (epoch) which
	 * is obtained from the OPERATOR_CONDITION_NAME environment variable to the openstack release version.
	 * In some downstream build systems CSV version bumps can be easily automated where as the
	 * OPENSTACK_RELEASE_VERSION is more of a static constant.
	 * The reason we don't always use the CSV version is that the raw version in those same downstream
	 * builds does not match the OpenStackVersion (for example CSV version is 1.0 where as OpenStack product
	 * version is set to 18.0, go figure!)
	 */
	if releaseVersionScheme == "csvEpochAppend" {
		re := regexp.MustCompile(`\.[[:digit:]]{10,}\.p`)
		operatorConditionEpoch := re.FindString(operatorConditionName)
		if operatorConditionEpoch == "" {
			return openstackReleaseVersion
		} else {
			return openstackReleaseVersion + operatorConditionEpoch
		}
	}
	return openstackReleaseVersion
}

// GetOpenStackReleaseVersion - returns the OpenStack release version
func GetOpenStackReleaseVersion(envVars []string) string {

	return getOpenStackReleaseVersion(
		util.GetEnvVar("OPENSTACK_RELEASE_VERSION", ""),
		// can be set to csvEpochAppend
		util.GetEnvVar("OPENSTACK_RELEASE_VERSION_SCHEME", ""),
		// NOTE: dprince this is essentially the CSV version. OLM sets this to provide
		// a way for the controller-manager to know the operator condition name
		// we do it this way to *avoid requiring the CSV structs in this operator*
		util.GetEnvVar("OPERATOR_CONDITION_NAME", ""),
	)

}

// GetOpenStackVersions - returns the OpenStackVersion resource(s) associated with the namespace
func GetOpenStackVersions(namespace string, k8sClient client.Client) (*OpenStackVersionList, error) {
	versionList := &OpenStackVersionList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if err := k8sClient.List(context.TODO(), versionList, listOpts...); err != nil {
		return nil, err
	}

	return versionList, nil
}

// isContainerTemplateEmpty checks if all fields in a ContainerTemplate are nil
func isContainerTemplateEmpty(ct ContainerTemplate) bool {
	v := reflect.ValueOf(ct)
	numFields := v.NumField()
	for i := 0; i < numFields; i++ {
		field := v.Field(i)
		// Check if field is a pointer and not nil
		if field.Kind() == reflect.Ptr && !field.IsNil() {
			return false
		}
	}
	return true
}

// customContainerImagesModified compares two CustomContainerImages and returns true if they are different
func customContainerImagesAllModified(a, b CustomContainerImages) bool {
	if !containerTemplateEqual(a.ContainerTemplate, b.ContainerTemplate) {
		return true
	}

	if !stringMapEqual(a.CinderVolumeImages, b.CinderVolumeImages) {
		return true
	}

	if !stringMapEqual(a.ManilaShareImages, b.ManilaShareImages) {
		return true
	}

	// If all fields are equal, return false (not modified)
	return false
}

// containerTemplateEqual compares two ContainerTemplate structs for equality using reflection
func containerTemplateEqual(a, b ContainerTemplate) bool {
	va := reflect.ValueOf(a)
	vb := reflect.ValueOf(b)

	numFields := va.NumField()
	for i := 0; i < numFields; i++ {
		fieldA := va.Field(i)
		fieldB := vb.Field(i)

		// Both fields should be *string type
		if fieldA.Kind() != reflect.Ptr || fieldB.Kind() != reflect.Ptr {
			continue
		}

		if fieldA.IsNil() && fieldB.IsNil() {
			continue
		}
		if fieldA.IsNil() || fieldB.IsNil() {
			return false
		}
		if fieldA.Elem().String() != fieldB.Elem().String() {
			return false
		}
	}

	return true
}

// stringPtrEqual compares two string pointers for equality
func stringPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// stringMapEqual compares two string maps for equality
func stringMapEqual(a, b map[string]*string) bool {
	if len(a) != len(b) {
		return false
	}

	for key, valueA := range a {
		valueB, exists := b[key]
		if !exists || !stringPtrEqual(valueA, valueB) {
			return false
		}
	}

	return true
}
