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
	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	k8s_corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	commonbackup "github.com/openstack-k8s-operators/lib-common/modules/common/backup"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	backupv1 "github.com/openstack-k8s-operators/openstack-operator/api/backup/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
)

func GetOpenStackBackupConfig(name types.NamespacedName) *backupv1.OpenStackBackupConfig {
	instance := &backupv1.OpenStackBackupConfig{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func OpenStackBackupConfigConditionGetter(name types.NamespacedName) condition.Conditions {
	instance := GetOpenStackBackupConfig(name)
	return instance.Status.Conditions
}

func backupLabelingPtr(p backupv1.BackupLabelingPolicy) *backupv1.BackupLabelingPolicy {
	return &p
}

func CreateBackupConfig(name types.NamespacedName) *backupv1.OpenStackBackupConfig {
	backupConfig := &backupv1.OpenStackBackupConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name.Name,
			Namespace: name.Namespace,
		},
		Spec: backupv1.OpenStackBackupConfigSpec{
			// Kubebuilder defaults are only applied via webhooks.
			// Set them explicitly for envtest.
			DefaultRestoreOrder: "10",
			Secrets: backupv1.ResourceBackupConfig{
				Labeling: backupLabelingPtr(backupv1.BackupLabelingEnabled),
			},
			ConfigMaps: backupv1.ResourceBackupConfig{
				Labeling:     backupLabelingPtr(backupv1.BackupLabelingEnabled),
				ExcludeNames: []string{"kube-root-ca.crt", "openshift-service-ca.crt"},
			},
			Issuers: backupv1.ResourceBackupConfig{
				Labeling:     backupLabelingPtr(backupv1.BackupLabelingEnabled),
				RestoreOrder: "20",
			},
			NetworkAttachmentDefinitions: backupv1.ResourceBackupConfig{
				Labeling: backupLabelingPtr(backupv1.BackupLabelingEnabled),
			},
		},
	}
	Expect(k8sClient.Create(ctx, backupConfig)).Should(Succeed())
	return backupConfig
}

