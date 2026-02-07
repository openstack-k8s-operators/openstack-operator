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

package dataplane

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"time"

	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	deployment "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// finalizerPrefix is the domain prefix for our finalizers
	finalizerPrefix = "nodeset.os/"
)

// computeFinalizerHash computes a deterministic 8-character hash from a nodeset name.
// Uses SHA256 and returns the first 8 hex characters.
// This hash is stored in the nodeset status and used to build unique finalizer names.
func computeFinalizerHash(nodesetName string) string {
	hash := sha256.Sum256([]byte(nodesetName))
	return hex.EncodeToString(hash[:])[:8]
}

// buildFinalizerName creates a unique, deterministic finalizer name using a hash.
//
// Format: nodeset.os/{8-char-hash}-{service}
//
// The hash is derived from SHA256(nodeset.metadata.name) and stored in the nodeset status
// for easy lookup of which nodeset owns a specific finalizer.
//
// Examples:
//   - nodeset.os/a3f2b5c8-nova (28 chars)
//   - nodeset.os/7e9d1234-neutron (30 chars)
//   - nodeset.os/5a6b7c8d-ironic (29 chars)
//
// Benefits:
// 1. Guaranteed collision-free (SHA256 hash uniqueness)
// 2. Always fits within 63-char Kubernetes limit (max ~30 chars)
// 3. Deterministic (same nodeset → same hash → same finalizer)
// 4. Easy debugging via nodeset status (finalizerHash field)
func buildFinalizerName(finalizerHash, serviceName string) string {
	return fmt.Sprintf("%s%s-%s", finalizerPrefix, finalizerHash, serviceName)
}

