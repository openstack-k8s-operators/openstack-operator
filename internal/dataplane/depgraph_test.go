package deployment

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	_, err = resolveDependency("self-service", "self-service", "", typeToName, nameToType)
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
