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

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"k8s.io/apimachinery/pkg/types"

	rabbitmqv1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/rabbitmq/v1beta1"
	rabbitmqv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
)

// GetRabbitmqCluster - get RabbitmqCluster object in namespace
func GetRabbitmqCluster(
	ctx context.Context,
	h *helper.Helper,
	instance *rabbitmqv1beta1.TransportURL,
) (*rabbitmqv1.RabbitmqCluster, error) {
	rabbitMqCluster := &rabbitmqv1.RabbitmqCluster{}

	err := h.GetClient().Get(ctx, types.NamespacedName{Name: instance.Spec.RabbitmqClusterName, Namespace: instance.Namespace}, rabbitMqCluster)

	return rabbitMqCluster, err
}
