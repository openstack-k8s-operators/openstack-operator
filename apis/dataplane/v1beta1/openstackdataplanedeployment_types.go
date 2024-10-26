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

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OpenStackDataPlaneDeploymentSpec defines the desired state of OpenStackDataPlaneDeployment
type OpenStackDataPlaneDeploymentSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems:=1
	// NodeSets is the list of NodeSets deployed
	NodeSets []string `json:"nodeSets"`

	// BackoffLimit allows to define the maximum number of retried executions (defaults to 6).
	// +kubebuilder:default:=6
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	// PreserveJobs default: true
	// +kubebuilder:validation:Enum:=true;false
	// +kubebuilder:default:=true
	PreserveJobs bool `json:"preserveJobs,omitempty"`

	// AnsibleTags for ansible execution
	// +kubebuilder:validation:Optional
	AnsibleTags string `json:"ansibleTags,omitempty"`

	// AnsibleLimit for ansible execution
	// +kubebuilder:validation:Optional
	AnsibleLimit string `json:"ansibleLimit,omitempty"`

	// AnsibleSkipTags for ansible execution
	// +kubebuilder:validation:Optional
	AnsibleSkipTags string `json:"ansibleSkipTags,omitempty"`

	// +kubebuilder:validation:Optional
	// AnsibleExtraVars for ansible execution
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	AnsibleExtraVars map[string]json.RawMessage `json:"ansibleExtraVars,omitempty"`

	// +kubebuilder:validation:Optional
	// ServicesOverride list
	ServicesOverride []string `json:"servicesOverride,omitempty"`

	// Time before the deployment is requeued in seconds
	// +kubebuilder:validation:Minimum:=1
	// +kubebuilder:default:=15
	DeploymentRequeueTime int `json:"deploymentRequeueTime"`

	// +kubebuilder:validation:Optional
	// AnsibleJobNodeSelector to target subset of worker nodes running the ansible jobs
	AnsibleJobNodeSelector map[string]string `json:"ansibleJobNodeSelector,omitempty"`
}

// OpenStackDataPlaneDeploymentStatus defines the observed state of OpenStackDataPlaneDeployment
type OpenStackDataPlaneDeploymentStatus struct {
	// NodeSetConditions
	NodeSetConditions map[string]condition.Conditions `json:"nodeSetConditions,omitempty" optional:"true"`

	// AnsibleEEHashes
	AnsibleEEHashes map[string]string `json:"ansibleEEHashes,omitempty" optional:"true"`

	// ConfigMapHashes
	ConfigMapHashes map[string]string `json:"configMapHashes,omitempty" optional:"true"`

	// SecretHashes
	SecretHashes map[string]string `json:"secretHashes,omitempty" optional:"true"`

	// NodeSetHashes
	NodeSetHashes map[string]string `json:"nodeSetHashes,omitempty" optional:"true"`

	// ContainerImages
	ContainerImages map[string]string `json:"containerImages,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	//ObservedGeneration - the most recent generation observed for this Deployment. If the observed generation is less than the spec generation, then the controller has not processed the latest changes.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// DeployedVersion
	DeployedVersion string `json:"deployedVersion,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:booleanSwitch"}
	// Deployed
	Deployed bool `json:"deployed,omitempty" optional:"true"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Data Plane Deployments"
// +kubebuilder:resource:shortName=osdpd;osdpdeployment;osdpdeployments
// +kubebuilder:printcolumn:name="NodeSets",type="string",JSONPath=".spec.nodeSets",description="NodeSets"
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
// +kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackDataPlaneDeployment is the Schema for the openstackdataplanedeployments API
// OpenStackDataPlaneDeployment name must be a valid RFC1123 as it is used in labels
type OpenStackDataPlaneDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="OpenStackDataPlaneDeployment Spec is immutable"
	Spec   OpenStackDataPlaneDeploymentSpec   `json:"spec,omitempty"`
	Status OpenStackDataPlaneDeploymentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OpenStackDataPlaneDeploymentList contains a list of OpenStackDataPlaneDeployment
type OpenStackDataPlaneDeploymentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackDataPlaneDeployment `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStackDataPlaneDeployment{}, &OpenStackDataPlaneDeploymentList{})
}

// IsReady - returns true if the OpenStackDataPlaneDeployment is ready
func (instance OpenStackDataPlaneDeployment) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.ReadyCondition)
}

// InitConditions - Initializes Status Conditons
func (instance *OpenStackDataPlaneDeployment) InitConditions() {
	instance.Status.Conditions = condition.Conditions{}

	cl := condition.CreateList(
		condition.UnknownCondition(condition.DeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage),
		condition.UnknownCondition(condition.InputReadyCondition, condition.InitReason, condition.InputReadyInitMessage),
	)
	instance.Status.Conditions.Init(&cl)
	instance.Status.NodeSetConditions = make(map[string]condition.Conditions)
	if instance.Spec.NodeSets != nil {
		for _, nodeSet := range instance.Spec.NodeSets {
			nsConds := condition.Conditions{}
			nsConds.Set(condition.UnknownCondition(
				NodeSetDeploymentReadyCondition, condition.InitReason, condition.DeploymentReadyInitMessage))
			instance.Status.NodeSetConditions[nodeSet] = nsConds

		}
	}

	instance.Status.Deployed = false
}

// InitHashesAndImages - Initialize ConfigHashes and Images
func (instance *OpenStackDataPlaneDeployment) InitHashesAndImages() {
	if instance.Status.ConfigMapHashes == nil {
		instance.Status.ConfigMapHashes = make(map[string]string)
	}
	if instance.Status.SecretHashes == nil {
		instance.Status.SecretHashes = make(map[string]string)
	}
	if instance.Status.NodeSetHashes == nil {
		instance.Status.NodeSetHashes = make(map[string]string)
	}
	if instance.Status.AnsibleEEHashes == nil {
		instance.Status.AnsibleEEHashes = make(map[string]string)
	}
	if instance.Status.ContainerImages == nil {
		instance.Status.ContainerImages = make(map[string]string)
	}
}
