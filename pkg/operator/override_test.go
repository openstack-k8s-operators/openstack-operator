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

	// Define custom tolerations for testing
	customTolerations := []corev1.Toleration{
		{
			Key:      "example.com/special-node",
			Operator: corev1.TolerationOpEqual,
			Value:    "special",
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](300),
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
		{
			Name: "nova",
			ControllerManager: operatorv1beta1.ContainerSpec{
				Tolerations: customTolerations,
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
		expectedReplicas    *int32
		expectedLimits      *ResourceList
		expectedRequests    *ResourceList
		expectedTolerations []corev1.Toleration
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
			operatorName: "neutron",
			initialOp: &Operator{
				Name:       "neutron",
				Deployment: Deployment{Replicas: ptr.To[int32](1)},
			},
			overrideList:     allOverrides,
			expectedReplicas: ptr.To[int32](1), // Expect this to remain unchanged
		},
		{
			name:         "Scenario 5: Tolerations override is applied (Nova)",
			operatorName: "nova",
			initialOp: &Operator{
				Name: "nova",
				Deployment: Deployment{
					Replicas: ptr.To[int32](1),
					Manager: Container{
						Resources: defaultResources,
					},
				},
			},
			overrideList:        allOverrides,
			expectedReplicas:    ptr.To[int32](1),
			expectedTolerations: customTolerations,
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
			if tc.expectedRequests != nil {
				if !reflect.DeepEqual(tc.initialOp.Deployment.Manager.Resources.Requests, tc.expectedRequests) {
					t.Errorf("wrong resource requests:\n got: %+v\nwant: %+v", tc.initialOp.Deployment.Manager.Resources.Requests, tc.expectedRequests)
				}
			}

			// Assert Tolerations
			if tc.expectedTolerations != nil {
				if !reflect.DeepEqual(tc.initialOp.Deployment.Tolerations, tc.expectedTolerations) {
					t.Errorf("wrong tolerations:\n got: %+v\nwant: %+v", tc.initialOp.Deployment.Tolerations, tc.expectedTolerations)
				}
			}
		})
	}
}

// --- Test specifically for tolerations functionality ---

func TestTolerationsOverride(t *testing.T) {
	testTolerations := []corev1.Toleration{
		{
			Key:      "node.example.com/gpu",
			Operator: corev1.TolerationOpEqual,
			Value:    "nvidia",
			Effect:   corev1.TaintEffectNoSchedule,
		},
		{
			Key:               corev1.TaintNodeMemoryPressure, // "node.kubernetes.io/memory-pressure",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](600),
		},
	}

	// Default tolerations for testing merge behavior
	defaultTolerations := []corev1.Toleration{
		{
			Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
		{
			Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
	}

	testCases := []struct {
		name                string
		operatorSpec        operatorv1beta1.OperatorSpec
		initialTolerations  []corev1.Toleration
		expectedTolerations []corev1.Toleration
	}{
		{
			name: "Add tolerations to empty list",
			operatorSpec: operatorv1beta1.OperatorSpec{
				Name: "test-operator",
				ControllerManager: operatorv1beta1.ContainerSpec{
					Tolerations: testTolerations,
				},
			},
			initialTolerations:  nil,
			expectedTolerations: testTolerations,
		},
		{
			name: "No custom tolerations, keep defaults unchanged",
			operatorSpec: operatorv1beta1.OperatorSpec{
				Name:              "test-operator",
				ControllerManager: operatorv1beta1.ContainerSpec{
					// No tolerations specified
				},
			},
			initialTolerations:  defaultTolerations,
			expectedTolerations: defaultTolerations,
		},
		{
			name: "Merge custom tolerations with defaults (different keys)",
			operatorSpec: operatorv1beta1.OperatorSpec{
				Name: "test-operator",
				ControllerManager: operatorv1beta1.ContainerSpec{
					Tolerations: testTolerations, // Different keys than defaults
				},
			},
			initialTolerations:  defaultTolerations,
			expectedTolerations: append(defaultTolerations, testTolerations...),
		},
		{
			name: "Override default tolerations (same key)",
			operatorSpec: operatorv1beta1.OperatorSpec{
				Name: "test-operator",
				ControllerManager: operatorv1beta1.ContainerSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready", // Same key as default
							Operator:          corev1.TolerationOpExists,
							Effect:            corev1.TaintEffectNoExecute,
							TolerationSeconds: ptr.To[int64](600), // Different value
						},
					},
				},
			},
			initialTolerations: defaultTolerations,
			expectedTolerations: []corev1.Toleration{
				{
					Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](600), // Overridden value
				},
				{
					Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](120), // Unchanged default
				},
			},
		},
		{
			name: "Mixed scenario: override one default, add new custom",
			operatorSpec: operatorv1beta1.OperatorSpec{
				Name: "test-operator",
				ControllerManager: operatorv1beta1.ContainerSpec{
					Tolerations: []corev1.Toleration{
						{
							Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready", // Override default
							Operator:          corev1.TolerationOpExists,
							Effect:            corev1.TaintEffectNoExecute,
							TolerationSeconds: ptr.To[int64](300),
						},
						{
							Key:      "node.example.com/gpu", // Add new
							Operator: corev1.TolerationOpEqual,
							Value:    "nvidia",
							Effect:   corev1.TaintEffectNoSchedule,
						},
					},
				},
			},
			initialTolerations: defaultTolerations,
			expectedTolerations: []corev1.Toleration{
				{
					Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready", // Overridden
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300),
				},
				{
					Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable", // Unchanged default
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](120),
				},
				{
					Key:      "node.example.com/gpu", // New addition
					Operator: corev1.TolerationOpEqual,
					Value:    "nvidia",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			op := &Operator{
				Name: tc.operatorSpec.Name,
				Deployment: Deployment{
					Tolerations: tc.initialTolerations,
				},
			}

			SetOverrides(tc.operatorSpec, op)

			if !reflect.DeepEqual(op.Deployment.Tolerations, tc.expectedTolerations) {
				t.Errorf("wrong tolerations after override:\n got: %+v\nwant: %+v", op.Deployment.Tolerations, tc.expectedTolerations)
			}
		})
	}
}

