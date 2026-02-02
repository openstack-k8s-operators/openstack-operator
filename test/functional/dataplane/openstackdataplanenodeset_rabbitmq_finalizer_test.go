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
package functional

import (
	"fmt"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	infrav1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Dataplane NodeSet RabbitMQ Finalizer Management", func() {

	// Helper function to create Nova cell config secret
	CreateNovaCellConfigSecret := func(cellName, username, cluster string) *corev1.Secret {
		transportURL := fmt.Sprintf("rabbit://%s:password@%s.openstack.svc:5672/", username, cluster)
		config := fmt.Sprintf("[DEFAULT]\ntransport_url = %s\n", transportURL)
		name := types.NamespacedName{
			Namespace: namespace,
			Name:      fmt.Sprintf("nova-%s-compute-config", cellName),
		}
		return th.CreateSecret(name, map[string][]byte{
			"01-nova.conf": []byte(config),
		})
	}

	// Helper function to update Nova cell config secret
	UpdateNovaCellConfigSecret := func(cellName, username, cluster string) {
		transportURL := fmt.Sprintf("rabbit://%s:password@%s.openstack.svc:5672/", username, cluster)
		config := fmt.Sprintf("[DEFAULT]\ntransport_url = %s\n", transportURL)
		name := types.NamespacedName{
			Namespace: namespace,
			Name:      fmt.Sprintf("nova-%s-compute-config", cellName),
		}
		secret := &corev1.Secret{}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, name, secret)
			g.Expect(err).NotTo(HaveOccurred())
		}, timeout, interval).Should(Succeed())

		secret.Data["01-nova.conf"] = []byte(config)
		Expect(k8sClient.Update(ctx, secret)).Should(Succeed())
	}

	// Helper function to create Neutron agent config secret
	CreateNeutronAgentConfigSecret := func(agentType, username, cluster string) *corev1.Secret {
		transportURL := fmt.Sprintf("rabbit://%s:password@%s.openstack.svc:5672/", username, cluster)
		config := fmt.Sprintf("[DEFAULT]\ntransport_url = %s\n", transportURL)
		name := types.NamespacedName{
			Namespace: namespace,
			Name:      fmt.Sprintf("neutron-%s-agent-neutron-config", agentType),
		}
		configKey := fmt.Sprintf("10-neutron-%s.conf", agentType)
		return th.CreateSecret(name, map[string][]byte{
			configKey: []byte(config),
		})
	}

	// Helper function to create Ironic Neutron Agent config secret
	CreateIronicNeutronAgentConfigSecret := func(username, cluster string) *corev1.Secret {
		transportURL := fmt.Sprintf("rabbit://%s:password@%s.openstack.svc:5672/", username, cluster)
		config := fmt.Sprintf("[DEFAULT]\ntransport_url = %s\n", transportURL)
		name := types.NamespacedName{
			Namespace: namespace,
			Name:      "ironic-neutron-agent-config-data",
		}
		return th.CreateSecret(name, map[string][]byte{
			"01-ironic_neutron_agent.conf": []byte(config),
		})
	}

	// Helper function to create RabbitMQUser
	CreateRabbitMQUser := func(username string) *rabbitmqv1.RabbitMQUser {
		user := &rabbitmqv1.RabbitMQUser{
			ObjectMeta: metav1.ObjectMeta{
				Name:      username,
				Namespace: namespace,
			},
			Spec: rabbitmqv1.RabbitMQUserSpec{
				Username: username,
			},
		}
		Expect(k8sClient.Create(ctx, user)).Should(Succeed())
		// Set status username to match spec
		user.Status.Username = username
		Expect(k8sClient.Status().Update(ctx, user)).Should(Succeed())
		return user
	}

	// Helper function to get RabbitMQUser
	GetRabbitMQUser := func(username string) *rabbitmqv1.RabbitMQUser {
		user := &rabbitmqv1.RabbitMQUser{}
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      username,
				Namespace: namespace,
			}, user)
			g.Expect(err).NotTo(HaveOccurred())
		}, timeout, interval).Should(Succeed())
		return user
	}

	// Helper to check if finalizer exists on RabbitMQ user
	HasFinalizer := func(username, finalizer string) bool {
		user := GetRabbitMQUser(username)
		for _, f := range user.Finalizers {
			if f == finalizer {
				return true
			}
		}
		return false
	}

	// Helper to simulate deployment completion
	SimulateDeploymentComplete := func(deploymentName types.NamespacedName, nodesetName string, ansibleLimit []string) {
		// First, complete the AnsibleEE jobs for each service
		deployment := GetDataplaneDeployment(deploymentName)
		nodeset := GetDataplaneNodeSet(types.NamespacedName{
			Namespace: deploymentName.Namespace,
			Name:      nodesetName,
		})

		// Get list of services from deployment or nodeset
		var services []string
		if len(deployment.Spec.ServicesOverride) != 0 {
			services = deployment.Spec.ServicesOverride
		} else {
			services = nodeset.Spec.Services
		}

		// Complete AnsibleEE job for each service
		for _, serviceName := range services {
			service := &dataplanev1.OpenStackDataPlaneService{}
			g := NewWithT(GinkgoT())
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Namespace: deploymentName.Namespace,
				Name:      serviceName,
			}, service)).Should(Succeed())

			aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
				service, deployment.GetName(), nodeset.GetName())
			Eventually(func(g Gomega) {
				ansibleeeName := types.NamespacedName{
					Name:      aeeName,
					Namespace: deploymentName.Namespace,
				}
				ansibleEE := GetAnsibleee(ansibleeeName)
				ansibleEE.Status.Succeeded = 1
				g.Expect(k8sClient.Status().Update(ctx, ansibleEE)).To(Succeed())
			}, timeout, interval).Should(Succeed())
		}

		// Then set the deployment status to ready
		Eventually(func(g Gomega) {
			deployment := &dataplanev1.OpenStackDataPlaneDeployment{}
			g.Expect(k8sClient.Get(ctx, deploymentName, deployment)).Should(Succeed())

			// Get the nodeset to access its ConfigHash
			nodeset := &dataplanev1.OpenStackDataPlaneNodeSet{}
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{
				Namespace: deploymentName.Namespace,
				Name:      nodesetName,
			}, nodeset)).Should(Succeed())

			// Set the deployment to ready with matching hashes
			deployment.Status.NodeSetConditions = map[string]condition.Conditions{
				nodesetName: {
					{
						Type:               dataplanev1.NodeSetDeploymentReadyCondition,
						Status:             corev1.ConditionTrue,
						Reason:             condition.ReadyReason,
						Message:            condition.DeploymentReadyMessage,
						LastTransitionTime: metav1.Time{Time: time.Now()},
					},
				},
			}
			// Set hashes to match nodeset
			if deployment.Status.NodeSetHashes == nil {
				deployment.Status.NodeSetHashes = make(map[string]string)
			}
			deployment.Status.NodeSetHashes[nodesetName] = nodeset.Status.ConfigHash

			g.Expect(k8sClient.Status().Update(ctx, deployment)).Should(Succeed())
		}, timeout, interval).Should(Succeed())
	}

	Context("Incremental Node Deployments", func() {
		var dataplaneNodeSetName types.NamespacedName
		var novaServiceName types.NamespacedName
		var dataplaneSSHSecretName types.NamespacedName
		var caBundleSecretName types.NamespacedName
		var dataplaneNetConfigName types.NamespacedName
		var dnsMasqName types.NamespacedName
		var novaUser *rabbitmqv1.RabbitMQUser

		BeforeEach(func() {
			err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
			Expect(err).NotTo(HaveOccurred())

			dataplaneNodeSetName = types.NamespacedName{
				Name:      "compute-rolling",
				Namespace: namespace,
			}
			novaServiceName = types.NamespacedName{
				Name:      "nova",
				Namespace: namespace,
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
				Name:      "dataplane-netconfig-rolling",
			}
			dnsMasqName = types.NamespacedName{
				Name:      "dnsmasq-rolling",
				Namespace: namespace,
			}

			// Create Nova config secret with user1
			CreateNovaCellConfigSecret("cell1", "nova-user1", "rabbitmq-cell1")
			novaUser = CreateRabbitMQUser("nova-user1")

			// Create Nova service
			CreateDataPlaneServiceFromSpec(novaServiceName, map[string]interface{}{
				"edpmServiceType": "nova",
			})
			DeferCleanup(th.DeleteService, novaServiceName)

			// Create network infrastructure
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create SSH and CA secrets
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)

			// Create nova-migration-ssh-key secret required by nova service
			CreateSSHSecret(types.NamespacedName{
				Namespace: namespace,
				Name:      "nova-migration-ssh-key",
			})
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, novaUser)
		})

		It("Should add finalizer only after ALL nodes are updated in rolling deployment", func() {
			// Create nodeset with 3 nodes
			nodeSetSpec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"compute-0": {
						HostName: "compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.100",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
					"compute-1": {
						HostName: "compute-1",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.101",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
					"compute-2": {
						HostName: "compute-2",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.102",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))

			// Simulate IP sets
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-0"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-1"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-2"})
			SimulateDNSDataComplete(dataplaneNodeSetName)

			// Step 1: Deploy first node (ansibleLimit: compute-0)
			deployment1Name := types.NamespacedName{
				Name:      "deploy-compute-0",
				Namespace: namespace,
			}
			deploymentSpec1 := map[string]interface{}{
				"nodeSets":     []string{dataplaneNodeSetName.Name},
				"ansibleLimit": "compute-0",
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployment1Name, deploymentSpec1))
			SimulateDeploymentComplete(deployment1Name, dataplaneNodeSetName.Name, []string{"compute-0"})

			// After first deployment, finalizer should NOT be added (only 1/3 nodes)
			Consistently(func(g Gomega) {
				g.Expect(HasFinalizer("nova-user1", "nodeset.os/")).Should(BeFalse())
			}, time.Second*5, interval).Should(Succeed())

			// Step 2: Deploy second node (ansibleLimit: compute-1)
			deployment2Name := types.NamespacedName{
				Name:      "deploy-compute-1",
				Namespace: namespace,
			}
			deploymentSpec2 := map[string]interface{}{
				"nodeSets":     []string{dataplaneNodeSetName.Name},
				"ansibleLimit": "compute-1",
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployment2Name, deploymentSpec2))
			SimulateDeploymentComplete(deployment2Name, dataplaneNodeSetName.Name, []string{"compute-1"})

			// After second deployment, finalizer should still NOT be added (only 2/3 nodes)
			Consistently(func(g Gomega) {
				g.Expect(HasFinalizer("nova-user1", "nodeset.os/")).Should(BeFalse())
			}, time.Second*5, interval).Should(Succeed())

			// Step 3: Deploy third node (ansibleLimit: compute-2)
			deployment3Name := types.NamespacedName{
				Name:      "deploy-compute-2",
				Namespace: namespace,
			}
			deploymentSpec3 := map[string]interface{}{
				"nodeSets":     []string{dataplaneNodeSetName.Name},
				"ansibleLimit": "compute-2",
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployment3Name, deploymentSpec3))
			SimulateDeploymentComplete(deployment3Name, dataplaneNodeSetName.Name, []string{"compute-2"})

			// After ALL nodes deployed, finalizer SHOULD be added
			// First verify all deployments are actually marked as Ready
			Eventually(func(g Gomega) {
				for _, deployName := range []string{"deploy-compute-0", "deploy-compute-1", "deploy-compute-2"} {
					deploy := &dataplanev1.OpenStackDataPlaneDeployment{}
					g.Expect(k8sClient.Get(ctx, types.NamespacedName{
						Namespace: namespace,
						Name:      deployName,
					}, deploy)).Should(Succeed())
					conds := deploy.Status.NodeSetConditions[dataplaneNodeSetName.Name]
					g.Expect(conds).ShouldNot(BeNil(), fmt.Sprintf("%s should have conditions", deployName))
					readyCond := conds.Get(dataplanev1.NodeSetDeploymentReadyCondition)
					g.Expect(readyCond).ShouldNot(BeNil(), fmt.Sprintf("%s should have ready condition", deployName))
					g.Expect(readyCond.Status).Should(Equal(corev1.ConditionTrue), fmt.Sprintf("%s should be ready", deployName))
				}
			}, timeout, interval).Should(Succeed())

			// Now check for finalizer
			Eventually(func(g Gomega) {
				user := GetRabbitMQUser("nova-user1")
				hasFinalizerPrefix := false
				for _, f := range user.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						hasFinalizerPrefix = true
						break
					}
				}
				g.Expect(hasFinalizerPrefix).Should(BeTrue(), "Finalizer should be added after all nodes updated")
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("Multi-NodeSet Shared User Management", func() {
		var nodeset1Name, nodeset2Name types.NamespacedName
		var novaServiceName types.NamespacedName
		var novaUser *rabbitmqv1.RabbitMQUser

		BeforeEach(func() {
			err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
			Expect(err).NotTo(HaveOccurred())

			nodeset1Name = types.NamespacedName{
				Name:      "compute-zone1",
				Namespace: namespace,
			}
			nodeset2Name = types.NamespacedName{
				Name:      "compute-zone2",
				Namespace: namespace,
			}
			novaServiceName = types.NamespacedName{
				Name:      "nova",
				Namespace: namespace,
			}

			// Both nodesets use the same Nova cluster and user
			CreateNovaCellConfigSecret("cell1", "nova-cell1", "rabbitmq-cell1")
			novaUser = CreateRabbitMQUser("nova-cell1")

			// Create Nova service
			CreateDataPlaneServiceFromSpec(novaServiceName, map[string]interface{}{
				"edpmServiceType": "nova",
			})
			DeferCleanup(th.DeleteService, novaServiceName)

			// Create nova-migration-ssh-key secret required by nova service
			CreateSSHSecret(types.NamespacedName{
				Namespace: namespace,
				Name:      "nova-migration-ssh-key",
			})
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, novaUser)
		})

		It("Should add independent finalizers from each nodeset to shared user", func() {
			// Setup network infrastructure
			netConfigName := types.NamespacedName{Namespace: namespace, Name: "dataplane-netconfig-multi"}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(netConfigName, DefaultNetConfigSpec()))

			dnsMasqName := types.NamespacedName{Namespace: namespace, Name: "dnsmasq-multi"}
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create first nodeset
			nodeSet1Spec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"zone1-compute-0": {
						HostName: "zone1-compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.110",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			CreateDataplaneNodeSet(nodeset1Name, nodeSet1Spec)
			// Note: nodeset1 is explicitly deleted as part of the test, so no DeferCleanup needed

			// Setup for zone1
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "dataplane-ansible-ssh-private-key-secret"})
			CreateCABundleSecret(types.NamespacedName{Namespace: namespace, Name: "combined-ca-bundle"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "zone1-compute-0"})
			SimulateDNSDataComplete(nodeset1Name)

			// Deploy zone1
			deploy1Name := types.NamespacedName{Name: "deploy-zone1", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deploy1Name, map[string]interface{}{
				"nodeSets": []string{nodeset1Name.Name},
			}))
			SimulateDeploymentComplete(deploy1Name, nodeset1Name.Name, []string{"zone1-compute-0"})

			// Verify zone1 finalizer added
			var zone1Finalizer string
			Eventually(func(g Gomega) {
				user := GetRabbitMQUser("nova-cell1")
				found := false
				for _, f := range user.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						zone1Finalizer = f
						found = true
						break
					}
				}
				g.Expect(found).Should(BeTrue(), "Zone1 should add its finalizer")
			}, timeout, interval).Should(Succeed())

			// Create second nodeset
			nodeSet2Spec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"zone2-compute-0": {
						HostName: "zone2-compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.120",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			CreateDataplaneNodeSet(nodeset2Name, nodeSet2Spec)
			DeferCleanup(func() { th.DeleteInstance(GetDataplaneNodeSet(nodeset2Name)) })

			// Setup for zone2
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "zone2-compute-0"})
			SimulateDNSDataComplete(nodeset2Name)

			// Deploy zone2
			deploy2Name := types.NamespacedName{Name: "deploy-zone2", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deploy2Name, map[string]interface{}{
				"nodeSets": []string{nodeset2Name.Name},
			}))
			SimulateDeploymentComplete(deploy2Name, nodeset2Name.Name, []string{"zone2-compute-0"})

			// Verify BOTH finalizers exist (zone1 and zone2)
			Eventually(func(g Gomega) {
				user := GetRabbitMQUser("nova-cell1")
				finalizerCount := 0
				for _, f := range user.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						finalizerCount++
					}
				}
				g.Expect(finalizerCount).Should(Equal(2), "Both nodesets should have independent finalizers")
			}, timeout, interval).Should(Succeed())

			// Delete zone1 nodeset
			Expect(k8sClient.Delete(ctx, GetDataplaneNodeSet(nodeset1Name))).Should(Succeed())

			// Verify zone1 finalizer removed but zone2 finalizer remains
			Eventually(func(g Gomega) {
				user := GetRabbitMQUser("nova-cell1")
				hasZone1 := false
				hasZone2 := false
				for _, f := range user.Finalizers {
					if f == zone1Finalizer {
						hasZone1 = true
					}
					if len(f) > 11 && f[:11] == "nodeset.os/" && f != zone1Finalizer {
						hasZone2 = true
					}
				}
				g.Expect(hasZone1).Should(BeFalse(), "Zone1 finalizer should be removed after deletion")
				g.Expect(hasZone2).Should(BeTrue(), "Zone2 finalizer should remain")
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("RabbitMQ User Credential Rotation", func() {
		var dataplaneNodeSetName types.NamespacedName
		var novaServiceName types.NamespacedName
		var oldUser, newUser *rabbitmqv1.RabbitMQUser

		BeforeEach(func() {
			err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
			Expect(err).NotTo(HaveOccurred())

			dataplaneNodeSetName = types.NamespacedName{
				Name:      "compute-rotation",
				Namespace: namespace,
			}
			novaServiceName = types.NamespacedName{
				Name:      "nova",
				Namespace: namespace,
			}

			// Create initial config with old user
			CreateNovaCellConfigSecret("cell1", "nova-old", "rabbitmq-cell1")
			oldUser = CreateRabbitMQUser("nova-old")
			newUser = CreateRabbitMQUser("nova-new")

			// Create Nova service
			CreateDataPlaneServiceFromSpec(novaServiceName, map[string]interface{}{
				"edpmServiceType": "nova",
			})
			DeferCleanup(th.DeleteService, novaServiceName)

			// Create nova-migration-ssh-key secret required by nova service
			CreateSSHSecret(types.NamespacedName{
				Namespace: namespace,
				Name:      "nova-migration-ssh-key",
			})
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, oldUser)
			k8sClient.Delete(ctx, newUser)
		})

		It("Should switch finalizer from old user to new user after rotation completes", func() {
			// Setup network infrastructure
			netConfigName := types.NamespacedName{Namespace: namespace, Name: "dataplane-netconfig-rotation"}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(netConfigName, DefaultNetConfigSpec()))

			dnsMasqName := types.NamespacedName{Namespace: namespace, Name: "dnsmasq-rotation"}
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create nodeset with 2 nodes
			nodeSetSpec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"compute-0": {
						HostName: "compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.100",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
					"compute-1": {
						HostName: "compute-1",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.101",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec)
			DeferCleanup(func() { th.DeleteInstance(GetDataplaneNodeSet(dataplaneNodeSetName)) })

			// Setup
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "dataplane-ansible-ssh-private-key-secret"})
			CreateCABundleSecret(types.NamespacedName{Namespace: namespace, Name: "combined-ca-bundle"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-0"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-1"})
			SimulateDNSDataComplete(dataplaneNodeSetName)

			// Initial deployment with old user
			deploy1Name := types.NamespacedName{Name: "deploy-initial", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deploy1Name, map[string]interface{}{
				"nodeSets": []string{dataplaneNodeSetName.Name},
			}))
			SimulateDeploymentComplete(deploy1Name, dataplaneNodeSetName.Name, []string{"compute-0", "compute-1"})

			// Verify old user has finalizer
			var oldUserFinalizer string
			Eventually(func(g Gomega) {
				user := GetRabbitMQUser("nova-old")
				found := false
				for _, f := range user.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						oldUserFinalizer = f
						found = true
						break
					}
				}
				g.Expect(found).Should(BeTrue(), "Old user should have finalizer after initial deployment")
			}, timeout, interval).Should(Succeed())

			// Rotate credentials: update secret to use new user
			UpdateNovaCellConfigSecret("cell1", "nova-new", "rabbitmq-cell1")

			// Wait a moment for secret to propagate
			time.Sleep(time.Second * 2)

			// Rolling update with new credentials - first node
			deploy2Name := types.NamespacedName{Name: "deploy-rotate-node0", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deploy2Name, map[string]interface{}{
				"nodeSets":     []string{dataplaneNodeSetName.Name},
				"ansibleLimit": "compute-0",
			}))
			SimulateDeploymentComplete(deploy2Name, dataplaneNodeSetName.Name, []string{"compute-0"})

			// After first node, old user finalizer should remain (not all nodes updated)
			Consistently(func(g Gomega) {
				g.Expect(HasFinalizer("nova-old", oldUserFinalizer)).Should(BeTrue())
			}, time.Second*5, interval).Should(Succeed())

			// Rolling update - second node
			deploy3Name := types.NamespacedName{Name: "deploy-rotate-node1", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deploy3Name, map[string]interface{}{
				"nodeSets":     []string{dataplaneNodeSetName.Name},
				"ansibleLimit": "compute-1",
			}))
			SimulateDeploymentComplete(deploy3Name, dataplaneNodeSetName.Name, []string{"compute-1"})

			// After all nodes updated with new credentials:
			// 1. New user should have finalizer
			// 2. Old user finalizer should be removed
			Eventually(func(g Gomega) {
				newUserObj := GetRabbitMQUser("nova-new")
				newUserHasFinalizer := false
				for _, f := range newUserObj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						newUserHasFinalizer = true
						break
					}
				}
				g.Expect(newUserHasFinalizer).Should(BeTrue(), "New user should have finalizer after rotation")

				oldUserObj := GetRabbitMQUser("nova-old")
				oldUserHasFinalizer := false
				for _, f := range oldUserObj.Finalizers {
					if f == oldUserFinalizer {
						oldUserHasFinalizer = true
						break
					}
				}
				g.Expect(oldUserHasFinalizer).Should(BeFalse(), "Old user finalizer should be removed after rotation")
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("Multi-Service RabbitMQ Cluster Management", func() {
		var dataplaneNodeSetName types.NamespacedName
		var novaServiceName, neutronServiceName, ironicServiceName types.NamespacedName
		var novaUser, neutronUser, ironicUser *rabbitmqv1.RabbitMQUser

		BeforeEach(func() {
			err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
			Expect(err).NotTo(HaveOccurred())

			dataplaneNodeSetName = types.NamespacedName{
				Name:      "compute-multiservice",
				Namespace: namespace,
			}
			novaServiceName = types.NamespacedName{Name: "nova", Namespace: namespace}
			neutronServiceName = types.NamespacedName{Name: "neutron", Namespace: namespace}
			ironicServiceName = types.NamespacedName{Name: "ironic-neutron-agent", Namespace: namespace}

			// Create config secrets for different services using DIFFERENT clusters
			CreateNovaCellConfigSecret("cell1", "nova-cell1", "rabbitmq-cell1")
			CreateNeutronAgentConfigSecret("dhcp", "neutron", "rabbitmq-network")
			CreateIronicNeutronAgentConfigSecret("ironic", "rabbitmq-baremetal")

			// Create RabbitMQ users
			novaUser = CreateRabbitMQUser("nova-cell1")
			neutronUser = CreateRabbitMQUser("neutron")
			ironicUser = CreateRabbitMQUser("ironic")

			// Create services
			CreateDataPlaneServiceFromSpec(novaServiceName, map[string]interface{}{"edpmServiceType": "nova"})
			CreateDataPlaneServiceFromSpec(neutronServiceName, map[string]interface{}{"edpmServiceType": "neutron-dhcp"})
			CreateDataPlaneServiceFromSpec(ironicServiceName, map[string]interface{}{"edpmServiceType": "ironic-neutron-agent"})

			DeferCleanup(th.DeleteService, novaServiceName)
			DeferCleanup(th.DeleteService, neutronServiceName)
			DeferCleanup(th.DeleteService, ironicServiceName)

			// Create migration secrets required by each service
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "nova-migration-ssh-key"})
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "neutron-migration-ssh-key"})
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "ironic-neutron-agent-migration-ssh-key"})
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, novaUser)
			k8sClient.Delete(ctx, neutronUser)
			k8sClient.Delete(ctx, ironicUser)
		})

		It("Should manage service-specific finalizers independently across different clusters", func() {
			// Setup network infrastructure
			netConfigName := types.NamespacedName{Namespace: namespace, Name: "dataplane-netconfig-multiservice"}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(netConfigName, DefaultNetConfigSpec()))

			dnsMasqName := types.NamespacedName{Namespace: namespace, Name: "dnsmasq-multiservice"}
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create nodeset with multiple services
			nodeSetSpec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova", "neutron", "ironic-neutron-agent"},
				"nodes": map[string]dataplanev1.NodeSection{
					"compute-0": {
						HostName: "compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.100",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec)
			DeferCleanup(func() { th.DeleteInstance(GetDataplaneNodeSet(dataplaneNodeSetName)) })

			// Setup
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "dataplane-ansible-ssh-private-key-secret"})
			CreateCABundleSecret(types.NamespacedName{Namespace: namespace, Name: "combined-ca-bundle"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-0"})
			SimulateDNSDataComplete(dataplaneNodeSetName)

			// Deploy all services
			deployName := types.NamespacedName{Name: "deploy-all-services", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployName, map[string]interface{}{
				"nodeSets": []string{dataplaneNodeSetName.Name},
			}))
			SimulateDeploymentComplete(deployName, dataplaneNodeSetName.Name, []string{"compute-0"})

			// Each service should have its own finalizer on its respective user
			// Format: nodeset.os/{hash}-{service}
			Eventually(func(g Gomega) {
				// Nova should have finalizer with -nova suffix
				novaUserObj := GetRabbitMQUser("nova-cell1")
				novaHasFinalizer := false
				for _, f := range novaUserObj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" && len(f) >= 17 && f[len(f)-5:] == "-nova" {
						novaHasFinalizer = true
						break
					}
				}
				g.Expect(novaHasFinalizer).Should(BeTrue(), "Nova user should have service-specific finalizer")

				// Neutron should have finalizer with -neutron suffix
				neutronUserObj := GetRabbitMQUser("neutron")
				neutronHasFinalizer := false
				for _, f := range neutronUserObj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" && len(f) >= 19 && f[len(f)-8:] == "-neutron" {
						neutronHasFinalizer = true
						break
					}
				}
				g.Expect(neutronHasFinalizer).Should(BeTrue(), "Neutron user should have service-specific finalizer")

				// Ironic should have finalizer with -ironic suffix
				ironicUserObj := GetRabbitMQUser("ironic")
				ironicHasFinalizer := false
				for _, f := range ironicUserObj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" && len(f) >= 19 && f[len(f)-7:] == "-ironic" {
						ironicHasFinalizer = true
						break
					}
				}
				g.Expect(ironicHasFinalizer).Should(BeTrue(), "Ironic user should have service-specific finalizer")
			}, timeout, interval).Should(Succeed())

			// Verify that finalizers are independent (each user has exactly 1 finalizer)
			Eventually(func(g Gomega) {
				novaCount := 0
				for _, f := range GetRabbitMQUser("nova-cell1").Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						novaCount++
					}
				}
				g.Expect(novaCount).Should(Equal(1), "Nova user should have exactly 1 finalizer")

				neutronCount := 0
				for _, f := range GetRabbitMQUser("neutron").Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						neutronCount++
					}
				}
				g.Expect(neutronCount).Should(Equal(1), "Neutron user should have exactly 1 finalizer")

				ironicCount := 0
				for _, f := range GetRabbitMQUser("ironic").Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						ironicCount++
					}
				}
				g.Expect(ironicCount).Should(Equal(1), "Ironic user should have exactly 1 finalizer")
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("Deployment Timing and Secret Changes", func() {
		var dataplaneNodeSetName types.NamespacedName
		var novaServiceName types.NamespacedName
		var novaUser *rabbitmqv1.RabbitMQUser

		BeforeEach(func() {
			err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
			Expect(err).NotTo(HaveOccurred())

			dataplaneNodeSetName = types.NamespacedName{
				Name:      "compute-edge",
				Namespace: namespace,
			}
			novaServiceName = types.NamespacedName{
				Name:      "nova",
				Namespace: namespace,
			}

			CreateNovaCellConfigSecret("cell1", "nova-user1", "rabbitmq-cell1")
			novaUser = CreateRabbitMQUser("nova-user1")

			CreateDataPlaneServiceFromSpec(novaServiceName, map[string]interface{}{
				"edpmServiceType": "nova",
			})
			DeferCleanup(th.DeleteService, novaServiceName)

			// Create nova-migration-ssh-key secret required by nova service
			CreateSSHSecret(types.NamespacedName{
				Namespace: namespace,
				Name:      "nova-migration-ssh-key",
			})
		})

		AfterEach(func() {
			k8sClient.Delete(ctx, novaUser)
		})

		It("Should use deployment completion time not creation time", func() {
			// Setup network infrastructure
			netConfigName := types.NamespacedName{Namespace: namespace, Name: "dataplane-netconfig-timing"}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(netConfigName, DefaultNetConfigSpec()))

			dnsMasqName := types.NamespacedName{Namespace: namespace, Name: "dnsmasq-timing"}
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create nodeset
			nodeSetSpec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"compute-0": {
						HostName: "compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.100",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec)
			DeferCleanup(func() { th.DeleteInstance(GetDataplaneNodeSet(dataplaneNodeSetName)) })

			// Setup
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "dataplane-ansible-ssh-private-key-secret"})
			CreateCABundleSecret(types.NamespacedName{Namespace: namespace, Name: "combined-ca-bundle"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-0"})
			SimulateDNSDataComplete(dataplaneNodeSetName)

			// Create deployment (creation time = now)
			deployName := types.NamespacedName{Name: "deploy-before-secret", Namespace: namespace}
			CreateDataplaneDeployment(deployName, map[string]interface{}{
				"nodeSets": []string{dataplaneNodeSetName.Name},
			})
			DeferCleanup(func() { th.DeleteInstance(GetDataplaneDeployment(deployName)) })

			// Wait a moment
			time.Sleep(time.Second * 2)

			// Rotate secret (secret modified time = now, AFTER deployment creation)
			UpdateNovaCellConfigSecret("cell1", "nova-user2", "rabbitmq-cell1")
			newUser := CreateRabbitMQUser("nova-user2")
			DeferCleanup(func() { k8sClient.Delete(ctx, newUser) })

			// Wait for secret to propagate
			time.Sleep(time.Second * 2)

			// Now complete the deployment (completion time = now, AFTER secret change)
			SimulateDeploymentComplete(deployName, dataplaneNodeSetName.Name, []string{"compute-0"})

			// Since deployment COMPLETED after secret change, it should track the node
			// and manage finalizers for the NEW user (nova-user2), not the old one
			Eventually(func(g Gomega) {
				newUserObj := GetRabbitMQUser("nova-user2")
				hasNewFinalizer := false
				for _, f := range newUserObj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						hasNewFinalizer = true
						break
					}
				}
				g.Expect(hasNewFinalizer).Should(BeTrue(), "New user should have finalizer (deployment completed after secret change)")
			}, timeout, interval).Should(Succeed())
		})

		It("Should reset tracking when secret changes during deployment", func() {
			// Setup network infrastructure
			netConfigName := types.NamespacedName{Namespace: namespace, Name: "dataplane-netconfig-reset"}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(netConfigName, DefaultNetConfigSpec()))

			dnsMasqName := types.NamespacedName{Namespace: namespace, Name: "dnsmasq-reset"}
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create nodeset with 2 nodes
			nodeSetSpec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"compute-0": {
						HostName: "compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.100",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
					"compute-1": {
						HostName: "compute-1",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.101",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec)
			DeferCleanup(func() { th.DeleteInstance(GetDataplaneNodeSet(dataplaneNodeSetName)) })

			// Setup
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "dataplane-ansible-ssh-private-key-secret"})
			CreateCABundleSecret(types.NamespacedName{Namespace: namespace, Name: "combined-ca-bundle"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-0"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-1"})
			SimulateDNSDataComplete(dataplaneNodeSetName)

			// Initial full deployment with nova-user1 to establish baseline
			deployInitialName := types.NamespacedName{Name: "deploy-initial", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployInitialName, map[string]interface{}{
				"nodeSets": []string{dataplaneNodeSetName.Name},
			}))
			SimulateDeploymentComplete(deployInitialName, dataplaneNodeSetName.Name, []string{"compute-0", "compute-1"})

			// Wait for finalizer to be added
			Eventually(func(g Gomega) {
				user := GetRabbitMQUser("nova-user1")
				hasOldFinalizer := false
				for _, f := range user.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						hasOldFinalizer = true
						break
					}
				}
				g.Expect(hasOldFinalizer).Should(BeTrue(), "Initial user should have finalizer after full deployment")
			}, timeout, interval).Should(Succeed())

			// Deploy first node again with same credentials (partial deployment)
			deploy1Name := types.NamespacedName{Name: "deploy-node0", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deploy1Name, map[string]interface{}{
				"nodeSets":     []string{dataplaneNodeSetName.Name},
				"ansibleLimit": "compute-0",
			}))
			SimulateDeploymentComplete(deploy1Name, dataplaneNodeSetName.Name, []string{"compute-0"})

			// After partial deployment, old user should still have finalizer (not all nodes updated)
			Consistently(func(g Gomega) {
				user := GetRabbitMQUser("nova-user1")
				hasOldFinalizer := false
				for _, f := range user.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						hasOldFinalizer = true
						break
					}
				}
				g.Expect(hasOldFinalizer).Should(BeTrue(), "Old user should keep finalizer after partial deployment")
			}, time.Second*5, interval).Should(Succeed())

			// Change secret (this triggers credential rotation)
			UpdateNovaCellConfigSecret("cell1", "nova-user2", "rabbitmq-cell1")
			newUser := CreateRabbitMQUser("nova-user2")
			DeferCleanup(func() { k8sClient.Delete(ctx, newUser) })

			// Wait for secret to propagate
			time.Sleep(time.Second * 2)

			// Deploy both nodes with new secret
			// This simulates a redeploy after secret rotation
			deploy2Name := types.NamespacedName{Name: "deploy-both-new", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deploy2Name, map[string]interface{}{
				"nodeSets": []string{dataplaneNodeSetName.Name},
			}))
			SimulateDeploymentComplete(deploy2Name, dataplaneNodeSetName.Name, []string{"compute-0", "compute-1"})

			// After all nodes deployed with new credentials:
			// 1. New user should have finalizer
			// 2. Old user finalizer should be removed
			Eventually(func(g Gomega) {
				newUserObj := GetRabbitMQUser("nova-user2")
				newUserHasFinalizer := false
				for _, f := range newUserObj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						newUserHasFinalizer = true
						break
					}
				}
				g.Expect(newUserHasFinalizer).Should(BeTrue(), "New user should have finalizer after full deployment with new credentials")

				oldUserObj := GetRabbitMQUser("nova-user1")
				oldUserHasFinalizer := false
				for _, f := range oldUserObj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						oldUserHasFinalizer = true
						break
					}
				}
				g.Expect(oldUserHasFinalizer).Should(BeFalse(), "Old user finalizer should be removed after rotation completes")
			}, timeout, interval).Should(Succeed())
		})

		PIt("Should add finalizers immediately during partial deployment (improved protection)", func() {
			// This test demonstrates the improved behavior: finalizers are added as soon as
			// ANY node starts using credentials, providing protection during rolling updates
			// TODO: This test needs more work on credential rotation tracking in the controller

			// Setup network infrastructure
			netConfigName := types.NamespacedName{Namespace: namespace, Name: "dataplane-netconfig-partial"}
			DeferCleanup(th.DeleteInstance, CreateNetConfig(netConfigName, DefaultNetConfigSpec()))

			dnsMasqName := types.NamespacedName{Namespace: namespace, Name: "dnsmasq-partial"}
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create nodeset with 2 nodes
			nodeSetSpec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"compute-0": {
						HostName: "compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.100",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
					"compute-1": {
						HostName: "compute-1",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.101",
						},
						Networks: []infrav1.IPSetNetwork{
							{Name: "ctlplane", SubnetName: "subnet1"},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec)
			DeferCleanup(func() { th.DeleteInstance(GetDataplaneNodeSet(dataplaneNodeSetName)) })

			// Setup
			CreateSSHSecret(types.NamespacedName{Namespace: namespace, Name: "dataplane-ansible-ssh-private-key-secret"})
			CreateCABundleSecret(types.NamespacedName{Namespace: namespace, Name: "combined-ca-bundle"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-0"})
			SimulateIPSetComplete(types.NamespacedName{Namespace: namespace, Name: "compute-1"})
			SimulateDNSDataComplete(dataplaneNodeSetName)

			// Initial full deployment with user1
			deployInitialName := types.NamespacedName{Name: "deploy-initial-partial", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployInitialName, map[string]interface{}{
				"nodeSets": []string{dataplaneNodeSetName.Name},
			}))
			SimulateDeploymentComplete(deployInitialName, dataplaneNodeSetName.Name, []string{"compute-0", "compute-1"})

			// Verify user1 has finalizer after full deployment
			Eventually(func(g Gomega) {
				user1 := GetRabbitMQUser("nova-user1")
				hasFinalizer := false
				for _, f := range user1.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						hasFinalizer = true
						break
					}
				}
				g.Expect(hasFinalizer).Should(BeTrue(), "User1 should have finalizer after full deployment")
			}, timeout, interval).Should(Succeed())

			// Change secret to user2
			UpdateNovaCellConfigSecret("cell1", "nova-user2", "rabbitmq-cell1")
			user2 := CreateRabbitMQUser("nova-user2")
			DeferCleanup(func() { k8sClient.Delete(ctx, user2) })

			// Wait for secret to propagate
			time.Sleep(time.Second * 2)

			// PARTIAL deployment with new user2 (only compute-0)
			// This is the key test: user2 should get finalizer immediately
			deployPartialName := types.NamespacedName{Name: "deploy-partial-node0", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployPartialName, map[string]interface{}{
				"nodeSets":     []string{dataplaneNodeSetName.Name},
				"ansibleLimit": "compute-0",
			}))
			SimulateDeploymentComplete(deployPartialName, dataplaneNodeSetName.Name, []string{"compute-0"})

			// CRITICAL TEST: After partial deployment (1 of 2 nodes):
			// - user2 should have finalizer (NEW BEHAVIOR - immediate protection)
			// - user1 should STILL have finalizer (compute-1 still using it)
			Eventually(func(g Gomega) {
				user2Obj := GetRabbitMQUser("nova-user2")
				user2HasFinalizer := false
				for _, f := range user2Obj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						user2HasFinalizer = true
						break
					}
				}
				g.Expect(user2HasFinalizer).Should(BeTrue(), "User2 should have finalizer immediately after ANY node uses it (partial deployment)")

				user1Obj := GetRabbitMQUser("nova-user1")
				user1HasFinalizer := false
				for _, f := range user1Obj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						user1HasFinalizer = true
						break
					}
				}
				g.Expect(user1HasFinalizer).Should(BeTrue(), "User1 should keep finalizer during partial deployment (compute-1 still using it)")
			}, timeout, interval).Should(Succeed())

			// Complete deployment with user2 (both nodes)
			deployCompleteName := types.NamespacedName{Name: "deploy-complete-both", Namespace: namespace}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(deployCompleteName, map[string]interface{}{
				"nodeSets": []string{dataplaneNodeSetName.Name},
			}))
			SimulateDeploymentComplete(deployCompleteName, dataplaneNodeSetName.Name, []string{"compute-0", "compute-1"})

			// After full deployment: user2 keeps finalizer, user1 finalizer removed
			Eventually(func(g Gomega) {
				user2Obj := GetRabbitMQUser("nova-user2")
				user2HasFinalizer := false
				for _, f := range user2Obj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						user2HasFinalizer = true
						break
					}
				}
				g.Expect(user2HasFinalizer).Should(BeTrue(), "User2 should keep finalizer after full deployment")

				user1Obj := GetRabbitMQUser("nova-user1")
				user1HasFinalizer := false
				for _, f := range user1Obj.Finalizers {
					if len(f) > 11 && f[:11] == "nodeset.os/" {
						user1HasFinalizer = true
						break
					}
				}
				g.Expect(user1HasFinalizer).Should(BeFalse(), "User1 finalizer should be removed after all nodes migrated")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A NodeSet with 2 nodes is created", func() {
		var dataplaneNodeSetName types.NamespacedName
		var dataplaneSSHSecretName types.NamespacedName
		var caBundleSecretName types.NamespacedName
		var dataplaneNetConfigName types.NamespacedName
		var dnsMasqName types.NamespacedName
		var dataplaneNode0Name types.NamespacedName
		var dataplaneNode1Name types.NamespacedName
		var novaServiceName types.NamespacedName

		BeforeEach(func() {
			// Set OPERATOR_SERVICES to point to services directory
			err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
			Expect(err).NotTo(HaveOccurred())

			dnsMasqName = types.NamespacedName{
				Name:      "dnsmasq-rabbitmq-test",
				Namespace: namespace,
			}
			dataplaneNodeSetName = types.NamespacedName{
				Name:      "edpm-compute-rabbitmq-test",
				Namespace: namespace,
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
				Name:      "dataplane-netconfig-rabbitmq-test",
			}
			dataplaneNode0Name = types.NamespacedName{
				Namespace: namespace,
				Name:      "edpm-compute-0",
			}
			dataplaneNode1Name = types.NamespacedName{
				Namespace: namespace,
				Name:      "edpm-compute-1",
			}
			novaServiceName = types.NamespacedName{
				Namespace: namespace,
				Name:      "nova",
			}
		})

		BeforeEach(func() {
			// Create nova service
			CreateDataPlaneServiceFromSpec(novaServiceName, map[string]interface{}{
				"edpmServiceType": "nova",
			})
			DeferCleanup(th.DeleteService, novaServiceName)

			// Create network infrastructure
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create nodeset with 2 nodes
			nodeSetSpec := map[string]interface{}{
				"preProvisioned": true,
				"services":       []string{"nova"},
				"nodes": map[string]dataplanev1.NodeSection{
					"edpm-compute-0": {
						HostName: "edpm-compute-0",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.100",
						},
						Networks: []infrav1.IPSetNetwork{
							{
								Name:       "ctlplane",
								SubnetName: "subnet1",
							},
						},
					},
					"edpm-compute-1": {
						HostName: "edpm-compute-1",
						Ansible: dataplanev1.AnsibleOpts{
							AnsibleHost: "192.168.122.101",
						},
						Networks: []infrav1.IPSetNetwork{
							{
								Name:       "ctlplane",
								SubnetName: "subnet1",
							},
						},
					},
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"managementNetwork":          "ctlplane",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-admin",
					},
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, nodeSetSpec))

			// Create SSH and CA secrets
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)

			// Simulate IP sets
			SimulateIPSetComplete(dataplaneNode0Name)
			SimulateIPSetComplete(dataplaneNode1Name)
			SimulateDNSDataComplete(dataplaneNodeSetName)
		})

		It("Should correctly count nodes without IP address aliases", func() {
			// Verify that getAllNodeNamesFromNodeset returns only 2 nodes, not 4
			// This validates the fix for Bug 3 where IP addresses were counted as separate nodes
			Eventually(func(g Gomega) {
				nodeset := GetDataplaneNodeSet(dataplaneNodeSetName)
				// Should have exactly 2 nodes defined (not 4 with hostName and ansibleHost)
				g.Expect(len(nodeset.Spec.Nodes)).Should(Equal(2))
			}, th.Timeout, th.Interval).Should(Succeed())
		})
	})
})
