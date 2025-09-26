package functional

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	v1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("DataplaneNodeSet Webhook", func() {

	var dataplaneNodeSetName types.NamespacedName
	var dataplaneDeploymentName types.NamespacedName

	BeforeEach(func() {
		dataplaneNodeSetName = types.NamespacedName{
			Name:      "edpm-compute-nodeset",
			Namespace: namespace,
		}
		dataplaneDeploymentName = types.NamespacedName{
			Name:      "edpm-deployment",
			Namespace: namespace,
		}
		err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
		Expect(err).NotTo(HaveOccurred())
	})

	When("User tries to change forbidden items in the baremetalSetTemplate", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(false)
			nodeSetSpec["preProvisioned"] = false
			nodeSetSpec["nodes"] = map[string]any{
				"compute-0": map[string]any{
					"hostName": "compute-0"},
			}
			nodeSetSpec["baremetalSetTemplate"] = map[string]any{
				"bmhLabelSelector": map[string]string{
					"app": "test-openstack",
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
		})

		It("Should block changes to the BmhLabelSelector object in baremetalSetTemplate spec", func() {
			Eventually(func(_ Gomega) string {
				instance := GetDataplaneNodeSet(dataplaneNodeSetName)
				instance.Spec.BaremetalSetTemplate.BmhLabelSelector = map[string]string{
					"app": "openstack1",
				}
				err := th.K8sClient.Update(th.Ctx, instance)
				return fmt.Sprintf("%s", err)
			}).Should(ContainSubstring("Forbidden: cannot change"))
		})
	})

	When("A user changes an allowed field in the baremetalSetTemplate", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(false)
			nodeSetSpec["preProvisioned"] = false
			nodeSetSpec["baremetalSetTemplate"] = map[string]any{
				"cloudUserName": "test-user",
				"bmhLabelSelector": map[string]string{
					"app": "test-openstack",
				},
				"baremetalHosts": map[string]any{
					"compute-0": map[string]any{
						"ctlPlaneIP": "192.168.1.12/24",
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
		})
		It("Should allow changes to the CloudUserName", func() {
			Eventually(func(_ Gomega) error {
				instance := GetDataplaneNodeSet(dataplaneNodeSetName)
				instance.Spec.BaremetalSetTemplate.CloudUserName = "new-user"
				instance.Spec.BaremetalSetTemplate.BmhLabelSelector = map[string]string{
					"app": "test-openstack",
				}

				return th.K8sClient.Update(th.Ctx, instance)
			}).Should(Succeed())
		})
	})

	When("domainName in baremetalSetTemplate", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(false)
			nodeSetSpec["preProvisioned"] = false
			nodeSetSpec["nodes"] = map[string]any{
				"compute-0": map[string]any{
					"hostName": "compute-0"},
			}
			nodeSetSpec["baremetalSetTemplate"] = map[string]any{
				"domainName": "example.com",
				"bmhLabelSelector": map[string]string{
					"app": "test-openstack",
				},
				"baremetalHosts": map[string]any{
					"compute-0": map[string]any{
						"ctlPlaneIP": "192.168.1.12/24",
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
		})

		It("hostName should be fqdn", func() {
			instance := GetDataplaneNodeSet(dataplaneNodeSetName)
			Expect(instance.Spec.Nodes["compute-0"].HostName).Should(Equal(
				"compute-0.example.com"))
		})

	})

	When("A user tries to redeclare an existing node in a new NodeSet", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(false)
			nodeSetSpec["preProvisioned"] = true
			nodeSetSpec["nodes"] = map[string]any{
				"compute-0": map[string]any{
					"hostName": "compute-0"},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
		})

		It("Should block duplicate node declaration", func() {
			Eventually(func(_ Gomega) string {
				newNodeSetSpec := DefaultDataPlaneNoNodeSetSpec(false)
				newNodeSetSpec["preProvisioned"] = true
				newNodeSetSpec["nodes"] = map[string]any{
					"compute-0": map[string]any{
						"hostName": "compute-0"},
				}
				newInstance := DefaultDataplaneNodeSetTemplate(types.NamespacedName{Name: "test-duplicate-node", Namespace: namespace}, newNodeSetSpec)
				unstructuredObj := &unstructured.Unstructured{Object: newInstance}
				_, err := controllerutil.CreateOrPatch(
					th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
				return fmt.Sprintf("%s", err)
			}).Should(ContainSubstring("already exists in another cluster"))
		})

		It("Should block NodeSets if they contain a duplicate ansibleHost", func() {
			Eventually(func(_ Gomega) string {
				newNodeSetSpec := DefaultDataPlaneNoNodeSetSpec(false)
				newNodeSetSpec["preProvisioned"] = true
				newNodeSetSpec["nodes"] = map[string]any{
					"compute-3": map[string]any{
						"hostName": "compute-3",
						"ansible": map[string]any{
							"ansibleHost": "compute-3",
						},
					},
					"compute-2": map[string]any{
						"hostName": "compute-2"},
					"compute-8": map[string]any{
						"hostName": "compute-8"},
					"compute-0": map[string]any{
						"ansible": map[string]any{
							"ansibleHost": "compute-0",
						},
					},
				}
				newInstance := DefaultDataplaneNodeSetTemplate(types.NamespacedName{Name: "test-nodeset-with-duplicate-node", Namespace: namespace}, newNodeSetSpec)
				unstructuredObj := &unstructured.Unstructured{Object: newInstance}
				_, err := controllerutil.CreateOrPatch(
					th.Ctx, th.K8sClient, unstructuredObj, func() error { return nil })
				return fmt.Sprintf("%s", err)
			}).Should(ContainSubstring("already exists in another cluster"))
		})
	})
	When("A NodeSet is updated with a OpenStackDataPlaneDeployment", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(false)
			nodeSetSpec["preProvisioned"] = true
			nodeSetSpec["nodes"] = map[string]any{
				"compute-0": map[string]any{
					"hostName": "compute-0"},
			}

			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
		})
		It("Should allow for NodeSet updates if Deployment is Completed", func() {
			Eventually(func(g Gomega) error {
				instance := GetDataplaneNodeSet(dataplaneNodeSetName)
				instance.Spec.NodeTemplate.Ansible = v1beta1.AnsibleOpts{
					AnsibleUser: "random-user",
				}

				deploymentReadyConditions := condition.Conditions{}
				deploymentReadyConditions.MarkTrue(
					v1beta1.NodeSetDeploymentReadyCondition,
					condition.ReadyMessage)

				instance.Status.DeploymentStatuses = make(map[string]condition.Conditions)
				instance.Status.DeploymentStatuses[dataplaneDeploymentName.Name] = deploymentReadyConditions
				g.Expect(th.K8sClient.Status().Update(th.Ctx, instance)).To(Succeed())

				return th.K8sClient.Update(th.Ctx, instance)
			}).Should(Succeed())
		})
		It("Should block NodeSet updates if Deployment is NOT completed", func() {
			Eventually(func(g Gomega) string {
				instance := GetDataplaneNodeSet(dataplaneNodeSetName)

				deploymentReadyConditions := condition.Conditions{}
				deploymentReadyConditions.MarkFalse(
					v1beta1.NodeSetDeploymentReadyCondition,
					"mock-error",
					condition.SeverityWarning,
					condition.ReadyMessage)

				instance.Status.DeploymentStatuses = make(map[string]condition.Conditions)
				instance.Status.DeploymentStatuses[dataplaneDeploymentName.Name] = deploymentReadyConditions
				g.Expect(th.K8sClient.Status().Update(th.Ctx, instance)).To(Succeed())

				instance.Spec.NodeTemplate.Ansible = v1beta1.AnsibleOpts{
					AnsibleUser: "random-user",
				}
				err := th.K8sClient.Update(th.Ctx, instance)
				return fmt.Sprintf("%s", err)
			}).Should(ContainSubstring(fmt.Sprintf("could not patch openstackdataplanenodeset while openstackdataplanedeployment %s (blocked on %s condition) is running",
				dataplaneDeploymentName.Name, string(v1beta1.NodeSetDeploymentReadyCondition))))
		})
	})
})
