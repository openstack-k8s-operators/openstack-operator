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
	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/route"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	rabbitmqv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Container image fall-back defaults

	// RabbitMqContainerImage is the fall-back container image for RabbitMQ
	RabbitMqContainerImage = "quay.io/podified-antelope-centos9/openstack-rabbitmq:current-podified"

	// OvnDbCaName -
	OvnDbCaName = tls.DefaultCAPrefix + "ovn"
	// LibvirtCaName -
	LibvirtCaName = tls.DefaultCAPrefix + "libvirt"
)

// OpenStackControlPlaneSpec defines the desired state of OpenStackControlPlane
type OpenStackControlPlaneSpec struct {

	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Secret"}
	// Secret - FIXME: make this optional
	Secret string `json:"secret"`

	// +kubebuilder:validation:Required
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:StorageClass"}
	// StorageClass -
	StorageClass string `json:"storageClass"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// NodeSelector to target subset of worker nodes running control plane services (currently only applies to KeystoneAPI and PlacementAPI)
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={ingress: {enabled: true, ca: {duration: "87600h"}, cert: {duration: "43800h"}}, podLevel: {enabled: true, internal:{ca: {duration: "87600h"}, cert: {duration: "43800h"}}, libvirt: {ca: {duration: "87600h"}, cert: {duration: "43800h"}}, ovn: {ca: {duration: "87600h"}, cert: {duration: "43800h"}}}}
	// TLS - Parameters related to the TLS
	TLS TLSSection `json:"tls"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// DNS - Parameters related to the DNSMasq service
	DNS DNSMasqSection `json:"dns,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Keystone - Parameters related to the Keystone service
	Keystone KeystoneSection `json:"keystone,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Placement - Parameters related to the Placement service
	Placement PlacementSection `json:"placement,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Glance - Parameters related to the Glance service
	Glance GlanceSection `json:"glance,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Cinder - Parameters related to the Cinder service
	Cinder CinderSection `json:"cinder,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Galera - Parameters related to the Galera services
	Galera GaleraSection `json:"galera,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Rabbitmq - Parameters related to the Rabbitmq service
	Rabbitmq RabbitmqSection `json:"rabbitmq,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Memcached - Parameters related to the Memcached service
	Memcached MemcachedSection `json:"memcached,omitempty"`

	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Ovn - Overrides to use when creating the OVN Services
	Ovn OvnSection `json:"ovn,omitempty"`

	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Neutron - Overrides to use when creating the Neutron Service
	Neutron NeutronSection `json:"neutron,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Nova - Parameters related to the Nova services
	Nova NovaSection `json:"nova,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Heat - Parameters related to the Heat services
	Heat HeatSection `json:"heat,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Ironic - Parameters related to the Ironic services
	Ironic IronicSection `json:"ironic,omitempty"`

	// Manila - Parameters related to the Manila service
	Manila ManilaSection `json:"manila,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Horizon - Parameters related to the Horizon services
	Horizon HorizonSection `json:"horizon,omitempty"`

	// +kubebuilder:validation:Optional
	// Telemetry - Parameters related to the OpenStack Telemetry services
	Telemetry TelemetrySection `json:"telemetry,omitempty"`

	// Swift - Parameters related to the Swift service
	Swift SwiftSection `json:"swift,omitempty"`

	// Octavia - Parameters related to the Octavia service
	Octavia OctaviaSection `json:"octavia,omitempty"`

	// Designate - Parameters related to the Designate service
	Designate DesignateSection `json:"designate,omitempty"`

	// Barbican - Parameters related to the Barbican service
	Barbican BarbicanSection `json:"barbican,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// OpenStackClient - Parameters related to the OpenStackClient
	OpenStackClient OpenStackClientSection `json:"openstackclient,omitempty"`

	// ExtraMounts containing conf files and credentials that should be provided
	// to the underlying operators.
	// This struct can be defined in the top level CR and propagated to the
	// underlying operators that accept it in their API (e.g., cinder/glance).
	// However, if extraVolumes are specified within the single operator
	// template Section, the globally defined ExtraMounts are ignored and
	// overridden for the operator which has this section already.
	ExtraMounts []OpenStackExtraVolMounts `json:"extraMounts,omitempty"`
}

