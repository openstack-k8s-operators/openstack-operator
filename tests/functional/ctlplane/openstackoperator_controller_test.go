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
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"

	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	routev1 "github.com/openshift/api/route/v1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"

	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	watcherv1 "github.com/openstack-k8s-operators/watcher-operator/api/v1beta1"
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

	var (
		galeraService = Entry("the galera service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := mariadb.GetGalera(names.DBName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		keystoneService = Entry("the keystone service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := keystone.GetKeystoneAPI(names.KeystoneAPIName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		rabbitService = Entry("the rabbitmq service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetRabbitMQCluster(names.RabbitMQName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		memcachedService = Entry("the memcached service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := infra.GetMemcached(names.MemcachedName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		glanceService = Entry("the glance service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetGlance(names.GlanceName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		cinderService = Entry("the cinder service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetCinder(names.CinderName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		manilaService = Entry("the manila service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetManila(names.ManilaName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		neutronService = Entry("the neutron service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetNeutron(names.NeutronName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		horizonService = Entry("the horizon service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetHorizon(names.HorizonName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		heatService = Entry("the heat service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetHeat(names.HeatName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		telemetryService = Entry("the telemetry service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetTelemetry(names.TelemetryName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
		watcherService = Entry("the watcher service", func() (
			client.Object, *topologyv1.TopoRef) {
			svc := GetWatcher(names.WatcherName)
			tp := svc.Spec.TopologyRef
			return svc, tp
		})
	)
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

			// rabbitmq exists
			Eventually(func(g Gomega) {
				rabbitmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(rabbitmq).Should(Not(BeNil()))
				g.Expect(rabbitmq.Spec.Replicas).Should(Equal(ptr.To[int32](1)))
			}, timeout, interval).Should(Succeed())

			// keystone exists
			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
		})
		// Default route timeouts are set
		It("should have default timeout for the routes set", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "120s"))
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.neutron.openstack.org/timeout", "120s"))
			Expect(OSCtlplane.Spec.Cinder.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Cinder.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Cinder.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.cinder.openstack.org/timeout", "60s"))
			Expect(OSCtlplane.Spec.Glance.Template).Should(Not(BeNil()))
			for name := range OSCtlplane.Spec.Glance.Template.GlanceAPIs {
				Expect(OSCtlplane.Spec.Glance.APIOverride[name].Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
				Expect(OSCtlplane.Spec.Glance.APIOverride[name].Route.Annotations).Should(HaveKeyWithValue("api.glance.openstack.org/timeout", "60s"))
			}
			Expect(OSCtlplane.Spec.Heat.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Heat.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "600s"))
			Expect(OSCtlplane.Spec.Heat.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.heat.openstack.org/timeout", "600s"))
			Expect(OSCtlplane.Spec.Heat.CnfAPIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Heat.CnfAPIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "600s"))
			Expect(OSCtlplane.Spec.Heat.CnfAPIOverride.Route.Annotations).Should(HaveKeyWithValue("api.heat.openstack.org/timeout", "600s"))
			Expect(OSCtlplane.Spec.Manila.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Manila.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Manila.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.manila.openstack.org/timeout", "60s"))
			//TODO (froyo) Enable these tests when Octavia would be enabled on FTs
			//Expect(OSCtlplane.Spec.Octavia.APIOverride.Route).Should(Not(BeNil()))
			//Expect(OSCtlplane.Spec.Octavia.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "120s"))
			//Expect(OSCtlplane.Spec.Octavia.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.octavia.openstack.org/timeout", "120s"))
			Expect(OSCtlplane.Spec.Telemetry.AodhAPIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Telemetry.AodhAPIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Telemetry.AodhAPIOverride.Route.Annotations).Should(HaveKeyWithValue("api.aodh.openstack.org/timeout", "60s"))
			//TODO: Enable these tests when Barbican would be enabled on FTs
			// Expect(OSCtlplane.Spec.Barbican.APIOverride.Route).Should(Not(BeNil()))
			// Expect(OSCtlplane.Spec.Barbican.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "90s"))
			// Expect(OSCtlplane.Spec.Barbican.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.barbican.openstack.org/timeout", "90s"))
			Expect(OSCtlplane.Spec.Keystone.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Keystone.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Keystone.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.keystone.openstack.org/timeout", "60s"))
			Expect(OSCtlplane.Spec.Ironic.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Ironic.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Ironic.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.ironic.openstack.org/timeout", "60s"))
			Expect(OSCtlplane.Spec.Ironic.InspectorOverride.Route.Annotations).Should(HaveKeyWithValue("inspector.ironic.openstack.org/timeout", "60s"))
			//TODO: Enable these tests when Nova and Placement would be enabled on FTs
			//Expect(OSCtlplane.Spec.Nova.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			//Expect(OSCtlplane.Spec.Nova.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.nova.openstack.org/timeout", "60s"))
			//Expect(OSCtlplane.Spec.Placement.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			//Expect(OSCtlplane.Spec.Placement.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.placement.openstack.org/timeout", "60s"))
		})

		It("should create selfsigned issuer and public+internal CA and issuer", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)

			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeFalse())

			// creates selfsigned issuer
			Eventually(func(_ Gomega) {
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
					names.OpenStackVersionName,
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

				volMounts := map[string][]string{}
				for _, x := range pod.Spec.Containers[0].VolumeMounts {
					volMounts[x.Name] = append(volMounts[x.Name], x.MountPath)
				}
				g.Expect(volMounts).To(HaveKeyWithValue("combined-ca-bundle", []string{"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"}))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config", []string{"/home/cloud-admin/.config/openstack/clouds.yaml"}))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config-secret", []string{"/home/cloud-admin/.config/openstack/secure.yaml", "/home/cloud-admin/cloudrc"}))

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
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNMetricsCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))
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

			// rabbitmq exists
			Eventually(func(g Gomega) {
				rabbitmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(rabbitmq).Should(Not(BeNil()))
				g.Expect(rabbitmq.Spec.Replicas).Should(Equal(ptr.To[int32](1)))
			}, timeout, interval).Should(Succeed())

			// keystone exists
			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
		})
		// Default route timeouts are set
		It("should have default timeout for the routes set", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "120s"))
			Expect(OSCtlplane.Spec.Neutron.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.neutron.openstack.org/timeout", "120s"))
			Expect(OSCtlplane.Spec.Heat.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Heat.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "600s"))
			Expect(OSCtlplane.Spec.Heat.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.heat.openstack.org/timeout", "600s"))
			Expect(OSCtlplane.Spec.Heat.CnfAPIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Heat.CnfAPIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "600s"))
			Expect(OSCtlplane.Spec.Heat.CnfAPIOverride.Route.Annotations).Should(HaveKeyWithValue("api.heat.openstack.org/timeout", "600s"))
			Expect(OSCtlplane.Spec.Telemetry.AodhAPIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Telemetry.AodhAPIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Telemetry.AodhAPIOverride.Route.Annotations).Should(HaveKeyWithValue("api.aodh.openstack.org/timeout", "60s"))
			//TODO: Enable these tests when Barbican would be enabled on FTs
			// Expect(OSCtlplane.Spec.Barbican.APIOverride.Route).Should(Not(BeNil()))
			// Expect(OSCtlplane.Spec.Barbican.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "90s"))
			// Expect(OSCtlplane.Spec.Barbican.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.barbican.openstack.org/timeout", "90s"))
			Expect(OSCtlplane.Spec.Keystone.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Keystone.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Keystone.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.keystone.openstack.org/timeout", "60s"))
			//TODO: Enable these tests when Nova and Placement would be enabled on FTs
			//Expect(OSCtlplane.Spec.Nova.APIOverride.Route).Should(Not(BeNil()))
			//Expect(OSCtlplane.Spec.Nova.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			//Expect(OSCtlplane.Spec.Nova.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.nova.openstack.org/timeout", "60s"))
			//Expect(OSCtlplane.Spec.Placement.APIOverride.Route).Should(Not(BeNil()))
			//Expect(OSCtlplane.Spec.Placement.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			//Expect(OSCtlplane.Spec.Placement.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.placement.openstack.org/timeout", "60s"))
		})

		It("should create selfsigned issuer and public, internal, libvirt and ovn CA and issuer", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)

			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())

			// creates selfsigned issuer
			Eventually(func(_ Gomega) {
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
					names.OpenStackVersionName,
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

				volMounts := map[string][]string{}
				for _, x := range pod.Spec.Containers[0].VolumeMounts {
					volMounts[x.Name] = append(volMounts[x.Name], x.MountPath)
				}
				g.Expect(volMounts).To(HaveKeyWithValue("combined-ca-bundle", []string{"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem"}))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config", []string{"/home/cloud-admin/.config/openstack/clouds.yaml"}))
				g.Expect(volMounts).To(HaveKeyWithValue("openstack-config-secret", []string{"/home/cloud-admin/.config/openstack/secure.yaml", "/home/cloud-admin/cloudrc"}))

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
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))
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
				"OpenStackControlPlane CAs issuer custom-issuer error occured error getting issuer : Issuer.cert-manager.io \"custom-issuer\" not found",
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
				Eventually(func(_ Gomega) {
					crtmgr.GetIssuer(names.SelfSignedIssuerName)
				}, timeout, interval).Should(Succeed())

				// does not create public CA, as custom issuer is used
				Eventually(func(_ Gomega) {
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
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))

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

			// rabbitmq exists
			Eventually(func(g Gomega) {
				rabbitmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(rabbitmq).Should(Not(BeNil()))
				g.Expect(rabbitmq.Spec.Replicas).Should(Equal(ptr.To[int32](1)))
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
					fmt.Sprintf(corev1.OpenStackControlPlaneCustomTLSReadyWaitingMessage, names.CustomServiceCertSecretName.Name),
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
				Eventually(func(_ Gomega) {
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
					g.Expect(keystoneRoute.Spec.TLS).Should(Not(BeNil()))
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
			Eventually(func(_ Gomega) {
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
			Eventually(func(_ Gomega) {
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
			Eventually(func(_ Gomega) {
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
			Eventually(func(_ Gomega) {
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
					"ovnController": map[string]interface{}{
						"nicMappings": map[string]interface{}{
							"datacentre": "ospbr",
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
			Eventually(func(_ Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneOVNReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("should remove ovn-controller if nicMappings are removed", func() {
			// Update spec
			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				OSCtlplane.Spec.Ovn.Template.OVNController.NicMappings = nil
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			// ovn services exist
			Eventually(func(g Gomega) {
				ovnNorthd := ovn.GetOVNNorthd(names.OVNNorthdName)
				g.Expect(ovnNorthd).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// If nicMappings are not configured, ovnController shouldn't spawn
			Eventually(func(g Gomega) {
				instance := &ovnv1.OVNController{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.OVNControllerName, instance)).Should(Not(Succeed()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				ovnDbServerNB := ovn.GetOVNDBCluster(names.OVNDbServerNBName)
				g.Expect(ovnDbServerNB).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				ovnDbServerSB := ovn.GetOVNDBCluster(names.OVNDbServerSBName)
				g.Expect(ovnDbServerSB).Should(Not(BeNil()))
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

	When("A OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			// NOTE(bogdando): DBs certs need to be created here as well, but those are already existing somehow
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))

			DeferCleanup(k8sClient.Delete, ctx,
				th.CreateSecret(types.NamespacedName{Name: "openstack-config-secret", Namespace: namespace}, map[string][]byte{"secure.yaml": []byte("foo")}))
			DeferCleanup(k8sClient.Delete, ctx,
				th.CreateConfigMap(types.NamespacedName{Name: "openstack-config", Namespace: namespace}, map[string]interface{}{"clouds.yaml": string("foo"), "OS_CLOUD": "default"}))

			spec := GetDefaultOpenStackControlPlaneSpec()
			// enable dependencies
			spec["nova"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"apiTimeout": 60,
					"cellTemplates": map[string]interface{}{
						"cell0": map[string]interface{}{},
					},
				},
			}
			spec["galera"] = map[string]interface{}{
				"enabled": true,
			}
			spec["memcached"] = map[string]interface{}{
				"enabled": true,
				"templates": map[string]interface{}{
					"memcached": map[string]interface{}{
						"replicas": 1,
					},
				},
			}
			spec["rabbitmq"] = map[string]interface{}{
				"enabled": true,
				"templates": map[string]interface{}{
					"rabbitmq": map[string]interface{}{
						"replicas": 1,
					},
				},
			}
			spec["keystone"] = map[string]interface{}{
				"enabled": true,
			}
			spec["glance"] = map[string]interface{}{
				"enabled": true,
			}
			spec["neutron"] = map[string]interface{}{
				"enabled": true,
			}
			spec["placement"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"apiTimeout": 60,
				},
			}
			// turn off unrelated to this test case services
			spec["horizon"] = map[string]interface{}{
				"enabled": false,
			}
			spec["cinder"] = map[string]interface{}{
				"enabled": false,
			}
			spec["swift"] = map[string]interface{}{
				"enabled": false,
			}
			spec["redis"] = map[string]interface{}{
				"enabled": false,
			}
			spec["ironic"] = map[string]interface{}{
				"enabled": false,
			}
			spec["designate"] = map[string]interface{}{
				"enabled": false,
			}
			spec["barbican"] = map[string]interface{}{
				"enabled": false,
			}
			spec["manila"] = map[string]interface{}{
				"enabled": false,
			}
			spec["heat"] = map[string]interface{}{
				"enabled": false,
			}
			spec["telemetry"] = map[string]interface{}{
				"enabled": false,
			}

			Eventually(func(g Gomega) {
				g.Expect(CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec)).Should(Not(BeNil()))
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
				SimulateControlplaneReady()
			}, timeout, interval).Should(Succeed())

			DeferCleanup(
				th.DeleteInstance,
				GetOpenStackControlPlane(names.OpenStackControlplaneName),
			)

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				OSCtlplane.Status.ObservedGeneration = OSCtlplane.Generation
				OSCtlplane.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneMemcachedReadyCondition, "Ready")
				OSCtlplane.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneRabbitMQReadyCondition, "Ready")
				OSCtlplane.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneNeutronReadyCondition, "Ready")
				OSCtlplane.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneGlanceReadyCondition, "Ready")
				OSCtlplane.Status.Conditions.MarkTrue(corev1.OpenStackControlPlanePlacementAPIReadyCondition, "Ready")
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
				th.Logger.Info("Simulated nova dependencies ready", "on", names.OpenStackControlplaneName)
			}, timeout, interval).Should(Succeed())

			// nova to become ready
			Eventually(func(g Gomega) {
				conditions := OpenStackControlPlaneConditionGetter(names.OpenStackControlplaneName)
				g.Expect(conditions.Has(corev1.OpenStackControlPlaneNovaReadyCondition)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})

		It("should have configured nova", func() {
			nova := &novav1.Nova{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, names.NovaName, nova)).Should(Succeed())
				g.Expect(nova).ShouldNot(BeNil())
			}, timeout, interval).Should(Succeed())
		})

		It("should have configured nova from the service template", func() {
			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				OSCtlplane.Spec.Nova.Template.APIDatabaseInstance = "custom-db"
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			nova := &novav1.Nova{}
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, names.NovaName, nova)).Should(Succeed())
				g.Expect(nova).ShouldNot(BeNil())
				g.Expect(nova.Spec.APIDatabaseInstance).Should(Equal("custom-db"))

			}, timeout, interval).Should(Succeed())
		})
	})

	When("A watcher OpenStackControlplane instance is created with telemetry and default values", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["watcher"] = map[string]interface{}{
				"enabled": true,
			}
			spec["telemetry"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"ceilometer": map[string]interface{}{
						"enabled": true,
					},
					"metricStorage": map[string]interface{}{
						"enabled": true,
					},
				},
			}

			// create cert secrets for rabbitmq instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))
			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))

			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.WatcherCertPublicRouteName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.WatcherCertPublicSvcName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.WatcherCertInternalName))

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)

		})

		It("should have watcher enabled and default values", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Watcher.Enabled).Should(BeTrue())

			Expect(OSCtlplane.Spec.TLS.Ingress.Enabled).Should(BeTrue())
			Expect(OSCtlplane.Spec.TLS.PodLevel.Enabled).Should(BeTrue())

			Expect(OSCtlplane.Spec.Watcher.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Watcher.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "60s"))
			Expect(OSCtlplane.Spec.Watcher.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.watcher.openstack.org/timeout", "60s"))

			watcher := GetWatcher(names.WatcherName)
			// watcher services exist
			Eventually(func(g Gomega) {
				watcher := GetWatcher(names.WatcherName)
				g.Expect(watcher).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// default databaseInstance is openstack
			Expect(watcher.Spec.DatabaseInstance).Should(Equal(ptr.To("openstack")))
			Expect(watcher.Spec.DatabaseAccount).Should(Equal(ptr.To("watcher")))
			// default Watche container images are set
			Expect(watcher.Spec.APIContainerImageURL).Should(Not(BeNil()))
			Expect(watcher.Spec.ApplierContainerImageURL).Should(Not(BeNil()))
			Expect(watcher.Spec.DecisionEngineContainerImageURL).Should(Not(BeNil()))
			Expect(watcher.Spec.DecisionEngineContainerImageURL).Should(Equal("quay.io/podified-master-centos9/openstack-watcher-decision-engine:current-podified"))

			Expect(watcher.Spec.APIServiceTemplate.TLS.Ca.CaBundleSecretName).Should(Equal("combined-ca-bundle"))

		})

		It("should create watcher route and populate TLS secrets", func() {
			watcherPublicSvcName := types.NamespacedName{Name: "watcher-public", Namespace: namespace}
			watcherInternalSvcName := types.NamespacedName{Name: "watcher-internal", Namespace: namespace}

			th.CreateService(
				watcherPublicSvcName,
				map[string]string{
					"osctlplane-service": "watcher",
					"osctlplane":         "",
				},
				k8s_corev1.ServiceSpec{
					Ports: []k8s_corev1.ServicePort{
						{
							Name:     "watcher-public",
							Port:     int32(9322),
							Protocol: k8s_corev1.ProtocolTCP,
						},
					},
				})
			watcherSvc := th.GetService(watcherPublicSvcName)
			if watcherSvc.Annotations == nil {
				watcherSvc.Annotations = map[string]string{}
			}
			watcherSvc.Annotations[service.AnnotationIngressCreateKey] = "true"
			watcherSvc.Annotations[service.AnnotationEndpointKey] = "public"
			Expect(th.K8sClient.Status().Update(th.Ctx, watcherSvc)).To(Succeed())
			th.CreateService(
				watcherInternalSvcName,
				map[string]string{
					"osctlplane-service": "watcher",
					"osctlplane":         "",
				},
				k8s_corev1.ServiceSpec{
					Ports: []k8s_corev1.ServicePort{
						{
							Name:     "watcher-internal",
							Port:     int32(9322),
							Protocol: k8s_corev1.ProtocolTCP,
						},
					},
				})
			watcherIntSvc := th.GetService(watcherInternalSvcName)
			if watcherIntSvc.Annotations == nil {
				watcherIntSvc.Annotations = map[string]string{}
			}
			watcherIntSvc.Annotations[service.AnnotationIngressCreateKey] = "false"
			watcherIntSvc.Annotations[service.AnnotationEndpointKey] = "internal"
			Expect(th.K8sClient.Status().Update(th.Ctx, watcherIntSvc)).To(Succeed())

			Eventually(func(g Gomega) {
				watcherRouteName := types.NamespacedName{Name: "watcher-public", Namespace: namespace}
				watcherRoute := &routev1.Route{}

				g.Expect(th.K8sClient.Get(th.Ctx, watcherRouteName, watcherRoute)).Should(Succeed())
				g.Expect(watcherRoute.Spec.TLS).Should(Not(BeNil()))
				g.Expect(watcherRoute.Spec.TLS.Certificate).Should(Not(BeEmpty()))
				g.Expect(watcherRoute.Spec.TLS.Key).Should(Not(BeEmpty()))
				g.Expect(watcherRoute.Spec.TLS.CACertificate).Should(Not(BeEmpty()))
			}, timeout, interval).Should(Succeed())

			watcher := GetWatcher(names.WatcherName)
			Expect(watcher.Spec.APIServiceTemplate.TLS.API.Internal.SecretName).Should(Equal(ptr.To("cert-watcher-internal-svc")))
			Expect(watcher.Spec.APIServiceTemplate.TLS.API.Public.SecretName).Should(Equal(ptr.To("cert-watcher-public-svc")))
			Expect(watcher.Spec.APIServiceTemplate.Override.Service["public"].EndpointURL).Should(Not(BeNil()))

		})
		It("should have ControlPlaneWatcherReadyCondition false when watcher is not ready", func() {

			// expect the ready status to propagate to control plane object
			Eventually(func(_ Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneWatcherReadyCondition,
					k8s_corev1.ConditionFalse,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("should have ControlPlaneWatcherReadyCondition true when watcher is ready", func() {
			// simulate watcher ready state
			Eventually(func(g Gomega) {
				watcher := &watcherv1.Watcher{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.WatcherName, watcher)).Should(Succeed())
				watcher.Status.ObservedGeneration = watcher.Generation
				watcher.Status.Conditions.MarkTrue(condition.ReadyCondition, "Ready")
				g.Expect(th.K8sClient.Status().Update(th.Ctx, watcher)).To(Succeed())
				th.Logger.Info("Simulated Watcher ready", "on", names.WatcherName)
			}, timeout, interval).Should(Succeed())

			// expect the ready status to propagate to control plane object
			Eventually(func(_ Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneWatcherReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A watcher OpenStackControlplane instance is created with custom parameters", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["watcher"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"decisionengineServiceTemplate": map[string]interface{}{
						"customServiceConfig": "#testcustom",
					},
					"apiServiceTemplate": map[string]interface{}{
						"replicas": int32(2),
					},
					"databaseInstance": "custom-db",
					"databaseAccount":  "custom-account",
					"apiTimeout":       120,
				},
			}
			spec["telemetry"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"ceilometer": map[string]interface{}{
						"enabled": true,
					},
					"metricStorage": map[string]interface{}{
						"enabled": true,
					},
				},
			}

			// create cert secrets for rabbitmq instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))
			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.WatcherCertPublicRouteName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.WatcherCertPublicSvcName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.WatcherCertInternalName))

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})

		It("should have watcher enabled and default values", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Watcher.Enabled).Should(BeTrue())

			watcher := GetWatcher(names.WatcherName)
			// watcher services exist
			Eventually(func(g Gomega) {
				watcher := GetWatcher(names.WatcherName)
				g.Expect(watcher).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// default values
			Expect(watcher.Spec.ServiceUser).Should(Equal(ptr.To("watcher")))
			Expect(watcher.Spec.MemcachedInstance).Should(Equal(ptr.To("memcached")))
			Expect(OSCtlplane.Spec.Watcher.Template.DecisionEngineServiceTemplate.Replicas).Should(Equal(ptr.To(int32(1))))

			Expect(OSCtlplane.Spec.Watcher.APIOverride.Route).Should(Not(BeNil()))
			Expect(OSCtlplane.Spec.Watcher.APIOverride.Route.Annotations).Should(HaveKeyWithValue("haproxy.router.openshift.io/timeout", "120s"))
			Expect(OSCtlplane.Spec.Watcher.APIOverride.Route.Annotations).Should(HaveKeyWithValue("api.watcher.openstack.org/timeout", "120s"))
			// default Watche container images are set
			Expect(watcher.Spec.APIContainerImageURL).Should(Not(BeNil()))
			Expect(watcher.Spec.ApplierContainerImageURL).Should(Not(BeNil()))
			Expect(watcher.Spec.DecisionEngineContainerImageURL).Should(Equal("quay.io/podified-master-centos9/openstack-watcher-decision-engine:current-podified"))

		})

		It("should have watcher parameters from controlplane template", func() {
			watcher := GetWatcher(names.WatcherName)
			Expect(watcher.Spec.DecisionEngineServiceTemplate.CustomServiceConfig).Should(Equal("#testcustom"))
			Expect(watcher.Spec.APIServiceTemplate.Replicas).Should(Equal(ptr.To(int32(2))))
			Expect(watcher.Spec.DatabaseInstance).Should(Equal(ptr.To("custom-db")))
			Expect(watcher.Spec.DatabaseAccount).Should(Equal(ptr.To("custom-account")))
		})

		It("should have ControlPlaneWatcherReadyCondition false when watcher is not ready", func() {

			// expect the ready status to propagate to control plane object
			Eventually(func(_ Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneWatcherReadyCondition,
					k8s_corev1.ConditionFalse,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("should have ControlPlaneWatcherReadyCondition true when watcher is ready", func() {
			// simulate watcher ready state
			Eventually(func(g Gomega) {
				watcher := &watcherv1.Watcher{}
				g.Expect(th.K8sClient.Get(th.Ctx, names.WatcherName, watcher)).Should(Succeed())
				watcher.Status.ObservedGeneration = watcher.Generation
				watcher.Status.Conditions.MarkTrue(condition.ReadyCondition, "Ready")
				g.Expect(th.K8sClient.Status().Update(th.Ctx, watcher)).To(Succeed())
				th.Logger.Info("Simulated Watcher ready", "on", names.WatcherName)
			}, timeout, interval).Should(Succeed())

			// expect the ready status to propagate to control plane object
			Eventually(func(_ Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneWatcherReadyCondition,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("should delete the watcher instance when watcher is disabled", func() {
			// watcher services exist
			Eventually(func(g Gomega) {
				watcher := GetWatcher(names.WatcherName)
				g.Expect(watcher).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				OSCtlplane.Spec.Watcher.Enabled = false
				g.Expect(th.K8sClient.Update(th.Ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				instance := &watcherv1.Watcher{}
				err := th.K8sClient.Get(th.Ctx, names.WatcherName, instance)
				g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
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
					names.OpenStackVersionName,
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

	When("OpenStackControlplane instance is created and an OpenStackVersion already exists with an arbitrary name", func() {
		BeforeEach(func() {
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackVersion(names.OpenStackVersionName2, GetDefaultOpenStackVersionSpec()),
			)
		})

		It("rejects the OpenStackControlPlane if its name is not that same as the OpenStackVersion's name", func() {
			raw := map[string]interface{}{
				"apiVersion": "core.openstack.org/v1beta1",
				"kind":       "OpenStackControlPlane",
				"metadata": map[string]interface{}{
					"name":      names.OpenStackControlplaneName.Name,
					"namespace": names.Namespace,
				},
				"spec": GetDefaultOpenStackControlPlaneSpec(),
			}

			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).Should(HaveOccurred())
			var statusError *k8s_errors.StatusError
			Expect(errors.As(err, &statusError)).To(BeTrue())
			Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
			Expect(statusError.ErrStatus.Message).To(
				ContainSubstring(
					"must have same name as the existing"),
			)

			// we remove the finalizer as this is needed when the OpenStackControlplane
			// does not create the OpenStackVersion itself
			DeferCleanup(
				OpenStackVersionRemoveFinalizer,
				ctx,
				names.OpenStackVersionName2,
			)
		})

		It("accepts the OpenStackControlPlane if its name is the same as the OpenStackVersion's name", func() {
			raw := map[string]interface{}{
				"apiVersion": "core.openstack.org/v1beta1",
				"kind":       "OpenStackControlPlane",
				"metadata": map[string]interface{}{
					"name":      names.OpenStackVersionName2.Name,
					"namespace": names.Namespace,
				},
				"spec": GetDefaultOpenStackControlPlaneSpec(),
			}

			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).ShouldNot(HaveOccurred())

			openStackControlPlane := &corev1.OpenStackControlPlane{}
			openStackControlPlane.Namespace = names.Namespace
			openStackControlPlane.Name = names.OpenStackVersionName2.Name
			err = k8sClient.Delete(ctx, openStackControlPlane)
			Expect(err).ShouldNot(HaveOccurred())

			// we remove the finalizer as this is needed when the OpenStackControlplane
			// does not create the OpenStackVersion itself
			DeferCleanup(
				OpenStackVersionRemoveFinalizer,
				ctx,
				names.OpenStackVersionName2,
			)
		})
	})

	When("An OpenStackControlplane instance is created with nodeSelector", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["tls"] = GetTLSPublicSpec()
			nodeSelector := map[string]string{
				"foo": "bar",
			}
			spec["nodeSelector"] = nodeSelector
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)

			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
			keystone.SimulateKeystoneAPIReady(names.KeystoneAPIName)

			Eventually(func(g Gomega) {
				osversion := GetOpenStackVersion(names.OpenStackControlplaneName)
				g.Expect(osversion).Should(Not(BeNil()))

				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())

			th.CreateSecret(types.NamespacedName{Name: "openstack-config-secret", Namespace: namespace}, map[string][]byte{"secure.yaml": []byte("foo")})
			th.CreateConfigMap(types.NamespacedName{Name: "openstack-config", Namespace: namespace}, map[string]interface{}{"clouds.yaml": string("foo"), "OS_CLOUD": "default"})
		})

		It("sets nodeSelector in resource specs", func() {
			Eventually(func(g Gomega) {
				osc := th.GetPod(names.OpenStackClientName)
				g.Expect(osc.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(*db.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				rmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(*rmq.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

		})

		It("updates nodeSelector in resource specs when changed", func() {
			Eventually(func(g Gomega) {
				osc := th.GetPod(names.OpenStackClientName)
				g.Expect(osc.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(*db.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				rmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(*rmq.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				newNodeSelector := map[string]string{
					"foo2": "bar2",
				}
				OSCtlplane.Spec.NodeSelector = newNodeSelector
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				osc := th.GetPod(names.OpenStackClientName)
				g.Expect(osc.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(*db.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				rmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(*rmq.Spec.NodeSelector).To(Equal(map[string]string{"foo2": "bar2"}))
			}, timeout, interval).Should(Succeed())
		})

		It("allows nodeSelector service override", func() {
			Eventually(func(g Gomega) {
				osc := th.GetPod(names.OpenStackClientName)
				g.Expect(osc.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(*db.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				rmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(*rmq.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)

				oscNodeSelector := map[string]string{
					"foo": "osc",
				}
				OSCtlplane.Spec.OpenStackClient.Template.NodeSelector = &oscNodeSelector

				galeraNodeSelector := map[string]string{
					"foo": "galera",
				}
				galeraTemplates := *(OSCtlplane.Spec.Galera.Templates)
				dbTemplate := galeraTemplates[names.DBName.Name]
				dbTemplate.NodeSelector = &galeraNodeSelector
				galeraTemplates[names.DBName.Name] = dbTemplate
				OSCtlplane.Spec.Galera.Templates = &galeraTemplates

				rmqNodeSelector := map[string]string{
					"foo": "rmq",
				}
				rmqTemplates := *OSCtlplane.Spec.Rabbitmq.Templates
				rmqTemplate := rmqTemplates[names.RabbitMQName.Name]
				rmqTemplate.NodeSelector = &rmqNodeSelector
				rmqTemplates[names.RabbitMQName.Name] = rmqTemplate
				OSCtlplane.Spec.Rabbitmq.Templates = &rmqTemplates

				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				osc := th.GetPod(names.OpenStackClientName)
				g.Expect(osc.Spec.NodeSelector).To(Equal(map[string]string{"foo": "osc"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(*db.Spec.NodeSelector).To(Equal(map[string]string{"foo": "galera"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				rmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(*rmq.Spec.NodeSelector).To(Equal(map[string]string{"foo": "rmq"}))
			}, timeout, interval).Should(Succeed())
		})

		It("allows nodeSelector service override to empty", func() {
			Eventually(func(g Gomega) {
				osc := th.GetPod(names.OpenStackClientName)
				g.Expect(osc.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(*db.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				rmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(*rmq.Spec.NodeSelector).To(Equal(map[string]string{"foo": "bar"}))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)

				oscNodeSelector := map[string]string{}
				OSCtlplane.Spec.OpenStackClient.Template.NodeSelector = &oscNodeSelector

				galeraNodeSelector := map[string]string{}
				galeraTemplates := *(OSCtlplane.Spec.Galera.Templates)
				dbTemplate := galeraTemplates[names.DBName.Name]
				dbTemplate.NodeSelector = &galeraNodeSelector
				galeraTemplates[names.DBName.Name] = dbTemplate
				OSCtlplane.Spec.Galera.Templates = &galeraTemplates

				rmqNodeSelector := map[string]string{}
				rmqTemplates := *OSCtlplane.Spec.Rabbitmq.Templates
				rmqTemplate := rmqTemplates[names.RabbitMQName.Name]
				rmqTemplate.NodeSelector = &rmqNodeSelector
				rmqTemplates[names.RabbitMQName.Name] = rmqTemplate
				OSCtlplane.Spec.Rabbitmq.Templates = &rmqTemplates

				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				osc := th.GetPod(names.OpenStackClientName)
				g.Expect(osc.Spec.NodeSelector).To(BeNil())
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(*db.Spec.NodeSelector).To(Equal(map[string]string{}))
			}, timeout, interval).Should(Succeed())
			Eventually(func(g Gomega) {
				rmq := GetRabbitMQCluster(names.RabbitMQName)
				g.Expect(*rmq.Spec.NodeSelector).To(Equal(map[string]string{}))
			}, timeout, interval).Should(Succeed())
		})
	})

	When("An OpenStackControlplane instance references a wrong topology", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["topologyRef"] = map[string]interface{}{
				"name": "foo",
			}
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
		})
		It("points to a non existing topology CR", func() {
			// Reconciliation does not succeed because TopologyReadyCondition
			// is not marked as True
			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionFalse,
			)
			// TopologyReadyCondition is Unknown as it waits for the Topology
			// CR to be available
			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				condition.TopologyReadyCondition,
				k8s_corev1.ConditionFalse,
			)
		})
	})

	When("An OpenStackControlplane instance references an existing topology", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
			spec["topologyRef"] = map[string]string{
				"name": names.OpenStackTopology[0].Name,
			}
			spec["telemetry"] = map[string]interface{}{
				"enabled": true,
				"template": map[string]interface{}{
					"ceilometer": map[string]interface{}{
						"enabled": true,
					},
					"metricStorage": map[string]interface{}{
						"enabled": true,
					},
				},
			}
			spec["watcher"] = map[string]interface{}{
				"enabled": true,
			}
			// Build the topology Spec
			topologySpec := GetSampleTopologySpec()
			// Create Test Topologies
			for _, t := range names.OpenStackTopology {
				CreateTopology(t, topologySpec)
			}

			// create cert secrets for rabbitmq instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))
			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)
			Eventually(func(g Gomega) {
				keystoneAPI := keystone.GetKeystoneAPI(names.KeystoneAPIName)
				g.Expect(keystoneAPI).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
			keystone.SimulateKeystoneAPIReady(names.KeystoneAPIName)

			Eventually(func(g Gomega) {
				osversion := GetOpenStackVersion(names.OpenStackControlplaneName)
				g.Expect(osversion).Should(Not(BeNil()))

				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)
			}, timeout, interval).Should(Succeed())
			th.CreateSecret(types.NamespacedName{
				Name:      "openstack-config-secret",
				Namespace: namespace,
			}, map[string][]byte{"secure.yaml": []byte("foo")})

			th.CreateConfigMap(types.NamespacedName{
				Name:      "openstack-config",
				Namespace: namespace,
			}, map[string]interface{}{
				"clouds.yaml": string("foo"),
				"OS_CLOUD":    "default",
			})
		})
		It("points to an existing topology CR", func() {
			// TopologyReadyCondition is True
			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				condition.TopologyReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})
		DescribeTable("it is propagated to",
			func(serviceNameFunc func() (client.Object, *topologyv1.TopoRef)) {
				expectedTopology := &topologyv1.TopoRef{
					Name:      names.OpenStackTopology[0].Name,
					Namespace: names.OpenStackTopology[0].Namespace,
				}

				svc, toporef := serviceNameFunc()
				// service exists and TopologyRef has been propagated
				Eventually(func(g Gomega) {
					g.Expect(svc).Should(Not(BeNil()))
					g.Expect(toporef).To(Equal(expectedTopology))
				}, timeout, interval).Should(Succeed())
			},
			// The entry list depends on the default enabled services in the
			// default spec
			galeraService,
			keystoneService,
			rabbitService,
			memcachedService,
			telemetryService,
			glanceService,
			cinderService,
			manilaService,
			neutronService,
			horizonService,
			heatService,
			watcherService,
		)
		DescribeTable("An OpenStackControlplane updates the topology reference",
			func(serviceNameFunc func() (client.Object, *topologyv1.TopoRef)) {
				Eventually(func(g Gomega) {
					ctlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
					ctlplane.Spec.TopologyRef = &topologyv1.TopoRef{
						Name:      names.OpenStackTopology[1].Name,
						Namespace: names.Namespace,
					}
					g.Expect(k8sClient.Update(ctx, ctlplane)).To(Succeed())
					svc, toporef := serviceNameFunc()
					expectedTopology := &topologyv1.TopoRef{
						Name:      names.OpenStackTopology[1].Name,
						Namespace: names.OpenStackTopology[1].Namespace,
					}
					// service exists and TopologyRef has been propagated
					g.Expect(svc).Should(Not(BeNil()))
					g.Expect(toporef).To(Equal(expectedTopology))
				}, timeout, interval).Should(Succeed())
			},
			galeraService,
			keystoneService,
			rabbitService,
			memcachedService,
			telemetryService,
			glanceService,
			cinderService,
			manilaService,
			neutronService,
			horizonService,
			heatService,
			watcherService,
		)
		DescribeTable("An OpenStackControlplane Service (Glance) overrides the topology reference",
			func(serviceNameFunc func() (client.Object, *topologyv1.TopoRef)) {
				Eventually(func(g Gomega) {
					ctlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
					// Overrides a single resource
					ctlplane.Spec.TopologyRef = &topologyv1.TopoRef{
						Name:      names.OpenStackTopology[1].Name,
						Namespace: names.Namespace,
					}
					ctlplane.Spec.Glance.Template.TopologyRef = &topologyv1.TopoRef{
						Name:      names.OpenStackTopology[0].Name,
						Namespace: names.Namespace,
					}
					g.Expect(k8sClient.Update(ctx, ctlplane)).To(Succeed())
					svc, toporef := serviceNameFunc()
					servicesExpectedTopology := &topologyv1.TopoRef{
						Name:      names.OpenStackTopology[1].Name,
						Namespace: names.OpenStackTopology[1].Namespace,
					}
					// Override glance TopologyRef with the previous value
					glanceExpectedTopology := &topologyv1.TopoRef{
						Name:      names.OpenStackTopology[0].Name,
						Namespace: names.OpenStackTopology[0].Namespace,
					}
					// service exists and TopologyRef has been propagated
					g.Expect(svc).Should(Not(BeNil()))
					g.Expect(toporef).To(Equal(servicesExpectedTopology))

					// glance exists and TopologyRef has not been propagated
					glance := GetGlance(names.GlanceName)
					g.Expect(glance).Should(Not(BeNil()))
					g.Expect(glance.Spec.TopologyRef).To(Equal(glanceExpectedTopology))
				}, timeout, interval).Should(Succeed())
			},
			// The entry list depends on the default enabled services in the
			// default spec
			galeraService,
			keystoneService,
			rabbitService,
			memcachedService,
			telemetryService,
			cinderService,
			manilaService,
			neutronService,
			horizonService,
			heatService,
			watcherService,
		)
		DescribeTable("An OpenStackControlplane removes the topology reference",
			func(serviceNameFunc func() (client.Object, *topologyv1.TopoRef)) {
				Eventually(func(g Gomega) {
					ctlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
					ctlplane.Spec.TopologyRef = nil
					g.Expect(k8sClient.Update(ctx, ctlplane)).To(Succeed())

					svc, toporef := serviceNameFunc()
					// service exists and TopologyRef has been propagated
					g.Expect(svc).Should(Not(BeNil()))
					g.Expect(toporef).To(BeNil())
				}, timeout, interval).Should(Succeed())
			},
			// The entry list depends on the default enabled services in the
			// default spec
			galeraService,
			keystoneService,
			rabbitService,
			memcachedService,
			telemetryService,
			glanceService,
			cinderService,
			manilaService,
			neutronService,
			horizonService,
			heatService,
			watcherService,
		)
	})
})

var _ = Describe("OpenStackOperator Webhook", func() {

	DescribeTable("notificationsBusInstance",
		func(getNotificationField func() (string, string)) {
			spec := GetDefaultOpenStackControlPlaneSpec()
			value, errMsg := getNotificationField()
			spec["notificationsBusInstance"] = value
			raw := map[string]interface{}{
				"apiVersion": "core.openstack.org/v1beta1",
				"kind":       "OpenStackControlPlane",
				"metadata": map[string]interface{}{
					"name":      "foo",
					"namespace": namespace,
				},
				"spec": spec,
			}
			unstructuredObj := &unstructured.Unstructured{Object: raw}
			_, err := controllerutil.CreateOrPatch(
				th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
			Expect(err).Should(HaveOccurred())
			var statusError *k8s_errors.StatusError
			Expect(errors.As(err, &statusError)).To(BeTrue())
			Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
			Expect(statusError.ErrStatus.Message).To(
				ContainSubstring(errMsg),
			)
		},
		Entry("notificationsBusInstance is wrong", func() (string, string) {
			return "foo", "spec.notificationsBusInstance: Invalid value: \"foo\": notificationsBusInstance must match an existing RabbitMQ instance name"
		}),
		Entry("notificationsBusInstance is an empty string", func() (string, string) {
			return "", "spec.notificationsBusInstance: Invalid value: \"\": notificationsBusInstance is not a valid string"
		}),
	)

	It("Blocks creating multiple ctlplane CRs in the same namespace", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["tls"] = GetTLSPublicSpec()
		DeferCleanup(
			th.DeleteInstance,
			CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
		)

		OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
		Expect(OSCtlplane.Labels).Should(Not(BeNil()))
		Expect(OSCtlplane.Labels).Should(HaveKeyWithValue("core.openstack.org/openstackcontrolplane", ""))

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": OSCtlplane.GetNamespace(),
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Forbidden: Only one OpenStackControlPlane instance per namespace is supported at this time."),
		)
	})

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

	It("Enforces queueType=Quorum for new resources", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["tls"] = GetTLSPublicSpec()

		DeferCleanup(
			th.DeleteInstance,
			CreateOpenStackControlPlane(types.NamespacedName{Name: "test-new-quorum", Namespace: namespace}, spec),
		)

		OSCtlplane := GetOpenStackControlPlane(types.NamespacedName{Name: "test-new-quorum", Namespace: namespace})
		Expect(OSCtlplane.Spec.Rabbitmq.Templates).Should(Not(BeNil()))

		// Verify that all templates get queueType=Quorum for new resources
		for templateName, template := range *OSCtlplane.Spec.Rabbitmq.Templates {
			Expect(template.QueueType).Should(Equal("Quorum"), "RabbitMQ template %s should have queueType=Quorum", templateName)
		}
	})

	It("Preserves existing queueType values on updates", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["tls"] = GetTLSPublicSpec()

		// Create a resource first (will get Quorum by default)
		ctlplane := CreateOpenStackControlPlane(types.NamespacedName{Name: "test-preserve-existing", Namespace: namespace}, spec)
		DeferCleanup(th.DeleteInstance, ctlplane)

		// Manually set it to Mirrored to simulate existing deployment
		Eventually(func(g Gomega) {
			existingCtlplane := GetOpenStackControlPlane(types.NamespacedName{Name: "test-preserve-existing", Namespace: namespace})
			(*existingCtlplane.Spec.Rabbitmq.Templates)["rabbitmq"] = rabbitmqv1.RabbitMqSpecCore{
				QueueType: "Mirrored", // Simulate existing value
			}
			g.Expect(k8sClient.Update(ctx, existingCtlplane)).Should(Succeed())
		}).Should(Succeed())

		// Verify it's preserved on subsequent updates
		Eventually(func(g Gomega) {
			updatedCtlplane := GetOpenStackControlPlane(types.NamespacedName{Name: "test-preserve-existing", Namespace: namespace})
			updatedCtlplane.Spec.Secret = "updated-secret" // Trigger webhook
			g.Expect(k8sClient.Update(ctx, updatedCtlplane)).Should(Succeed())

			// Should still be Mirrored after update
			finalCtlplane := GetOpenStackControlPlane(types.NamespacedName{Name: "test-preserve-existing", Namespace: namespace})
			rabbitTemplate := (*finalCtlplane.Spec.Rabbitmq.Templates)["rabbitmq"]
			g.Expect(rabbitTemplate.QueueType).Should(Equal("Mirrored"), "Existing queueType should be preserved")
		}).Should(Succeed())
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

	It("Blocks creating ctlplane CRs with to long memcached keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		memcachedTemplate := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": map[string]interface{}{
				"replicas": 1,
			},
		}

		spec["memcached"] = map[string]interface{}{
			"enabled":   true,
			"templates": memcachedTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 52 characters"),
		)
	})

	It("Blocks creating ctlplane CRs with wrong memcached keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		memcachedTemplate := map[string]interface{}{
			"foo_bar": map[string]interface{}{
				"replicas": 1,
			},
		}

		spec["memcached"] = map[string]interface{}{
			"enabled":   true,
			"templates": memcachedTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo_bar\": a lowercase RFC 1123 label must consist"),
		)
	})

	It("Blocks creating ctlplane CRs with to long rabbitmq keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		rabbitmqTemplate := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": map[string]interface{}{
				"replicas": 1,
			},
		}

		spec["rabbitmq"] = map[string]interface{}{
			"enabled":   true,
			"templates": rabbitmqTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 52 characters"),
		)
	})

	It("Blocks creating ctlplane CRs with wrong rabbitmq keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		rabbitmqTemplate := map[string]interface{}{
			"foo_bar": map[string]interface{}{
				"replicas": 1,
			},
		}

		spec["rabbitmq"] = map[string]interface{}{
			"enabled":   true,
			"templates": rabbitmqTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo_bar\": a lowercase RFC 1123 label must consist"),
		)
	})

	It("Blocks creating ctlplane CRs with to long galera keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		galeraTemplate := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": map[string]interface{}{
				"storageRequest": "500M",
			},
		}

		spec["galera"] = map[string]interface{}{
			"enabled":   true,
			"templates": galeraTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 46 characters"),
		)
	})

	It("Blocks creating ctlplane CRs with wrong galera keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		galeraTemplate := map[string]interface{}{
			"foo_bar": map[string]interface{}{
				"storageRequest": "500M",
			},
		}

		spec["galera"] = map[string]interface{}{
			"enabled":   true,
			"templates": galeraTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo_bar\": a lowercase RFC 1123 label must consist"),
		)
	})

	It("Blocks creating ctlplane CRs with to long glanceapi keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		apiList := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": map[string]interface{}{
				"replicas": 1,
			},
		}

		glanceTemplate := map[string]interface{}{
			"databaseInstance": "openstack",
			"secret":           "secret",
			"databaseAccount":  "account",
			"glanceAPIs":       apiList,
		}

		spec["glance"] = map[string]interface{}{
			"enabled":        true,
			"uniquePodNames": false,
			"template":       glanceTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 39 characters"),
		)
	})

	It("Blocks creating ctlplane CRs with to long glanceapi keys/names (uniquePodNames)", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		apiList := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": map[string]interface{}{
				"replicas": 1,
			},
		}

		glanceTemplate := map[string]interface{}{
			"databaseInstance": "openstack",
			"secret":           "secret",
			"databaseAccount":  "account",
			"glanceAPIs":       apiList,
		}

		spec["glance"] = map[string]interface{}{
			"enabled":        true,
			"uniquePodNames": true,
			"template":       glanceTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 33 characters"),
		)
	})

	It("Blocks creating ctlplane CRs with wrong glanceapi keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		apiList := map[string]interface{}{
			"foo_bar": map[string]interface{}{
				"replicas": 1,
			},
		}

		glanceTemplate := map[string]interface{}{
			"databaseInstance": "openstack",
			"secret":           "secret",
			"databaseAccount":  "account",
			"glanceAPIs":       apiList,
		}

		spec["glance"] = map[string]interface{}{
			"enabled":  true,
			"template": glanceTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo_bar\": a lowercase RFC 1123 label must consist"),
		)
	})

	It("Blocks creating ctlplane CRs with to long cinderVolume keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		volumeList := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": map[string]interface{}{},
		}
		cinderTemplate := map[string]interface{}{
			"databaseInstance": "openstack",
			"secret":           "secret",
			"databaseAccount":  "account",
			"cinderVolumes":    volumeList,
		}

		spec["cinder"] = map[string]interface{}{
			"enabled":        true,
			"uniquePodNames": false,
			"template":       cinderTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 38 characters"),
		)
	})

	It("Blocks creating ctlplane CRs with to long cinderVolume keys/names (uniquePodNames)", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		volumeList := map[string]interface{}{
			"foo-1234567890-1234567890-1234567890-1234567890-1234567890": map[string]interface{}{},
		}
		cinderTemplate := map[string]interface{}{
			"databaseInstance": "openstack",
			"secret":           "secret",
			"databaseAccount":  "account",
			"cinderVolumes":    volumeList,
		}

		spec["cinder"] = map[string]interface{}{
			"enabled":        true,
			"uniquePodNames": true,
			"template":       cinderTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo-1234567890-1234567890-1234567890-1234567890-1234567890\": must be no more than 32 characters"),
		)
	})

	It("Blocks creating ctlplane CRs with wrong cinderVolume keys/names", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()

		volumeList := map[string]interface{}{
			"foo_bar": map[string]interface{}{},
		}
		cinderTemplate := map[string]interface{}{
			"databaseInstance": "openstack",
			"secret":           "secret",
			"databaseAccount":  "account",
			"cinderVolumes":    volumeList,
		}

		spec["cinder"] = map[string]interface{}{
			"enabled":        true,
			"uniquePodNames": true,
			"template":       cinderTemplate,
		}

		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackControlPlane"))
		Expect(statusError.ErrStatus.Message).To(
			ContainSubstring(
				"Invalid value: \"foo_bar\": a lowercase RFC 1123 label must consist"),
		)
	})
	It("Blocks creating ctlplane CRs with wrong topology namespace", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["topologyRef"] = map[string]interface{}{
			"name":      "foo",
			"namespace": "bar",
		}
		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(err.Error()).To(
			ContainSubstring(
				"Invalid value: \"namespace\": Customizing namespace field is not supported"),
		)
	})
	It("Blocks creating ctlplane CRs with watcher enabled without telemetry services", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
		spec["watcher"] = map[string]interface{}{
			"enabled": true,
		}
		raw := map[string]interface{}{
			"apiVersion": "core.openstack.org/v1beta1",
			"kind":       "OpenStackControlPlane",
			"metadata": map[string]interface{}{
				"name":      "foo",
				"namespace": namespace,
			},
			"spec": spec,
		}

		unstructuredObj := &unstructured.Unstructured{Object: raw}
		_, err := controllerutil.CreateOrPatch(
			th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
		Expect(err).Should(HaveOccurred())
		var statusError *k8s_errors.StatusError
		Expect(errors.As(err, &statusError)).To(BeTrue())
		Expect(err.Error()).To(
			ContainSubstring(
				"invalid: spec.watcher.enabled: Invalid value: true: Watcher requires these services to be enabled: Galera, Memcached, RabbitMQ, Keystone, Telemetry, Telemetry.Ceilometer, Telemetry.MetricStorage"),
		)
	})

})

