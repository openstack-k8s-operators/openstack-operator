# DataPlane RabbitMQ User Finalizer Management

## Overview

The OpenStack DataPlane operator manages RabbitMQ user finalizers to prevent deletion of RabbitMQ users that are still in use by compute nodes. This is critical during rolling upgrades and credential rotation scenarios.

## Problem Statement

During RabbitMQ credential rotation:
1. Nova creates a new RabbitMQ user (e.g., `user2`) with new credentials
2. The nova-cell1-compute-config secret is updated with the new `transport_url`
3. Compute nodes are updated incrementally (using `ansibleLimit` for rolling upgrades)
4. The old RabbitMQ user (e.g., `user1`) must remain until **ALL** nodes are updated
5. Only after complete migration should the old user's finalizer be removed

**Challenge**: With ansibleLimit deployments, nodes are updated in batches. We must track which nodes have been updated across multiple deployments.

## How It Works

### 1. Secret Change Detection

The controller monitors nova-cell compute-config secrets for changes:

```go
// When a secret's transport_url changes, the NovaCellSecretHash is updated
// This triggers UpdatedNodesAfterSecretChange to be reset to empty
if newHash != instance.Status.NovaCellSecretHash {
    instance.Status.NovaCellSecretHash = newHash
    instance.Status.UpdatedNodesAfterSecretChange = []string{}
}
```

**Location**: `openstackdataplanenodeset_controller.go:523-545`

### 2. Node Coverage Tracking

As deployments complete, the controller tracks which nodes have been updated:

```go
func updateNodeCoverage(deployment, nodeset, secretsLastModified) {
    // Check deployment completion time (not creation time!)
    deploymentCompletedTime := readyCondition.LastTransitionTime.Time

    // Only track nodes from deployments that completed AFTER secret change
    if deploymentCompletedTime.Before(secretModTime) {
        return // Skip old deployments
    }

    // Add nodes from this deployment to UpdatedNodesAfterSecretChange
    for _, nodeName := range deployment.AnsibleLimit {
        instance.Status.UpdatedNodesAfterSecretChange = append(...)
    }
}
```

**Location**: `openstackdataplanenodeset_controller.go:750-793`

**Key Design Decision**: Uses deployment completion timestamp (`NodeSetDeploymentReadyCondition.LastTransitionTime`) rather than creation timestamp, because deployments can be created before they run and rotate secrets.

### 3. Finalizer Management Decision

Finalizers are only managed when ALL safety conditions are met:

```go
// Multi-layer protection system
if novaServiceDeployed &&           // Nova service was successfully deployed
   isLatestDeployment &&             // This is the most recent deployment
   !isNodeSetDeploymentRunning &&   // No deployment currently running
   !isDeploymentBeingDeleted &&     // Deployment not being deleted
   hasActiveDeployments {            // At least one active deployment exists

    // Safety check 1: Secret hash must be set
    if instance.Status.NovaCellSecretHash == "" {
        return // Skip - tracking not initialized
    }

    // Safety check 2: All nodes must be accounted for
    allNodeNames := getAllNodeNamesFromNodeset(instance)
    if len(instance.Status.UpdatedNodesAfterSecretChange) != len(allNodeNames) {
        return // Skip - not all nodes updated yet
    }

    // Safety check 3: All nodesets using same cluster must be updated
    if !allNodesetsUsingClusterUpdated(instance) {
        return // Skip - other nodesets still pending
    }

    // All checks passed - safe to manage finalizers
    manageRabbitMQUserFinalizers(...)
}
```

**Location**: `openstackdataplanenodeset_controller.go:669-719`

### 4. RabbitMQ Cluster Identification

The controller extracts RabbitMQ cluster information from transport_url:

```go
// Parse transport_url from nova config files (01-nova.conf or custom.conf)
// Format: rabbit://username:password@rabbitmq-cell1.openstack.svc:5672/?ssl=1
transportURL := extractTransportURLFromConfig(configData)
cluster := extractClusterFromTransportURL(transportURL) // Returns "rabbitmq-cell1"
```

**Location**: `rabbitmq.go:278-372`

**Important**: The transport_url is embedded in config files (01-nova.conf), not as a top-level secret field.

## Safety Mechanisms

### 1. Deployment Completion Timestamp Check
Prevents old deployments (created before secret change) from incorrectly populating the updated nodes list.

### 2. Complete Node Coverage
Ensures ALL nodes in the nodeset are accounted for before removing old user finalizers.

### 3. Secret Hash Initialization
Requires the tracking system to be properly initialized before managing finalizers.

### 4. Cross-NodeSet Validation
When multiple nodesets share a RabbitMQ cluster, ensures all are updated before finalizer removal.

### 5. Deployment Deletion Protection
Prevents finalizer changes when ansibleLimit deployments are being deleted, avoiding incorrect state from old deployments.

### 6. Active Deployment Requirement
Requires at least one active deployment to ensure reliable state tracking.