// TLSSection defines the desired state of TLS configuration
type TLSSection struct {
	// +kubebuilder:validation:optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={enabled: true, ca: {duration: "87600h"}, cert: {duration: "43800h"}}
	Ingress TLSIngressConfig `json:"ingress,omitempty"`

	// +kubebuilder:validation:optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={enabled: true, internal:{ca: {duration: "87600h"}, cert: {duration: "43800h"}}, libvirt: {ca: {duration: "87600h"}, cert: {duration: "43800h"}}, ovn: {ca: {duration: "87600h"}, cert: {duration: "43800h"}}}
	PodLevel TLSPodLevelConfig `json:"podLevel,omitempty"`

	// +kubebuilder:validation:optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Secret containing any additional CA certificates, which should be added to deployment pods.
	// If services get configured to use a custom cert/key, add the CA cert to validate those in this
	// CA secret.
	tls.Ca `json:",inline"`
}

// TLSIngressConfig defines the desired state of the TLS configuration for the ingress configuration (route)
type TLSIngressConfig struct {
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether TLS should be enabled for endpoint type
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	CertSection `json:",inline"`
}

// TLSPodLevelConfig defines the desired state of the TLS configuration for TLS termination at the pod level
type TLSPodLevelConfig struct {
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether TLS should be enabled for endpoint type
	// +kubebuilder:default=true
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={ca: {duration: "87600h"}, cert: {duration: "43800h"}}
	// Internal - default CA used for all OpenStackControlPlane and OpenStackDataplane endpoints,
	// except OVN related CA and certs
	Internal CertSection `json:"internal"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={ca: {duration: "87600h"}, cert: {duration: "43800h"}}
	// Libvirt - CA used for libvirt/qemu services on OpenStackControlPlane and OpenStackDataplane
	Libvirt CertSection `json:"libvirt"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={ca: {duration: "87600h"}, cert: {duration: "43800h"}}
	// Ovn - CA used for all OVN services on OpenStackControlPlane and OpenStackDataplane
	Ovn CertSection `json:"ovn"`
}

// CertSection defines details for CA config and its certs
type CertSection struct {
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={duration: "43800h"}
	// Cert - defines details for cert config
	Cert CertConfig `json:"cert"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// +kubebuilder:default={duration: "87600h"}
	// Ca - defines details for CA cert config
	Ca CACertConfig `json:"ca"`
}

// CACertConfig defines details for ca cert configs
type CACertConfig struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="87600h"
	// The requested 'duration' (i.e. lifetime) of the Certificate.
	// The Certificate will be renewed either 2/3 through its duration or
	// `renewBefore` period before its expiry, whichever is later. Minimum
	// accepted duration is 1 hour. Value must be in units accepted by Go
	// time.ParseDuration https://golang.org/pkg/time/#ParseDuration
	Duration *metav1.Duration `json:"duration"`

	// +kubebuilder:validation:Optional
	// How long before the currently issued certificate's expiry
	// cert-manager should renew the certificate. The default is 2/3 of the
	// issued certificate's duration. Minimum accepted value is 5 minutes.
	// Value must be in units accepted by Go time.ParseDuration
	// https://golang.org/pkg/time/#ParseDuration
	RenewBefore *metav1.Duration `json:"renewBefore,omitempty"`

	// +kubebuilder:validation:Optional
	// CustomIssuer - use pre-created issue for this CA. No CA and issure is being created
	// the CA cert and chain needs to be added using the CaBundleSecretName.
	CustomIssuer *string `json:"customIssuer,omitempty"`
}

// CertConfig defines details for cert configs
type CertConfig struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="43800h"
	// The requested 'duration' (i.e. lifetime) of the Certificate.
	// The Certificate will be renewed either 2/3 through its duration or
	// `renewBefore` period before its expiry, whichever is later. Minimum
	// accepted duration is 1 hour. Value must be in units accepted by Go
	// time.ParseDuration https://golang.org/pkg/time/#ParseDuration
	Duration *metav1.Duration `json:"duration"`

	// +kubebuilder:validation:Optional
	// How long before the currently issued certificate's expiry
	// cert-manager should renew the certificate. The default is 2/3 of the
	// issued certificate's duration. Minimum accepted value is 5 minutes.
	// Value must be in units accepted by Go time.ParseDuration
	// https://golang.org/pkg/time/#ParseDuration
	RenewBefore *metav1.Duration `json:"renewBefore,omitempty"`
}

// DNSMasqSection defines the desired state of DNSMasq service
type DNSMasqSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether DNSMasq service should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the DNSMasq service
	Template networkv1.DNSMasqSpec `json:"template,omitempty"`
}

// KeystoneSection defines the desired state of Keystone service
type KeystoneSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Keystone service should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Keystone service
	Template keystonev1.KeystoneAPISpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// Override to override the generated manifest of several child resources.
type Override struct {
	// +kubebuilder:validation:Optional
	// Route overrides to use when creating the public service endpoint
	Route *route.OverrideSpec `json:"route,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - overrides tls parameters for public endpoint
	TLS *TLSServiceOverride `json:"tls,omitempty"`
}

