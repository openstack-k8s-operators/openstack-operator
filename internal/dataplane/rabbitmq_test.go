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

package deployment

import (
	"context"
	"fmt"
	"testing"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// setupTestHelper creates a fake client and helper for testing
func setupTestHelper(objects ...client.Object) *helper.Helper {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		Build()

	// Create a fake kubernetes clientset
	fakeKubeClient := fake.NewSimpleClientset()

	// Create a mock object for the helper (minimal valid object)
	mockObj := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "test-namespace",
		},
	}

	h, _ := helper.NewHelper(
		mockObj,
		fakeClient,
		fakeKubeClient,
		scheme,
		ctrl.Log.WithName("test"),
	)
	return h
}

func TestGetNovaCellRabbitMqUserFromSecret(t *testing.T) {
	ctx := context.Background()
	namespace := "openstack"
	cellName := "cell1"

	tests := []struct {
		name          string
		secrets       []runtime.Object
		cellName      string
		expectedUser  string
		expectedError bool
		errorContains string
	}{
		{
			name:     "New format: rabbitmq_user_name field present (preferred path)",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"rabbitmq_user_name": []byte("nova-cell1-user"),
						"01-nova.conf":       []byte("[DEFAULT]\ntransport_url = rabbit://old-user:pass@host:5672/\n"),
					},
				},
			},
			expectedUser:  "nova-cell1-user",
			expectedError: false,
		},
		{
			name:     "Old format: transport_url in config file (fallback path)",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"01-nova.conf": []byte("[DEFAULT]\ntransport_url = rabbit://fallback-user:password@rabbitmq.openstack.svc:5672/\n"),
					},
				},
			},
			expectedUser:  "fallback-user",
			expectedError: false,
		},
		{
			name:     "Both fields present: should prefer rabbitmq_user_name",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"rabbitmq_user_name": []byte("preferred-user"),
						"01-nova.conf":       []byte("[DEFAULT]\ntransport_url = rabbit://fallback-user:pass@host:5672/\n"),
					},
				},
			},
			expectedUser:  "preferred-user",
			expectedError: false,
		},
		{
			name:     "Secret with versioned suffix (nova-cell1-compute-config-1)",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config-1",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"rabbitmq_user_name": []byte("versioned-user"),
					},
				},
			},
			expectedUser:  "versioned-user",
			expectedError: false,
		},
		{
			name:     "Empty rabbitmq_user_name falls back to transport_url",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"rabbitmq_user_name": []byte(""),
						"01-nova.conf":       []byte("[DEFAULT]\ntransport_url = rabbit://empty-fallback:pass@host:5672/\n"),
					},
				},
			},
			expectedUser:  "empty-fallback",
			expectedError: false,
		},
		{
			name:          "No matching secret found",
			cellName:      "cell99",
			secrets:       []runtime.Object{},
			expectedUser:  "",
			expectedError: true,
			errorContains: "no RabbitMQ username found for cell cell99",
		},
		{
			name:     "Secret exists but no transport_url or rabbitmq_user_name",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"other-data": []byte("some-value"),
					},
				},
			},
			expectedUser:  "",
			expectedError: true,
			errorContains: "no RabbitMQ username found for cell cell1",
		},
		{
			name:     "Transport URL with TLS",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"01-nova.conf": []byte("[DEFAULT]\ntransport_url = rabbit+tls://tls-user:password@host1:5671,host2:5671/vhost\n"),
					},
				},
			},
			expectedUser:  "tls-user",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert runtime.Object slice to client.Object slice
			clientObjects := make([]client.Object, len(tt.secrets))
			for i, obj := range tt.secrets {
				clientObjects[i] = obj.(client.Object)
			}

			// Create helper with fake client
			h := setupTestHelper(clientObjects...)

			// Call the function
			username, err := GetNovaCellRabbitMqUserFromSecret(ctx, h, namespace, tt.cellName)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUser, username)
			}
		})
	}
}

