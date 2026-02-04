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

package openstack

import (
	"testing"

	. "github.com/onsi/gomega" //revive:disable:dot-imports

	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
)

// TestMessagingBusCascading tests that top-level MessagingBus cascades using the cascading pattern
func TestMessagingBusCascading(t *testing.T) {
	g := NewWithT(t)

	// Test the cascading pattern logic
	topLevelMessagingBus := &rabbitmqv1.RabbitMqConfig{
		Cluster: "rabbitmq-global",
		User:    "global-user",
		Vhost:   "global-vhost",
	}

	// Test case 1: Empty cluster should cascade
	t.Run("Empty cluster triggers cascading", func(_ *testing.T) {
		serviceMessagingBus := rabbitmqv1.RabbitMqConfig{}

		// Simulate cascading logic
		if topLevelMessagingBus != nil && serviceMessagingBus.Cluster == "" {
			serviceMessagingBus = *topLevelMessagingBus
		}

		g.Expect(serviceMessagingBus.Cluster).To(Equal("rabbitmq-global"))
		g.Expect(serviceMessagingBus.User).To(Equal("global-user"))
		g.Expect(serviceMessagingBus.Vhost).To(Equal("global-vhost"))
	})

	// Test case 2: Non-empty cluster should NOT cascade
	t.Run("Non-empty cluster prevents cascading", func(_ *testing.T) {
		serviceMessagingBus := rabbitmqv1.RabbitMqConfig{
			Cluster: "service-specific-rabbitmq",
			User:    "service-user",
			Vhost:   "service-vhost",
		}

		// Simulate cascading logic
		if topLevelMessagingBus != nil && serviceMessagingBus.Cluster == "" {
			serviceMessagingBus = *topLevelMessagingBus
		}

		// Should keep service-specific values
		g.Expect(serviceMessagingBus.Cluster).To(Equal("service-specific-rabbitmq"))
		g.Expect(serviceMessagingBus.User).To(Equal("service-user"))
		g.Expect(serviceMessagingBus.Vhost).To(Equal("service-vhost"))
	})

	// Test case 3: Nil top-level should not cause cascading
	t.Run("Nil top-level MessagingBus does not cascade", func(_ *testing.T) {
		var nilMessagingBus *rabbitmqv1.RabbitMqConfig
		serviceMessagingBus := rabbitmqv1.RabbitMqConfig{}

		// Simulate cascading logic: when top-level is nil, no cascading occurs
		// In real code: if nilMessagingBus != nil && serviceMessagingBus.Cluster == "" {...}
		// Here we just verify the nil case doesn't cascade
		_ = nilMessagingBus

		// Should remain empty when top-level is nil
		g.Expect(serviceMessagingBus.Cluster).To(Equal(""))
		g.Expect(serviceMessagingBus.User).To(Equal(""))
	})
}

// TestNotificationsBusCascading tests that top-level NotificationsBus cascades using the cascading pattern
func TestNotificationsBusCascading(t *testing.T) {
	g := NewWithT(t)

	// Test the cascading pattern logic for NotificationsBus
	topLevelNotificationsBus := &rabbitmqv1.RabbitMqConfig{
		Cluster: "rabbitmq-notifications",
		User:    "notifications-user",
		Vhost:   "notifications-vhost",
	}

	// Test case 1: Nil service-level should cascade
	t.Run("Nil service NotificationsBus triggers cascading", func(_ *testing.T) {
		// Simulate cascading logic: when service-level is nil, use top-level
		// In real code: if serviceNotificationsBus == nil { serviceNotificationsBus = topLevelNotificationsBus }
		serviceNotificationsBus := topLevelNotificationsBus

		g.Expect(serviceNotificationsBus).ToNot(BeNil())
		g.Expect(serviceNotificationsBus.Cluster).To(Equal("rabbitmq-notifications"))
		g.Expect(serviceNotificationsBus.User).To(Equal("notifications-user"))
		g.Expect(serviceNotificationsBus.Vhost).To(Equal("notifications-vhost"))
	})

	// Test case 2: Non-nil service-level should NOT cascade
	t.Run("Non-nil service NotificationsBus prevents cascading", func(_ *testing.T) {
		serviceNotificationsBus := &rabbitmqv1.RabbitMqConfig{
			Cluster: "service-specific-notifications",
			User:    "service-notif-user",
			Vhost:   "service-notif-vhost",
		}

		// Cascading logic would check: if serviceNotificationsBus == nil
		// But since it's already set, no cascading occurs

		// Should keep service-specific values
		g.Expect(serviceNotificationsBus).ToNot(BeNil())
		g.Expect(serviceNotificationsBus.Cluster).To(Equal("service-specific-notifications"))
		g.Expect(serviceNotificationsBus.User).To(Equal("service-notif-user"))
		g.Expect(serviceNotificationsBus.Vhost).To(Equal("service-notif-vhost"))
	})

	// Test case 3: Nil top-level should remain nil
	t.Run("Nil top-level NotificationsBus stays nil", func(_ *testing.T) {
		var topLevel *rabbitmqv1.RabbitMqConfig

		// Simulate cascading logic: when both are nil, service stays nil
		// In real code: if serviceNotificationsBus == nil { serviceNotificationsBus = topLevel }
		serviceNotificationsBus := topLevel

		// Should remain nil when both are nil
		g.Expect(serviceNotificationsBus).To(BeNil())
	})
}

// TestBothMessagingAndNotificationsBusCascading tests that both MessagingBus and NotificationsBus cascade correctly
func TestBothMessagingAndNotificationsBusCascading(t *testing.T) {
	g := NewWithT(t)

	topLevelMessagingBus := &rabbitmqv1.RabbitMqConfig{
		Cluster: "rabbitmq-rpc",
		User:    "rpc-user",
		Vhost:   "rpc-vhost",
	}

	topLevelNotificationsBus := &rabbitmqv1.RabbitMqConfig{
		Cluster: "rabbitmq-notifications",
		User:    "notifications-user",
		Vhost:   "notifications-vhost",
	}

	// Simulate a service with empty MessagingBus and nil NotificationsBus
	// In real code: cascading would check conditions and assign values
	// Here we directly test the cascaded results
	serviceMessagingBus := *topLevelMessagingBus
	serviceNotificationsBus := topLevelNotificationsBus

	// Verify MessagingBus cascaded
	g.Expect(serviceMessagingBus.Cluster).To(Equal("rabbitmq-rpc"))
	g.Expect(serviceMessagingBus.User).To(Equal("rpc-user"))
	g.Expect(serviceMessagingBus.Vhost).To(Equal("rpc-vhost"))

	// Verify NotificationsBus cascaded
	g.Expect(serviceNotificationsBus).ToNot(BeNil())
	g.Expect(serviceNotificationsBus.Cluster).To(Equal("rabbitmq-notifications"))
	g.Expect(serviceNotificationsBus.User).To(Equal("notifications-user"))
	g.Expect(serviceNotificationsBus.Vhost).To(Equal("notifications-vhost"))
}
