package v1beta1

import (
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
)

// Condition types for OpenStackBackupConfig
const (
	// OpenStackBackupConfigSecretsReadyCondition - Secrets labeling status
	OpenStackBackupConfigSecretsReadyCondition condition.Type = "SecretsReady"

	// OpenStackBackupConfigConfigMapsReadyCondition - ConfigMaps labeling status
	OpenStackBackupConfigConfigMapsReadyCondition condition.Type = "ConfigMapsReady"

	// OpenStackBackupConfigNADsReadyCondition - NetworkAttachmentDefinitions labeling status
	OpenStackBackupConfigNADsReadyCondition condition.Type = "NADsReady"

	// OpenStackBackupConfigIssuersReadyCondition - cert-manager Issuers labeling status
	OpenStackBackupConfigIssuersReadyCondition condition.Type = "IssuersReady"

	// OpenStackBackupConfigCRsReadyCondition - CR instances labeling status
	OpenStackBackupConfigCRsReadyCondition condition.Type = "CRsReady"
)
