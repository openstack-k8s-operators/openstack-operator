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
	"encoding/json"

	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	corev1 "k8s.io/api/core/v1"
)

// LocalObjectReference contains enough information to let you locate the
// referenced object inside the same namespace.
// +structType=atomic
type LocalObjectReference struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	// TODO: Add other useful fields. apiVersion, kind, uid?
	// +optional
	// +kubebuilder:validation:MaxLength:=253
	Name string `json:"name,omitempty" yaml:"name,omitempty"`
}

// ConfigMapEnvSource selects a ConfigMap to populate the environment
// variables with.
//
// The contents of the target ConfigMap's Data field will represent the
// key-value pairs as environment variables.
type ConfigMapEnvSource struct {
	// The ConfigMap to select from.
	LocalObjectReference `json:",inline" yaml:",inline"`
	// Specify whether the ConfigMap must be defined
	// +optional
	Optional *bool `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// SecretEnvSource selects a Secret to populate the environment
// variables with.
//
// The contents of the target Secret's Data field will represent the
// key-value pairs as environment variables.
type SecretEnvSource struct {
	// The Secret to select from.
	LocalObjectReference `json:",inline" yaml:",inline"`
	// Specify whether the Secret must be defined
	// +optional
	Optional *bool `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// DataSource represents the source of a set of ConfigMaps/Secrets
type DataSource struct {
	// An optional identifier to prepend to each key in the ConfigMap. Must be a C_IDENTIFIER.
	// +optional
	Prefix string `json:"prefix,omitempty" yaml:"prefix,omitempty"`
	// The ConfigMap to select from
	// +optional
	ConfigMapRef *ConfigMapEnvSource `json:"configMapRef,omitempty" yaml:"configMapRef,omitempty"`
	// The Secret to select from
	// +optional
	SecretRef *SecretEnvSource `json:"secretRef,omitempty" yaml:"secretRef,omitempty"`
}

// AnsibleOpts defines a logical grouping of Ansible related configuration options.
type AnsibleOpts struct {
	// AnsibleUser SSH user for Ansible connection
	// +kubebuilder:validation:Optional
	AnsibleUser string `json:"ansibleUser,omitempty"`

	// AnsibleHost SSH host for Ansible connection
	// +kubebuilder:validation:Optional
	AnsibleHost string `json:"ansibleHost,omitempty"`

	// AnsibleVars for configuring ansible
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AnsibleVars map[string]json.RawMessage `json:"ansibleVars,omitempty"`

	// AnsibleVarsFrom is a list of sources to populate ansible variables from.
	// Values defined by an AnsibleVars with a duplicate key take precedence.
	// +kubebuilder:validation:Optional
	AnsibleVarsFrom []DataSource `json:"ansibleVarsFrom,omitempty"`

	// AnsiblePort SSH port for Ansible connection
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	AnsiblePort int `json:"ansiblePort,omitempty"`
}

// NodeSection defines the top level attributes inherited by nodes in the CR.
type NodeSection struct {
	// Networks - Instance networks
	// +kubebuilder:validation:Optional
	Networks []infranetworkv1.IPSetNetwork `json:"networks,omitempty"`

	// +kubebuilder:validation:Optional
	// BmhLabelSelector allows for a sub-selection of BaremetalHosts based on arbitrary labels for a node.
	BmhLabelSelector map[string]string `json:"bmhLabelSelector,omitempty"`

	// UserData  node specific user-data
	// +kubebuilder:validation:Optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// NetworkData  node specific network-data
	// +kubebuilder:validation:Optional
	NetworkData *corev1.SecretReference `json:"networkData,omitempty"`

	// Ansible is the group of Ansible related configuration options.
	// +kubebuilder:validation:Optional
	Ansible AnsibleOpts `json:"ansible,omitempty"`

	// HostName - node name
	// +kubebuilder:validation:Optional
	HostName string `json:"hostName,omitempty"`

	// ManagementNetwork - Name of network to use for management (SSH/Ansible)
	// +kubebuilder:validation:Optional
	ManagementNetwork string `json:"managementNetwork,omitempty"`

	// CtlplaneInterface - Interface on the provisioned nodes to use for ctlplane network
	// +kubebuilder:validation:Optional
	CtlplaneInterface string `json:"ctlplaneInterface,omitempty"`
}

// NodeTemplate is a specification of the node attributes that override top level attributes.
type NodeTemplate struct {
	// ExtraMounts containing files which can be mounted into an Ansible Execution Pod
	// +kubebuilder:validation:Optional
	ExtraMounts []storage.VolMounts `json:"extraMounts,omitempty"`

	// Networks - Instance networks
	// +kubebuilder:validation:Optional
	Networks []infranetworkv1.IPSetNetwork `json:"networks,omitempty"`

	// UserData  node specific user-data
	// +kubebuilder:validation:Optional
	UserData *corev1.SecretReference `json:"userData,omitempty"`

	// NetworkData  node specific network-data
	// +kubebuilder:validation:Optional
	NetworkData *corev1.SecretReference `json:"networkData,omitempty"`

	// AnsibleSSHPrivateKeySecret Name of a private SSH key secret containing
	// private SSH key for connecting to node.
	// The named secret must be of the form:
	// Secret.data.ssh-privatekey: <base64 encoded private key contents>
	// <https://kubernetes.io/docs/concepts/configuration/secret/#ssh-authentication-secrets>
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MaxLength:=253
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:io.kubernetes:Secret"}
	AnsibleSSHPrivateKeySecret string `json:"ansibleSSHPrivateKeySecret"`
	// ManagementNetwork - Name of network to use for management (SSH/Ansible)
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=ctlplane
	ManagementNetwork string `json:"managementNetwork"`

	// Ansible is the group of Ansible related configuration options.
	// +kubebuilder:validation:Optional
	Ansible AnsibleOpts `json:"ansible,omitempty"`
}

// AnsibleEESpec is a specification of the ansible EE attributes
type AnsibleEESpec struct {
	// ExtraMounts containing files which can be mounted into an Ansible Execution Pod
	ExtraMounts []storage.VolMounts `json:"extraMounts,omitempty"`
	// Env is a list containing the environment variables to pass to the pod
	Env []corev1.EnvVar `json:"env,omitempty"`
	// ExtraVars for ansible execution
	ExtraVars map[string]json.RawMessage `json:"extraVars,omitempty"`
	// DNSConfig for setting dnsservers
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`
	// NetworkAttachments is a list of NetworkAttachment resource names to pass to the ansibleee resource
	// which allows to connect the ansibleee runner to the given network
	NetworkAttachments []string `json:"networkAttachments"`
	// OpenStackAnsibleEERunnerImage image to use as the ansibleEE runner image
	OpenStackAnsibleEERunnerImage string `json:"openStackAnsibleEERunnerImage,omitempty"`
	// AnsibleTags for ansible execution
	AnsibleTags string `json:"ansibleTags,omitempty"`
	// AnsibleLimit for ansible execution
	AnsibleLimit string `json:"ansibleLimit,omitempty"`
	// AnsibleSkipTags for ansible execution
	AnsibleSkipTags string `json:"ansibleSkipTags,omitempty"`
	// ServiceAccountName allows to specify what ServiceAccountName do we want
	// the ansible execution run with. Without specifying, it will run with
	// default serviceaccount
	ServiceAccountName string
	// AnsibleEEEnvConfigMapName is the name of ConfigMap containing environment
	// variables to inject to the Ansible execution environment pod.
	AnsibleEEEnvConfigMapName string `json:"ansibleEEEnvConfigMapName,omitempty"`
}
