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

package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// OpenStack Condition Types used by API objects.
const (
	// OpenStackOperatorReadyCondition Status=True condition which indicates if operators have been deployed
	OpenStackOperatorReadyCondition condition.Type = "OpenStackOperatorReadyCondition"
)

// Common Messages used by Openstack operator
const (
	//
	// OpenStackOperator condition messages
	//

	// OpenStackOperatorErrorMessage
	OpenStackOperatorErrorMessage = "OpenStackOperator error occured %s"

	// OpenStackOperatorReadyInitMessage
	OpenStackOperatorReadyInitMessage = "OpenStackOperator not started"

	// OpenStackOperatorReadyRunningMessage
	OpenStackOperatorReadyRunningMessage = "OpenStackOperator in progress"

	// OpenStackOperatorReadyMessage
	OpenStackOperatorReadyMessage = "OpenStackOperator completed"
)
