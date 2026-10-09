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

package operator

import (
	"context"
	"testing"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	. "github.com/onsi/gomega" //revive:disable:dot-imports

	operatorv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/operator/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(operatorv1beta1.AddToScheme(s)).To(Succeed())
	g.Expect(certmgrv1.AddToScheme(s)).To(Succeed())
	return s
}

func readyCertificate(name, namespace string) *certmgrv1.Certificate {
	return &certmgrv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Generation: 1},
		Status: certmgrv1.CertificateStatus{
			Conditions: []certmgrv1.CertificateCondition{
				{Type: certmgrv1.CertificateConditionReady, Status: cmmeta.ConditionTrue, ObservedGeneration: 1},
			},
		},
	}
}

// TestCheckWebhookCertificates covers the cert-manager Certificate readiness gate that
// prevents the operator from waiting on deployments that can't start because their webhook
// certificate Secret hasn't been issued yet.
func TestCheckWebhookCertificates(t *testing.T) {
	ctx := context.Background()
	instance := &operatorv1beta1.OpenStack{
		ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: "openstack-operators"},
	}

	t.Run("requeues when a certificate is missing", func(t *testing.T) {
		g := NewWithT(t)
		s := newTestScheme(t)
		fakeClient := fakeclient.NewClientBuilder().WithScheme(s).WithObjects(instance).Build()
		r := &OpenStackReconciler{Client: fakeClient, Scheme: s}

		result, err := r.checkWebhookCertificates(ctx, instance)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
	})

	t.Run("requeues when a certificate exists but isn't ready", func(t *testing.T) {
		g := NewWithT(t)
		s := newTestScheme(t)
		notReady := &certmgrv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{Name: "openstack-operator-serving-cert", Namespace: "openstack-operators"},
		}
		objs := []client.Object{
			instance, notReady,
			readyCertificate("infra-operator-serving-cert", "openstack-operators"),
			readyCertificate("openstack-baremetal-operator-serving-cert", "openstack-operators"),
		}
		fakeClient := fakeclient.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
		r := &OpenStackReconciler{Client: fakeClient, Scheme: s}

		result, err := r.checkWebhookCertificates(ctx, instance)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).NotTo(BeZero())
	})

	t.Run("requeues when Ready is stale from a previous generation", func(t *testing.T) {
		g := NewWithT(t)
		s := newTestScheme(t)
		// Certificate spec changed (Generation bumped to 2) but status still reflects the old
		// generation's Ready condition - cert-manager hasn't reconciled the new spec yet.
		stale := &certmgrv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{Name: "openstack-operator-serving-cert", Namespace: "openstack-operators", Generation: 2},
			Status: certmgrv1.CertificateStatus{
				Conditions: []certmgrv1.CertificateCondition{
					{Type: certmgrv1.CertificateConditionReady, Status: cmmeta.ConditionTrue, ObservedGeneration: 1},
				},
			},
		}
		objs := []client.Object{
			instance, stale,
			readyCertificate("infra-operator-serving-cert", "openstack-operators"),
			readyCertificate("openstack-baremetal-operator-serving-cert", "openstack-operators"),
		}
		fakeClient := fakeclient.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
		r := &OpenStackReconciler{Client: fakeClient, Scheme: s}

		result, err := r.checkWebhookCertificates(ctx, instance)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).NotTo(BeZero(), "a Ready condition from a stale generation must not be treated as current")
	})

	t.Run("proceeds when all certificates are ready", func(t *testing.T) {
		g := NewWithT(t)
		s := newTestScheme(t)
		objs := []client.Object{
			instance,
			readyCertificate("openstack-operator-serving-cert", "openstack-operators"),
			readyCertificate("infra-operator-serving-cert", "openstack-operators"),
			readyCertificate("openstack-baremetal-operator-serving-cert", "openstack-operators"),
		}
		fakeClient := fakeclient.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
		r := &OpenStackReconciler{Client: fakeClient, Scheme: s}

		result, err := r.checkWebhookCertificates(ctx, instance)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("propagates a non-NotFound error from the API server", func(t *testing.T) {
		g := NewWithT(t)
		s := newTestScheme(t)
		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(instance).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if _, ok := obj.(*certmgrv1.Certificate); ok {
						return apierrors.NewServiceUnavailable("etcd unavailable")
					}
					return c.Get(ctx, key, obj, opts...)
				},
			}).
			Build()
		r := &OpenStackReconciler{Client: fakeClient, Scheme: s}

		result, err := r.checkWebhookCertificates(ctx, instance)
		g.Expect(err).To(HaveOccurred())
		g.Expect(result).To(Equal(ctrl.Result{}))
	})

	t.Run("skips disabled operators even if their certificate is missing", func(t *testing.T) {
		g := NewWithT(t)
		s := newTestScheme(t)
		disabledInstance := &operatorv1beta1.OpenStack{
			ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: "openstack-operators"},
			Spec: operatorv1beta1.OpenStackSpec{
				OperatorOverrides: []operatorv1beta1.OperatorSpec{
					{Name: "infra", Replicas: ptr.To(int32(0))},
					{Name: "openstack-baremetal", Replicas: ptr.To(int32(0))},
				},
			},
		}
		objs := []client.Object{
			disabledInstance,
			readyCertificate("openstack-operator-serving-cert", "openstack-operators"),
		}
		fakeClient := fakeclient.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()
		r := &OpenStackReconciler{Client: fakeClient, Scheme: s}

		result, err := r.checkWebhookCertificates(ctx, disabledInstance)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero(), "disabled operators' missing certificates should not block")
	})
}

// TestAdoptAwayServiceAccounts covers the ownership-migration logic: sub-operator
// ServiceAccounts left over from before openstack-operator#2109 are still
// controlled by the OpenStack instance, but this controller no longer manages them
// (they're OLM/CSV-owned now). The controller reference must be dropped, without
// deleting the object, and only for ServiceAccounts this instance actually
// controls.
func TestAdoptAwayServiceAccounts(t *testing.T) {
	g := NewWithT(t)
	s := newTestScheme(t)

	instance := &operatorv1beta1.OpenStack{
		ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: "openstack-operators"},
	}

	controlledSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "barbican-operator-controller-manager", Namespace: "openstack-operators"},
	}
	g.Expect(controllerutil.SetControllerReference(instance, controlledSA, s)).To(Succeed())

	// A ServiceAccount OLM created fresh (CSV-owned only, not controlled by instance)
	// must be left untouched.
	uncontrolledSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: "watcher-operator-controller-manager", Namespace: "openstack-operators"},
	}

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(s).
		WithObjects(instance, controlledSA, uncontrolledSA).
		Build()

	r := &OpenStackReconciler{Client: fakeClient, Scheme: s}

	g.Expect(r.adoptAwayServiceAccounts(context.Background(), instance)).To(Succeed())

	gotControlled := &corev1.ServiceAccount{}
	g.Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(controlledSA), gotControlled)).To(Succeed())
	g.Expect(gotControlled.GetOwnerReferences()).To(BeEmpty(), "controller reference should have been removed, not the object deleted")

	gotUncontrolled := &corev1.ServiceAccount{}
	g.Expect(fakeClient.Get(context.Background(), client.ObjectKeyFromObject(uncontrolledSA), gotUncontrolled)).To(Succeed())
	g.Expect(gotUncontrolled.GetOwnerReferences()).To(BeEmpty(), "ServiceAccount never controlled by instance should be untouched")

	// Idempotent: calling it again with the reference already gone must not error.
	g.Expect(r.adoptAwayServiceAccounts(context.Background(), instance)).To(Succeed())
}
