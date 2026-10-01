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

package openstack

import (
	"cmp"
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega" //revive:disable:dot-imports

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	routev1 "github.com/openshift/api/route/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

func init() {
	// Register OpenShift route scheme
	_ = routev1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
	_ = certmgrv1.AddToScheme(scheme.Scheme)
}

// setupTestHelper creates a fake client and helper for testing
func setupTestHelper(objects ...client.Object) *helper.Helper {
	s := scheme.Scheme
	_ = routev1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = certmgrv1.AddToScheme(s)

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(s).
		WithObjects(objects...).
		WithStatusSubresource(&routev1.Route{}).
		WithStatusSubresource(&certmgrv1.Certificate{}).
		Build()

	// Create a fake kubernetes clientset
	fakeKubeClient := fake.NewSimpleClientset()

	// Create a mock OpenStackControlPlane object for the helper
	mockObj := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
	}

	h, _ := helper.NewHelper(
		mockObj,
		fakeClient,
		fakeKubeClient,
		s,
		ctrl.Log.WithName("test"),
	)
	return h
}

func TestCheckRouteAdmissionStatus_RouteNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	h := setupTestHelper()

	// Test with non-existent route
	err := checkRouteAdmissionStatus(ctx, h, "non-existent-route", "test-namespace")
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found"))
}

func TestCheckRouteAdmissionStatus_NoIngressStatus(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create a route without ingress status
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{},
		},
	}

	h := setupTestHelper(route)

	// Test with route that has no ingress status yet
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).ToNot(HaveOccurred(), "Should return nil for routes without ingress status (normal during initial creation)")
}

func TestCheckRouteAdmissionStatus_AdmittedTrue(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create a route with admitted status = true
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "test.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:   routev1.RouteAdmitted,
							Status: k8s_corev1.ConditionTrue,
						},
					},
				},
			},
		},
	}

	h := setupTestHelper(route)

	// Test with successfully admitted route
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).ToNot(HaveOccurred(), "Should return nil for successfully admitted routes")
}

func TestCheckRouteAdmissionStatus_AdmittedFalse(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	errorMessage := "HostAlreadyClaimed: route host is already claimed by another route"

	// Create a route with admitted status = false
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "test.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:    routev1.RouteAdmitted,
							Status:  k8s_corev1.ConditionFalse,
							Message: errorMessage,
						},
					},
				},
			},
		},
	}

	h := setupTestHelper(route)

	// Test with route that failed admission
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).To(HaveOccurred(), "Should return error for routes with failed admission")
	g.Expect(err.Error()).To(Equal(errorMessage), "Error message should match the route's admission failure message")
}

func TestCheckRouteAdmissionStatus_NoAdmissionCondition(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create a route with ingress status but no admission condition
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "test.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:   "SomeOtherCondition",
							Status: k8s_corev1.ConditionTrue,
						},
					},
				},
			},
		},
	}

	h := setupTestHelper(route)

	// Test with route that has no admission condition yet
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).ToNot(HaveOccurred(), "Should return nil for routes without admission condition (still being processed)")
}

func TestCheckRouteAdmissionStatus_MultipleConditions(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create a route with multiple conditions, including admitted = true
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "test.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:   "SomeOtherCondition",
							Status: k8s_corev1.ConditionTrue,
						},
						{
							Type:   routev1.RouteAdmitted,
							Status: k8s_corev1.ConditionTrue,
						},
						{
							Type:   "AnotherCondition",
							Status: k8s_corev1.ConditionFalse,
						},
					},
				},
			},
		},
	}

	h := setupTestHelper(route)

	// Test with route that has multiple conditions
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).ToNot(HaveOccurred(), "Should return nil when admitted condition is true, regardless of other conditions")
}

func TestCheckRouteAdmissionStatus_AdmittedFalseWithNoMessage(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create a route with admitted status = false but no error message
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "test.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:    routev1.RouteAdmitted,
							Status:  k8s_corev1.ConditionFalse,
							Message: "",
						},
					},
				},
			},
		},
	}

	h := setupTestHelper(route)

	// Test with route that failed admission but has no message
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).To(HaveOccurred(), "Should return error for routes with failed admission")
	g.Expect(err.Error()).To(Equal(""), "Error message should be empty when route has no failure message")
}

func TestCheckRouteAdmissionStatus_DifferentFailureMessages(t *testing.T) {
	testCases := []struct {
		name          string
		errorMessage  string
		expectedError string
	}{
		{
			name:          "HostAlreadyClaimed",
			errorMessage:  "HostAlreadyClaimed: route host is already claimed by another route",
			expectedError: "HostAlreadyClaimed: route host is already claimed by another route",
		},
		{
			name:          "RouteNotAdmitted",
			errorMessage:  "RouteNotAdmitted: host rejected due to validation errors",
			expectedError: "RouteNotAdmitted: host rejected due to validation errors",
		},
		{
			name:          "InvalidHost",
			errorMessage:  "InvalidHost: invalid characters in hostname",
			expectedError: "InvalidHost: invalid characters in hostname",
		},
		{
			name:          "NamespaceOwnershipError",
			errorMessage:  "NamespaceOwnershipError: namespace does not own the host",
			expectedError: "NamespaceOwnershipError: namespace does not own the host",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()

			route := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "test-namespace",
				},
				Spec: routev1.RouteSpec{
					Host: "test.example.com",
				},
				Status: routev1.RouteStatus{
					Ingress: []routev1.RouteIngress{
						{
							Host: "test.example.com",
							Conditions: []routev1.RouteIngressCondition{
								{
									Type:    routev1.RouteAdmitted,
									Status:  k8s_corev1.ConditionFalse,
									Message: tc.errorMessage,
								},
							},
						},
					},
				},
			}

			h := setupTestHelper(route)

			err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(Equal(tc.expectedError))
		})
	}
}

func TestCheckRouteAdmissionStatus_MultipleIngressEntries(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create a route with multiple ingress entries (only first should be checked)
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{
			Ingress: []routev1.RouteIngress{
				{
					Host: "test.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:   routev1.RouteAdmitted,
							Status: k8s_corev1.ConditionTrue,
						},
					},
				},
				{
					Host: "other.example.com",
					Conditions: []routev1.RouteIngressCondition{
						{
							Type:    routev1.RouteAdmitted,
							Status:  k8s_corev1.ConditionFalse,
							Message: "This should be ignored",
						},
					},
				},
			},
		},
	}

	h := setupTestHelper(route)

	// Test that only the first ingress entry is checked
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).ToNot(HaveOccurred(), "Should only check first ingress entry")
}

