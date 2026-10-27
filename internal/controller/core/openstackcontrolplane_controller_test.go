package core

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
)

var overrideType = reflect.TypeFor[corev1beta1.Override]()

// containsOverride returns true if t is, or holds, a corev1beta1.Override.
func containsOverride(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Struct:
		if t == overrideType {
			return true
		}
		if t.PkgPath() != overrideType.PkgPath() {
			return false
		}
		for f := range t.Fields() {
			if f.IsExported() && containsOverride(f.Type) {
				return true
			}
		}
	case reflect.Pointer, reflect.Slice, reflect.Array, reflect.Map:
		return containsOverride(t.Elem())
	}
	return false
}

// setOverrideSecrets walks v and sets a unique TLS secretName on every
// corev1beta1.Override found, including map values, and collects the names.
// It fails on shapes holding an Override that it can't handle, so a new shape
// in the spec can't be silently skipped.
func setOverrideSecrets(t *testing.T, v reflect.Value, names *[]string) {
	t.Helper()
	switch {
	case v.Type() == overrideType:
		name := fmt.Sprintf("secret-%d", len(*names))
		v.Set(reflect.ValueOf(corev1beta1.Override{TLS: &corev1beta1.TLSServiceOverride{SecretName: name}}))
		*names = append(*names, name)
	case v.Kind() == reflect.Struct && v.Type().PkgPath() == overrideType.PkgPath():
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				setOverrideSecrets(t, v.Field(i), names)
			}
		}
	case v.Kind() == reflect.Map && v.Type().Elem().Kind() == reflect.Struct && containsOverride(v.Type().Elem()):
		v.Set(reflect.MakeMap(v.Type()))
		elem := reflect.New(v.Type().Elem()).Elem()
		setOverrideSecrets(t, elem, names)
		v.SetMapIndex(reflect.ValueOf("key").Convert(v.Type().Key()), elem)
	case containsOverride(v.Type()):
		t.Fatalf("unsupported shape %s holding an Override, extend setOverrideSecrets and apiOverrideTLSSecretNames", v.Type())
	}
}

// TestAPIOverrideTLSSecretNames guards against Override fields being added to
// the spec without being added to apiOverrideTLSSecretNames.
func TestAPIOverrideTLSSecretNames(t *testing.T) {
	spec := &corev1beta1.OpenStackControlPlaneSpec{}
	if got := apiOverrideTLSSecretNames(spec); len(got) != 0 {
		t.Fatalf("expected no secrets for empty spec, got %v", got)
	}

	var want []string
	setOverrideSecrets(t, reflect.ValueOf(spec).Elem(), &want)
	if len(want) == 0 {
		t.Fatal("found no Override fields in the spec")
	}
	got := apiOverrideTLSSecretNames(spec)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("secret names not collected for all Override fields\nwant: %v\ngot:  %v", want, got)
	}

	// empty secretName and nil TLS are ignored
	spec = &corev1beta1.OpenStackControlPlaneSpec{}
	spec.Keystone.APIOverride.TLS = &corev1beta1.TLSServiceOverride{}
	if got := apiOverrideTLSSecretNames(spec); len(got) != 0 {
		t.Errorf("expected empty secretName to be ignored, got %v", got)
	}
}

// TestFindObjectsForSrcAPIOverrideTLSSecret verifies a change of a custom
// route cert secret enqueues the control plane referencing it.
func TestFindObjectsForSrcAPIOverrideTLSSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cp := &corev1beta1.OpenStackControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "cp", Namespace: "ns"}}
	cp.Spec.Keystone.APIOverride.TLS = &corev1beta1.TLSServiceOverride{SecretName: "custom-cert"}

	builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cp)
	for field, indexer := range fieldIndexers {
		builder = builder.WithIndex(&corev1beta1.OpenStackControlPlane{}, field, indexer)
	}
	r := &OpenStackControlPlaneReconciler{Client: builder.Build()}

	secret := func(name, ns string) client.Object {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns}}
	}

	if got := r.findObjectsForSrc(context.Background(), secret("custom-cert", "ns")); len(got) != 1 || got[0].Name != "cp" {
		t.Errorf("expected control plane to be enqueued, got %v", got)
	}
	if got := r.findObjectsForSrc(context.Background(), secret("other", "ns")); len(got) != 0 {
		t.Errorf("expected no requests for unrelated secret, got %v", got)
	}
	if got := r.findObjectsForSrc(context.Background(), secret("custom-cert", "other-ns")); len(got) != 0 {
		t.Errorf("expected no requests for secret in another namespace, got %v", got)
	}
}