// TLSServiceOverride overrides tls parameters for public endpoint
type TLSServiceOverride struct {
	// +kubebuilder:validation:Optional
	// Name of a Secret in the same Namespace as the service, containing the server's private key, public certificate
	// and CA certificate for TLS.
	// The Secret must store these as tls.key, tls.crt and ca.crt respectively.
	SecretName string `json:"secretName,omitempty"`
}

// PlacementSection defines the desired state of Placement service
type PlacementSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Placement service should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Placement API
	Template placementv1.PlacementAPISpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// GlanceSection defines the desired state of Glance service
type GlanceSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Glance service should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Glance Service
	Template glancev1.GlanceSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride map[string]Override `json:"apiOverrides,omitempty"`
}

// CinderSection defines the desired state of Cinder service
type CinderSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Cinder service should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating Cinder Resources
	Template cinderv1.CinderSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// GaleraSection defines the desired state of Galera services
type GaleraSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Galera services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Templates - Overrides to use when creating the Galera databases
	Templates map[string]mariadbv1.GaleraSpecCore `json:"templates,omitempty"`
}

// RabbitmqSection defines the desired state of RabbitMQ service
type RabbitmqSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether RabbitMQ services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Templates - Overrides to use when creating the Rabbitmq clusters
	Templates map[string]RabbitmqTemplate `json:"templates"`
}

// MemcachedSection defines the desired state of Memcached services
type MemcachedSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Memcached services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Templates - Overrides to use when creating the Memcached databases
	Templates map[string]memcachedv1.MemcachedSpecCore `json:"templates,omitempty"`
}

// RabbitmqTemplate definition
type RabbitmqTemplate struct {
	// +kubebuilder:validation:Required
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Overrides to use when creating the Rabbitmq clusters
	rabbitmqv2.RabbitmqClusterSpecCore `json:",inline"`
}

// OvnSection defines the desired state of OVN services
type OvnSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether OVN services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the OVN services
	Template OvnResources `json:"template,omitempty"`
}

// OvnResources defines the desired state of OVN services
type OvnResources struct {
	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// OVNDBCluster - Overrides to use when creating the OVNDBCluster services
	OVNDBCluster map[string]ovnv1.OVNDBClusterSpecCore `json:"ovnDBCluster,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// OVNNorthd - Overrides to use when creating the OVNNorthd service
	OVNNorthd ovnv1.OVNNorthdSpecCore `json:"ovnNorthd,omitempty"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// OVNController - Overrides to use when creating the OVNController service
	OVNController ovnv1.OVNControllerSpecCore `json:"ovnController,omitempty"`
}

// NeutronSection defines the desired state of Neutron service
type NeutronSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Neutron service should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Neutron Service
	Template neutronv1.NeutronAPISpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// NovaSection defines the desired state of Nova services
type NovaSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Nova services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Nova services
	Template novav1.NovaSpec `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// CellOverride, provides the ability to override the generated manifest of several child resources
	// for a nova cell. cell0 never have compute nodes and therefore it won't have a noVNCProxy deployed.
	// Providing an override for cell0 noVNCProxy does not have an effect.
	CellOverride map[string]NovaCellOverrideSpec `json:"cellOverride,omitempty"`
}

// NovaCellOverrideSpec to override the generated manifest of several child resources.
type NovaCellOverrideSpec struct {
	// +kubebuilder:validation:Optional
	NoVNCProxy Override `json:"noVNCProxy,omitempty"`
}

// HeatSection defines the desired state of Heat services
type HeatSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Heat services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Heat services
	Template heatv1.HeatSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// CnfAPIOverride, provides the ability to override the generated manifest of several child resources.
	CnfAPIOverride Override `json:"cnfAPIOverride,omitempty"`
}

// IronicSection defines the desired state of Ironic services
type IronicSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Ironic services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Ironic services
	Template ironicv1.IronicSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// InspectorOverride, provides the ability to override the generated manifest of several child resources.
	InspectorOverride Override `json:"inspectorOverride,omitempty"`
}

// ManilaSection defines the desired state of Manila service
type ManilaSection struct {
	// +kubebuilder:validation:Optional
	// Enabled - Whether Manila service should be deployed and managed
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating Manila Resources
	Template manilav1.ManilaSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// HorizonSection defines the desired state of Horizon services
type HorizonSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// Enabled - Whether Horizon services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	// Template - Overrides to use when creating the Horizon services
	Template horizonv1.HorizonSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// TelemetrySection defines the desired state of OpenStack Telemetry services
type TelemetrySection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether OpenStack Telemetry services should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the OpenStack Telemetry services
	Template telemetryv1.TelemetrySpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// AodhAPIOverride, provides the ability to override the generated manifest of several child resources.
	AodhAPIOverride Override `json:"aodhApiOverride,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// PrometheusOverride, provides the ability to override the generated manifest of several child resources.
	PrometheusOverride Override `json:"prometheusOverride,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// AlertmanagerOverride, provides the ability to override the generated manifest of several child resources.
	AlertmanagerOverride Override `json:"alertmanagerOverride,omitempty"`
}

// SwiftSection defines the desired state of Swift service
type SwiftSection struct {
	// +kubebuilder:validation:Optional
	// Enabled - Whether Swift service should be deployed and managed
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating Swift Resources
	Template swiftv1.SwiftSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// ProxyOverride, provides the ability to override the generated manifest of several child resources.
	ProxyOverride Override `json:"proxyOverride,omitempty"`
}

// OctaviaSection defines the desired state of the Octavia service
type OctaviaSection struct {
	// +kubebuilder:validation:Optional
	// Enabled - Whether the Octavia service should be deployed and managed
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// +kubebuilder:valdiation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating Octavia Resources
	Template octaviav1.OctaviaSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// DesignateSection defines the desired state of the Designate service
type DesignateSection struct {
	// +kubebuilder:validation:Optional
	// Enabled - Whether the Designate service should be deployed and managed
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	Enabled bool `json:"enabled"`

	// +kubebuilder:valdiation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating Designate Resources
	Template designatev1.DesignateSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// BarbicanSection defines the desired state of Barbican service
type BarbicanSection struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=true
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Enabled - Whether Barbican service should be deployed and managed
	Enabled bool `json:"enabled"`

	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the Barbican Service
	Template barbicanv1.BarbicanSpecCore `json:"template,omitempty"`

	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// APIOverride, provides the ability to override the generated manifest of several child resources.
	APIOverride Override `json:"apiOverride,omitempty"`
}

// OpenStackClientSection defines the desired state of the OpenStackClient
type OpenStackClientSection struct {
	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	// Template - Overrides to use when creating the OpenStackClient Resource
	Template v1beta1.OpenStackClientSpec `json:"template,omitempty"`
}

// OpenStackControlPlaneStatus defines the observed state of OpenStackControlPlane
type OpenStackControlPlaneStatus struct {
	//+operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	//+operator-sdk:csv:customresourcedefinitions:type=status
	// TLS
	TLS TLSStatus `json:"tls,omitempty" optional:"true"`

	//+operator-sdk:csv:customresourcedefinitions:type=status
	// DeployedVersion
	DeployedVersion *string `json:"deployedVersion,omitempty"`

	//+operator-sdk:csv:customresourcedefinitions:type=status
	// DeployedOVNVersion
	DeployedOVNVersion *string `json:"deployedOVNVersion,omitempty"`

	// ContainerImages
	ContainerImages ContainerImages `json:"containerImages,omitempty"`

	//ObservedGeneration - the most recent generation observed for this object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// TLSStatus defines the observed state of TLS
type TLSStatus struct {
	CAList []TLSCAStatus `json:"caList,omitempty"`
	tls.Ca `json:",inline"`
}

// TLSCAStatus defines the observed state of TLS
type TLSCAStatus struct {
	Name    string `json:"name"`
	Expires string `json:"expires"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack ControlPlane"
// +kubebuilder:resource:shortName=osctlplane;osctlplanes;oscp;oscps
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
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	Name string `json:"name,omitempty"`
	// +kubebuilder:validation:Optional
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	Region string `json:"region,omitempty"`
	// +kubebuilder:validation:Required
	//+operator-sdk:csv:customresourcedefinitions:type=spec
	VolMounts []storage.VolMounts `json:"extraVol"`
}

func init() {
	SchemeBuilder.Register(&OpenStackControlPlane{}, &OpenStackControlPlaneList{})
}

// IsReady - returns true if service is ready to serve requests
func (instance OpenStackControlPlane) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

