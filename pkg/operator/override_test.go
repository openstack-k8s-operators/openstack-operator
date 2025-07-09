/*
Copyright 2025.

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

package operator

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"

	// Using a mock path for the import
	// operatorv1beta1 "example.com/project/api/v1beta1"
	operatorv1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/operator/v1beta1"
)

// --- Test for hasOverrides (Unit Test) ---

func TestHasOverrides(t *testing.T) {
	overrides := []operatorv1beta1.OperatorSpec{
		{Name: "keystone"},
		{Name: "glance"},
		{Name: "cinder"},
	}

	testCases := []struct {
		name         string
		overrides    []operatorv1beta1.OperatorSpec
		operatorName string
		expectFound  bool
		expectedName string
	}{
		{
			name:         "Operator exists in overrides",
			overrides:    overrides,
			operatorName: "glance",
			expectFound:  true,
			expectedName: "glance",
		},
		{
			name:         "Operator does not exist in overrides",
			overrides:    overrides,
			operatorName: "nova",
			expectFound:  false,
		},
		{
			name:         "Empty override list",
			overrides:    []operatorv1beta1.OperatorSpec{},
			operatorName: "keystone",
			expectFound:  false,
		},
		{
			name:         "Nil override list",
			overrides:    nil,
			operatorName: "keystone",
			expectFound:  false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := HasOverrides(tc.overrides, tc.operatorName)

			if tc.expectFound && result == nil {
				t.Errorf("expected to find override for '%s', but got nil", tc.operatorName)
			}

			if !tc.expectFound && result != nil {
				t.Errorf("expected not to find override for '%s', but got: %+v", tc.operatorName, result)
			}

			if tc.expectFound && result != nil && result.Name != tc.expectedName {
				t.Errorf("found override, but name was incorrect. got: %s, want: %s", result.Name, tc.expectedName)
			}
		})
	}
}

// --- Test for setOverrides and hasOverrides working together (Functional Test) ---

func TestApplyOperatorOverrides(t *testing.T) {
	defaultResources := Resource{
		Limits: &ResourceList{
			CPU:    operatorv1beta1.DefaultManagerCPULimit.String(),
			Memory: operatorv1beta1.DefaultManagerMemoryLimit.String(),
		},
		Requests: &ResourceList{
			CPU:    operatorv1beta1.DefaultManagerCPURequests.String(),
			Memory: operatorv1beta1.DefaultManagerMemoryRequests.String(),
		},
	}

	// A list of potential overrides that can be applied
	allOverrides := []operatorv1beta1.OperatorSpec{
		{
			Name:     "keystone",
			Replicas: ptr.To[int32](3),
		},
		{
			Name: "glance",
			ControllerManager: operatorv1beta1.ContainerSpec{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    *cpuQuantity(2000),
						corev1.ResourceMemory: *memQuantity(4096),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU: *cpuQuantity(1000),
					},
				},
			},
		},
		{
			Name:     "infra",
			Replicas: ptr.To[int32](0),
			ControllerManager: operatorv1beta1.ContainerSpec{
				Resources: corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						corev1.ResourceCPU:    *cpuQuantity(5000),
						corev1.ResourceMemory: *memQuantity(8192),
					},
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    *cpuQuantity(1000),
						corev1.ResourceMemory: *memQuantity(4096),
					},
				},
			},
		},
	}

	// --- Define Test Cases ---
	testCases := []struct {
		name         string
		operatorName string
		initialOp    *Operator
		overrideList []operatorv1beta1.OperatorSpec
		// We will assert on specific fields instead of the whole struct
		expectedReplicas *int32
		expectedLimits   *ResourceList
		expectedRequests *ResourceList
	}{
		{
			name:         "Scenario 1: Override is found and applied (Keystone)",
			operatorName: "keystone",
			initialOp: &Operator{
				Name: "keystone",
				Deployment: Deployment{
					Replicas: ptr.To[int32](1),
					Manager: Container{
						Resources: defaultResources,
					},
				},
			},
			overrideList:     allOverrides,
			expectedReplicas: ptr.To[int32](3), // Expect this to change
		},
		{
			name:         "Scenario 2: Another override is found and applied (Glance)",
			operatorName: "glance",
			initialOp: &Operator{
				Name: "glance",
				Deployment: Deployment{
					Replicas: ptr.To[int32](1),
					Manager: Container{
						Resources: defaultResources,
					},
				},
			},
			overrideList: allOverrides,
			// Expectations for this test case
			expectedReplicas: ptr.To[int32](1), // Expect this to remain unchanged
			expectedLimits: &ResourceList{
				CPU:    "2",
				Memory: "4Gi",
			},
			expectedRequests: &ResourceList{
				CPU:    "1",
				Memory: "128Mi", // the default must not be overridden if we just change the cpu
			},
		},
		{
			name:         "Scenario 3: Another override is found and applied (Infra)",
			operatorName: "infra",
			initialOp: &Operator{
				Name: "infra",
				Deployment: Deployment{
					Replicas: ptr.To[int32](1),
					Manager: Container{
						Resources: defaultResources,
					},
				},
			},
			overrideList: allOverrides,
			// Expectations for this test case
			expectedReplicas: ptr.To[int32](0),
			expectedLimits: &ResourceList{
				CPU:    "5",
				Memory: "8Gi",
			},
			expectedRequests: &ResourceList{
				CPU:    "1",
				Memory: "4Gi",
			},
		},
		{
			name:         "Scenario 4: Operator not in override list, no changes applied",
			operatorName: "nova",
			initialOp: &Operator{
				Name:       "nova",
				Deployment: Deployment{Replicas: ptr.To[int32](1)},
			},
			overrideList:     allOverrides,
			expectedReplicas: ptr.To[int32](1), // Expect this to remain unchanged
		},
	}

	// --- Run Test Cases ---
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Step 1: Call the first function to find the override
			opOvr := HasOverrides(tc.overrideList, tc.operatorName)

			// Step 2: If an override is found, call the second function to apply it
			if opOvr != nil {
				// Initialize maps if they are nil to avoid panics in setOverrides
				if tc.initialOp.Deployment.Manager.Resources.Limits == nil {
					tc.initialOp.Deployment.Manager.Resources.Limits = &ResourceList{}
				}
				if tc.initialOp.Deployment.Manager.Resources.Requests == nil {
					tc.initialOp.Deployment.Manager.Resources.Requests = &ResourceList{}
				}
				SetOverrides(*opOvr, tc.initialOp)
			}

			// Step 3: Assert on specific fields instead of using DeepEqual on the whole struct

			// Assert Replicas
			if tc.expectedReplicas != nil {
				if tc.initialOp.Deployment.Replicas == nil {
					t.Errorf("Replicas are nil, want %d", *tc.expectedReplicas)
				} else if *tc.initialOp.Deployment.Replicas != *tc.expectedReplicas {
					t.Errorf("wrong replica count: got %d, want %d", *tc.initialOp.Deployment.Replicas, *tc.expectedReplicas)
				}
			}

			// Assert Limits
			if tc.expectedLimits != nil {
				if !reflect.DeepEqual(tc.initialOp.Deployment.Manager.Resources.Limits, tc.expectedLimits) {
					t.Errorf("wrong resource limits:\n got: %+v\nwant: %+v", tc.initialOp.Deployment.Manager.Resources.Limits, tc.expectedLimits)
				}
			}

			// Assert Requests
			if tc.expectedLimits != nil {
				if !reflect.DeepEqual(tc.initialOp.Deployment.Manager.Resources.Requests, tc.expectedRequests) {
					t.Errorf("wrong resource limits:\n got: %+v\nwant: %+v", tc.initialOp.Deployment.Manager.Resources.Requests, tc.expectedRequests)
				}
			}
		})
	}
}
