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
	"encoding/json"

	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Container image fall-back defaults

	// OpenStackAnsibleEEContainerImage is the fall-back container image for OpenStackAnsibleEE
	OpenStackAnsibleEEContainerImage = "quay.io/openstack-k8s-operators/openstack-ansibleee-runner:latest"
)

const (
	// JobStatusSucceeded -
	JobStatusSucceeded = "Succeeded"

	// JobStatusFailed -
	JobStatusFailed = "Failed"

	// JobStatusRunning -
	JobStatusRunning = "Running"

	// JobStatusPending -
	JobStatusPending = "Pending"
)

// OpenStackAnsibleEESpec defines the desired state of OpenStackAnsibleEE
type OpenStackAnsibleEESpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// PlaybookContents is an inline playbook contents that ansible will run on execution.
	PlaybookContents string `json:"playbookContents,omitempty"`
	// Playbook is the playbook that ansible will run on this execution, accepts path or FQN from collection
	Playbook string `json:"playbook,omitempty"`
	// Image is the container image that will execute the ansible command
	Image string `json:"image,omitempty"`
	// Args are the command plus the playbook executed by the image. If args is passed, Playbook is ignored.
	Args []string `json:"args,omitempty"`
	// Name is the name of the internal container inside the pod
	// +kubebuilder:default:="openstackansibleee"
	Name string `json:"name,omitempty"`
	// EnvConfigMapName is the name of the k8s config map that contains the ansible env variables
	// +kubebuilder:default:="openstack-aee-default-env"
	EnvConfigMapName string `json:"envConfigMapName,omitempty"`
	// Env is a list containing the environment variables to pass to the pod
	Env []corev1.EnvVar `json:"env,omitempty"`
	// RestartPolicy is the policy applied to the Job on whether it needs to restart the Pod. It can be "OnFailure" or "Never".
	// RestartPolicy default: Never
	// +kubebuilder:validation:Enum:=OnFailure;Never
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:select:OnFailure","urn:alm:descriptor:com.tectonic.ui:select:Never"}
	// +kubebuilder:default:="Never"
	RestartPolicy string `json:"restartPolicy,omitempty"`
	// PreserveJobs - do not delete jobs after they finished e.g. to check logs
	// PreserveJobs default: true
	// +kubebuilder:validation:Enum:=true;false
	// +kubebuilder:default:=true
	PreserveJobs bool `json:"preserveJobs,omitempty"`
	// UID is the userid that will be used to run the container.
	// +kubebuilder:default:=1001
	UID int64 `json:"uid,omitempty"`
	// Inventory is the primary inventory that the ansible playbook will use to launch the job.
	// Further inventories may be provided as ExtraMount in the `/runner/inventory/` path.
	Inventory string `json:"inventory,omitempty"`
	// +kubebuilder:validation:Optional
	// ExtraMounts containing conf files, credentials and inventories
	ExtraMounts []storage.VolMounts `json:"extraMounts,omitempty"`
	// BackoffLimit allows to define the maximum number of retried executions (defaults to 6).
	// +kubebuilder:default:=6
	// +operator-sdk:csv:customresourcedefinitions:type=spec,xDescriptors={"urn:alm:descriptor:com.tectonic.ui:number"}
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`
	// +kubebuilder:validation:Optional
	// NetworkAttachments is a list of NetworkAttachment resource names to expose the services to the given network
	NetworkAttachments []string `json:"networkAttachments,omitempty"`
	// +kubebuilder:validation:Optional
	// CmdLine is the command line passed to ansible-runner
	CmdLine string `json:"cmdLine,omitempty"`
	// +kubebuilder:validation:Optional
	// InitContainers allows the passing of an array of containers that will be executed before the ansibleee execution itself
	InitContainers []corev1.Container `json:"initContainers,omitempty"`
	// +kubebuilder:validation:Optional
	// ServiceAccountName allows to specify what ServiceAccountName do we want the ansible execution run with. Without specifying,
	// it will run with default serviceaccount
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// DNSConfig allows to specify custom dnsservers and search domains
	// +kubebuilder:validation:Optional
	DNSConfig *corev1.PodDNSConfig `json:"dnsConfig,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	// Extra vars to be passed to ansible process during execution. This can be used to override default values in plays.
	ExtraVars map[string]json.RawMessage `json:"extraVars,omitempty"`
}

// OpenStackAnsibleEEStatus defines the observed state of OpenStackAnsibleEE
type OpenStackAnsibleEEStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Map of hashes to track e.g. job status
	Hash map[string]string `json:"hash,omitempty"`

	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// NetworkAttachments status of the deployment pods
	NetworkAttachments map[string][]string `json:"networkAttachments,omitempty"`

	// +kubebuilder:validation:Enum:=Pending;Running;Succeeded;Failed
	// +kubebuilder:default:=Pending
	// JobStatus status of the executed job (Pending/Running/Succeeded/Failed)
	JobStatus string `json:"JobStatus,omitempty" optional:"true"`

	// ObservedGeneration - the most recent generation observed for this
	// service. If the observed generation is less than the spec generation,
	// then the controller has not processed the latest changes injected by
	// the opentack-operator in the top-level CR (e.g. the ContainerImage)
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+operator-sdk:csv:customresourcedefinitions:displayName="OpenStack Ansible EE"
// +kubebuilder:resource:shortName=osaee;osaees;osansible;osansibles
//+kubebuilder:printcolumn:name="NetworkAttachments",type="string",JSONPath=".spec.networkAttachments",description="NetworkAttachments"
//+kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
//+kubebuilder:printcolumn:name="Message",type="string",JSONPath=".status.conditions[0].message",description="Message"

// OpenStackAnsibleEE is the Schema for the openstackansibleees API
type OpenStackAnsibleEE struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackAnsibleEESpec   `json:"spec,omitempty"`
	Status OpenStackAnsibleEEStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackAnsibleEEList contains a list of OpenStackAnsibleEE
type OpenStackAnsibleEEList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStackAnsibleEE `json:"items"`
}

// Config is a specification of where to mount a certain ConfigMap object
type Config struct {
	// Name is the name of the ConfigMap that we want to mount
	Name string `json:"name"`
	// MountPoint is the directory of the container where the ConfigMap will be mounted
	MountPath string `json:"mountpath"`
}

func init() {
	SchemeBuilder.Register(&OpenStackAnsibleEE{}, &OpenStackAnsibleEEList{})
}

// IsReady - returns true if the OpenStackAnsibleEE is ready
func (instance OpenStackAnsibleEE) IsReady() bool {
	return instance.Status.Conditions.IsTrue(condition.JobReadyCondition)
}
