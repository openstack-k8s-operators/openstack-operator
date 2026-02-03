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

// TestMessagingBusMigrationWithInheritance tests the complete MessagingBus migration and inheritance logic
// This tests the pattern used in service reconcilers (e.g., ReconcileCinder, ReconcileNova, etc.)
func TestMessagingBusMigrationWithInheritance(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name                    string
		deprecatedValue         string                     // Service-level rabbitMqClusterName
		serviceMessagingBus     rabbitmqv1.RabbitMqConfig  // Service-level messagingBus
		topLevelMessagingBus    *rabbitmqv1.RabbitMqConfig // Top-level messagingBus from OpenStackControlPlane
		expectedClusterValue    string
		expectedDeprecatedValue string
		description             string
	}{
		// Scenario 1: Migration from deprecated field
		{
			name:                    "Migrate from service-level deprecated rabbitMqClusterName",
			deprecatedValue:         "custom-rabbitmq",
			serviceMessagingBus:     rabbitmqv1.RabbitMqConfig{Cluster: ""},
			topLevelMessagingBus:    nil,
			expectedClusterValue:    "custom-rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should migrate deprecated rabbitMqClusterName to messagingBus.cluster",
		},
		{
			name:                    "Migrate deprecated value even when top-level exists",
			deprecatedValue:         "custom-rabbitmq",
			serviceMessagingBus:     rabbitmqv1.RabbitMqConfig{Cluster: ""},
			topLevelMessagingBus:    &rabbitmqv1.RabbitMqConfig{Cluster: "top-level-rabbitmq"},
			expectedClusterValue:    "custom-rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Migration takes precedence over top-level inheritance",
		},

		// Scenario 2: Inheritance from top-level
		{
			name:                    "Inherit from top-level when service-level is empty",
			deprecatedValue:         "",
			serviceMessagingBus:     rabbitmqv1.RabbitMqConfig{Cluster: ""},
			topLevelMessagingBus:    &rabbitmqv1.RabbitMqConfig{Cluster: "top-level-rabbitmq"},
			expectedClusterValue:    "top-level-rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should inherit from top-level messagingBus when service-level is empty",
		},
		{
			name:                "Inherit from top-level with user/vhost",
			deprecatedValue:     "",
			serviceMessagingBus: rabbitmqv1.RabbitMqConfig{Cluster: ""},
			topLevelMessagingBus: &rabbitmqv1.RabbitMqConfig{
				Cluster: "top-level-rabbitmq",
				User:    "custom-user",
				Vhost:   "custom-vhost",
			},
			expectedClusterValue:    "top-level-rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should inherit entire top-level messagingBus config",
		},

		// Scenario 3: Default when nothing is set
		{
			name:                    "Default when no deprecated, no top-level, service-level empty",
			deprecatedValue:         "",
			serviceMessagingBus:     rabbitmqv1.RabbitMqConfig{Cluster: ""},
			topLevelMessagingBus:    nil,
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should default to 'rabbitmq' when nothing is configured",
		},
		{
			name:                    "Default when top-level has empty cluster",
			deprecatedValue:         "",
			serviceMessagingBus:     rabbitmqv1.RabbitMqConfig{Cluster: ""},
			topLevelMessagingBus:    &rabbitmqv1.RabbitMqConfig{Cluster: ""},
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should default when top-level exists but cluster is empty",
		},

		// Edge cases
		{
			name:                    "Service-level new field takes precedence over everything",
			deprecatedValue:         "old-rabbitmq",
			serviceMessagingBus:     rabbitmqv1.RabbitMqConfig{Cluster: "service-rabbitmq"},
			topLevelMessagingBus:    &rabbitmqv1.RabbitMqConfig{Cluster: "top-level-rabbitmq"},
			expectedClusterValue:    "service-rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Explicit service-level value overrides deprecated and top-level",
		},
		{
			name:                    "Already migrated - no changes",
			deprecatedValue:         "",
			serviceMessagingBus:     rabbitmqv1.RabbitMqConfig{Cluster: "rabbitmq"},
			topLevelMessagingBus:    &rabbitmqv1.RabbitMqConfig{Cluster: "top-level-rabbitmq"},
			expectedClusterValue:    "rabbitmq",
			expectedDeprecatedValue: "",
			description:             "Should not override already-migrated service-level config",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Copy inputs to simulate the reconciler's behavior
			messagingBus := tc.serviceMessagingBus
			deprecatedField := tc.deprecatedValue
			topLevelBus := tc.topLevelMessagingBus

			// Apply the migration and inheritance logic (matches the pattern in cinder.go, nova.go, etc.)
			if messagingBus.Cluster == "" {
				// Priority 1: Migrate from service-level deprecated field
				if deprecatedField != "" {
					messagingBus.Cluster = deprecatedField
					// Priority 2: Inherit from top-level
				} else if topLevelBus != nil && topLevelBus.Cluster != "" {
					messagingBus = *topLevelBus
					// Priority 3: Default
				} else {
					messagingBus.Cluster = "rabbitmq"
				}
			}

			// Clear deprecated field after migration
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

// TestMessagingBusInheritanceFullStruct verifies that entire RabbitMqConfig is inherited, not just Cluster
func TestMessagingBusInheritanceFullStruct(t *testing.T) {
	g := NewWithT(t)

	topLevelBus := &rabbitmqv1.RabbitMqConfig{
		Cluster: "top-level-cluster",
		User:    "top-level-user",
		Vhost:   "top-level-vhost",
	}

	// Simulate service-level empty messagingBus
	messagingBus := rabbitmqv1.RabbitMqConfig{Cluster: ""}
	deprecatedField := ""

	// Apply inheritance logic
	if messagingBus.Cluster == "" {
		if deprecatedField != "" {
			messagingBus.Cluster = deprecatedField
		} else if topLevelBus != nil && topLevelBus.Cluster != "" {
			messagingBus = *topLevelBus // Copy entire struct
		} else {
			messagingBus.Cluster = "rabbitmq"
		}
	}

	// Verify entire struct is inherited
	g.Expect(messagingBus.Cluster).To(Equal("top-level-cluster"))
	g.Expect(messagingBus.User).To(Equal("top-level-user"))
	g.Expect(messagingBus.Vhost).To(Equal("top-level-vhost"))
}

// TestNotificationsBusMigrationWithInheritance tests NotificationsBus migration and inheritance
// NotificationsBus is a pointer field, so the logic is slightly different
func TestNotificationsBusMigrationWithInheritance(t *testing.T) {
	g := NewWithT(t)

	testCases := []struct {
		name                     string
		deprecatedValue          *string // pointer to string
		serviceNotificationsBus  *rabbitmqv1.RabbitMqConfig
		topLevelNotificationsBus *rabbitmqv1.RabbitMqConfig
		expectedClusterValue     string
		expectedNil              bool
		description              string
	}{
		{
			name:                     "Migrate from deprecated pointer field",
			deprecatedValue:          ptrString("custom-notifications"),
			serviceNotificationsBus:  nil,
			topLevelNotificationsBus: nil,
			expectedClusterValue:     "custom-notifications",
			expectedNil:              false,
			description:              "Should create NotificationsBus and migrate from deprecated",
		},
		{
			name:                     "Inherit from top-level when service is nil",
			deprecatedValue:          nil,
			serviceNotificationsBus:  nil,
			topLevelNotificationsBus: &rabbitmqv1.RabbitMqConfig{Cluster: "top-notif"},
			expectedClusterValue:     "top-notif",
			expectedNil:              false,
			description:              "Should inherit top-level NotificationsBus",
		},
		{
			name:                     "No default for NotificationsBus (optional field)",
			deprecatedValue:          nil,
			serviceNotificationsBus:  nil,
			topLevelNotificationsBus: nil,
			expectedNil:              true,
			description:              "NotificationsBus should remain nil when not configured",
		},
		{
			name:                     "Migration takes precedence over top-level",
			deprecatedValue:          ptrString("migrated-notif"),
			serviceNotificationsBus:  nil,
			topLevelNotificationsBus: &rabbitmqv1.RabbitMqConfig{Cluster: "top-notif"},
			expectedClusterValue:     "migrated-notif",
			expectedNil:              false,
			description:              "Deprecated field migration overrides top-level",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Copy inputs
			notificationsBus := tc.serviceNotificationsBus
			deprecatedField := tc.deprecatedValue
			topLevelBus := tc.topLevelNotificationsBus

			// Apply migration and inheritance logic for NotificationsBus
			// NotificationsBus is optional (can be nil), so we don't default it
			if notificationsBus == nil || notificationsBus.Cluster == "" {
				// Priority 1: Migrate from deprecated field
				if deprecatedField != nil && *deprecatedField != "" {
					if notificationsBus == nil {
						notificationsBus = &rabbitmqv1.RabbitMqConfig{}
					}
					notificationsBus.Cluster = *deprecatedField
					// Priority 2: Inherit from top-level
				} else if topLevelBus != nil && topLevelBus.Cluster != "" {
					notificationsBus = topLevelBus
				}
				// No Priority 3 default - NotificationsBus is optional
			}

			// Verify results
			if tc.expectedNil {
				g.Expect(notificationsBus).To(BeNil(), tc.description+" - should be nil")
			} else {
				g.Expect(notificationsBus).ToNot(BeNil(), tc.description+" - should not be nil")
				g.Expect(notificationsBus.Cluster).To(Equal(tc.expectedClusterValue),
					tc.description+" - Cluster value mismatch")
			}
		})
	}
}

// Helper function to create pointer to string
func ptrString(s string) *string {
	return &s
}
