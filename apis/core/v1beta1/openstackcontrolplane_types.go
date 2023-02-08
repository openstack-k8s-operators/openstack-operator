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
	corev1 "k8s.io/api/core/v1"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	ovsv1 "github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackControlPlaneSpec defines the desired state of OpenStackControlPlane
type OpenStackControlPlaneSpec struct {

	// +kubebuilder:validation:Required
	// Secret - FIXME: make this optional
	Secret string `json:"secret"`

	// +kubebuilder:validation:Required
	// StorageClass -
	StorageClass string `json:"storageClass"`

	// +kubebuilder:validation:Optional
	// NodeSelector to target subset of worker nodes running control plane services (currently only applies to KeystoneAPI and PlacementAPI)
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// Keystone - Parameters related to the Keystone service
	Keystone KeystoneSection `json:"keystone,omitempty"`

	// +kubebuilder:validation:Optional
	// Placement - Parameters related to the Placement service
	Placement PlacementSection `json:"placement,omitempty"`

	// +kubebuilder:validation:Optional
	// Glance - Parameters related to the Glance service
	Glance GlanceSection `json:"glance,omitempty"`

	// +kubebuilder:validation:Optional
	// Cinder - Parameters related to the Cinder service
	Cinder CinderSection `json:"cinder,omitempty"`

	// +kubebuilder:validation:Optional
	// Mariadb - Parameters related to the Mariadb service
	Mariadb MariadbSection `json:"mariadb,omitempty"`

	// +kubebuilder:validation:Optional
	// Galera - Parameters related to the Galera services
	Galera GaleraSection `json:"galera,omitempty"`

	// +kubebuilder:validation:Optional
	// Rabbitmq - Parameters related to the Rabbitmq service
	Rabbitmq RabbitmqSection `json:"rabbitmq,omitempty"`

	// Ovn - Overrides to use when creating the OVN Services
	Ovn OvnSection `json:"ovn,omitempty"`

	// Ovs - Overrides to use when creating the OVS Services
	Ovs OvsSection `json:"ovs,omitempty"`

	// Neutron - Overrides to use when creating the Neutron Service
	Neutron NeutronSection `json:"neutron,omitempty"`

	// +kubebuilder:validation:Optional
	// Nova - Parameters related to the Nova services
	Nova NovaSection `json:"nova,omitempty"`

	// +kubebuilder:validation:Optional
	// Ironic - Parameters related to the Ironic services
	Ironic IronicSection `json:"ironic,omitempty"`

	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files and credentials that should be provided
	// to the underlying operators.
	// This struct can be defined in the top level CR and propagated to the
	// underlying operators that accept it in their API (e.g., cinder/glance).
	// However, if extraVolumes are specified within the single operator
	// template Section, the globally defined ExtraMounts are ignored and
	// overridden for the operator which has this section already.
	ExtraMounts []OpenStackExtraVolMounts `json:"extraMounts"`
}

// KeystoneSection defines the desired state of Keystone service
type KeystoneSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether Keystone service should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the Keystone service
	Template keystonev1.KeystoneAPISpec `json:"template,omitempty"`
}

// PlacementSection defines the desired state of Placement service
type PlacementSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether Placement service should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the Placement API
	Template placementv1.PlacementAPISpec `json:"template,omitempty"`
}

// GlanceSection defines the desired state of Glance service
type GlanceSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether Glance service should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the Glance Service
	Template glancev1.GlanceSpec `json:"template,omitempty"`
}

// CinderSection defines the desired state of Cinder service
type CinderSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether Cinder service should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating Cinder Resources
	Template cinderv1.CinderSpec `json:"template,omitempty"`
}

// MariadbSection defines the desired state of MariaDB service
type MariadbSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether MariaDB service should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Templates - Overrides to use when creating the MariaDB databases
	Templates map[string]mariadbv1.MariaDBSpec `json:"templates,omitempty"`
}

// GaleraSection defines the desired state of Galera services
type GaleraSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether Galera services should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Templates - Overrides to use when creating the Galera databases
	Templates map[string]mariadbv1.GaleraSpec `json:"templates,omitempty"`
}

// RabbitmqSection defines the desired state of RabbitMQ service
type RabbitmqSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether RabbitMQ services should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Templates - Overrides to use when creating the Rabbitmq clusters
	Templates map[string]RabbitmqTemplate `json:"templates"`
}

// RabbitmqTemplate definition
type RabbitmqTemplate struct {
	// +kubebuilder:validation:Required
	// Overrides to use when creating the Rabbitmq clusters
	rabbitmqv1.RabbitmqClusterSpec `json:",inline"`

	// +kubebuilder:validation:Optional
	// ExternalEndpoint, expose a VIP via MetalLB on the pre-created address pool
	ExternalEndpoint *MetalLBConfig `json:"externalEndpoint"`
}