func TestCheckRouteAdmissionStatus_RealWorldScenarios(t *testing.T) {
	testCases := []struct {
		name          string
		route         *routev1.Route
		expectedError bool
		errorContains string
		description   string
	}{
		{
			name: "Fresh route - no status",
			route: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "barbican-public",
					Namespace: "openstack",
				},
				Spec: routev1.RouteSpec{
					Host: "barbican.apps.example.com",
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "barbican-public",
					},
				},
				Status: routev1.RouteStatus{},
			},
			expectedError: false,
			description:   "Newly created route without any status",
		},
		{
			name: "Route admitted successfully",
			route: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "barbican-public",
					Namespace: "openstack",
				},
				Spec: routev1.RouteSpec{
					Host: "barbican.apps.example.com",
				},
				Status: routev1.RouteStatus{
					Ingress: []routev1.RouteIngress{
						{
							Host:       "barbican.apps.example.com",
							RouterName: "default",
							Conditions: []routev1.RouteIngressCondition{
								{
									Type:               routev1.RouteAdmitted,
									Status:             k8s_corev1.ConditionTrue,
									LastTransitionTime: &metav1.Time{},
								},
							},
						},
					},
				},
			},
			expectedError: false,
			description:   "Route successfully admitted by router",
		},
		{
			name: "Route with hostname conflict",
			route: &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "barbican-public",
					Namespace: "openstack",
				},
				Spec: routev1.RouteSpec{
					Host: "barbican.apps.example.com",
				},
				Status: routev1.RouteStatus{
					Ingress: []routev1.RouteIngress{
						{
							Host:       "barbican.apps.example.com",
							RouterName: "default",
							Conditions: []routev1.RouteIngressCondition{
								{
									Type:               routev1.RouteAdmitted,
									Status:             k8s_corev1.ConditionFalse,
									Reason:             "HostAlreadyClaimed",
									Message:            "route host barbican.apps.example.com is already claimed by route keystone-public in namespace openstack",
									LastTransitionTime: &metav1.Time{},
								},
							},
						},
					},
				},
			},
			expectedError: true,
			errorContains: "route host barbican.apps.example.com is already claimed",
			description:   "Route hostname conflict with another route",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()

			h := setupTestHelper(tc.route)

			err := checkRouteAdmissionStatus(
				ctx,
				h,
				tc.route.Name,
				tc.route.Namespace,
			)

			if tc.expectedError {
				g.Expect(err).To(HaveOccurred(), tc.description)
				if tc.errorContains != "" {
					g.Expect(err.Error()).To(ContainSubstring(tc.errorContains), tc.description)
				}
			} else {
				g.Expect(err).ToNot(HaveOccurred(), tc.description)
			}
		})
	}
}

// TestCheckRouteAdmissionStatus_UpdatedStatus tests that we properly handle route status updates
func TestCheckRouteAdmissionStatus_UpdatedStatus(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create initial route without status
	route := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-route",
			Namespace: "test-namespace",
		},
		Spec: routev1.RouteSpec{
			Host: "test.example.com",
		},
		Status: routev1.RouteStatus{},
	}

	h := setupTestHelper(route)

	// First check - no status
	err := checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).ToNot(HaveOccurred())

	// Update route status to admitted
	updatedRoute := &routev1.Route{}
	err = h.GetClient().Get(ctx, types.NamespacedName{Name: "test-route", Namespace: "test-namespace"}, updatedRoute)
	g.Expect(err).ToNot(HaveOccurred())

	updatedRoute.Status.Ingress = []routev1.RouteIngress{
		{
			Host: "test.example.com",
			Conditions: []routev1.RouteIngressCondition{
				{
					Type:   routev1.RouteAdmitted,
					Status: k8s_corev1.ConditionTrue,
				},
			},
		},
	}
	err = h.GetClient().Status().Update(ctx, updatedRoute)
	g.Expect(err).ToNot(HaveOccurred())

	// Second check - should see admitted status
	err = checkRouteAdmissionStatus(ctx, h, "test-route", "test-namespace")
	g.Expect(err).ToNot(HaveOccurred())
}

func TestValidateRouteCertSecret(t *testing.T) {
	tests := []struct {
		name      string
		data      map[string][]byte
		wantErr   bool
		errSubstr string
	}{
		{
			name: "cert, key and ca.crt present",
			data: map[string][]byte{
				"tls.crt": []byte("cert"),
				"tls.key": []byte("key"),
				"ca.crt":  []byte("ca"),
			},
			wantErr: false,
		},
		{
			// ACME issuers (e.g. Let's Encrypt) do not populate ca.crt; the
			// secret must still be accepted.
			name: "cert and key present, ca.crt absent (ACME)",
			data: map[string][]byte{
				"tls.crt": []byte("cert"),
				"tls.key": []byte("key"),
			},
			wantErr: false,
		},
		{
			name: "tls.key missing",
			data: map[string][]byte{
				"tls.crt": []byte("cert"),
			},
			wantErr:   true,
			errSubstr: "does not provide tls.key",
		},
		{
			name: "tls.crt missing",
			data: map[string][]byte{
				"tls.key": []byte("key"),
			},
			wantErr:   true,
			errSubstr: "does not provide tls.crt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			secret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "custom-route-cert", Namespace: "test-namespace"},
				Data:       tt.data,
			}

			err := validateRouteCertSecret(secret, secret.Name)
			if tt.wantErr {
				g.Expect(err).To(HaveOccurred())
				g.Expect(err.Error()).To(ContainSubstring(tt.errSubstr))
			} else {
				g.Expect(err).ToNot(HaveOccurred())
			}
		})
	}
}

func TestValidateRouteCertSecret_NilSecret(t *testing.T) {
	g := NewWithT(t)
	// A nil secret is tolerated (the caller may not have fetched one).
	g.Expect(validateRouteCertSecret(nil, "custom-route-cert")).ToNot(HaveOccurred())
}

// TestEnsureEndpointConfig_EndpointURL_CustomHostname tests that when endpointURL
// is set on a LoadBalancer public endpoint, the custom hostname flows through correctly.
// issuerFor creates a fake cert-manager Issuer so certmanager.EnsureCert can
// proceed far enough to create the Certificate object (the secret need not
// exist - EnsureCert requeues at that point, after the Certificate is created).
func issuerFor(name string) *certmgrv1.Issuer {
	return &certmgrv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test-namespace"},
	}
}

func TestEnsureEndpointConfig_EndpointURL_CustomHostname(t *testing.T) {
	ctx := context.Background()
	clusterDomain := clusterdns.GetDNSClusterDomain()

	tests := []struct {
		name         string
		endpointURL  *string
		expectedSANs []string
	}{
		{
			name:        "endpointURL with custom hostname",
			endpointURL: ptr.To("https://keystone.example.com:5000"),
			expectedSANs: []string{
				"keystone.example.com",
				"keystone-public.test-namespace.svc",
				"keystone-public.test-namespace.svc." + clusterDomain,
			},
		},
		{
			name:        "no endpointURL",
			endpointURL: nil,
			expectedSANs: []string{
				"keystone-public.test-namespace.svc",
				"keystone-public.test-namespace.svc." + clusterDomain,
			},
		},
		{
			name:        "empty endpointURL",
			endpointURL: ptr.To(""),
			expectedSANs: []string{
				"keystone-public.test-namespace.svc",
				"keystone-public.test-namespace.svc." + clusterDomain,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create a public service with endpoint annotation
			publicSvc := &k8s_corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keystone-public",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						service.AnnotationEndpointKey:      "public",
						service.AnnotationIngressCreateKey: "false",
					},
				},
				Spec: k8s_corev1.ServiceSpec{
					Type: k8s_corev1.ServiceTypeLoadBalancer,
					Ports: []k8s_corev1.ServicePort{
						{Name: "keystone-public", Port: 5000},
					},
				},
			}

			h := setupTestHelper(publicSvc, issuerFor("rootca-public"))
			instance := &corev1.OpenStackControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-controlplane",
					Namespace: "test-namespace",
				},
				Spec: corev1.OpenStackControlPlaneSpec{
					TLS: corev1.TLSSection{
						PodLevel: corev1.TLSPodLevelConfig{
							Enabled: true,
						},
					},
				},
			}

			svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}

			svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{}
			if tt.endpointURL != nil {
				svcOverrides[service.EndpointPublic] = service.RoutedOverrideSpec{
					EndpointURL: tt.endpointURL,
				}
			}

			_, _, err := EnsureEndpointConfig(
				ctx,
				instance,
				h,
				instance,
				svcList,
				svcOverrides,
				corev1.Override{},
				"TestCondition",
				false,
				tls.API{},
			)
			g.Expect(err).ToNot(HaveOccurred())

			cert := &certmgrv1.Certificate{}
			g.Expect(h.GetClient().Get(ctx, types.NamespacedName{
				Name: "keystone-public-svc", Namespace: "test-namespace",
			}, cert)).To(Succeed())
			g.Expect(cert.Spec.DNSNames).To(ConsistOf(tt.expectedSANs))
		})
	}
}

