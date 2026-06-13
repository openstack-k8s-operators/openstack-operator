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

package deployment

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
)

// BuildServiceLevels takes a flat list of service names, looks up each
// service's DependsOn field, and returns a list of execution levels.
// Services within the same level have all their dependencies satisfied by
// earlier levels and may run in parallel.
//
// When fallbackToListOrder is true, services without an explicit dependsOn
// inherit an implicit dependency on the preceding service in the list.
// When false, only explicit dependsOn entries affect the DAG.
// Dependencies referencing services not present in the current list are
// silently skipped.
func BuildServiceLevels(ctx context.Context, helper *helper.Helper,
	serviceCache *ServiceCache,
	services []string,
	fallbackToListOrder bool,
) ([][]string, error) {

	if len(services) == 0 {
		return nil, nil
	}

	deps, err := loadDependencies(ctx, helper, serviceCache, services, fallbackToListOrder)
	if err != nil {
		return nil, err
	}

	return topoLevelSort(services, deps)
}

// loadDependencies fetches every service CR and collects its DependsOn
// list. Each service has an effective service type (EDPMServiceType, or
// CR name as fallback). The dependsOn entries are resolved against these
// service types; references to services not in the list are skipped.
func loadDependencies(ctx context.Context, helper *helper.Helper,
	serviceCache *ServiceCache,
	services []string,
	fallbackToListOrder bool,
) (map[string][]string, error) {

	// Build effective serviceType <-> CR name lookups from the service list.
	typeToName := make(map[string]string, len(services))
	nameToType := make(map[string]string, len(services))
	serviceObjects := make(map[string]dataplanev1.OpenStackDataPlaneService, len(services))

	for _, svc := range services {
		service, err := serviceCache.Get(ctx, helper, svc)
		if err != nil {
			return nil, err
		}
		serviceObjects[svc] = service
		serviceType := service.Spec.EDPMServiceType
		typeToName[serviceType] = svc
		nameToType[svc] = serviceType
	}

	deps := make(map[string][]string, len(services))

	// With fallback enabled, services without dependsOn follow predecessor
	// order. Otherwise only explicit dependsOn entries participate in the DAG.
	for i, svc := range services {
		service := serviceObjects[svc]
		seen := make(map[string]struct{})
		var resolved []string
		addDep := func(dep string) {
			if _, ok := seen[dep]; ok {
				return
			}
			seen[dep] = struct{}{}
			resolved = append(resolved, dep)
		}

		if len(service.Spec.DependsOn) > 0 {
			for _, dep := range service.Spec.DependsOn {
				target, err := resolveDependency(dep, svc, nameToType[svc], typeToName, nameToType)
				if err != nil {
					return nil, err
				}
				if target != "" {
					addDep(target)
				}
			}
		} else if fallbackToListOrder && i > 0 {
			addDep(services[i-1])
		}

		deps[svc] = resolved
	}

	return deps, nil
}

// resolveDependency resolves a single dependsOn entry (service type or
// CR name) to a CR name in the current list. Dependencies referencing
// services not in the list are silently skipped (returns empty string).
func resolveDependency(
	dep string,
	sourceSvc string,
	sourceType string,
	typeToName map[string]string,
	nameToType map[string]string,
) (string, error) {
	if _, ok := typeToName[dep]; !ok {
		if resolvedType, ok := nameToType[dep]; ok {
			dep = resolvedType
		}
	}

	if dep == sourceType {
		return "", fmt.Errorf("service %q has a self-dependency", sourceSvc)
	}

	if target, ok := typeToName[dep]; ok {
		if target == sourceSvc {
			return "", fmt.Errorf("service %q has a self-dependency", sourceSvc)
		}
		return target, nil
	}

	return "", nil
}

// topoLevelSort performs a Kahn-style topological sort that groups nodes
// into parallel execution levels. Returns an error on cycles.
func topoLevelSort(services []string, deps map[string][]string) ([][]string, error) {
	inDegree := make(map[string]int, len(services))
	dependents := make(map[string][]string, len(services))

	for _, svc := range services {
		if _, ok := inDegree[svc]; !ok {
			inDegree[svc] = 0
		}
		for _, dep := range deps[svc] {
			inDegree[svc]++
			dependents[dep] = append(dependents[dep], svc)
		}
	}

	// Seed level 0 with all services that have no dependencies, in original
	// list order for determinism.
	var current []string
	for _, svc := range services {
		if inDegree[svc] == 0 {
			current = append(current, svc)
		}
	}

	var levels [][]string
	visited := 0

	for len(current) > 0 {
		levels = append(levels, current)
		visited += len(current)

		// Build the next level: services whose in-degree drops to 0.
		// Collect into a set first, then emit in original list order.
		ready := make(map[string]struct{})
		for _, svc := range current {
			for _, dep := range dependents[svc] {
				inDegree[dep]--
				if inDegree[dep] == 0 {
					ready[dep] = struct{}{}
				}
			}
		}

		var next []string
		for _, svc := range services {
			if _, ok := ready[svc]; ok {
				next = append(next, svc)
			}
		}
		current = next
	}

	if visited != len(services) {
		return nil, fmt.Errorf("circular dependency detected among services")
	}

	return levels, nil
}
