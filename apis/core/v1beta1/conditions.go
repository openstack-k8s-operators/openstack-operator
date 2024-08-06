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

// OpenStackControlPlane Condition Types used by API objects.
const (
	// OpenStackControlPlaneRabbitMQReadyCondition Status=True condition which indicates if RabbitMQ is configured and operational
	OpenStackControlPlaneRabbitMQReadyCondition condition.Type = "OpenStackControlPlaneRabbitMQReady"

	// OpenStackControlPlaneMariaDBReadyCondition Status=True condition which indicates if MariaDB is configured and operational
	OpenStackControlPlaneMariaDBReadyCondition condition.Type = "OpenStackControlPlaneMariaDBReady"

	// OpenStackControlPlaneMemcachedReadyCondition Status=True condition which indicates if Memcached is configured and operational
	OpenStackControlPlaneMemcachedReadyCondition condition.Type = "OpenStackControlPlaneMemcachedReady"

	// OpenStackControlPlaneKeystoneAPIReadyCondition Status=True condition which indicates if KeystoneAPI is configured and operational
	OpenStackControlPlaneKeystoneAPIReadyCondition condition.Type = "OpenStackControlPlaneKeystoneAPIReady"

	// OpenStackControlPlaneExposeKeystoneAPIReadyCondition Status=True condition which indicates if KeystoneAPI is exposed via a route
	OpenStackControlPlaneExposeKeystoneAPIReadyCondition condition.Type = "OpenStackControlPlaneExposeKeystoneAPIReady"

	// OpenStackControlPlanePlacementAPIReadyCondition Status=True condition which indicates if PlacementAPI is configured and operational
	OpenStackControlPlanePlacementAPIReadyCondition condition.Type = "OpenStackControlPlanePlacementAPIReady"

	// OpenStackControlPlaneExposePlacementAPIReadyCondition Status=True condition which indicates if PlacementAPI is exposed via a route
	OpenStackControlPlaneExposePlacementAPIReadyCondition condition.Type = "OpenStackControlPlaneExposePlacementAPIReady"

	// OpenStackControlPlaneGlanceReadyCondition Status=True condition which indicates if Glance is configured and operational
	OpenStackControlPlaneGlanceReadyCondition condition.Type = "OpenStackControlPlaneGlanceReady"

	// OpenStackControlPlaneExposeGlanceReadyCondition Status=True condition which indicates if Glance is exposed via a route
	OpenStackControlPlaneExposeGlanceReadyCondition condition.Type = "OpenStackControlPlaneExposeGlanceReady"

	// OpenStackControlPlaneCinderReadyCondition Status=True condition which indicates if Cinder is configured and operational
	OpenStackControlPlaneCinderReadyCondition condition.Type = "OpenStackControlPlaneCinderReady"

	// OpenStackControlPlaneExposeCinderReadyCondition Status=True condition which indicates if Cinder is exposed via a route
	OpenStackControlPlaneExposeCinderReadyCondition condition.Type = "OpenStackControlPlaneExposeCinderReady"

	// OpenStackControlPlaneOVNReadyCondition Status=True condition which indicates if OVN is configured and operational
	OpenStackControlPlaneOVNReadyCondition condition.Type = "OpenStackControlPlaneOVNReady"

	// OpenStackControlPlaneNeutronReadyCondition Status=True condition which indicates if Neutron is configured and operational
	OpenStackControlPlaneNeutronReadyCondition condition.Type = "OpenStackControlPlaneNeutronReady"

	// OpenStackControlPlaneExposeNeutronReadyCondition Status=True condition which indicates if Neutron is exposed via a route
	OpenStackControlPlaneExposeNeutronReadyCondition condition.Type = "OpenStackControlPlaneExposeNeutronReady"

	// OpenStackControlPlaneNovaReadyCondition Status=True condition which indicates if Nova is configured and operational
	OpenStackControlPlaneNovaReadyCondition condition.Type = "OpenStackControlPlaneNovaReady"

	// OpenStackControlPlaneExposeNovaReadyCondition Status=True condition which indicates if Nova is exposed via a route
	OpenStackControlPlaneExposeNovaReadyCondition condition.Type = "OpenStackControlPlaneExposeNovaReady"

	// OpenStackControlPlaneHeatReadyCondition Status=True condition which indicates if Heat is configured and operational
	OpenStackControlPlaneHeatReadyCondition condition.Type = "OpenStackControlPlaneHeatReady"

	// OpenStackControlPlaneExposeHeatReadyCondition Status=True condition which indicates if Heat is exposed via a route
	OpenStackControlPlaneExposeHeatReadyCondition condition.Type = "OpenStackControlPlaneExposeHeatReady"

	// OpenStackControlPlaneIronicReadyCondition Status=True condition which indicates if Ironic is configured and operational
	OpenStackControlPlaneIronicReadyCondition condition.Type = "OpenStackControlPlaneIronicReady"

	// OpenStackControlPlaneExposeIronicReadyCondition Status=True condition which indicates if Ironic is exposed via a route
	OpenStackControlPlaneExposeIronicReadyCondition condition.Type = "OpenStackControlPlaneExposeIronicReady"

	// OpenStackControlPlaneHorizonReadyCondition Status=True condition which indicates if Horizon is configured and operational
	OpenStackControlPlaneHorizonReadyCondition condition.Type = "OpenStackControlPlaneHorizonReady"

	// OpenStackControlPlaneExposeHorizonReadyCondition Status=True condition which indicates if Horizon is exposed via a route
	OpenStackControlPlaneExposeHorizonReadyCondition condition.Type = "OpenStackControlPlaneExposeHorizonReady"

	// OpenStackControlPlaneClientReadyCondition Status=True condition which indicates if OpenStackClient is configured and operational
	OpenStackControlPlaneClientReadyCondition condition.Type = "OpenStackControlPlaneClientReady"

	// OpenStackClientReadyCondition Status=True condition which indicates if OpenStackClient is configured and operational
	OpenStackClientReadyCondition condition.Type = "OpenStackClientReady"

	// OpenStackControlPlaneManilaReadyCondition Status=True condition which indicates if Manila is configured and operational
	OpenStackControlPlaneManilaReadyCondition condition.Type = "OpenStackControlPlaneManilaReady"

	// OpenStackControlPlaneExposeManilaReadyCondition Status=True condition which indicates if Manila is exposed via a route
	OpenStackControlPlaneExposeManilaReadyCondition condition.Type = "OpenStackControlPlaneExposeManilaReady"

	// OpenStackControlPlaneDNSReadyCondition Status=True condition which indicates if DNSMasq is configured and operational
	OpenStackControlPlaneDNSReadyCondition condition.Type = "OpenStackControlPlaneDNSReadyCondition"

	// OpenStackControlPlaneCAReadyCondition Status=True condition which indicates if the CAs are configured and operational
	OpenStackControlPlaneCAReadyCondition condition.Type = "OpenStackControlPlaneCAReadyCondition"

	// OpenStackControlPlaneCustomTLSReadyCondition Status=True condition which indicates if custom TLS certificate secrets are configured and operational
	OpenStackControlPlaneCustomTLSReadyCondition condition.Type = "OpenStackControlPlaneCustomTLSReadyCondition"

	// OpenStackControlPlaneTelemetryReadyCondition Status=True condition which indicates if OpenStack Telemetry service is configured and operational
	OpenStackControlPlaneTelemetryReadyCondition condition.Type = "OpenStackControlPlaneTelemetryReady"

	// OpenStackControlPlaneExposeTelemetryReadyCondition Status=True condition which indicates if Telemetry is exposed via a route
	OpenStackControlPlaneExposeTelemetryReadyCondition condition.Type = "OpenStackControlPlaneExposeTelemetryReady"

	// OpenStackControlPlaneServiceOverrideReadyCondition Status=True condition which indicates if OpenStack service override has created ok
	OpenStackControlPlaneServiceOverrideReadyCondition condition.Type = "OpenStackControlPlaneServiceOverrideReady"

	// OpenStackControlPlaneSwiftReadyCondition Status=True condition which indicates if Swift is configured and operational
	OpenStackControlPlaneSwiftReadyCondition condition.Type = "OpenStackControlPlaneSwiftReady"

	// OpenStackControlPlaneExposeSwiftReadyCondition Status=True condition which indicates if Swift is exposed via a route
	OpenStackControlPlaneExposeSwiftReadyCondition condition.Type = "OpenStackControlPlaneExposeSwiftReady"

	// OpenStackControlPlaneOctaviaReadyCondition Status=True condition which indicates if Octavia is configured and operational
	OpenStackControlPlaneOctaviaReadyCondition condition.Type = "OpenStackControlPlaneOctaviaReady"

	// OpenStackControlPlaneDesignateReadyCondition Status=True condition which indicates if Designate is configured and operational
	OpenStackControlPlaneDesignateReadyCondition condition.Type = "OpenStackControlPlaneDesignateReady"

	// OpenStackControlPlaneBarbicanReadyCondition Status=True condition which indicates if Barbican is configured and operational
	OpenStackControlPlaneBarbicanReadyCondition condition.Type = "OpenStackControlPlaneBarbicanReady"

	// OpenStackControlPlaneExposeOctaviaReadyCondition Status=True condition which indicates if Octavia is exposed via a route
	OpenStackControlPlaneExposeOctaviaReadyCondition condition.Type = "OpenStackControlPlaneExposeOctaviaReady"

	// OpenStackControlPlaneExposeDesignateReadyCondition Status=True condition which indicates if Designate is exposed via a route
	OpenStackControlPlaneExposeDesignateReadyCondition condition.Type = "OpenStackControlPlaneExposeDesignateReady"

	// OpenStackControlPlaneExposeBarbicanReadyCondition Status=True condition which indicates if Barbican is exposed via a route
	OpenStackControlPlaneExposeBarbicanReadyCondition condition.Type = "OpenStackControlPlaneExposeBarbicanReady"

	// OpenStackControlPlaneTestCMReadyCondition Status=True condition which indicates if Test operator CM is ready
	OpenStackControlPlaneTestCMReadyCondition condition.Type = "OpenStackControlPlaneTestCMReadyCondition"
)

