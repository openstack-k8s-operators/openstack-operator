/*
Copyright 2023.

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
	"maps"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetNovaCellRabbitMqUserFromSecret extracts the RabbitMQ username from a nova-cellX-compute-config secret
// Returns the username extracted from rabbitmq_user_name field (preferred) or transport_url (fallback)
// As of nova-operator PR #1066, the RabbitMQUser CR name is propagated directly in the
// rabbitmq_user_name and notification_rabbitmq_user_name fields for easier tracking.
func GetNovaCellRabbitMqUserFromSecret(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	cellName string,
) (string, error) {
	// List all secrets in the namespace
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return "", fmt.Errorf("failed to list secrets: %w", err)
	}

	// Pattern to match nova-cellX-compute-config secrets
	// Supports both split secrets (nova-cell1-compute-config-0) and non-split (nova-cell1-compute-config)
	secretPattern := regexp.MustCompile(`^nova-(` + cellName + `)-compute-config(-\d+)?$`)

	for _, secret := range secretList.Items {
		matches := secretPattern.FindStringSubmatch(secret.Name)
		if matches == nil {
			continue
		}

		// Preferred: Use the rabbitmq_user_name field directly if available
		// This field is populated by nova-operator PR #1066 with the RabbitMQUser CR name
		if rabbitmqUserName, ok := secret.Data["rabbitmq_user_name"]; ok && len(rabbitmqUserName) > 0 {
			return string(rabbitmqUserName), nil
		}

		// Fallback: Extract transport_url from secret data for backwards compatibility
		transportURLBytes, ok := secret.Data["transport_url"]
		if !ok {
			// Try to extract from config files as fallback (in case it's embedded)
			// Check both custom.conf and 01-nova.conf
			for _, configKey := range []string{"custom.conf", "01-nova.conf"} {
				customConfig, hasCustom := secret.Data[configKey]
				if !hasCustom {
					continue
				}
				// Try to extract from custom config
				username := extractUsernameFromCustomConfig(string(customConfig))
				if username != "" {
					return username, nil
				}
			}
			continue
		}

		// Parse transport_url to extract username
		username, err := parseUsernameFromTransportURL(string(transportURLBytes))
		if err != nil {
			h.GetLogger().Info("Failed to parse transport_url", "secret", secret.Name, "error", err)
			continue
		}

		if username != "" {
			return username, nil
		}
	}

	return "", fmt.Errorf("no RabbitMQ username found for cell %s", cellName)
}

// GetNovaCellNotificationRabbitMqUserFromSecret extracts the notification RabbitMQ username
// from a nova-cellX-compute-config secret. This is used for tracking the RabbitMQUser CR
// used for notifications (separate from the messaging/RPC bus).
// Returns the username extracted from notification_rabbitmq_user_name field (preferred)
// or attempts to extract from notification transport_url (fallback).
func GetNovaCellNotificationRabbitMqUserFromSecret(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	cellName string,
) (string, error) {
	// List all secrets in the namespace
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return "", fmt.Errorf("failed to list secrets: %w", err)
	}

	// Pattern to match nova-cellX-compute-config secrets
	secretPattern := regexp.MustCompile(`^nova-(` + cellName + `)-compute-config(-\d+)?$`)

	for _, secret := range secretList.Items {
		matches := secretPattern.FindStringSubmatch(secret.Name)
		if matches == nil {
			continue
		}

		// Preferred: Use the notification_rabbitmq_user_name field directly if available
		// This field is populated by nova-operator PR #1066 with the RabbitMQUser CR name
		if notificationRabbitmqUserName, ok := secret.Data["notification_rabbitmq_user_name"]; ok && len(notificationRabbitmqUserName) > 0 {
			return string(notificationRabbitmqUserName), nil
		}

		// If notification_rabbitmq_user_name is not available, this likely means:
		// 1. Nova-operator hasn't been updated to PR #1066 yet, or
		// 2. Notifications are not configured for this cell
		// Return empty string to indicate no notification user (not an error)
		return "", nil
	}

	return "", fmt.Errorf("no compute-config secret found for cell %s", cellName)
}

// parseUsernameFromTransportURL extracts the username from a RabbitMQ transport URL
// Format: rabbit://username:password@host:port/vhost
// Also supports: rabbit+tls://username:password@host1:port1,host2:port2/vhost
func parseUsernameFromTransportURL(transportURL string) (string, error) {
	// Handle empty URLs
	if transportURL == "" {
		return "", fmt.Errorf("empty transport URL")
	}

	// Parse the URL
	// First, replace rabbit:// or rabbit+tls:// with http:// for URL parsing
	tempURL := strings.Replace(transportURL, "rabbit://", "http://", 1)
	tempURL = strings.Replace(tempURL, "rabbit+tls://", "http://", 1)

	parsedURL, err := url.Parse(tempURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Extract username from UserInfo
	if parsedURL.User == nil {
		return "", fmt.Errorf("no user info in transport URL")
	}

	username := parsedURL.User.Username()
	if username == "" {
		return "", fmt.Errorf("empty username in transport URL")
	}

	return username, nil
}

// extractUsernameFromCustomConfig attempts to extract RabbitMQ username from custom config
// This is a fallback for cases where transport_url is embedded in the config file
func extractUsernameFromCustomConfig(customConfig string) string {
	// Look for transport_url in the config
	// Format: transport_url = rabbit://username:password@...
	transportURLPattern := regexp.MustCompile(`transport_url\s*=\s*rabbit[^:]*://([^:]+):`)
	matches := transportURLPattern.FindStringSubmatch(customConfig)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// extractTransportURLFromConfig attempts to extract the full transport_url from a config file
// This is used to get the RabbitMQ cluster information when transport_url is embedded in the config
func extractTransportURLFromConfig(customConfig string) string {
	// Look for transport_url in the config
	// Format: transport_url=rabbit://username:password@host:port/vhost?options
	// or: transport_url = rabbit://username:password@host:port/vhost?options
	transportURLPattern := regexp.MustCompile(`transport_url\s*=\s*(rabbit[^\s\n]+)`)
	matches := transportURLPattern.FindStringSubmatch(customConfig)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// GetNovaComputeConfigCellNames returns a list of cell names from nova-cellX-compute-config secrets
// referenced in the NodeSet's dataSources
func GetNovaComputeConfigCellNames(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) ([]string, error) {
	// List all secrets in the namespace
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	cellNames := []string{}
	// Pattern to match nova-cellX-compute-config secrets
	secretPattern := regexp.MustCompile(`^nova-(cell\d+)-compute-config(-\d+)?$`)

	for _, secret := range secretList.Items {
		matches := secretPattern.FindStringSubmatch(secret.Name)
		if matches == nil {
			continue
		}

		cellName := matches[1] // Extract cell name (e.g., "cell1")
		// Avoid duplicates
		found := false
		for _, cn := range cellNames {
			if cn == cellName {
				found = true
				break
			}
		}
		if !found {
			cellNames = append(cellNames, cellName)
		}
	}

	return cellNames, nil
}

// ExtractCellNameFromSecretName extracts the cell name from a secret name
// Example: "nova-cell1-compute-config" -> "cell1"
// Example: "nova-cell1-compute-config-0" -> "cell1"
func ExtractCellNameFromSecretName(secretName string) string {
	pattern := regexp.MustCompile(`nova-(cell\d+)-compute-config`)
	matches := pattern.FindStringSubmatch(secretName)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// ComputeNovaCellSecretsHash calculates a hash of all nova-cellX-compute-config secrets
// This is used to detect when the secrets change and reset node update tracking
func ComputeNovaCellSecretsHash(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (string, error) {
	secretsLastModified, err := GetNovaCellSecretsLastModified(ctx, h, namespace)
	if err != nil {
		return "", err
	}

	if len(secretsLastModified) == 0 {
		return "", nil
	}

	// Build a stable string representation of all secrets and their modification times
	var secretNames []string
	for name := range secretsLastModified {
		secretNames = append(secretNames, name)
	}
	// Sort for stable hash
	slices.Sort(secretNames)

	hashData := ""
	for _, name := range secretNames {
		modTime := secretsLastModified[name]
		hashData += fmt.Sprintf("%s:%d;", name, modTime.Unix())
	}

	// Use a simple hash
	return fmt.Sprintf("%x", hashData), nil
}

// GetNovaCellSecretsLastModified returns a map of nova-cellX-compute-config secret names
// to their last modification timestamps
func GetNovaCellSecretsLastModified(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (map[string]time.Time, error) {
	// List all secrets in the namespace
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	secretTimes := make(map[string]time.Time)
	// Pattern to match nova-cellX-compute-config secrets
	secretPattern := regexp.MustCompile(`^nova-(cell\d+)-compute-config(-\d+)?$`)

	for _, secret := range secretList.Items {
		matches := secretPattern.FindStringSubmatch(secret.Name)
		if matches == nil {
			continue
		}

		// Use the resource version change time if available, otherwise creation time
		modTime := secret.CreationTimestamp.Time
		if secret.ManagedFields != nil {
			for _, field := range secret.ManagedFields {
				if field.Time != nil && field.Time.After(modTime) {
					modTime = field.Time.Time
				}
			}
		}

		secretTimes[secret.Name] = modTime
	}

	return secretTimes, nil
}

// GetRabbitMQClusterForCell returns the RabbitMQ cluster name used by a specific nova cell
// by extracting it from the transport_url in the nova-cellX-compute-config secret
func GetRabbitMQClusterForCell(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	cellName string,
) (string, error) {
	// List all secrets in the namespace
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return "", fmt.Errorf("failed to list secrets: %w", err)
	}

	// Pattern to match nova-cellX-compute-config secrets
	secretPattern := regexp.MustCompile(`^nova-(` + cellName + `)-compute-config(-\d+)?$`)

	for _, secret := range secretList.Items {
		matches := secretPattern.FindStringSubmatch(secret.Name)
		if matches == nil {
			continue
		}

		// Extract transport_url from config files (01-nova.conf or custom.conf)
		// The transport_url is embedded in the config, not as a separate field
		var transportURL string
		for _, configKey := range []string{"custom.conf", "01-nova.conf"} {
			configData, hasConfig := secret.Data[configKey]
			if !hasConfig {
				continue
			}
			// Try to extract transport_url from config
			transportURL = extractTransportURLFromConfig(string(configData))
			if transportURL != "" {
				break
			}
		}

		if transportURL == "" {
			continue
		}

		// Parse transport_url to extract hostname (which typically includes cluster info)
		// Format: rabbit://username:password@host:port/vhost
		// The host part often contains the cluster name
		cluster, err := extractClusterFromTransportURL(transportURL)
		if err == nil && cluster != "" {
			return cluster, nil
		}
	}

	return "", fmt.Errorf("no RabbitMQ cluster found for cell %s", cellName)
}

// extractClusterFromTransportURL extracts the cluster identifier from a RabbitMQ transport URL
// This is a heuristic approach - the cluster name is often part of the hostname
func extractClusterFromTransportURL(transportURL string) (string, error) {
	if transportURL == "" {
		return "", fmt.Errorf("empty transport URL")
	}

	// Replace rabbit:// or rabbit+tls:// with http:// for URL parsing
	tempURL := strings.Replace(transportURL, "rabbit://", "http://", 1)
	tempURL = strings.Replace(tempURL, "rabbit+tls://", "http://", 1)

	parsedURL, err := url.Parse(tempURL)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %w", err)
	}

	// Extract the hostname (may contain multiple hosts separated by commas)
	host := parsedURL.Host
	if host == "" {
		return "", fmt.Errorf("no host in transport URL")
	}

	// If multiple hosts, take the first one
	hosts := strings.Split(host, ",")
	if len(hosts) > 0 {
		// Parse the first host to get just the hostname (strip port)
		firstHost := strings.Split(hosts[0], ":")[0]

		// Extract cluster name - typically the first part of the hostname
		// e.g., "rabbitmq-cell1.openstack.svc" -> "rabbitmq-cell1"
		// or "rabbitmq.openstack.svc" -> "rabbitmq"
		parts := strings.Split(firstHost, ".")
		if len(parts) > 0 {
			return parts[0], nil
		}
		return firstHost, nil
	}

	return "", fmt.Errorf("could not extract cluster from transport URL")
}

// ServiceSecretConfig defines the secret and config file patterns for a service
type ServiceSecretConfig struct {
	SecretNames []string
	ConfigKeys  []string
}

// GetRabbitMqUserFromServiceSecrets extracts the RabbitMQ username from service config secrets
// This is a generic function that works with any service that uses RabbitMQ
func GetRabbitMqUserFromServiceSecrets(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	config ServiceSecretConfig,
	serviceName string,
) (string, error) {
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return "", fmt.Errorf("failed to list secrets: %w", err)
	}

	for _, secret := range secretList.Items {
		for _, pattern := range config.SecretNames {
			if secret.Name == pattern {
				for _, configKey := range config.ConfigKeys {
					configData, hasConfig := secret.Data[configKey]
					if !hasConfig {
						continue
					}
					username := extractUsernameFromCustomConfig(string(configData))
					if username != "" {
						return username, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no RabbitMQ username found in %s secrets", serviceName)
}

// GetNeutronRabbitMqUserFromSecret extracts the RabbitMQ username from Neutron agent config secrets
func GetNeutronRabbitMqUserFromSecret(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (string, error) {
	config := ServiceSecretConfig{
		SecretNames: []string{
			"neutron-dhcp-agent-neutron-config",
			"neutron-sriov-agent-neutron-config",
		},
		ConfigKeys: []string{"10-neutron-dhcp.conf", "10-neutron-sriov.conf"},
	}
	return GetRabbitMqUserFromServiceSecrets(ctx, h, namespace, config, "Neutron")
}

// GetServiceSecretsLastModified returns a map of service secret names to their last modification timestamps
func GetServiceSecretsLastModified(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	secretNames []string,
) (map[string]time.Time, error) {
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	secretTimes := make(map[string]time.Time)
	for _, secret := range secretList.Items {
		for _, pattern := range secretNames {
			if secret.Name == pattern {
				modTime := secret.CreationTimestamp.Time
				if secret.ManagedFields != nil {
					for _, field := range secret.ManagedFields {
						if field.Time != nil && field.Time.After(modTime) {
							modTime = field.Time.Time
						}
					}
				}
				secretTimes[secret.Name] = modTime
			}
		}
	}

	return secretTimes, nil
}

// GetNeutronSecretsLastModified returns a map of Neutron agent config secret names to their last modification timestamps
func GetNeutronSecretsLastModified(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (map[string]time.Time, error) {
	secretNames := []string{
		"neutron-dhcp-agent-neutron-config",
		"neutron-sriov-agent-neutron-config",
	}
	return GetServiceSecretsLastModified(ctx, h, namespace, secretNames)
}

// GetRabbitMQClusterForService returns the RabbitMQ cluster name by extracting from transport_url
func GetRabbitMQClusterForService(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	config ServiceSecretConfig,
	serviceName string,
) (string, error) {
	secretList := &corev1.SecretList{}
	err := h.GetClient().List(ctx, secretList, client.InNamespace(namespace))
	if err != nil {
		return "", fmt.Errorf("failed to list secrets: %w", err)
	}

	for _, secret := range secretList.Items {
		for _, pattern := range config.SecretNames {
			if secret.Name == pattern {
				for _, configKey := range config.ConfigKeys {
					configData, hasConfig := secret.Data[configKey]
					if !hasConfig {
						continue
					}
					transportURL := extractTransportURLFromConfig(string(configData))
					if transportURL == "" {
						continue
					}
					cluster, err := extractClusterFromTransportURL(transportURL)
					if err == nil && cluster != "" {
						return cluster, nil
					}
				}
			}
		}
	}

	return "", fmt.Errorf("no RabbitMQ cluster found in %s secrets", serviceName)
}

// GetRabbitMQClusterForNeutron returns the RabbitMQ cluster name used by Neutron agents
func GetRabbitMQClusterForNeutron(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (string, error) {
	config := ServiceSecretConfig{
		SecretNames: []string{
			"neutron-dhcp-agent-neutron-config",
			"neutron-sriov-agent-neutron-config",
		},
		ConfigKeys: []string{"10-neutron-dhcp.conf", "10-neutron-sriov.conf"},
	}
	return GetRabbitMQClusterForService(ctx, h, namespace, config, "Neutron")
}

// ComputeServiceSecretsHash calculates a hash of service secrets to detect changes
func ComputeServiceSecretsHash(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	secretNames []string,
) (string, error) {
	secretsLastModified, err := GetServiceSecretsLastModified(ctx, h, namespace, secretNames)
	if err != nil {
		return "", err
	}

	if len(secretsLastModified) == 0 {
		return "", nil
	}

	var names []string
	for name := range secretsLastModified {
		names = append(names, name)
	}
	slices.Sort(names)

	hashData := ""
	for _, name := range names {
		modTime := secretsLastModified[name]
		hashData += fmt.Sprintf("%s:%d;", name, modTime.Unix())
	}

	return fmt.Sprintf("%x", hashData), nil
}

// ComputeNeutronSecretsHash calculates a hash of Neutron agent config secrets
func ComputeNeutronSecretsHash(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (string, error) {
	secretNames := []string{
		"neutron-dhcp-agent-neutron-config",
		"neutron-sriov-agent-neutron-config",
	}
	return ComputeServiceSecretsHash(ctx, h, namespace, secretNames)
}

// GetIronicRabbitMqUserFromSecret extracts the RabbitMQ username from Ironic Neutron Agent config secrets
func GetIronicRabbitMqUserFromSecret(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (string, error) {
	config := ServiceSecretConfig{
		SecretNames: []string{"ironic-neutron-agent-config-data"},
		ConfigKeys:  []string{"01-ironic_neutron_agent.conf"},
	}
	return GetRabbitMqUserFromServiceSecrets(ctx, h, namespace, config, "Ironic Neutron Agent")
}

// GetIronicSecretsLastModified returns a map of Ironic Neutron Agent config secret names to their last modification timestamps
func GetIronicSecretsLastModified(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (map[string]time.Time, error) {
	secretNames := []string{"ironic-neutron-agent-config-data"}
	return GetServiceSecretsLastModified(ctx, h, namespace, secretNames)
}

// GetRabbitMQClusterForIronic returns the RabbitMQ cluster name used by Ironic Neutron Agent
func GetRabbitMQClusterForIronic(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (string, error) {
	config := ServiceSecretConfig{
		SecretNames: []string{"ironic-neutron-agent-config-data"},
		ConfigKeys:  []string{"01-ironic_neutron_agent.conf"},
	}
	return GetRabbitMQClusterForService(ctx, h, namespace, config, "Ironic Neutron Agent")
}

// ComputeIronicSecretsHash calculates a hash of Ironic Neutron Agent config secrets
func ComputeIronicSecretsHash(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (string, error) {
	secretNames := []string{"ironic-neutron-agent-config-data"}
	return ComputeServiceSecretsHash(ctx, h, namespace, secretNames)
}

// GetRabbitMQSecretsLastModified returns a combined map of all RabbitMQ-related secret names
// (Nova, Neutron, and Ironic) to their last modification timestamps
func GetRabbitMQSecretsLastModified(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
) (map[string]time.Time, error) {
	allSecrets := make(map[string]time.Time)

	// Get and merge Nova cell secrets
	novaSecretsLastModified, err := GetNovaCellSecretsLastModified(ctx, h, namespace)
	if err != nil {
		return nil, err
	}
	maps.Copy(allSecrets, novaSecretsLastModified)

	// Get and merge Neutron agent secrets
	neutronSecretsLastModified, err := GetNeutronSecretsLastModified(ctx, h, namespace)
	if err != nil {
		return nil, err
	}
	maps.Copy(allSecrets, neutronSecretsLastModified)

	// Get and merge Ironic Neutron Agent secrets
	ironicSecretsLastModified, err := GetIronicSecretsLastModified(ctx, h, namespace)
	if err != nil {
		return nil, err
	}
	maps.Copy(allSecrets, ironicSecretsLastModified)

	return allSecrets, nil
}
