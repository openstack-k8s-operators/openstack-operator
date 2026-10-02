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

	. "github.com/onsi/gomega" //revive:disable:dot-imports
	"k8s.io/utils/ptr"

	operatorv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/operator/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(operatorv1beta1.AddToScheme(s)).To(Succeed())
	return s
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

func TestCheckWebhookCertificates(t *testing.T) {
	s := newTestScheme(t)
	ctx := context.Background()

	instance := &operatorv1beta1.OpenStack{
		ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: "openstack-operators"},
		Spec:       operatorv1beta1.OpenStackSpec{},
	}

	t.Run("returns requeue when certificate secret is missing", func(t *testing.T) {
		g := NewWithT(t)

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(instance).
			Build()

		r := &OpenStackReconciler{
			Client:                 fakeClient,
			Scheme:                 s,
			webhookCertSecretNames: []string{"webhook-server-cert"},
		}

		result, err := r.checkWebhookCertificates(ctx, instance)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).NotTo(BeZero(), "should requeue when cert is missing")
	})

	t.Run("proceeds when all certificate secrets exist", func(t *testing.T) {
		g := NewWithT(t)

		cert := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webhook-server-cert",
				Namespace: "openstack-operators",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(instance, cert).
			Build()

		r := &OpenStackReconciler{
			Client:                 fakeClient,
			Scheme:                 s,
			webhookCertSecretNames: []string{"webhook-server-cert"},
		}

		result, err := r.checkWebhookCertificates(ctx, instance)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(result.RequeueAfter).To(BeZero(), "should not requeue when all certs exist")
	})

	t.Run("skips disabled operators", func(t *testing.T) {
		g := NewWithT(t)

		instanceWithDisabled := &operatorv1beta1.OpenStack{
			ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: "openstack-operators"},
			Spec: operatorv1beta1.OpenStackSpec{
				OperatorOverrides: []operatorv1beta1.OperatorSpec{
					{
						Name:     "infra",
						Replicas: ptr.To(int32(0)),
					},
				},
			},
		}

		// Only create openstack-operator cert, not infra-operator cert
		cert := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "webhook-server-cert",
				Namespace: "openstack-operators",
			},
		}

		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(instanceWithDisabled, cert).
			Build()

		r := &OpenStackReconciler{
			Client: fakeClient,
			Scheme: s,
			// Both certs are in the discovered list
			webhookCertSecretNames: []string{
				"webhook-server-cert",
				"infra-operator-webhook-server-cert",
			},
		}

		result, err := r.checkWebhookCertificates(ctx, instanceWithDisabled)
		g.Expect(err).NotTo(HaveOccurred())
		// Should not requeue - infra cert is not required because infra is disabled
		g.Expect(result.RequeueAfter).To(BeZero(), "should not wait for disabled operator's cert")
	})

	t.Run("waits for enabled operators even when others disabled", func(t *testing.T) {
		g := NewWithT(t)

		instanceWithDisabled := &operatorv1beta1.OpenStack{
			ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: "openstack-operators"},
			Spec: operatorv1beta1.OpenStackSpec{
				OperatorOverrides: []operatorv1beta1.OperatorSpec{
					{
						Name:     "infra",
						Replicas: ptr.To(int32(0)),
					},
				},
			},
		}

		// No certs exist
		fakeClient := fakeclient.NewClientBuilder().
			WithScheme(s).
			WithObjects(instanceWithDisabled).
			Build()

		r := &OpenStackReconciler{
			Client: fakeClient,
			Scheme: s,
			webhookCertSecretNames: []string{
				"webhook-server-cert",                // needed (openstack operator)
				"infra-operator-webhook-server-cert", // not needed (disabled)
			},
		}

		result, err := r.checkWebhookCertificates(ctx, instanceWithDisabled)
		g.Expect(err).NotTo(HaveOccurred())
		// Should requeue - still need openstack-operator cert even though infra is disabled
		g.Expect(result.RequeueAfter).NotTo(BeZero(), "should wait for enabled operator's cert")
	})
}