// manageServiceFinalizers manages finalizers on RabbitMqUser CRs for a specific service
// This function:
// 1. ALWAYS adds finalizers to users currently in use (even during partial deployments)
// 2. Checks if all nodes for this service have been updated
// 3. Checks if all nodesets using the same RabbitMQ cluster for this service are updated
// 4. Only removes finalizers from old users when ALL nodes and ALL nodesets are updated
//
// This ensures credentials are protected as soon as they're in use, preventing accidental
// deletion during rolling updates or partial deployments.
func (r *OpenStackDataPlaneNodeSetReconciler) manageServiceFinalizers(
	ctx context.Context,
	helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	serviceName string,
	serviceDisplayName string,
	secretsLastModified map[string]time.Time,
) {
	Log := r.GetLogger(ctx)

	// Get the service tracking data
	tracking, err := deployment.GetServiceTracking(ctx, helper, instance.Name, instance.Namespace, serviceName)
	if err != nil {
		Log.Error(err, "Failed to get service tracking", "service", serviceName)
		return
	}

	// Safety check: Ensure we have a secret hash (meaning we're tracking secret changes)
	if tracking.SecretHash == "" {
		Log.Info("No secret hash set yet for service, skipping finalizer management",
			"service", serviceName)
		return
	}

	// Check if at least some nodes have been updated (we need at least one to add finalizers)
	allNodeNames := r.getAllNodeNamesFromNodeset(instance)
	if len(tracking.UpdatedNodes) == 0 {
		Log.Info("No nodes updated yet for service, skipping finalizer management",
			"service", serviceName)
		return
	}

	// Check if all nodes in this nodeset are updated
	allNodesInNodesetUpdated := len(tracking.UpdatedNodes) == len(allNodeNames)

	// Check if all nodesets using the same RabbitMQ cluster have been updated
	// This includes tracking node coverage across multiple AnsibleLimit deployments
	allNodesetsUpdated, err := r.allNodesetsUsingClusterUpdated(ctx, helper, instance, serviceName, secretsLastModified)
	if err != nil {
		Log.Error(err, "Failed to check if all nodesets are updated", "service", serviceName)
		return
	}

	// Determine if we should remove old finalizers
	// Only safe to remove when ALL nodes across ALL nodesets are updated
	shouldRemoveOldFinalizers := allNodesInNodesetUpdated && allNodesetsUpdated

	if shouldRemoveOldFinalizers {
		Log.Info("Service deployed successfully and all nodes updated, managing RabbitMQ user finalizers",
			"service", serviceDisplayName,
			"updatedNodes", len(tracking.UpdatedNodes),
			"totalNodes", len(allNodeNames))
	} else {
		Log.Info("Adding finalizers to current RabbitMQ users (partial deployment in progress)",
			"service", serviceDisplayName,
			"updatedNodes", len(tracking.UpdatedNodes),
			"totalNodes", len(allNodeNames),
			"allNodesInNodesetUpdated", allNodesInNodesetUpdated,
			"allNodesetsUpdated", allNodesetsUpdated)
	}

	// Get the finalizer hash from nodeset status
	// This hash is a deterministic SHA256-based identifier used to create unique finalizer names
	// The hash is computed and stored during reconciliation
	finalizerHash := instance.Status.FinalizerHash
	if finalizerHash == "" {
		Log.Error(fmt.Errorf("finalizerHash not set in nodeset status"), "Cannot manage finalizers without hash",
			"nodeset", instance.Name)
		return
	}

	// Build the finalizer name: nodeset.os/{hash}-{service}
	// This format is guaranteed unique, collision-free, and always fits in 63 chars
	finalizerName := buildFinalizerName(finalizerHash, serviceName)
	Log.Info("Using finalizer", "name", finalizerName, "hash", finalizerHash, "length", len(finalizerName))

	// Track current usernames that should have our finalizer
	currentUsernames := make(map[string]bool)

	// Get usernames based on service type
	if serviceName == "nova" {
		// Get Nova cell usernames
		cellNames, err := deployment.GetNovaComputeConfigCellNames(ctx, helper, instance.Namespace)
		if err != nil {
			Log.Error(err, "Failed to get Nova cell names")
			return
		}

		for _, cellName := range cellNames {
			username, err := deployment.GetNovaCellRabbitMqUserFromSecret(ctx, helper, instance.Namespace, cellName)
			if err != nil {
				Log.Info("Failed to get RabbitMQ username for Nova cell", "cell", cellName, "error", err)
				continue
			}

			if username != "" {
				currentUsernames[username] = true
				Log.Info("Found current RabbitMQ username for Nova cell", "cell", cellName, "username", username)
			}
		}
	} else if serviceName == "neutron" {
		// Get Neutron username (shared across DHCP and SRIOV agents)
		neutronUsername, err := deployment.GetNeutronRabbitMqUserFromSecret(ctx, helper, instance.Namespace)
		if err != nil {
			Log.Error(err, "Failed to get RabbitMQ username for Neutron")
			return
		}

		if neutronUsername != "" {
			currentUsernames[neutronUsername] = true
			Log.Info("Found current RabbitMQ username for Neutron", "username", neutronUsername)
		}
	} else if serviceName == "ironic" {
		// Get Ironic Neutron Agent username
		ironicUsername, err := deployment.GetIronicRabbitMqUserFromSecret(ctx, helper, instance.Namespace)
		if err != nil {
			Log.Error(err, "Failed to get RabbitMQ username for Ironic Neutron Agent")
			return
		}

		if ironicUsername != "" {
			currentUsernames[ironicUsername] = true
			Log.Info("Found current RabbitMQ username for Ironic Neutron Agent", "username", ironicUsername)
		}
	}

	// If we found no RabbitMQ users for this service, skip finalizer management
	if len(currentUsernames) == 0 {
		Log.Info("No RabbitMQ users found in secrets for service, skipping finalizer management",
			"service", serviceName)
		return
	}

	// List all RabbitMqUsers in the namespace
	rabbitmqUserList := &rabbitmqv1.RabbitMQUserList{}
	err = r.List(ctx, rabbitmqUserList, client.InNamespace(instance.Namespace))
	if err != nil {
		Log.Error(err, "Failed to list RabbitMQUsers", "service", serviceName)
		return
	}

	// Process each RabbitMqUser
	for i := range rabbitmqUserList.Items {
		rabbitmqUser := &rabbitmqUserList.Items[i]

		// Check if this user is currently in use by this nodeset for this service
		// Match by either CR name or status username
		isCurrentlyInUse := currentUsernames[rabbitmqUser.Name] || currentUsernames[rabbitmqUser.Status.Username]

		hasFinalizer := slices.Contains(rabbitmqUser.Finalizers, finalizerName)

		if isCurrentlyInUse && !hasFinalizer {
			// Add finalizer to this RabbitMqUser (this is the current user)
			// We add immediately when ANY node starts using this user to protect it
			Log.Info("Adding finalizer to RabbitMqUser",
				"service", serviceName,
				"user", rabbitmqUser.Name,
				"finalizer", finalizerName)
			rabbitmqUser.Finalizers = append(rabbitmqUser.Finalizers, finalizerName)
			err = r.Update(ctx, rabbitmqUser)
			if err != nil {
				Log.Error(err, "Failed to add finalizer to RabbitMQUser",
					"service", serviceName,
					"user", rabbitmqUser.Name)
				// Don't fail reconciliation, just log the error
			}
		} else if !isCurrentlyInUse && hasFinalizer && shouldRemoveOldFinalizers {
			// Remove finalizer from this RabbitMqUser (no longer in use)
			// Safe to remove because we only reach here when:
			// 1. The deployment was created AFTER the secret was modified
			// 2. No deployments are currently running
			// 3. This user is not in the current secret configuration for this service
			// 4. All nodes for this service have been updated
			// 5. All nodesets using the same cluster for this service have been updated
			Log.Info("Removing finalizer from RabbitMqUser (no longer in use)",
				"service", serviceName,
				"user", rabbitmqUser.Name,
				"finalizer", finalizerName)
			var newFinalizers []string
			for _, f := range rabbitmqUser.Finalizers {
				if f != finalizerName {
					newFinalizers = append(newFinalizers, f)
				}
			}
			rabbitmqUser.Finalizers = newFinalizers
			err = r.Update(ctx, rabbitmqUser)
			if err != nil {
				Log.Error(err, "Failed to remove finalizer from RabbitMQUser",
					"service", serviceName,
					"user", rabbitmqUser.Name)
				// Don't fail reconciliation, just log the error
			}
		} else if !isCurrentlyInUse && hasFinalizer && !shouldRemoveOldFinalizers {
			// This user is no longer in use but we can't remove the finalizer yet
			// because not all nodes/nodesets have been updated
			Log.Info("RabbitMqUser has finalizer but is no longer in use - waiting for all nodes to update before removing",
				"service", serviceName,
				"user", rabbitmqUser.Name,
				"finalizer", finalizerName,
				"updatedNodes", len(tracking.UpdatedNodes),
				"totalNodes", len(allNodeNames))
		}
	}
}
