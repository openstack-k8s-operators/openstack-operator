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
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"

	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
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

				g.Expect(*version.Status.AvailableVersion).Should(Equal("0.0.1"))
				g.Expect(version.Spec.TargetVersion).Should(Equal("0.0.1"))

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
				g.Expect(version.Status.ContainerImages.EdpmOvnBgpAgentImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.GlanceAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.HeatAPIImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.HeatCfnapiImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.HeatEngineImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.InfraDnsmasqImage).ShouldNot(BeNil())
				g.Expect(version.Status.ContainerImages.InfraMemcachedImage).ShouldNot(BeNil())
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

			}, timeout, interval).Should(Succeed())
		})

	})

})
