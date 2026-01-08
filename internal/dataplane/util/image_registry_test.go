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

package util

import (
	"context"
	"encoding/base64"
	"errors"
	"testing"

	. "github.com/onsi/gomega" //revive:disable:dot-imports

	ocpidms "github.com/openshift/api/config/v1"
	mc "github.com/openshift/api/machineconfiguration/v1"
	ocpicsp "github.com/openshift/api/operator/v1alpha1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// setupTestHelper creates a fake client and helper for testing
// The includeOpenShiftCRDs parameter controls whether OpenShift-specific CRDs are registered
func setupTestHelper(includeOpenShiftCRDs bool, objects ...client.Object) *helper.Helper {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = k8s_corev1.AddToScheme(s)

	if includeOpenShiftCRDs {
		_ = ocpicsp.AddToScheme(s)
		_ = ocpidms.AddToScheme(s)
		_ = mc.AddToScheme(s)
	}

	fakeClient := fakeclient.NewClientBuilder().
		WithScheme(s).
		WithObjects(objects...).
		Build()

	fakeKubeClient := fake.NewSimpleClientset()

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

// Test IsNoMatchError
func TestIsNoMatchError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "no matches for kind error",
			err:      errors.New("no matches for kind \"ImageContentSourcePolicy\" in version \"operator.openshift.io/v1alpha1\""),
			expected: true,
		},
		{
			name:     "no matches for kind - IDMS",
			err:      errors.New("no matches for kind \"ImageDigestMirrorSet\" in version \"config.openshift.io/v1\""),
			expected: true,
		},
		{
			name:     "no matches for kind - MachineConfig",
			err:      errors.New("no matches for kind \"MachineConfig\" in version \"machineconfiguration.openshift.io/v1\""),
			expected: true,
		},
		{
			name:     "no kind is registered error (fake client)",
			err:      errors.New("no kind is registered for the type v1alpha1.ImageContentSourcePolicyList in scheme \"pkg/runtime/scheme.go:100\""),
			expected: true,
		},
		{
			name:     "no kind is registered - MachineConfig (fake client)",
			err:      errors.New("no kind is registered for the type v1.MachineConfig in scheme \"pkg/runtime/scheme.go:100\""),
			expected: true,
		},
		{
			name:     "not found error",
			err:      errors.New("machineconfigs.machineconfiguration.openshift.io \"99-master-generated-registries\" not found"),
			expected: false,
		},
		{
			name:     "generic error",
			err:      errors.New("connection refused"),
			expected: false,
		},
		{
			name:     "permission error",
			err:      errors.New("forbidden: User cannot list resource"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			result := IsNoMatchError(tt.err)
			g.Expect(result).To(Equal(tt.expected))
		})
	}
}

// Test IsDisconnectedOCP scenarios
func TestIsDisconnectedOCP_WithICSP(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create an ICSP resource
	icsp := &ocpicsp.ImageContentSourcePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-icsp",
		},
		Spec: ocpicsp.ImageContentSourcePolicySpec{
			RepositoryDigestMirrors: []ocpicsp.RepositoryDigestMirrors{
				{
					Source:  "registry.redhat.io",
					Mirrors: []string{"local-registry.example.com"},
				},
			},
		},
	}

	h := setupTestHelper(true, icsp)

	disconnected, err := IsDisconnectedOCP(ctx, h)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(disconnected).To(BeTrue(), "Should detect disconnected environment when ICSP exists")
}

func TestIsDisconnectedOCP_WithIDMS(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create an IDMS resource
	idms := &ocpidms.ImageDigestMirrorSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-idms",
		},
		Spec: ocpidms.ImageDigestMirrorSetSpec{
			ImageDigestMirrors: []ocpidms.ImageDigestMirrors{
				{
					Source: "registry.redhat.io",
					Mirrors: []ocpidms.ImageMirror{
						"local-registry.example.com",
					},
				},
			},
		},
	}

	h := setupTestHelper(true, idms)

	disconnected, err := IsDisconnectedOCP(ctx, h)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(disconnected).To(BeTrue(), "Should detect disconnected environment when IDMS exists")
}

