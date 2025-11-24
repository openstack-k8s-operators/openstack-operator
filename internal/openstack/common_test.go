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
	"context"
	"testing"

	. "github.com/onsi/gomega" //revive:disable:dot-imports

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	// Register OpenShift route scheme
	_ = routev1.AddToScheme(scheme.Scheme)
	_ = corev1.AddToScheme(scheme.Scheme)
}

// setupTestHelper creates a fake client and helper for testing
func setupTestHelper(objects ...client.Object) *helper.Helper {
	s := scheme.Scheme
	_ = routev1.AddToScheme(s)
	_ = corev1.AddToScheme(s)

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(s).
		WithObjects(objects...).
		WithStatusSubresource(&routev1.Route{}).
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
