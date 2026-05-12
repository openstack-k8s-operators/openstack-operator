package deployment

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTopoLevelSort(t *testing.T) {
	tests := []struct {
		name     string
		services []string
		deps     map[string][]string
		want     [][]string
		wantErr  string
	}{
		{
			name:     "no deps - single level",
			services: []string{"a", "b", "c"},
			deps:     map[string][]string{"a": nil, "b": nil, "c": nil},
			want:     [][]string{{"a", "b", "c"}},
		},
		{
			name:     "linear chain",
			services: []string{"a", "b", "c"},
			deps:     map[string][]string{"a": nil, "b": {"a"}, "c": {"b"}},
			want:     [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name:     "diamond",
			services: []string{"a", "b", "c", "d"},
			deps:     map[string][]string{"a": nil, "b": {"a"}, "c": {"a"}, "d": {"b", "c"}},
			want:     [][]string{{"a"}, {"b", "c"}, {"d"}},
		},
		{
			name:     "multiple roots",
			services: []string{"x", "y", "z", "w"},
			deps:     map[string][]string{"x": nil, "y": nil, "z": {"x"}, "w": {"y"}},
			want:     [][]string{{"x", "y"}, {"z", "w"}},
		},
		{
			name:     "single service",
			services: []string{"only"},
			deps:     map[string][]string{"only": nil},
			want:     [][]string{{"only"}},
		},
		{
			name:     "preserves order within level",
			services: []string{"c", "b", "a", "d"},
			deps:     map[string][]string{"c": nil, "b": nil, "a": nil, "d": {"c", "b", "a"}},
			want:     [][]string{{"c", "b", "a"}, {"d"}},
		},
		{
			name:     "complex DAG",
			services: []string{"bootstrap", "configure-network", "install-os", "ovn", "libvirt", "nova"},
			deps: map[string][]string{
				"bootstrap": nil, "configure-network": {"bootstrap"}, "install-os": {"bootstrap"},
				"ovn": {"configure-network"}, "libvirt": {"install-os"}, "nova": {"ovn", "libvirt"},
			},
			want: [][]string{{"bootstrap"}, {"configure-network", "install-os"}, {"ovn", "libvirt"}, {"nova"}},
		},
		{
			name:     "cycle detected",
			services: []string{"a", "b", "c"},
			deps:     map[string][]string{"a": {"c"}, "b": {"a"}, "c": {"b"}},
			wantErr:  "circular dependency",
		},
		{
			name:     "two-node cycle",
			services: []string{"a", "b"},
			deps:     map[string][]string{"a": {"b"}, "b": {"a"}},
			wantErr:  "circular dependency",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels, err := topoLevelSort(tt.services, tt.deps)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Len(t, levels, len(tt.want))
			for i := range tt.want {
				assert.ElementsMatch(t, tt.want[i], levels[i], "level %d", i)
			}
		})
	}
}

func newTestHelper(t *testing.T, services ...*dataplanev1.OpenStackDataPlaneService) *helper.Helper {
	t.Helper()
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = dataplanev1.AddToScheme(s)

	var objs []client.Object
	for _, svc := range services {
		objs = append(objs, svc)
	}
	fakeClient := fakeclient.NewClientBuilder().WithScheme(s).WithObjects(objs...).Build()

	mockObj := &dataplanev1.OpenStackDataPlaneDeployment{
		ObjectMeta: metav1.ObjectMeta{Name: "test-deployment", Namespace: "openstack"},
	}
	h, err := helper.NewHelper(mockObj, fakeClient, nil, s, logr.Discard())
	require.NoError(t, err)
	return h
}

func testService(name, serviceType string, dependsOn ...string) *dataplanev1.OpenStackDataPlaneService {
	return &dataplanev1.OpenStackDataPlaneService{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "openstack"},
		Spec: dataplanev1.OpenStackDataPlaneServiceSpec{
			EDPMServiceType: serviceType,
			DependsOn:       dependsOn,
		},
	}
}

