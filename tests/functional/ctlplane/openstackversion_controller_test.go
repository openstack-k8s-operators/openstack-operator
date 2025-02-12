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

package functional_test

import (
	"errors"
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	//revive:disable-next-line:dot-imports
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
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

	})

	When("A default OpenStackVersion instance is created with no Controlplane", func() {
		BeforeEach(func() {
			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackVersion(names.OpenStackVersionName, GetDefaultOpenStackVersionSpec()),
			)

			// Ensure that the version instance is not marked new any more
			// to avoid racing between the below cleanup removing the finalizer
			// and the controller adding the finalizer to the new instance.
			th.ExpectCondition(
				names.OpenStackVersionName,
				ConditionGetterFunc(OpenStackVersionConditionGetter),
				corev1.OpenStackVersionInitialized,
				k8s_corev1.ConditionTrue,
			)

			// we remove the finalizer as this is needed without the Controlplane
			DeferCleanup(
				OpenStackVersionRemoveFinalizer,
				ctx,
				names.OpenStackVersionName,
			)
		})

		It("should fail to create more than one OpenStackVersion", func() {

			instance := &corev1.OpenStackVersion{}
			instance.ObjectMeta.Namespace = names.Namespace
			instance.Name = "foo"
			err := k8sClient.Create(ctx, instance)

			Expect(err).Should(HaveOccurred())
			var statusError *k8s_errors.StatusError
			Expect(errors.As(err, &statusError)).To(BeTrue())
			Expect(statusError.ErrStatus.Details.Kind).To(Equal("OpenStackVersion"))
			Expect(statusError.ErrStatus.Message).To(
				ContainSubstring(
					"Forbidden: Only one OpenStackVersion instance is supported at this time."),
			)

		})

		It("should initialize container images", func() {
			Eventually(func(g Gomega) {

				version := GetOpenStackVersion(names.OpenStackVersionName)
				g.Expect(version).Should(Not(BeNil()))

				// no condition which reflects an update is available
				g.Expect(version.Status.Conditions.Has(corev1.OpenStackVersionMinorUpdateAvailable)).To(BeFalse())

				g.Expect(*version.Status.AvailableVersion).Should(ContainSubstring("0.0.1"))
				g.Expect(version.Spec.TargetVersion).Should(ContainSubstring("0.0.1"))

				g.Expect(version.Status.ContainerImages.AgentImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.AnsibleeeImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.AodhAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.AodhEvaluatorImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.AodhListenerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.AodhNotifierImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.ApacheImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.BarbicanAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.BarbicanKeystoneListenerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.BarbicanWorkerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CeilometerCentralImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CeilometerComputeImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CeilometerNotificationImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CeilometerSgcoreImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CeilometerProxyImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CeilometerMysqldExporterImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CinderAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CinderBackupImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.CinderSchedulerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.DesignateAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.DesignateBackendbind9Image).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.DesignateCentralImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.DesignateMdnsImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.DesignateProducerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.DesignateUnboundImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.DesignateWorkerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmFrrImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmIscsidImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmLogrotateCrondImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmMultipathdImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmNeutronMetadataAgentImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmNeutronSriovAgentImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmNodeExporterImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmKeplerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmOvnBgpAgentImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.GlanceAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.HeatAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.HeatCfnapiImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.HeatEngineImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.InfraDnsmasqImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.InfraMemcachedImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.InfraRedisImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.IronicAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.IronicConductorImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.IronicInspectorImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.IronicNeutronAgentImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.IronicPxeImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.IronicPythonAgentImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.KeystoneAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.ManilaAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.ManilaSchedulerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.MariadbImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.NetUtilsImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.NeutronAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.NovaAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.NovaComputeImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.NovaConductorImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.NovaNovncImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.NovaSchedulerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OctaviaApacheImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OctaviaAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OctaviaHealthmanagerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OctaviaHousekeepingImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OctaviaWorkerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OctaviaRsyslogImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OpenstackClientImage).ShouldNot(BeNil())
				//fixme wire this one in
				//g.Expect(version.Status.ContainerImages.OsContainerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OvnControllerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OvnControllerOvsImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OvnNbDbclusterImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OvnNorthdImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.OvnSbDbclusterImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.PlacementAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.RabbitmqImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.SwiftAccountImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.SwiftContainerImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.SwiftObjectImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.SwiftProxyImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmNodeExporterImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.EdpmKeplerImage).ShouldNot(BeNil())

			}, timeout, interval).Should(Succeed())
		})

	})

	// target version is *not* multiple OSVersion objects. Each time there is an update to a container image
	// a new targetVersion is "discovered". This test is meant to simulate that environment
	When("Multiple target versions exist", func() {

		initialVersion := "old"
		updatedVersion := "0.0.1"
		targetOvnControllerVersion := ""
		testOvnControllerImage := "foo/bar:0.0.2"

		// a lightweight controlplane spec we'll use for minor update testing
		// we are missing some test helpers to simulate ready state so once we have
		// we can go back to a more complete controlplane spec
		BeforeEach(func() {

			// this is a very "lightweight" controlplane for minimal
			spec := GetDefaultOpenStackControlPlaneSpec()

			// a single galera database
			galeraTemplate := map[string]interface{}{
				names.DBName.Name: map[string]interface{}{
					"storageRequest": "500M",
				},
			}
			spec["galera"] = map[string]interface{}{
				"enabled":   true,
				"templates": galeraTemplate,
			}

			spec["horizon"] = map[string]interface{}{
				"enabled": false,
			}

			spec["glance"] = map[string]interface{}{
				"enabled": false,
			}
			spec["cinder"] = map[string]interface{}{
				"enabled": false,
			}
			spec["neutron"] = map[string]interface{}{
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
				CreateOpenStackVersion(names.OpenStackVersionName, GetDefaultOpenStackVersionSpec()),
			)

			// create cert secrets for rabbitmq instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.RabbitMQCell1CertName))

			// (mschuppert) create root CA secrets as there is no certmanager running.
			// it is not used, just to make sure reconcile proceeds and creates the ca-bundle.
			DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAPublicName))
			DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAInternalName))
			DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCAOvnName))
			DeferCleanup(k8sClient.Delete, ctx, CreateCertSecret(names.RootCALibvirtName))
			// create cert secrets for galera instances
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.DBCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.DBCell1CertName))
			// create cert secrets for memcached instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.MemcachedCertName))
			// create cert secrets for ovn instance
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNNorthdCertName))
			DeferCleanup(k8sClient.Delete, ctx, th.CreateCertSecret(names.OVNControllerCertName))

			// wait for initial version to be created (this gives us version 0.0.1)
			Eventually(func(g Gomega) {

				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)

				version := GetOpenStackVersion(names.OpenStackVersionName)
				// capture this here as we'll need it below (this one comes from RELATED_IMAGES in hack/export_related_images.sh)
				targetOvnControllerVersion = *version.Status.ContainerImages.OvnControllerImage
				g.Expect(version).Should(Not(BeNil()))

				g.Expect(*version.Status.AvailableVersion).Should(ContainSubstring("0.0.1"))
				g.Expect(version.Spec.TargetVersion).Should(ContainSubstring("0.0.1"))
				updatedVersion = *version.Status.AvailableVersion
			}, timeout, interval).Should(Succeed())

			// inject an "old" version
			Eventually(func(g Gomega) {
				version := GetOpenStackVersion(names.OpenStackVersionName)
				version.Status.ContainerImageVersionDefaults[initialVersion] = version.Status.ContainerImageVersionDefaults[updatedVersion]
				version.Status.ContainerImageVersionDefaults[initialVersion].OvnControllerImage = &testOvnControllerImage
				g.Expect(th.K8sClient.Status().Update(th.Ctx, version)).To(Succeed())

				th.Logger.Info("Version injected", "on", names.OpenStackVersionName)
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				version := GetOpenStackVersion(names.OpenStackVersionName)
				version.Spec.TargetVersion = initialVersion
				g.Expect(th.K8sClient.Update(th.Ctx, version)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {

				osversion := GetOpenStackVersion(names.OpenStackVersionName)
				g.Expect(osversion).Should(Not(BeNil()))
				g.Expect(osversion.Generation).Should(Equal(osversion.Status.ObservedGeneration))

				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)

				g.Expect(*osversion.Status.AvailableVersion).Should(Equal(updatedVersion))
				g.Expect(osversion.Spec.TargetVersion).Should(Equal(initialVersion))
				g.Expect(osversion.Status.DeployedVersion).Should(BeNil())
				// but the images should stay the same as we haven't switched to it yet
				g.Expect(*osversion.Status.ContainerImages.OvnControllerImage).Should(Equal(testOvnControllerImage))

			}, timeout, interval).Should(Succeed())

			DeferCleanup(
				th.DeleteInstance,
				CreateOpenStackControlPlane(names.OpenStackControlplaneName, spec),
			)

			DeferCleanup(
				th.DeleteInstance,
				CreateDataplaneNodeSet(names.OpenStackVersionName, DefaultDataPlaneNoNodeSetSpec(false)),
			)

			dataplanenodeset := GetDataplaneNodeset(names.OpenStackVersionName)
			dataplanenodeset.Status.DeployedVersion = initialVersion
			Expect(th.K8sClient.Status().Update(th.Ctx, dataplanenodeset)).To(Succeed())

			th.CreateSecret(types.NamespacedName{Name: "openstack-config-secret", Namespace: namespace}, map[string][]byte{"secure.yaml": []byte("foo")})
			th.CreateConfigMap(types.NamespacedName{Name: "openstack-config", Namespace: namespace}, map[string]interface{}{"clouds.yaml": string("foo"), "OS_CLOUD": "default"})

			// verify that the controlplane deploys the old OVN controller image
			OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
			Expect(OSCtlplane.Spec.Ovn.Enabled).Should(BeTrue())

			SimulateControlplaneReady()

			// verify that DeployedVersion is set on the OpenStackControlplane to the initialversion
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					condition.ReadyCondition,
					k8s_corev1.ConditionTrue,
				)
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				g.Expect(OSCtlplane.Status.DeployedVersion).Should(Equal(&initialVersion))

			}, timeout, interval).Should(Succeed())

			// verify DeployedVersion also gets set on the OpenStackVersion resource
			Eventually(func(g Gomega) {

				osversion := GetOpenStackVersion(names.OpenStackVersionName)
				g.Expect(osversion).Should(Not(BeNil()))
				g.Expect(osversion.Generation).Should(Equal(osversion.Status.ObservedGeneration))

				g.Expect(osversion.Status.DeployedVersion).Should(Equal(&initialVersion))

			}, timeout, interval).Should(Succeed())

		})

		// 1) bump the targetVersion to 0.0.1
		// 2) verify that the OVN controller image gets updated on the controlplane
		// 3) simulate the OVN controller image getting updated on the dataplane
		// 4) verify that the rest of the container images get updated on the controlplane
		// 5) simulate 1 more dataplanenodeset update to finish the minor update workflow
		It("updating targetVersion triggers a minor update workflow", func() {

			// 1) switch to version 0.0.1, this triggers a minor update
			osversion := GetOpenStackVersion(names.OpenStackVersionName)

			// should have a condition which reflects an update is available
			th.ExpectCondition(
				names.OpenStackVersionName,
				ConditionGetterFunc(OpenStackVersionConditionGetter),
				corev1.OpenStackVersionMinorUpdateAvailable,
				k8s_corev1.ConditionTrue,
			)

			osversion.Spec.TargetVersion = updatedVersion
			Expect(k8sClient.Update(ctx, osversion)).Should(Succeed())

			// verify the OpenStackVersion gets re-initialized with 0.0.1 image for OVN
			Eventually(func(g Gomega) {

				osversion := GetOpenStackVersion(names.OpenStackVersionName)
				g.Expect(osversion).Should(Not(BeNil()))
				g.Expect(osversion.Generation).Should(Equal(osversion.Status.ObservedGeneration))

				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionInitialized,
					k8s_corev1.ConditionTrue,
				)

				g.Expect(*osversion.Status.AvailableVersion).Should(Equal(updatedVersion))
				g.Expect(osversion.Spec.TargetVersion).Should(Equal(updatedVersion))
				// the target OVN Controller image should be set
				g.Expect(*osversion.Status.ContainerImages.OvnControllerImage).Should(Equal(targetOvnControllerVersion))

			}, timeout, interval).Should(Succeed())

			// 2) now we check that the target OVN version gets set on the OVN Controller
			SimulateControlplaneReady()
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					corev1.OpenStackControlPlaneOVNReadyCondition,
					k8s_corev1.ConditionTrue,
				)
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					condition.ReadyCondition,
					k8s_corev1.ConditionUnknown,
				)

				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				// verify the image is set
				g.Expect(*OSCtlplane.Status.ContainerImages.OvnControllerImage).Should(Equal(targetOvnControllerVersion))

			}, timeout, interval).Should(Succeed())

			// verify that OpenStackVersion is in the correct state (control plane OVN got updated)
			Eventually(func(g Gomega) {

				osversion := GetOpenStackVersion(names.OpenStackVersionName)
				g.Expect(osversion).Should(Not(BeNil()))
				g.Expect(osversion.OwnerReferences).Should(HaveLen(1))
				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionMinorUpdateOVNControlplane,
					k8s_corev1.ConditionTrue,
				)

			}, timeout, interval).Should(Succeed())

			// 3) simulate the OVN controller image getting updated on the dataplane
			// NOTE: the real workflow here requires manual steps as well for now
			dataplanenodeset := GetDataplaneNodeset(names.OpenStackVersionName)
			dataplanenodeset.Status.ObservedGeneration = dataplanenodeset.Generation
			dataplanenodeset.Status.ContainerImages = make(map[string]string)
			dataplanenodeset.Status.ContainerImages["OvnControllerImage"] = targetOvnControllerVersion
			dataplanenodeset.Status.Conditions.MarkTrue(condition.ReadyCondition, dataplanev1.NodeSetReadyMessage)
			Expect(th.K8sClient.Status().Update(th.Ctx, dataplanenodeset)).To(Succeed())

			// and now finally we verify that OpenStackVersion is in the correct state (data plane OVN got updated)
			Eventually(func(g Gomega) {

				osversion := GetOpenStackVersion(names.OpenStackVersionName)
				g.Expect(osversion).Should(Not(BeNil()))
				g.Expect(osversion.OwnerReferences).Should(HaveLen(1))
				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionMinorUpdateOVNDataplane,
					k8s_corev1.ConditionTrue,
				)

			}, timeout, interval).Should(Succeed())

			// 4) verify that the rest of the container images get updated on the controlplane
			// this would occur automatically via the watch on the DataPlaneNodeSet's by openstackcontrolplane
			// so once the administrator executes the DataplaneDeployment and that finishes the controlplane will update the images immediately
			SimulateControlplaneReady()
			// now we check that the rest of the container images got updated
			Eventually(func(g Gomega) {
				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					corev1.OpenStackVersionMinorUpdateControlplane,
					k8s_corev1.ConditionTrue,
				)
				th.ExpectCondition(
					names.OpenStackControlplaneName,
					ConditionGetterFunc(OpenStackControlPlaneConditionGetter),
					condition.ReadyCondition,
					k8s_corev1.ConditionTrue,
				)

				osversion := GetOpenStackVersion(names.OpenStackVersionName)
				OSCtlplane := GetOpenStackControlPlane(names.OpenStackControlplaneName)
				// verify images match for deployed services on the controlplane
				g.Expect(OSCtlplane.Status.ContainerImages.MariadbImage).Should(Equal(osversion.Status.ContainerImages.MariadbImage))
				g.Expect(OSCtlplane.Status.ContainerImages.KeystoneAPIImage).Should(Equal(osversion.Status.ContainerImages.KeystoneAPIImage))
				g.Expect(OSCtlplane.Status.ContainerImages.InfraMemcachedImage).Should(Equal(osversion.Status.ContainerImages.InfraMemcachedImage))
				g.Expect(OSCtlplane.Status.ContainerImages.OvnControllerImage).Should(Equal(osversion.Status.ContainerImages.OvnControllerImage))
				g.Expect(OSCtlplane.Status.ContainerImages.OvnControllerOvsImage).Should(Equal(osversion.Status.ContainerImages.OvnControllerOvsImage))
				g.Expect(OSCtlplane.Status.ContainerImages.OvnNbDbclusterImage).Should(Equal(osversion.Status.ContainerImages.OvnNbDbclusterImage))
				g.Expect(OSCtlplane.Status.ContainerImages.OvnNorthdImage).Should(Equal(osversion.Status.ContainerImages.OvnNorthdImage))
				g.Expect(OSCtlplane.Status.ContainerImages.OvnSbDbclusterImage).Should(Equal(osversion.Status.ContainerImages.OvnSbDbclusterImage))

			}, timeout, interval).Should(Succeed())

			// 5) simulate 1 more dataplanenodeset update to finish the minor update workflow
			// NOTE: the real workflow here requires manual intervention as well for now
			dataplanenodeset = GetDataplaneNodeset(names.OpenStackVersionName)
			dataplanenodeset.Status.ObservedGeneration = dataplanenodeset.Generation
			dataplanenodeset.Status.DeployedVersion = osversion.Spec.TargetVersion
			dataplanenodeset.Status.Conditions.MarkTrue(condition.ReadyCondition, dataplanev1.NodeSetReadyMessage)
			Expect(th.K8sClient.Status().Update(th.Ctx, dataplanenodeset)).To(Succeed())

			// and now finally we verify that OpenStackVersion is in the correct state (data plane conditions, etc)
			Eventually(func(g Gomega) {

				osversion := GetOpenStackVersion(names.OpenStackVersionName)
				g.Expect(osversion).Should(Not(BeNil()))
				g.Expect(osversion.OwnerReferences).Should(HaveLen(1))
				th.ExpectCondition(
					names.OpenStackVersionName,
					ConditionGetterFunc(OpenStackVersionConditionGetter),
					condition.ReadyCondition,
					k8s_corev1.ConditionTrue,
				)
				g.Expect(osversion.Status.DeployedVersion).Should(Equal(&updatedVersion)) // we're done here
				// no condition which reflects an update is available
				g.Expect(osversion.Status.Conditions.Has(corev1.OpenStackVersionMinorUpdateAvailable)).To(BeFalse())
			}, timeout, interval).Should(Succeed())

		})

	})

})