// TestEnsureEndpointConfig_InternalEndpointURL_CustomHostname is the Gap 2
// regression guard: a custom internal endpointURL must land in the internal
// cert SAN too, not just the public one.
func TestEnsureEndpointConfig_InternalEndpointURL_CustomHostname(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clusterDomain := clusterdns.GetDNSClusterDomain()

	internalSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-internal",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey: "internal",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type:  k8s_corev1.ServiceTypeClusterIP,
			Ports: []k8s_corev1.ServicePort{{Name: "keystone-internal", Port: 5000}},
		},
	}

	h := setupTestHelper(internalSvc, issuerFor("rootca-internal"))
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{PodLevel: corev1.TLSPodLevelConfig{Enabled: true}},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*internalSvc}}
	svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{
		service.EndpointInternal: {EndpointURL: ptr.To("https://keystone-internal.example.com")},
	}

	_, _, err := EnsureEndpointConfig(
		ctx, instance, h, instance, svcList, svcOverrides, corev1.Override{},
		"TestCondition", false, tls.API{},
	)
	g.Expect(err).ToNot(HaveOccurred())

	cert := &certmgrv1.Certificate{}
	g.Expect(h.GetClient().Get(ctx, types.NamespacedName{
		Name: "keystone-internal-svc", Namespace: "test-namespace",
	}, cert)).To(Succeed())
	g.Expect(cert.Spec.DNSNames).To(ConsistOf(
		"keystone-internal.example.com",
		"keystone-internal.test-namespace.svc",
		"keystone-internal.test-namespace.svc."+clusterDomain,
	))
}

// TestEnsureEndpointConfig_InternalBYOCert verifies tls.api.internal.secretName is
// used as the internal pod-level cert and the operator-issued Service cert deleted.
func TestEnsureEndpointConfig_InternalBYOCert(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	internalSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "keystone-internal",
			Namespace:   "test-namespace",
			Annotations: map[string]string{service.AnnotationEndpointKey: "internal"},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type:  k8s_corev1.ServiceTypeClusterIP,
			Ports: []k8s_corev1.ServicePort{{Name: "keystone-internal", Port: 5000}},
		},
	}
	byoSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "byo-internal-cert", Namespace: "test-namespace"},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	}
	issuedCert := &certmgrv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "keystone-internal-svc", Namespace: "test-namespace"},
	}

	h := setupTestHelper(internalSvc, byoSecret, issuedCert, issuerFor("rootca-internal"))
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{PodLevel: corev1.TLSPodLevelConfig{Enabled: true}},
		},
	}
	tlsConfig := tls.API{}
	tlsConfig.API.Internal.SecretName = ptr.To("byo-internal-cert")

	endpoints, _, err := EnsureEndpointConfig(
		ctx, instance, h, instance, &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*internalSvc}},
		map[service.Endpoint]service.RoutedOverrideSpec{}, corev1.Override{},
		"TestCondition", false, tlsConfig,
	)
	g.Expect(err).ToNot(HaveOccurred())

	ed := endpoints.EndpointDetails[service.EndpointInternal]
	g.Expect(ed.Service.TLS.SecretName).ToNot(BeNil())
	g.Expect(*ed.Service.TLS.SecretName).To(Equal("byo-internal-cert"))

	err = h.GetClient().Get(ctx, client.ObjectKeyFromObject(issuedCert), &certmgrv1.Certificate{})
	g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue(), "issued internal service cert must be deleted")
}

// TestResolveServiceCert_BYO covers the BYO outcomes of resolveServiceCert: a missing
// secret requeues with CustomTLSReady False and must not continue, a present secret
// marks CustomTLSReady True and deletes the previously issued cert.
func TestResolveServiceCert_BYO(t *testing.T) {
	for _, secretExists := range []bool{false, true} {
		t.Run(fmt.Sprintf("secretExists=%t", secretExists), func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()

			issuedCert := &certmgrv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{Name: "keystone-public-svc", Namespace: "test-namespace"},
			}
			objects := []client.Object{issuedCert}
			if secretExists {
				objects = append(objects, &k8s_corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{Name: "byo-cert", Namespace: "test-namespace"},
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
						"tls.key": []byte("key-data"),
						"ca.crt":  []byte("ca-data"),
					},
				})
			}
			h := setupTestHelper(objects...)
			instance := &corev1.OpenStackControlPlane{
				ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
			}
			ed := &EndpointDetail{
				Name:      "keystone-public",
				Namespace: "test-namespace",
				Service:   ServiceDetails{TLS: ServiceTLSDetails{CertName: "keystone-public-svc"}},
			}

			res, err := ed.resolveServiceCert(ctx, instance, h, certmanager.CertificateRequest{}, ptr.To("byo-cert"))
			g.Expect(err).ToNot(HaveOccurred())

			cond := instance.Status.Conditions.Get(corev1.OpenStackControlPlaneCustomTLSReadyCondition)
			g.Expect(cond).ToNot(BeNil())
			getErr := h.GetClient().Get(ctx, client.ObjectKeyFromObject(issuedCert), &certmgrv1.Certificate{})
			if secretExists {
				g.Expect(res).To(Equal(ctrl.Result{}))
				g.Expect(cond.Status).To(Equal(k8s_corev1.ConditionTrue))
				g.Expect(k8s_errors.IsNotFound(getErr)).To(BeTrue(), "issued cert must be deleted")
			} else {
				g.Expect(res.RequeueAfter).To(Equal(10 * time.Second))
				g.Expect(cond.Status).To(Equal(k8s_corev1.ConditionFalse))
				g.Expect(getErr).ToNot(HaveOccurred(), "issued cert must be kept until the BYO secret exists")
			}
		})
	}
}

