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
	ansibleeev1 "github.com/openstack-k8s-operators/openstack-ansibleee-operator/api/v1beta1"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"

	//revive:disable-next-line:dot-imports
	infrav1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
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
	var dataplaneNetConfigName types.NamespacedName
	var dnsMasqName types.NamespacedName
	var dataplaneNodeName types.NamespacedName
	var dataplaneDeploymentName types.NamespacedName
	var dataplaneConfigHash string
	var dataplaneGlobalServiceName types.NamespacedName
	var dataplaneUpdateServiceName types.NamespacedName

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
				emptyNodeSpec := dataplanev1.OpenStackDataPlaneNodeSetSpec{
					BaremetalSetTemplate: baremetalv1.OpenStackBaremetalSetSpec{
						BaremetalHosts:        nil,
						OSImage:               "",
						AutomatedCleaningMode: "metadata",
						ProvisionServerName:   "",
						ProvisioningInterface: "",
						CtlplaneInterface:     "",
						CtlplaneGateway:       "",
						CtlplaneNetmask:       "255.255.255.0",
						BmhNamespace:          "openshift-machine-api",
						HardwareReqs: baremetalv1.HardwareReqs{
							CPUReqs: baremetalv1.CPUReqs{
								Arch:     "",
								CountReq: baremetalv1.CPUCountReq{Count: 0, ExactMatch: false},
								MhzReq:   baremetalv1.CPUMhzReq{Mhz: 0, ExactMatch: false},
							},
							MemReqs: baremetalv1.MemReqs{
								GbReq: baremetalv1.MemGbReq{Gb: 0, ExactMatch: false},
							},
							DiskReqs: baremetalv1.DiskReqs{
								GbReq:  baremetalv1.DiskGbReq{Gb: 0, ExactMatch: false},
								SSDReq: baremetalv1.DiskSSDReq{SSD: false, ExactMatch: false},
							},
						},
						PasswordSecret:   nil,
						CloudUserName:    "",
						DomainName:       "",
						BootstrapDNS:     nil,
						DNSSearchDomains: nil,
					},
					NodeTemplate: dataplanev1.NodeTemplate{
						AnsibleSSHPrivateKeySecret: "dataplane-ansible-ssh-private-key-secret",
						ManagementNetwork:          "ctlplane",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleUser: "cloud-admin",
							AnsibleHost: "",
							AnsiblePort: 0,
							AnsibleVars: nil,
						},
						ExtraMounts: nil,
						Networks: []infrav1.IPSetNetwork{{
							Name:       "ctlplane",
							SubnetName: "subnet1",
						},
						},
					},
					Env:                nil,
					PreProvisioned:     true,
					NetworkAttachments: nil,
					SecretMaxSize:      1048576,
					TLSEnabled:         tlsEnabled,
					Nodes: map[string]dataplanev1.NodeSection{
						dataplaneNodeName.Name: {
							HostName: dataplaneNodeName.Name,
						},
					},
					Services: []string{
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
						"telemetry"},
				}
				Expect(dataplaneNodeSetInstance.Spec).Should(Equal(emptyNodeSpec))
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
				emptyNodeSpec := dataplanev1.OpenStackDataPlaneNodeSetSpec{
					BaremetalSetTemplate: baremetalv1.OpenStackBaremetalSetSpec{
						BaremetalHosts:        nil,
						OSImage:               "",
						AutomatedCleaningMode: "metadata",
						ProvisionServerName:   "",
						ProvisioningInterface: "",
						CtlplaneInterface:     "",
						CtlplaneGateway:       "",
						CtlplaneNetmask:       "255.255.255.0",
						BmhNamespace:          "openshift-machine-api",
						HardwareReqs: baremetalv1.HardwareReqs{
							CPUReqs: baremetalv1.CPUReqs{
								Arch:     "",
								CountReq: baremetalv1.CPUCountReq{Count: 0, ExactMatch: false},
								MhzReq:   baremetalv1.CPUMhzReq{Mhz: 0, ExactMatch: false},
							},
							MemReqs: baremetalv1.MemReqs{
								GbReq: baremetalv1.MemGbReq{Gb: 0, ExactMatch: false},
							},
							DiskReqs: baremetalv1.DiskReqs{
								GbReq:  baremetalv1.DiskGbReq{Gb: 0, ExactMatch: false},
								SSDReq: baremetalv1.DiskSSDReq{SSD: false, ExactMatch: false},
							},
						},
						PasswordSecret:   nil,
						CloudUserName:    "",
						DomainName:       "",
						BootstrapDNS:     nil,
						DNSSearchDomains: nil,
					},
					NodeTemplate: dataplanev1.NodeTemplate{
						AnsibleSSHPrivateKeySecret: "dataplane-ansible-ssh-private-key-secret",
						Networks: []infrav1.IPSetNetwork{{
							Name:       "ctlplane",
							SubnetName: "subnet1",
						},
						},
						ManagementNetwork: "ctlplane",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleUser: "cloud-admin",
							AnsibleHost: "",
							AnsiblePort: 0,
							AnsibleVars: nil,
						},
						ExtraMounts: nil,
					},
					Env:                nil,
					PreProvisioned:     true,
					NetworkAttachments: nil,
					SecretMaxSize:      1048576,
					TLSEnabled:         tlsEnabled,
					Nodes: map[string]dataplanev1.NodeSection{
						dataplaneNodeName.Name: {
							HostName: dataplaneNodeName.Name,
						},
					},
					Services: []string{
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
						"global-service"},
				}
				Expect(dataplaneNodeSetInstance.Spec).Should(Equal(emptyNodeSpec))
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
				nodeOverrideSpec := dataplanev1.NodeSection{
					HostName: dataplaneNodeName.Name,
					Networks: []infrav1.IPSetNetwork{{
						Name:       "ctlplane",
						SubnetName: "subnet1",
					},
					},
					Ansible: dataplanev1.AnsibleOpts{
						AnsibleUser: "test-user",
					},
				}

				nodeTemplateOverrideSpec := map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				}

				nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				nodeSetSpec["nodes"].(map[string]dataplanev1.NodeSection)[dataplaneNodeName.Name] = nodeOverrideSpec
				nodeSetSpec["nodeTemplate"] = nodeTemplateOverrideSpec

				DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
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
				emptyNodeSpec := dataplanev1.OpenStackDataPlaneNodeSetSpec{
					BaremetalSetTemplate: baremetalv1.OpenStackBaremetalSetSpec{
						BaremetalHosts:        nil,
						OSImage:               "",
						AutomatedCleaningMode: "metadata",
						ProvisionServerName:   "",
						ProvisioningInterface: "",
						CtlplaneInterface:     "",
						CtlplaneGateway:       "",
						CtlplaneNetmask:       "255.255.255.0",
						BmhNamespace:          "openshift-machine-api",
						HardwareReqs: baremetalv1.HardwareReqs{
							CPUReqs: baremetalv1.CPUReqs{
								Arch:     "",
								CountReq: baremetalv1.CPUCountReq{Count: 0, ExactMatch: false},
								MhzReq:   baremetalv1.CPUMhzReq{Mhz: 0, ExactMatch: false},
							},
							MemReqs: baremetalv1.MemReqs{
								GbReq: baremetalv1.MemGbReq{Gb: 0, ExactMatch: false},
							},
							DiskReqs: baremetalv1.DiskReqs{
								GbReq:  baremetalv1.DiskGbReq{Gb: 0, ExactMatch: false},
								SSDReq: baremetalv1.DiskSSDReq{SSD: false, ExactMatch: false},
							},
						},
						PasswordSecret:   nil,
						CloudUserName:    "",
						DomainName:       "",
						BootstrapDNS:     nil,
						DNSSearchDomains: nil,
					},
					NodeTemplate: dataplanev1.NodeTemplate{
						AnsibleSSHPrivateKeySecret: "dataplane-ansible-ssh-private-key-secret",
						Networks: []infrav1.IPSetNetwork{{
							Name:       "ctlplane",
							SubnetName: "subnet1",
						},
						},
						ManagementNetwork: "ctlplane",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleUser: "cloud-admin",
							AnsibleHost: "",
							AnsiblePort: 0,
							AnsibleVars: nil,
						},
						ExtraMounts: nil,
						UserData:    nil,
						NetworkData: nil,
					},
					Env:                nil,
					PreProvisioned:     true,
					NetworkAttachments: nil,
					SecretMaxSize:      1048576,
					TLSEnabled:         tlsEnabled,
					Nodes: map[string]dataplanev1.NodeSection{
						dataplaneNodeName.Name: {
							HostName: dataplaneNodeName.Name,
						},
					},
					Services: []string{
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
						"telemetry"},
				}
				Expect(dataplaneNodeSetInstance.Spec).Should(Equal(emptyNodeSpec))
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
				nodeOverrideSpec := dataplanev1.NodeSection{
					HostName: dataplaneNodeName.Name,
					Networks: []infrav1.IPSetNetwork{{
						Name:       "ctlplane",
						SubnetName: "subnet1",
					},
					},
					Ansible: dataplanev1.AnsibleOpts{
						AnsibleUser: "test-user",
					},
				}

				nodeTemplateOverrideSpec := map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				}

				nodeSetSpec := DefaultDataPlaneNoNodeSetSpec(tlsEnabled)
				nodeSetSpec["nodes"].(map[string]dataplanev1.NodeSection)[dataplaneNodeName.Name] = nodeOverrideSpec
				nodeSetSpec["nodeTemplate"] = nodeTemplateOverrideSpec

				DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))
				CreateSSHSecret(dataplaneSSHSecretName)
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
				ansibleEE.Status.JobStatus = ansibleeev1.JobStatusSucceeded
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

})
