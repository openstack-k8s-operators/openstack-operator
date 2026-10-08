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
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega" //revive:disable:dot-imports

	operatorv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/operator/v1beta1"
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func newTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	s := runtime.NewScheme()
	g := NewWithT(t)
	g.Expect(corev1.AddToScheme(s)).To(Succeed())
	g.Expect(discoveryv1.AddToScheme(s)).To(Succeed())
	g.Expect(operatorv1beta1.AddToScheme(s)).To(Succeed())
	return s
}

// emptyEndpointSlice builds an EndpointSlice with no endpoints, as if the backing
// Service has no ready pods yet.
func emptyEndpointSlice(name, namespace string) *discoveryv1.EndpointSlice {
	return &discoveryv1.EndpointSlice{
		ObjectMeta: metav1.ObjectMeta{Name: name + "-abcde", Namespace: namespace,
			Labels: map[string]string{discoveryv1.LabelServiceName: name}},
		AddressType: discoveryv1.AddressTypeIPv4,
	}
}

// TestCheckServiceEndpointsBlocksOnInfraAndBaremetal covers a regression where
// switching from a hardcoded webhook-service list to dynamic discovery
// (webhookServiceNames) stopped blocking readiness on the infra-operator and
// openstack-baremetal-operator webhook endpoints. Those two back always-active
// admission webhooks (see hack/sync-bindata.sh's special-casing), unlike
// service-operator conversion webhooks which stay dormant until v1beta2 lands and
// must not block readiness.
func TestCheckServiceEndpointsBlocksOnInfraAndBaremetal(t *testing.T) {
	g := NewWithT(t)
	s := newTestScheme(t)
	ns := "openstack-operators"

	instance := &operatorv1beta1.OpenStack{
		ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: ns},
	}

	// openstack-operator's own webhook is up.
	ownSlice := emptyEndpointSlice("openstack-operator-webhook-service", ns)
	ownSlice.Endpoints = []discoveryv1.Endpoint{{
		Addresses:  []string{"10.0.0.1"},
		Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true), Serving: ptr.To(true)},
	}}

	// infra-operator's and openstack-baremetal-operator's webhooks have no endpoints yet
	// (e.g. pods not scheduled): either one must block.
	infraSlice := emptyEndpointSlice("infra-operator-webhook-service", ns)
	baremetalSlice := emptyEndpointSlice("openstack-baremetal-operator-webhook-service", ns)

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(s).
		WithObjects(instance, ownSlice, infraSlice, baremetalSlice).
		Build()

	r := &OpenStackReconciler{
		Client: fakeClient,
		Scheme: s,
		// true: both manifests register an admission webhook (see loadWebhookOperatorSet).
		webhookOperatorSet: map[string]bool{"infra": true, "openstack-baremetal": true},
	}

	result, err := r.checkServiceEndpoints(context.Background(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeNumerically(">", 0), "missing infra-operator and baremetal webhook endpoints must requeue, not be treated as non-blocking")

	// infra-operator's webhook becomes ready, but baremetal's doesn't yet: must still block.
	infraSlice.Endpoints = []discoveryv1.Endpoint{{
		Addresses:  []string{"10.0.0.2"},
		Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true), Serving: ptr.To(true)},
	}}
	g.Expect(fakeClient.Update(context.Background(), infraSlice)).To(Succeed())

	result, err = r.checkServiceEndpoints(context.Background(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeNumerically(">", 0), "missing openstack-baremetal-operator webhook endpoint alone must still requeue")

	// Once openstack-baremetal-operator's webhook is also ready, reconciliation proceeds.
	baremetalSlice.Endpoints = []discoveryv1.Endpoint{{
		Addresses:  []string{"10.0.0.3"},
		Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true), Serving: ptr.To(true)},
	}}
	g.Expect(fakeClient.Update(context.Background(), baremetalSlice)).To(Succeed())

	result, err = r.checkServiceEndpoints(context.Background(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeZero())
}

// TestCheckServiceEndpointsDormantServiceOperatorWebhookNonBlocking covers the
// companion case: a service-operator's conversion webhook (dormant until v1beta2)
// must not block readiness even with no endpoints.
func TestCheckServiceEndpointsDormantServiceOperatorWebhookNonBlocking(t *testing.T) {
	g := NewWithT(t)
	s := newTestScheme(t)
	ns := "openstack-operators"

	instance := &operatorv1beta1.OpenStack{
		ObjectMeta: metav1.ObjectMeta{Name: "openstack", Namespace: ns},
	}

	ownSlice := emptyEndpointSlice("openstack-operator-webhook-service", ns)
	ownSlice.Endpoints = []discoveryv1.Endpoint{{
		Addresses:  []string{"10.0.0.1"},
		Conditions: discoveryv1.EndpointConditions{Ready: ptr.To(true), Serving: ptr.To(true)},
	}}

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(s).
		WithObjects(instance, ownSlice).
		Build()

	r := &OpenStackReconciler{
		Client: fakeClient,
		Scheme: s,
		// false: keystone's manifest carries only the dormant conversion webhook's
		// serving resources, not an admission webhook (see loadWebhookOperatorSet).
		webhookOperatorSet: map[string]bool{"keystone": false},
	}

	result, err := r.checkServiceEndpoints(context.Background(), instance)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(result.RequeueAfter).To(BeZero(), "missing dormant conversion webhook endpoint must not block readiness")
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

// TestLoadWebhookOperatorSet covers the startup discovery that drives whether a webhook
// service blocks readiness: a manifest with a Mutating/ValidatingWebhookConfiguration
// (written by hack/sync-bindata.sh's write_webhooks, e.g. for infra-operator) must be
// flagged as an admission webhook, while one with only the serving Service+Certificate
// (write_webhook_serving_resources, a dormant CRD conversion webhook) must not.
func TestLoadWebhookOperatorSet(t *testing.T) {
	g := NewWithT(t)
	bindir := t.TempDir()
	operatorDir := filepath.Join(bindir, "operator")
	g.Expect(os.MkdirAll(operatorDir, 0o755)).To(Succeed())

	admissionManifest := `apiVersion: v1
kind: Service
metadata:
  name: infra-operator-webhook-service
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: infra-operator-mutating-webhook-configuration
`
	conversionOnlyManifest := `apiVersion: v1
kind: Service
metadata:
  name: keystone-operator-webhook-service
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: keystone-operator-serving-cert
`
	g.Expect(os.WriteFile(filepath.Join(operatorDir, "infra-operator-webhooks.yaml"), []byte(admissionManifest), 0o644)).To(Succeed())
	g.Expect(os.WriteFile(filepath.Join(operatorDir, "keystone-operator-webhooks.yaml"), []byte(conversionOnlyManifest), 0o644)).To(Succeed())

	// Unreadable manifest (a directory, not a file) must fail closed: treated as an
	// admission webhook rather than silently downgraded to dormant/non-blocking.
	g.Expect(os.MkdirAll(filepath.Join(operatorDir, "broken-operator-webhooks.yaml"), 0o755)).To(Succeed())

	result := loadWebhookOperatorSet(context.Background(), bindir)

	g.Expect(result).To(HaveKeyWithValue("infra", true), "manifest with MutatingWebhookConfiguration must be flagged as an admission webhook")
	g.Expect(result).To(HaveKeyWithValue("keystone", false), "manifest with only serving resources must be flagged as dormant conversion-only")
	g.Expect(result).To(HaveKeyWithValue("broken", true), "unreadable manifest must fail closed as an admission webhook, not silently become non-blocking")
}