var _ = Describe("OpenStackOperator controller nova cell deletion", func() {
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
	})

	When("openstack galera and rabbitmq deletion by cell", func() {

		var extGaleraName types.NamespacedName
		var extRabbitName types.NamespacedName

		BeforeEach(func() {
			// create cert secrets for galera instances
			th.CreateCertSecret(names.DBCertName)
			th.CreateCertSecret(names.DBCell1CertName)

			// create cert secrets for rabbitmq instances
			th.CreateCertSecret(names.RabbitMQCertName)
			th.CreateCertSecret(names.RabbitMQCell1CertName)

			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NeutronOVNCertName))

			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))

			extGalera := CreateGaleraConfig(namespace, GetDefaultGaleraSpec())
			extGaleraName.Name = extGalera.GetName()
			extGaleraName.Namespace = extGalera.GetNamespace()
			DeferCleanup(th.DeleteInstance, extGalera)

			extRabbitMq := CreateRabbitMQConfig(namespace, GetDefaultRabbitMQSpec())
			extRabbitName.Name = extRabbitMq.GetName()
			extRabbitName.Namespace = extRabbitMq.GetNamespace()
			DeferCleanup(th.DeleteInstance, extRabbitMq)

			spec := GetDefaultOpenStackControlPlaneSpec()
			// not sure why we must need tls for galera, on commenting it, got below error
			// Message: "galeras.mariadb.openstack.org \"openstack\" not found",
			spec["tls"] = GetTLSPublicSpec()
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)

			// enable TLS
			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				OSCtlplane.Spec.TLS.PodLevel.Enabled = true
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())
		})

		It("cell1 galera should be deleted from CR", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Galera.Enabled).Should(BeTrue())

			var secretName types.NamespacedName
			var certName types.NamespacedName

			// galera exists
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBName)
				g.Expect(db).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())
			// cell1 is present in galera
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(names.DBCell1Name)

				secretName = types.NamespacedName{Name: *db.Spec.TLS.SecretName, Namespace: namespace}
				secret := th.GetSecret(secretName)

				certName = types.NamespacedName{Name: "galera-openstack-cell1-svc", Namespace: namespace}
				cert := crtmgr.GetCert(certName)

				g.Expect(db).Should(Not(BeNil()))
				g.Expect(secret).Should(Not(BeNil()))
				g.Expect(cert).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// external galera exists
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(extGaleraName)
				g.Expect(db).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				galeraTemplates := *(OSCtlplane.Spec.Galera.Templates)
				g.Expect(galeraTemplates).Should(HaveLen(2))
				delete(galeraTemplates, names.DBCell1Name.Name)
				OSCtlplane.Spec.Galera.Templates = &galeraTemplates
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			// Only 1 cell in galera template
			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				galeraTemplates := *(OSCtlplane.Spec.Galera.Templates)
				g.Expect(galeraTemplates).Should(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			// cell1.galera should not exists in db
			Eventually(func(g Gomega) {
				db := &mariadbv1.Galera{}
				err := th.K8sClient.Get(ctx, names.DBCell1Name, db)
				g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			// cert is deleted too
			crtmgr.AssertCertDoesNotExist(certName)

			// secret is not deleted
			Eventually(func(g Gomega) {
				secret := th.GetSecret(secretName)
				g.Expect(secret).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// external galera exists even after openstack-cell1 deletion
			Eventually(func(g Gomega) {
				db := mariadb.GetGalera(extGaleraName)
				g.Expect(db).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

		})

		It("cell1 rabbitmq should be deleted from CR", func() {
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Rabbitmq.Enabled).Should(BeTrue())

			var secretName types.NamespacedName
			var certName types.NamespacedName

			// rabbitmq exists
			Eventually(func(g Gomega) {
				rabbitmq := GetRabbitMQCluster(names.RabbitMQCell1Name)

				secretName = types.NamespacedName{Name: "cert-rabbitmq-cell1-svc", Namespace: namespace}
				secret := th.GetSecret(secretName)

				certName = types.NamespacedName{Name: "rabbitmq-cell1-svc", Namespace: namespace}
				cert := crtmgr.GetCert(certName)

				g.Expect(rabbitmq).Should(Not(BeNil()))
				g.Expect(rabbitmq.Spec.Replicas).Should(Equal(ptr.To[int32](1)))
				g.Expect(secret).Should(Not(BeNil()))
				g.Expect(cert).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// external rabbitmq exists
			Eventually(func(g Gomega) {
				rabbitmq := GetRabbitMQCluster(extRabbitName)
				g.Expect(rabbitmq).Should(Not(BeNil()))
				g.Expect(rabbitmq.Spec.Replicas).Should(Equal(ptr.To[int32](1)))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				rabbitTemplates := *(OSCtlplane.Spec.Rabbitmq.Templates)
				g.Expect(rabbitTemplates).Should(HaveLen(2))
				delete(rabbitTemplates, names.RabbitMQCell1Name.Name)
				OSCtlplane.Spec.Rabbitmq.Templates = &rabbitTemplates
				g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			// Only 1 cell in rabbitmq template
			Eventually(func(g Gomega) {
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				rabbitTemplates := *(OSCtlplane.Spec.Rabbitmq.Templates)
				g.Expect(rabbitTemplates).Should(HaveLen(1))
			}, timeout, interval).Should(Succeed())

			// cell1.rabbitmq should not exists in db
			Eventually(func(g Gomega) {
				db := &rabbitmqv1.RabbitMq{}
				err := th.K8sClient.Get(ctx, names.RabbitMQCell1Name, db)
				g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
			}, timeout, interval).Should(Succeed())

			// cert is deleted too
			crtmgr.AssertCertDoesNotExist(certName)

			// secret is not deleted
			Eventually(func(g Gomega) {
				secret := th.GetSecret(secretName)
				g.Expect(secret).Should(Not(BeNil()))
			}, timeout, interval).Should(Succeed())

			// external rabbitmq exists even after rabbitmq-cell1 deletion
			Eventually(func(g Gomega) {
				rabbitmq := GetRabbitMQCluster(extRabbitName)
				g.Expect(rabbitmq).Should(Not(BeNil()))
				g.Expect(rabbitmq.Spec.Replicas).Should(Equal(ptr.To[int32](1)))
			}, timeout, interval).Should(Succeed())

		})

		When("The novncproxy k8s service is created for cell1", func() {
			/*
				- generate certs and routes for novncproxy
					- enable nova and dependencies
					- create novncproxy service
				- find and verify certs and routes are created
				- reproduce cell1 deletion
					- remove cell1 from oscp CR
					- delete novncproxy service
				- verify if there are no residue certs and routes
			*/

			BeforeEach(func() {
				// enable Nova and dependencies
				Eventually(func(g Gomega) {
					OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
					OSCtlplane.Spec.Nova.Enabled = true
					OSCtlplane.Spec.Nova.Template = &novav1.NovaSpecCore{}
					// enable "Galera, Memcached, RabbitMQ, Keystone, Glance, Neutron, Placement" too

					OSCtlplane.Spec.Keystone.Enabled = true
					OSCtlplane.Spec.Glance.Enabled = true
					OSCtlplane.Spec.Neutron.Enabled = true
					OSCtlplane.Spec.Placement.Enabled = true

					if OSCtlplane.Spec.Placement.Template == nil {
						OSCtlplane.Spec.Placement.Template = &placementv1.PlacementAPISpecCore{}
						OSCtlplane.Spec.Placement.Template.APITimeout = 10
					}
					g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
				}, timeout, interval).Should(Succeed())

				// nova-novncproxy-cell1-public
				novncProxyPublicSvcName := types.NamespacedName{
					Name:      "nova-novncproxy-cell1-public",
					Namespace: namespace}

				th.CreateService(
					novncProxyPublicSvcName,
					map[string]string{
						"osctlplane-service": "nova-novncproxy",
						"osctlplane":         "",
						"cell":               "cell1",
					},
					k8s_corev1.ServiceSpec{
						Ports: []k8s_corev1.ServicePort{
							{
								Name:     "nova-novncproxy-cell1-public",
								Port:     int32(6080),
								Protocol: k8s_corev1.ProtocolTCP,
							},
						},
					})

				novncProxySvc := th.GetService(novncProxyPublicSvcName)

				if novncProxySvc.Annotations == nil {
					novncProxySvc.Annotations = map[string]string{}
				}

				novncProxySvc.Annotations[service.AnnotationIngressCreateKey] = "true"
				novncProxySvc.Annotations[service.AnnotationEndpointKey] = "public"

				Expect(th.K8sClient.Status().Update(th.Ctx, novncProxySvc)).To(Succeed())
				// novncProxySvc = th.GetService(novncProxyPublicSvcName)
				// logger.Info("", "XXX novncproxy labels", novncProxySvc.Labels)

				// vnproxy certs
				DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NoVNCProxyCell1CertPublicRouteName))
				DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NoVNCProxyCell1CertPublicSvcName))
				DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.NoVNCProxyCell1CertVencryptName))

			})

			It("novncproxy certs and routes should be deleted on respective cell deletion", func() {
				certNames := []types.NamespacedName{
					{Name: "nova-novncproxy-cell1-public-route", Namespace: namespace},
					{Name: "nova-novncproxy-cell1-public-svc", Namespace: namespace},
					{Name: "nova-novncproxy-cell1-vencrypt", Namespace: namespace},
				}

				// verify all certs for novncproxy exists
				Eventually(func(g Gomega) {
					for _, certName := range certNames {
						cert := crtmgr.GetCert(certName)
						g.Expect(cert).NotTo(BeNil())
					}
				}, timeout, interval).Should(Succeed())

				// verify route is present
				Eventually(func(g Gomega) {
					novncproxyRouteName := types.NamespacedName{Name: "nova-novncproxy-cell1-public", Namespace: namespace}
					novncproxyRoute := &routev1.Route{}

					g.Expect(th.K8sClient.Get(th.Ctx, novncproxyRouteName, novncproxyRoute)).Should(Succeed())
					g.Expect(novncproxyRoute.Spec.TLS).Should(Not(BeNil()))
					g.Expect(novncproxyRoute.Spec.TLS.Certificate).Should(Not(BeEmpty()))
					g.Expect(novncproxyRoute.Spec.TLS.Key).Should(Not(BeEmpty()))
					g.Expect(novncproxyRoute.Spec.TLS.CACertificate).Should(Not(BeEmpty()))
				}, timeout, interval).Should(Succeed())

				// remove from oscp
				Eventually(func(g Gomega) {
					OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
					delete(OSCtlplane.Spec.Nova.Template.CellTemplates, "cell1")
					g.Expect(k8sClient.Update(ctx, OSCtlplane)).Should(Succeed())
				}, timeout, interval).Should(Succeed())

				th.DeleteService(
					types.NamespacedName{
						Name:      "nova-novncproxy-cell1-public",
						Namespace: namespace,
					})

				// verify all certs for novncproxy
				for _, certName := range certNames {
					crtmgr.AssertCertDoesNotExist(certName)
				}

				// verify route for novncproxy
				novncproxyRouteName := types.NamespacedName{Name: "nova-novncproxy-cell1-public", Namespace: namespace}
				novncproxyRoute := &routev1.Route{}
				Eventually(func(g Gomega) {
					err := th.K8sClient.Get(th.Ctx, novncproxyRouteName, novncproxyRoute)
					g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
				}, timeout, interval).Should(Succeed())
			})

		})
	})
})