// MetalLBConfig to configure the MetalLB loadbalancer service
type MetalLBConfig struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// IPAddressPool expose VIP via MetalLB on the IPAddressPool
	IPAddressPool string `json:"ipAddressPool"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// SharedIP if true, VIP/VIPs get shared with multiple services
	SharedIP bool `json:"sharedIP"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:default=""
	// SharedIPKey specifies the sharing key which gets set as the annotation on the LoadBalancer service.
	// Services which share the same VIP must have the same SharedIPKey. Defaults to the IPAddressPool if
	// SharedIP is true, but no SharedIPKey specified.
	SharedIPKey string `json:"sharedIPKey"`

	// +kubebuilder:validation:Optional
	// LoadBalancerIPs, request given IPs from the pool if available. Using a list to allow dual stack (IPv4/IPv6) support
	LoadBalancerIPs []string `json:"loadBalancerIPs"`
}

// OvnSection defines the desired state of OVN services
type OvnSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether OVN services should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the OVN services
	Template OvnResources `json:"template,omitempty"`
}

// OvnResources defines the desired state of OVN services
type OvnResources struct {
	// +kubebuilder:validation:Optional
	// OVNDBCluster - Overrides to use when creating the OVNDBCluster services
	OVNDBCluster map[string]ovnv1.OVNDBClusterSpec `json:"ovnDBCluster,omitempty"`

	// +kubebuilder:validation:Optional
	// OVNNorthd - Overrides to use when creating the OVNNorthd service
	OVNNorthd ovnv1.OVNNorthdSpec `json:"ovnNorthd,omitempty"`
}

// OvsSection defines the desired state of OVS services
type OvsSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether OVS services should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the OVS services
	Template ovsv1.OVSSpec `json:"template,omitempty"`
}

// NeutronSection defines the desired state of Neutron service
type NeutronSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether Neutron service should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the Neutron service
	Template neutronv1.NeutronAPISpec `json:"template,omitempty"`
}

// NovaSection defines the desired state of Nova services
type NovaSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// Enabled - Whether Nova services should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the Nova services
	Template novav1.NovaSpec `json:"template,omitempty"`
}

// IronicSection defines the desired state of Ironic services
type IronicSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Enabled - Whether Ironic services should be deployed and managed
	Enabled bool `json:"enabled,omitempty"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the Ironic services
	Template ironicv1.IronicSpec `json:"template,omitempty"`
}

// OpenStackControlPlaneStatus defines the observed state of OpenStackControlPlane
type OpenStackControlPlaneStatus struct {
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack ControlPlane"
// +kubebuilder:resource:shortName=osctlplane;osctlplanes
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackControlPlane is the Schema for the openstackcontrolplanes API
type OpenStackControlPlane struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackControlPlaneSpec   `json:"spec,omitempty"`
	Status OpenStackControlPlaneStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackControlPlaneList contains a list of OpenStackControlPlane
type OpenStackControlPlaneList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackControlPlane `json:"items"`
}

// OpenStackExtraVolMounts exposes additional parameters processed by the openstack-operator
// and defines the common VolMounts structure provided by the main storage module
type OpenStackExtraVolMounts struct {
	// +kubebuilder:validation:Optional
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Required
	VolMounts []storage.VolMounts `json:"extraVol"`
}

func init() {
	SchemeBuilder.Register(&OpenStackControlPlane{}, &OpenStackControlPlaneList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance OpenStackControlPlane) IsReady() bool {
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
func (instance OpenStackControlPlane) InitConditions() {
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
	}
	cl := condition.CreateList(
		condition.UnknownCondition(OpenStackControlPlaneRabbitMQReadyCondition, condition.InitReason, OpenStackControlPlaneRabbitMQReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneOVNReadyCondition, condition.InitReason, OpenStackControlPlaneOVNReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneOVSReadyCondition, condition.InitReason, OpenStackControlPlaneOVSReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneNeutronReadyCondition, condition.InitReason, OpenStackControlPlaneNeutronReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneMariaDBReadyCondition, condition.InitReason, OpenStackControlPlaneMariaDBReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneKeystoneAPIReadyCondition, condition.InitReason, OpenStackControlPlaneKeystoneAPIReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlanePlacementAPIReadyCondition, condition.InitReason, OpenStackControlPlanePlacementAPIReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneGlanceReadyCondition, condition.InitReason, OpenStackControlPlaneGlanceReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneCinderReadyCondition, condition.InitReason, OpenStackControlPlaneCinderReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneNovaReadyCondition, condition.InitReason, OpenStackControlPlaneNovaReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneIronicReadyCondition, condition.InitReason, OpenStackControlPlaneIronicReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneClientReadyCondition, condition.InitReason, OpenStackControlPlaneClientReadyInitMessage),

		// Also add the overall status condition as Unknown
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
	)
	// initialize conditions used later as Status=Unknown
	instance.Status.Conditions.Init(&cl)
}
