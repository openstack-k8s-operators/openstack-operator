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
// OpenStackControlPlane Condition Types used by API objects.
//
const (
	// OpenStackControlPlaneRabbitMQReadyCondition Status=True condition which indicates if RabbitMQ is configured and operational
	OpenStackControlPlaneRabbitMQReadyCondition condition.Type = "OpenStackControlPlaneRabbitMQReady"

	// OpenStackControlPlaneMariaDBReadyCondition Status=True condition which indicates if MariaDB is configured and operational
	OpenStackControlPlaneMariaDBReadyCondition condition.Type = "OpenStackControlPlaneMariaDBReady"

	// OpenStackControlPlaneKeystoneAPIReadyCondition Status=True condition which indicates if KeystoneAPI is configured and operational
	OpenStackControlPlaneKeystoneAPIReadyCondition condition.Type = "OpenStackControlPlaneKeystoneAPIReady"

	// OpenStackControlPlanePlacementAPIReadyCondition Status=True condition which indicates if PlacementAPI is configured and operational
	OpenStackControlPlanePlacementAPIReadyCondition condition.Type = "OpenStackControlPlanePlacementAPIReady"

	// OpenStackControlPlaneGlanceReadyCondition Status=True condition which indicates if Glance is configured and operational
	OpenStackControlPlaneGlanceReadyCondition condition.Type = "OpenStackControlPlaneGlanceReady"

	// OpenStackControlPlaneCinderReadyCondition Status=True condition which indicates if Cinder is configured and operational
	OpenStackControlPlaneCinderReadyCondition condition.Type = "OpenStackControlPlaneCinderReady"

	// OpenStackControlPlaneOVNReadyCondition Status=True condition which indicates if OVN is configured and operational
	OpenStackControlPlaneOVNReadyCondition condition.Type = "OpenStackControlPlaneOVNReady"

	// OpenStackControlPlaneOVSReadyCondition Status=True condition which indicates if OVS is configured and operational
	OpenStackControlPlaneOVSReadyCondition condition.Type = "OpenStackControlPlaneOVSReady"

	// OpenStackControlPlaneNeutronReadyCondition Status=True condition which indicates if Neutron is configured and operational
	OpenStackControlPlaneNeutronReadyCondition condition.Type = "OpenStackControlPlaneNeutronReady"

	// OpenStackControlPlaneNovaReadyCondition Status=True condition which indicates if Nova is configured and operational
	OpenStackControlPlaneNovaReadyCondition condition.Type = "OpenStackControlPlaneNovaReady"

	// OpenStackControlPlaneClientReadyCondition Status=True condition which indicates if OpenStackClient is configured and operational
	OpenStackControlPlaneClientReadyCondition condition.Type = "OpenStackControlPlaneClientReady"

	// OpenStackClientReadyCondition Status=True condition which indicates if OpenStackClient is configured and operational
	OpenStackClientReadyCondition condition.Type = "OpenStackClientReady"
)

//
// OpenStackControlPlane Reasons used by API objects.
//
const ()

