package functional

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
)

var _ = Describe("DataplaneDeployment Webhook", func() {
	var dataplaneDeploymentName types.NamespacedName

	createDeployment := func(name types.NamespacedName) {
		CreateDataplaneDeploymentWithoutConfirm(name, DefaultDataPlaneDeploymentSpec())
		DeferCleanup(func() {
			instance := &dataplanev1.OpenStackDataPlaneDeployment{}
			err := th.K8sClient.Get(th.Ctx, name, instance)
			if apierrors.IsNotFound(err) {
				return
			}
			Expect(err).NotTo(HaveOccurred())

			if instance.Annotations == nil {
				instance.Annotations = map[string]string{}
			}
			instance.Annotations[dataplanev1.ConfirmDeleteAnnotation] = "true"
			Expect(th.K8sClient.Update(th.Ctx, instance)).To(Succeed())
			Expect(th.K8sClient.Delete(th.Ctx, instance)).To(Succeed())
		})
	}

	setDeploymentRunning := func(name types.NamespacedName) {
		Eventually(func(g Gomega) error {
			instance := &dataplanev1.OpenStackDataPlaneDeployment{}
			g.Expect(th.K8sClient.Get(th.Ctx, name, instance)).To(Succeed())
			instance.Status.Conditions = condition.Conditions{}
			instance.Status.Conditions.MarkFalse(
				condition.DeploymentReadyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				condition.DeploymentReadyRunningMessage,
			)
			return th.K8sClient.Status().Update(th.Ctx, instance)
		}).Should(Succeed())
	}

	setDeploymentCompleted := func(name types.NamespacedName) {
		Eventually(func(g Gomega) error {
			instance := &dataplanev1.OpenStackDataPlaneDeployment{}
			g.Expect(th.K8sClient.Get(th.Ctx, name, instance)).To(Succeed())
			instance.Status.Conditions = condition.Conditions{}
			instance.Status.Conditions.MarkTrue(
				condition.DeploymentReadyCondition,
				condition.DeploymentReadyMessage,
			)
			return th.K8sClient.Status().Update(th.Ctx, instance)
		}).Should(Succeed())
	}

	When("a running deployment is deleted without the confirm annotation", func() {
		BeforeEach(func() {
			dataplaneDeploymentName = types.NamespacedName{
				Name:      "edpm-running-delete-blocked",
				Namespace: namespace,
			}
			createDeployment(dataplaneDeploymentName)
			setDeploymentRunning(dataplaneDeploymentName)
		})

		It("should reject the delete request", func() {
			Eventually(func() string {
				instance := GetDataplaneDeployment(dataplaneDeploymentName)
				err := th.K8sClient.Delete(th.Ctx, instance)
				return fmt.Sprintf("%s", err)
			}).Should(ContainSubstring("deletion of a running deployment not allowed"))
		})
	})

	When("a running deployment is deleted with the confirm annotation", func() {
		BeforeEach(func() {
			dataplaneDeploymentName = types.NamespacedName{
				Name:      "edpm-running-delete-confirmed",
				Namespace: namespace,
			}
			createDeployment(dataplaneDeploymentName)
			setDeploymentRunning(dataplaneDeploymentName)
		})

		It("should allow the delete request", func() {
			Eventually(func(g Gomega) error {
				instance := GetDataplaneDeployment(dataplaneDeploymentName)
				if instance.Annotations == nil {
					instance.Annotations = map[string]string{}
				}
				instance.Annotations[dataplanev1.ConfirmDeleteAnnotation] = "true"
				g.Expect(th.K8sClient.Update(th.Ctx, instance)).To(Succeed())
				return th.K8sClient.Delete(th.Ctx, instance)
			}).Should(Succeed())

			Eventually(func() bool {
				instance := &dataplanev1.OpenStackDataPlaneDeployment{}
				err := th.K8sClient.Get(th.Ctx, dataplaneDeploymentName, instance)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())
		})
	})

	When("a completed deployment is deleted without the confirm annotation", func() {
		BeforeEach(func() {
			dataplaneDeploymentName = types.NamespacedName{
				Name:      "edpm-completed-delete-allowed",
				Namespace: namespace,
			}
			createDeployment(dataplaneDeploymentName)
			setDeploymentCompleted(dataplaneDeploymentName)
		})

		It("should allow the delete request", func() {
			Eventually(func() error {
				instance := GetDataplaneDeployment(dataplaneDeploymentName)
				return th.K8sClient.Delete(th.Ctx, instance)
			}).Should(Succeed())

			Eventually(func() bool {
				instance := &dataplanev1.OpenStackDataPlaneDeployment{}
				err := th.K8sClient.Get(th.Ctx, dataplaneDeploymentName, instance)
				return apierrors.IsNotFound(err)
			}).Should(BeTrue())
		})
	})
})