func TestGetNovaCellNotificationRabbitMqUserFromSecret(t *testing.T) {
	ctx := context.Background()
	namespace := "openstack"
	cellName := "cell1"

	tests := []struct {
		name          string
		secrets       []runtime.Object
		cellName      string
		expectedUser  string
		expectedError bool
		errorContains string
	}{
		{
			name:     "notification_rabbitmq_user_name field present",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"notification_rabbitmq_user_name": []byte("nova-cell1-notification-user"),
						"rabbitmq_user_name":              []byte("nova-cell1-user"),
					},
				},
			},
			expectedUser:  "nova-cell1-notification-user",
			expectedError: false,
		},
		{
			name:     "notification_rabbitmq_user_name not present (notifications not configured)",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"rabbitmq_user_name": []byte("nova-cell1-user"),
					},
				},
			},
			expectedUser:  "",
			expectedError: false,
		},
		{
			name:     "Empty notification_rabbitmq_user_name",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"notification_rabbitmq_user_name": []byte(""),
						"rabbitmq_user_name":              []byte("nova-cell1-user"),
					},
				},
			},
			expectedUser:  "",
			expectedError: false,
		},
		{
			name:     "Secret with versioned suffix",
			cellName: cellName,
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config-2",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"notification_rabbitmq_user_name": []byte("versioned-notification-user"),
					},
				},
			},
			expectedUser:  "versioned-notification-user",
			expectedError: false,
		},
		{
			name:          "No matching secret found",
			cellName:      "cell99",
			secrets:       []runtime.Object{},
			expectedUser:  "",
			expectedError: true,
			errorContains: "no compute-config secret found for cell cell99",
		},
		{
			name:     "Multiple cells - should match correct cell",
			cellName: "cell2",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell1-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"notification_rabbitmq_user_name": []byte("cell1-notification-user"),
					},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "nova-cell2-compute-config",
						Namespace: namespace,
					},
					Data: map[string][]byte{
						"notification_rabbitmq_user_name": []byte("cell2-notification-user"),
					},
				},
			},
			expectedUser:  "cell2-notification-user",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert runtime.Object slice to client.Object slice
			clientObjects := make([]client.Object, len(tt.secrets))
			for i, obj := range tt.secrets {
				clientObjects[i] = obj.(client.Object)
			}

			// Create helper with fake client
			h := setupTestHelper(clientObjects...)

			// Call the function
			username, err := GetNovaCellNotificationRabbitMqUserFromSecret(ctx, h, namespace, tt.cellName)

			// Verify results
			if tt.expectedError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedUser, username)
			}
		})
	}
}

func TestGetNovaCellRabbitMqUserFromSecret_EdgeCases(t *testing.T) {
	ctx := context.Background()
	namespace := "openstack"

	t.Run("Multiple secrets for same cell - should use first match", func(t *testing.T) {
		secrets := []runtime.Object{
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nova-cell1-compute-config",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"rabbitmq_user_name": []byte("first-user"),
				},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "nova-cell1-compute-config-1",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"rabbitmq_user_name": []byte("second-user"),
				},
			},
		}

		// Convert runtime.Object slice to client.Object slice
		clientObjects := make([]client.Object, len(secrets))
		for i, obj := range secrets {
			clientObjects[i] = obj.(client.Object)
		}

		h := setupTestHelper(clientObjects...)

		username, err := GetNovaCellRabbitMqUserFromSecret(ctx, h, namespace, "cell1")
		assert.NoError(t, err)
		// Should get one of the users (order may vary, but shouldn't error)
		assert.NotEmpty(t, username)
		assert.Contains(t, []string{"first-user", "second-user"}, username)
	})

	t.Run("Complex transport URL parsing", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nova-cell1-compute-config",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"01-nova.conf": []byte(fmt.Sprintf(`[DEFAULT]
debug = true
transport_url = rabbit://complex-user:p@ssw0rd@host1:5672,host2:5672,host3:5672/cell1
log_dir = /var/log/nova
`)),
			},
		}

		h := setupTestHelper(secret)

		username, err := GetNovaCellRabbitMqUserFromSecret(ctx, h, namespace, "cell1")
		assert.NoError(t, err)
		assert.Equal(t, "complex-user", username)
	})

	t.Run("Cell name with special characters", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nova-cell-prod-az1-compute-config",
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"rabbitmq_user_name": []byte("cell-prod-az1-user"),
			},
		}

		h := setupTestHelper(secret)

		username, err := GetNovaCellRabbitMqUserFromSecret(ctx, h, namespace, "cell-prod-az1")
		assert.NoError(t, err)
		assert.Equal(t, "cell-prod-az1-user", username)
	})
}
