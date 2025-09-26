package deployment

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProcessConfigMapData_NumbersAsInts(t *testing.T) {
	tests := []struct {
		name         string
		data         map[string]string
		prefix       string
		expected     map[string]any
		expectedType any
	}{
		{
			name: "string value remains string",
			data: map[string]string{
				"foo": "bar",
			},
			prefix: "",
			expected: map[string]any{
				"foo": "bar",
			},
			expectedType: "",
		},
		{
			name: "integer number decoded correctly",
			data: map[string]string{
				"intVal": "123",
			},
			prefix: "",
			expected: map[string]any{
				"intVal": json.Number("123"),
			},
			expectedType: json.Number("1"),
		},
		{
			name: "float number decoded correctly",
			data: map[string]string{
				"floatVal": "123.45",
			},
			prefix: "",
			expected: map[string]any{
				"floatVal": json.Number("123.45"),
			},
			expectedType: json.Number("1"),
		},
		{
			name: "with prefix",
			data: map[string]string{
				"somekey": "42",
			},
			prefix: "myprefix-",
			expected: map[string]any{
				"myprefix-somekey": json.Number("42"),
			},
			expectedType: json.Number("1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]any)
			err := processConfigMapData(tt.data, tt.prefix, result)
			assert.NoError(t, err)

			for k, v := range tt.expected {
				actual, ok := result[k]
				assert.True(t, ok, "key %s should exist", k)

				assert.IsType(t, tt.expectedType, actual)

				expectedVal, ok1 := v.(json.Number)
				actualVal, ok2 := actual.(json.Number)

				if ok1 && ok2 {
					assert.Equal(t, expectedVal, actualVal)
				} else {
					assert.Equal(t, v, actual)
				}
			}
		})
	}
}

func TestProcessSecretData_NumbersAsInts(t *testing.T) {
	tests := []struct {
		name         string
		data         map[string][]byte
		prefix       string
		expected     map[string]any
		expectedType any
	}{
		{
			name: "string value remains string",
			data: map[string][]byte{
				"foo": []byte("bar"),
			},
			prefix: "",
			expected: map[string]any{
				"foo": "bar",
			},
			expectedType: "",
		},
		{
			name: "integer number decoded correctly",
			data: map[string][]byte{
				"intVal": []byte("123"),
			},
			prefix: "",
			expected: map[string]any{
				"intVal": json.Number("123"),
			},
			expectedType: json.Number("1"),
		},
		{
			name: "float number decoded correctly",
			data: map[string][]byte{
				"floatVal": []byte("123.45"),
			},
			prefix: "",
			expected: map[string]any{
				"floatVal": json.Number("123.45"),
			},
			expectedType: json.Number("1"),
		},
		{
			name: "with prefix",
			data: map[string][]byte{
				"somekey": []byte("42"),
			},
			prefix: "myprefix-",
			expected: map[string]any{
				"myprefix-somekey": json.Number("42"),
			},
			expectedType: json.Number("1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]any)
			err := processSecretData(tt.data, tt.prefix, result)
			assert.NoError(t, err)

			for k, v := range tt.expected {
				actual, ok := result[k]
				assert.True(t, ok, "key %s should exist", k)

				assert.IsType(t, tt.expectedType, actual)

				expectedVal, ok1 := v.(json.Number)
				actualVal, ok2 := actual.(json.Number)

				if ok1 && ok2 {
					assert.Equal(t, expectedVal, actualVal)
				} else {
					assert.Equal(t, v, actual)
				}
			}
		})
	}
}