### 7. Accurate Node Counting
Counts only actual node names (map keys), excluding IP addresses and hostnames which are just aliases.

**Location**: `openstackdataplanenodeset_controller.go:930-945`

## Example Scenario: AnsibleLimit Rolling Upgrade

### Initial State
- NodeSet: `edpm-compute` with 2 nodes (compute-0, compute-1)
- RabbitMQ user: `user1`
- Secret: nova-cell1-compute-config (transport_url uses user1)

### Step 1: Secret Updated
```
Secret updated: transport_url changed to use user2
NovaCellSecretHash: updated from hash1 to hash2
UpdatedNodesAfterSecretChange: [] (reset to empty)
```

### Step 2: First AnsibleLimit Deployment
```
Deployment: edpm-deployment-compute-0
AnsibleLimit: "edpm-compute-0"
Status: Running → Completed
UpdatedNodesAfterSecretChange: ["edpm-compute-0"]

Finalizer Action: NONE (only 1 of 2 nodes updated)
```

### Step 3: Second AnsibleLimit Deployment
```
Deployment: edpm-deployment-compute-1
AnsibleLimit: "edpm-compute-1"
Status: Running → Completed
UpdatedNodesAfterSecretChange: ["edpm-compute-0", "edpm-compute-1"]

Finalizer Action: EXECUTE
- Add finalizer to user2 (new user)
- Remove finalizer from user1 (old user can now be deleted)
```

## Edge Cases Handled

### 1. Deployment Created Before Secret Change
If a deployment is created at T1, but runs and rotates the secret at T2, it correctly tracks as completing after the secret change.

### 2. Cluster Identification Failure
If transport_url cannot be parsed (missing or malformed), the controller logs a warning and skips finalizer management rather than risking incorrect cleanup.

**Location**: `openstackdataplanenodeset_controller.go:860-866`

### 3. AnsibleLimit Deployments Deleted
If ansibleLimit deployments are deleted after completing, the old deployment (created before secret change) does NOT trigger finalizer management, preventing incorrect state restoration.

### 4. Secret Rotation During Deployment
If the secret changes while a deployment is running, the hash change resets tracking and the deployment must re-run to populate the updated nodes list.

### 5. In-Memory vs Cluster State Race Condition (Fixed)
**Problem**: When a deployment completes, the reconciliation loop:
1. Calls `updateNodeCoverage()` to update `UpdatedNodesAfterSecretChange` in memory
2. Calls `allNodesetsUsingClusterUpdated()` which reads nodesets from the cluster
3. The cluster still has the old status (before the update in step 1)
4. Finalizer management is skipped due to stale data

**Solution**: The controller now uses the in-memory nodeset when checking the current nodeset, avoiding stale cluster reads.

**Location**: `openstackdataplanenodeset_controller.go:897-902`

## Code References

### Controller Logic
- Main reconciliation: `openstackdataplanenodeset_controller.go:380-750`
- Node coverage tracking: `openstackdataplanenodeset_controller.go:750-793`
- Finalizer management: `openstackdataplanenodeset_controller.go:669-719`
- Cross-nodeset validation: `openstackdataplanenodeset_controller.go:795-920`

### RabbitMQ Utilities
- Cluster identification: `rabbitmq.go:278-372`
- Username extraction: `rabbitmq.go:33-122`
- Cell name extraction: `rabbitmq.go:151-202`
- Secret hash computation: `rabbitmq.go:204-276`

## Testing

### Unit Test
- **Test**: "Should correctly count nodes without IP address aliases"
- **Validates**: Node counting excludes hostName and ansibleHost aliases
- **Location**: `test/functional/dataplane/openstackdataplanenodeset_rabbitmq_finalizer_test.go:141-149`

### Integration Testing
Complex deployment workflows require testing in a real cluster environment where multiple controllers (NodeSet, Deployment, Ansibleee) work together.

## Debugging

### Check Node Coverage Status
```bash
kubectl get openstackdataplanenodeset edpm-compute -n openstack -o jsonpath='{.status.updatedNodesAfterSecretChange}'
```

### Check Secret Hash
```bash
kubectl get openstackdataplanenodeset edpm-compute -n openstack -o jsonpath='{.status.novaCellSecretHash}'
```

### Check Deployment Completion Time
```bash
kubectl get openstackdataplanedeployment edpm-deployment -n openstack -o jsonpath='{.status.nodeSetConditions.edpm-compute[?(@.type=="NodeSetDeploymentReady")].lastTransitionTime}'
```

### Check RabbitMQ User Finalizers
```bash
kubectl get rabbitmquser nova-cell1-transport-user1 -n openstack -o jsonpath='{.metadata.finalizers}'
```

## Future Considerations

1. **Metrics**: Add Prometheus metrics for tracking finalizer management events
2. **Events**: Emit Kubernetes events when finalizers are added/removed for better visibility
3. **Status Conditions**: Add dedicated condition type for RabbitMQ user finalizer management
4. **Multi-Cell Support**: Current implementation handles multiple cells through separate nodesets
