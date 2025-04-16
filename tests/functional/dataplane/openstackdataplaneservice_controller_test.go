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
package functional

import (
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("OpenstackDataplaneService Test", func() {
	var dataplaneServiceName types.NamespacedName
	BeforeEach(func() {
		dataplaneServiceName = types.NamespacedName{
			Namespace: namespace,
			Name:      "configure-network",
		}
	})

	When("A defined service resource is created", func() {
		BeforeEach(func() {
			_ = os.Unsetenv("OPERATOR_SERVICES")
			CreateDataplaneService(dataplaneServiceName, false)
			DeferCleanup(th.DeleteService, dataplaneServiceName)
		})

		It("spec fields are set up", func() {
			service := GetService(dataplaneServiceName)
			Expect(service.Spec.DataSources).To(BeEmpty())
			Expect(service.Spec.PlaybookContents).To(BeEmpty())
			Expect(service.Spec.Role).To(BeEmpty())
			Expect(service.Spec.DeployOnAllNodeSets).To(BeFalse())
		})
	})

	When("A defined service resource for all nodes is created", func() {
		BeforeEach(func() {
			_ = os.Unsetenv("OPERATOR_SERVICES")
			CreateDataplaneService(dataplaneServiceName, true)
			DeferCleanup(th.DeleteService, dataplaneServiceName)
		})

		It("spec fields are set up", func() {
			service := GetService(dataplaneServiceName)
			Expect(service.Spec.DataSources).To(BeEmpty())
			Expect(service.Spec.PlaybookContents).To(BeEmpty())
			Expect(service.Spec.Role).To(BeEmpty())
			Expect(service.Spec.DeployOnAllNodeSets).To(BeTrue())
		})
	})
})