// TestEnsureEndpointConfig_AdditionalSubjectNames verifies additionalSubjectNames
// land on the cert SAN on top of the canonical {short, FQDN} set, since it is a
// cert-only escape hatch appended after the GetVHostServerName call, not a
// replacement for the canonical set.
func TestEnsureEndpointConfig_AdditionalSubjectNames(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clusterDomain := clusterdns.GetDNSClusterDomain()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey:      "public",
				service.AnnotationIngressCreateKey: "false",
				tls.AdditionalSubjectNamesKey:      "extra1.example.com,extra2.example.com",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type:  k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{{Name: "keystone-public", Port: 5000}},
		},
	}

	h := setupTestHelper(publicSvc, issuerFor("rootca-public"))
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{PodLevel: corev1.TLSPodLevelConfig{Enabled: true}},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}

	_, _, err := EnsureEndpointConfig(
		ctx, instance, h, instance, svcList,
		map[service.Endpoint]service.RoutedOverrideSpec{}, corev1.Override{},
		"TestCondition", false, tls.API{},
	)
	g.Expect(err).ToNot(HaveOccurred())

	cert := &certmgrv1.Certificate{}
	g.Expect(h.GetClient().Get(ctx, types.NamespacedName{
		Name: "keystone-public-svc", Namespace: "test-namespace",
	}, cert)).To(Succeed())
	g.Expect(cert.Spec.DNSNames).To(ConsistOf(
		"keystone-public.test-namespace.svc",
		"keystone-public.test-namespace.svc."+clusterDomain,
		"extra1.example.com",
		"extra2.example.com",
	))
}

// TestEnsureEndpointConfig_BadEndpointURL_FallsBackToBaseNames covers the
// common.go defense-in-depth fallback for a CR created before the admission
// webhook validated endpointURL (or any value that slips through): the
// reconcile must degrade to the base {short, FQDN} SANs, never error or panic.
func TestEnsureEndpointConfig_BadEndpointURL_FallsBackToBaseNames(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	clusterDomain := clusterdns.GetDNSClusterDomain()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey:      "public",
				service.AnnotationIngressCreateKey: "false",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type:  k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{{Name: "keystone-public", Port: 5000}},
		},
	}

	h := setupTestHelper(publicSvc, issuerFor("rootca-public"))
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{PodLevel: corev1.TLSPodLevelConfig{Enabled: true}},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}
	svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{
		// no scheme/host - GetVHostServerName returns ErrEndpointURLNoHost
		service.EndpointPublic: {EndpointURL: ptr.To("not-a-valid-url")},
	}

	g.Expect(func() {
		_, _, err := EnsureEndpointConfig(
			ctx, instance, h, instance, svcList, svcOverrides, corev1.Override{},
			"TestCondition", false, tls.API{},
		)
		g.Expect(err).ToNot(HaveOccurred())
	}).ToNot(Panic())

	cert := &certmgrv1.Certificate{}
	g.Expect(h.GetClient().Get(ctx, types.NamespacedName{
		Name: "keystone-public-svc", Namespace: "test-namespace",
	}, cert)).To(Succeed())
	g.Expect(cert.Spec.DNSNames).To(ConsistOf(
		"keystone-public.test-namespace.svc",
		"keystone-public.test-namespace.svc."+clusterDomain,
	))
}

// TestEnsureEndpointConfig_LoadBalancerBYOCert tests that for LoadBalancer
// endpoints the pod-level service cert is brought-your-own via
// tls.api.public.secretName (passed as tlsConfig.API.Public.SecretName), while
// apiOverride.tls.secretName (ingressOverride) is the Route cert only and must NOT
// feed the service cert.
func TestEnsureEndpointConfig_LoadBalancerBYOCert(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name                  string
		serviceType           k8s_corev1.ServiceType
		ingressOverrideTLS    *string
		tlsConfigPublicSecret *string
		expectSecretUsed      bool
		expectedSecretName    string
	}{
		{
			// apiOverride.tls is route-only: it must not become the service cert, so
			// the service cert falls through to issuance instead of the override secret.
			name:               "LoadBalancer ingressOverride does not feed service cert",
			serviceType:        k8s_corev1.ServiceTypeLoadBalancer,
			ingressOverrideTLS: ptr.To("custom-service-cert"),
			expectSecretUsed:   false,
		},
		{
			name:                  "LoadBalancer with tls.api.public.secretName",
			serviceType:           k8s_corev1.ServiceTypeLoadBalancer,
			ingressOverrideTLS:    nil,
			tlsConfigPublicSecret: ptr.To("global-public-cert"),
			expectSecretUsed:      true,
			expectedSecretName:    "global-public-cert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create a public service
			publicSvc := &k8s_corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keystone-public",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						service.AnnotationEndpointKey:      "public",
						service.AnnotationIngressCreateKey: "true",
					},
				},
				Spec: k8s_corev1.ServiceSpec{
					Type: tt.serviceType,
					Ports: []k8s_corev1.ServicePort{
						{Name: "keystone-public", Port: 5000},
					},
				},
			}

			// Create the BYO cert secret if expected
			var objects []client.Object
			// Create the issuers needed for cert issuance
			publicIssuer := &certmgrv1.Issuer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rootca-public",
					Namespace: "test-namespace",
				},
			}
			objects = append(objects, publicIssuer)
			// A previously operator-issued service cert: must be deleted when the
			// user brings their own, left alone otherwise.
			issuedCert := &certmgrv1.Certificate{
				ObjectMeta: metav1.ObjectMeta{Name: "keystone-public-svc", Namespace: "test-namespace"},
				Status: certmgrv1.CertificateStatus{Conditions: []certmgrv1.CertificateCondition{
					{Type: certmgrv1.CertificateConditionReady, Status: certmgrmetav1.ConditionTrue},
				}},
			}
			issuedSecret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "cert-keystone-public-svc", Namespace: "test-namespace"},
				Data: map[string][]byte{
					"tls.crt": []byte("cert-data"),
					"tls.key": []byte("key-data"),
					"ca.crt":  []byte("ca-data"),
				},
			}
			objects = append(objects, issuedCert, issuedSecret)
			if tt.expectSecretUsed {
				certSecret := &k8s_corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      tt.expectedSecretName,
						Namespace: "test-namespace",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("cert-data"),
						"tls.key": []byte("key-data"),
						"ca.crt":  []byte("ca-data"),
					},
				}
				objects = append(objects, certSecret)
			}
			objects = append(objects, publicSvc)

			h := setupTestHelper(objects...)
			instance := &corev1.OpenStackControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-controlplane",
					Namespace: "test-namespace",
				},
				Spec: corev1.OpenStackControlPlaneSpec{
					TLS: corev1.TLSSection{
						Ingress: corev1.TLSIngressConfig{
							Enabled: true,
						},
						PodLevel: corev1.TLSPodLevelConfig{
							Enabled: true,
						},
					},
				},
			}

			svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}

			ingressOverride := corev1.Override{}
			if tt.ingressOverrideTLS != nil {
				ingressOverride.TLS = &corev1.TLSServiceOverride{
					SecretName: *tt.ingressOverrideTLS,
				}
			}

			tlsConfig := tls.API{}
			if tt.tlsConfigPublicSecret != nil {
				tlsConfig.API.Public.SecretName = tt.tlsConfigPublicSecret
			}

			// Call EnsureEndpointConfig
			endpoints, _, err := EnsureEndpointConfig(
				ctx,
				instance,
				h,
				instance,
				svcList,
				map[service.Endpoint]service.RoutedOverrideSpec{},
				ingressOverride,
				"TestCondition",
				false,
				tlsConfig,
			)

			if tt.expectSecretUsed {
				// Should succeed when BYO cert secret exists and is valid
				g.Expect(err).ToNot(HaveOccurred())

				// Verify the service cert uses the expected secret
				publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
				g.Expect(publicEndpoint.Service.TLS.SecretName).ToNot(BeNil())
				g.Expect(*publicEndpoint.Service.TLS.SecretName).To(Equal(tt.expectedSecretName))

				// the operator-issued Service cert (not the Route cert) is deleted
				err = h.GetClient().Get(ctx, client.ObjectKeyFromObject(issuedCert), &certmgrv1.Certificate{})
				g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue(), "issued service cert must be deleted")
			} else {
				// For ClusterIP or when no BYO cert, cert issuance will be attempted
				// (may fail in test env, but that's expected)
				// apiOverride (ingress) secret is the Route cert only: with no Route it is
				// not even validated (it does not exist here), and it must NOT leak into
				// the service cert - the operator-issued one is used and kept.
				g.Expect(err).ToNot(HaveOccurred())
				publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
				g.Expect(publicEndpoint.Service.TLS.SecretName).ToNot(BeNil())
				g.Expect(*publicEndpoint.Service.TLS.SecretName).To(Equal("cert-keystone-public-svc"))
				g.Expect(h.GetClient().Get(ctx, client.ObjectKeyFromObject(issuedCert), &certmgrv1.Certificate{})).To(Succeed())
				g.Expect(instance.Status.Conditions.Get(corev1.OpenStackControlPlaneCustomTLSReadyCondition)).To(BeNil())
			}
		})
	}
}

