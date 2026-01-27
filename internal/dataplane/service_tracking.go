/*
Copyright 2024.

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
	"encoding/json"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// ServiceTrackingConfigMapSuffix is appended to nodeset name to create tracking ConfigMap
	ServiceTrackingConfigMapSuffix = "-service-tracking"
)

// ServiceTrackingData stores tracking information for a service's credential rotation
type ServiceTrackingData struct {
	// SecretHash is the hash of the service's secrets to detect changes
	SecretHash string `json:"secretHash"`
	// UpdatedNodes is the list of nodes that have been updated after the secret change
	UpdatedNodes []string `json:"updatedNodes"`
}

// GetServiceTrackingConfigMapName returns the name of the tracking ConfigMap for a nodeset
func GetServiceTrackingConfigMapName(nodesetName string) string {
	return nodesetName + ServiceTrackingConfigMapSuffix
}

// EnsureServiceTrackingConfigMap ensures the tracking ConfigMap exists for a nodeset
func EnsureServiceTrackingConfigMap(
	ctx context.Context,
	h *helper.Helper,
	nodesetName string,
	namespace string,
	ownerRefs []metav1.OwnerReference,
) (*corev1.ConfigMap, error) {
	configMapName := GetServiceTrackingConfigMapName(nodesetName)

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:            configMapName,
			Namespace:       namespace,
			OwnerReferences: ownerRefs,
		},
		Data: make(map[string]string),
	}

	// Try to get existing ConfigMap
	existing := &corev1.ConfigMap{}
	err := h.GetClient().Get(ctx, client.ObjectKey{Name: configMapName, Namespace: namespace}, existing)
	if err == nil {
		// ConfigMap exists, return it
		return existing, nil
	}

	if !k8s_errors.IsNotFound(err) {
		return nil, fmt.Errorf("failed to get service tracking ConfigMap: %w", err)
	}

	// ConfigMap doesn't exist, create it
	err = h.GetClient().Create(ctx, configMap)
	if err != nil {
		return nil, fmt.Errorf("failed to create service tracking ConfigMap: %w", err)
	}

	return configMap, nil
}

// GetServiceTracking retrieves tracking data for a specific service from the ConfigMap
func GetServiceTracking(
	ctx context.Context,
	h *helper.Helper,
	nodesetName string,
	namespace string,
	serviceName string,
) (*ServiceTrackingData, error) {
	configMapName := GetServiceTrackingConfigMapName(nodesetName)

	configMap := &corev1.ConfigMap{}
	err := h.GetClient().Get(ctx, client.ObjectKey{Name: configMapName, Namespace: namespace}, configMap)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			// ConfigMap doesn't exist yet, return empty tracking data
			return &ServiceTrackingData{
				SecretHash:   "",
				UpdatedNodes: []string{},
			}, nil
		}
		return nil, fmt.Errorf("failed to get service tracking ConfigMap: %w", err)
	}

	// Get the data for this service
	secretHashKey := fmt.Sprintf("%s.secretHash", serviceName)
	updatedNodesKey := fmt.Sprintf("%s.updatedNodes", serviceName)

	tracking := &ServiceTrackingData{
		SecretHash:   configMap.Data[secretHashKey],
		UpdatedNodes: []string{},
	}

	// Parse the updated nodes JSON array
	if nodesJSON, ok := configMap.Data[updatedNodesKey]; ok && nodesJSON != "" {
		err := json.Unmarshal([]byte(nodesJSON), &tracking.UpdatedNodes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse updated nodes JSON: %w", err)
		}
	}

	return tracking, nil
}

// UpdateServiceTracking updates tracking data for a specific service in the ConfigMap
func UpdateServiceTracking(
	ctx context.Context,
	h *helper.Helper,
	nodesetName string,
	namespace string,
	serviceName string,
	tracking *ServiceTrackingData,
	ownerRefs []metav1.OwnerReference,
) error {
	configMapName := GetServiceTrackingConfigMapName(nodesetName)

	// Marshal updated nodes to JSON
	nodesJSON, err := json.Marshal(tracking.UpdatedNodes)
	if err != nil {
		return fmt.Errorf("failed to marshal updated nodes: %w", err)
	}

	secretHashKey := fmt.Sprintf("%s.secretHash", serviceName)
	updatedNodesKey := fmt.Sprintf("%s.updatedNodes", serviceName)

	// Use CreateOrUpdate to handle both creation and update
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
	}

	_, err = controllerutil.CreateOrUpdate(ctx, h.GetClient(), configMap, func() error {
		if configMap.Data == nil {
			configMap.Data = make(map[string]string)
		}
		configMap.Data[secretHashKey] = tracking.SecretHash
		configMap.Data[updatedNodesKey] = string(nodesJSON)

		// Set owner references if provided and not already set
		if len(ownerRefs) > 0 && len(configMap.OwnerReferences) == 0 {
			configMap.OwnerReferences = ownerRefs
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to update service tracking ConfigMap: %w", err)
	}

	return nil
}

// ResetServiceNodeTracking resets the updated nodes list for a service (called when secret hash changes)
func ResetServiceNodeTracking(
	ctx context.Context,
	h *helper.Helper,
	nodesetName string,
	namespace string,
	serviceName string,
	newSecretHash string,
	ownerRefs []metav1.OwnerReference,
) error {
	tracking := &ServiceTrackingData{
		SecretHash:   newSecretHash,
		UpdatedNodes: []string{},
	}

	return UpdateServiceTracking(ctx, h, nodesetName, namespace, serviceName, tracking, ownerRefs)
}

// AddUpdatedNode adds a node to the updated nodes list for a service
func AddUpdatedNode(
	ctx context.Context,
	h *helper.Helper,
	nodesetName string,
	namespace string,
	serviceName string,
	nodeName string,
	ownerRefs []metav1.OwnerReference,
) error {
	tracking, err := GetServiceTracking(ctx, h, nodesetName, namespace, serviceName)
	if err != nil {
		return err
	}

	// Check if node is already in the list
	for _, existingNode := range tracking.UpdatedNodes {
		if existingNode == nodeName {
			// Node already tracked
			return nil
		}
	}

	// Add the node
	tracking.UpdatedNodes = append(tracking.UpdatedNodes, nodeName)

	return UpdateServiceTracking(ctx, h, nodesetName, namespace, serviceName, tracking, ownerRefs)
}

// AddUpdatedNodes adds multiple nodes to the updated nodes list for a service
func AddUpdatedNodes(
	ctx context.Context,
	h *helper.Helper,
	nodesetName string,
	namespace string,
	serviceName string,
	nodeNames []string,
	ownerRefs []metav1.OwnerReference,
) error {
	tracking, err := GetServiceTracking(ctx, h, nodesetName, namespace, serviceName)
	if err != nil {
		return err
	}

	// Build a map of existing nodes for fast lookup
	existingNodes := make(map[string]bool)
	for _, node := range tracking.UpdatedNodes {
		existingNodes[node] = true
	}

	// Add new nodes
	for _, nodeName := range nodeNames {
		if !existingNodes[nodeName] {
			tracking.UpdatedNodes = append(tracking.UpdatedNodes, nodeName)
			existingNodes[nodeName] = true
		}
	}

	return UpdateServiceTracking(ctx, h, nodesetName, namespace, serviceName, tracking, ownerRefs)
}
