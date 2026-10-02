/*
Copyright 2026.

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
	"slices"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane/util"
)

// JobState caches AnsibleEE Job outcomes for one reconcile pass. Deployment
// scoped services share one Job name across nodesets.
type JobState map[string]bool

// Deployment scoped services are looked up without the nodeset label: their
// single Job is not nodeset owned.
func serviceJobCompleted(
	ctx context.Context,
	helper *helper.Helper,
	deployment *dataplanev1.OpenStackDataPlaneDeployment,
	service dataplanev1.OpenStackDataPlaneService,
	nodeSetName string,
	state JobState,
) (bool, error) {
	name, labels := dataplaneutil.GetAnsibleExecutionNameAndLabels(
		&service, deployment.GetName(), nodeSetName)
	if service.Spec.DeployOnAllNodeSets {
		labels = dataplaneutil.GetGlobalAnsibleExecutionLabels(service.Name, deployment.GetName())
	}

	if completed, ok := state[name]; ok {
		return completed, nil
	}

	ansibleJob, err := dataplaneutil.GetAnsibleExecution(ctx, helper, deployment, labels)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			state[name] = false
			return false, nil
		}
		return false, err
	}

	completed := ansibleJob.Status.Succeeded > 0
	state[name] = completed
	return completed, nil
}

// GlobalServiceGateOpen reports whether a deployment scoped service may start.
// Every service ordered before it in each listing nodeset must have completed;
// nodesets that do not list it impose no constraint.
func GlobalServiceGateOpen(
	ctx context.Context,
	helper *helper.Helper,
	plan *ServicePlan,
	deployment *dataplanev1.OpenStackDataPlaneDeployment,
	service string,
	serviceLevels map[string][][]string,
	state JobState,
) (bool, error) {
	for _, nodeSetName := range plan.NodesetsWithGlobalService[service] {
		for _, level := range serviceLevels[nodeSetName] {
			if slices.Contains(level, service) {
				// levels are ordered, so everything before it completed
				break
			}
			for _, predecessor := range level {
				foundService, err := plan.Cache.Get(ctx, helper, predecessor)
				if err != nil {
					return false, err
				}
				completed, err := serviceJobCompleted(ctx, helper, deployment, foundService, nodeSetName, state)
				if err != nil {
					return false, err
				}
				if !completed {
					return false, nil
				}
			}
		}
	}
	return true, nil
}
