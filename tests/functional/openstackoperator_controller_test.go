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
	"errors"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	routev1 "github.com/openshift/api/route/v1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
)

var _ = Describe("OpenStackOperator controller", func() {
	BeforeEach(func() {
		// lib-common uses OPERATOR_TEMPLATES env var to locate the "templates"
		// directory of the operator. We need to set them othervise lib-common
		// will fail to generate the ConfigMap as it does not find common.sh
		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())

		// create cluster config map which is used to validate if cluster supports fips
		DeferCleanup(k8sClient.Delete, ctx, CreateClusterConfigCM())

		// (mschuppert) create root CA secrets as there is no certmanager running.
		// it is not used, just to make sure reconcile proceeds and creates the ca-bundle.
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAPublicName))
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAInternalName))
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAOvnName))
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCALibvirtName))
		// create cert secrets for galera instances
		DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.DBCertName))
		DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.DBCell1CertName))
	})

	//
	// Validate TLS input settings
	//
	When("TLS - A public TLS OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSPublicSpec()
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A public TLS OpenStackControlplane instance is created with customized ca duration", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			tlsSpec := GetTLSPublicSpec()
			tlsSpec["ingress"] = map[string]interface{}{
				"ca": map[string]interface{}{
					"duration": "100h",
				},
			}
			spec["tls"] = tlsSpec

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(100)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A public TLS OpenStackControlplane instance is created with customized cert duration", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			tlsSpec := GetTLSPublicSpec()
			tlsSpec["ingress"] = map[string]interface{}{
				"cert": map[string]interface{}{
					"duration": "10h",
				},
			}
			spec["tls"] = tlsSpec

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(10)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "10h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A public TLS OpenStackControlplane instance is created with customized ca and cert duration", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			tlsSpec := GetTLSPublicSpec()
			tlsSpec["ingress"] = map[string]interface{}{
				"ca": map[string]interface{}{
					"duration": "100h",
				},
				"cert": map[string]interface{}{
					"duration": "10h",
				},
			}
			spec["tls"] = tlsSpec

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(100)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(10)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "10h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A public TLS OpenStackControlplane instance is created with customized renewBefore", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			tlsSpec := GetTLSPublicSpec()
			tlsSpec["ingress"] = map[string]interface{}{
				"ca": map[string]interface{}{
					"renewBefore": "100h",
				},
				"cert": map[string]interface{}{
					"renewBefore": "10h",
				},
			}
			spec["tls"] = tlsSpec

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.RenewBefore.Duration.Hours()).Should(Equal(float64(100)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.RenewBefore.Duration.Hours()).Should(Equal(float64(10)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertRenewBeforeAnnotation, "10h0m0s"))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A public TLS OpenStackControlplane instance is created with a custom issuer", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			tlsSpec := GetTLSPublicSpec()
			tlsSpec["ingress"] = map[string]interface{}{
				"ca": map[string]interface{}{
					"customIssuer": "myissuer",
				},
			}
			spec["tls"] = tlsSpec

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.CustomIssuer).Should(Not(BeNil()))
			Expect(*OSCtlplane.Spec.TLS.Ingress.Ca.CustomIssuer).Should(Equal("myissuer"))
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())
		})
	})
	When("TLS - A TLSe OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAInternalName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCALibvirtName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAOvnName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A TLSe OpenStackControlplane instance is created with customized internal ca duration", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = map[string]interface{}{
				"podLevel": map[string]interface{}{
					"internal": map[string]interface{}{
						"ca": map[string]interface{}{
							"duration": "100h",
						},
					},
				},
			}
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Ca.Duration.Duration.Hours()).Should(Equal(float64(100)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAInternalName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCALibvirtName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAOvnName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A TLSe OpenStackControlplane instance is created with customized internal cert duration", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = map[string]interface{}{
				"podLevel": map[string]interface{}{
					"internal": map[string]interface{}{
						"cert": map[string]interface{}{
							"duration": "10h",
						},
					},
				},
			}
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration.Hours()).Should(Equal(float64(10)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAPublicName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAInternalName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "10h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCALibvirtName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				issuer := crtmgr.GetIssuer(names.RootCAOvnName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Annotations).Should(HaveKeyWithValue(certmanager.CertDurationAnnotation, "43800h0m0s"))
				g.Expect(issuer.Annotations).Should(Not(HaveKey(certmanager.CertRenewBeforeAnnotation)))
			}, timeout, interval).Should(Succeed())
		})
	})
	When("TLS - A TLSe OpenStackControlplane instance is created with an internal custom issuer", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = map[string]interface{}{
				"podLevel": map[string]interface{}{
					"internal": map[string]interface{}{
						"ca": map[string]interface{}{
							"customIssuer": "myissuer",
						},
					},
				},
			}

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())
			Expect(*OSCtlplane.Spec.TLS.PodLevel.Internal.Ca.CustomIssuer).Should(Equal("myissuer"))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
		})
	})
	When("TLS - A TLSe OpenStackControlplane instance is created with an libvirt custom issuer", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = map[string]interface{}{
				"podLevel": map[string]interface{}{
					"libvirt": map[string]interface{}{
						"ca": map[string]interface{}{
							"customIssuer": "myissuer",
						},
						"cert": map[string]interface{}{
							"duration": "43800h", // can we come up with a single default duration for certs?
						},
					},
				},
			}

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(*OSCtlplane.Spec.TLS.PodLevel.Libvirt.Ca.CustomIssuer).Should(Equal("myissuer"))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
		})
	})
	When("TLS - A TLSe OpenStackControlplane instance is created with an ovn custom issuer", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = map[string]interface{}{
				"podLevel": map[string]interface{}{
					"ovn": map[string]interface{}{
						"ca": map[string]interface{}{
							"customIssuer": "myissuer",
						},
					},
				},
			}

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("should have the TLS Spec fields set/defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.Ingress.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Libvirt.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
			Expect(*OSCtlplane.Spec.TLS.PodLevel.Ovn.Ca.CustomIssuer).Should(Equal("myissuer"))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Ca.Duration.Duration.Hours()).Should(Equal(float64(87600)))
			Expect(OSCtlplane.Spec.TLS.PodLevel.Ovn.Cert.Duration.Duration.Hours()).Should(Equal(float64(43800)))
		})
	})
	//
	// Validate TLS input settings -END
	//

	When("A public TLS OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSPublicSpec()
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})

		It("should have the Spec fields defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Galera.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Rabbitmq.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Memcached.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Keystone.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())

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
		// Default route timeouts are set
		It("should have default timeout for the routes set", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "120s"))
			Expect(OSCtlplane.Spec.Cinder.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Cinder.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.cinder.openstack.org/timeout", "60s"))
			for name := range OSCtlplane.Spec.Glance.Template.GlanceAPIs {
				Expect(OSCtlplane.Spec.Glance.APIOverride[name].Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
				Expect(OSCtlplane.Spec.Glance.APIOverride[name].Route.Annotations).Should(HaveKeyWithValue("api.glance.openstack.org/timeout", "60s"))
			}
			Expect(OSCtlplane.Spec.Manila.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Manila.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.manila.openstack.org/timeout", "60s"))
		})

		It("should create selfsigned issuer and public+internal CA and issuer", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)

			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())

			// creates selfsigned issuer
			Eventually(func(g Gomega) {
				crtmgr.GetIssuer(names.SelfSignedIssuerName)
			}, timeout, interval).Should(Succeed())

			// creates public, internal and ovn root CA and issuer
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
			Eventually(func(g Gomega) {
				// ca cert
				cert := crtmgr.GetCert(names.RootCAInternalName)
				g.Expect(cert).Should(Not(BeNil()))
				g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCAInternalName.Name))
				g.Expect(cert.Spec.IsCA).Should(BeTrue())
				g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
				g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCAInternalName.Name))
				// issuer
				issuer := crtmgr.GetIssuer(names.RootCAInternalName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCAInternalName.Name))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				// ca cert
				cert := crtmgr.GetCert(names.RootCAOvnName)
				g.Expect(cert).Should(Not(BeNil()))
				g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCAOvnName.Name))
				g.Expect(cert.Spec.IsCA).Should(BeTrue())
				g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
				g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCAOvnName.Name))
				// issuer
				issuer := crtmgr.GetIssuer(names.RootCAOvnName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCAOvnName.Name))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				// ca cert
				cert := crtmgr.GetCert(names.RootCALibvirtName)
				g.Expect(cert).Should(Not(BeNil()))
				g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCALibvirtName.Name))
				g.Expect(cert.Spec.IsCA).Should(BeTrue())
				g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
				g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCALibvirtName.Name))
				// issuer
				issuer := crtmgr.GetIssuer(names.RootCALibvirtName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCALibvirtName.Name))
			}, timeout, interval).Should(Succeed())
		})

		It("should create full ca bundle", func() {
			crtmgr.GetCert(names.RootCAPublicName)
			crtmgr.GetIssuer(names.RootCAPublicName)
			crtmgr.GetCert(names.RootCAInternalName)
			crtmgr.GetIssuer(names.RootCAInternalName)
			crtmgr.GetCert(names.RootCAOvnName)
			crtmgr.GetIssuer(names.RootCAOvnName)
			crtmgr.GetCert(names.RootCALibvirtName)
			crtmgr.GetIssuer(names.RootCALibvirtName)

			Eventually(func(g Gomega) {
				th.GetSecret(names.RootCAPublicName)
				caBundle := th.GetSecret(names.CABundleName)
				g.Expect(caBundle.Data).Should(HaveLen(int(2)))
				g.Expect(caBundle.Data).Should(HaveKey(tls.CABundleKey))
				g.Expect(caBundle.Data).Should(HaveKey(tls.InternalCABundleKey))
			}, timeout, interval).Should(Succeed())
		})

		It("should create an openstackclient", func() {
			// keystone exists
			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
			// make keystoneAPI ready and create secrets usually created by keystone-controller
			keystone.SimulateKeystoneAPIReady(names.KeystoneAPIName)

			// openstackversion exists
			Eventually(func(g Gomega) {
				osversion := GetOpenStackVersion(names.OpenStackControlplaneName)
				g.Expect(osversion).Should(Not(BeNil()))

				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())

			th.CreateSecret(types.NamespacedName{Name: "openstack-config-secret", Namespace: namespace}, map[string][]byte{"secure.yaml": []byte("foo")})
			th.CreateConfigMap(types.NamespacedName{Name: "openstack-config", Namespace: namespace}, map[string]interface{}{"clouds.yaml": string("foo"), "OS_CLOUD": "default"})

			// client pod exists
			Eventually(func(g Gomega) {
				pod := &k8s_corev1.Pod{}
				err := th.K8sClient.Get(ctx, names.OpenStackClientName, pod)
				g.Expect(pod).Should(Not(BeNil()))
				g.Expect(err).ToNot(HaveOccurred())
				vols := []string{}
				for _, x := range pod.Spec.Volumes {
					vols = append(vols, x.Name)
				}
				g.Expect(vols).To(ContainElements("combined-ca-bundle", "openstack-config", "openstack-config-secret"))

				volMounts := map[string]string{}
				for _, x := range pod.Spec.Containers[0].VolumeMounts {
					volMounts[x.Name] = x.MountPath
				}
				g.Expect(volMounts).To(HaveKeyWithValue("combined-ca-bundle", "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config", "/home/cloud-admin/.config/openstack/clouds.yaml"))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config-secret", "/home/cloud-admin/.config/openstack/secure.yaml"))

				// simulate pod being in the ready state
				th.SimulatePodReady(names.OpenStackClientName)
			}, timeout, interval).Should(Succeed())

			// openstackclient exists
			Eventually(func(g Gomega) {
				osclient := GetOpenStackClient(names.OpenStackClientName)
				g.Expect(osclient).Should(Not(BeNil()))

				th.ExpectCondition(
					names.OpenStackClientName,
					ConditionGetterFunc(OpenStackClientConditionGetter),
					clientv1.OpenStackClientReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A TLSe OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			// create cert secrets for rabbitmq instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))
			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))
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
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())

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
		// Default route timeouts are set
		It("should have default timeout for the routes set", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "120s"))
		})

		It("should create selfsigned issuer and public, internal, libvirt and ovn CA and issuer", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)

			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())

			// creates selfsigned issuer
			Eventually(func(g Gomega) {
				crtmgr.GetIssuer(names.SelfSignedIssuerName)
			}, timeout, interval).Should(Succeed())

			// creates public, internal and ovn root CA and issuer
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
				g.Expect(issuer.Labels).Should(HaveKey(certmanager.RootCAIssuerPublicLabel))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				// ca cert
				cert := crtmgr.GetCert(names.RootCAInternalName)
				g.Expect(cert).Should(Not(BeNil()))
				g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCAInternalName.Name))
				g.Expect(cert.Spec.IsCA).Should(BeTrue())
				g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
				g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCAInternalName.Name))
				// issuer
				issuer := crtmgr.GetIssuer(names.RootCAInternalName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCAInternalName.Name))
				g.Expect(issuer.Labels).Should(HaveKey(certmanager.RootCAIssuerInternalLabel))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				// ca cert
				cert := crtmgr.GetCert(names.RootCAOvnName)
				g.Expect(cert).Should(Not(BeNil()))
				g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCAOvnName.Name))
				g.Expect(cert.Spec.IsCA).Should(BeTrue())
				g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
				g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCAOvnName.Name))
				// issuer
				issuer := crtmgr.GetIssuer(names.RootCAOvnName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCAOvnName.Name))
				g.Expect(issuer.Labels).Should(HaveKey(certmanager.RootCAIssuerOvnDBLabel))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				// ca cert
				cert := crtmgr.GetCert(names.RootCALibvirtName)
				g.Expect(cert).Should(Not(BeNil()))
				g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCALibvirtName.Name))
				g.Expect(cert.Spec.IsCA).Should(BeTrue())
				g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
				g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCALibvirtName.Name))
				// issuer
				issuer := crtmgr.GetIssuer(names.RootCALibvirtName)
				g.Expect(issuer).Should(Not(BeNil()))
				g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCALibvirtName.Name))
				g.Expect(issuer.Labels).Should(HaveKey(certmanager.RootCAIssuerLibvirtLabel))
			}, timeout, interval).Should(Succeed())

			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionFalse,
			)

			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				corev1.OpenStackControlPlaneCAReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})

		It("should create full ca bundle", func() {
			crtmgr.GetCert(names.RootCAPublicName)
			crtmgr.GetIssuer(names.RootCAPublicName)
			crtmgr.GetCert(names.RootCAInternalName)
			crtmgr.GetIssuer(names.RootCAInternalName)
			crtmgr.GetCert(names.RootCAOvnName)
			crtmgr.GetIssuer(names.RootCAOvnName)
			crtmgr.GetCert(names.RootCALibvirtName)
			crtmgr.GetIssuer(names.RootCALibvirtName)

			Eventually(func(g Gomega) {
				th.GetSecret(names.RootCAPublicName)
				caBundle := th.GetSecret(names.CABundleName)
				g.Expect(caBundle.Data).Should(HaveLen(int(2)))
				g.Expect(caBundle.Data).Should(HaveKey(tls.CABundleKey))
				g.Expect(caBundle.Data).Should(HaveKey(tls.InternalCABundleKey))
			}, timeout, interval).Should(Succeed())
		})

		It("should create an openstackclient", func() {
			// keystone exists
			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
			// make keystoneAPI ready and create secrets usually created by keystone-controller
			keystone.SimulateKeystoneAPIReady(names.KeystoneAPIName)

			// openstackversion exists
			Eventually(func(g Gomega) {
				osversion := GetOpenStackVersion(names.OpenStackControlplaneName)
				g.Expect(osversion).Should(Not(BeNil()))

				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())

			th.CreateSecret(types.NamespacedName{Name: "openstack-config-secret", Namespace: namespace}, map[string][]byte{"secure.yaml": []byte("foo")})
			th.CreateConfigMap(types.NamespacedName{Name: "openstack-config", Namespace: namespace}, map[string]interface{}{"clouds.yaml": string("foo"), "OS_CLOUD": "default"})

			// client pod exists
			Eventually(func(g Gomega) {
				pod := &k8s_corev1.Pod{}
				err := th.K8sClient.Get(ctx, names.OpenStackClientName, pod)
				g.Expect(pod).Should(Not(BeNil()))
				g.Expect(err).ToNot(HaveOccurred())
				vols := []string{}
				for _, x := range pod.Spec.Volumes {
					vols = append(vols, x.Name)
				}
				g.Expect(vols).To(ContainElements("combined-ca-bundle", "openstack-config", "openstack-config-secret"))

				volMounts := map[string]string{}
				for _, x := range pod.Spec.Containers[0].VolumeMounts {
					volMounts[x.Name] = x.MountPath
				}
				g.Expect(volMounts).To(HaveKeyWithValue("combined-ca-bundle", "/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config", "/home/cloud-admin/.config/openstack/clouds.yaml"))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config-secret", "/home/cloud-admin/.config/openstack/secure.yaml"))

				// simulate pod being in the ready state
				th.SimulatePodReady(names.OpenStackClientName)
			}, timeout, interval).Should(Succeed())

			// openstackclient exists
			Eventually(func(g Gomega) {
				osclient := GetOpenStackClient(names.OpenStackClientName)
				g.Expect(osclient).Should(Not(BeNil()))

				th.ExpectCondition(
					names.OpenStackClientName,
					ConditionGetterFunc(OpenStackClientConditionGetter),
					clientv1.OpenStackClientReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

		When("The TLSe OpenStackControlplane instance switches to use a custom public issuer", func() {
			BeforeEach(func() {
				// wait for default issuer
				Eventually(func(g Gomega) {
					issuer := crtmgr.GetIssuer(names.RootCAPublicName)
					g.Expect(issuer).Should(Not(BeNil()))
					g.Expect(issuer.Labels).Should(HaveKey(certmanager.RootCAIssuerPublicLabel))
				}, timeout, interval).Should(Succeed())

				// create custom issuer
				DeferCleanup(k8sClient.Delete, ctx, crtmgr.CreateIssuer(names.CustomIssuerName))
				DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.CustomIssuerName))

				// update ctlplane to use the custom isssuer
				Eventually(func(g Gomega) {
					OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
					OSCtlplane.Spec.TLS.Ingress.Ca.CustomIssuer = ptr.To(names.CustomIssuerName.Name)
					g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
				}, timeout, interval).Should(Succeed())
			})

			It("should remove the certmanager.RootCAIssuerPublicLabel label from the defaultIssuer", func() {
				Eventually(func(g Gomega) {
					issuer := crtmgr.GetIssuer(names.RootCAPublicName)
					g.Expect(issuer).Should(Not(BeNil()))
					g.Expect(issuer.Labels).Should(Not(HaveKey(certmanager.RootCAIssuerPublicLabel)))
				}, timeout, interval).Should(Succeed())
			})

			It("should add the certmanager.RootCAIssuerPublicLabel label to the customIssuer", func() {
				Eventually(func(g Gomega) {
					issuer := crtmgr.GetIssuer(names.CustomIssuerName)
					g.Expect(issuer).Should(Not(BeNil()))
					g.Expect(issuer.Labels).Should(HaveKey(certmanager.RootCAIssuerPublicLabel))
				}, timeout, interval).Should(Succeed())
			})

			When("The TLSe OpenStackControlplane instance switches again back to default public issuer", func() {
				BeforeEach(func() {
					// update ctlplane to NOT use the custom isssuer
					Eventually(func(g Gomega) {
						OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
						OSCtlplane.Spec.TLS.Ingress.Ca.CustomIssuer = nil
						g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
					}, timeout, interval).Should(Succeed())
				})

				It("should add the certmanager.RootCAIssuerPublicLabel label to the defaultIssuer", func() {
					Eventually(func(g Gomega) {
						issuer := crtmgr.GetIssuer(names.RootCAPublicName)
						g.Expect(issuer).Should(Not(BeNil()))
						g.Expect(issuer.Labels).Should(HaveKey(certmanager.RootCAIssuerPublicLabel))
					}, timeout, interval).Should(Succeed())
				})

				It("should remove the certmanager.RootCAIssuerPublicLabel label from the customIssuer", func() {
					Eventually(func(g Gomega) {
						issuer := crtmgr.GetIssuer(names.CustomIssuerName)
						g.Expect(issuer).Should(Not(BeNil()))
						g.Expect(issuer.Labels).Should(Not(HaveKey(certmanager.RootCAIssuerPublicLabel)))
					}, timeout, interval).Should(Succeed())
				})
			})
		})
	})

	When("A TLSe OpenStackControlplane instance with custom public issuer is created", func() {
		BeforeEach(func() {
			// create cert secrets for rabbitmq instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))
			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSeCustomIssuerSpec()
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})

		It("should have the Spec fields defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Galera.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Rabbitmq.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Memcached.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Keystone.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())
		})

		It("should have OpenStackControlPlaneCAReadyCondition not ready with issuer missing", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.TLS.Ingress.Ca.CustomIssuer).Should(Not(BeNil()))
			Expect(*OSCtlplane.Spec.TLS.Ingress.Ca.CustomIssuer).Should(Equal(names.CustomIssuerName.Name))

			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionFalse,
			)

			th.ExpectConditionWithDetails(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				corev1.OpenStackControlPlaneCAReadyCondition,
				k8s_corev1.ConditionFalse,
				condition.ErrorReason,
				"OpenStackControlPlane CAs issuer custom-issuer error occured Error getting issuer : Issuer.cert-manager.io \"custom-issuer\" not found",
			)
		})

		When("The proper custom issuer is provided", func() {
			BeforeEach(func() {
				// create custom issuer
				DeferCleanup(k8sClient.Delete, ctx, crtmgr.CreateIssuer(names.CustomIssuerName))
				DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.CustomIssuerName))
			})

			It("should have OpenStackControlPlaneCAReadyCondition ready when custom issuer exist", func() {
				// creates selfsigned issuer
				Eventually(func(g Gomega) {
					crtmgr.GetIssuer(names.SelfSignedIssuerName)
				}, timeout, interval).Should(Succeed())

				// does not create public CA, as custom issuer is used
				Eventually(func(g Gomega) {
					crtmgr.AssertCertDoesNotExist(names.RootCAPublicName)
				}, timeout, interval).Should(Succeed())

				// creates Internal and OVN CA
				Eventually(func(g Gomega) {
					// ca cert
					cert := crtmgr.GetCert(names.RootCAInternalName)
					g.Expect(cert).Should(Not(BeNil()))
					g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCAInternalName.Name))
					g.Expect(cert.Spec.IsCA).Should(BeTrue())
					g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
					g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCAInternalName.Name))
					// issuer
					issuer := crtmgr.GetIssuer(names.RootCAInternalName)
					g.Expect(issuer).Should(Not(BeNil()))
					g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCAInternalName.Name))
				}, timeout, interval).Should(Succeed())
				Eventually(func(g Gomega) {
					// ca cert
					cert := crtmgr.GetCert(names.RootCAOvnName)
					g.Expect(cert).Should(Not(BeNil()))
					g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCAOvnName.Name))
					g.Expect(cert.Spec.IsCA).Should(BeTrue())
					g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
					g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCAOvnName.Name))
					// issuer
					issuer := crtmgr.GetIssuer(names.RootCAOvnName)
					g.Expect(issuer).Should(Not(BeNil()))
					g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCAOvnName.Name))
				}, timeout, interval).Should(Succeed())
				Eventually(func(g Gomega) {
					// ca cert
					cert := crtmgr.GetCert(names.RootCALibvirtName)
					g.Expect(cert).Should(Not(BeNil()))
					g.Expect(cert.Spec.CommonName).Should(Equal(names.RootCALibvirtName.Name))
					g.Expect(cert.Spec.IsCA).Should(BeTrue())
					g.Expect(cert.Spec.IssuerRef.Name).Should(Equal(names.SelfSignedIssuerName.Name))
					g.Expect(cert.Spec.SecretName).Should(Equal(names.RootCALibvirtName.Name))
					// issuer
					issuer := crtmgr.GetIssuer(names.RootCALibvirtName)
					g.Expect(issuer).Should(Not(BeNil()))
					g.Expect(issuer.Spec.CA.SecretName).Should(Equal(names.RootCALibvirtName.Name))
				}, timeout, interval).Should(Succeed())

				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					condition.ReadyCondition,
					k8s_corev1.ConditionFalse,
				)

				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneCAReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			})
		})
	})

	When("A public TLS OpenStackControlplane instance with custom public certificate is created", func() {
		BeforeEach(func() {
			// create cert secrets for rabbitmq instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))
			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))

			DeferCleanup(k8sClient.Delete, ctx,
				th.CreateSecret(types.NamespacedName{Name: "openstack-config-secret", Namespace: namespace}, map[string][]byte{"secure.yaml": []byte("foo")}))
			DeferCleanup(k8sClient.Delete, ctx,
				th.CreateConfigMap(types.NamespacedName{Name: "openstack-config", Namespace: namespace}, map[string]interface{}{"clouds.yaml": string("foo"), "OS_CLOUD": "default"}))
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSPublicSpec()
			spec["keystone"] = map[string]interface{}{
				"enabled": true,
				"apiOverride": map[string]interface{}{
					"tls": map[string]interface{}{
						"secretName": names.CustomServiceCertSecretName.Name,
					},
				},
				"template": map[string]interface{}{
					"databaseInstance": names.KeystoneAPIName.Name,
					"secret":           "osp-secret",
				},
			}
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})

		It("should have the Spec fields defaulted", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Galera.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Rabbitmq.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Memcached.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.Keystone.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())
		})

		It("should have galera, memcached, rabbit and keystone deployed", func() {
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

			// keystone exists
			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
		})

		When("The keystone k8s service is created", func() {
			BeforeEach(func() {
				keystonePublicSvcName := types.NamespacedName{Name: "keystone-public", Namespace: namespace}
				keystoneInternalSvcName := types.NamespacedName{Name: "keystone-internal", Namespace: namespace}

				th.CreateService(
					keystonePublicSvcName,
					map[string]string{
						"osctlplane-service": "keystone",
						"osctlplane":         "",
					},
					k8s_corev1.ServiceSpec{
						Ports: []k8s_corev1.ServicePort{
							{
								Name:     "keystone-public",
								Port:     int32(5000),
								Protocol: k8s_corev1.ProtocolTCP,
							},
						},
					})
				keystoneSvc := th.GetService(keystonePublicSvcName)
				if keystoneSvc.Annotations == nil {
					keystoneSvc.Annotations = map[string]string{}
				}
				keystoneSvc.Annotations[service.AnnotationIngressCreateKey] = "true"
				keystoneSvc.Annotations[service.AnnotationEndpointKey] = "public"
				Expect(th.K8sClient.Status().Update(th.Ctx, keystoneSvc)).To(Succeed())

				th.CreateService(
					keystoneInternalSvcName,
					map[string]string{
						"osctlplane-service": "keystone",
						"osctlplane":         "",
					},
					k8s_corev1.ServiceSpec{
						Ports: []k8s_corev1.ServicePort{
							{
								Name:     "keystone-internal",
								Port:     int32(5000),
								Protocol: k8s_corev1.ProtocolTCP,
							},
						},
					})
				keystoneSvc = th.GetService(keystoneInternalSvcName)
				if keystoneSvc.Annotations == nil {
					keystoneSvc.Annotations = map[string]string{}
				}
				keystoneSvc.Annotations[service.AnnotationIngressCreateKey] = "false"
				keystoneSvc.Annotations[service.AnnotationEndpointKey] = "internal"
				Expect(th.K8sClient.Status().Update(th.Ctx, keystoneSvc)).To(Succeed())

				// create custom issuer
				DeferCleanup(k8sClient.Delete, ctx, crtmgr.CreateIssuer(names.CustomIssuerName))
				DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.CustomIssuerName))
			})

			It("should have OpenStackControlPlaneCustomTLSReadyCondition not ready with secret missing", func() {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				Expect(OSCtlplane.Spec.Keystone.APIOverride.TLS.SecretName).Should(Equal(names.CustomServiceCertSecretName.Name))

				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					condition.ReadyCondition,
					k8s_corev1.ConditionFalse,
				)

				th.ExpectConditionWithDetails(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneCustomTLSReadyCondition,
					k8s_corev1.ConditionFalse,
					condition.ErrorReason,
					fmt.Sprintf("OpenStackControlPlane custom TLS cert secret custom-service-cert error occured Secret %s/custom-service-cert not found", namespace),
				)
			})

			It("should have created keystone with route using custom cert secret", func() {
				DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.CustomServiceCertSecretName))

				// keystone exists
				Eventually(func(g Gomega) {
					keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
					g.Expect(keystoneAPI).Should(Not(BeNil()))
				}, timeout, interval).Should(Succeed())

				keystone.SimulateKeystoneAPIReady(names.KeystoneAPIName)

				// expect the ready status to propagate to control plane object
				Eventually(func(g Gomega) {
					th.ExpectCondition(
						names.OpenStackControlplaneName,
						ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
						corev1.OpenStackControlPlaneKeystoneAPIReadyCondition,
						k8s_corev1.ConditionTrue,
					)
				}, timeout, interval).Should(Succeed())

				Eventually(func(g Gomega) {
					keystoneRouteName := types.NamespacedName{Name: "keystone-public", Namespace: namespace}
					keystoneRoute := &routev1.Route{}

					g.Expect(th.K8sClient.Get(th.Ctx, keystoneRouteName, keystoneRoute)).Should(Succeed())
					g.Expect(keystoneRoute.Spec.TLS.Certificate).Should(Not(BeEmpty()))
					g.Expect(keystoneRoute.Spec.TLS.Key).Should(Not(BeEmpty()))
					g.Expect(keystoneRoute.Spec.TLS.CACertificate).Should(Not(BeEmpty()))
				}, timeout, interval).Should(Succeed())

			})

		})
	})

	When("A Manila OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSPublicSpec()
			spec["manila"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"manilaAPI": map[string]interface{}{
						"replicas": 1,
					},
					"manilaScheduler": map[string]interface{}{
						"replicas": 1,
					},
					"manilaShares": map[string]interface{}{
						"share1": map[string]interface{}{
							"replicas": 1,
						},
					},
				},
			}
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})

		It("should have Manila enabled", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Manila.Enabled).Should(BeTrue())

			// manila exists
			manila := &manilav1.Manila{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, names.ManilaName, manila)).Should(Succeed())
				g.Expect(manila).ShouldNot(BeNil())
			}, timeout, interval).Should(Succeed())

			// FIXME add helpers to manila-operator to simulate ready state
			Eventually(func(g Gomega) {
				manila := &manilav1.Manila{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.ManilaName, manila)).Should(Succeed())
				manila.Status.ObservedGeneration = manila.Generation
				manila.Status.Conditions.MarkTrue(manilav1.ManilaAPIReadyCondition, "Ready")
				manila.Status.Conditions.MarkTrue(manilav1.ManilaSchedulerReadyCondition, "Ready")
				manila.Status.Conditions.MarkTrue(manilav1.ManilaShareReadyCondition, "Ready")
				g.Expect(th.K8sClient.Status().Update(th.Ctx, manila)).To(Succeed())

				th.Logger.Info("Simulated Manila ready", "on", names.ManilaName)
			}, timeout, interval).Should(Succeed())

			// expect the ready status to propagate to control plane object
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneManilaReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("should have Manila Shares configured", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Manila.Enabled).Should(BeTrue())

			// manila exists
			manila := &manilav1.Manila{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, names.ManilaName, manila)).Should(Succeed())
				g.Expect(manila).ShouldNot(BeNil())
			}, timeout, interval).Should(Succeed())

			// FIXME add helpers to manila-operator to simulate ready state
			Eventually(func(g Gomega) {
				manila := &manilav1.Manila{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.ManilaName, manila)).Should(Succeed())
				manila.Status.ObservedGeneration = manila.Generation
				manila.Status.Conditions.MarkTrue(manilav1.ManilaAPIReadyCondition, "Ready")
				manila.Status.Conditions.MarkTrue(manilav1.ManilaSchedulerReadyCondition, "Ready")
				manila.Status.Conditions.MarkTrue(manilav1.ManilaShareReadyCondition, "Ready")
				g.Expect(th.K8sClient.Status().Update(th.Ctx, manila)).To(Succeed())

				g.Expect(manila.Spec.ManilaShares).Should(HaveLen(1))
				g.Expect(manila.Spec.ManilaShares["share1"]).ShouldNot(BeNil())
				replicas := int32(1)
				g.Expect(manila.Spec.ManilaShares["share1"].Replicas).Should(Equal(&replicas))

				th.Logger.Info("Simulated Manila ready", "on", names.ManilaName)
			}, timeout, interval).Should(Succeed())

			// expect the ready status to propagate to control plane object
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneManilaReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

	})

	When("A Cinder OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSPublicSpec()
			spec["cinder"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"cinderAPI": map[string]interface{}{
						"replicas": 1,
					},
					"cinderBackup": map[string]interface{}{
						"replicas": 1,
					},
					"cinderScheduler": map[string]interface{}{
						"replicas": 1,
					},
					"cinderVolumes": map[string]interface{}{
						"volume1": map[string]interface{}{
							"replicas": 1,
						},
					},
				},
			}
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})

		It("should have Cinder enabled", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Cinder.Enabled).Should(BeTrue())

			// cinder exists
			cinder := &cinderv1.Cinder{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, names.CinderName, cinder)).Should(Succeed())
				g.Expect(cinder).ShouldNot(BeNil())
			}, timeout, interval).Should(Succeed())

			// FIXME add helpers to cinder-operator to simulate ready state
			Eventually(func(g Gomega) {
				cinder := &cinderv1.Cinder{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.CinderName, cinder)).Should(Succeed())
				cinder.Status.ObservedGeneration = cinder.Generation
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderAPIReadyCondition, "Ready")
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderBackupReadyCondition, "Ready")
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderSchedulerReadyCondition, "Ready")
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderVolumeReadyCondition, "Ready")
				g.Expect(th.K8sClient.Status().Update(th.Ctx, cinder)).To(Succeed())

				th.Logger.Info("Simulated Cinder ready", "on", names.CinderName)
			}, timeout, interval).Should(Succeed())

			// expect the ready status to propagate to control plane object
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneCinderReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("should have Cinder Volume configured", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Cinder.Enabled).Should(BeTrue())

			cinder := &cinderv1.Cinder{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, names.CinderName, cinder)).Should(Succeed())
				g.Expect(cinder).ShouldNot(BeNil())
			}, timeout, interval).Should(Succeed())

			// FIXME add helpers to cinder-operator to simulate ready state
			Eventually(func(g Gomega) {
				cinder := &cinderv1.Cinder{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.CinderName, cinder)).Should(Succeed())
				cinder.Status.ObservedGeneration = cinder.Generation
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderAPIReadyCondition, "Ready")
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderBackupReadyCondition, "Ready")
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderSchedulerReadyCondition, "Ready")
				cinder.Status.Conditions.MarkTrue(cinderv1.CinderVolumeReadyCondition, "Ready")
				g.Expect(th.K8sClient.Status().Update(th.Ctx, cinder)).To(Succeed())

				g.Expect(cinder.Spec.CinderVolumes).Should(HaveLen(1))
				g.Expect(cinder.Spec.CinderVolumes["volume1"]).ShouldNot(BeNil())
				replicas := int32(1)
				g.Expect(cinder.Spec.CinderVolumes["volume1"].Replicas).Should(Equal(&replicas))

				th.Logger.Info("Simulated Cinder ready", "on", names.CinderName)
			}, timeout, interval).Should(Succeed())

			// expect the ready status to propagate to control plane object
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneCinderReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

	})

	When("A OVN OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSPublicSpec()
			spec["ovn"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"ovnDBCluster": map[string]interface{}{
						"ovndbcluster-nb": map[string]interface{}{
							"dbType": "NB",
						},
						"ovndbcluster-sb": map[string]interface{}{
							"dbType": "SB",
						},
					},
				},
			}
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})

		It("should have OVN enabled", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Ovn.Enabled).Should(BeTrue())

			// ovn services exist
			Eventually(func(g Gomega) {
				ovnNorthd := ovn.GetOVNNorthd(names.OVNNorthdName)
				g.Expect(ovnNorthd).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				ovnController := ovn.GetOVNController(names.OVNControllerName)
				g.Expect(ovnController).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				ovnDbServerNB := ovn.GetOVNDBCluster(names.OVNDbServerNBName)
				g.Expect(ovnDbServerNB).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				ovnDbServerSB := ovn.GetOVNDBCluster(names.OVNDbServerSBName)
				g.Expect(ovnDbServerSB).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// set ready states for each ovn component
			ovn.SimulateOVNNorthdReady(names.OVNNorthdName)
			ovn.SimulateOVNDBClusterReady(names.OVNDbServerNBName)
			ovn.SimulateOVNDBClusterReady(names.OVNDbServerSBName)
			ovn.SimulateOVNControllerReady(names.OVNControllerName)

			// expect the ready status to propagate to control plane object
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneOVNReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("should remove OVN resources on disable", func() {
			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				OSCtlplane.Spec.Ovn.Enabled = false
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				g.Expect(OSCtlplane.Spec.Ovn.Enabled).Should(BeFalse())
			}, timeout, interval).Should(Succeed())

			// ovn services don't exist
			Eventually(func(g Gomega) {
				instance := &ovnv1.OVNNorthd{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.OVNNorthdName, instance)).Should(Not(Succeed()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				instance := &ovnv1.OVNDBCluster{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.OVNDbServerNBName, instance)).Should(Not(Succeed()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				instance := &ovnv1.OVNDBCluster{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.OVNDbServerSBName, instance)).Should(Not(Succeed()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				instance := &ovnv1.OVNController{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.OVNControllerName, instance)).Should(Not(Succeed()))
			}, timeout, interval).Should(Succeed())

			// expect the ovn ready condition removed to not affect deployment success
			Eventually(func(g Gomega) {
				conditions := OpenStackControlPlaneConditionGetter(names.OpenStackControlplaneName)
				g.Expect(conditions.Has(corev1.OpenStackControlPlaneOVNReadyCondition)).To(BeFalse())
			}, timeout, interval).Should(Succeed())
		})
	})

	When("OpenStackControlplane instance is deleted", func() {
		BeforeEach(func() {
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, GetDefaultOpenStackControlPlaneSpec()),
			)
		})

		It("deletes the OpenStackVersion resource", func() {

			// openstackversion exists
			Eventually(func(g Gomega) {
				osversion := GetOpenStackVersion(names.OpenStackControlplaneName)
				g.Expect(osversion).Should(Not(BeNil()))
				g.Expect(osversion.OwnerReferences).Should(HaveLen(1))

				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				ctlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				g.Expect(ctlplane.Finalizers).Should(HaveLen(1))
				th.DeleteInstance(ctlplane)
			}, timeout, interval).Should(Succeed())

			// deleting the OpenStackControlPlane should remove the OpenStackVersion finalizer we are good
			Eventually(func(g Gomega) {
				osversion := GetOpenStackVersion(names.OpenStackControlplaneName)
				g.Expect(osversion.Finalizers).Should(BeEmpty())
			}, timeout, interval).Should(Succeed())

		})
	})

})

