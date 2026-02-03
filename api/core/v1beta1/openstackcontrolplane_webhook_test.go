package v1beta1

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("OpenStackControlPlane Webhook", func() {

	Context("ValidateMessagingBusConfig", func() {
		var instance *OpenStackControlPlane
		var basePath *field.Path

		BeforeEach(func() {
			instance = &OpenStackControlPlane{
				Spec: OpenStackControlPlaneSpec{},
			}
			basePath = field.NewPath("spec")
		})

		It("should allow only Cluster field in messagingBus", func() {
			instance.Spec.MessagingBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(BeEmpty())
		})

		It("should allow Cluster and Vhost fields in messagingBus", func() {
			instance.Spec.MessagingBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq",
				Vhost:   "/openstack",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(BeEmpty())
		})

		It("should reject User field in messagingBus", func() {
			instance.Spec.MessagingBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq",
				User:    "shared-user",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeForbidden))
			Expect(errs[0].Field).To(Equal("spec.messagingBus.user"))
			Expect(errs[0].Detail).To(ContainSubstring("user field is not allowed at the top level"))
		})

		It("should reject User field even with other valid fields in messagingBus", func() {
			instance.Spec.MessagingBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq",
				Vhost:   "/openstack",
				User:    "shared-user",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeForbidden))
			Expect(errs[0].Field).To(Equal("spec.messagingBus.user"))
		})

		It("should allow only Cluster field in notificationsBus", func() {
			instance.Spec.NotificationsBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq-notifications",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(BeEmpty())
		})

		It("should allow Cluster and Vhost fields in notificationsBus", func() {
			instance.Spec.NotificationsBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq-notifications",
				Vhost:   "/notifications",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(BeEmpty())
		})

		It("should reject User field in notificationsBus", func() {
			instance.Spec.NotificationsBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq-notifications",
				User:    "shared-user",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(HaveLen(1))
			Expect(errs[0].Type).To(Equal(field.ErrorTypeForbidden))
			Expect(errs[0].Field).To(Equal("spec.notificationsBus.user"))
			Expect(errs[0].Detail).To(ContainSubstring("user field is not allowed at the top level"))
		})

		It("should reject User field in both messagingBus and notificationsBus", func() {
			instance.Spec.MessagingBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq",
				User:    "rpc-user",
			}
			instance.Spec.NotificationsBus = &rabbitmqv1.RabbitMqConfig{
				Cluster: "rabbitmq-notifications",
				User:    "notif-user",
			}

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(HaveLen(2))
			Expect(errs[0].Field).To(Equal("spec.messagingBus.user"))
			Expect(errs[1].Field).To(Equal("spec.notificationsBus.user"))
		})

		It("should allow nil messagingBus and notificationsBus", func() {
			instance.Spec.MessagingBus = nil
			instance.Spec.NotificationsBus = nil

			errs := instance.ValidateMessagingBusConfig(basePath)
			Expect(errs).To(BeEmpty())
		})
	})
})