//
// Common Messages used by API objects.
//
const (
	//
	// OpenStackControlPlaneReady condition messages
	//

	// OpenStackControlPlaneReadyErrorMessage
	OpenStackControlPlaneReadyErrorMessage = "OpenStackControlPlane error occured %s"

	// OpenStackControlPlaneRabbitMQReadyInitMessage
	OpenStackControlPlaneRabbitMQReadyInitMessage = "OpenStackControlPlane RabbitMQ not started"

	// OpenStackControlPlaneRabbitMQReadyMessage
	OpenStackControlPlaneRabbitMQReadyMessage = "OpenStackControlPlane RabbitMQ completed"

	// OpenStackControlPlaneRabbitMQReadyRunningMessage
	OpenStackControlPlaneRabbitMQReadyRunningMessage = "OpenStackControlPlane RabbitMQ in progress"

	// OpenStackControlPlaneRabbitMQReadyErrorMessage
	OpenStackControlPlaneRabbitMQReadyErrorMessage = "OpenStackControlPlane RabbitMQ error occured %s"

	// OpenStackControlPlaneMariaDBReadyInitMessage
	OpenStackControlPlaneMariaDBReadyInitMessage = "OpenStackControlPlane MariaDB not started"

	// OpenStackControlPlaneMariaDBReadyMessage
	OpenStackControlPlaneMariaDBReadyMessage = "OpenStackControlPlane MariaDB completed"

	// OpenStackControlPlaneMariaDBReadyRunningMessage
	OpenStackControlPlaneMariaDBReadyRunningMessage = "OpenStackControlPlane MariaDB in progress"

	// OpenStackControlPlaneMariaDBReadyErrorMessage
	OpenStackControlPlaneMariaDBReadyErrorMessage = "OpenStackControlPlane MariaDB error occured %s"

	// OpenStackControlPlaneKeystoneAPIReadyInitMessage
	OpenStackControlPlaneKeystoneAPIReadyInitMessage = "OpenStackControlPlane KeystoneAPI not started"

	// OpenStackControlPlaneKeystoneAPIReadyMessage
	OpenStackControlPlaneKeystoneAPIReadyMessage = "OpenStackControlPlane KeystoneAPI completed"

	// OpenStackControlPlaneKeystoneAPIReadyRunningMessage
	OpenStackControlPlaneKeystoneAPIReadyRunningMessage = "OpenStackControlPlane KeystoneAPI in progress"

	// OpenStackControlPlaneKeystoneAPIReadyErrorMessage
	OpenStackControlPlaneKeystoneAPIReadyErrorMessage = "OpenStackControlPlane KeystoneAPI error occured %s"

	// OpenStackControlPlanePlacementAPIReadyInitMessage
	OpenStackControlPlanePlacementAPIReadyInitMessage = "OpenStackControlPlane PlacementAPI not started"

	// OpenStackControlPlanePlacementAPIReadyMessage
	OpenStackControlPlanePlacementAPIReadyMessage = "OpenStackControlPlane PlacementAPI completed"

	// OpenStackControlPlanePlacementAPIReadyRunningMessage
	OpenStackControlPlanePlacementAPIReadyRunningMessage = "OpenStackControlPlane PlacementAPI in progress"

	// OpenStackControlPlanePlacementAPIReadyErrorMessage
	OpenStackControlPlanePlacementAPIReadyErrorMessage = "OpenStackControlPlane PlacementAPI error occured %s"

	// OpenStackControlPlaneGlanceReadyInitMessage
	OpenStackControlPlaneGlanceReadyInitMessage = "OpenStackControlPlane Glance not started"

	// OpenStackControlPlaneGlanceReadyMessage
	OpenStackControlPlaneGlanceReadyMessage = "OpenStackControlPlane Glance completed"

	// OpenStackControlPlaneGlanceReadyRunningMessage
	OpenStackControlPlaneGlanceReadyRunningMessage = "OpenStackControlPlane Glance in progress"

	// OpenStackControlPlaneGlanceReadyErrorMessage
	OpenStackControlPlaneGlanceReadyErrorMessage = "OpenStackControlPlane Glance error occured %s"

	// OpenStackControlPlaneCinderReadyInitMessage
	OpenStackControlPlaneCinderReadyInitMessage = "OpenStackControlPlane Cinder not started"

	// OpenStackControlPlaneCinderReadyMessage
	OpenStackControlPlaneCinderReadyMessage = "OpenStackControlPlane Cinder completed"

	// OpenStackControlPlaneCinderReadyRunningMessage
	OpenStackControlPlaneCinderReadyRunningMessage = "OpenStackControlPlane Cinder in progress"

	// OpenStackControlPlaneCinderReadyErrorMessage
	OpenStackControlPlaneCinderReadyErrorMessage = "OpenStackControlPlane Cinder error occured %s"

	// OpenStackControlPlaneOVNReadyInitMessage
	OpenStackControlPlaneOVNReadyInitMessage = "OpenStackControlPlane OVN not started"

	// OpenStackControlPlaneOVNReadyMessage
	OpenStackControlPlaneOVNReadyMessage = "OpenStackControlPlane OVN completed"

	// OpenStackControlPlaneOVNReadyRunningMessage
	OpenStackControlPlaneOVNReadyRunningMessage = "OpenStackControlPlane OVN in progress"

	// OpenStackControlPlaneOVNReadyErrorMessage
	OpenStackControlPlaneOVNReadyErrorMessage = "OpenStackControlPlane OVN error occured %s"

	// OpenStackControlPlaneOVSReadyInitMessage
	OpenStackControlPlaneOVSReadyInitMessage = "OpenStackControlPlane OVS not started"

	// OpenStackControlPlaneOVSReadyMessage
	OpenStackControlPlaneOVSReadyMessage = "OpenStackControlPlane OVS completed"

	// OpenStackControlPlaneOVSReadyRunningMessage
	OpenStackControlPlaneOVSReadyRunningMessage = "OpenStackControlPlane OVS in progress"

	// OpenStackControlPlaneOVSReadyErrorMessage
	OpenStackControlPlaneOVSReadyErrorMessage = "OpenStackControlPlane OVS error occured %s"

	// OpenStackControlPlaneNeutronReadyInitMessage
	OpenStackControlPlaneNeutronReadyInitMessage = "OpenStackControlPlane Neutron not started"

	// OpenStackControlPlaneNeutronReadyMessage
	OpenStackControlPlaneNeutronReadyMessage = "OpenStackControlPlane Neutron completed"

	// OpenStackControlPlaneNeutronReadyRunningMessage
	OpenStackControlPlaneNeutronReadyRunningMessage = "OpenStackControlPlane Neutron in progress"

	// OpenStackControlPlaneNeutronReadyErrorMessage
	OpenStackControlPlaneNeutronReadyErrorMessage = "OpenStackControlPlane Neutron error occured %s"

	// OpenStackControlPlaneNovaReadyInitMessage
	OpenStackControlPlaneNovaReadyInitMessage = "OpenStackControlPlane Nova not started"

	// OpenStackControlPlaneNovaReadyMessage
	OpenStackControlPlaneNovaReadyMessage = "OpenStackControlPlane Nova completed"

	// OpenStackControlPlaneNovaReadyRunningMessage
	OpenStackControlPlaneNovaReadyRunningMessage = "OpenStackControlPlane Nova in progress"

	// OpenStackControlPlaneNovaReadyErrorMessage
	OpenStackControlPlaneNovaReadyErrorMessage = "OpenStackControlPlane Nova error occured %s"

	// OpenStackControlPlaneClientReadyInitMessage
	OpenStackControlPlaneClientReadyInitMessage = "OpenStackControlPlane Client not started"

	// OpenStackControlPlaneClientReadyMessage
	OpenStackControlPlaneClientReadyMessage = "OpenStackControlPlane Client completed"

	// OpenStackControlPlaneClientReadyRunningMessage
	OpenStackControlPlaneClientReadyRunningMessage = "OpenStackControlPlane Client in progress"

	// OpenStackControlPlaneClientReadyErrorMessage
	OpenStackControlPlaneClientReadyErrorMessage = "OpenStackControlPlane Client error occured %s"

	// OpenStackClientReadyInitMessage
	OpenStackClientReadyInitMessage = "OpenStack Client not started, waiting on keystone API"

	// OpenStackClientKeystoneWaitingMessage
	OpenStackClientKeystoneWaitingMessage = "OpenStack Client keystone API not yet ready"

	// OpenStackClientConfigMapWaitingMessage
	OpenStackClientConfigMapWaitingMessage = "OpenStack Client waiting for keystone configmap"

	// OpenStackClientSecretWaitingMessage
	OpenStackClientSecretWaitingMessage = "OpenStack Client waiting for secret"

	// OpenStackClientInputReady
	OpenStackClientInputReady = "OpenStack Client input ready"

	// OpenStackClientReadyMessage
	OpenStackClientReadyMessage = "OpenStack Client created"

	// OpenStackClientReadyErrorMessage
	OpenStackClientReadyErrorMessage = "OpenStack Client error occured %s"
)
