/*
Copyright 2026.

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
	"testing"

	"github.com/onsi/gomega"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"
)

func TestMergeCABundles(t *testing.T) {
	g := gomega.NewWithT(t)

	g.Expect(mergeCABundles([]byte("source\n"), []byte("service\n"))).To(
		gomega.Equal([]byte("source\nservice\n")))
	g.Expect(mergeCABundles([]byte("source\nservice\n"), []byte("service\n"))).To(
		gomega.Equal([]byte("source\nservice\n")))
	g.Expect(mergeCABundles([]byte("source"), nil)).To(
		gomega.Equal([]byte("source\n")))
}

func TestReconcileCABundle(t *testing.T) {
	g := gomega.NewWithT(t)
	ctx := context.Background()
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(assistantv1.AddToScheme(scheme)).To(gomega.Succeed())

	instance := &assistantv1.OpenStackAssistant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "assistant",
			Namespace: "openstack",
			UID:       types.UID("assistant-uid"),
		},
		Spec: assistantv1.OpenStackAssistantSpec{
			Ca: tls.Ca{CaBundleSecretName: "source-ca"},
		},
	}
	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "source-ca", Namespace: "openstack"},
		Data: map[string][]byte{
			tls.CABundleKey: []byte("source-ca\n"),
		},
	}
	serviceCA := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openShiftServiceCAConfigMapName,
			Namespace: "openstack",
		},
		Data: map[string]string{
			openShiftServiceCAKey: "service-ca\n",
		},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance, source, serviceCA).
		Build()
	reconciler := &OpenStackAssistantReconciler{Client: client, Scheme: scheme}

	name, initialHash, err := reconciler.reconcileCABundle(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(name).To(gomega.Equal(assistantCABundleSecretName(instance)))
	g.Expect(initialHash).NotTo(gomega.BeEmpty())

	generated := &corev1.Secret{}
	g.Expect(client.Get(ctx, types.NamespacedName{Name: name, Namespace: "openstack"}, generated)).To(gomega.Succeed())
	g.Expect(generated.Data).To(gomega.Equal(map[string][]byte{
		tls.CABundleKey: []byte("source-ca\nservice-ca\n"),
	}))
	g.Expect(generated.OwnerReferences).To(gomega.HaveLen(1))
	g.Expect(generated.OwnerReferences[0].UID).To(gomega.Equal(instance.UID))
	g.Expect(generated.OwnerReferences[0].Controller).NotTo(gomega.BeNil())
	g.Expect(*generated.OwnerReferences[0].Controller).To(gomega.BeTrue())

	serviceCA.Data[openShiftServiceCAKey] = "rotated-service-ca\n"
	g.Expect(client.Update(ctx, serviceCA)).To(gomega.Succeed())
	_, rotatedHash, err := reconciler.reconcileCABundle(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())
	g.Expect(rotatedHash).NotTo(gomega.Equal(initialHash))

	g.Expect(client.Get(ctx, types.NamespacedName{Name: name, Namespace: "openstack"}, generated)).To(gomega.Succeed())
	g.Expect(generated.Data[tls.CABundleKey]).To(gomega.Equal([]byte("source-ca\nrotated-service-ca\n")))
}

func TestReconcileCABundleWithoutOpenShiftServiceCA(t *testing.T) {
	g := gomega.NewWithT(t)
	ctx := context.Background()
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(assistantv1.AddToScheme(scheme)).To(gomega.Succeed())

	instance := &assistantv1.OpenStackAssistant{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "assistant",
			Namespace: "openstack",
			UID:       types.UID("assistant-uid"),
		},
		Spec: assistantv1.OpenStackAssistantSpec{
			Ca: tls.Ca{CaBundleSecretName: "source-ca"},
		},
	}
	source := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "source-ca", Namespace: "openstack"},
		Data: map[string][]byte{
			tls.CABundleKey: []byte("source-ca\n"),
		},
	}

	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance, source).Build()
	reconciler := &OpenStackAssistantReconciler{Client: client, Scheme: scheme}

	name, _, err := reconciler.reconcileCABundle(ctx, instance)
	g.Expect(err).NotTo(gomega.HaveOccurred())

	generated := &corev1.Secret{}
	g.Expect(client.Get(ctx, types.NamespacedName{Name: name, Namespace: "openstack"}, generated)).To(gomega.Succeed())
	g.Expect(generated.Data[tls.CABundleKey]).To(gomega.Equal([]byte("source-ca\n")))
}

func TestFindObjectsForSrcEnqueuesAssistantsForServiceCA(t *testing.T) {
	g := gomega.NewWithT(t)
	ctx := context.Background()
	scheme := runtime.NewScheme()
	g.Expect(corev1.AddToScheme(scheme)).To(gomega.Succeed())
	g.Expect(assistantv1.AddToScheme(scheme)).To(gomega.Succeed())

	first := &assistantv1.OpenStackAssistant{
		ObjectMeta: metav1.ObjectMeta{Name: "first", Namespace: "openstack"},
	}
	second := &assistantv1.OpenStackAssistant{
		ObjectMeta: metav1.ObjectMeta{Name: "second", Namespace: "openstack"},
	}
	otherNamespace := &assistantv1.OpenStackAssistant{
		ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "other"},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(first, second, otherNamespace).
		Build()
	reconciler := &OpenStackAssistantReconciler{Client: client, Scheme: scheme}
	serviceCA := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      openShiftServiceCAConfigMapName,
			Namespace: "openstack",
		},
	}

	requests := reconciler.findObjectsForSrc(ctx, serviceCA)
	g.Expect(requests).To(gomega.ConsistOf(
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "first", Namespace: "openstack"}},
		reconcile.Request{NamespacedName: types.NamespacedName{Name: "second", Namespace: "openstack"}},
	))
}
