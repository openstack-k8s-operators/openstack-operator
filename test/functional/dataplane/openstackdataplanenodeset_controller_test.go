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
	"encoding/json"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	"gopkg.in/yaml.v3"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// Ansible Inventory Structs for testing specific values
type AnsibleInventory struct {
	EdpmComputeNodeset struct {
		Vars struct {
			AnsibleUser string `yaml:"ansible_user"`
		} `yaml:"vars"`
		Hosts struct {
			Node struct {
				AnsibleHost       string        `yaml:"ansible_host"`
				AnsiblePort       string        `yaml:"ansible_port"`
				AnsibleUser       string        `yaml:"ansible_user"`
				CtlPlaneIP        string        `yaml:"ctlplane_ip"`
				DNSSearchDomains  []interface{} `yaml:"dns_search_domains"`
				ManagementNetwork string        `yaml:"management_network"`
				Networks          []interface{} `yaml:"networks"`
			} `yaml:"edpm-compute-node-1"`
		} `yaml:"hosts"`
	} `yaml:"edpm-compute-nodeset"`
}

var _ = Describe("Dataplane NodeSet Test", func() {
	var dataplaneNodeSetName types.NamespacedName
	var dataplaneSecretName types.NamespacedName
	var dataplaneSSHSecretName types.NamespacedName
	var caBundleSecretName types.NamespacedName
	var dataplaneNetConfigName types.NamespacedName
	var dnsMasqName types.NamespacedName
	var dataplaneNodeName types.NamespacedName
	var dataplaneDeploymentName types.NamespacedName
	var dataplaneConfigHash string
	var dataplaneGlobalServiceName types.NamespacedName
	var dataplaneUpdateServiceName types.NamespacedName
	var newDataplaneUpdateServiceName types.NamespacedName

	defaultEdpmServiceList := []string{
		"edpm_frr_image",
		"edpm_iscsid_image",
		"edpm_logrotate_crond_image",
		"edpm_neutron_metadata_agent_image",
		"edpm_nova_compute_image",
		"edpm_ovn_controller_agent_image",
		"edpm_ovn_bgp_agent_image",
	}

	BeforeEach(func() {
		dnsMasqName = types.NamespacedName{
			Name:      "dnsmasq",
			Namespace: namespace,
		}
		dataplaneNodeSetName = types.NamespacedName{
			Name:      "edpm-compute-nodeset",
			Namespace: namespace,
		}
		dataplaneSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "dataplanenodeset-edpm-compute-nodeset",
		}
		dataplaneSSHSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "dataplane-ansible-ssh-private-key-secret",
		}
		caBundleSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "combined-ca-bundle",
		}
		dataplaneNetConfigName = types.NamespacedName{
			Namespace: namespace,
			Name:      "dataplane-netconfig",
		}
		dataplaneNodeName = types.NamespacedName{
			Namespace: namespace,
			Name:      "edpm-compute-node-1",
		}
		dataplaneDeploymentName = types.NamespacedName{
			Name:      "edpm-deployment",
			Namespace: namespace,
		}
		dataplaneGlobalServiceName = types.NamespacedName{
			Name:      "global-service",
			Namespace: namespace,
		}
		dataplaneUpdateServiceName = types.NamespacedName{
			Name:      "update",
			Namespace: namespace,
		}
		newDataplaneUpdateServiceName = types.NamespacedName{
			Name:      "update-services",
			Namespace: namespace,
		}
		err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
		Expect(err).NotTo(HaveOccurred())
	})
	When("A Dataplane nodeset is created and no netconfig", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance,
				CreateDataplaneNodeSet(dataplaneNodeSetName,
					DefaultDataPlaneNoNodeSetSpec(false)))
		})
		It("should have ip reservation not ready and unknown Conditions initialized", func() {
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionUnknown,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				dataplanev1.NodeSetIPReservationReadyCondition,
				corev1.ConditionFalse,
			)
		})
	})

	When("A Dataplane nodeset is created and no ctlplane network in networks", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance,
				CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))

			DeferCleanup(th.DeleteInstance,
				CreateDataplaneNodeSet(dataplaneNodeSetName,
					DefaultDataPlaneNoNodeSetSpec(false)))
		})

		It("Should fail to set NodeSetIPReservationReadyCondition true when ctlplane is not in the networks", func() {
			Eventually(func(g Gomega) {
				instance := GetDataplaneNodeSet(dataplaneNodeSetName)
				instance.Spec.NodeTemplate.Networks[1].Name = "notctlplane"
				g.Expect(th.K8sClient.Update(th.Ctx, instance)).Should(Succeed())
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionUnknown,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.NodeSetIPReservationReadyCondition,
					corev1.ConditionFalse,
				)
				conditions := DataplaneConditionGetter(dataplaneNodeSetName)
				message := &conditions.Get(dataplanev1.NodeSetIPReservationReadyCondition).Message
				g.Expect(*message).Should(ContainSubstring("ctlplane network should be defined for node"))
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A Dataplane nodeset is created and no dnsmasq", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance,
				CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance,
				CreateDataplaneNodeSet(dataplaneNodeSetName,
					DefaultDataPlaneNoNodeSetSpec(false)))
			SimulateIPSetComplete(dataplaneNodeName)
		})
		It("should have dnsdata not ready and unknown Conditions initialized", func() {
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionUnknown,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				dataplanev1.NodeSetIPReservationReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				dataplanev1.NodeSetDNSDataReadyCondition,
				corev1.ConditionFalse,
			)

		})
	})

	When("A Dataplane nodeset is created and more than one dnsmasq", func() {
		BeforeEach(func() {
			DeferCleanup(th.DeleteInstance,
				CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			firstDNSMasqName := types.NamespacedName{
				Name:      "first-dnsmasq",
				Namespace: namespace,
			}
			DeferCleanup(th.DeleteInstance,
				CreateDNSMasq(firstDNSMasqName, DefaultDNSMasqSpec()))
			secondDNSMasqName := types.NamespacedName{
				Name:      "second-dnsmasq",
				Namespace: namespace,
			}
			DeferCleanup(th.DeleteInstance,
				CreateDNSMasq(secondDNSMasqName, DefaultDNSMasqSpec()))
			DeferCleanup(th.DeleteInstance,
				CreateDataplaneNodeSet(dataplaneNodeSetName,
					DefaultDataPlaneNoNodeSetSpec(false)))
			SimulateIPSetComplete(dataplaneNodeName)
		})
		It("should have multiple dnsdata error message and unknown Conditions initialized", func() {
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionUnknown,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				dataplanev1.NodeSetIPReservationReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				dataplanev1.NodeSetDNSDataReadyCondition,
				corev1.ConditionFalse,
			)
			conditions := DataplaneConditionGetter(dataplaneNodeSetName)
			message := &conditions.Get(dataplanev1.NodeSetDNSDataReadyCondition).Message
			Expect(*message).Should(Equal(dataplanev1.NodeSetDNSDataMultipleDNSMasqErrorMessage))

		})
	})

	When("TLS is enabled", func() {
		tlsEnabled := true
		When("A Dataplane resource is created with PreProvisioned nodes, no deployment", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				SimulateDNSMasqComplete(dnsMasqName)
				DeferCleanup(th.DeleteInstance,
					CreateDataplaneNodeSet(dataplaneNodeSetName,
						DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("should have the Spec fields initialized", func() {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				ansibleOpts := dataplanev1.AnsibleOpts{
					AnsibleUser: "cloud-admin",
					AnsibleHost: "",
					AnsiblePort: 0,
					AnsibleVars: nil,
				}
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret).Should(
					Equal("dataplane-ansible-ssh-private-key-secret"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.ManagementNetwork).Should(
					Equal("ctlplane"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Ansible).Should(Equal(ansibleOpts))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Networks[0].Name).Should(
					BeEquivalentTo("networkinternal"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Networks[1].Name).Should(
					BeEquivalentTo("ctlplane"))
				Expect(dataplaneNodeSetInstance.Spec.PreProvisioned).Should(BeTrue())
				Expect(dataplaneNodeSetInstance.Spec.TLSEnabled).Should(Equal(tlsEnabled))
				nodes := map[string]dataplanev1.NodeSection{
					dataplaneNodeName.Name: {
						HostName: dataplaneNodeName.Name,
					},
				}
				Expect(dataplaneNodeSetInstance.Spec.Nodes).Should(Equal(nodes))
				services := []string{
					"redhat",
					"download-cache",
					"bootstrap",
					"configure-network",
					"validate-network",
					"install-os",
					"configure-os",
					"ssh-known-hosts",
					"run-os",
					"reboot-os",
					"install-certs",
					"ovn",
					"neutron-metadata",
					"libvirt",
					"nova",
					"telemetry"}

				Expect(dataplaneNodeSetInstance.Spec.Services).Should(Equal(services))
			})

			It("should have input not ready and unknown Conditions initialized", func() {
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.SetupReadyCondition,
					corev1.ConditionFalse,
				)
			})

			It("Should not have created a Secret", func() {
				th.AssertSecretDoesNotExist(dataplaneSecretName)
			})
		})

		When("A Dataplane resource is created with PreProvisioned nodes, no deployment and global service", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				nodeSetSpec["services"] = []string{
					"redhat",
					"download-cache",
					"bootstrap",
					"configure-network",
					"validate-network",
					"install-os",
					"configure-os",
					"run-os",
					"reboot-os",
					"install-certs",
					"ovn",
					"neutron-metadata",
					"libvirt",
					"nova",
					"telemetry",
					"global-service"}

				CreateDataplaneService(dataplaneGlobalServiceName, true)
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				SimulateDNSMasqComplete(dnsMasqName)
				DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("should have the Spec fields initialized", func() {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				ansibleOpts := dataplanev1.AnsibleOpts{
					AnsibleUser: "cloud-admin",
					AnsibleHost: "",
					AnsiblePort: 0,
					AnsibleVars: nil,
				}
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret).Should(
					Equal("dataplane-ansible-ssh-private-key-secret"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.ManagementNetwork).Should(
					Equal("ctlplane"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Ansible).Should(Equal(ansibleOpts))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Networks[0].Name).Should(
					BeEquivalentTo("networkinternal"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Networks[1].Name).Should(
					BeEquivalentTo("ctlplane"))
				Expect(dataplaneNodeSetInstance.Spec.PreProvisioned).Should(BeTrue())
				Expect(dataplaneNodeSetInstance.Spec.TLSEnabled).Should(Equal(tlsEnabled))
				nodes := map[string]dataplanev1.NodeSection{
					dataplaneNodeName.Name: {
						HostName: dataplaneNodeName.Name,
					},
				}
				Expect(dataplaneNodeSetInstance.Spec.Nodes).Should(Equal(nodes))
				services := []string{
					"redhat",
					"download-cache",
					"bootstrap",
					"configure-network",
					"validate-network",
					"install-os",
					"configure-os",
					"run-os",
					"reboot-os",
					"install-certs",
					"ovn",
					"neutron-metadata",
					"libvirt",
					"nova",
					"telemetry",
					"global-service"}

				Expect(dataplaneNodeSetInstance.Spec.Services).Should(Equal(services))
			})

			It("should have input not ready and unknown Conditions initialized", func() {
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.SetupReadyCondition,
					corev1.ConditionFalse,
				)
			})

			It("Should not have created a Secret", func() {
				th.AssertSecretDoesNotExist(dataplaneSecretName)
			})

			It("Should have service called 'global-service'", func() {
				service := GetService(dataplaneGlobalServiceName)
				Expect(service.Spec.DeployOnAllNodeSets).Should(BeTrue())
			})
		})

		When("A Dataplane resorce is created without PreProvisioned nodes and ordered deployment", func() {
			BeforeEach(func() {
				spec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				spec["metadata"] = map[string]interface{}{"ansiblesshprivatekeysecret": ""}
				spec["preProvisioned"] = false
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, spec))
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("should have the Spec fields initialized", func() {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				Expect(dataplaneNodeSetInstance.Spec.PreProvisioned).Should(BeFalse())
			})

			It("should have ReadyCondition, InputReadyCondition and SetupReadyCondition set to false, and DeploymentReadyCondition set to Unknown", func() {
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.SetupReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.DeploymentReadyCondition,
					corev1.ConditionUnknown,
				)
			})

			It("Should not have created a Secret", func() {
				th.AssertSecretDoesNotExist(dataplaneSecretName)
			})
		})

		When("A Dataplane resorce is created without PreProvisioned nodes but is marked as PreProvisioned, with ordered deployment", func() {
			BeforeEach(func() {
				spec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				spec["metadata"] = map[string]interface{}{"ansiblesshprivatekeysecret": ""}
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, spec))
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("should have the Spec fields initialized", func() {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				Expect(dataplaneNodeSetInstance.Spec.PreProvisioned).Should(BeTrue())
			})

			It("should have ReadyCondition, InputReadCondition and SetupReadyCondition set to false, and DeploymentReadyCondition set to unknown", func() {
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.SetupReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.DeploymentReadyCondition,
					corev1.ConditionUnknown,
				)
			})

			It("Should not have created a Secret", func() {
				th.AssertSecretDoesNotExist(dataplaneSecretName)
			})
		})

		When("A ssh secret is created", func() {

			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have created a Secret", func() {
				secret := th.GetSecret(dataplaneSecretName)
				Expect(secret.Data["inventory"]).Should(
					ContainSubstring("edpm-compute-nodeset"))
			})
			It("Should set Input and Setup ready", func() {

				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionTrue,
				)
			})
		})

		When("No default service image is provided", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have default service values provided", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcImage := range defaultEdpmServiceList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(svcImage))
				}
			})
		})

		When("A user provides a custom service image", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, CustomServiceImageSpec()))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have the user defined image in the inventory", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcAnsibleVar := range DefaultEdpmServiceAnsibleVarList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(fmt.Sprintf("%s.%s", svcAnsibleVar, CustomEdpmServiceDomainTag)))
				}
			})
		})

		When("No default service image is provided", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have default service values provided", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcAnsibleVar := range DefaultEdpmServiceAnsibleVarList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(svcAnsibleVar))
				}
			})
		})

		When("A user provides a custom service image", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, CustomServiceImageSpec()))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have the user defined image in the inventory", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcAnsibleVar := range DefaultEdpmServiceAnsibleVarList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(fmt.Sprintf("%s.%s", svcAnsibleVar, CustomEdpmServiceDomainTag)))
				}
			})
		})

		When("The nodeTemplate contains a ansibleUser but the individual node does not", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should not have set the node specific ansible_user variable", func() {
				secret := th.GetSecret(dataplaneSecretName)
				secretData := secret.Data["inventory"]

				var inv AnsibleInventory
				err := yaml.Unmarshal(secretData, &inv)
				if err != nil {
					fmt.Printf("Error: %v", err)
				}
				Expect(inv.EdpmComputeNodeset.Vars.AnsibleUser).Should(Equal("cloud-user"))
				Expect(inv.EdpmComputeNodeset.Hosts.Node.AnsibleUser).Should(BeEmpty())
			})
		})

		When("The individual node has a AnsibleUser override", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				nodeOverrideSpec := map[string]interface{}{
					"hostName": dataplaneNodeName.Name,
					"networks": []map[string]interface{}{
						{
							"name":       "networkinternal",
							"subnetName": "subnet1",
						},
						{
							"name":       "ctlplane",
							"subnetName": "subnet1",
						},
					},
					"ansible": map[string]interface{}{
						"ansibleUser": "test-user",
					},
				}

				nodeTemplateOverrideSpec := map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				}

				nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				nodeSetSpec["nodes"] = map[string]interface{}{dataplaneNodeName.Name: nodeOverrideSpec}
				nodeSetSpec["nodeTemplate"] = nodeTemplateOverrideSpec

				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have a node specific override that is different to the group", func() {
				secret := th.GetSecret(dataplaneSecretName)
				secretData := secret.Data["inventory"]

				var inv AnsibleInventory
				err := yaml.Unmarshal(secretData, &inv)
				if err != nil {
					fmt.Printf("Error: %v", err)
				}
				Expect(inv.EdpmComputeNodeset.Hosts.Node.AnsibleUser).Should(Equal("test-user"))
				Expect(inv.EdpmComputeNodeset.Vars.AnsibleUser).Should(Equal("cloud-user"))
			})
		})

		When("A nodeSet is created with IPAM", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should set the ctlplane_ip variable in the Ansible inventory secret", func() {
				Eventually(func() string {
					secret := th.GetSecret(dataplaneSecretName)
					return getCtlPlaneIP(&secret)
				}).Should(Equal("172.20.12.76"))
			})
		})

		When("A DataPlaneNodeSet is created with NoNodes and a OpenStackDataPlaneDeployment is created", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should reach Input and Setup Ready completion", func() {
				var conditionList = []condition.Type{
					condition.InputReadyCondition,
					dataplanev1.SetupReadyCondition,
				}
				for _, cond := range conditionList {
					th.ExpectCondition(
						dataplaneNodeSetName,
						ConditionGetterFunc(DataplaneConditionGetter),
						cond,
						corev1.ConditionTrue,
					)
				}
			})
		})
	})
	When("TLS is not enabled explicitly its enabled by default", func() {
		tlsEnabled := true
		When("A Dataplane resorce is created with PreProvisioned nodes, no deployment", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)

			})
			It("should have the Spec fields initialized", func() {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				ansibleOpts := dataplanev1.AnsibleOpts{
					AnsibleUser: "cloud-admin",
					AnsibleHost: "",
					AnsiblePort: 0,
					AnsibleVars: nil,
				}
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.AnsibleSSHPrivateKeySecret).Should(
					Equal("dataplane-ansible-ssh-private-key-secret"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.ManagementNetwork).Should(
					Equal("ctlplane"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Ansible).Should(Equal(ansibleOpts))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Networks[0].Name).Should(
					BeEquivalentTo("networkinternal"))
				Expect(dataplaneNodeSetInstance.Spec.NodeTemplate.Networks[1].Name).Should(
					BeEquivalentTo("ctlplane"))
				Expect(dataplaneNodeSetInstance.Spec.PreProvisioned).Should(BeTrue())
				Expect(dataplaneNodeSetInstance.Spec.TLSEnabled).Should(Equal(tlsEnabled))
				nodes := map[string]dataplanev1.NodeSection{
					dataplaneNodeName.Name: {
						HostName: dataplaneNodeName.Name,
					},
				}
				Expect(dataplaneNodeSetInstance.Spec.Nodes).Should(Equal(nodes))
				services := []string{
					"redhat",
					"download-cache",
					"bootstrap",
					"configure-network",
					"validate-network",
					"install-os",
					"configure-os",
					"ssh-known-hosts",
					"run-os",
					"reboot-os",
					"install-certs",
					"ovn",
					"neutron-metadata",
					"libvirt",
					"nova",
					"telemetry"}

				Expect(dataplaneNodeSetInstance.Spec.Services).Should(Equal(services))
			})
			It("should have input not ready and unknown Conditions initialized", func() {
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.SetupReadyCondition,
					corev1.ConditionFalse,
				)
			})

			It("Should not have created a Secret", func() {
				th.AssertSecretDoesNotExist(dataplaneSecretName)
			})
		})

		When("A Dataplane resorce is created without PreProvisioned nodes and ordered deployment", func() {
			BeforeEach(func() {
				spec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				spec["metadata"] = map[string]interface{}{"ansiblesshprivatekeysecret": ""}
				spec["preProvisioned"] = false
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, spec))
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("should have the Spec fields initialized", func() {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				Expect(dataplaneNodeSetInstance.Spec.PreProvisioned).Should(BeFalse())
			})

			It("should have ReadyCondition, InputReadyCondition and SetupReadyCondition set to false, and DeploymentReadyCondition set to Unknown", func() {
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.SetupReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.DeploymentReadyCondition,
					corev1.ConditionUnknown,
				)
			})

			It("Should not have created a Secret", func() {
				th.AssertSecretDoesNotExist(dataplaneSecretName)
			})
		})

		When("A Dataplane resorce is created without PreProvisioned nodes but is marked as PreProvisioned, with ordered deployment", func() {
			BeforeEach(func() {
				spec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				spec["metadata"] = map[string]interface{}{"ansiblesshprivatekeysecret": ""}
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, spec))
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("should have the Spec fields initialized", func() {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				Expect(dataplaneNodeSetInstance.Spec.PreProvisioned).Should(BeTrue())
			})

			It("should have ReadyCondition, InputReadCondition and SetupReadyCondition set to false, and DeploymentReadyCondition set to unknown", func() {
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.ReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					dataplanev1.SetupReadyCondition,
					corev1.ConditionFalse,
				)
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.DeploymentReadyCondition,
					corev1.ConditionUnknown,
				)
			})

			It("Should not have created a Secret", func() {
				th.AssertSecretDoesNotExist(dataplaneSecretName)
			})
		})

		When("A ssh secret is created", func() {

			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have created a Secret", func() {
				secret := th.GetSecret(dataplaneSecretName)
				Expect(secret.Data["inventory"]).Should(
					ContainSubstring("edpm-compute-nodeset"))
			})
			It("Should set Input and Setup ready", func() {

				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.InputReadyCondition,
					corev1.ConditionTrue,
				)
			})
		})

		When("No default service image is provided", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have default service values provided", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcImage := range defaultEdpmServiceList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(svcImage))
				}
			})
		})

		When("A user provides a custom service image", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, CustomServiceImageSpec()))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have the user defined image in the inventory", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcAnsibleVar := range DefaultEdpmServiceAnsibleVarList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(fmt.Sprintf("%s.%s", svcAnsibleVar, CustomEdpmServiceDomainTag)))
				}
			})
		})

		When("No default service image is provided", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have default service values provided", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcAnsibleVar := range DefaultEdpmServiceAnsibleVarList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(svcAnsibleVar))
				}
			})
		})

		When("A user provides a custom service image", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, CustomServiceImageSpec()))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have the user defined image in the inventory", func() {
				secret := th.GetSecret(dataplaneSecretName)
				for _, svcAnsibleVar := range DefaultEdpmServiceAnsibleVarList {
					Expect(secret.Data["inventory"]).Should(
						ContainSubstring(fmt.Sprintf("%s.%s", svcAnsibleVar, CustomEdpmServiceDomainTag)))
				}
			})
		})

		When("The nodeTemplate contains a ansibleUser but the individual node does not", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should not have set the node specific ansible_user variable", func() {
				secret := th.GetSecret(dataplaneSecretName)
				secretData := secret.Data["inventory"]

				var inv AnsibleInventory
				err := yaml.Unmarshal(secretData, &inv)
				if err != nil {
					fmt.Printf("Error: %v", err)
				}
				Expect(inv.EdpmComputeNodeset.Vars.AnsibleUser).Should(Equal("cloud-user"))
				Expect(inv.EdpmComputeNodeset.Hosts.Node.AnsibleUser).Should(BeEmpty())
			})
		})

		When("The individual node has a AnsibleUser override", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				nodeOverrideSpec := map[string]interface{}{
					"hostName": dataplaneNodeName.Name,
					"networks": []map[string]interface{}{
						{
							"name":       "networkinternal",
							"subnetName": "subnet1",
						},
						{
							"name":       "ctlplane",
							"subnetName": "subnet1",
						},
					},
					"ansible": map[string]interface{}{
						"ansibleUser": "test-user",
					},
				}

				nodeTemplateOverrideSpec := map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				}

				nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				nodeSetSpec["nodes"] = map[string]interface{}{dataplaneNodeName.Name: nodeOverrideSpec}
				nodeSetSpec["nodeTemplate"] = nodeTemplateOverrideSpec

				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should have a node specific override that is different to the group", func() {
				secret := th.GetSecret(dataplaneSecretName)
				secretData := secret.Data["inventory"]

				var inv AnsibleInventory
				err := yaml.Unmarshal(secretData, &inv)
				if err != nil {
					fmt.Printf("Error: %v", err)
				}
				Expect(inv.EdpmComputeNodeset.Hosts.Node.AnsibleUser).Should(Equal("test-user"))
				Expect(inv.EdpmComputeNodeset.Vars.AnsibleUser).Should(Equal("cloud-user"))
			})
		})

		When("A nodeSet is created with IPAM", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should set the ctlplane_ip variable in the Ansible inventory secret", func() {
				Eventually(func() string {
					secret := th.GetSecret(dataplaneSecretName)
					return getCtlPlaneIP(&secret)
				}).Should(Equal("172.20.12.76"))
			})
		})

		When("A DataPlaneNodeSet is created with NoNodes and a OpenStackDataPlaneDeployment is created", func() {
			BeforeEach(func() {
				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(tlsEnabled)))
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})
			It("Should reach Input and Setup Ready completion", func() {
				var conditionList = []condition.Type{
					condition.InputReadyCondition,
					dataplanev1.SetupReadyCondition,
				}
				for _, cond := range conditionList {
					th.ExpectCondition(
						dataplaneNodeSetName,
						ConditionGetterFunc(DataplaneConditionGetter),
						cond,
						corev1.ConditionTrue,
					)
				}
			})
		})
	})

	When("A user changes spec field that would require a new Ansible execution", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNodeSetSpec(dataplaneNodeSetName.Name)
			nodeSetSpec["nodeTemplate"] = dataplanev1.NodeTemplate{
				Ansible: dataplanev1.AnsibleOpts{
					AnsibleVars: map[string]json.RawMessage{
						"edpm_network_config_hide_sensitive_logs": json.RawMessage([]byte(`"false"`)),
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)
		})

		It("Should change the ConfigHash", func() {
			Eventually(func(_ Gomega) error {
				instance := GetDataplaneNodeSet(dataplaneNodeSetName)
				dataplaneConfigHash = instance.Status.ConfigHash
				instance.Spec.NodeTemplate.Ansible.AnsibleVars = map[string]json.RawMessage{
					"edpm_network_config_hide_sensitive_logs": json.RawMessage([]byte(`"true"`)),
				}
				return th.K8sClient.Update(th.Ctx, instance)
			}).Should(Succeed())
			Eventually(func(_ Gomega) bool {
				updatedInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				return dataplaneConfigHash != updatedInstance.Status.ConfigHash
			}).Should(BeTrue())
		})
	})

	When("A DataPlaneNodeSet is created with NoNodes and a MinorUpdate OpenStackDataPlaneDeployment is created", func() {
		BeforeEach(func() {

			dataplanev1.SetupDefaults()
			updateServiceSpec := map[string]interface{}{
				"playbook": "osp.edpm.update",
			}
			CreateDataPlaneServiceFromSpec(dataplaneUpdateServiceName, updateServiceSpec)
			DeferCleanup(th.DeleteService, dataplaneUpdateServiceName)
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(false)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, MinorUpdateDataPlaneDeploymentSpec()))
			openstackVersionName := types.NamespacedName{
				Name:      "openstackversion",
				Namespace: namespace,
			}
			err := os.Setenv("OPENSTACK_RELEASE_VERSION", "0.0.1")
			Expect(err).NotTo(HaveOccurred())
			openstackv1.SetupVersionDefaults()
			DeferCleanup(th.DeleteInstance, CreateOpenStackVersion(openstackVersionName))

			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			SimulateDNSMasqComplete(dnsMasqName)
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)

			Eventually(func(g Gomega) {
				// Make an AnsibleEE name for each service
				ansibleeeName := types.NamespacedName{
					Name:      "update-edpm-deployment-edpm-compute-nodeset",
					Namespace: namespace,
				}
				ansibleEE := GetAnsibleee(ansibleeeName)
				ansibleEE.Status.Succeeded = 1
				g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
			}, th.Timeout, th.Interval).Should(Succeed())

		})
		It("NodeSet.Status.DeployedVersion should be set to latest version", Label("update"), func() {
			Eventually(func() string {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				return dataplaneNodeSetInstance.Status.DeployedVersion
			}).Should(Equal("0.0.1"))
		})
	})

	When("A DataPlaneNodeSet is created with NoNodes and a MinorUpdateServices OpenStackDataPlaneDeployment is created", func() {
		BeforeEach(func() {

			dataplanev1.SetupDefaults()
			updateServiceSpec := map[string]interface{}{
				"playbook": "osp.edpm.update_services",
			}
			CreateDataPlaneServiceFromSpec(newDataplaneUpdateServiceName, updateServiceSpec)
			DeferCleanup(th.DeleteService, newDataplaneUpdateServiceName)
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNoNodeSetSpec(false)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, MinorUpdateServicesDataPlaneDeploymentSpec()))
			openstackVersionName := types.NamespacedName{
				Name:      "openstackversion",
				Namespace: namespace,
			}
			err := os.Setenv("OPENSTACK_RELEASE_VERSION", "0.0.1")
			Expect(err).NotTo(HaveOccurred())
			openstackv1.SetupVersionDefaults()
			DeferCleanup(th.DeleteInstance, CreateOpenStackVersion(openstackVersionName))

			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			SimulateDNSMasqComplete(dnsMasqName)
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)

			Eventually(func(g Gomega) {
				// Make an AnsibleEE name for each service
				ansibleeeName := types.NamespacedName{
					Name:      "update-services-edpm-deployment-edpm-compute-nodeset",
					Namespace: namespace,
				}
				ansibleEE := GetAnsibleee(ansibleeeName)
				ansibleEE.Status.Succeeded = 1
				g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
			}, th.Timeout, th.Interval).Should(Succeed())

		})
		It("NodeSet.Status.DeployedVersion should be set to latest version", Label("update"), func() {
			Eventually(func() string {
				dataplaneNodeSetInstance := GetDataplaneNodeSet(dataplaneNodeSetName)
				return dataplaneNodeSetInstance.Status.DeployedVersion
			}).Should(Equal("0.0.1"))
		})
	})

	When("A NodeSet and Deployment are created", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
			nodeSetSpec["preProvisioned"] = true
			nodeSetSpec["services"] = []string{"bootstrap"}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			SimulateDNSMasqComplete(dnsMasqName)
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)

		})
		It("Should have SSH and Inventory volume mounts", func() {
			Eventually(func(g Gomega) {
				const bootstrapName string = "bootstrap"
				// Make an AnsibleEE name for each service
				ansibleeeName := types.NamespacedName{
					Name: fmt.Sprintf("%s-%s-%s",
						bootstrapName, dataplaneDeploymentName.Name, dataplaneNodeSetName.Name),
					Namespace: namespace,
				}
				ansibleEE := GetAnsibleee(ansibleeeName)
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes).To(HaveLen(3))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[0].Name).To(Equal("bootstrap-combined-ca-bundle"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[1].Name).To(Equal("ssh-key"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[2].Name).To(Equal("inventory"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[0].VolumeSource.Secret.SecretName).To(Equal("combined-ca-bundle"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[1].VolumeSource.Secret.SecretName).To(Equal("dataplane-ansible-ssh-private-key-secret"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[1].VolumeSource.Secret.Items[0].Path).To(Equal("ssh_key"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[1].VolumeSource.Secret.Items[0].Key).To(Equal("ssh-privatekey"))

			}, th.Timeout, th.Interval).Should(Succeed())
		})
	})

	When("A NodeSet and Deployment are created with extraMounts with pvc template", func() {
		BeforeEach(func() {
			edpmVolClaimTemplate := corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "edpm-ansible",
				ReadOnly:  true,
			}
			nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
			nodeSetSpec["preProvisioned"] = true
			nodeSetSpec["services"] = []string{"bootstrap"}
			nodeSetSpec["nodeTemplate"] = map[string]interface{}{
				"extraMounts": []storage.VolMounts{
					{
						Mounts: []corev1.VolumeMount{
							{
								Name:      "edpm-ansible",
								MountPath: "/usr/share/ansible/collections/ansible_collections/osp/edpm",
							},
						},
						Volumes: []storage.Volume{
							{
								Name: "edpm-ansible",
								VolumeSource: storage.VolumeSource{
									PersistentVolumeClaim: &edpmVolClaimTemplate,
								},
							},
						},
					},
				},
				"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
				"ansible": map[string]interface{}{
					"ansibleUser": "cloud-user",
				},
			}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			SimulateDNSMasqComplete(dnsMasqName)
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)

		})
		It("Should have ExtraMounts, SSH and Inventory volume mounts", func() {
			Eventually(func(g Gomega) {
				const bootstrapName string = "bootstrap"
				// Make an AnsibleEE name for each service
				ansibleeeName := types.NamespacedName{
					Name: fmt.Sprintf("%s-%s-%s",
						bootstrapName, dataplaneDeploymentName.Name, dataplaneNodeSetName.Name),
					Namespace: namespace,
				}
				ansibleEE := GetAnsibleee(ansibleeeName)
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes).To(HaveLen(4))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[0].Name).To(Equal("edpm-ansible"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[1].Name).To(Equal("bootstrap-combined-ca-bundle"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[2].Name).To(Equal("ssh-key"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[3].Name).To(Equal("inventory"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[0].VolumeSource.PersistentVolumeClaim.ClaimName).To(Equal("edpm-ansible"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[1].VolumeSource.Secret.SecretName).To(Equal("combined-ca-bundle"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[2].VolumeSource.Secret.SecretName).To(Equal("dataplane-ansible-ssh-private-key-secret"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[2].VolumeSource.Secret.Items[0].Path).To(Equal("ssh_key"))
				g.Expect(ansibleEE.Spec.Template.Spec.Volumes[2].VolumeSource.Secret.Items[0].Key).To(Equal("ssh-privatekey"))

			}, th.Timeout, th.Interval).Should(Succeed())
		})
	})

	When("A ImageContentSourcePolicy exists in the cluster", func() {
		BeforeEach(func() {
			nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
			nodeSetSpec["preProvisioned"] = true
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateICSP(dataplaneNodeSetName, DefaultICSPSpec())
			CreateMachineConfig()
			SimulateDNSMasqComplete(dnsMasqName)
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)

		})
		It("Should set edpm_podman_disconnected_ocp variable", func() {
			secret := th.GetSecret(dataplaneSecretName)
			Expect(secret.Data["inventory"]).Should(
				ContainSubstring("edpm_podman_disconnected_ocp"))
			Expect(secret.Data["inventory"]).Should(
				ContainSubstring("edpm_podman_registries_conf"))
		})
	})

	When("Testing deployment filtering logic", func() {
		var secondDeploymentName types.NamespacedName

		BeforeEach(func() {
			secondDeploymentName = types.NamespacedName{
				Name:      "edpm-deployment-2",
				Namespace: namespace,
			}
		})

		When("Multiple deployments exist with ServicesOverride", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				nodeSetSpec["services"] = []string{"bootstrap"}

				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))

				// Create deployment without ServicesOverride
				normalDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, normalDeploymentSpec))

				// Create deployment with ServicesOverride
				overrideDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				overrideDeploymentSpec["servicesOverride"] = []string{"bootstrap", "configure-network"}
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(secondDeploymentName, overrideDeploymentSpec))

				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})

			It("Should show all deployments in status with ServicesOverride latest", func() {
				// Complete the normal deployment
				Eventually(func(g Gomega) {
					ansibleeeName := types.NamespacedName{
						Name:      "bootstrap-" + dataplaneDeploymentName.Name + "-" + dataplaneNodeSetName.Name,
						Namespace: namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())

				// Both deployments should be in status (for visibility)
				// All completed deployments are processed, with latest deployment determining final state
				Eventually(func(g Gomega) {
					instance := GetDataplaneNodeSet(dataplaneNodeSetName)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(dataplaneDeploymentName.Name))
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(secondDeploymentName.Name))
				}, th.Timeout, th.Interval).Should(Succeed())
			})
		})

		When("Latest deployment has ServicesOverride", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				nodeSetSpec["services"] = []string{"bootstrap"}

				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))

				// Create first deployment (normal)
				firstDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, firstDeploymentSpec))

				// Create second deployment with ServicesOverride (latest)
				overrideDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				overrideDeploymentSpec["servicesOverride"] = []string{"bootstrap"}
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(secondDeploymentName, overrideDeploymentSpec))

				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})

			It("Should show all deployments in status and process all completed deployments", func() {
				// Complete the first deployment
				Eventually(func(g Gomega) {
					ansibleeeName := types.NamespacedName{
						Name:      "bootstrap-" + dataplaneDeploymentName.Name + "-" + dataplaneNodeSetName.Name,
						Namespace: namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())

				// Both deployments should be included (for visibility)
				// All completed deployments are processed, with latest determining final state
				Eventually(func(g Gomega) {
					instance := GetDataplaneNodeSet(dataplaneNodeSetName)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(dataplaneDeploymentName.Name))
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(secondDeploymentName.Name))
				}, th.Timeout, th.Interval).Should(Succeed())
			})
		})

		When("Running deployments exist with completed deployment", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				nodeSetSpec["services"] = []string{"bootstrap"}

				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))

				// Create first deployment (will complete)
				firstDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, firstDeploymentSpec))

				// Create second deployment (will be running)
				secondDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(secondDeploymentName, secondDeploymentSpec))

				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})

			It("Should show all deployments in status with running deployments", func() {
				// Complete the first deployment
				Eventually(func(g Gomega) {
					ansibleeeName := types.NamespacedName{
						Name:      "bootstrap-" + dataplaneDeploymentName.Name + "-" + dataplaneNodeSetName.Name,
						Namespace: namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())

				// Leave second deployment running (don't complete it)
				// Both deployments should be in status (for visibility)
				// First deployment is processed (completed), second affects final state (running and latest)
				Eventually(func(g Gomega) {
					instance := GetDataplaneNodeSet(dataplaneNodeSetName)
					// Should have first deployment (completed, processed)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(dataplaneDeploymentName.Name))
					// Should have second deployment (running and latest)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(secondDeploymentName.Name))
				}, th.Timeout, th.Interval).Should(Succeed())
			})
		})

		When("Multiple deployments exist with different error states", func() {
			BeforeEach(func() {
				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				nodeSetSpec["services"] = []string{"bootstrap"}

				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))

				// Create first deployment (will be older)
				firstDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, firstDeploymentSpec))

				// Create second deployment (will be newer)
				secondDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(secondDeploymentName, secondDeploymentSpec))

				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(dataplaneNodeSetName)
			})

			It("Should show all deployments in status but only latest error affects state", func() {
				// Simulate first deployment (older) failure
				Eventually(func(g Gomega) {
					ansibleeeName := types.NamespacedName{
						Name:      "bootstrap-" + dataplaneDeploymentName.Name + "-" + dataplaneNodeSetName.Name,
						Namespace: namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Failed = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())

				// Simulate second deployment (newer) failure
				Eventually(func(g Gomega) {
					ansibleeeName := types.NamespacedName{
						Name:      "bootstrap-" + secondDeploymentName.Name + "-" + dataplaneNodeSetName.Name,
						Namespace: namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Failed = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())

				// Both error deployments should be in status (for visibility)
				// First failed deployment is skipped (not latest), second error affects final state (latest)
				Eventually(func(g Gomega) {
					instance := GetDataplaneNodeSet(dataplaneNodeSetName)
					// Should have first deployment (for visibility only)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(dataplaneDeploymentName.Name))
					// Should have second deployment (latest - affects overall state)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(secondDeploymentName.Name))
				}, th.Timeout, th.Interval).Should(Succeed())

				// The overall deployment condition should be false due to the latest error
				th.ExpectCondition(
					dataplaneNodeSetName,
					ConditionGetterFunc(DataplaneConditionGetter),
					condition.DeploymentReadyCondition,
					corev1.ConditionFalse,
				)
			})
		})

		When("Failed deployment followed by completed deployment", func() {
			var testNodeSetName types.NamespacedName

			BeforeEach(func() {
				// Use unique nodeset name for this test
				testNodeSetName = types.NamespacedName{
					Name:      "edpm-compute-nodeset-failsuccess",
					Namespace: namespace,
				}

				nodeSetSpec := DefaultDataPlaneNodeSetSpec("edpm-compute")
				nodeSetSpec["preProvisioned"] = true
				nodeSetSpec["services"] = []string{"bootstrap"}

				DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(testNodeSetName, nodeSetSpec))

				// Create first deployment (will fail)
				firstDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				firstDeploymentSpec["nodeSets"] = []string{testNodeSetName.Name}
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, firstDeploymentSpec))

				// Create second deployment (will complete successfully)
				secondDeploymentSpec := DefaultDataPlaneDeploymentSpec()
				secondDeploymentSpec["nodeSets"] = []string{testNodeSetName.Name}
				DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(secondDeploymentName, secondDeploymentSpec))

				CreateSSHSecret(dataplaneSSHSecretName)
				CreateCABundleSecret(caBundleSecretName)
				SimulateDNSMasqComplete(dnsMasqName)
				SimulateIPSetComplete(dataplaneNodeName)
				SimulateDNSDataComplete(testNodeSetName)
			})

			It("Should show all deployments in status and process completed deployment", func() {
				// Fail the first deployment
				Eventually(func(g Gomega) {
					ansibleeeName := types.NamespacedName{
						Name:      "bootstrap-" + dataplaneDeploymentName.Name + "-" + testNodeSetName.Name,
						Namespace: namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Failed = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())

				// Complete the second deployment
				Eventually(func(g Gomega) {
					ansibleeeName := types.NamespacedName{
						Name:      "bootstrap-" + secondDeploymentName.Name + "-" + testNodeSetName.Name,
						Namespace: namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())

				// Wait for deployment controller to populate hashes
				Eventually(func(g Gomega) {
					deployment := GetDataplaneDeployment(secondDeploymentName)
					g.Expect(deployment.Status.NodeSetHashes).Should(HaveKey(testNodeSetName.Name))
				}, th.Timeout, th.Interval).Should(Succeed())

				// Both deployments should be in status (for visibility)
				// First failed deployment is skipped (not latest), second completed deployment is processed and determines final state
				Eventually(func(g Gomega) {
					instance := GetDataplaneNodeSet(testNodeSetName)
					// Should have first deployment (for visibility only)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(dataplaneDeploymentName.Name))
					// Should have second deployment (completed and latest - affects overall state)
					g.Expect(instance.Status.DeploymentStatuses).Should(HaveKey(secondDeploymentName.Name))
				}, th.Timeout, th.Interval).Should(Succeed())

				// The overall deployment condition should be true from the successful deployment
				Eventually(func(_ Gomega) {
					th.ExpectCondition(
						testNodeSetName,
						ConditionGetterFunc(DataplaneConditionGetter),
						condition.DeploymentReadyCondition,
						corev1.ConditionTrue,
					)
				}, th.Timeout, th.Interval).Should(Succeed())
			})
		})
	})
})
