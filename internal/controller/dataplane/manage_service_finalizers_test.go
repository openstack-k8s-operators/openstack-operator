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

package dataplane

import (
	"strings"
	"testing"
)

func TestComputeFinalizerHash(t *testing.T) {
	tests := []struct {
		name         string
		nodesetName  string
		expectedHash string // First 8 chars of SHA256
	}{
		{
			name:         "Short nodeset name",
			nodesetName:  "compute",
			expectedHash: "b04a12f6", // First 8 chars of SHA256("compute")
		},
		{
			name:         "Medium length name",
			nodesetName:  "edpm-compute-nodes",
			expectedHash: "32188c96", // First 8 chars of SHA256("edpm-compute-nodes")
		},
		{
			name:         "Long nodeset name",
			nodesetName:  "my-very-long-openstack-dataplane-nodeset-name-for-production",
			expectedHash: "ae0c35f8", // First 8 chars of SHA256(...)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeFinalizerHash(tt.nodesetName)

			// Check length is exactly 8 characters
			if len(result) != 8 {
				t.Errorf("computeFinalizerHash() length = %v, want 8", len(result))
			}

			// Check it's a valid hex string
			for _, c := range result {
				if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
					t.Errorf("computeFinalizerHash() contains non-hex char %c, result = %v", c, result)
				}
			}

			// Check it matches expected hash
			if result != tt.expectedHash {
				t.Errorf("computeFinalizerHash() = %v, want %v", result, tt.expectedHash)
			}

			t.Logf("Hash for %q: %s", tt.nodesetName, result)
		})
	}
}

func TestComputeFinalizerHashDeterministic(t *testing.T) {
	// The same input should always produce the same hash
	nodesetName := "my-very-long-openstack-dataplane-nodeset-name"

	hash1 := computeFinalizerHash(nodesetName)
	hash2 := computeFinalizerHash(nodesetName)

	if hash1 != hash2 {
		t.Errorf("computeFinalizerHash() not deterministic: first=%v, second=%v", hash1, hash2)
	}
}

func TestComputeFinalizerHashUniqueness(t *testing.T) {
	// Different inputs should produce different hashes
	tests := []struct {
		nodeset1, nodeset2 string
	}{
		{"compute-zone1", "compute-zone2"},
		{"edpm-compute-nodes-a", "edpm-compute-nodes-b"},
		{"my-very-long-name-production-zone1", "my-very-long-name-staging-zone1"},
	}

	for _, tt := range tests {
		hash1 := computeFinalizerHash(tt.nodeset1)
		hash2 := computeFinalizerHash(tt.nodeset2)

		if hash1 == hash2 {
			t.Errorf("computeFinalizerHash() collision detected:\n  %s => %s\n  %s => %s",
				tt.nodeset1, hash1,
				tt.nodeset2, hash2)
		}
	}
}

func TestBuildFinalizerName(t *testing.T) {
	tests := []struct {
		name           string
		finalizerHash  string
		serviceName    string
		expectedResult string
		expectedMaxLen int
	}{
		{
			name:           "Nova service",
			finalizerHash:  "a3f2b5c8",
			serviceName:    "nova",
			expectedResult: "nodeset.os/a3f2b5c8-nova",
			expectedMaxLen: 63,
		},
		{
			name:           "Neutron service",
			finalizerHash:  "7e9d1234",
			serviceName:    "neutron",
			expectedResult: "nodeset.os/7e9d1234-neutron",
			expectedMaxLen: 63,
		},
		{
			name:           "Ironic service",
			finalizerHash:  "5a6b7c8d",
			serviceName:    "ironic",
			expectedResult: "nodeset.os/5a6b7c8d-ironic",
			expectedMaxLen: 63,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildFinalizerName(tt.finalizerHash, tt.serviceName)

			// Check exact result
			if result != tt.expectedResult {
				t.Errorf("buildFinalizerName() = %v, want %v", result, tt.expectedResult)
			}

			// Check length is within Kubernetes limit
			if len(result) > tt.expectedMaxLen {
				t.Errorf("buildFinalizerName() length = %v, exceeds max %v", len(result), tt.expectedMaxLen)
			}

			// Check format
			if !strings.HasPrefix(result, finalizerPrefix) {
				t.Errorf("buildFinalizerName() missing prefix %q, got %v", finalizerPrefix, result)
			}

			if !strings.HasSuffix(result, "-"+tt.serviceName) {
				t.Errorf("buildFinalizerName() missing service suffix %q, got %v", tt.serviceName, result)
			}

			t.Logf("Result: %s (length: %d)", result, len(result))
		})
	}
}

func TestBuildFinalizerNameDeterministic(t *testing.T) {
	// The same inputs should always produce the same output
	finalizerHash := "a3f2b5c8"
	serviceName := "nova"

	result1 := buildFinalizerName(finalizerHash, serviceName)
	result2 := buildFinalizerName(finalizerHash, serviceName)

	if result1 != result2 {
		t.Errorf("buildFinalizerName() not deterministic: first=%v, second=%v", result1, result2)
	}
}

func TestBuildFinalizerNameUniqueness(t *testing.T) {
	// Different inputs should produce different outputs
	tests := []struct {
		hash1, service1 string
		hash2, service2 string
	}{
		{
			hash1: "a3f2b5c8", service1: "nova",
			hash2: "7e9d1234", service2: "nova",
		},
		{
			hash1: "a3f2b5c8", service1: "nova",
			hash2: "a3f2b5c8", service2: "neutron",
		},
	}

	for _, tt := range tests {
		result1 := buildFinalizerName(tt.hash1, tt.service1)
		result2 := buildFinalizerName(tt.hash2, tt.service2)

		if result1 == result2 {
			t.Errorf("buildFinalizerName() not unique:\n  input1: %s-%s = %s\n  input2: %s-%s = %s",
				tt.hash1, tt.service1, result1,
				tt.hash2, tt.service2, result2)
		}
	}
}

func TestEndToEndFinalizerGeneration(t *testing.T) {
	// Test complete workflow: nodeset name -> hash -> finalizer
	nodesets := []string{
		"compute",
		"edpm-compute-nodes",
		"production-compute-cluster-zone1",
		"my-extremely-long-dataplane-nodeset-name-for-testing",
		"edpm-very-long-name-production-zone1",
		"edpm-very-long-name-staging-zone1", // Should have different hash than above
	}

	services := []string{"nova", "neutron", "ironic"}

	t.Log("End-to-end finalizer generation examples:")

	// Track all generated finalizers to ensure uniqueness
	finalizers := make(map[string]string)

	for _, nodeset := range nodesets {
		hash := computeFinalizerHash(nodeset)

		for _, service := range services {
			finalizer := buildFinalizerName(hash, service)

			// Check for collisions
			if existingNodeset, exists := finalizers[finalizer]; exists {
				t.Errorf("Collision detected! Finalizer %q used by both %q and %q",
					finalizer, existingNodeset, nodeset+"-"+service)
			}
			finalizers[finalizer] = nodeset + "-" + service

			// Verify length limit
			if len(finalizer) > 63 {
				t.Errorf("INVALID: finalizer exceeds 63 chars: %s", finalizer)
			}

			// Verify format
			if !strings.HasPrefix(finalizer, finalizerPrefix) {
				t.Errorf("INVALID: finalizer missing prefix: %s", finalizer)
			}

			t.Logf("  %s + %s => %s (hash=%s, len=%d)",
				nodeset, service, finalizer, hash, len(finalizer))
		}
	}

	t.Logf("Generated %d unique finalizers across %d nodesets and %d services",
		len(finalizers), len(nodesets), len(services))
}