// --- Test for mergeTolerations function ---

func TestMergeTolerations(t *testing.T) {
	defaultTolerations := []corev1.Toleration{
		{
			Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
		{
			Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
	}

	testCases := []struct {
		name     string
		defaults []corev1.Toleration
		custom   []corev1.Toleration
		expected []corev1.Toleration
	}{
		{
			name:     "Empty custom tolerations should return defaults",
			defaults: defaultTolerations,
			custom:   []corev1.Toleration{},
			expected: defaultTolerations,
		},
		{
			name:     "Nil custom tolerations should return defaults",
			defaults: defaultTolerations,
			custom:   nil,
			expected: defaultTolerations,
		},
		{
			name:     "Add new toleration to defaults",
			defaults: defaultTolerations,
			custom: []corev1.Toleration{
				{
					Key:      "node.example.com/gpu",
					Operator: corev1.TolerationOpEqual,
					Value:    "nvidia",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			expected: []corev1.Toleration{
				{
					Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](120),
				},
				{
					Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](120),
				},
				{
					Key:      "node.example.com/gpu",
					Operator: corev1.TolerationOpEqual,
					Value:    "nvidia",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
		},
		{
			name:     "Override existing toleration",
			defaults: defaultTolerations,
			custom: []corev1.Toleration{
				{
					Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](600),
				},
			},
			expected: []corev1.Toleration{
				{
					Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](600), // Overridden
				},
				{
					Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](120), // Unchanged
				},
			},
		},
		{
			name:     "Mixed: override one, add one",
			defaults: defaultTolerations,
			custom: []corev1.Toleration{
				{
					Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300),
				},
				{
					Key:      "node.example.com/special",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule,
				},
			},
			expected: []corev1.Toleration{
				{
					Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](300), // Overridden
				},
				{
					Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable",
					Operator:          corev1.TolerationOpExists,
					Effect:            corev1.TaintEffectNoExecute,
					TolerationSeconds: ptr.To[int64](120), // Unchanged
				},
				{
					Key:      "node.example.com/special",
					Operator: corev1.TolerationOpEqual,
					Value:    "true",
					Effect:   corev1.TaintEffectNoSchedule, // Added
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := mergeTolerations(tc.defaults, tc.custom)
			if !reflect.DeepEqual(result, tc.expected) {
				t.Errorf("mergeTolerations() failed:\n got: %+v\nwant: %+v", result, tc.expected)
			}
		})
	}
}

// --- Test for global defaults initialization ---

func TestGlobalTolerationsDefaults(t *testing.T) {
	// Test that the default tolerations are correctly defined
	if len(operatorv1beta1.DefaultTolerations) != 2 {
		t.Errorf("Expected 2 default tolerations, got %d", len(operatorv1beta1.DefaultTolerations))
	}

	// Verify the specific default tolerations
	expectedDefaults := []corev1.Toleration{
		{
			Key:               corev1.TaintNodeNotReady, // "node.kubernetes.io/not-ready",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
		{
			Key:               corev1.TaintNodeUnreachable, // "node.kubernetes.io/unreachable",
			Operator:          corev1.TolerationOpExists,
			Effect:            corev1.TaintEffectNoExecute,
			TolerationSeconds: ptr.To[int64](120),
		},
	}

	if !reflect.DeepEqual(operatorv1beta1.DefaultTolerations, expectedDefaults) {
		t.Errorf("Default tolerations don't match expected:\n got: %+v\nwant: %+v", operatorv1beta1.DefaultTolerations, expectedDefaults)
	}
}
