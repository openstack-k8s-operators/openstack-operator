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
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
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

	})

	When("A default OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
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

		It("should create full ca bundle", func() {
			crtmgr.GetCert(names.RootCAPublicName)
			crtmgr.GetIssuer(names.RootCAPublicName)

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
			th.CreateSecret(types.NamespacedName{Name: "openstack-config-secret", Namespace: namespace}, map[string][]byte{"secure.yaml": []byte("foo")})
			th.CreateConfigMap(types.NamespacedName{Name: "openstack-config", Namespace: namespace}, map[string]interface{}{"clouds.yaml": string("foo"), "OS_CLOUD": "default"})

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
			}, timeout, interval).Should(Succeed())
		})
	})
	When("A OVN OpenStackControlplane instance is created", func() {
		BeforeEach(func() {
			spec := GetDefaultOpenStackControlPlaneSpec()
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
			// TODO: had to disable tls to allow control plane status to update
			spec["tls"] = map[string]interface{}{}
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
})

var _ = Describe("OpenStackOperator Webhook", func() {

	It("calls placement validation webhook", func() {
		spec := GetDefaultOpenStackControlPlaneSpec()
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
