package bindata

import (
	"testing"

	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// makeWebhook builds a single webhook entry for use in test fixtures.
func makeWebhook(name, path string, caBundle interface{}) map[string]interface{} {
	cc := map[string]interface{}{
		"service": map[string]interface{}{
			"name":      "webhook-service",
			"namespace": "openstack-operators",
			"path":      path,
		},
	}
	if caBundle != nil {
		cc["caBundle"] = caBundle
	}
	return map[string]interface{}{
		"name":         name,
		"clientConfig": cc,
		"rules": []interface{}{
			map[string]interface{}{
				"apiGroups": []interface{}{"example.org"},
				"resources": []interface{}{"things"},
			},
		},
	}
}

// makeWebhookConfig builds an Unstructured MutatingWebhookConfiguration.
func makeWebhookConfig(webhooks ...map[string]interface{}) *uns.Unstructured {
	whList := make([]interface{}, len(webhooks))
	for i, w := range webhooks {
		whList[i] = w
	}
	obj := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "admissionregistration.k8s.io/v1",
		"kind":       "MutatingWebhookConfiguration",
		"metadata":   map[string]interface{}{"name": "test"},
		"webhooks":   whList,
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "admissionregistration.k8s.io",
		Version: "v1",
		Kind:    "MutatingWebhookConfiguration",
	})
	return obj
}

func getWebhookPath(obj *uns.Unstructured, index int) string {
	wh := obj.Object["webhooks"].([]interface{})[index].(map[string]interface{})
	cc := wh["clientConfig"].(map[string]interface{})
	svc := cc["service"].(map[string]interface{})
	return svc["path"].(string)
}

func getWebhookName(obj *uns.Unstructured, index int) string {
	wh := obj.Object["webhooks"].([]interface{})[index].(map[string]interface{})
	return wh["name"].(string)
}

func getWebhookCABundle(obj *uns.Unstructured, index int) interface{} {
	wh := obj.Object["webhooks"].([]interface{})[index].(map[string]interface{})
	cc := wh["clientConfig"].(map[string]interface{})
	return cc["caBundle"]
}

func TestMergeWebhookPreservesPathsWhenOrderDiffers(t *testing.T) {
	// Simulate the bug scenario: Kubernetes sorts webhooks alphabetically by name,
	// but the template has a different order. The merge must match by name, not index.

	// Current (on cluster): sorted alphabetically by name, with caBundle from cert-manager
	current := makeWebhookConfig(
		makeWebhook("mmemcached-v1beta1.kb.io", "/mutate-memcached", "CABUNDLE-memcached"),
		makeWebhook("mrabbitmq-v1beta1.kb.io", "/mutate-rabbitmq", "CABUNDLE-rabbitmq"),
		makeWebhook("mreservation-v1beta1.kb.io", "/mutate-reservation", "CABUNDLE-reservation"),
	)

	// Updated (from template): different order, no caBundle
	updated := makeWebhookConfig(
		makeWebhook("mreservation-v1beta1.kb.io", "/mutate-reservation", nil),
		makeWebhook("mmemcached-v1beta1.kb.io", "/mutate-memcached", nil),
		makeWebhook("mrabbitmq-v1beta1.kb.io", "/mutate-rabbitmq", nil),
	)

	if err := MergeWebhookConfigurationForUpdate(current, updated); err != nil {
		t.Fatalf("MergeWebhookConfigurationForUpdate failed: %v", err)
	}

	// Verify each webhook kept its correct path and got the right caBundle
	tests := []struct {
		index          int
		expectedName   string
		expectedPath   string
		expectedBundle string
	}{
		{0, "mreservation-v1beta1.kb.io", "/mutate-reservation", "CABUNDLE-reservation"},
		{1, "mmemcached-v1beta1.kb.io", "/mutate-memcached", "CABUNDLE-memcached"},
		{2, "mrabbitmq-v1beta1.kb.io", "/mutate-rabbitmq", "CABUNDLE-rabbitmq"},
	}

	for _, tc := range tests {
		name := getWebhookName(updated, tc.index)
		path := getWebhookPath(updated, tc.index)
		bundle := getWebhookCABundle(updated, tc.index)

		if name != tc.expectedName {
			t.Errorf("webhook[%d]: got name %q, want %q", tc.index, name, tc.expectedName)
		}
		if path != tc.expectedPath {
			t.Errorf("webhook[%d] (%s): got path %q, want %q", tc.index, name, path, tc.expectedPath)
		}
		if bundle != tc.expectedBundle {
			t.Errorf("webhook[%d] (%s): got caBundle %v, want %q", tc.index, name, bundle, tc.expectedBundle)
		}
	}
}

