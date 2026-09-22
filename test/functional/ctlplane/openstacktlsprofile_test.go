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

package functional_test

import (
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	ocp_configv1 "github.com/openshift/api/config/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// createAPIServer - create the cluster-scoped APIServer CR carrying the
// cluster-wide TLS security profile, and remove it again after the spec
func createAPIServer(profile *ocp_configv1.TLSSecurityProfile) *ocp_configv1.APIServer {
	apiServer := &ocp_configv1.APIServer{
		ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
		Spec:       ocp_configv1.APIServerSpec{TLSSecurityProfile: profile},
	}
	Expect(k8sClient.Create(ctx, apiServer)).Should(Succeed())
	// tolerate specs that delete the singleton themselves
	DeferCleanup(deleteAPIServer, apiServer)
	return apiServer
}

// deleteAPIServer - drop the cluster-scoped singleton, tolerating its absence
func deleteAPIServer(apiServer *ocp_configv1.APIServer) {
	err := k8sClient.Delete(ctx, apiServer)
	if err != nil && !k8s_errors.IsNotFound(err) {
		Expect(err).ShouldNot(HaveOccurred())
	}
}

// expectNoTLSProfileConfigMap - the operator publishes nothing, so lib-common
// renders its own built-in defaults
func expectNoTLSProfileConfigMap() {
	cm := &k8s_corev1.ConfigMap{}
	Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, types.NamespacedName{
			Name:      util.TLSProfileConfigMap,
			Namespace: names.Namespace,
		}, cm)
		g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())
}

// getTLSProfileConfigMap - read back the ConfigMap the operator publishes for
// lib-common to merge into every rendered Template
func getTLSProfileConfigMap() *k8s_corev1.ConfigMap {
	cm := &k8s_corev1.ConfigMap{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name:      util.TLSProfileConfigMap,
			Namespace: names.Namespace,
		}, cm)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return cm
}

// profileCiphers - the cipher list OpenShift ships for a built-in profile,
// joined the way Apache httpd expects it
func profileCiphers(profileType ocp_configv1.TLSProfileType) string {
	return strings.Join(ocp_configv1.TLSProfiles[profileType].Ciphers, ":")
}

// defaultInheritingSpec - the default control plane spec with cluster TLS
// profile inheritance explicitly enabled: the CRD default is off, so every
// spec that expects the operator to publish the profile has to ask for it
func defaultInheritingSpec() map[string]interface{} {
	spec := GetDefaultOpenStackControlPlaneSpec()
	spec["tls"] = map[string]interface{}{"inheritClusterProfile": true}
	return spec
}

// hasTLSProfileCondition - whether the control plane status still carries
// the TLS profile condition
func hasTLSProfileCondition(cp *corev1.OpenStackControlPlane) bool {
	for _, c := range cp.Status.Conditions {
		if c.Type == corev1.OpenStackControlPlaneTLSProfileReadyCondition {
			return true
		}
	}
	return false
}

