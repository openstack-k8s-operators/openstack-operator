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
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	"k8s.io/apimachinery/pkg/types"
)

var _ = Describe("Dataplane NodeSet RabbitMQ Finalizer Management", func() {
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

	When("A NodeSet with 2 nodes is created", func() {
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
				"nodes": map[string]interface{}{
					"edpm-compute-0": map[string]interface{}{
						"hostName": "edpm-compute-0",
						"ansible": map[string]interface{}{
							"ansibleHost": "192.168.122.100",
						},
						"networks": []map[string]interface{}{
							{
								"name":       "ctlplane",
								"subnetName": "subnet1",
							},
						},
					},
					"edpm-compute-1": map[string]interface{}{
						"hostName": "edpm-compute-1",
						"ansible": map[string]interface{}{
							"ansibleHost": "192.168.122.101",
						},
						"networks": []map[string]interface{}{
							{
								"name":       "ctlplane",
								"subnetName": "subnet1",
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