// TestEnsureEndpointConfig_FrontPortInjection_TLSLoadBalancer tests that a TLS
// LoadBalancer public endpoint without an endpointURL keeps its native port: no
// director/RHOSP legacy port is injected implicitly.
func TestEnsureEndpointConfig_FrontPortInjection_TLSLoadBalancer(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey: "public",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type: k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{
				{Name: "public", Port: 5000},
			},
		},
	}

	certSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "global-public-cert",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	}

	h := setupTestHelper(publicSvc, certSecret)
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{
				Ingress:  corev1.TLSIngressConfig{Enabled: true},
				PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
			},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}
	tlsConfig := tls.API{}
	tlsConfig.API.Public.SecretName = ptr.To("global-public-cert")

	endpoints, _, err := EnsureEndpointConfig(
		ctx,
		instance,
		h,
		instance,
		svcList,
		map[service.Endpoint]service.RoutedOverrideSpec{},
		corev1.Override{},
		"TestCondition",
		false,
		tlsConfig,
	)
	g.Expect(err).ToNot(HaveOccurred())

	publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec).To(BeNil())
}

// TestEnsureEndpointConfig_FrontPortInjection_Skips tests the skip conditions:
// non-TLS and non-LoadBalancer. (A user-supplied ports[] override is covered by
// TestEnsureEndpointConfig_FrontPortInjection_RefreshesStaleName; a port that
// disagrees with endpointURL is rejected at admission, so it is not tested here.)
func TestEnsureEndpointConfig_FrontPortInjection_Skips(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		svcType     k8s_corev1.ServiceType
		podLevelTLS bool
	}{
		{
			name:        "non-TLS LoadBalancer public endpoint",
			podLevelTLS: false,
		},
		{
			// no ingress-create annotation, so Route.Create is false: only the
			// Service type keeps a non-LoadBalancer public Service out.
			name:        "ClusterIP public endpoint without ingress-create annotation",
			svcType:     k8s_corev1.ServiceTypeClusterIP,
			podLevelTLS: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			publicSvc := &k8s_corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "keystone-public",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						service.AnnotationEndpointKey: "public",
					},
				},
				Spec: k8s_corev1.ServiceSpec{
					Type: cmp.Or(tt.svcType, k8s_corev1.ServiceTypeLoadBalancer),
					Ports: []k8s_corev1.ServicePort{
						{Name: "public", Port: 5000},
					},
				},
			}

			certSecret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "global-public-cert",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"tls.crt": []byte("cert-data"),
					"tls.key": []byte("key-data"),
					"ca.crt":  []byte("ca-data"),
				},
			}

			h := setupTestHelper(publicSvc, certSecret)
			instance := &corev1.OpenStackControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-controlplane",
					Namespace: "test-namespace",
				},
				Spec: corev1.OpenStackControlPlaneSpec{
					TLS: corev1.TLSSection{
						Ingress:  corev1.TLSIngressConfig{Enabled: true},
						PodLevel: corev1.TLSPodLevelConfig{Enabled: tt.podLevelTLS},
					},
				},
			}

			// always set an endpointURL so only the skip condition under test
			// keeps the port from being injected
			svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{
				service.EndpointPublic: {EndpointURL: ptr.To("https://keystone.foo.bar:13000")},
			}

			svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}
			tlsConfig := tls.API{}
			tlsConfig.API.Public.SecretName = ptr.To("global-public-cert")

			endpoints, _, err := EnsureEndpointConfig(
				ctx,
				instance,
				h,
				instance,
				svcList,
				svcOverrides,
				corev1.Override{},
				"TestCondition",
				false,
				tlsConfig,
			)
			g.Expect(err).ToNot(HaveOccurred())

			publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
			g.Expect(publicEndpoint.Service.OverrideSpec.Spec).To(BeNil())
		})
	}
}

// TestEnsureEndpointConfig_FrontPortInjection_SkipsRoute tests that a routed
// (ClusterIP) public endpoint never gets the legacy port injected, even when pod
// level TLS is enabled for the service.
func TestEnsureEndpointConfig_FrontPortInjection_SkipsRoute(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey:      "public",
				service.AnnotationIngressCreateKey: "true",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type: k8s_corev1.ServiceTypeClusterIP,
			Ports: []k8s_corev1.ServicePort{
				{Name: "public", Port: 5000},
			},
		},
	}

	// Pre-create the service cert secret so EnsureCert's issuance path (taken
	// because this is a routed endpoint, so the BYO-secret shortcut does not
	// apply) finds it immediately instead of requeuing.
	svcCertSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cert-keystone-public-svc",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
		},
	}
	publicIssuer := &certmgrv1.Issuer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "rootca-public",
			Namespace: "test-namespace",
		},
	}

	h := setupTestHelper(publicSvc, svcCertSecret, publicIssuer)
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{
				// Route (ingress) TLS left disabled so route creation doesn't need
				// its own cert plumbing - only pod-level (service) TLS matters here.
				PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
			},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}

	endpoints, _, err := EnsureEndpointConfig(
		ctx,
		instance,
		h,
		instance,
		svcList,
		map[service.Endpoint]service.RoutedOverrideSpec{
			service.EndpointPublic: {EndpointURL: ptr.To("https://keystone.foo.bar:13000")},
		},
		corev1.Override{},
		"TestCondition",
		false,
		tls.API{},
	)
	g.Expect(err).ToNot(HaveOccurred())

	publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec).To(BeNil())
}