// Common Messages used by API objects.
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

	// OpenStackControlPlaneMemcachedReadyInitMessage
	OpenStackControlPlaneMemcachedReadyInitMessage = "OpenStackControlPlane Memcached not started"

	// OpenStackControlPlaneMemcachedReadyMessage
	OpenStackControlPlaneMemcachedReadyMessage = "OpenStackControlPlane Memcached completed"

	// OpenStackControlPlaneMemcachedReadyRunningMessage
	OpenStackControlPlaneMemcachedReadyRunningMessage = "OpenStackControlPlane Memcached in progress"

	// OpenStackControlPlaneMemcachedReadyErrorMessage
	OpenStackControlPlaneMemcachedReadyErrorMessage = "OpenStackControlPlane Memcached error occured %s"

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

	// OpenStackControlPlaneHeatReadyInitMessage
	OpenStackControlPlaneHeatReadyInitMessage = "OpenStackControlPlane Heat not started"

	// OpenStackControlPlaneHeatReadyMessage
	OpenStackControlPlaneHeatReadyMessage = "OpenStackControlPlane Heat completed"

	// OpenStackControlPlaneHeatReadyRunningMessage
	OpenStackControlPlaneHeatReadyRunningMessage = "OpenStackControlPlane Heat in progress"

	// OpenStackControlPlaneHeatReadyErrorMessage
	OpenStackControlPlaneHeatReadyErrorMessage = "OpenStackControlPlane Heat error occured %s"

	// OpenStackControlPlaneIronicReadyInitMessage
	OpenStackControlPlaneIronicReadyInitMessage = "OpenStackControlPlane Ironic not started"

	// OpenStackControlPlaneIronicReadyMessage
	OpenStackControlPlaneIronicReadyMessage = "OpenStackControlPlane Ironic completed"

	// OpenStackControlPlaneIronicReadyRunningMessage
	OpenStackControlPlaneIronicReadyRunningMessage = "OpenStackControlPlane Ironic in progress"

	// OpenStackControlPlaneIronicReadyErrorMessage
	OpenStackControlPlaneIronicReadyErrorMessage = "OpenStackControlPlane Ironic error occured %s"

	// OpenStackControlPlaneClientReadyInitMessage
	OpenStackControlPlaneClientReadyInitMessage = "OpenStackControlPlane Client not started"

	// OpenStackControlPlaneClientReadyMessage
	OpenStackControlPlaneClientReadyMessage = "OpenStackControlPlane Client completed"

	// OpenStackControlPlaneClientReadyRunningMessage
	OpenStackControlPlaneClientReadyRunningMessage = "OpenStackControlPlane Client in progress"

	// OpenStackControlPlaneClientReadyErrorMessage
	OpenStackControlPlaneClientReadyErrorMessage = "OpenStackControlPlane Client error occured %s"

	// OpenStackControlPlaneHorizonReadyInitMessage
	OpenStackControlPlaneHorizonReadyInitMessage = "OpenStackControlPlane Horizon not started"

	// OpenStackControlPlaneHorizonReadyMessage
	OpenStackControlPlaneHorizonReadyMessage = "OpenStackControlPlane Horizon completed"

	// OpenStackControlPlaneHorizonReadyRunningMessage
	OpenStackControlPlaneHorizonReadyRunningMessage = "OpenStackControlPlane Horizon in progress"

	// OpenStackControlPlaneHorizonReadyErrorMessage
	OpenStackControlPlaneHorizonReadyErrorMessage = "OpenStackControlPlane Horizon error occured %s"

	// OpenStackControlPlaneDNSReadyInitMessage
	OpenStackControlPlaneDNSReadyInitMessage = "OpenStackControlPlane DNS not started"

	// OpenStackControlPlaneDNSReadyMessage
	OpenStackControlPlaneDNSReadyMessage = "OpenStackControlPlane DNS completed"

	// OpenStackControlPlaneDNSReadyRunningMessage
	OpenStackControlPlaneDNSReadyRunningMessage = "OpenStackControlPlane DNS in progress"

	// OpenStackControlPlaneDNSReadyErrorMessage
	OpenStackControlPlaneDNSReadyErrorMessage = "OpenStackControlPlane DNS error occured %s"

	// OpenStackControlPlaneTelemetryReadyInitMessage
	OpenStackControlPlaneTelemetryReadyInitMessage = "OpenStackControlPlane Telemetry not started"

	// OpenStackControlPlaneTelemetryReadyMessage
	OpenStackControlPlaneTelemetryReadyMessage = "OpenStackControlPlane Telemetry completed"

	// OpenStackControlPlaneTelemetryReadyRunningMessage
	OpenStackControlPlaneTelemetryReadyRunningMessage = "OpenStackControlPlane Telemetry in progress"

	// OpenStackControlPlaneTelemetryReadyErrorMessage
	OpenStackControlPlaneTelemetryReadyErrorMessage = "OpenStackControlPlane Telemetry error occured %s"

	// OpenStackControlPlaneOctaviaReadyInitMessage
	OpenStackControlPlaneOctaviaReadyInitMessage = "OpenStackControlPlane Octavia not started"

	// OpenStackControlPlaneOctaviaReadyMessage
	OpenStackControlPlaneOctaviaReadyMessage = "OpenStackControlPlane Octavia completed"

	// OpenStackControlPlaneOctaviaReadyRunningMessage
	OpenStackControlPlaneOctaviaReadyRunningMessage = "OpenStackControlPlane Octavia in progress"

	// OpenStackControlPlaneOctaviaReadyErrorMessage
	OpenStackControlPlaneOctaviaReadyErrorMessage = "OpenStackControlPlane Octavia error occured %s"

	// OpenStackControlPlaneDesignateReadyInitMessage
	OpenStackControlPlaneDesignateReadyInitMessage = "OpenStackControlPlane Designate not started"

	// OpenStackControlPlaneDesignateReadyMessage
	OpenStackControlPlaneDesignateReadyMessage = "OpenStackControlPlane Designate completed"

	// OpenStackControlPlaneDesignateReadyRunningMessage
	OpenStackControlPlaneDesignateReadyRunningMessage = "OpenStackControlPlane Designate in progress"

	// OpenStackControlPlaneDesignateReadyErrorMessage
	OpenStackControlPlaneDesignateReadyErrorMessage = "OpenStackControlPlane Designate error occured %s"

	// OpenStackControlPlaneBarbicanReadyInitMessage
	OpenStackControlPlaneBarbicanReadyInitMessage = "OpenStackControlPlane Barbican not started"

	// OpenStackControlPlaneBarbicanReadyMessage
	OpenStackControlPlaneBarbicanReadyMessage = "OpenStackControlPlane Barbican completed"

	// OpenStackControlPlaneBarbicanReadyRunningMessage
	OpenStackControlPlaneBarbicanReadyRunningMessage = "OpenStackControlPlane Barbican in progress"

	// OpenStackControlPlaneBarbicanReadyErrorMessage
	OpenStackControlPlaneBarbicanReadyErrorMessage = "OpenStackControlPlane Barbican error occured %s"

	// OpenStackControlPlaneSwiftReadyInitMessage
	OpenStackControlPlaneSwiftReadyInitMessage = "OpenStackControlPlane Swift not started"

	// OpenStackControlPlaneSwiftReadyMessage
	OpenStackControlPlaneSwiftReadyMessage = "OpenStackControlPlane Swift completed"

	// OpenStackControlPlaneSwiftReadyRunningMessage
	OpenStackControlPlaneSwiftReadyRunningMessage = "OpenStackControlPlane Swift in progress"

	// OpenStackControlPlaneSwiftReadyErrorMessage
	OpenStackControlPlaneSwiftReadyErrorMessage = "OpenStackControlPlane Swift error occured %s"

	// OpenStackControlPlaneManilaReadyInitMessage
	OpenStackControlPlaneManilaReadyInitMessage = "OpenStackControlPlane Manila not started"

	// OpenStackControlPlaneManilaReadyMessage
	OpenStackControlPlaneManilaReadyMessage = "OpenStackControlPlane Manila completed"

	// OpenStackControlPlaneManilaReadyRunningMessage
	OpenStackControlPlaneManilaReadyRunningMessage = "OpenStackControlPlane Manila in progress"

	// OpenStackControlPlaneManilaReadyErrorMessage
	OpenStackControlPlaneManilaReadyErrorMessage = "OpenStackControlPlane Manila error occured %s"

	// OpenStackControlPlaneExposeServiceReadyInitMessage
	OpenStackControlPlaneExposeServiceReadyInitMessage = "OpenStackControlPlane %s exposing service %s not started"

	// OpenStackControlPlaneExposeServiceReadyErrorMessage
	OpenStackControlPlaneExposeServiceReadyErrorMessage = "OpenStackControlPlane %s exposing service via route %s error occured %s"

	// OpenStackControlPlaneExposeServiceReadyMessage
	OpenStackControlPlaneExposeServiceReadyMessage = "OpenStackControlPlane %s service exposed"

	// OpenStackControlPlaneCAReadyInitMessage
	OpenStackControlPlaneCAReadyInitMessage = "OpenStackControlPlane CAs not started"

	// OpenStackControlPlaneCAReadyMessage
	OpenStackControlPlaneCAReadyMessage = "OpenStackControlPlane CAs completed"

	// OpenStackControlPlaneCAReadyRunningMessage
	OpenStackControlPlaneCAReadyRunningMessage = "OpenStackControlPlane CAs in progress"

	// OpenStackControlPlaneCAReadyErrorMessage
	OpenStackControlPlaneCAReadyErrorMessage = "OpenStackControlPlane CAs %s %s error occured %s"

	// OpenStackControlPlaneCAReadyMessage
	OpenStackControlPlaneCustomTLSReadyMessage = "OpenStackControlPlane custom TLS cert secret available"

	// OpenStackControlPlaneCAReadyErrorMessage
	OpenStackControlPlaneCustomTLSReadyErrorMessage = "OpenStackControlPlane custom TLS cert secret %s error occured %s"

	// OpenStackControlPlaneTestCMReadyErrorMessage
	OpenStackControlPlaneTestCMReadyErrorMessage = "OpenStackControlPlane Test Operator CM error occured %s"

	// OpenStackControlPlaneTestCMReadyMessage
	OpenStackControlPlaneTestCMReadyMessage = "OpenStackControlPlane Test Operator CM is available"
)

