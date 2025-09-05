package bindata

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	//sprig "github.com/go-task/slim-sprig/v3"
	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// RenderData -
type RenderData struct {
	Funcs template.FuncMap
	Data  map[string]interface{}
}

// MakeRenderData -
func MakeRenderData() RenderData {
	return RenderData{
		Funcs: template.FuncMap{},
		Data:  map[string]interface{}{},
	}
}

// RenderDir will render all manifests in a directory, descending in to subdirectories
// It will perform template substitutions based on the data supplied by the RenderData
func RenderDir(manifestDir string, d *RenderData) ([]*unstructured.Unstructured, error) {
	out := []*unstructured.Unstructured{}

	if err := filepath.Walk(manifestDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Skip non-manifest files
		if !strings.HasSuffix(path, ".yml") && !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".json") {
			return nil
		}

		objs, err := RenderTemplate(path, d)
		if err != nil {
			return err
		}
		out = append(out, objs...)
		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "error rendering manifests")
	}

	return out, nil
}

// hasEnvVar checks if an EnvVar with a specific name exists in a slice.
func hasEnvVar(envVars []corev1.EnvVar, name string) bool {
	for _, env := range envVars {
		if env.Name == name {
			return true
		}
	}
	return false
}

// isEnvVarTrue checks if an EnvVar with a specific name exists
// and if its value is "true" (case-insensitive).
func isEnvVarTrue(envVars []corev1.EnvVar, name string) bool {
	for _, env := range envVars {
		if env.Name == name {
			// Found the right env var, now check its value.
			// Using ToLower for a case-insensitive comparison ("true", "True", etc.)
			return strings.ToLower(env.Value) == "true"
		}
	}
	// Return false if the env var was not found at all.
	return false
}

// RenderTemplate reads, renders, and attempts to parse a yaml or
// json file representing one or more k8s api objects
func RenderTemplate(path string, d *RenderData) ([]*unstructured.Unstructured, error) {
	// Create a FuncMap to register our function.
	funcMap := template.FuncMap{
		"hasEnvVar":    hasEnvVar,
		"isEnvVarTrue": isEnvVarTrue,
	}

	tmpl := template.New(path).Option("missingkey=error").Funcs(funcMap)
	if d.Funcs != nil {
		tmpl.Funcs(d.Funcs)
	}

	// Add universal functions
	//tmpl.Funcs(sprig.TxtFuncMap())

	source, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read manifest %s", path)
	}

	if _, err := tmpl.Parse(string(source)); err != nil {
		return nil, errors.Wrapf(err, "failed to parse manifest %s as template", path)
	}

	rendered := bytes.Buffer{}
	if err := tmpl.Execute(&rendered, d.Data); err != nil {
		return nil, errors.Wrapf(err, "failed to render manifest %s", path)
	}

	out := []*unstructured.Unstructured{}

	// special case - if the entire file is whitespace, skip
	if len(strings.TrimSpace(rendered.String())) == 0 {
		return out, nil
	}

	decoder := yaml.NewYAMLOrJSONDecoder(&rendered, 4096)
	for {
		u := unstructured.Unstructured{}
		if err := decoder.Decode(&u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, errors.Wrapf(err, "failed to unmarshal manifest %s", path)
		}
		out = append(out, &u)
	}

	return out, nil
}