var _ = Describe("OpenStackBackupConfig controller", func() {
	var backupConfigName types.NamespacedName

	When("A OpenStackBackupConfig is created", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-config",
				Namespace: namespace,
			}

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should exist and be retrievable", func() {
			backupConfig := &backupv1.OpenStackBackupConfig{}
			Expect(k8sClient.Get(ctx, backupConfigName, backupConfig)).Should(Succeed())
			Expect(backupConfig.Namespace).To(Equal(namespace))
		})

		It("Should initialize all conditions", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigSecretsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigConfigMapsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigNADsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigIssuersReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigCRsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})

		It("Should become Ready when all sub-conditions are True", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})
	})

	When("A secret without ownerRef exists in the namespace", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-secrets",
				Namespace: namespace,
			}

			// Create a user-provided secret (no ownerRef)
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"key": []byte("value"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should label the secret for backup", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "user-secret", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"))
			}, timeout, interval).Should(Succeed())
		})

		It("Should set SecretsReady condition to True", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigSecretsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})

		It("Should update status counts", func() {
			Eventually(func(g Gomega) {
				backupConfig := GetOpenStackBackupConfig(backupConfigName)
				g.Expect(backupConfig.Status.LabeledResources.Secrets).To(BeNumerically(">=", 1))
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A secret already has a restore label", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-existing-restore-label",
				Namespace: namespace,
			}

			// Create a secret with restore=false (as set by controlplane controller for leaf certs)
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cert-secret",
					Namespace: namespace,
					Labels: map[string]string{
						commonbackup.BackupRestoreLabel: "false",
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte("cert"),
				},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should not overwrite the existing restore label", func() {
			// Wait for reconciliation to complete
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)

			// Verify the restore label was preserved as "false"
			secret := &k8s_corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: "cert-secret", Namespace: namespace,
			}, secret)).Should(Succeed())

			labels := secret.GetLabels()
			Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("false"))
			// Should NOT have backup or restore-order labels
			Expect(labels[commonbackup.BackupLabel]).To(BeEmpty())
		})
	})

	When("A configmap without ownerRef exists in the namespace", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-configmaps",
				Namespace: namespace,
			}

			// Create a user-provided configmap (no ownerRef)
			cm := &k8s_corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "user-configmap",
					Namespace: namespace,
				},
				Data: map[string]string{
					"key": "value",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, cm)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should label the configmap for backup", func() {
			Eventually(func(g Gomega) {
				cm := &k8s_corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "user-configmap", Namespace: namespace,
				}, cm)).Should(Succeed())

				labels := cm.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"))
			}, timeout, interval).Should(Succeed())
		})

		It("Should set ConfigMapsReady condition to True", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigConfigMapsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})
	})

	When("An excluded configmap exists in the namespace", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-exclude-cm",
				Namespace: namespace,
			}

			// Create a system configmap (excluded by default)
			cm := &k8s_corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "kube-root-ca.crt",
					Namespace: namespace,
				},
				Data: map[string]string{
					"ca.crt": "system-ca",
				},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, cm)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should not label the excluded configmap", func() {
			// Wait for reconciliation
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)

			cm := &k8s_corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: "kube-root-ca.crt", Namespace: namespace,
			}, cm)).Should(Succeed())

			labels := cm.GetLabels()
			Expect(labels[commonbackup.BackupRestoreLabel]).To(BeEmpty())
		})
	})

	When("A custom cert-manager Issuer without ownerRef exists", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-issuers",
				Namespace: namespace,
			}

			// Create a custom Issuer (no ownerRef - user-provided)
			issuer := &certmgrv1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-custom-issuer",
					Namespace: namespace,
				},
				Spec: certmgrv1.IssuerSpec{
					IssuerConfig: certmgrv1.IssuerConfig{
						SelfSigned: &certmgrv1.SelfSignedIssuer{},
					},
				},
			}
			Expect(k8sClient.Create(ctx, issuer)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, issuer)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should label the custom issuer for backup", func() {
			Eventually(func(g Gomega) {
				issuer := &certmgrv1.Issuer{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "my-custom-issuer", Namespace: namespace,
				}, issuer)).Should(Succeed())

				labels := issuer.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"))
			}, timeout, interval).Should(Succeed())
		})

		It("Should set IssuersReady condition to True", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigIssuersReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})

		It("Should update issuer count in status", func() {
			Eventually(func(g Gomega) {
				backupConfig := GetOpenStackBackupConfig(backupConfigName)
				g.Expect(backupConfig.Status.LabeledResources.Issuers).To(BeNumerically(">=", 1))
			}, timeout, interval).Should(Succeed())
		})
	})

	When("An operator-created Issuer with ownerRef exists", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-issuer-ownerref",
				Namespace: namespace,
			}

			// Create an Issuer with ownerRef (simulating operator-created)
			issuer := &certmgrv1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rootca-internal",
					Namespace: namespace,
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "core.openstack.org/v1beta1",
							Kind:       "OpenStackControlPlane",
							Name:       "controlplane",
							UID:        "fake-uid",
						},
					},
				},
				Spec: certmgrv1.IssuerSpec{
					IssuerConfig: certmgrv1.IssuerConfig{
						CA: &certmgrv1.CAIssuer{
							SecretName: "rootca-internal",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, issuer)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, issuer)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should not label the operator-created issuer", func() {
			// Wait for reconciliation
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)

			issuer := &certmgrv1.Issuer{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: "rootca-internal", Namespace: namespace,
			}, issuer)).Should(Succeed())

			labels := issuer.GetLabels()
			Expect(labels[commonbackup.BackupRestoreLabel]).To(BeEmpty())
		})
	})

	When("OpenStackBackupConfig reconciles with CRs in namespace", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-with-crs",
				Namespace: namespace,
			}

			// Create OpenStackControlPlane (CRD has backup-restore labels)
			controlPlaneName := types.NamespacedName{
				Name:      "test-controlplane",
				Namespace: namespace,
			}
			spec := GetDefaultOpenStackControlPlaneSpec()
			CreateOpenStackControlPlane(controlPlaneName, spec)
			DeferCleanup(th.DeleteInstance, GetOpenStackControlPlane(controlPlaneName))

			// Create OpenStackBackupConfig after CRs exist
			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should label CR instances with backup labels", func() {
			controlPlaneName := types.NamespacedName{
				Name:      "test-controlplane",
				Namespace: namespace,
			}

			Eventually(func(g Gomega) {
				controlPlane := &corev1.OpenStackControlPlane{}
				g.Expect(k8sClient.Get(ctx, controlPlaneName, controlPlane)).Should(Succeed())

				labels := controlPlane.GetLabels()
				g.Expect(labels).NotTo(BeNil(), "ControlPlane should have labels")
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(
					Equal("true"),
					"ControlPlane should have backup label",
				)
				g.Expect(labels[commonbackup.BackupRestoreOrderLabel]).To(
					Equal("30"),
					"ControlPlane should have restore-order label from CRD",
				)
			}, timeout, interval).Should(Succeed())
		})

		It("Should set CRsReady condition to True", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigCRsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})

		It("Should set ReadyCondition to True when all resources are labeled", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})
	})

	When("A CA cert secret already labeled for restore exists", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-ca-cert-secret",
				Namespace: namespace,
			}

			// Create a CA cert secret with restore labels (as set by controlplane controller)
			caSecret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rootca-internal",
					Namespace: namespace,
					Labels: map[string]string{
						commonbackup.BackupRestoreLabel:      "true",
						commonbackup.BackupRestoreOrderLabel: "10",
						commonbackup.BackupLabel:             "true",
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte("ca-cert"),
					"tls.key": []byte("ca-key"),
				},
			}
			Expect(k8sClient.Create(ctx, caSecret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, caSecret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should preserve the existing restore labels set by the controlplane controller", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)

			secret := &k8s_corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: "rootca-internal", Namespace: namespace,
			}, secret)).Should(Succeed())

			labels := secret.GetLabels()
			Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"),
				"CA cert secret restore label must be preserved")
			Expect(labels[commonbackup.BackupRestoreOrderLabel]).To(Equal("10"),
				"CA cert secret restore order must be preserved")
		})
	})

	When("A leaf cert secret already labeled restore=false exists", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-leaf-cert-secret",
				Namespace: namespace,
			}

			// Create a leaf cert secret with restore=false (as set by controlplane controller)
			leafSecret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cert-keystone-internal-svc",
					Namespace: namespace,
					Labels: map[string]string{
						commonbackup.BackupRestoreLabel: "false",
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte("leaf-cert"),
					"tls.key": []byte("leaf-key"),
				},
			}
			Expect(k8sClient.Create(ctx, leafSecret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, leafSecret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should not overwrite the restore=false label set by the controlplane controller", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)

			secret := &k8s_corev1.Secret{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: "cert-keystone-internal-svc", Namespace: namespace,
			}, secret)).Should(Succeed())

			labels := secret.GetLabels()
			Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("false"),
				"Leaf cert secret should keep restore=false")
			Expect(labels).NotTo(HaveKey(commonbackup.BackupRestoreOrderLabel),
				"restore-order should not be set when restore=false")
		})
	})

	When("A user-provided secret without a restore label exists", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-user-cert-secret",
				Namespace: namespace,
			}

			// Create a user-provided secret (no restore label)
			userSecret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "my-custom-cert-tls",
					Namespace: namespace,
				},
				Data: map[string][]byte{
					"tls.crt": []byte("user-cert"),
					"tls.key": []byte("user-key"),
				},
			}
			Expect(k8sClient.Create(ctx, userSecret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, userSecret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should label the user-provided secret for restore", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "my-custom-cert-tls", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"),
					"User-provided secret should be labeled for restore")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A secret has a restore annotation override set to false", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-annotation-false",
				Namespace: namespace,
			}

			// Create a secret with annotation override restore=false
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "override-skip-secret",
					Namespace: namespace,
					Annotations: map[string]string{
						commonbackup.BackupRestoreLabel: "false",
					},
				},
				Data: map[string][]byte{"key": []byte("value")},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should sync the annotation to label restore=false", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "override-skip-secret", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("false"),
					"Annotation override restore=false should be synced to label")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A secret has a restore annotation override set to true", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-annotation-true",
				Namespace: namespace,
			}

			// Create a secret that would normally be excluded (has ownerRef)
			// but has annotation override restore=true
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "override-restore-secret",
					Namespace: namespace,
					Annotations: map[string]string{
						commonbackup.BackupRestoreLabel: "true",
					},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: "core.openstack.org/v1beta1",
							Kind:       "OpenStackControlPlane",
							Name:       "controlplane",
							UID:        "fake-uid",
						},
					},
				},
				Data: map[string][]byte{"key": []byte("value")},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should sync the annotation to label restore=true with default restore-order even with ownerRef", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "override-restore-secret", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"),
					"Annotation override restore=true should be synced to label, overriding ownerRef exclusion")
				g.Expect(labels[commonbackup.BackupRestoreOrderLabel]).NotTo(BeEmpty(),
					"restore-order should be set to default when restore=true via annotation")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A secret has a restore-order annotation override", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-annotation-order",
				Namespace: namespace,
			}

			// Create a secret with annotation override for restore order
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "custom-order-secret",
					Namespace: namespace,
					Annotations: map[string]string{
						commonbackup.BackupRestoreLabel:      "true",
						commonbackup.BackupRestoreOrderLabel: "05",
					},
				},
				Data: map[string][]byte{"key": []byte("value")},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should sync both restore and restore-order annotations to labels", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "custom-order-secret", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"))
				g.Expect(labels[commonbackup.BackupRestoreOrderLabel]).To(Equal("05"),
					"Annotation override restore-order=05 should be synced to label")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A secret with restore=false label has annotation override restore=true", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-cert-override",
				Namespace: namespace,
			}

			// Create a secret with restore=false label but annotation override to force restore
			leafSecret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cert-keystone-override-svc",
					Namespace: namespace,
					Labels: map[string]string{
						commonbackup.BackupRestoreLabel: "false",
					},
					Annotations: map[string]string{
						commonbackup.BackupRestoreLabel: "true",
					},
				},
				Data: map[string][]byte{
					"tls.crt": []byte("leaf-cert"),
					"tls.key": []byte("leaf-key"),
				},
			}
			Expect(k8sClient.Create(ctx, leafSecret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, leafSecret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should honor annotation override and set restore=true with default restore-order", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "cert-keystone-override-svc", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"),
					"Annotation override should take precedence over operator-set label")
				g.Expect(labels[commonbackup.BackupRestoreOrderLabel]).NotTo(BeEmpty(),
					"restore-order should be set to default when restore=true via annotation")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A secret has only a restore-order annotation override", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-annotation-order-only",
				Namespace: namespace,
			}

			// Create a secret with only restore-order annotation (no restore annotation)
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "order-only-secret",
					Namespace: namespace,
					Annotations: map[string]string{
						commonbackup.BackupRestoreOrderLabel: "05",
					},
				},
				Data: map[string][]byte{"key": []byte("value")},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should imply restore=true and use the specified restore-order", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "order-only-secret", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"),
					"restore-order annotation should imply restore=true")
				g.Expect(labels[commonbackup.BackupRestoreOrderLabel]).To(Equal("05"),
					"restore-order should use the annotation value")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("A secret has annotation override with mixed case value", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-annotation-case",
				Namespace: namespace,
			}

			// Create a secret with mixed-case annotation value
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "mixed-case-secret",
					Namespace: namespace,
					Annotations: map[string]string{
						commonbackup.BackupRestoreLabel: "True",
					},
				},
				Data: map[string][]byte{"key": []byte("value")},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should normalize the annotation value to lowercase in the label", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "mixed-case-secret", Namespace: namespace,
				}, secret)).Should(Succeed())

				labels := secret.GetLabels()
				g.Expect(labels).NotTo(BeNil())
				g.Expect(labels[commonbackup.BackupRestoreLabel]).To(Equal("true"),
					"Mixed case 'True' annotation should be normalized to 'true' label")
			}, timeout, interval).Should(Succeed())
		})
	})

	When("Multiple resource types exist in the namespace", func() {
		BeforeEach(func() {
			backupConfigName = types.NamespacedName{
				Name:      "test-backup-multi",
				Namespace: namespace,
			}

			// Create a user secret
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-test-secret",
					Namespace: namespace,
				},
				Data: map[string][]byte{"key": []byte("val")},
			}
			Expect(k8sClient.Create(ctx, secret)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, secret)

			// Create a user configmap
			cm := &k8s_corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-test-cm",
					Namespace: namespace,
				},
				Data: map[string]string{"key": "val"},
			}
			Expect(k8sClient.Create(ctx, cm)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, cm)

			// Create a custom issuer
			issuer := &certmgrv1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multi-test-issuer",
					Namespace: namespace,
				},
				Spec: certmgrv1.IssuerSpec{
					IssuerConfig: certmgrv1.IssuerConfig{
						SelfSigned: &certmgrv1.SelfSignedIssuer{},
					},
				},
			}
			Expect(k8sClient.Create(ctx, issuer)).Should(Succeed())
			DeferCleanup(th.DeleteInstance, issuer)

			backupConfig := CreateBackupConfig(backupConfigName)
			DeferCleanup(th.DeleteInstance, backupConfig)
		})

		It("Should set all sub-conditions to True and ReadyCondition to True", func() {
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigSecretsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigConfigMapsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigNADsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigIssuersReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				backupv1.OpenStackBackupConfigCRsReadyCondition,
				k8s_corev1.ConditionTrue,
			)
			th.ExpectCondition(
				backupConfigName,
				ConditionGetterFunc(OpenStackBackupConfigConditionGetter),
				condition.ReadyCondition,
				k8s_corev1.ConditionTrue,
			)
		})

		It("Should label all resource types", func() {
			Eventually(func(g Gomega) {
				secret := &k8s_corev1.Secret{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "multi-test-secret", Namespace: namespace,
				}, secret)).Should(Succeed())
				g.Expect(secret.GetLabels()[commonbackup.BackupRestoreLabel]).To(Equal("true"))

				cm := &k8s_corev1.ConfigMap{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "multi-test-cm", Namespace: namespace,
				}, cm)).Should(Succeed())
				g.Expect(cm.GetLabels()[commonbackup.BackupRestoreLabel]).To(Equal("true"))

				issuer := &certmgrv1.Issuer{}
				g.Expect(k8sClient.Get(ctx, types.NamespacedName{
					Name: "multi-test-issuer", Namespace: namespace,
				}, issuer)).Should(Succeed())
				g.Expect(issuer.GetLabels()[commonbackup.BackupRestoreLabel]).To(Equal("true"))
			}, timeout, interval).Should(Succeed())
		})
	})
})