// TestEnsureEndpointConfig_FrontPortInjection_SkipsMultiplePorts tests that a
// public Service with more than one port is left alone - it's ambiguous which
// port is "the" public one, and the admin assigns ports for that case via the
// existing ports[] override instead.
func TestEnsureEndpointConfig_FrontPortInjection_SkipsMultiplePorts(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey: "public",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type: k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{
				{Name: "public", Port: 5000},
				{Name: "keystone-public-2", Port: 5001},
			},
		},
	}

	certSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "global-public-cert",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	}

	h := setupTestHelper(publicSvc, certSecret)
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{
				Ingress:  corev1.TLSIngressConfig{Enabled: true},
				PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
			},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}
	tlsConfig := tls.API{}
	tlsConfig.API.Public.SecretName = ptr.To("global-public-cert")

	endpoints, _, err := EnsureEndpointConfig(
		ctx,
		instance,
		h,
		instance,
		svcList,
		map[service.Endpoint]service.RoutedOverrideSpec{
			service.EndpointPublic: {EndpointURL: ptr.To("https://keystone.foo.bar:13000")},
		},
		corev1.Override{},
		"TestCondition",
		false,
		tlsConfig,
	)
	g.Expect(err).ToNot(HaveOccurred())

	publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec).To(BeNil())
}

// TestEnsureEndpointConfig_FrontPortInjection_FollowsEndpointURL tests that
// the Service front-port follows the port implied by endpointURL - explicit,
// or the scheme's standard port when omitted - instead of backing off to
// native, so the catalog URL and the live Service agree.
func TestEnsureEndpointConfig_FrontPortInjection_FollowsEndpointURL(t *testing.T) {
	tests := []struct {
		name         string
		endpointURL  *string
		userPorts    []service.OverrideServicePort
		expectedPort []service.OverrideServicePort
	}{
		{
			// Also covers clearing a previously-set ports[] override on a later
			// reconcile: EnsureEndpointConfig is stateless per call, so that
			// collapses to this same case - the Service falls back to the
			// endpointURL port, not to native.
			name:         "endpointURL with explicit port, no ports[] override: Service follows the URL port",
			endpointURL:  ptr.To("https://placement.foo.bar:13000"),
			expectedPort: []service.OverrideServicePort{{Name: "public", Port: 13000}},
		},
		{
			name:         "endpointURL with explicit port, matching ports[] override: name re-synced",
			endpointURL:  ptr.To("https://placement.foo.bar:13000"),
			userPorts:    []service.OverrideServicePort{{Name: "stale-name", Port: 13000}},
			expectedPort: []service.OverrideServicePort{{Name: "public", Port: 13000}},
		},
		{
			name:         "endpointURL without a port: falls back to the https scheme default",
			endpointURL:  ptr.To("https://placement.foo.bar"),
			expectedPort: []service.OverrideServicePort{{Name: "public", Port: 443}},
		},
		{
			name:         "endpointURL with an unrecognized scheme and no port: no inject, stays native",
			endpointURL:  ptr.To("ftp://placement.foo.bar"),
			expectedPort: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			ctx := context.Background()

			publicSvc := &k8s_corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "placement-public",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						service.AnnotationEndpointKey: "public",
					},
				},
				Spec: k8s_corev1.ServiceSpec{
					Type: k8s_corev1.ServiceTypeLoadBalancer,
					Ports: []k8s_corev1.ServicePort{
						{Name: "public", Port: 8778},
					},
				},
			}

			certSecret := &k8s_corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "global-public-cert",
					Namespace: "test-namespace",
				},
				Data: map[string][]byte{
					"tls.crt": []byte("cert-data"),
					"tls.key": []byte("key-data"),
					"ca.crt":  []byte("ca-data"),
				},
			}

			h := setupTestHelper(publicSvc, certSecret)
			instance := &corev1.OpenStackControlPlane{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-controlplane",
					Namespace: "test-namespace",
				},
				Spec: corev1.OpenStackControlPlaneSpec{
					TLS: corev1.TLSSection{
						Ingress:  corev1.TLSIngressConfig{Enabled: true},
						PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
					},
				},
			}

			override := service.RoutedOverrideSpec{EndpointURL: tt.endpointURL}
			if tt.userPorts != nil {
				override.OverrideSpec = service.OverrideSpec{
					Spec: &service.OverrideServiceSpec{Ports: tt.userPorts},
				}
			}
			svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{
				service.EndpointPublic: override,
			}

			svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}
			tlsConfig := tls.API{}
			tlsConfig.API.Public.SecretName = ptr.To("global-public-cert")

			endpoints, _, err := EnsureEndpointConfig(
				ctx,
				instance,
				h,
				instance,
				svcList,
				svcOverrides,
				corev1.Override{},
				"TestCondition",
				false,
				tlsConfig,
			)
			g.Expect(err).ToNot(HaveOccurred())

			publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
			if tt.expectedPort == nil {
				g.Expect(publicEndpoint.Service.OverrideSpec.Spec).To(BeNil())
				return
			}
			g.Expect(publicEndpoint.Service.OverrideSpec.Spec).ToNot(BeNil())
			g.Expect(publicEndpoint.Service.OverrideSpec.Spec.Ports).To(Equal(tt.expectedPort))
		})
	}
}

// TestEnsureEndpointConfig_FrontPortInjection_RefreshesStaleName tests that a
// previously-injected ports[] entry has its Name re-synced to the live
// Service's current port name on every reconcile - e.g. after a service
// operator renames the port - while the Port value (which may have been
// hand-tuned by the admin) is preserved as-is.
func TestEnsureEndpointConfig_FrontPortInjection_RefreshesStaleName(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "placement-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey: "public",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type: k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{
				// Renamed port: object name stays "placement-public", but the
				// live port is now bare "public".
				{Name: "public", Port: 8778},
			},
		},
	}

	certSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "global-public-cert",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	}

	h := setupTestHelper(publicSvc, certSecret)
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{
				Ingress:  corev1.TLSIngressConfig{Enabled: true},
				PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
			},
		},
	}

	const legacyPort int32 = 13778

	// Simulate a stale entry injected by a previous reconcile, back when the
	// live Service's port was still named "placement-public".
	svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{
		service.EndpointPublic: {
			EndpointURL: ptr.To("https://placement.foo.bar:13778"),
			OverrideSpec: service.OverrideSpec{
				Spec: &service.OverrideServiceSpec{
					Ports: []service.OverrideServicePort{{Name: "placement-public", Port: legacyPort}},
				},
			},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}
	tlsConfig := tls.API{}
	tlsConfig.API.Public.SecretName = ptr.To("global-public-cert")

	endpoints, _, err := EnsureEndpointConfig(
		ctx,
		instance,
		h,
		instance,
		svcList,
		svcOverrides,
		corev1.Override{},
		"TestCondition",
		false,
		tlsConfig,
	)
	g.Expect(err).ToNot(HaveOccurred())

	publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec).ToNot(BeNil())
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec.Ports).To(Equal(
		[]service.OverrideServicePort{{Name: "public", Port: legacyPort}},
	))
	// the caller's (parent spec) override must not be mutated in place
	g.Expect(svcOverrides[service.EndpointPublic].Spec.Ports[0].Name).To(Equal("placement-public"))
}