// InitConditions - Initializes Status Conditons
func (instance *OpenStackControlPlane) InitConditions() {
	if instance.Status.Conditions == nil {
		instance.Status.Conditions = condition.Conditions{}
	}
	cl := condition.CreateList(
		condition.UnknownCondition(OpenStackControlPlaneRabbitMQReadyCondition, condition.InitReason, OpenStackControlPlaneRabbitMQReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneOVNReadyCondition, condition.InitReason, OpenStackControlPlaneOVNReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneNeutronReadyCondition, condition.InitReason, OpenStackControlPlaneNeutronReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneMariaDBReadyCondition, condition.InitReason, OpenStackControlPlaneMariaDBReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneMemcachedReadyCondition, condition.InitReason, OpenStackControlPlaneMemcachedReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneKeystoneAPIReadyCondition, condition.InitReason, OpenStackControlPlaneKeystoneAPIReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlanePlacementAPIReadyCondition, condition.InitReason, OpenStackControlPlanePlacementAPIReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneGlanceReadyCondition, condition.InitReason, OpenStackControlPlaneGlanceReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneCinderReadyCondition, condition.InitReason, OpenStackControlPlaneCinderReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneNovaReadyCondition, condition.InitReason, OpenStackControlPlaneNovaReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneIronicReadyCondition, condition.InitReason, OpenStackControlPlaneIronicReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneClientReadyCondition, condition.InitReason, OpenStackControlPlaneClientReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneManilaReadyCondition, condition.InitReason, OpenStackControlPlaneManilaReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneHorizonReadyCondition, condition.InitReason, OpenStackControlPlaneHorizonReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneDNSReadyCondition, condition.InitReason, OpenStackControlPlaneDNSReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneTelemetryReadyCondition, condition.InitReason, OpenStackControlPlaneTelemetryReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneHeatReadyCondition, condition.InitReason, OpenStackControlPlaneHeatReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneSwiftReadyCondition, condition.InitReason, OpenStackControlPlaneSwiftReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneOctaviaReadyCondition, condition.InitReason, OpenStackControlPlaneOctaviaReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneDesignateReadyCondition, condition.InitReason, OpenStackControlPlaneDesignateReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneBarbicanReadyCondition, condition.InitReason, OpenStackControlPlaneBarbicanReadyInitMessage),
		condition.UnknownCondition(OpenStackControlPlaneCAReadyCondition, condition.InitReason, OpenStackControlPlaneCAReadyInitMessage),

		// Also add the overall status condition as Unknown
		condition.UnknownCondition(condition.ReadyCondition, condition.InitReason, condition.ReadyInitMessage),
	)
	// initialize conditions used later as Status=Unknown
	instance.Status.Conditions.Init(&cl)
}

// SetupDefaults - initializes any CRD field defaults based on environment variables (the defaulting mechanism itself is implemented via webhooks)
func SetupDefaults() {
	// Acquire environmental defaults and initialize OpenStackControlPlane defaults with them
	openstackControlPlaneDefaults := OpenStackControlPlaneDefaults{
		RabbitMqImageURL: util.GetEnvVar("RELATED_IMAGE_RABBITMQ_IMAGE_URL_DEFAULT", RabbitMqContainerImage),
	}

	SetupOpenStackControlPlaneDefaults(openstackControlPlaneDefaults)
}

// IsCustomIssuer - returns true if CustomIssuer is provided and not empty string
func (ca CACertConfig) IsCustomIssuer() bool {
	return ca.CustomIssuer != nil && *ca.CustomIssuer != ""
}

// GetPublicIssuer - returns the public CA issuer name or custom if configured
func (instance OpenStackControlPlane) GetPublicIssuer() string {
	// use custom issuer if set
	if instance.Spec.TLS.Ingress.Ca.IsCustomIssuer() {
		return *instance.Spec.TLS.Ingress.Ca.CustomIssuer
	}

	return tls.DefaultCAPrefix + string(service.EndpointPublic)
}

// GetInternalIssuer - returns the internal CA issuer name or custom if configured
func (instance OpenStackControlPlane) GetInternalIssuer() string {
	// use custom issuer if set
	if instance.Spec.TLS.PodLevel.Internal.Ca.IsCustomIssuer() {
		return *instance.Spec.TLS.PodLevel.Internal.Ca.CustomIssuer
	}

	return tls.DefaultCAPrefix + string(service.EndpointInternal)
}

// GetOvnIssuer - returns the ovn CA issuer name or custom if configured
func (instance OpenStackControlPlane) GetOvnIssuer() string {
	// use custom issuer if set
	if instance.Spec.TLS.PodLevel.Ovn.Ca.IsCustomIssuer() {
		return *instance.Spec.TLS.PodLevel.Ovn.Ca.CustomIssuer
	}

	return OvnDbCaName
}

// GetLibvirtIssuer - returns the libvirt CA issuer name or custom if configured
func (instance OpenStackControlPlane) GetLibvirtIssuer() string {
	// use custom issuer if set
	if instance.Spec.TLS.PodLevel.Libvirt.Ca.IsCustomIssuer() {
		return *instance.Spec.TLS.PodLevel.Libvirt.Ca.CustomIssuer
	}

	return LibvirtCaName
}
