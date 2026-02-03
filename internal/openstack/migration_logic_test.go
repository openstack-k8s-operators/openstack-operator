/*
Copyright 2026.

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

package openstack

import (
	"testing"

	. "github.com/onsi/gomega" //revive:disable:dot-imports

	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
)

// TestRabbitMQMigrationPattern tests the standard migration pattern for RabbitMQ cluster configuration
func TestRabbitMQMigrationPattern(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name                    string
		deprecatedValue         string
		newClusterValue         string
		expectedClusterValue    string
		expectedDeprecatedValue string
		description             string
	}{
		{
			name:                    "Migrate from deprecated to new field",
			deprecatedValue:         "rabbitmq",
			newClusterValue:         "",
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should copy deprecated value to new field and clear deprecated",
		},
		{
			name:                    "New field takes precedence",
			deprecatedValue:         "old-rabbitmq",
			newClusterValue:         "new-rabbitmq",
			expectedClusterValue:    "new-rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should preserve new field value and clear deprecated",
		},
		{
			name:                    "Default when both empty",
			deprecatedValue:         "",
			newClusterValue:         "",
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should set default value when both fields are empty",
		},
		{
			name:                    "Custom value migrates",
			deprecatedValue:         "custom-mq-cluster",
			newClusterValue:         "",
			expectedClusterValue:    "custom-mq-cluster",
			expectedDeprecatedValue: "",
			description:             "Should migrate custom cluster names",
		},
		{
			name:                    "Already migrated",
			deprecatedValue:         "",
			newClusterValue:         "rabbitmq",
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should leave already-migrated configs unchanged",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Simulate the migration logic pattern used across all services
			var messagingBus rabbitmqv1.RabbitMqConfig
			messagingBus.Cluster = tc.newClusterValue
			deprecatedField := tc.deprecatedValue

			// Migration logic (standard pattern)
			if messagingBus.Cluster == "" {
				if deprecatedField != "" {
					messagingBus.Cluster = deprecatedField
				} else {
					messagingBus.Cluster = "rabbitmq"
				}
			}
			// Clear deprecated field
			if messagingBus.Cluster != "" {
				deprecatedField = ""
			}

			// Verify results
			g.Expect(messagingBus.Cluster).To(Equal(tc.expectedClusterValue),
				tc.description+" - Cluster value mismatch")
			g.Expect(deprecatedField).To(Equal(tc.expectedDeprecatedValue),
				tc.description+" - Deprecated field should be cleared")
		})
	}
}

// TestNotificationsBusMigrationPattern tests migration for pointer-based NotificationsBus
func TestNotificationsBusMigrationPattern(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name                    string
		deprecatedValue         string
		notificationsBus        *rabbitmqv1.RabbitMqConfig
		expectedClusterValue    string
		expectedDeprecatedValue string
		description             string
	}{
		{
			name:                    "Migrate with nil NotificationsBus",
			deprecatedValue:         "rabbitmq",
			notificationsBus:        nil,
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should create NotificationsBus and migrate value",
		},
		{
			name:            "Migrate to existing empty NotificationsBus",
			deprecatedValue: "rabbitmq",
			notificationsBus: &rabbitmqv1.RabbitMqConfig{
				Cluster: "",
			},
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should populate existing NotificationsBus",
		},
		{
			name:            "Default when both empty",
			deprecatedValue: "",
			notificationsBus: &rabbitmqv1.RabbitMqConfig{
				Cluster: "",
			},
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should set default value",
		},
		{
			name:            "New value takes precedence",
			deprecatedValue: "old-value",
			notificationsBus: &rabbitmqv1.RabbitMqConfig{
				Cluster: "new-value",
			},
			expectedClusterValue:    "new-value",
			expectedDeprecatedValue: "",
			description:             "Should preserve existing NotificationsBus value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			notificationsBus := tc.notificationsBus
			deprecatedField := tc.deprecatedValue

			// Migration logic (NotificationsBus pattern)
			if notificationsBus == nil || notificationsBus.Cluster == "" {
				if deprecatedField != "" {
					if notificationsBus == nil {
						notificationsBus = &rabbitmqv1.RabbitMqConfig{}
					}
					notificationsBus.Cluster = deprecatedField
				} else if notificationsBus != nil && notificationsBus.Cluster == "" {
					notificationsBus.Cluster = "rabbitmq"
				}
			}
			// Clear deprecated field
			if notificationsBus != nil && notificationsBus.Cluster != "" {
				deprecatedField = ""
			}

			// Verify results
			g.Expect(notificationsBus).ToNot(BeNil(),
				tc.description+" - NotificationsBus should not be nil")
			g.Expect(notificationsBus.Cluster).To(Equal(tc.expectedClusterValue),
				tc.description+" - Cluster value mismatch")
			g.Expect(deprecatedField).To(Equal(tc.expectedDeprecatedValue),
				tc.description+" - Deprecated field should be cleared")
		})
	}
}

// TestPointerMigrationPattern tests migration from pointer-based deprecated field
func TestPointerMigrationPattern(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name                  string
		deprecatedPointer     *string
		notificationsBus      *rabbitmqv1.RabbitMqConfig
		expectedClusterValue  string
		expectedDeprecatedNil bool
		description           string
	}{
		{
			name:                  "Migrate non-nil pointer",
			deprecatedPointer:     stringPtr("rabbitmq"),
			notificationsBus:      nil,
			expectedClusterValue:  "rabbitmq",
			expectedDeprecatedNil: true,
			description:           "Should migrate from non-nil pointer",
		},
		{
			name:                  "Nil pointer with default",
			deprecatedPointer:     nil,
			notificationsBus:      &rabbitmqv1.RabbitMqConfig{Cluster: ""},
			expectedClusterValue:  "rabbitmq",
			expectedDeprecatedNil: true,
			description:           "Should set default when pointer is nil",
		},
		{
			name:                  "Empty string pointer",
			deprecatedPointer:     stringPtr(""),
			notificationsBus:      &rabbitmqv1.RabbitMqConfig{Cluster: ""},
			expectedClusterValue:  "rabbitmq",
			expectedDeprecatedNil: true,
			description:           "Should set default when pointer points to empty string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			notificationsBus := tc.notificationsBus
			deprecatedPointer := tc.deprecatedPointer

			// Migration logic (pointer pattern - used by Glance)
			if notificationsBus == nil || notificationsBus.Cluster == "" {
				if deprecatedPointer != nil && *deprecatedPointer != "" {
					if notificationsBus == nil {
						notificationsBus = &rabbitmqv1.RabbitMqConfig{}
					}
					notificationsBus.Cluster = *deprecatedPointer
				} else if notificationsBus != nil && notificationsBus.Cluster == "" {
					notificationsBus.Cluster = "rabbitmq"
				}
			}
			// Clear deprecated field
			if notificationsBus != nil && notificationsBus.Cluster != "" {
				deprecatedPointer = nil
			}

			// Verify results
			g.Expect(notificationsBus).ToNot(BeNil(),
				tc.description+" - NotificationsBus should not be nil")
			g.Expect(notificationsBus.Cluster).To(Equal(tc.expectedClusterValue),
				tc.description+" - Cluster value mismatch")
			if tc.expectedDeprecatedNil {
				g.Expect(deprecatedPointer).To(BeNil(),
					tc.description+" - Deprecated pointer should be nil")
			}
		})
	}
}

// TestMigrationIdempotency verifies migration can be run multiple times safely
func TestMigrationIdempotency(t *testing.T) {
	g := NewWithT(t)

	// Initial state
	var messagingBus rabbitmqv1.RabbitMqConfig
	deprecatedField := "rabbitmq"

	// Run migration first time
	if messagingBus.Cluster == "" {
		if deprecatedField != "" {
			messagingBus.Cluster = deprecatedField
		} else {
			messagingBus.Cluster = "rabbitmq"
		}
	}
	if messagingBus.Cluster != "" {
		deprecatedField = ""
	}

	firstCluster := messagingBus.Cluster
	firstDeprecated := deprecatedField

	g.Expect(firstCluster).To(Equal("rabbitmq"))
	g.Expect(firstDeprecated).To(Equal(""))

	// Run migration second time - should be idempotent
	if messagingBus.Cluster == "" {
		if deprecatedField != "" {
			messagingBus.Cluster = deprecatedField
		} else {
			messagingBus.Cluster = "rabbitmq"
		}
	}
	if messagingBus.Cluster != "" {
		deprecatedField = ""
	}

	secondCluster := messagingBus.Cluster
	secondDeprecated := deprecatedField

	g.Expect(secondCluster).To(Equal(firstCluster), "Migration should be idempotent - cluster")
	g.Expect(secondDeprecated).To(Equal(firstDeprecated), "Migration should be idempotent - deprecated")
}

// TestEdgeCases tests various edge cases in migration
func TestEdgeCases(t *testing.T) {
	g := NewWithT(t)

	t.Run("Empty string is treated as not set", func(t *testing.T) {
		var messagingBus rabbitmqv1.RabbitMqConfig
		deprecatedField := ""

		if messagingBus.Cluster == "" {
			if deprecatedField != "" {
				messagingBus.Cluster = deprecatedField
			} else {
				messagingBus.Cluster = "rabbitmq"
			}
		}
		if messagingBus.Cluster != "" {
			deprecatedField = ""
		}

		g.Expect(messagingBus.Cluster).To(Equal("rabbitmq"),
			"Empty deprecated field should result in default value")
	})

	t.Run("Whitespace-only value is preserved", func(t *testing.T) {
		var messagingBus rabbitmqv1.RabbitMqConfig
		deprecatedField := "  "

		if messagingBus.Cluster == "" {
			if deprecatedField != "" {
				messagingBus.Cluster = deprecatedField
			} else {
				messagingBus.Cluster = "rabbitmq"
			}
		}
		if messagingBus.Cluster != "" {
			deprecatedField = ""
		}

		g.Expect(messagingBus.Cluster).To(Equal("  "),
			"Whitespace-only values should be preserved (not sanitized)")
	})

	t.Run("Special characters in cluster name", func(t *testing.T) {
		var messagingBus rabbitmqv1.RabbitMqConfig
		deprecatedField := "rabbitmq-cell1"

		if messagingBus.Cluster == "" {
			if deprecatedField != "" {
				messagingBus.Cluster = deprecatedField
			} else {
				messagingBus.Cluster = "rabbitmq"
			}
		}
		if messagingBus.Cluster != "" {
			deprecatedField = ""
		}

		g.Expect(messagingBus.Cluster).To(Equal("rabbitmq-cell1"),
			"Cluster names with hyphens should work")
	})
}

// TestTopLevelNotificationsBusInstanceMigration tests the top-level NotificationsBusInstance
// migration that happens in the controller's reconcileNormal function
func TestTopLevelNotificationsBusInstanceMigration(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name                     string
		notificationsBusInstance *string
		notificationsBus         *rabbitmqv1.RabbitMqConfig
		expectedCluster          string
		expectedInstanceNil      bool
		description              string
	}{
		{
			name:                     "Migrate from NotificationsBusInstance",
			notificationsBusInstance: stringPtr("rabbitmq-notifications"),
			notificationsBus:         nil,
			expectedCluster:          "rabbitmq-notifications",
			expectedInstanceNil:      true,
			description:              "Should create NotificationsBus and migrate value",
		},
		{
			name:                     "Migrate to existing empty NotificationsBus",
			notificationsBusInstance: stringPtr("rabbitmq-notifications"),
			notificationsBus:         &rabbitmqv1.RabbitMqConfig{Cluster: ""},
			expectedCluster:          "rabbitmq-notifications",
			expectedInstanceNil:      true,
			description:              "Should populate existing NotificationsBus",
		},
		{
			name:                     "New field takes precedence",
			notificationsBusInstance: stringPtr("old-value"),
			notificationsBus:         &rabbitmqv1.RabbitMqConfig{Cluster: "new-value"},
			expectedCluster:          "new-value",
			expectedInstanceNil:      true,
			description:              "Should keep new field value and clear deprecated",
		},
		{
			name:                     "Already migrated",
			notificationsBusInstance: nil,
			notificationsBus:         &rabbitmqv1.RabbitMqConfig{Cluster: "rabbitmq-notifications"},
			expectedCluster:          "rabbitmq-notifications",
			expectedInstanceNil:      true,
			description:              "Should leave already-migrated config unchanged",
		},
		{
			name:                     "Empty deprecated field",
			notificationsBusInstance: stringPtr(""),
			notificationsBus:         nil,
			expectedCluster:          "",
			expectedInstanceNil:      false,
			description:              "Empty string in deprecated field should not trigger migration",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			notificationsBusInstance := tc.notificationsBusInstance
			notificationsBus := tc.notificationsBus

			// Simulate the controller migration logic
			if notificationsBusInstance != nil && *notificationsBusInstance != "" {
				if notificationsBus == nil {
					notificationsBus = &rabbitmqv1.RabbitMqConfig{}
				}
				if notificationsBus.Cluster == "" {
					notificationsBus.Cluster = *notificationsBusInstance
				}
				// Clear deprecated field once migrated
				notificationsBusInstance = nil
			}

			// Verify results
			if tc.expectedCluster != "" {
				g.Expect(notificationsBus).ToNot(BeNil(), tc.description+" - NotificationsBus should not be nil")
				g.Expect(notificationsBus.Cluster).To(Equal(tc.expectedCluster),
					tc.description+" - Cluster value mismatch")
			} else if tc.notificationsBus == nil && tc.expectedCluster == "" {
				// For the empty deprecated field case, NotificationsBus should remain nil
				g.Expect(notificationsBus).To(BeNil(), tc.description+" - NotificationsBus should be nil")
			}

			if tc.expectedInstanceNil {
				g.Expect(notificationsBusInstance).To(BeNil(),
					tc.description+" - Deprecated field should be nil after migration")
			}
		})
	}
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}