// TestEnsureEndpointConfig_FrontPortInjection_SkipsUnrenamedPort tests that no
// ports[] entry is injected while the live Service port still carries the old
// "<service>-public" name: the service operator's by-name merge would reject
// that name before it could patch (rename) the Service, deadlocking the rename.
func TestEnsureEndpointConfig_FrontPortInjection_SkipsUnrenamedPort(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "placement-public",
			Namespace:   "test-namespace",
			Annotations: map[string]string{service.AnnotationEndpointKey: "public"},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type:  k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{{Name: "placement-public", Port: 8778}},
		},
	}
	certSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "global-public-cert", Namespace: "test-namespace"},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	}

	h := setupTestHelper(publicSvc, certSecret)
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{
				Ingress:  corev1.TLSIngressConfig{Enabled: true},
				PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
			},
		},
	}

	tlsConfig := tls.API{}
	tlsConfig.API.Public.SecretName = ptr.To("global-public-cert")

	endpoints, _, err := EnsureEndpointConfig(
		ctx,
		instance,
		h,
		instance,
		&k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}},
		map[service.Endpoint]service.RoutedOverrideSpec{
			service.EndpointPublic: {EndpointURL: ptr.To("https://keystone.foo.bar:13000")},
		},
		corev1.Override{},
		"TestCondition",
		false,
		tlsConfig,
	)
	g.Expect(err).ToNot(HaveOccurred())

	publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec).To(BeNil())
}

// TestEnsureEndpointConfig_FrontPortInjection_PreservesMultiplePortsOverride
// tests that an override with more than one port entry (an admin config this
// default never produces on its own) is left completely untouched.
func TestEnsureEndpointConfig_FrontPortInjection_PreservesMultiplePortsOverride(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey: "public",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type: k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{
				{Name: "public", Port: 5000},
			},
		},
	}

	certSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "global-public-cert",
			Namespace: "test-namespace",
		},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	}

	h := setupTestHelper(publicSvc, certSecret)
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{
				Ingress:  corev1.TLSIngressConfig{Enabled: true},
				PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
			},
		},
	}

	userPorts := []service.OverrideServicePort{
		{Name: "public", Port: 9999},
		{Name: "keystone-debug", Port: 9998},
	}
	svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{
		service.EndpointPublic: {
			EndpointURL: ptr.To("https://keystone.foo.bar:13000"),
			OverrideSpec: service.OverrideSpec{
				Spec: &service.OverrideServiceSpec{Ports: userPorts},
			},
		},
	}

	svcList := &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}}
	tlsConfig := tls.API{}
	tlsConfig.API.Public.SecretName = ptr.To("global-public-cert")

	endpoints, _, err := EnsureEndpointConfig(
		ctx,
		instance,
		h,
		instance,
		svcList,
		svcOverrides,
		corev1.Override{},
		"TestCondition",
		false,
		tlsConfig,
	)
	g.Expect(err).ToNot(HaveOccurred())

	publicEndpoint := endpoints.EndpointDetails[service.EndpointPublic]
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec).ToNot(BeNil())
	g.Expect(publicEndpoint.Service.OverrideSpec.Spec.Ports).To(Equal(userPorts))
}

// TestCreateRoute_TargetPortName_ReadsLivePortName tests that CreateRoute reads
// the target port name straight off the live Service's port, not off the
// Service object's own name - the two are not guaranteed to be identical.
func TestCreateRoute_TargetPortName_ReadsLivePortName(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	h := setupTestHelper()
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
	}

	ed := &EndpointDetail{
		Name:      "placement-public",
		Namespace: "test-namespace",
		Service: ServiceDetails{
			Spec: &k8s_corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "placement-public",
					Namespace: "test-namespace",
				},
				Spec: k8s_corev1.ServiceSpec{
					Ports: []k8s_corev1.ServicePort{
						{Name: "public", Port: 8778},
					},
				},
			},
		},
	}

	_, err := ed.CreateRoute(ctx, instance, h, instance)
	g.Expect(err).ToNot(HaveOccurred())

	createdRoute := &routev1.Route{}
	err = h.GetClient().Get(ctx, types.NamespacedName{Name: ed.Name, Namespace: ed.Namespace}, createdRoute)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(createdRoute.Spec.Port).ToNot(BeNil())
	g.Expect(createdRoute.Spec.Port.TargetPort.String()).To(Equal("public"))
}

// TestCreateRoute_TargetPortName_MultiplePortsKeepsServiceName tests that with
// several live ports the first one is not picked arbitrarily: the Service name
// stays the default target.
func TestCreateRoute_TargetPortName_MultiplePortsKeepsServiceName(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	h := setupTestHelper()
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
	}

	ed := &EndpointDetail{
		Name:      "placement-public",
		Namespace: "test-namespace",
		Service: ServiceDetails{
			Spec: &k8s_corev1.Service{
				ObjectMeta: metav1.ObjectMeta{Name: "placement-public", Namespace: "test-namespace"},
				Spec: k8s_corev1.ServiceSpec{
					Ports: []k8s_corev1.ServicePort{
						{Name: "public", Port: 8778},
						{Name: "other", Port: 9999},
					},
				},
			},
		},
	}

	_, err := ed.CreateRoute(ctx, instance, h, instance)
	g.Expect(err).ToNot(HaveOccurred())

	createdRoute := &routev1.Route{}
	g.Expect(h.GetClient().Get(ctx, types.NamespacedName{Name: ed.Name, Namespace: ed.Namespace}, createdRoute)).To(Succeed())
	g.Expect(createdRoute.Spec.Port.TargetPort.String()).To(Equal("placement-public"))
}

// TestCreateRoute_TargetPortName_AnnotationOverrideWins tests that the
// AnnotationIngressTargetPortNameKey override still takes precedence over the
// live Service's port name.
func TestCreateRoute_TargetPortName_AnnotationOverrideWins(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	h := setupTestHelper()
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-controlplane",
			Namespace: "test-namespace",
		},
	}

	ed := &EndpointDetail{
		Name:      "placement-public",
		Namespace: "test-namespace",
		Service: ServiceDetails{
			Spec: &k8s_corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "placement-public",
					Namespace: "test-namespace",
					Annotations: map[string]string{
						service.AnnotationIngressTargetPortNameKey: "custom-target-port",
					},
				},
				Spec: k8s_corev1.ServiceSpec{
					Ports: []k8s_corev1.ServicePort{
						{Name: "public", Port: 8778},
					},
				},
			},
		},
	}

	_, err := ed.CreateRoute(ctx, instance, h, instance)
	g.Expect(err).ToNot(HaveOccurred())

	createdRoute := &routev1.Route{}
	err = h.GetClient().Get(ctx, types.NamespacedName{Name: ed.Name, Namespace: ed.Namespace}, createdRoute)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(createdRoute.Spec.Port).ToNot(BeNil())
	g.Expect(createdRoute.Spec.Port.TargetPort.String()).To(Equal("custom-target-port"))
}

func TestServicePortOrTypeChangedPredicate(t *testing.T) {
	svc := func(label bool, ports ...string) *k8s_corev1.Service {
		s := &k8s_corev1.Service{}
		if label {
			s.Labels = map[string]string{ooSelector: ""}
		}
		for _, p := range ports {
			s.Spec.Ports = append(s.Spec.Ports, k8s_corev1.ServicePort{Name: p, Port: 8778})
		}
		return s
	}
	tests := []struct {
		name     string
		old, new *k8s_corev1.Service
		want     bool
	}{
		{"managed service, port renamed", svc(true, "placement-public"), svc(true, "public"), true},
		{"managed service, port number only", svc(true, "public"), func() *k8s_corev1.Service {
			s := svc(true, "public")
			s.Spec.Ports[0].Port = 13778
			return s
		}(), false},
		{"managed service, type changed", svc(true, "public"), func() *k8s_corev1.Service {
			s := svc(true, "public")
			s.Spec.Type = k8s_corev1.ServiceTypeLoadBalancer
			return s
		}(), true},
		{"managed service, unchanged", svc(true, "public"), svc(true, "public"), false},
		{"unmanaged service, port renamed", svc(false, "a"), svc(false, "b"), false},
	}
	p := ServicePortOrTypeChangedPredicate()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(p.Update(event.UpdateEvent{ObjectOld: tt.old, ObjectNew: tt.new})).To(Equal(tt.want))
		})
	}
	g := NewWithT(t)
	g.Expect(p.Create(event.CreateEvent{Object: svc(true, "public")})).To(BeFalse())
}

