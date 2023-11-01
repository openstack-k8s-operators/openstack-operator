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
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"k8s.io/utils/ptr"
)

var _ = Describe("OpenStackOperator controller", func() {
	BeforeEach(func() {
		// lib-common uses OPERATOR_TEMPLATES env var to locate the "templates"
		// directory of the operator. We need to set them othervise lib-common
		// will fail to generate the ConfigMap as it does not find common.sh
		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())
	})

	When("A default OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			// (mschuppert) create root CA secret as there is no certmanager running.
			// it is not used, just to make sure reconcile proceeds and creates the ca-bundle.
			Eventually(func(g Gomega) {
				th.CreateSecret(
					names.RootCAPublicName,
					map[string][]byte{
						"ca.crt":  []byte("test"),
						"tls.crt": []byte("test"),
						"tls.key": []byte("test"),
					})
			}, timeout, interval).Should(Succeed())

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, GetDefaultOpenStackControlPlaneSpec()),
			)
		})

		It("should have the Spec fields defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Galera.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Rabbitmq.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Memcached.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Keystone.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Endpoint[service.EndpointPublic].Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Endpoint[service.EndpointInternal].Enabled).Should(BeFalse())

			// galera exists
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(db).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBCell1Name)
				g.Expect(db).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// memcached exists
			Eventually(func(g Gomega) {
				memcached := infra.GetMemcached(names.MemcachedName)
				g.Expect(memcached).Should(Not(BeNil()))
				g.Expect(memcached.Spec.Replicas).Should(Equal(ptr.To[int32](1)))
			}, timeout, interval).Should(Succeed())

			// TODO rabbitmq exists

			// keystone exists
			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
		})

		It("should create selfsigned issuer and public CA and issuer", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)

			Expect(OSCtlplane.Spec.TLS.Endpoint[service.EndpointPublic].Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Endpoint[service.EndpointInternal].Enabled).Should(BeFalse())

			// creates selfsigned issuer
			Eventually(func(g Gomega) {
				crtmgr.GetIssuer(names.SelfSignedIssuerName)
			}, timeout, interval).Should(Succeed())

			// creates public root CA and issuer
			Eventually(func(g Gomega) {
				// ca cert
				cert := crtmgr.GetCert(names.RootCAPublicName)
				g.Expect(cert).Should(Not(BeNil()))
				g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCAPublicName.Name))
				g.Expect(cert.Spec.IsCA).Should(BeTrue())
				g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
				g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCAPublicName.Name))
				// issuer
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCAPublicName.Name))

			}, timeout, interval).Should(Succeed())
		})

		It("should create ca bundle", func() {
			crtmgr.GetCert(names.RootCAPublicName)
			crtmgr.GetIssuer(names.RootCAPublicName)

			Eventually(func(g Gomega) {
				th.GetSecret(names.RootCAPublicName)
				caBundle := th.GetSecret(names.CABundleName)
				g.Expect(caBundle.Data).Should(HaveLen(int(1)))
				// g.Expect(len(caBundle.Data)).Should(Equal(int(1)))
			}, timeout, interval).Should(Succeed())
		})
	})
})