var _ = Describe("OpenStackOperator Webhook", func() {

	It("Adds default label via defaulting webhook", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["tls"] = GetTLSPublicSpec()
		DeferCleanup(
			th.DeleteInstance,
			CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
		)

		OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
		Expect(OSCtlplane.Labels).Should(Not(BeNil()))
		Expect(OSCtlplane.Labels).Should(HaveKeyWithValue("core.openstack.org/openstackcontrolplane", ""))
	})

	It("Does not override default label via defaulting webhook when provided", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["tls"] = GetTLSPublicSpec()
		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "openstack",
				"namespace": namespace,
				"labels": map[string]interface{}{
					"core.openstack.org/openstackcontrolplane": "foo",
				},
			},
			"spec": spec,
		}
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })

		Expect(err).ShouldNot(HaveOccurred())

		OSCtlplane := GetOpenStackControlPlane(types.NamespacedName{Name: "openstack", Namespace: namespace})
		Expect(OSCtlplane.Labels).Should(Not(BeNil()))
		Expect(OSCtlplane.Labels).Should(HaveKeyWithValue("core.openstack.org/openstackcontrolplane", "foo"))
	})

	It("calls placement validation webhook", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["tls"] = GetTLSPublicSpec()
		spec["placement"] = map[string]interface{}{
			"template": map[string]interface{}{
				"defaultConfigOverwrite": map[string]interface{}{
					"api-paste.ini": "not supported",
				},
			},
		}
		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "openstack",
				"namespace": namespace,
			},
			"spec": spec,
		}
		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			ctx, k8sClient, unstructuredObj, func() error { return nil })

		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"invalid: spec.placement.template.defaultConfigOverwrite: " +
					"Invalid value: \"api-paste.ini\": " +
					"Only the following keys are valid: policy.yaml",
			),
		)
	})
})