// TestResolveServiceCert_BYOMissingNotHiddenByLaterValid mirrors the glance per-API
// loop: an endpoint with a missing BYO secret sets CustomTLSReady False, and a later
// endpoint in the same pass with a valid secret must not flip it back to True.
func TestResolveServiceCert_BYOMissingNotHiddenByLaterValid(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	h := setupTestHelper(&k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "valid-cert", Namespace: "test-namespace"},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	})
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
	}
	newED := func(name string) *EndpointDetail {
		return &EndpointDetail{
			Name:      name,
			Namespace: "test-namespace",
			Service:   ServiceDetails{TLS: ServiceTLSDetails{CertName: name + "-svc"}},
		}
	}

	res, err := newED("glance-a-public").resolveServiceCert(ctx, instance, h, certmanager.CertificateRequest{}, ptr.To("missing-cert"))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(res.RequeueAfter).To(Equal(10 * time.Second))

	_, err = newED("glance-b-public").resolveServiceCert(ctx, instance, h, certmanager.CertificateRequest{}, ptr.To("valid-cert"))
	g.Expect(err).ToNot(HaveOccurred())

	cond := instance.Status.Conditions.Get(corev1.OpenStackControlPlaneCustomTLSReadyCondition)
	g.Expect(cond).ToNot(BeNil())
	g.Expect(cond.Status).To(Equal(k8s_corev1.ConditionFalse))
}

// TestEnsureEndpointConfig_RouteToLoadBalancer_KeepsEndpointURL tests that moving an
// endpoint from a Route to a LoadBalancer (ingress-create "false") deletes the owned
// Route but keeps the user's endpointURL, instead of clearing it for one reconcile.
func TestEnsureEndpointConfig_RouteToLoadBalancer_KeepsEndpointURL(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Labels:    map[string]string{ooAppSelector: "keystone"},
			Annotations: map[string]string{
				service.AnnotationEndpointKey:      "public",
				service.AnnotationIngressCreateKey: "false",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type:  k8s_corev1.ServiceTypeLoadBalancer,
			Ports: []k8s_corev1.ServicePort{{Name: "public", Port: 5000}},
		},
	}
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace", UID: "cp-uid"},
	}
	ownedRoute := &routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Labels:    map[string]string{ooAppSelector: "keystone"},
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: "core.openstack.org/v1beta1", Kind: "OpenStackControlPlane",
				Name: "test-controlplane", UID: "cp-uid", Controller: ptr.To(true),
			}},
		},
		Spec: routev1.RouteSpec{To: routev1.RouteTargetReference{Name: "keystone-public"}},
	}

	h := setupTestHelper(publicSvc, ownedRoute)
	svcOverrides := map[service.Endpoint]service.RoutedOverrideSpec{
		service.EndpointPublic: {EndpointURL: ptr.To("https://keystone.example.com")},
	}

	endpoints, _, err := EnsureEndpointConfig(
		ctx, instance, h, instance, &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}},
		svcOverrides, corev1.Override{}, "TestCondition", false, tls.API{},
	)
	g.Expect(err).ToNot(HaveOccurred())

	err = h.GetClient().Get(ctx, client.ObjectKeyFromObject(ownedRoute), &routev1.Route{})
	g.Expect(k8s_errors.IsNotFound(err)).To(BeTrue(), "owned route must be deleted")

	ed := endpoints.EndpointDetails[service.EndpointPublic]
	g.Expect(ed.Service.OverrideSpec.EndpointURL).To(Equal(ptr.To("https://keystone.example.com")))
}

// TestEnsureEndpointConfig_RouteBackedPublic_IgnoresBYOServiceCert tests that a
// Route-backed public endpoint keeps the operator-issued pod cert even when
// tls.api.public.secretName is set: a custom cert for a Route goes on the Route
// (apiOverride.tls.secretName).
func TestEnsureEndpointConfig_RouteBackedPublic_IgnoresBYOServiceCert(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	publicSvc := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keystone-public",
			Namespace: "test-namespace",
			Annotations: map[string]string{
				service.AnnotationEndpointKey:      "public",
				service.AnnotationIngressCreateKey: "true",
			},
		},
		Spec: k8s_corev1.ServiceSpec{
			Type:  k8s_corev1.ServiceTypeClusterIP,
			Ports: []k8s_corev1.ServicePort{{Name: "public", Port: 5000}},
		},
	}
	byoSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "byo-public-cert", Namespace: "test-namespace"},
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("ca-data"),
		},
	}
	issuedCert := &certmgrv1.Certificate{
		ObjectMeta: metav1.ObjectMeta{Name: "keystone-public-svc", Namespace: "test-namespace"},
		Status: certmgrv1.CertificateStatus{Conditions: []certmgrv1.CertificateCondition{
			{Type: certmgrv1.CertificateConditionReady, Status: certmgrmetav1.ConditionTrue},
		}},
	}

	issuedSecret := &k8s_corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "cert-keystone-public-svc", Namespace: "test-namespace"},
		Data:       byoSecret.Data,
	}

	h := setupTestHelper(publicSvc, byoSecret, issuedCert, issuedSecret, issuerFor("rootca-public"))
	instance := &corev1.OpenStackControlPlane{
		ObjectMeta: metav1.ObjectMeta{Name: "test-controlplane", Namespace: "test-namespace"},
		Spec: corev1.OpenStackControlPlaneSpec{
			TLS: corev1.TLSSection{
				Ingress:  corev1.TLSIngressConfig{Enabled: true},
				PodLevel: corev1.TLSPodLevelConfig{Enabled: true},
			},
		},
	}
	tlsConfig := tls.API{}
	tlsConfig.API.Public.SecretName = ptr.To("byo-public-cert")

	// the call may requeue on the (not yet ready) route cert; the service cert is
	// resolved before that, which is what is asserted below
	_, _, err := EnsureEndpointConfig(
		ctx, instance, h, instance, &k8s_corev1.ServiceList{Items: []k8s_corev1.Service{*publicSvc}},
		map[service.Endpoint]service.RoutedOverrideSpec{}, corev1.Override{},
		"TestCondition", false, tlsConfig,
	)
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(h.GetClient().Get(ctx, client.ObjectKeyFromObject(issuedCert), &certmgrv1.Certificate{})).To(Succeed(),
		"operator-issued service cert must be kept")
	g.Expect(instance.Status.Conditions.Get(corev1.OpenStackControlPlaneCustomTLSReadyCondition)).To(BeNil(),
		"an ignored BYO secret must not set CustomTLSReady")
}
