/*
Copyright 2023.

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

package functional_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"

	openstackclientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
)

var _ = Describe("OpenStackClient Webhook", func() {

	var openstackclientName types.NamespacedName

	BeforeEach(func() {

		openstackclientName = types.NamespacedName{
			Name:      "foo",
			Namespace: namespace,
		}

		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())
	})

	When("A OpenStackClient instance is created without container images", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance, CreateOpenStackClient(openstackclientName, GetDefaultOpenStackClientSpec()))
		})

		It("should have the defaults initialized by webhook", func() {
			OpenStackClient := GetOpenStackClient(openstackclientName)
			Expect(OpenStackClient.Spec.ContainerImage).Should(Equal(
				openstackclientv1.OpenStackClientContainerImage,
			))
		})
	})

	When("A OpenStackClient instance is created with container images", func() {
		BeforeEach(func() {
			openstackclientSpec := GetDefaultOpenStackClientSpec()
			openstackclientSpec["containerImage"] = "api-container-image"
			DeferCleanup(th.DeleteInstance, CreateOpenStackClient(openstackclientName, openstackclientSpec))
		})

		It("should use the given values", func() {
			OpenStackClient := GetOpenStackClient(openstackclientName)
			Expect(OpenStackClient.Spec.ContainerImage).Should(Equal(
				"api-container-image",
			))
		})
	})
})
