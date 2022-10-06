/*

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

package rabbitmq

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"sigs.k8s.io/controller-runtime/pkg/client"

	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
)

//
// GetRabbitmqCluster - get RabbitmqCluster object in namespace
func GetRabbitmqCluster(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	labelSelector map[string]string,
) (*rabbitmqv1.RabbitmqCluster, error) {
	rabbitmqClusterList := &rabbitmqv1.RabbitmqClusterList{}

	listOpts := []client.ListOption{
		client.InNamespace(namespace),
	}

	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}

	err := h.GetClient().List(ctx, rabbitmqClusterList, listOpts...)
	if err != nil {
		return nil, err
	}

	if len(rabbitmqClusterList.Items) > 1 {
		return nil, fmt.Errorf("more then one RabbitmqCluster object found in namespace %s", namespace)
	}

	if len(rabbitmqClusterList.Items) == 0 {
		return nil, k8s_errors.NewNotFound(
			appsv1.Resource("RabbitmqCluster"),
			fmt.Sprintf("No RabbitmqCluster object found in namespace %s", namespace),
		)
	}

	return &rabbitmqClusterList.Items[0], nil
}
