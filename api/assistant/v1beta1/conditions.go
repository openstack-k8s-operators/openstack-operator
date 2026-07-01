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

// OpenStackAssistant Condition Types used by API objects.
const (
	// OpenStackAssistantReadyCondition Status=True condition which indicates if OpenStackAssistant is configured and operational
	OpenStackAssistantReadyCondition condition.Type = "OpenStackAssistantReady"
)

// Common Messages used by API objects.
const (
	// OpenStackAssistantReadyInitMessage
	OpenStackAssistantReadyInitMessage = "OpenStack Assistant not started"

	// OpenStackAssistantReadyRunningMessage
	OpenStackAssistantReadyRunningMessage = "OpenStack Assistant in progress"

	// OpenStackAssistantReadyMessage
	OpenStackAssistantReadyMessage = "OpenStack Assistant created"

	// OpenStackAssistantReadyErrorMessage
	OpenStackAssistantReadyErrorMessage = "OpenStack Assistant error occured %s"

	// OpenStackAssistantProviderSecretWaitingMessage
	OpenStackAssistantProviderSecretWaitingMessage = "Waiting for lightspeed provider secret"

	// OpenStackAssistantRecipesWaitingMessage
	OpenStackAssistantRecipesWaitingMessage = "Waiting for Goose recipes ConfigMap"

	// OpenStackAssistantHintsWaitingMessage
	OpenStackAssistantHintsWaitingMessage = "Waiting for Goose hints ConfigMap"
)
