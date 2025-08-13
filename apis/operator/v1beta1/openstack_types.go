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
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

const (
	OpenStackOperatorName          = "openstack"
	BarbicanOperatorName           = "barbican"
	CinderOperatorName             = "cinder"
	DesignateOperatorName          = "designate"
	GlanceOperatorName             = "glance"
	HeatOperatorName               = "heat"
	HorizonOperatorName            = "horizon"
	InfraOperatorName              = "infra"
	IronicOperatorName             = "ironic"
	KeystoneOperatorName           = "keystone"
	ManilaOperatorName             = "manila"
	MariaDBOperatorName            = "mariadb"
	NeutronOperatorName            = "neutron"
	NovaOperatorName               = "nova"
	OctaviaOperatorName            = "octavia"
	OpenStackBaremetalOperatorName = "openstack-baremetal"
	OvnOperatorName                = "ovn"
	PlacementOperatorName          = "placement"
	RabbitMQOperatorName           = "rabbitmq-cluster"
	SwiftOperatorName              = "swift"
	TelemetryOperatorName          = "telemetry"
	TestOperatorName               = "test"
	// ReplicasEnabled - default replicas count when enabled
	ReplicasEnabled int32 = 1
	// ReplicasDisabled - replicas when disabled
	ReplicasDisabled int32 = 0
)

var (
	// DefaultManagerCPULimit - Default controller manager container CPU limit
	DefaultManagerCPULimit resource.Quantity = resource.MustParse("500m")
	// DefaultManagerCPURequests - Default controller manager container CPU requests
	DefaultManagerCPURequests resource.Quantity = resource.MustParse("10m")
	// DefaultManagerMemoryLimit - Default controller manager container memory limit
	DefaultManagerMemoryLimit resource.Quantity = resource.MustParse("512Mi")
	// DefaultManagerMemoryRequests - Default controller manager container memory requests
	DefaultManagerMemoryRequests resource.Quantity = resource.MustParse("256Mi")
	// DefaultRbacProxyCPULimit - Default kube rbac proxy container CPU limit
	DefaultRbacProxyCPULimit resource.Quantity = resource.MustParse("500m")
	// DefaultRbacProxyCPURequests - Default kube rbac proxy container CPU requests
	DefaultRbacProxyCPURequests resource.Quantity = resource.MustParse("5m")
	// DefaultRbacProxyMemoryLimit - Default kube rbac proxy container memory limit
	DefaultRbacProxyMemoryLimit resource.Quantity = resource.MustParse("128Mi")
	// DefaultRbacProxyMemoryRequests - Default kube rbac proxy container memory requests
	DefaultRbacProxyMemoryRequests resource.Quantity = resource.MustParse("64Mi")

	// DefaultTolerations - Default tolerations for all operators
	DefaultTolerations = []corev1.Toleration{
		{
			Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready"
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
		{
			Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable"
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
	}

	// OperatorList - list of all operators with optional different defaults then the above.
	// NOTE: test-operator was deployed as a independant package so it may or may not be installed
	// NOTE: depending on how watcher-operator is released for FR2 and then in FR3 it may need to be
	// added into this list in the future
	// IMPORTANT: have this list in sync with the Enum in OperatorSpec.Name parameter
	OperatorList []OperatorSpec = []OperatorSpec{
		{
			Name: OpenStackOperatorName,
			ControllerManager: ContainerSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
		},
		{
			Name: BarbicanOperatorName,
		},
		{
			Name: CinderOperatorName,
		},
		{
			Name: DesignateOperatorName,
		},
		{
			Name: GlanceOperatorName,
		},
		{
			Name: HeatOperatorName,
		},
		{
			Name: HorizonOperatorName,
		},
		{
			Name: InfraOperatorName,
			ControllerManager: ContainerSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("512Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceMemory: resource.MustParse("1Gi"),
					},
				},
			},
		},
		{
			Name: IronicOperatorName,
		},
		{
			Name: KeystoneOperatorName,
		},
		{
			Name: ManilaOperatorName,
		},
		{
			Name: MariaDBOperatorName,
		},
		{
			Name: NeutronOperatorName,
		},
		{
			Name: NovaOperatorName,
		},
		{
			Name: OctaviaOperatorName,
		},
		{
			Name: OpenStackBaremetalOperatorName,
		},
		{
			Name: OvnOperatorName,
		},
		{
			Name: PlacementOperatorName,
		},
		{
			Name: RabbitMQOperatorName,
			ControllerManager: ContainerSpec{
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("5m"),
						corev1.ResourceMemory: resource.MustParse("64Mi"),
					},
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse("200m"),
						corev1.ResourceMemory: resource.MustParse("500Mi"),
					},
				},
			},
		},
		{
			Name: SwiftOperatorName,
		},
		{
			Name: TelemetryOperatorName,
		},
		{
			Name: TestOperatorName,
		},
	}
)

