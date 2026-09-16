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

package operator

import (
	"testing"

	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func makeKeystoneCRD(annotationNS, serviceNS string) *uns.Unstructured {
	obj := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]interface{}{
			"name": "keystoneapis.keystone.openstack.org",
			"annotations": map[string]interface{}{
				"cert-manager.io/inject-ca-from": annotationNS + "/keystone-operator-serving-cert",
			},
		},
		"spec": map[string]interface{}{
			"conversion": map[string]interface{}{
				"strategy": "Webhook",
				"webhook": map[string]interface{}{
					"clientConfig": map[string]interface{}{
						"service": map[string]interface{}{
							"name":      "keystone-operator-webhook-service",
							"namespace": serviceNS,
							"path":      "/convert",
						},
					},
				},
			},
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition",
	})
	return obj
}

func TestRewriteCRDNamespaces_RewritesBothFields(t *testing.T) {
	obj := makeKeystoneCRD("keystone-operator-system", "keystone-operator-system")
	if err := rewriteCRDNamespaces(obj, "openstack-operators"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	annos := obj.GetAnnotations()
	got := annos["cert-manager.io/inject-ca-from"]
	want := "openstack-operators/keystone-operator-serving-cert"
	if got != want {
		t.Errorf("inject-ca-from: got %q, want %q", got, want)
	}

	svcNs, _, _ := uns.NestedString(obj.Object, "spec", "conversion", "webhook", "clientConfig", "service", "namespace")
	if svcNs != "openstack-operators" {
		t.Errorf("service.namespace: got %q, want %q", svcNs, "openstack-operators")
	}
}

func TestRewriteCRDNamespaces_AlreadyCorrect(t *testing.T) {
	obj := makeKeystoneCRD("openstack-operators", "openstack-operators")
	if err := rewriteCRDNamespaces(obj, "openstack-operators"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	annos := obj.GetAnnotations()
	if got := annos["cert-manager.io/inject-ca-from"]; got != "openstack-operators/keystone-operator-serving-cert" {
		t.Errorf("annotation changed unexpectedly: %q", got)
	}
}

func TestRewriteCRDNamespaces_AnnotationOnlyNoConversionSection(t *testing.T) {
	// CRD has the inject-ca-from annotation but no spec.conversion.webhook.clientConfig.service
	// (e.g. a CRD that uses cainjection for a non-conversion purpose). Service rewrite is a no-op.
	obj := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata": map[string]interface{}{
			"name": "somecrd.example.org",
			"annotations": map[string]interface{}{
				"cert-manager.io/inject-ca-from": "source-ns/some-cert",
			},
		},
		"spec": map[string]interface{}{},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition",
	})
	if err := rewriteCRDNamespaces(obj, "openstack-operators"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	annos := obj.GetAnnotations()
	if got := annos["cert-manager.io/inject-ca-from"]; got != "openstack-operators/some-cert" {
		t.Errorf("inject-ca-from not rewritten: got %q", got)
	}
}

func TestRewriteCRDNamespaces_NoAnnotation(t *testing.T) {
	obj := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]interface{}{"name": "no-annotation.example.org"},
		"spec":       map[string]interface{}{},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition",
	})
	if err := rewriteCRDNamespaces(obj, "openstack-operators"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRewriteCRDNamespaces_SkipsNonCRD(t *testing.T) {
	obj := &uns.Unstructured{Object: map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name": "test",
			"annotations": map[string]interface{}{
				"cert-manager.io/inject-ca-from": "wrong-ns/some-cert",
			},
		},
	}}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	if err := rewriteCRDNamespaces(obj, "openstack-operators"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-CRD: annotation must NOT be touched
	if got := obj.GetAnnotations()["cert-manager.io/inject-ca-from"]; got != "wrong-ns/some-cert" {
		t.Errorf("non-CRD annotation was rewritten: %q", got)
	}
}
