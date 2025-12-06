/*
Copyright 2025.

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

// Package operator provides functionality for managing operator overrides and configurations
package operator

import (
	"slices"

	operatorv1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/operator/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

// custom struct types for rendering the operator gotemplates

// Operator -
type Operator struct {
	Name       string
	Namespace  string
	Deployment Deployment
}

// Deployment -
type Deployment struct {
	Replicas      *int32
	Manager       Container
	KubeRbacProxy Container
	Tolerations   []corev1.Toleration
}

// Container -
type Container struct {
	Image     string
	Env       []corev1.EnvVar
	Resources Resource
}

// Resource -
type Resource struct {
	Requests *ResourceList
	Limits   *ResourceList
}

// ResourceList -
type ResourceList struct {
	CPU    string // using string here instead of resource.Quantity since this can not be direct used in the gotemplate
	Memory string
}

func cpuQuantity(milli int64) *resource.Quantity {
	q := resource.NewMilliQuantity(milli, resource.DecimalSI)
	return q
}

func memQuantity(mega int64) *resource.Quantity {
	q := resource.NewQuantity(mega*1024*1024, resource.BinarySI)
	return q
}

// HasOverrides checks if the given operator name has overrides in the provided list
func HasOverrides(operatorOverrides []operatorv1beta1.OperatorSpec, operatorName string) *operatorv1beta1.OperatorSpec {
	// validate of operatorName is in the list of operatorOverrides
	f := func(c operatorv1beta1.OperatorSpec) bool {
		return c.Name == operatorName
	}
	idx := slices.IndexFunc(operatorOverrides, f)
	if idx >= 0 {
		return &operatorOverrides[idx]
	}

	return nil
}

// SetOverrides applies the provided operator overrides to the operator configuration
func SetOverrides(opOvr operatorv1beta1.OperatorSpec, op *Operator) {
	if opOvr.Replicas != nil {
		op.Deployment.Replicas = opOvr.Replicas
	}
	if opOvr.ControllerManager.Resources.Limits != nil {
		if op.Deployment.Manager.Resources.Limits == nil {
			op.Deployment.Manager.Resources.Limits = &ResourceList{}
		}
		if opOvr.ControllerManager.Resources.Limits.Cpu() != nil && opOvr.ControllerManager.Resources.Limits.Cpu().Value() > 0 {
			op.Deployment.Manager.Resources.Limits.CPU = opOvr.ControllerManager.Resources.Limits.Cpu().String()
		}
		if opOvr.ControllerManager.Resources.Limits.Memory() != nil && opOvr.ControllerManager.Resources.Limits.Memory().Value() > 0 {
			op.Deployment.Manager.Resources.Limits.Memory = opOvr.ControllerManager.Resources.Limits.Memory().String()
		}
	}
	if opOvr.ControllerManager.Resources.Requests != nil {
		if op.Deployment.Manager.Resources.Requests == nil {
			op.Deployment.Manager.Resources.Requests = &ResourceList{}
		}
		if opOvr.ControllerManager.Resources.Requests.Cpu() != nil && opOvr.ControllerManager.Resources.Requests.Cpu().Value() > 0 {
			op.Deployment.Manager.Resources.Requests.CPU = opOvr.ControllerManager.Resources.Requests.Cpu().String()
		}
		if opOvr.ControllerManager.Resources.Requests.Memory() != nil && opOvr.ControllerManager.Resources.Requests.Memory().Value() > 0 {
			op.Deployment.Manager.Resources.Requests.Memory = opOvr.ControllerManager.Resources.Requests.Memory().String()
		}
	}
	if len(opOvr.ControllerManager.Env) > 0 {
		op.Deployment.Manager.Env = mergeEnvVars(op.Deployment.Manager.Env, opOvr.ControllerManager.Env)
	}
	if len(opOvr.Tolerations) > 0 {
		op.Deployment.Tolerations = mergeTolerations(op.Deployment.Tolerations, opOvr.Tolerations)
	}
}

// mergeEnvVars merges custom environment variables with default environment variables.
// If a custom env var has the same name as a default one, it overrides the default.
// Otherwise, the custom env var is added to the list.
func mergeEnvVars(defaults, custom []corev1.EnvVar) []corev1.EnvVar {
	if len(custom) == 0 {
		return defaults
	}

	// Start with a copy of defaults
	merged := make([]corev1.EnvVar, len(defaults))
	copy(merged, defaults)

	// For each custom env var, check if it should override a default one
	for _, customEnv := range custom {
		f := func(c corev1.EnvVar) bool {
			return c.Name == customEnv.Name
		}
		idx := slices.IndexFunc(merged, f)
		if idx >= 0 {
			merged[idx] = customEnv
		} else {
			merged = append(merged, customEnv)
		}
	}

	return merged
}

// mergeTolerations merges custom tolerations with default tolerations.
// If a custom toleration has the same key as a default one, it overrides the default.
// Otherwise, the custom toleration is added to the list.
func mergeTolerations(defaults, custom []corev1.Toleration) []corev1.Toleration {
	if len(custom) == 0 {
		return defaults
	}

	// Start with a copy of defaults
	merged := make([]corev1.Toleration, len(defaults))
	copy(merged, defaults)

	// For each custom toleration, check if it should override a default one
	for _, customTol := range custom {

		f := func(c corev1.Toleration) bool {
			return c.Key == customTol.Key
		}
		idx := slices.IndexFunc(merged, f)
		if idx >= 0 {
			merged[idx] = customTol
		} else {
			merged = append(merged, customTol)
		}
	}

	return merged
}

// GetOperator finds and returns the operator with the given name from the list
func GetOperator(operators []Operator, name string) (int, Operator) {
	f := func(c Operator) bool {
		return c.Name == name
	}
	idx := slices.IndexFunc(operators, f)
	if idx >= 0 {
		return idx, operators[idx]
	}

	return idx, Operator{}
}
