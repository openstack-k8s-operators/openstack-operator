package core

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"testing"

	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/nova/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
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

// setTLSAPISecrets walks v and sets a unique secretName on both endpoints of
// every tls.APIService found, collecting the names.
func setTLSAPISecrets(v reflect.Value, names *[]string) {
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == tlsAPIServiceType {
			api := v.Addr().Interface().(*tls.APIService)
			for _, svc := range []*tls.GenericService{&api.Public, &api.Internal} {
				name := fmt.Sprintf("tls-secret-%d", len(*names))
				svc.SecretName = &name
				*names = append(*names, name)
			}
			return
		}
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				setTLSAPISecrets(v.Field(i), names)
			}
		}
	case reflect.Pointer:
		if v.IsNil() && v.Type().Elem().Kind() == reflect.Struct {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if !v.IsNil() {
			setTLSAPISecrets(v.Elem(), names)
		}
	case reflect.Map:
		if v.Type().Elem().Kind() == reflect.Struct {
			v.Set(reflect.MakeMap(v.Type()))
			elem := reflect.New(v.Type().Elem()).Elem()
			setTLSAPISecrets(elem, names)
			v.SetMapIndex(reflect.ValueOf("key").Convert(v.Type().Key()), elem)
		}
	}
}

// TestTLSServiceSecretNames verifies the tls.api secrets of every service in the
// spec (incl. map values like glance) are collected.
func TestTLSServiceSecretNames(t *testing.T) {
	spec := &corev1beta1.OpenStackControlPlaneSpec{}
	if got := tlsServiceSecretNames(spec); len(got) != 0 {
		t.Fatalf("expected no secrets for empty spec, got %v", got)
	}

	var want []string
	setTLSAPISecrets(reflect.ValueOf(spec).Elem(), &want)
	if len(want) == 0 {
		t.Fatal("found no tls.api fields in the spec")
	}
	got := tlsServiceSecretNames(spec)
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Errorf("tls.api secret names not collected\nwant: %v\ngot:  %v", want, got)
	}

	// empty secretName is ignored
	spec = &corev1beta1.OpenStackControlPlaneSpec{}
	spec.Keystone.Template = &keystonev1.KeystoneAPISpecCore{}
	spec.Keystone.Template.TLS.API.Public.SecretName = ptr.To("")
	if got := tlsServiceSecretNames(spec); len(got) != 0 {
		t.Errorf("expected empty secretName to be ignored, got %v", got)
	}
}

// TestTLSServiceSecretNamesSimpleTLS verifies the simple TLS sections the operator
// reads as bring-your-own are indexed, and the operator owned ones are not.
func TestTLSServiceSecretNamesSimpleTLS(t *testing.T) {
	spec := &corev1beta1.OpenStackControlPlaneSpec{}
	spec.Horizon.Template = &horizonv1.HorizonSpecCore{}
	spec.Horizon.Template.TLS.SecretName = ptr.To("horizon-cert")
	spec.Nova.Template = &novav1.NovaSpecCore{CellTemplates: map[string]novav1.NovaCellTemplate{"cell1": {}}}
	cell := spec.Nova.Template.CellTemplates["cell1"]
	cell.NoVNCProxyServiceTemplate.TLS.Service.SecretName = ptr.To("novnc-cert")
	cell.NoVNCProxyServiceTemplate.TLS.Vencrypt.SecretName = ptr.To("vencrypt-operator-owned")
	spec.Nova.Template.CellTemplates["cell1"] = cell
	spec.Telemetry.Template = &telemetryv1.TelemetrySpecCore{}
	spec.Telemetry.Template.MetricStorage.PrometheusTLS.SecretName = ptr.To("prometheus-cert")
	spec.Telemetry.Template.MetricStorage.AlertmanagerTLS.SecretName = ptr.To("alertmanager-cert")
	spec.Neutron.Template = &neutronv1.NeutronAPISpecCore{}
	spec.Neutron.Template.TLS.Ovn.SecretName = ptr.To("ovn-operator-owned")

	got := tlsServiceSecretNames(spec)
	slices.Sort(got)
	want := []string{"alertmanager-cert", "horizon-cert", "novnc-cert", "prometheus-cert"}
	if !slices.Equal(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

// TestFindObjectsForSrcTLSServiceSecret verifies a change of a BYO service cert
// secret enqueues the control plane referencing it.
func TestFindObjectsForSrcTLSServiceSecret(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := corev1beta1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	cp := &corev1beta1.OpenStackControlPlane{ObjectMeta: metav1.ObjectMeta{Name: "cp", Namespace: "ns"}}
	cp.Spec.Keystone.Template = &keystonev1.KeystoneAPISpecCore{}
	cp.Spec.Keystone.Template.TLS.API.Internal.SecretName = ptr.To("byo-cert")

	builder := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cp)
	for field, indexer := range fieldIndexers {
		builder = builder.WithIndex(&corev1beta1.OpenStackControlPlane{}, field, indexer)
	}
	r := &OpenStackControlPlaneReconciler{Client: builder.Build()}

	secret := func(name string) client.Object {
		return &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}}
	}
	if got := r.findObjectsForSrc(context.Background(), secret("byo-cert")); len(got) != 1 || got[0].Name != "cp" {
		t.Errorf("expected control plane to be enqueued, got %v", got)
	}
	if got := r.findObjectsForSrc(context.Background(), secret("other")); len(got) != 0 {
		t.Errorf("expected no requests for unrelated secret, got %v", got)
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