func TestLoadDependencies(t *testing.T) {
	ctx := context.Background()

	t.Run("undeclared services get predecessor edge", func(t *testing.T) {
		h := newTestHelper(t,
			testService("a", "type-a"),
			testService("b", "type-b"),
		)
		deps, err := loadDependencies(ctx, h, NewServiceCache(), []string{"a", "b"})
		require.NoError(t, err)
		assert.Empty(t, deps["a"])
		assert.Equal(t, []string{"a"}, deps["b"])
	})

	t.Run("explicit deps resolve by type and CR name", func(t *testing.T) {
		h := newTestHelper(t,
			testService("custom-a", "type-a"),
			testService("b", "type-b", "type-a", "custom-a"),
		)
		deps, err := loadDependencies(ctx, h, NewServiceCache(), []string{"custom-a", "b"})
		require.NoError(t, err)
		assert.Equal(t, []string{"custom-a"}, deps["b"])
	})

	t.Run("missing deps are skipped", func(t *testing.T) {
		h := newTestHelper(t,
			testService("a", "type-a"),
			testService("b", "type-b", "not-in-list"),
		)
		deps, err := loadDependencies(ctx, h, NewServiceCache(), []string{"a", "b"})
		require.NoError(t, err)
		assert.Empty(t, deps["b"])
	})

	t.Run("self dependency errors", func(t *testing.T) {
		h := newTestHelper(t,
			testService("a", "type-a", "type-a"),
		)
		_, err := loadDependencies(ctx, h, NewServiceCache(), []string{"a"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "self-dependency")
	})

	t.Run("self dependency via CR name errors", func(t *testing.T) {
		h := newTestHelper(t,
			testService("custom-a", "type-a", "custom-a"),
		)
		_, err := loadDependencies(ctx, h, NewServiceCache(), []string{"custom-a"})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "self-dependency")
	})
}

func TestBuildServiceLevelsEmpty(t *testing.T) {
	levels, err := BuildServiceLevels(context.Background(), nil, NewServiceCache(), nil)
	require.NoError(t, err)
	assert.Nil(t, levels)
}

func TestResolveDependency(t *testing.T) {
	typeToName := map[string]string{"ovn": "custom-ovn", "nova": "nova"}
	nameToType := map[string]string{"custom-ovn": "ovn", "nova": "nova"}

	target, err := resolveDependency("ovn", "nova", "nova", typeToName, nameToType)
	require.NoError(t, err)
	assert.Equal(t, "custom-ovn", target, "should resolve by type")

	target, err = resolveDependency("custom-ovn", "nova", "nova", typeToName, nameToType)
	require.NoError(t, err)
	assert.Equal(t, "custom-ovn", target, "should resolve by CR name")

	target, err = resolveDependency("missing", "nova", "nova", typeToName, nameToType)
	require.NoError(t, err)
	assert.Equal(t, "", target, "missing dep should be skipped")

	_, err = resolveDependency("nova", "nova", "nova", typeToName, nameToType)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self-dependency")

	typeToName = map[string]string{"self-service": "self-service"}
	nameToType = map[string]string{"self-service": "self-service"}
	_, err = resolveDependency("self-service", "self-service", "self-service", typeToName, nameToType)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self-dependency")

	typeToName = map[string]string{"ovn": "custom-ovn"}
	nameToType = map[string]string{"custom-ovn": "ovn"}
	_, err = resolveDependency("custom-ovn", "custom-ovn", "ovn", typeToName, nameToType)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "self-dependency")
}

func TestTopoLevelSortDependencySemantics(t *testing.T) {
	tests := []struct {
		name string
		deps map[string][]string
		svcs []string
		want [][]string
	}{
		{
			name: "fallback order forms chain",
			svcs: []string{"a", "b", "c"},
			deps: map[string][]string{"a": nil, "b": {"a"}, "c": {"b"}},
			want: [][]string{{"a"}, {"b"}, {"c"}},
		},
		{
			name: "explicit deps only keep multiple roots together",
			svcs: []string{"b", "c", "d", "e"},
			deps: map[string][]string{"b": nil, "c": nil, "d": {"c"}, "e": {"c"}},
			want: [][]string{{"b", "c"}, {"d", "e"}},
		},
		{
			name: "explicit deps do not inherit predecessor order",
			svcs: []string{"a", "b", "c"},
			deps: map[string][]string{"a": nil, "b": nil, "c": {"a"}},
			want: [][]string{{"a", "b"}, {"c"}},
		},
		{
			name: "explicit deps can chain independently",
			svcs: []string{"a", "b", "c", "d1"},
			deps: map[string][]string{"a": nil, "b": nil, "c": {"a"}, "d1": {"c"}},
			want: [][]string{{"a", "b"}, {"c"}, {"d1"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			levels, err := topoLevelSort(tt.svcs, tt.deps)
			require.NoError(t, err)
			require.Len(t, levels, len(tt.want))
			for i := range tt.want {
				assert.ElementsMatch(t, tt.want[i], levels[i], "level %d", i)
			}
		})
	}
}
