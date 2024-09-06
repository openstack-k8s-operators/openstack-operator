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

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// OpenstackDataPlaneServiceCert defines the property of a TLS cert issued for
// a dataplane service
type OpenstackDataPlaneServiceCert struct {
	// Contents of the certificate
	// This is a list of strings for properties that are needed in the cert
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems:=1
	Contents []string `json:"contents"`

	// Networks to include in SNI for the cert
	// +kubebuilder:validation:Optional
	Networks []infranetworkv1.NetNameStr `json:"networks,omitempty"`

	// Issuer is the label for the issuer to issue the cert
	// Only one issuer should have this label
	// +kubebuilder:validation:Optional
	Issuer string `json:"issuer,omitempty"`

	// KeyUsages to be added to the issued cert
	// +kubebuilder:validation:Optional
	KeyUsages []certmgrv1.KeyUsage `json:"keyUsages,omitempty" yaml:"keyUsages,omitempty"`

	// EDPMRoleServiceName is the value of the <role>_service_name variable from
	// the edpm-ansible role where this certificate is used. For example if the
	// certificate is for edpm_ovn from edpm-ansible, EDPMRoleServiceName must be
	// ovn, which matches the edpm_ovn_service_name variable from the role.  If
	// not set, OpenStackDataPlaneService.Spec.EDPMServiceType is used. If
	// OpenStackDataPlaneService.Spec.EDPMServiceType is not set, then
	// OpenStackDataPlaneService.Name is used.
	EDPMRoleServiceName string `json:"edpmRoleServiceName,omitempty"`
}

// OpenStackDataPlaneServiceSpec defines the desired state of OpenStackDataPlaneService
type OpenStackDataPlaneServiceSpec struct {
	// DataSources list of DataSource objects to mount as ExtraMounts for the
	// OpenStackAnsibleEE
	DataSources []DataSource `json:"dataSources,omitempty" yaml:"dataSources,omitempty"`

	// TLSCerts tls certs to be generated
	// +kubebuilder:validation:Optional
	TLSCerts map[string]OpenstackDataPlaneServiceCert `json:"tlsCerts,omitempty" yaml:"tlsCerts,omitempty"`

	// PlaybookContents is an inline playbook contents that ansible will run on execution.
	PlaybookContents string `json:"playbookContents,omitempty"`

	// Playbook is a path to the playbook that ansible will run on this execution
	Playbook string `json:"playbook,omitempty"`

	// CACerts - Secret containing the CA certificate chain
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxLength:=253
	CACerts string `json:"caCerts,omitempty" yaml:"caCerts,omitempty"`

	// OpenStackAnsibleEERunnerImage image to use as the ansibleEE runner image
	// +kubebuilder:validation:Optional
	OpenStackAnsibleEERunnerImage string `json:"openStackAnsibleEERunnerImage,omitempty" yaml:"openStackAnsibleEERunnerImage,omitempty"`

	// CertsFrom - Service name used to obtain TLSCert and CACerts data. If both
	// CertsFrom and either TLSCert or CACerts is set, then those fields take
	// precedence.
	// +kubebuilder:validation:Optional
	CertsFrom string `json:"certsFrom,omitempty" yaml:"certsFrom,omitempty"`

	// AddCertMounts - Whether to add cert mounts
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=false
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	AddCertMounts bool `json:"addCertMounts" yaml:"addCertMounts"`

	// DeployOnAllNodeSets - should the service be deploy across all nodesets
	// This will override default target of a service play, setting it to 'all'.
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	DeployOnAllNodeSets bool `json:"deployOnAllNodeSets,omitempty" yaml:"deployOnAllNodeSets,omitempty"`

	// ContainerImageFields - list of container image fields names that this
	// service deploys. The field names should match the
	// ContainerImages struct field names from
	// github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1
	ContainerImageFields []string `json:"containerImageFields,omitempty" yaml:"containerImageFields,omitempty"`

	// EDPMServiceType - service type, which typically corresponds to one of
	// the default service names (such as nova, ovn, etc). Also typically
	// corresponds to the ansible role name (without the "edpm_" prefix) used
	// to manage the service. If not set, will default to the
	// OpenStackDataPlaneService name.
	EDPMServiceType string `json:"edpmServiceType,omitempty" yaml:"edpmServiceType,omitempty"`
}

// OpenStackDataPlaneServiceStatus defines the observed state of OpenStackDataPlaneService
type OpenStackDataPlaneServiceStatus struct {
	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=osdps;osdpservice;osdpservices
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Data Plane Service"
// OpenStackDataPlaneService is the Schema for the openstackdataplaneservices API
// OpenStackDataPlaneService name must be a valid RFC1123 as it is used in labels
type OpenStackDataPlaneService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackDataPlaneServiceSpec   `json:"spec,omitempty"`
	Status OpenStackDataPlaneServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// OpenStackDataPlaneServiceList contains a list of OpenStackDataPlaneService
type OpenStackDataPlaneServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackDataPlaneService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackDataPlaneService{}, &OpenStackDataPlaneServiceList{})
}

// DefaultLabel - adding default label to the OpenStackDataPlaneService
func (r *OpenStackDataPlaneService) DefaultLabels() {
	labels := map[string]string{
		"app.kubernetes.io/name":     "openstackdataplaneservice",
		"app.kubernetes.io/instance": r.Name,
		"app.kubernetes.io/part-of":  "openstack-operator",
	}

	if r.Labels == nil {
		r.Labels = labels
	} else {
		for k, v := range labels {
			r.Labels[k] = v
		}
	}
}