func TestIsDisconnectedOCP_WithBothICSPAndIDMS(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	icsp := &ocpicsp.ImageContentSourcePolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-icsp",
		},
	}

	idms := &ocpidms.ImageDigestMirrorSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-idms",
		},
	}

	h := setupTestHelper(true, icsp, idms)

	disconnected, err := IsDisconnectedOCP(ctx, h)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(disconnected).To(BeTrue(), "Should detect disconnected environment when both ICSP and IDMS exist")
}

func TestIsDisconnectedOCP_EmptyLists(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// No ICSP or IDMS resources, but CRDs are registered
	h := setupTestHelper(true)

	disconnected, err := IsDisconnectedOCP(ctx, h)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(disconnected).To(BeFalse(), "Should not detect disconnected environment when lists are empty")
}

func TestIsDisconnectedOCP_CRDsNotInstalled(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Don't register OpenShift CRDs - simulates non-OpenShift cluster
	h := setupTestHelper(false)

	disconnected, err := IsDisconnectedOCP(ctx, h)
	g.Expect(err).ToNot(HaveOccurred(), "Should not return error when CRDs don't exist")
	g.Expect(disconnected).To(BeFalse(), "Should return false when CRDs don't exist (graceful degradation)")
}

// Test GetMCRegistryConf scenarios
func TestGetMCRegistryConf_Success(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// The expected registries.conf content
	expectedConfig := `[[registry]]
  prefix = ""
  location = "quay.io/openstack-k8s-operators"

  [[registry.mirror]]
    location = "local-registry.example.com/openstack-k8s-operators"
`

	// Encode the config as the MachineConfig expects it
	b64Config := base64.StdEncoding.EncodeToString([]byte(expectedConfig))
	ignitionConfig := `{
		"ignition": {"version": "3.2.0"},
		"storage": {
			"files": [{
				"contents": {
					"source": "data:text/plain;charset=utf-8;base64,` + b64Config + `"
				},
				"path": "/etc/containers/registries.conf"
			}]
		}
	}`

	machineConfig := &mc.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "99-master-generated-registries",
		},
		Spec: mc.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: []byte(ignitionConfig),
			},
		},
	}

	h := setupTestHelper(true, machineConfig)

	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(config).To(Equal(expectedConfig))
}

func TestGetMCRegistryConf_MachineConfigNotFound(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// MachineConfig CRD is registered but no resource exists
	// This simulates the case where MCO is installed but the specific
	// registry MachineConfig doesn't exist - this should be treated as an error,
	// not a warning, because if MCO is present and disconnected env is detected,
	// the registry config should exist.
	h := setupTestHelper(true)

	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("not found"))
	// Verify this is NOT a "no match" error - it's a regular NotFound error
	// which indicates the CRD exists but the resource doesn't
	g.Expect(IsNoMatchError(err)).To(BeFalse(),
		"NotFound error should NOT be treated as IsNoMatchError - CRD exists but resource doesn't")
	g.Expect(config).To(BeEmpty())
}

func TestGetMCRegistryConf_CRDNotInstalled(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Don't register MachineConfig CRD - simulates non-OpenShift cluster
	h := setupTestHelper(false)

	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).To(HaveOccurred())
	g.Expect(IsNoMatchError(err)).To(BeTrue(), "Error should be a 'no match' error indicating CRD doesn't exist")
	g.Expect(config).To(BeEmpty())
}

func TestGetMCRegistryConf_InvalidIgnitionFormat(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create MachineConfig with invalid JSON
	machineConfig := &mc.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "99-master-generated-registries",
		},
		Spec: mc.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: []byte(`invalid json`),
			},
		},
	}

	h := setupTestHelper(true, machineConfig)

	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).To(HaveOccurred())
	g.Expect(config).To(BeEmpty())
}

