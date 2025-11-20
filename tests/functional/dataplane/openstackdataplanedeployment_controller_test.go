package functional

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/pkg/dataplane/util"

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
)

var _ = Describe("Dataplane Deployment Test", func() {
	var dataplaneDeploymentName types.NamespacedName
	var dataplaneNodeSetName types.NamespacedName
	var dataplaneSSHSecretName types.NamespacedName
	var caBundleSecretName types.NamespacedName
	var neutronOvnMetadataSecretName types.NamespacedName
	var novaNeutronMetadataSecretName types.NamespacedName
	var novaCellComputeConfigSecretName types.NamespacedName
	var novaMigrationSSHKey types.NamespacedName
	var ceilometerConfigSecretName types.NamespacedName
	var dataplaneNetConfigName types.NamespacedName
	var dnsMasqName types.NamespacedName
	var dataplaneNodeName types.NamespacedName
	var dataplaneMultiNodesetDeploymentName types.NamespacedName
	var dataplaneServiceName types.NamespacedName
	var dataplaneUpdateServiceName types.NamespacedName
	var dataplaneGlobalServiceName types.NamespacedName
	var controlPlaneName types.NamespacedName

	BeforeEach(func() {
		dnsMasqName = types.NamespacedName{
			Name:      "dnsmasq",
			Namespace: namespace,
		}
		dataplaneDeploymentName = types.NamespacedName{
			Name:      "edpm-deployment",
			Namespace: namespace,
		}
		dataplaneNodeSetName = types.NamespacedName{
			Name:      "edpm-compute-nodeset",
			Namespace: namespace,
		}
		dataplaneNodeName = types.NamespacedName{
			Namespace: namespace,
			Name:      "edpm-compute-node-1",
		}
		dataplaneSSHSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "dataplane-ansible-ssh-private-key-secret",
		}
		caBundleSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "combined-ca-bundle",
		}
		neutronOvnMetadataSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "neutron-ovn-metadata-agent-neutron-config",
		}
		novaNeutronMetadataSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "nova-metadata-neutron-config",
		}
		novaCellComputeConfigSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "nova-cell1-compute-config",
		}
		novaMigrationSSHKey = types.NamespacedName{
			Namespace: namespace,
			Name:      "nova-migration-ssh-key",
		}
		ceilometerConfigSecretName = types.NamespacedName{
			Namespace: namespace,
			Name:      "ceilometer-compute-config-data",
		}
		dataplaneNetConfigName = types.NamespacedName{
			Namespace: namespace,
			Name:      "dataplane-netconfig",
		}
		dataplaneMultiNodesetDeploymentName = types.NamespacedName{
			Namespace: namespace,
			Name:      "edpm-compute-nodeset-global",
		}
		dataplaneServiceName = types.NamespacedName{
			Namespace: namespace,
			Name:      "foo-service",
		}
		dataplaneUpdateServiceName = types.NamespacedName{
			Namespace: namespace,
			Name:      "foo-update-service",
		}
		dataplaneGlobalServiceName = types.NamespacedName{
			Name:      "global-service",
			Namespace: namespace,
		}
		controlPlaneName = types.NamespacedName{
			Name:      "mock-control-plane",
			Namespace: namespace,
		}
		err := os.Setenv("OPERATOR_SERVICES", "../../../config/services")
		Expect(err).NotTo(HaveOccurred())
	})

	When("A dataplaneDeployment is created with matching NodeSet", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			// DefaultDataPlanenodeSetSpec comes with three mock services
			// default service
			CreateDataplaneService(dataplaneServiceName, false)
			// marked for deployment on all nodesets
			CreateDataplaneService(dataplaneGlobalServiceName, true)
			// with EDPMServiceType set
			CreateDataPlaneServiceFromSpec(dataplaneUpdateServiceName, map[string]interface{}{
				"edpmServiceType":               "foo-update-service",
				"openStackAnsibleEERunnerImage": "foo-image:latest"})

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNodeSetSpec(dataplaneNodeSetName.Name)))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneDeploymentName)
			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              []string{"edpm-compute-nodeset"},
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set", func() {

			nodeSet := dataplanev1.OpenStackDataPlaneNodeSet{}
			baremetal := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeSet.Name,
					Namespace: nodeSet.Namespace,
				},
			}
			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSet = *GetDataplaneNodeSet(dataplaneNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, dataplaneNodeSetName, &baremetal)).To(Succeed())
				baremetal.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetal)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			// Create all services necessary for deployment
			for _, serviceName := range nodeSet.Spec.Services {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneDeploymentName)
				//Retrieve service AnsibleEE and set JobStatus to Successful
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSet.GetName())
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)

					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
					if service.Spec.EDPMServiceType != "" {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring("edpm_service_type"))
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring(service.Spec.EDPMServiceType))
					} else {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring(serviceName))
					}
					if service.Spec.DeployOnAllNodeSets {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring("edpm_override_hosts"))
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring("all"))
					} else {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring("edpm_override_hosts"))
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring(dataplaneNodeSetName.Name))
					}
				}, th.Timeout, th.Interval).Should(Succeed())
			}

			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A dataplaneDeployment is created with two NodeSets", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))

			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			// Three services on both nodesets
			CreateDataplaneService(dataplaneServiceName, false)
			CreateDataplaneService(dataplaneGlobalServiceName, true)
			CreateDataPlaneServiceFromSpec(dataplaneUpdateServiceName, map[string]interface{}{
				"edpmServiceType":               "foo-update-service",
				"openStackAnsibleEERunnerImage": "foo-image:latest"})

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)

			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			// Create both nodesets

			betaNodeName := fmt.Sprintf("%s-node-1", betaNodeSetName.Name)
			betaNodeSetSpec := map[string]interface{}{
				"preProvisioned": false,
				"services": []string{
					"foo-service",
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				},
				"nodes": map[string]interface{}{
					betaNodeName: map[string]interface{}{
						"hostname": betaNodeName,
						"networks": []map[string]interface{}{{
							"name":       "CtlPlane",
							"subnetName": "subnet1",
						},
						},
					},
				},
				"baremetalSetTemplate": map[string]interface{}{
					"baremetalHosts": map[string]interface{}{
						"ctlPlaneIP": map[string]interface{}{},
					},
					"deploymentSSHSecret": "dataplane-ansible-ssh-private-key-secret",
					"ctlplaneInterface":   "172.20.12.1",
				},
				"tlsEnabled": true,
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(alphaNodeSetName, DefaultDataPlaneNodeSetSpec(alphaNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(betaNodeSetName, betaNodeSetSpec))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(alphaNodeSetName)
			SimulateIPSetComplete(types.NamespacedName{Name: betaNodeName, Namespace: namespace})
			SimulateDNSDataComplete(betaNodeSetName)

			deploymentSpec := map[string]interface{}{
				"nodeSets": []string{
					"alpha-nodeset",
					"beta-nodeset",
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneMultiNodesetDeploymentName, deploymentSpec))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
			nodeSetsNames := []string{
				"alpha-nodeset",
				"beta-nodeset",
			}

			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              nodeSetsNames,
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set", func() {
			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			baremetalAlpha := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      alphaNodeSetName.Name,
					Namespace: alphaNodeSetName.Namespace,
				},
			}

			baremetalBeta := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      betaNodeSetName.Name,
					Namespace: betaNodeSetName.Namespace,
				},
			}

			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSetAlpha := *GetDataplaneNodeSet(alphaNodeSetName)
			nodeSetBeta := *GetDataplaneNodeSet(betaNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, alphaNodeSetName, &baremetalAlpha)).To(Succeed())
				baremetalAlpha.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalAlpha)).To(Succeed())
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, betaNodeSetName, &baremetalBeta)).To(Succeed())
				baremetalBeta.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalBeta)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			// Create all services necessary for deployment
			for _, serviceName := range nodeSetAlpha.Spec.Services {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSetAlpha.GetName())
				//Retrieve service AnsibleEE and set JobStatus to Successful
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneMultiNodesetDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					if service.Spec.DeployOnAllNodeSets {
						g.Expect(ansibleEE.Spec.Template.Spec.Volumes).Should(HaveLen(5))
					} else {
						g.Expect(ansibleEE.Spec.Template.Spec.Volumes).Should(HaveLen(3))
					}
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
					if service.Spec.EDPMServiceType != "" {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring(service.Spec.EDPMServiceType))
					} else {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring(serviceName))
					}
					if service.Spec.DeployOnAllNodeSets {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring("edpm_override_hosts"))
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring("all"))

					}
				}, th.Timeout, th.Interval).Should(Succeed())
			}

			// Create all services necessary for deployment
			for _, serviceName := range nodeSetBeta.Spec.Services {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSetBeta.GetName())

				//Retrieve service AnsibleEE and set JobStatus to Successful
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneMultiNodesetDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					if service.Spec.DeployOnAllNodeSets {
						g.Expect(ansibleEE.Spec.Template.Spec.Volumes).Should(HaveLen(5))
					} else {
						g.Expect(ansibleEE.Spec.Template.Spec.Volumes).Should(HaveLen(3))
					}
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
					if service.Spec.EDPMServiceType != "" {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring(service.Spec.EDPMServiceType))
					} else {
						g.Expect(findEnvVar(ansibleEE.Spec.Template.Spec.Containers[0].Env).Value).To(ContainSubstring(serviceName))
					}
				}, th.Timeout, th.Interval).Should(Succeed())
			}

			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A dataplaneDeployment is created with a missing nodeset", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))

			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}

			// Two services on both nodesets
			CreateDataplaneService(dataplaneServiceName, false)

			DeferCleanup(th.DeleteService, dataplaneServiceName)

			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)

			// Create only one nodeset
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(alphaNodeSetName, DefaultDataPlaneNodeSetSpec(alphaNodeSetName.Name)))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(alphaNodeSetName)

			deploymentSpec := map[string]interface{}{
				"nodeSets": []string{
					"alpha-nodeset",
					"beta-nodeset",
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneMultiNodesetDeploymentName, deploymentSpec))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
			nodeSetsNames := []string{
				"alpha-nodeset",
				"beta-nodeset",
			}

			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              nodeSetsNames,
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set to unknown", func() {
			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			baremetalAlpha := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      alphaNodeSetName.Name,
					Namespace: alphaNodeSetName.Namespace,
				},
			}

			baremetalBeta := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      betaNodeSetName.Name,
					Namespace: betaNodeSetName.Namespace,
				},
			}

			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			// Set baremetal provisioning conditions to True
			// This must succeed, as the "alpha-nodeset" exists
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, alphaNodeSetName, &baremetalAlpha)).To(Succeed())
				baremetalAlpha.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalAlpha)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			// These must fail, as there is no "beta-nodeset"
			Expect(th.K8sClient.Get(th.Ctx, betaNodeSetName, &baremetalBeta)).NotTo(Succeed())
			baremetalBeta.Status.Conditions.MarkTrue(
				condition.ReadyCondition,
				condition.ReadyMessage)
			Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalBeta)).NotTo(Succeed())

			// These conditions must remain unknown
			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionUnknown,
			)
			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionUnknown,
			)
		})
	})
	When("A dataplaneDeployment is created with duplicate service in nodeset", func() {
		BeforeEach(func() {
			CreateDataplaneServicesWithSameServiceType(dataplaneServiceName)
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			// DefaultDataPlanenodeSetSpec comes with two mock services, one marked for deployment on all nodesets
			// But we will not create them to test this scenario
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DuplicateServiceNodeSetSpec(dataplaneNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)
		})
		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneDeploymentName)
			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              []string{"edpm-compute-nodeset"},
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set to true", func() {
			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)
			service := GetService(dataplaneServiceName)
			aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(service,
				dataplaneDeploymentName.Name, dataplaneNodeSetName.Name)
			ansibleeeName := types.NamespacedName{
				Name:      aeeName,
				Namespace: dataplaneDeploymentName.Namespace,
			}
			Eventually(func(g Gomega) {
				ansibleEE := GetAnsibleee(ansibleeeName)
				ansibleEE.Status.Succeeded = 1
				g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
			}, th.Timeout, th.Interval).Should(Succeed())

			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A dataplaneDeployment is created with two NodeSets both containing same globalservice", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))

			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			// Three services on both nodesets
			CreateDataplaneService(dataplaneServiceName, false)
			CreateDataplaneService(dataplaneGlobalServiceName, true)
			CreateDataPlaneServiceFromSpec(dataplaneUpdateServiceName, map[string]interface{}{
				"EDPMServiceType": "foo-update-service"})

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
			DeferCleanup(th.DeleteService, dataplaneUpdateServiceName)

			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			// Create both nodesets

			betaNodeName := fmt.Sprintf("%s-node-1", betaNodeSetName.Name)
			betaNodeSetSpec := map[string]interface{}{
				"preProvisioned": false,
				"services": []string{
					"foo-service",
					"global-service",
					"foo-update-service",
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				},
				"nodes": map[string]interface{}{
					betaNodeName: map[string]interface{}{
						"hostname": betaNodeName,
						"networks": []map[string]interface{}{{
							"name":       "CtlPlane",
							"subnetName": "subnet1",
						},
						},
					},
				},
				"baremetalSetTemplate": map[string]interface{}{
					"baremetalHosts": map[string]interface{}{
						"ctlPlaneIP": map[string]interface{}{},
					},
					"deploymentSSHSecret": "dataplane-ansible-ssh-private-key-secret",
					"ctlplaneInterface":   "172.20.12.1",
				},
				"tlsEnabled": true,
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(alphaNodeSetName, DefaultDataPlaneNodeSetSpec(alphaNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(betaNodeSetName, betaNodeSetSpec))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(alphaNodeSetName)
			SimulateIPSetComplete(types.NamespacedName{Name: betaNodeName, Namespace: namespace})
			SimulateDNSDataComplete(betaNodeSetName)

			deploymentSpec := map[string]interface{}{
				"nodeSets": []string{
					"alpha-nodeset",
					"beta-nodeset",
				},
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneMultiNodesetDeploymentName, deploymentSpec))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
			nodeSetsNames := []string{
				"alpha-nodeset",
				"beta-nodeset",
			}

			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              nodeSetsNames,
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set", func() {
			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			baremetalAlpha := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      alphaNodeSetName.Name,
					Namespace: alphaNodeSetName.Namespace,
				},
			}

			baremetalBeta := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      betaNodeSetName.Name,
					Namespace: betaNodeSetName.Namespace,
				},
			}

			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSetAlpha := *GetDataplaneNodeSet(alphaNodeSetName)
			nodeSetBeta := *GetDataplaneNodeSet(betaNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, alphaNodeSetName, &baremetalAlpha)).To(Succeed())
				baremetalAlpha.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalAlpha)).To(Succeed())
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, betaNodeSetName, &baremetalBeta)).To(Succeed())
				baremetalBeta.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalBeta)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			// Create all services necessary for deployment
			for _, serviceName := range nodeSetAlpha.Spec.Services {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSetAlpha.GetName())
				//Retrieve service AnsibleEE and set JobStatus to Successful
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneMultiNodesetDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())
			}

			servicesExcludingGlobal := []string{"foo-service", "foo-update-service"}
			// Create all services necessary for deployment
			for _, serviceName := range servicesExcludingGlobal {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSetBeta.GetName())

				//Retrieve service AnsibleEE and set JobStatus to Successful
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneMultiNodesetDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())
			}

			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A dataplaneDeployment is created with duplicate service in deployment service override", func() {
		BeforeEach(func() {
			CreateDataplaneServicesWithSameServiceType(dataplaneServiceName)
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			// DefaultDataPlanenodeSetSpec comes with two mock services, one marked for deployment on all nodesets
			// But we will not create them to test this scenario
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNodeSetSpec(dataplaneNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DuplicateServiceDeploymentSpec()))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)
		})
		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneDeploymentName)
			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              []string{"edpm-compute-nodeset"},
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      []string{dataplaneServiceName.Name, "duplicate-service"},
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set to true", func() {
			baremetal := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      dataplaneNodeSetName.Name,
					Namespace: dataplaneNodeName.Namespace,
				},
			}
			// Set baremetal provisioning conditions to True
			// This must succeed, as the "alpha-nodeset" exists
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, dataplaneNodeSetName, &baremetal)).To(Succeed())
				baremetal.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetal)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)
			service := GetService(dataplaneServiceName)
			aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(service,
				dataplaneDeploymentName.Name, dataplaneNodeSetName.Name)
			ansibleeeName := types.NamespacedName{
				Name:      aeeName,
				Namespace: dataplaneDeploymentName.Namespace,
			}
			Eventually(func(g Gomega) {
				ansibleEE := GetAnsibleee(ansibleeeName)
				ansibleEE.Status.Succeeded = 1
				g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
			}, th.Timeout, th.Interval).Should(Succeed())

			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A dataplaneDeployment is created with serviceoverride containing single global service", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))

			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			// Three services on both nodesets
			CreateDataplaneService(dataplaneServiceName, false)
			CreateDataplaneService(dataplaneGlobalServiceName, true)
			CreateDataPlaneServiceFromSpec(dataplaneUpdateServiceName, map[string]interface{}{
				"EDPMServiceType": "foo-update-service"})

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
			DeferCleanup(th.DeleteService, dataplaneUpdateServiceName)

			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			// Create both nodesets

			betaNodeName := fmt.Sprintf("%s-node-1", betaNodeSetName.Name)
			betaNodeSetSpec := map[string]interface{}{
				"preProvisioned": false,
				"services": []string{
					"foo-service",
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				},
				"nodes": map[string]interface{}{
					betaNodeName: map[string]interface{}{
						"hostname": betaNodeName,
						"networks": []map[string]interface{}{{
							"name":       "CtlPlane",
							"subnetName": "subnet1",
						},
						},
					},
				},
				"baremetalSetTemplate": map[string]interface{}{
					"baremetalHosts": map[string]interface{}{
						"ctlPlaneIP": map[string]interface{}{},
					},
					"deploymentSSHSecret": "dataplane-ansible-ssh-private-key-secret",
					"ctlplaneInterface":   "172.20.12.1",
				},
				"tlsEnabled": true,
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(alphaNodeSetName, DefaultDataPlaneNodeSetSpec(alphaNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(betaNodeSetName, betaNodeSetSpec))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(alphaNodeSetName)
			SimulateIPSetComplete(types.NamespacedName{Name: betaNodeName, Namespace: namespace})
			SimulateDNSDataComplete(betaNodeSetName)

			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneMultiNodesetDeploymentName, SingleGlobalServiceDeploymentSpec()))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
			nodeSetsNames := []string{
				"alpha-nodeset",
				"beta-nodeset",
			}

			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              nodeSetsNames,
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      []string{"global-service"},
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set", func() {
			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			baremetalAlpha := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      alphaNodeSetName.Name,
					Namespace: alphaNodeSetName.Namespace,
				},
			}

			baremetalBeta := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      betaNodeSetName.Name,
					Namespace: betaNodeSetName.Namespace,
				},
			}

			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSetAlpha := *GetDataplaneNodeSet(alphaNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, alphaNodeSetName, &baremetalAlpha)).To(Succeed())
				baremetalAlpha.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalAlpha)).To(Succeed())
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, betaNodeSetName, &baremetalBeta)).To(Succeed())
				baremetalBeta.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalBeta)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			// Create all services necessary for deployment
			for _, serviceName := range []string{"global-service"} {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSetAlpha.GetName())
				//Retrieve service AnsibleEE and set JobStatus to Successful
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneMultiNodesetDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())
			}
			th.ExpectCondition(
				betaNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				alphaNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A dataplaneDeployment is created with serviceoverride containing global service", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))

			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			// Three services on both nodesets
			CreateDataplaneService(dataplaneServiceName, false)
			CreateDataplaneService(dataplaneGlobalServiceName, true)
			CreateDataPlaneServiceFromSpec(dataplaneUpdateServiceName, map[string]interface{}{
				"EDPMServiceType": "foo-update-service"})

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
			DeferCleanup(th.DeleteService, dataplaneUpdateServiceName)

			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			// Create both nodesets

			betaNodeName := fmt.Sprintf("%s-node-1", betaNodeSetName.Name)
			betaNodeSetSpec := map[string]interface{}{
				"preProvisioned": false,
				"services": []string{
					"foo-service",
				},
				"nodeTemplate": map[string]interface{}{
					"ansibleSSHPrivateKeySecret": "dataplane-ansible-ssh-private-key-secret",
					"ansible": map[string]interface{}{
						"ansibleUser": "cloud-user",
					},
				},
				"nodes": map[string]interface{}{
					betaNodeName: map[string]interface{}{
						"hostname": betaNodeName,
						"networks": []map[string]interface{}{{
							"name":       "CtlPlane",
							"subnetName": "subnet1",
						},
						},
					},
				},
				"baremetalSetTemplate": map[string]interface{}{
					"baremetalHosts": map[string]interface{}{
						"ctlPlaneIP": map[string]interface{}{},
					},
					"deploymentSSHSecret": "dataplane-ansible-ssh-private-key-secret",
					"ctlplaneInterface":   "172.20.12.1",
				},
				"tlsEnabled": true,
			}
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(alphaNodeSetName, DefaultDataPlaneNodeSetSpec(alphaNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(betaNodeSetName, betaNodeSetSpec))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(alphaNodeSetName)
			SimulateIPSetComplete(types.NamespacedName{Name: betaNodeName, Namespace: namespace})
			SimulateDNSDataComplete(betaNodeSetName)

			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneMultiNodesetDeploymentName, GlobalServiceDeploymentSpec()))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
			nodeSetsNames := []string{
				"alpha-nodeset",
				"beta-nodeset",
			}

			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              nodeSetsNames,
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      []string{"foo-service", "global-service", "foo-update-service"},
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set", func() {
			alphaNodeSetName := types.NamespacedName{
				Name:      "alpha-nodeset",
				Namespace: namespace,
			}
			betaNodeSetName := types.NamespacedName{
				Name:      "beta-nodeset",
				Namespace: namespace,
			}

			baremetalAlpha := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      alphaNodeSetName.Name,
					Namespace: alphaNodeSetName.Namespace,
				},
			}

			baremetalBeta := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      betaNodeSetName.Name,
					Namespace: betaNodeSetName.Namespace,
				},
			}

			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSetAlpha := *GetDataplaneNodeSet(alphaNodeSetName)
			nodeSetBeta := *GetDataplaneNodeSet(betaNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, alphaNodeSetName, &baremetalAlpha)).To(Succeed())
				baremetalAlpha.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalAlpha)).To(Succeed())
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, betaNodeSetName, &baremetalBeta)).To(Succeed())
				baremetalBeta.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetalBeta)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			// Create all services necessary for deployment
			for _, serviceName := range []string{"foo-service", "global-service", "foo-update-service"} {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSetAlpha.GetName())
				//Retrieve service AnsibleEE and set JobStatus to Successful
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneMultiNodesetDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())
			}

			servicesExcludingGlobal := []string{"foo-service", "foo-update-service"}
			// Create all services necessary for deployment
			for _, serviceName := range servicesExcludingGlobal {
				dataplaneServiceName := types.NamespacedName{
					Name:      serviceName,
					Namespace: namespace,
				}
				service := GetService(dataplaneServiceName)
				deployment := GetDataplaneDeployment(dataplaneMultiNodesetDeploymentName)
				aeeName, _ := dataplaneutil.GetAnsibleExecutionNameAndLabels(
					service, deployment.GetName(), nodeSetBeta.GetName())

				//Retrieve service AnsibleEE and set JobStatus to Successful
				Eventually(func(g Gomega) {
					// Make an AnsibleEE name for each service
					ansibleeeName := types.NamespacedName{
						Name:      aeeName,
						Namespace: dataplaneMultiNodesetDeploymentName.Namespace,
					}
					ansibleEE := GetAnsibleee(ansibleeeName)
					ansibleEE.Status.Succeeded = 1
					g.Expect(th.K8sClient.Status().Update(th.Ctx, ansibleEE)).To(Succeed())
				}, th.Timeout, th.Interval).Should(Succeed())
			}

			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneMultiNodesetDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})
	})

	When("A dataplaneDeployment is created with non-existent service in nodeset", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			// DefaultDataPlanenodeSetSpec comes with two mock services, one marked for deployment on all nodesets
			// But we will not create them to test this scenario
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNodeSetSpec(dataplaneNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneDeploymentName)
			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              []string{"edpm-compute-nodeset"},
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				BackoffLimit:          &DefaultBackoffLimit,
				PreserveJobs:          true,
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have conditions set to false", func() {

			nodeSet := dataplanev1.OpenStackDataPlaneNodeSet{}
			baremetal := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeSet.Name,
					Namespace: nodeSet.Namespace,
				},
			}
			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSet = *GetDataplaneNodeSet(dataplaneNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, dataplaneNodeSetName, &baremetal)).To(Succeed())
				baremetal.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetal)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())
			// Attempt to get the service ... fail
			foundService := &dataplanev1.OpenStackDataPlaneService{}
			Expect(k8sClient.Get(ctx, dataplaneServiceName, foundService)).ShouldNot(Succeed())

			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionFalse,
			)
		})
	})

	When("A user sets TLSEnabled to true with control plane with PodLevel TLS disabled", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			// DefaultDataPlanenodeSetSpec comes with two mock services, one marked for deployment on all nodesets
			DeferCleanup(th.DeleteInstance, CreateDataplaneService(dataplaneServiceName, false))
			DeferCleanup(th.DeleteInstance, CreateDataplaneService(dataplaneGlobalServiceName, true))

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNodeSetSpec(dataplaneNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)

			DeferCleanup(th.DeleteInstance, CreateOpenStackControlPlane(controlPlaneName, GetDefaultOpenStackControlPlaneSpec(true, false)))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneDeploymentName)
			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              []string{"edpm-compute-nodeset"},
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
				BackoffLimit:          ptr.To(int32(6)),
				PreserveJobs:          true,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have ready condiction set to false and input condition set to unknown", func() {

			nodeSet := dataplanev1.OpenStackDataPlaneNodeSet{}
			baremetal := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeSet.Name,
					Namespace: nodeSet.Namespace,
				},
			}
			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSet = *GetDataplaneNodeSet(dataplaneNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, dataplaneNodeSetName, &baremetal)).To(Succeed())
				baremetal.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetal)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionUnknown,
			)
		})

	})

	When("A user sets TLSEnabled to true with control plane PodLevel TLS enabled", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			// DefaultDataPlanenodeSetSpec comes with two mock services, one marked for deployment on all nodesets
			DeferCleanup(th.DeleteInstance, CreateDataplaneService(dataplaneServiceName, false))
			DeferCleanup(th.DeleteInstance, CreateDataplaneService(dataplaneUpdateServiceName, false))
			CreateDataplaneService(dataplaneGlobalServiceName, true)

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNodeSetSpec(dataplaneNodeSetName.Name)))
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)

			DeferCleanup(th.DeleteInstance, CreateOpenStackControlPlane(controlPlaneName, GetDefaultOpenStackControlPlaneSpec(true, true)))
		})

		It("Should have Spec fields initialized", func() {
			dataplaneDeploymentInstance := GetDataplaneDeployment(dataplaneDeploymentName)
			expectedSpec := dataplanev1.OpenStackDataPlaneDeploymentSpec{
				NodeSets:              []string{"edpm-compute-nodeset"},
				AnsibleTags:           "",
				AnsibleLimit:          "",
				AnsibleSkipTags:       "",
				DeploymentRequeueTime: 15,
				ServicesOverride:      nil,
				BackoffLimit:          ptr.To(int32(6)),
				PreserveJobs:          true,
			}
			Expect(dataplaneDeploymentInstance.Spec).Should(Equal(expectedSpec))
		})

		It("should have ready condiction set to false, input condition set to true and nodeset setup ready condition set to true", func() {

			nodeSet := dataplanev1.OpenStackDataPlaneNodeSet{}
			baremetal := baremetalv1.OpenStackBaremetalSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      nodeSet.Name,
					Namespace: nodeSet.Namespace,
				},
			}
			// Create config map for OVN service
			ovnConfigMapName := types.NamespacedName{
				Namespace: namespace,
				Name:      "ovncontroller-config",
			}
			mapData := map[string]interface{}{
				"ovsdb-config": "test-ovn-config",
			}
			th.CreateConfigMap(ovnConfigMapName, mapData)

			nodeSet = *GetDataplaneNodeSet(dataplaneNodeSetName)

			// Set baremetal provisioning conditions to True
			Eventually(func(g Gomega) {
				// OpenStackBaremetalSet has the same name as OpenStackDataPlaneNodeSet
				g.Expect(th.K8sClient.Get(th.Ctx, dataplaneNodeSetName, &baremetal)).To(Succeed())
				baremetal.Status.Conditions.MarkTrue(
					condition.ReadyCondition,
					condition.ReadyMessage)
				g.Expect(th.K8sClient.Status().Update(th.Ctx, &baremetal)).To(Succeed())

			}, th.Timeout, th.Interval).Should(Succeed())

			th.ExpectCondition(
				dataplaneNodeSetName,
				ConditionGetterFunc(DataplaneConditionGetter),
				dataplanev1.SetupReadyCondition,
				corev1.ConditionTrue,
			)
			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.ReadyCondition,
				corev1.ConditionFalse,
			)
			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.InputReadyCondition,
				corev1.ConditionTrue,
			)
		})

	})

	When("A dataplaneDeployment fails with backoff limit exceeded", func() {
		BeforeEach(func() {
			CreateSSHSecret(dataplaneSSHSecretName)
			CreateCABundleSecret(caBundleSecretName)
			DeferCleanup(th.DeleteInstance, th.CreateSecret(neutronOvnMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaNeutronMetadataSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaCellComputeConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(novaMigrationSSHKey, map[string][]byte{
				"ssh-privatekey": []byte("fake-ssh-private-key"),
				"ssh-publickey":  []byte("fake-ssh-public-key"),
			}))
			DeferCleanup(th.DeleteInstance, th.CreateSecret(ceilometerConfigSecretName, map[string][]byte{
				"fake_keys": []byte("blih"),
			}))
			CreateDataplaneService(dataplaneServiceName, false)
			CreateDataplaneService(dataplaneGlobalServiceName, true)

			DeferCleanup(th.DeleteService, dataplaneServiceName)
			DeferCleanup(th.DeleteService, dataplaneGlobalServiceName)
			DeferCleanup(th.DeleteInstance, CreateNetConfig(dataplaneNetConfigName, DefaultNetConfigSpec()))
			DeferCleanup(th.DeleteInstance, CreateDNSMasq(dnsMasqName, DefaultDNSMasqSpec()))
			SimulateDNSMasqComplete(dnsMasqName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneNodeSet(dataplaneNodeSetName, DefaultDataPlaneNodeSetSpec(dataplaneNodeSetName.Name)))
			SimulateIPSetComplete(dataplaneNodeName)
			SimulateDNSDataComplete(dataplaneNodeSetName)
			DeferCleanup(th.DeleteInstance, CreateDataplaneDeployment(dataplaneDeploymentName, DefaultDataPlaneDeploymentSpec()))
		})

		It("should not reconcile after failure", func() {
			// Directly set the deployment status to backoff limit exceeded
			// This simulates a deployment that has failed after retries
			Eventually(func(g Gomega) {
				deployment := GetDataplaneDeployment(dataplaneDeploymentName)

				// Initialize conditions if not present
				if deployment.Status.Conditions == nil {
					deployment.Status.Conditions = condition.Conditions{}
				}

				// Initialize NodeSetConditions if not present
				if deployment.Status.NodeSetConditions == nil {
					deployment.Status.NodeSetConditions = make(map[string]condition.Conditions)
				}

				// Set the deployment condition to backoff limit exceeded
				deployment.Status.Conditions.MarkFalse(
					condition.DeploymentReadyCondition,
					condition.JobReasonBackoffLimitExceeded,
					condition.SeverityError,
					condition.DeploymentReadyErrorMessage,
					"Simulated backoff limit exceeded for testing")

				deployment.Status.Deployed = false
				deployment.Status.ObservedGeneration = deployment.Generation

				g.Expect(th.K8sClient.Status().Update(th.Ctx, deployment)).To(Succeed())
			}, th.Timeout, th.Interval).Should(Succeed())

			// Verify the deployment has the correct failure condition
			deployment := GetDataplaneDeployment(dataplaneDeploymentName)
			deploymentCondition := deployment.Status.Conditions.Get(condition.DeploymentReadyCondition)
			Expect(deploymentCondition).ToNot(BeNil())
			Expect(string(deploymentCondition.Severity)).To(Equal(string(condition.SeverityError)))
			Expect(string(deploymentCondition.Reason)).To(Equal(string(condition.JobReasonBackoffLimitExceeded)))

			// Verify the status is not deployed
			deployment = GetDataplaneDeployment(dataplaneDeploymentName)
			Expect(deployment.Status.Deployed).To(BeFalse())

			// Store the current ObservedGeneration and condition timestamp
			originalGeneration := deployment.Status.ObservedGeneration
			deploymentCondition = deployment.Status.Conditions.Get(condition.DeploymentReadyCondition)
			originalTransitionTime := deploymentCondition.LastTransitionTime

			// Trigger reconciliation by updating the deployment annotation
			// This should normally trigger reconciliation, but should be blocked by our early return check
			Eventually(func(g Gomega) {
				deployment := GetDataplaneDeployment(dataplaneDeploymentName)
				if deployment.Annotations == nil {
					deployment.Annotations = make(map[string]string)
				}
				deployment.Annotations["test-annotation"] = "trigger-reconcile"
				g.Expect(th.K8sClient.Update(th.Ctx, deployment)).To(Succeed())
			}, th.Timeout, th.Interval).Should(Succeed())

			// Wait a bit to ensure reconciliation would have been triggered if it was going to
			th.Logger.Info("Waiting to ensure no reconciliation occurs...")

			// Verify the deployment status hasn't changed after annotation update
			// The ObservedGeneration and condition LastTransitionTime should remain the same
			Consistently(func(g Gomega) {
				deployment := GetDataplaneDeployment(dataplaneDeploymentName)
				g.Expect(deployment.Status.ObservedGeneration).To(Equal(originalGeneration))
				deploymentCondition := deployment.Status.Conditions.Get(condition.DeploymentReadyCondition)
				g.Expect(deploymentCondition).ToNot(BeNil())
				g.Expect(string(deploymentCondition.Severity)).To(Equal(string(condition.SeverityError)))
				g.Expect(string(deploymentCondition.Reason)).To(Equal(string(condition.JobReasonBackoffLimitExceeded)))
				g.Expect(deploymentCondition.LastTransitionTime).To(Equal(originalTransitionTime))
				g.Expect(deployment.Status.Deployed).To(BeFalse())
			}, "5s", "1s").Should(Succeed())

			th.ExpectCondition(
				dataplaneDeploymentName,
				ConditionGetterFunc(DataplaneDeploymentConditionGetter),
				condition.DeploymentReadyCondition,
				corev1.ConditionFalse,
			)
		})
	})

})
