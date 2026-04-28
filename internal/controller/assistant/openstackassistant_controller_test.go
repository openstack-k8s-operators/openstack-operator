/*
Copyright 2022.

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

package assistant

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	assistantv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"
)

var _ = Describe("OpenStackAssistant Controller", func() {
	const resourceName = "test-assistant"
	const namespace = "default"
	const providerSecretName = "test-provider-secret"

	ctx := context.Background()

	typeNamespacedName := types.NamespacedName{
		Name:      resourceName,
		Namespace: namespace,
	}

	BeforeEach(func() {
		By("creating the provider secret")
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      providerSecretName,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				"lightspeed.json": []byte(`{"name":"lightspeed"}`),
			},
		}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: providerSecretName, Namespace: namespace}, &corev1.Secret{})
		if errors.IsNotFound(err) {
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		}
	})

	Context("When creating an OpenStackAssistant resource", func() {
		BeforeEach(func() {
			By("creating the OpenStackAssistant resource")
			resource := &assistantv1beta1.OpenStackAssistant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: namespace,
				},
				Spec: assistantv1beta1.OpenStackAssistantSpec{
					ContainerImage: "quay.io/dprince/goose:oc-fedora",
					Provider:       assistantv1beta1.ProviderGoose,
					LightspeedStack: assistantv1beta1.LightspeedStackSpec{
						ProviderSecret: providerSecretName,
					},
				},
			}
			err := k8sClient.Get(ctx, typeNamespacedName, &assistantv1beta1.OpenStackAssistant{})
			if errors.IsNotFound(err) {
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &assistantv1beta1.OpenStackAssistant{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			if err == nil {
				resource.Finalizers = nil
				Expect(k8sClient.Update(ctx, resource)).To(Succeed())
				Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			}
			// Clean up cluster-scoped resources
			clusterRoleName := "openstackassistant-" + namespace + "-" + resourceName
			cr := &rbacv1.ClusterRole{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: clusterRoleName}, cr); err == nil {
				_ = k8sClient.Delete(ctx, cr)
			}
			crb := &rbacv1.ClusterRoleBinding{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: clusterRoleName}, crb); err == nil {
				_ = k8sClient.Delete(ctx, crb)
			}
		})

		It("should add a finalizer on first reconcile", func() {
			reconciler := &OpenStackAssistantReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Kclient: kubernetes.NewForConfigOrDie(cfg),
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Requeue).To(BeTrue())

			updated := &assistantv1beta1.OpenStackAssistant{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, updated)).To(Succeed())
			Expect(updated.Finalizers).To(ContainElement(assistantFinalizer))
		})

		It("should create an entrypoint ConfigMap after reconciliation", func() {
			reconciler := &OpenStackAssistantReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Kclient: kubernetes.NewForConfigOrDie(cfg),
			}

			// First reconcile adds finalizer
			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			// Second reconcile does the actual work
			_, err = reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			cm := &corev1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name:      resourceName + "-entrypoint",
				Namespace: namespace,
			}, cm)).To(Succeed())
			Expect(cm.Data).To(HaveKey("entrypoint.sh"))
			Expect(cm.Data["entrypoint.sh"]).To(ContainSubstring("sleep infinity"))
		})

		It("should create a ClusterRole and ClusterRoleBinding", func() {
			reconciler := &OpenStackAssistantReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Kclient: kubernetes.NewForConfigOrDie(cfg),
			}

			// First reconcile adds finalizer
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			// Second reconcile creates resources
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			clusterRoleName := "openstackassistant-" + namespace + "-" + resourceName

			cr := &rbacv1.ClusterRole{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterRoleName}, cr)).To(Succeed())
			Expect(cr.Rules).NotTo(BeEmpty())

			hasNodesRule := false
			for _, rule := range cr.Rules {
				for _, resource := range rule.Resources {
					if resource == "nodes" {
						hasNodesRule = true
						break
					}
				}
			}
			Expect(hasNodesRule).To(BeTrue(), "ClusterRole should include nodes resource")

			crb := &rbacv1.ClusterRoleBinding{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: clusterRoleName}, crb)).To(Succeed())
			Expect(crb.RoleRef.Name).To(Equal(clusterRoleName))
			Expect(crb.Subjects).To(HaveLen(1))
			Expect(crb.Subjects[0].Name).To(Equal("openstackassistant-" + resourceName))
			Expect(crb.Subjects[0].Namespace).To(Equal(namespace))
		})

		It("should create a Pod", func() {
			reconciler := &OpenStackAssistantReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Kclient: kubernetes.NewForConfigOrDie(cfg),
			}

			// First reconcile adds finalizer
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			// Second reconcile creates resources
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			pod := &corev1.Pod{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, pod)).To(Succeed())
			Expect(pod.Spec.Containers).To(HaveLen(1))
			Expect(pod.Spec.Containers[0].Name).To(Equal("goose"))
			Expect(pod.Spec.Containers[0].Image).To(Equal("quay.io/dprince/goose:oc-fedora"))
			Expect(pod.Labels).To(HaveKeyWithValue("service", "openstackassistant"))
		})

		It("should set status conditions after reconciliation", func() {
			reconciler := &OpenStackAssistantReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Kclient: kubernetes.NewForConfigOrDie(cfg),
			}

			// First reconcile adds finalizer
			_, _ = reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			// Second reconcile creates resources
			_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: typeNamespacedName})
			Expect(err).NotTo(HaveOccurred())

			instance := &assistantv1beta1.OpenStackAssistant{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, instance)).To(Succeed())
			Expect(instance.Status.Conditions).NotTo(BeEmpty())
			Expect(instance.Status.PodName).To(Equal(resourceName))
			Expect(instance.Status.Hash).To(HaveKey("podSpec"))
		})
	})

	Context("When the CR does not exist", func() {
		It("should return no error", func() {
			reconciler := &OpenStackAssistantReconciler{
				Client:  k8sClient,
				Scheme:  k8sClient.Scheme(),
				Kclient: kubernetes.NewForConfigOrDie(cfg),
			}

			result, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      "nonexistent",
					Namespace: namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(reconcile.Result{}))
		})
	})
})