// Version Conditions used by API objects.
const (
	OpenStackVersionInitialized condition.Type = "Initialized"

	OpenStackVersionMinorUpdateOVNDataplane condition.Type = "MinorUpdateOVNDataplane"

	OpenStackVersionMinorUpdateOVNControlplane condition.Type = "MinorUpdateOVNControlplane"

	OpenStackVersionMinorUpdateControlplane condition.Type = "MinorUpdateControlplane"

	OpenStackVersionMinorUpdateDataplane condition.Type = "MinorUpdateDataplane"
)

// Version Messages used by API objects.
const (

	// OpenStackVersionInitializedInitMessage
	OpenStackVersionInitializedInitMessage = "not started"

	// OpenStackVersionInitializedReadyMessage
	OpenStackVersionInitializedReadyMessage = "completed"

	// OpenStackVersionInitializedReadyRunningMessage
	OpenStackVersionInitializedReadyRunningMessage = "in progress"

	// OpenStackVersionInitializedReadyErrorMessage
	OpenStackVersionInitializedReadyErrorMessage = "error occured %s"

	// OpenStackVersionMinorUpdateInitMessage
	OpenStackVersionMinorUpdateInitMessage = "not started"

	// OpenStackVersionMinorUpdateReadyMessage
	OpenStackVersionMinorUpdateReadyMessage = "completed"

	// OpenStackVersionMinorUpdateReadyRunningMessage
	OpenStackVersionMinorUpdateReadyRunningMessage = "in progress"

	// OpenStackVersionMinorUpdateReadyErrorMessage
	//OpenStackVersionMinorUpdateReadyErrorMessage = "error occured %s"
)
