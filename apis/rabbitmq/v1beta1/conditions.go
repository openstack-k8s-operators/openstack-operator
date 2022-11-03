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

//
// TransportURL Condition Types used by API objects.
//
const (
	// TransportURLReadyCondition Status=True condition which indicates if TransportURL is configured and operational
	TransportURLReadyCondition condition.Type = "TransportURLReady"
)

//
// TransportURL Reasons used by API objects.
//
const ()

//
// Common Messages used by API objects.
//
const (
	//
	// TransportURLReady condition messages
	//

	// TransportURLReadyErrorMessage
	TransportURLReadyErrorMessage = "TransportURL error occured %s"

	// TransportURLReadyInitMessage
	TransportURLReadyInitMessage = "TransportURL not configured"

	// TransportURLReadyMessage
	TransportURLReadyMessage = "TransportURL completed"

	// TransportURLInProgressMessage
	TransportURLInProgressMessage = "TransportURL in progress"
)