// OpenStackSpec defines the desired state of OpenStack
type OpenStackSpec struct {
	// +kubebuilder:validation:Optional
	// +listType=map
	// +listMapKey=name
	// OperatorOverrides - list of OperatorSpec which allows to customize operator deployments
	OperatorOverrides []OperatorSpec `json:"operatorOverrides"`
}

// OperatorSpec - customization for the operator deployment
type OperatorSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Enum:=openstack;barbican;cinder;designate;glance;heat;horizon;infra;ironic;keystone;manila;mariadb;neutron;nova;octavia;openstack-baremetal;ovn;placement;rabbitmq-cluster;swift;telemetry;test
	// Name of the service operators.
	Name string `json:"name"`

	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Maximum=1
	// +kubebuilder:validation:Minimum=0
	// Replicas of the operator deployment
	Replicas *int32 `json:"replicas,omitempty"`

	// +kubebuilder:validation:Optional
	// ControllerManager - tunings for the controller manager container
	ControllerManager ContainerSpec `json:"controllerManager,omitempty"`

	// +kubebuilder:validation:Optional
	// Tolerations - Tolerations for the service operator deployment pods
	// https://kubernetes.io/docs/concepts/scheduling-eviction/taint-and-toleration/
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
}

// ContainerSpec - customization for the container spec
type ContainerSpec struct {
	// +kubebuilder:validation:Optional
	// Resources - Compute Resources for the service operator controller manager
	// https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
}

// OpenStackStatus defines the observed state of OpenStack
type OpenStackStatus struct {

	// +operator-sdk:csv:customresourcedefinitions:type=status,xDescriptors={"urn:alm:descriptor:io.kubernetes.conditions"}
	// Conditions
	Conditions condition.Conditions `json:"conditions,omitempty" optional:"true"`

	// DeployedOperatorCount - the number of operators deployed
	DeployedOperatorCount *int `json:"deployedOperatorCount,omitempty"`

	// DisabledOperatorCount - the number of operators which has replicas set to 0
	DisabledOperatorCount *int `json:"disabledOperatorCount,omitempty"`

	// EnabledOperatorCount - the number of operators which has replicas set to 1
	EnabledOperatorCount *int `json:"enabledOperatorCount,omitempty"`

	// TotalOperatorCount - the number all operators available
	TotalOperatorCount *int `json:"totalOperatorCount,omitempty"`

	// ObservedGeneration - the most recent generation observed for this object.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"` // no spec yet so maybe we don't need this

	// ContainerImage - the container image that has been successfully deployed
	ContainerImage *string `json:"containerImage,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +operator-sdk:csv:customresourcedefinitions:displayName="OpenStack"
// +kubebuilder:printcolumn:name="Deployed Operator Count",type=integer,JSONPath=`.status.deployedOperatorCount`
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[0].status",description="Status"
// OpenStack is the Schema for the openstacks API
type OpenStack struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OpenStackSpec   `json:"spec,omitempty"`
	Status OpenStackStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OpenStackList contains a list of OpenStack
type OpenStackList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OpenStack `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OpenStack{}, &OpenStackList{})
}