func TestMergeWebhookNoCaBundleInCurrent(t *testing.T) {
	// When the current webhook has no caBundle (e.g. cert-manager hasn't injected yet),
	// the updated webhook should remain unchanged.
	current := makeWebhookConfig(
		makeWebhook("mfoo.kb.io", "/mutate-foo", nil),
	)
	updated := makeWebhookConfig(
		makeWebhook("mfoo.kb.io", "/mutate-foo", nil),
	)

	if err := MergeWebhookConfigurationForUpdate(current, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bundle := getWebhookCABundle(updated, 0); bundle != nil {
		t.Errorf("expected no caBundle, got %v", bundle)
	}
	if path := getWebhookPath(updated, 0); path != "/mutate-foo" {
		t.Errorf("expected path /mutate-foo, got %s", path)
	}
}

func TestMergeWebhookNewWebhookInUpdated(t *testing.T) {
	// When the updated template has a new webhook that doesn't exist in current,
	// it should be left as-is (no caBundle).
	current := makeWebhookConfig(
		makeWebhook("mfoo.kb.io", "/mutate-foo", "CABUNDLE-foo"),
	)
	updated := makeWebhookConfig(
		makeWebhook("mfoo.kb.io", "/mutate-foo", nil),
		makeWebhook("mbar.kb.io", "/mutate-bar", nil),
	)

	if err := MergeWebhookConfigurationForUpdate(current, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Existing webhook gets its caBundle
	if bundle := getWebhookCABundle(updated, 0); bundle != "CABUNDLE-foo" {
		t.Errorf("webhook[0]: expected caBundle CABUNDLE-foo, got %v", bundle)
	}
	// New webhook has no caBundle
	if bundle := getWebhookCABundle(updated, 1); bundle != nil {
		t.Errorf("webhook[1]: expected no caBundle, got %v", bundle)
	}
}

func TestMergeWebhookRemovedFromUpdated(t *testing.T) {
	// When a webhook exists in current but was removed from the updated template,
	// it should not appear in the result.
	current := makeWebhookConfig(
		makeWebhook("mfoo.kb.io", "/mutate-foo", "CABUNDLE-foo"),
		makeWebhook("mbar.kb.io", "/mutate-bar", "CABUNDLE-bar"),
	)
	updated := makeWebhookConfig(
		makeWebhook("mfoo.kb.io", "/mutate-foo", nil),
	)

	if err := MergeWebhookConfigurationForUpdate(current, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	webhooks := updated.Object["webhooks"].([]interface{})
	if len(webhooks) != 1 {
		t.Fatalf("expected 1 webhook, got %d", len(webhooks))
	}
	if name := getWebhookName(updated, 0); name != "mfoo.kb.io" {
		t.Errorf("expected mfoo.kb.io, got %s", name)
	}
}

func TestMergeWebhookSkipsNonWebhookResources(t *testing.T) {
	// Non-webhook resources should pass through unchanged.
	current := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "test"},
	}}
	current.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

	updated := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata":   map[string]interface{}{"name": "test"},
	}}
	updated.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})

	if err := MergeWebhookConfigurationForUpdate(current, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMergeWebhookValidatingConfig(t *testing.T) {
	// The fix should also work for ValidatingWebhookConfiguration.
	current := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "admissionregistration.k8s.io/v1",
		"kind":       "ValidatingWebhookConfiguration",
		"metadata":   map[string]interface{}{"name": "test"},
		"webhooks": []interface{}{
			makeWebhook("vbar.kb.io", "/validate-bar", "CABUNDLE-bar"),
			makeWebhook("vfoo.kb.io", "/validate-foo", "CABUNDLE-foo"),
		},
	}}
	current.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration",
	})

	updated := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "admissionregistration.k8s.io/v1",
		"kind":       "ValidatingWebhookConfiguration",
		"metadata":   map[string]interface{}{"name": "test"},
		"webhooks": []interface{}{
			makeWebhook("vfoo.kb.io", "/validate-foo", nil),
			makeWebhook("vbar.kb.io", "/validate-bar", nil),
		},
	}}
	updated.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "admissionregistration.k8s.io", Version: "v1", Kind: "ValidatingWebhookConfiguration",
	})

	if err := MergeWebhookConfigurationForUpdate(current, updated); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// vfoo should get vfoo's caBundle, not vbar's
	if bundle := getWebhookCABundle(updated, 0); bundle != "CABUNDLE-foo" {
		t.Errorf("webhook[0] (vfoo): expected CABUNDLE-foo, got %v", bundle)
	}
	if bundle := getWebhookCABundle(updated, 1); bundle != "CABUNDLE-bar" {
		t.Errorf("webhook[1] (vbar): expected CABUNDLE-bar, got %v", bundle)
	}
	if path := getWebhookPath(updated, 0); path != "/validate-foo" {
		t.Errorf("webhook[0]: expected /validate-foo, got %s", path)
	}
	if path := getWebhookPath(updated, 1); path != "/validate-bar" {
		t.Errorf("webhook[1]: expected /validate-bar, got %s", path)
	}
}
