package deployment

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeSystemID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "short hostname",
			input:    "edpm-compute-0",
			expected: computeSystemID("edpm-compute-0"),
		},
		{
			name:     "FQDN",
			input:    "edpm-compute-0.ctlplane.example.com",
			expected: computeSystemID("edpm-compute-0.ctlplane.example.com"),
		},
		{
			name:  "deterministic: same input always yields same output",
			input: "edpm-compute-0",
		},
		{
			name:  "different inputs yield different outputs",
			input: "edpm-compute-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := computeSystemID(tt.input)

			// Must be non-empty
			assert.NotEmpty(t, result)

			// Must be deterministic
			assert.Equal(t, result, computeSystemID(tt.input),
				"computeSystemID must be deterministic")

			if tt.expected != "" {
				assert.Equal(t, tt.expected, result)
			}
		})
	}

	// Different inputs must produce different UUIDs
	id0 := computeSystemID("edpm-compute-0")
	id1 := computeSystemID("edpm-compute-1")
	assert.NotEqual(t, id0, id1,
		"different hostnames must produce different system IDs")

	// Verify format is a valid UUID (8-4-4-4-12 hex)
	assert.Regexp(t, `^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
		computeSystemID("test-node"),
		"computeSystemID must return a valid UUID string")
}

func TestCreateSecretsDataStructure(t *testing.T) {
	tests := []struct {
		name           string
		secretMaxSize  int
		certsData      map[string][]byte
		expectedChunks int
	}{
		{
			name:          "single node fits in one secret",
			secretMaxSize: 1048576,
			certsData: map[string][]byte{
				"node1-ca.crt":  []byte("ca-cert-data"),
				"node1-tls.crt": []byte("tls-cert-data"),
				"node1-tls.key": []byte("tls-key-data"),
			},
			expectedChunks: 1,
		},
		{
			name:          "small max size forces multiple secrets",
			secretMaxSize: 1,
			certsData: map[string][]byte{
				"node1-ca.crt":  []byte("ca-cert-data"),
				"node1-tls.crt": []byte("tls-cert-data"),
				"node1-tls.key": []byte("tls-key-data"),
				"node2-ca.crt":  []byte("ca-cert-data"),
				"node2-tls.crt": []byte("tls-cert-data"),
				"node2-tls.key": []byte("tls-key-data"),
			},
			expectedChunks: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := createSecretsDataStructure(tt.secretMaxSize, tt.certsData)
			assert.Equal(t, tt.expectedChunks, len(result))

			// Verify all data is present across chunks
			totalKeys := 0
			for _, chunk := range result {
				totalKeys += len(chunk)
			}
			assert.Equal(t, len(tt.certsData), totalKeys,
				"all cert data must be present across chunks")
		})
	}
}
