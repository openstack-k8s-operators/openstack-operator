# Custom Route Certificate Implementation

This directory implements a hybrid approach that combines a script to create a ConfigMap in the cluster with kustomize replacements for certificate injection.

## How It Works

### Step 1: Script Creates ConfigMap in Cluster
The `tests/kuttl/common/prepare_placement_certs.sh` script:
1. Fetches certificates from the `placement-custom-route` secret
2. Applies a ConfigMap (`placement-cert-data`) directly to the cluster with `oc apply`
3. **No intermediate file is created** - the ConfigMap goes straight to the cluster

### Step 2: Kustomize Uses Replacements
The `kustomization.yaml`:
1. References the existing ConfigMap from the cluster (not from a file)
2. Patches the OpenStackControlPlane with empty certificate fields
3. Uses `replacements` to inject certificate data from the cluster ConfigMap

## Files

- **`kustomization.yaml`** - Kustomize configuration with replacements
- **`patch.yaml`** - Patch file with Barbican and Placement TLS configuration

**Helper Script:**
- **`../../../tests/kuttl/common/prepare_placement_certs.sh`** - Fetches certs from secret and creates ConfigMap in cluster

**Cluster Resources:**
- **`placement-cert-data`** (ConfigMap) - Created in the cluster by the script (no file)

## Usage

### Manual Testing

```bash
# Step 1: Create ConfigMap from secret
bash ../../../tests/kuttl/common/prepare_placement_certs.sh

# Step 2: Apply with kustomize
oc kustomize . | oc apply -n $NAMESPACE -f -
```

### In Kuttl Tests

The kuttl test splits the work across two steps for clarity:

**Step 02:** `02-deploy-custom-route-secret.yaml` - Create secrets and ConfigMap
```yaml
commands:
  - script: |
      source ../../common/create_custom_cert.sh
      INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
      create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"

  - script: |
      bash ../../common/prepare_placement_certs.sh
```

**Step 03:** `03-deploy-openstack.yaml` - Apply with kustomize
```yaml
commands:
  - script: |
      oc kustomize ../../../../config/samples/tls/custom_route_cert | oc apply -n $NAMESPACE -f -
```

## How Replacements Work

From `kustomization.yaml`:

```yaml
replacements:
- source:
    kind: ConfigMap
    name: placement-cert-data
    fieldPath: data.[tls.crt]
  targets:
  - select:
      kind: OpenStackControlPlane
    fieldPaths:
    - spec.placement.apiOverride.route.spec.tls.certificate
```

This replaces the empty `certificate: ""` field with the certificate data from the ConfigMap.

## Key Features

- ✅ Uses specific pre-existing secrets for testing
- ✅ More kustomize-native than pure bash approaches
- ✅ Doesn't modify source files during execution
- ✅ ConfigMap can be inspected/debugged in the cluster
- ✅ Clean separation: script generates data, kustomize applies

## Debugging

### View ConfigMap in Cluster
```bash
oc get configmap placement-cert-data -n $NAMESPACE -o yaml
```

### Test Kustomize Output
```bash
# See the full output
oc kustomize . | less

# Check just the placement TLS config
oc kustomize . | yq '.spec.placement.apiOverride.route.spec.tls'
```

### Verify Certificate Data
```bash
# Extract certificate from kustomize output
oc kustomize . | yq '.spec.placement.apiOverride.route.spec.tls.certificate'
```

## Troubleshooting

### Error: ConfigMap not found during kustomize
**Solution:** Run `bash ../../../tests/kuttl/common/prepare_placement_certs.sh` first to create the ConfigMap in the cluster

### Error: "Failed to fetch certificate data"
**Solution:** Ensure `placement-custom-route` secret exists in the namespace
```bash
oc get secret placement-custom-route -n $NAMESPACE
```

### Certificates Not Populated
**Solution:**
1. Check that prepare_placement_certs.sh ran successfully
2. Verify the ConfigMap was created in the cluster:
   ```bash
   oc get configmap placement-cert-data -n $NAMESPACE -o yaml
   ```
3. Ensure namespace in ConfigMap matches your target namespace
4. Check that the ConfigMap has the expected data keys: `tls.crt`, `tls.key`, `ca.crt`

### Replacement Not Working
**Solution:** Verify the field paths exist in the patch:
```bash
oc kustomize . | yq '.spec.placement.apiOverride.route.spec.tls'
```

### Script Fails with "Secret not found"
**Solution:** Ensure the `placement-custom-route` secret exists. The script waits up to 3 attempts (30 seconds total) for the secret to be created.

## Workflow Diagram

```
┌─────────────────────────────┐
│ placement-custom-route      │
│ (Secret in cluster)         │
└──────────────┬──────────────┘
               │
               │ prepare_placement_certs.sh fetches
               ▼
┌─────────────────────────────┐
│ placement-cert-data         │
│ (ConfigMap in cluster)      │
│ Applied with oc apply       │
└──────────────┬──────────────┘
               │
               │ kustomize references from cluster
               ▼
┌─────────────────────────────┐
│ Kustomize uses replacements │
│ to inject into OpenStack-   │
│ ControlPlane patch          │
└──────────────┬──────────────┘
               │
               │ oc apply
               ▼
┌─────────────────────────────┐
│ OpenStackControlPlane       │
│ with real certificates      │
└─────────────────────────────┘
```

## Why This Approach?

1. **Kustomize-Native**: Uses kustomize's built-in replacement mechanism
2. **Debuggable**: ConfigMap can be inspected in the cluster with `oc get`
3. **No File Modifications**: Doesn't modify any source files during execution
4. **No Generated Files**: ConfigMap applied directly to cluster, no intermediate files
5. **Clean Separation**: Script handles data fetching, kustomize handles application
6. **Test-Friendly**: Easy to use in kuttl tests with clear two-step process

## ConfigMap Details

The `placement-cert-data` ConfigMap is **NOT stored in a file**. It is created directly in the cluster by the script.

### How It's Created

```bash
# The script uses oc apply -f - with a heredoc
oc apply -f - << EOF
apiVersion: v1
kind: ConfigMap
metadata:
  name: placement-cert-data
  namespace: ${NAMESPACE}
data:
  tls.crt: |
    <certificate data>
  tls.key: |
    <key data>
  ca.crt: |
    <CA certificate data>
EOF
```

### In Kuttl Tests

The test flow automatically creates this ConfigMap:

1. **Step 02** - Creates secrets and applies ConfigMap to cluster
2. **Step 03** - Kustomize references the ConfigMap from cluster for replacements
3. **Cleanup** - ConfigMap is cleaned up with the test namespace

### Location

- **Script:** `tests/kuttl/common/prepare_placement_certs.sh`
- **ConfigMap:** `placement-cert-data` in the test namespace (no file)
- **Kustomize:** References the ConfigMap via `replacements` section

## Migration from apply_with_certs.sh

The old `apply_with_certs.sh` approach has been removed. The new approach:
- Creates ConfigMap directly in cluster instead of modifying files
- Uses kustomize replacements instead of sed
- Provides better debugging capabilities
- No intermediate files to manage

## See Also

- [../../tests/kuttl/tests/ctlplane-tls-custom-route/README_DYNAMIC_CERTS.md](../../tests/kuttl/tests/ctlplane-tls-custom-route/README_DYNAMIC_CERTS.md) - Test documentation
