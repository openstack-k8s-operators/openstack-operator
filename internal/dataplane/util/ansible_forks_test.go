package util //nolint:revive // util is an acceptable package name in this context

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestDefaultAnsibleForks(t *testing.T) {
	tests := []struct {
		name, request, limit string
		want                 int64
		wantOK               bool
	}{
		{name: "unset"},
		{name: "two cores", request: "2", want: 2, wantOK: true},
		{name: "fractional", request: "1500m", want: 2, wantOK: true},
		{name: "sub core", request: "250m", want: 1, wantOK: true},
		{name: "request capped", request: "4", limit: "1500m", want: 2, wantOK: true},
		{name: "request below limit", request: "250m", limit: "4", want: 1, wantOK: true},
		{name: "limit alone", limit: "3", want: 3, wantOK: true},
		{name: "zero request", request: "0"},
		{name: "negative request", request: "-1"},
		{name: "zero limit", limit: "0"},
		{name: "negative limit", limit: "-1"},
		{name: "above int32", limit: "2147483648", want: 2147483648, wantOK: true},
		{name: "int32 maximum", limit: "2147483647", want: 2147483647, wantOK: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := corev1.ResourceRequirements{}
			if tt.request != "" {
				resources.Requests = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(tt.request)}
			}
			if tt.limit != "" {
				resources.Limits = corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(tt.limit)}
			}
			got, ok := defaultAnsibleForks(resources)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("defaultAnsibleForks(%v) = %d, %t; want %d, %t", resources, got, ok, tt.want, tt.wantOK)
			}
		})
	}
	t.Run("memory alone", func(t *testing.T) {
		got, ok := defaultAnsibleForks(corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")}})
		if ok || got != 0 {
			t.Fatalf("memory-only forks = %d, %t; want 0, false", got, ok)
		}
	})
}

func TestAddDefaultAnsibleForks(t *testing.T) {
	tests := []struct {
		name, configValue, explicit, want            string
		configExists, configHasForks, explicitExists bool
	}{
		{name: "missing optional configmap", want: "2"},
		{name: "configmap without forks", configExists: true, want: "2"},
		{name: "custom configmap", configExists: true, configHasForks: true, configValue: "12"},
		{name: "empty custom value", configExists: true, configHasForks: true, configValue: ""},
		{name: "invalid custom value", configExists: true, configHasForks: true, configValue: "not-a-number"},
		{name: "nodeset wins", configExists: true, configHasForks: true, configValue: "12", explicitExists: true, explicit: "3", want: "3"},
		{name: "empty nodeset wins", configExists: true, configHasForks: true, configValue: "12", explicitExists: true, explicit: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var objects []client.Object
			if tt.configExists {
				data := map[string]string{"OTHER": "unchanged"}
				if tt.configHasForks {
					data["ANSIBLE_FORKS"] = tt.configValue
				}
				objects = append(objects, &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "chosen-env", Namespace: "test-namespace"}, Data: data})
			}
			h := setupTestHelper(false, objects...)
			env := []corev1.EnvVar{{Name: "OTHER", Value: "unchanged"}}
			if tt.explicitExists {
				env = append(env, corev1.EnvVar{Name: "ANSIBLE_FORKS", Value: tt.explicit})
			}
			job := EEJob{Namespace: "test-namespace", EnvConfigMapName: "chosen-env", Env: env,
				Resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1500m")}}}
			if err := job.addDefaultAnsibleForks(context.Background(), h.GetClient()); err != nil {
				t.Fatal(err)
			}
			if job.Env[0].Value != "unchanged" {
				t.Fatal("other environment variable changed")
			}
			count := 0
			for _, e := range job.Env {
				if e.Name == "ANSIBLE_FORKS" {
					count++
					if !tt.configHasForks || tt.explicitExists {
						if e.Value != tt.want {
							t.Fatalf("forks = %q; want %q", e.Value, tt.want)
						}
					}
				}
			}
			if tt.configHasForks && !tt.explicitExists {
				if count != 0 {
					t.Fatalf("ConfigMap forks masked by %v", job.Env)
				}
			} else if count != 1 {
				t.Fatalf("want exactly one explicit forks source: %v", job.Env)
			}
		})
	}
}

func TestAddDefaultAnsibleForksFallback(t *testing.T) {
	tests := []struct {
		name      string
		resources corev1.ResourceRequirements
	}{
		{name: "unset"},
		{name: "memory alone", resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")}}},
		{name: "zero CPU", resources: corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("0")}}},
		{name: "negative CPU", resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("-1")}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := EEJob{Namespace: "test-namespace", EnvConfigMapName: "chosen-env", Resources: tt.resources}
			if err := job.addDefaultAnsibleForks(context.Background(), setupTestHelper(false).GetClient()); err != nil {
				t.Fatal(err)
			}
			if len(job.Env) != 1 || job.Env[0].Name != "ANSIBLE_FORKS" || job.Env[0].Value != "5" {
				t.Fatalf("fallback environment = %v; want ANSIBLE_FORKS=5", job.Env)
			}
		})
	}
}

type failingConfigMapClient struct{ client.Client }

func (c failingConfigMapClient) Get(_ context.Context, _ client.ObjectKey, _ client.Object, _ ...client.GetOption) error {
	return errors.New("connection lost")
}

func TestConfigMapReadFailure(t *testing.T) {
	a := EEJob{Namespace: "test-namespace", EnvConfigMapName: "chosen-env"}
	err := a.addDefaultAnsibleForks(context.Background(), failingConfigMapClient{setupTestHelper(false).GetClient()})
	if err == nil {
		t.Fatal("non-NotFound ConfigMap errors must not be swallowed")
	}
}