func TestGetMCRegistryConf_MissingBase64Prefix(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create MachineConfig without the expected base64 prefix
	ignitionConfig := `{
		"ignition": {"version": "3.2.0"},
		"storage": {
			"files": [{
				"contents": {
					"source": "plain-text-without-prefix"
				},
				"path": "/etc/containers/registries.conf"
			}]
		}
	}`

	machineConfig := &mc.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "99-master-generated-registries",
		},
		Spec: mc.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: []byte(ignitionConfig),
			},
		},
	}

	h := setupTestHelper(true, machineConfig)

	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("no b64prefix found"))
	g.Expect(config).To(BeEmpty())
}

func TestGetMCRegistryConf_InvalidBase64Content(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Create MachineConfig with invalid base64 content
	ignitionConfig := `{
		"ignition": {"version": "3.2.0"},
		"storage": {
			"files": [{
				"contents": {
					"source": "data:text/plain;charset=utf-8;base64,!!!invalid-base64!!!"
				},
				"path": "/etc/containers/registries.conf"
			}]
		}
	}`

	machineConfig := &mc.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "99-master-generated-registries",
		},
		Spec: mc.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: []byte(ignitionConfig),
			},
		},
	}

	h := setupTestHelper(true, machineConfig)

	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).To(HaveOccurred())
	g.Expect(config).To(BeEmpty())
}

// Test real-world scenarios
func TestDisconnectedEnvironment_FullScenario(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Simulate a full disconnected environment with IDMS and MachineConfig
	expectedConfig := `[[registry]]
  prefix = ""
  location = "registry.redhat.io/rhosp-dev-preview"

  [[registry.mirror]]
    location = "disconnected-registry.example.com:5000/rhosp-dev-preview"
`

	b64Config := base64.StdEncoding.EncodeToString([]byte(expectedConfig))
	ignitionConfig := `{
		"ignition": {"version": "3.2.0"},
		"storage": {
			"files": [{
				"contents": {
					"source": "data:text/plain;charset=utf-8;base64,` + b64Config + `"
				},
				"path": "/etc/containers/registries.conf"
			}]
		}
	}`

	idms := &ocpidms.ImageDigestMirrorSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "image-policy",
		},
		Spec: ocpidms.ImageDigestMirrorSetSpec{
			ImageDigestMirrors: []ocpidms.ImageDigestMirrors{
				{
					Source: "registry.redhat.io/rhosp-dev-preview",
					Mirrors: []ocpidms.ImageMirror{
						"disconnected-registry.example.com:5000/rhosp-dev-preview",
					},
				},
			},
		},
	}

	machineConfig := &mc.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "99-master-generated-registries",
		},
		Spec: mc.MachineConfigSpec{
			Config: runtime.RawExtension{
				Raw: []byte(ignitionConfig),
			},
		},
	}

	h := setupTestHelper(true, idms, machineConfig)

	// Check for disconnected environment
	disconnected, err := IsDisconnectedOCP(ctx, h)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(disconnected).To(BeTrue())

	// Get the registry configuration
	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(config).To(Equal(expectedConfig))
}

func TestNonOpenShiftCluster_GracefulDegradation(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()

	// Simulate a non-OpenShift Kubernetes cluster (no OpenShift CRDs registered)
	h := setupTestHelper(false)

	// IsDisconnectedOCP should return false without error
	disconnected, err := IsDisconnectedOCP(ctx, h)
	g.Expect(err).ToNot(HaveOccurred(), "Should gracefully handle missing CRDs")
	g.Expect(disconnected).To(BeFalse())

	// GetMCRegistryConf should return an error that can be identified as "CRD not found"
	config, err := GetMCRegistryConf(ctx, h)
	g.Expect(err).To(HaveOccurred())
	g.Expect(IsNoMatchError(err)).To(BeTrue(), "Error should indicate CRD is not installed")
	g.Expect(config).To(BeEmpty())
}
