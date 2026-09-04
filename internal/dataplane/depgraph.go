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

// BuildServiceLevels topologically sorts services into parallel execution
// levels; services without dependsOn depend on their list predecessor.
func BuildServiceLevels(ctx context.Context, helper *helper.Helper,
	serviceCache *ServiceCache,
	services []string,
) ([][]string, error) {

	if len(services) == 0 {
		return nil, nil
	}

	deps, err := loadDependencies(ctx, helper, serviceCache, services)
	if err != nil {
		return nil, err
	}

	return topoLevelSort(services, deps)
}

// loadDependencies resolves dependsOn entries to CR names in the list;
// services without dependsOn get an implicit predecessor edge.
func loadDependencies(ctx context.Context, helper *helper.Helper,
	serviceCache *ServiceCache,
	services []string,
) (map[string][]string, error) {

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
				if target == "" {
					helper.GetLogger().Info("dependsOn entry not in nodeset service list, skipping",
						"service", svc, "dependsOn", dep)
					continue
				}
				addDep(target)
			}
		} else if i > 0 {
			addDep(services[i-1])
		}

		deps[svc] = resolved
	}

	return deps, nil
}

// resolveDependency maps a dependsOn entry (service type or CR name) to a
// CR name in the list; unknown references resolve to "".
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

	// Seed level 0 with dependency-free services, in list order for determinism.
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

		// Next level: newly ready services, emitted in list order for determinism.
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