var _ = Describe("OpenStackOperator TLS profile", func() {
	BeforeEach(func() {
		err := os.Setenv("OPERATOR_TEMPLATES", "../../templates")
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(k8sClient.Delete, ctx, CreateClusterConfigCM())

		// (mschuppert) create root CA secrets as there is no certmanager
		// running. ReconcileCAs requeues until they all exist, and
		// ReconcileTLSProfile runs right after it.
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAPublicName))
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAInternalName))
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAOvnName))
		DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCALibvirtName))
		// create cert secrets for galera instances
		DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.DBCertName))
		DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.DBCell1CertName))
	})

	// Every cluster TLS profile has to land in the ConfigMap as the pair of
	// httpd directives the service operators template into ssl.conf, plus the
	// minimum version in neutral form for the services that are not httpd.
	DescribeTable("publishes the resolved httpd directives as a ConfigMap",
		func(
			profile *ocp_configv1.TLSSecurityProfile,
			cipherSuite string,
			sslProtocol string,
			minTLSVersion ocp_configv1.TLSProtocolVersion,
		) {
			createAPIServer(profile)
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, defaultInheritingSpec()),
			)

			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				corev1.OpenStackControlPlaneTLSProfileReadyCondition,
				k8s_corev1.ConditionTrue,
			)

			cm := getTLSProfileConfigMap()
			Expect(cm.Data).To(HaveKeyWithValue("SSLCipherSuite", cipherSuite))
			Expect(cm.Data).To(HaveKeyWithValue("SSLProtocol", sslProtocol))
			Expect(cm.Data).To(HaveKeyWithValue("MinTLSVersion", string(minTLSVersion)))

			// owned by the control plane, so it is garbage-collected
			// together with it
			cp := &corev1.OpenStackControlPlane{}
			Expect(k8sClient.Get(ctx, names.OpenStackControlplaneName, cp)).To(Succeed())
			Expect(metav1.IsControlledBy(cm, cp)).To(BeTrue())
		},
		Entry("with no profile set, falling back to Intermediate",
			nil,
			profileCiphers(ocp_configv1.TLSProfileIntermediateType),
			"all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1",
			ocp_configv1.VersionTLS12),
		Entry("with the Old profile",
			&ocp_configv1.TLSSecurityProfile{Type: ocp_configv1.TLSProfileOldType},
			profileCiphers(ocp_configv1.TLSProfileOldType),
			"all -SSLv2 -SSLv3",
			ocp_configv1.VersionTLS10),
		Entry("with the Intermediate profile",
			&ocp_configv1.TLSSecurityProfile{Type: ocp_configv1.TLSProfileIntermediateType},
			profileCiphers(ocp_configv1.TLSProfileIntermediateType),
			"all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1",
			ocp_configv1.VersionTLS12),
		Entry("with the Modern profile",
			&ocp_configv1.TLSSecurityProfile{Type: ocp_configv1.TLSProfileModernType},
			profileCiphers(ocp_configv1.TLSProfileModernType),
			"all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1 -TLSv1.2",
			ocp_configv1.VersionTLS13),
		Entry("with a Custom profile, carrying the administrator's own ciphers and minimum version",
			&ocp_configv1.TLSSecurityProfile{
				Type: ocp_configv1.TLSProfileCustomType,
				Custom: &ocp_configv1.CustomTLSProfile{
					TLSProfileSpec: ocp_configv1.TLSProfileSpec{
						Ciphers:       []string{"ECDHE-RSA-AES256-GCM-SHA384", "DHE-RSA-AES256-GCM-SHA384"},
						MinTLSVersion: ocp_configv1.VersionTLS11,
					},
				},
			},
			"ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES256-GCM-SHA384",
			"all -SSLv2 -SSLv3 -TLSv1",
			ocp_configv1.VersionTLS11),
	)

	When("the APIServer CR does not exist", func() {
		BeforeEach(func() {
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, defaultInheritingSpec()),
			)
		})

		It("publishes no ConfigMap and leaves lib-common on its own defaults", func() {
			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				corev1.OpenStackControlPlaneTLSProfileReadyCondition,
				k8s_corev1.ConditionTrue,
			)

			cm := &k8s_corev1.ConfigMap{}
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      util.TLSProfileConfigMap,
					Namespace: names.Namespace,
				}, cm)
				g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})
	})

	When("the cluster asks for a Custom profile but provides no custom section", func() {
		BeforeEach(func() {
			createAPIServer(&ocp_configv1.TLSSecurityProfile{
				Type: ocp_configv1.TLSProfileCustomType,
			})
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, defaultInheritingSpec()),
			)
		})

		It("reports the profile as not ready and publishes nothing", func() {
			th.ExpectCondition(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				corev1.OpenStackControlPlaneTLSProfileReadyCondition,
				k8s_corev1.ConditionFalse,
			)

			cm := &k8s_corev1.ConfigMap{}
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      util.TLSProfileConfigMap,
				Namespace: names.Namespace,
			}, cm)
			Expect(k8s_errors.IsNotFound(err)).To(BeTrue())
		})
	})

	When("the cluster switches to a different TLS security profile", func() {
		var apiServer *ocp_configv1.APIServer

		BeforeEach(func() {
			apiServer = createAPIServer(&ocp_configv1.TLSSecurityProfile{
				Type: ocp_configv1.TLSProfileOldType,
			})
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, defaultInheritingSpec()),
			)
		})

		It("republishes the ConfigMap with the new directives", func() {
			cm := getTLSProfileConfigMap()
			Expect(cm.Data).To(HaveKeyWithValue(
				"SSLCipherSuite", profileCiphers(ocp_configv1.TLSProfileOldType)))

			// an admin tightening the cluster profile has to reach the
			// services: the watch on the APIServer CR is what makes the
			// controlplane notice
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx,
					types.NamespacedName{Name: "cluster"}, apiServer)).Should(Succeed())
				apiServer.Spec.TLSSecurityProfile = &ocp_configv1.TLSSecurityProfile{
					Type: ocp_configv1.TLSProfileModernType,
				}
				g.Expect(k8sClient.Update(ctx, apiServer)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				updated := &k8s_corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name:      util.TLSProfileConfigMap,
					Namespace: names.Namespace,
				}, updated)).Should(Succeed())
				g.Expect(updated.Data).To(HaveKeyWithValue(
					"SSLCipherSuite", profileCiphers(ocp_configv1.TLSProfileModernType)))
				g.Expect(updated.Data).To(HaveKeyWithValue(
					"SSLProtocol", "all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1 -TLSv1.2"))
			}, timeout, interval).Should(Succeed())
		})
	})

	When("the published ConfigMap is deleted out from under the operator", func() {
		BeforeEach(func() {
			createAPIServer(&ocp_configv1.TLSSecurityProfile{
				Type: ocp_configv1.TLSProfileOldType,
			})
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, defaultInheritingSpec()),
			)
		})

		It("republishes it with the current profile", func() {
			cm := getTLSProfileConfigMap()
			Expect(cm.Data).To(HaveKeyWithValue(
				"SSLCipherSuite", profileCiphers(ocp_configv1.TLSProfileOldType)))

			Expect(k8sClient.Delete(ctx, cm)).Should(Succeed())

			// nothing else fires for the control plane at this point: the
			// watch on the ConfigMap itself is what has to bring it back
			updated := getTLSProfileConfigMap()
			Expect(updated.Data).To(HaveKeyWithValue(
				"SSLCipherSuite", profileCiphers(ocp_configv1.TLSProfileOldType)))
		})
	})

	When("the cluster stops declaring a TLS security profile", func() {
		var apiServer *ocp_configv1.APIServer

		BeforeEach(func() {
			apiServer = createAPIServer(&ocp_configv1.TLSSecurityProfile{
				Type: ocp_configv1.TLSProfileModernType,
			})
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, defaultInheritingSpec()),
			)
		})

		It("withdraws the ConfigMap so lib-common falls back to its defaults", func() {
			// the profile is published first
			cm := getTLSProfileConfigMap()
			Expect(cm.Data).To(HaveKeyWithValue(
				"SSLCipherSuite", profileCiphers(ocp_configv1.TLSProfileModernType)))

			// removing the cluster-wide profile has to withdraw it again,
			// otherwise services stay pinned to a profile the cluster no
			// longer declares
			deleteAPIServer(apiServer)

			expectNoTLSProfileConfigMap()

			// ready, but for the "cluster mandates nothing" reason: the
			// message has to say so rather than claim a ConfigMap exists
			th.ExpectConditionWithDetails(
				names.OpenStackControlplaneName,
				ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
				corev1.OpenStackControlPlaneTLSProfileReadyCondition,
				k8s_corev1.ConditionTrue,
				condition.ReadyReason,
				corev1.OpenStackControlPlaneTLSProfileReadyNoProfileMessage,
			)
		})
	})

	When("the control plane does not inherit the cluster TLS profile", func() {
		BeforeEach(func() {
			// the cluster declares a profile that would otherwise be
			// published, and the default spec leaves inheritance off
			createAPIServer(&ocp_configv1.TLSSecurityProfile{
				Type: ocp_configv1.TLSProfileModernType,
			})
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, GetDefaultOpenStackControlPlaneSpec()),
			)
		})

		It("publishes no ConfigMap and sets no TLS profile condition", func() {
			Consistently(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      util.TLSProfileConfigMap,
					Namespace: names.Namespace,
				}, &k8s_corev1.ConfigMap{})
				g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())

				cp := &corev1.OpenStackControlPlane{}
				g.Expect(k8sClient.Get(ctx, names.OpenStackControlplaneName, cp)).To(Succeed())
				g.Expect(hasTLSProfileCondition(cp)).To(BeFalse())
			}, timeout, interval).Should(Succeed())
		})
	})

	When("inheritance is switched off while a profile is published", func() {
		BeforeEach(func() {
			createAPIServer(&ocp_configv1.TLSSecurityProfile{
				Type: ocp_configv1.TLSProfileOldType,
			})
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, defaultInheritingSpec()),
			)
		})

		It("withdraws the ConfigMap and removes the TLS profile condition", func() {
			cm := getTLSProfileConfigMap()
			Expect(cm.Data).To(HaveKeyWithValue(
				"SSLCipherSuite", profileCiphers(ocp_configv1.TLSProfileOldType)))

			// an admin keeping the feature disabled, e.g. through a minor
			// update: the services must go back to the built-in defaults
			Eventually(func(g Gomega) {
				cp := &corev1.OpenStackControlPlane{}
				g.Expect(k8sClient.Get(ctx, names.OpenStackControlplaneName, cp)).Should(Succeed())
				cp.Spec.TLS.InheritClusterProfile = false
				g.Expect(k8sClient.Update(ctx, cp)).Should(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      util.TLSProfileConfigMap,
					Namespace: names.Namespace,
				}, &k8s_corev1.ConfigMap{})
				g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue())

				cp := &corev1.OpenStackControlPlane{}
				g.Expect(k8sClient.Get(ctx, names.OpenStackControlplaneName, cp)).To(Succeed())
				g.Expect(hasTLSProfileCondition(cp)).To(BeFalse())
			}, timeout, interval).Should(Succeed())
		})
	})
})
