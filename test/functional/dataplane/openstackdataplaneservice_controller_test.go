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

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
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

	When("A service with TLSCerts including system-id CommonName is created", func() {
		BeforeEach(func() {
			_ = os.Unsetenv("OPERATOR_SERVICES")
			DeferCleanup(th.DeleteInstance, CreateDataPlaneServiceFromSpec(
				dataplaneServiceName,
				map[string]interface{}{
					"edpmServiceType": "ovn",
					"tlsCerts": map[string]interface{}{
						"default": map[string]interface{}{
							"contents": []string{"dnsnames", "ips"},
							"issuer":   "osp-rootca-issuer-ovn",
							"keyUsages": []string{
								"digital signature",
								"key encipherment",
								"server auth",
								"client auth",
							},
						},
						"rbac": map[string]interface{}{
							"commonName": "system-id",
							"issuer":     "osp-rootca-issuer-ovn",
							"keyUsages": []string{
								"digital signature",
								"client auth",
							},
						},
					},
				}))
			DeferCleanup(th.DeleteService, dataplaneServiceName)
		})

		It("should store TLSCerts with CommonName and empty Contents", func() {
			service := GetService(dataplaneServiceName)

			Expect(service.Spec.TLSCerts).To(HaveLen(2))

			defaultCert := service.Spec.TLSCerts["default"]
			Expect(defaultCert.Contents).To(ConsistOf("dnsnames", "ips"))
			Expect(defaultCert.Issuer).To(Equal("osp-rootca-issuer-ovn"))
			Expect(defaultCert.CommonName).To(BeEmpty())
			Expect(defaultCert.KeyUsages).To(ContainElements(
				certmgrv1.UsageServerAuth,
				certmgrv1.UsageClientAuth,
			))

			rbacCert := service.Spec.TLSCerts["rbac"]
			Expect(rbacCert.CommonName).To(Equal("system-id"))
			Expect(rbacCert.Contents).To(BeEmpty())
			Expect(rbacCert.Issuer).To(Equal("osp-rootca-issuer-ovn"))
			Expect(rbacCert.KeyUsages).To(ContainElements(
				certmgrv1.UsageDigitalSignature,
				certmgrv1.UsageClientAuth,
			))
			Expect(rbacCert.KeyUsages).ToNot(ContainElement(
				certmgrv1.UsageServerAuth,
			))
		})
	})
})
