# Custom Route Certificate Configuration

This directory implements custom TLS certificate management for OpenStackControlPlane using a hybrid ConfigMap + Kustomize approach.

## Quick Start

```bash
# Step 1: Generate ConfigMap from secret
bash ../../../tests/kuttl/common/prepare_placement_certs.sh

# Step 2: Apply with kustomize
oc kustomize . | oc apply -n $NAMESPACE -f -
```

## Files

### Active Files

- **`kustomization.yaml`** - Kustomize configuration with replacements (references cluster ConfigMap)
- **`patch.yaml`** - Patch file with Barbican and Placement TLS configuration
- **`README.md`** - This file
- **`IMPLEMENTATION.md`** - Detailed implementation guide (includes ConfigMap details)

## How It Works

1. **`tests/kuttl/common/prepare_placement_certs.sh`** fetches certificates from `placement-custom-route` secret and creates a ConfigMap in the cluster
2. **`kustomization.yaml`** uses `replacements` to inject certificate data from the ConfigMap into OpenStackControlPlane
3. The result is a fully configured OpenStackControlPlane with real certificates

## Architecture

```
Secret (placement-custom-route)
    ↓
tests/kuttl/common/prepare_placement_certs.sh creates
    ↓
ConfigMap in cluster (placement-cert-data)
    ↓
Kustomize replacements inject into
    ↓
OpenStackControlPlane patch
    ↓
Final OpenStackControlPlane with certificates
```

## Usage in Kuttl Tests

See `tests/kuttl/tests/ctlplane-tls-custom-route/02-deploy-custom-route-secret.yaml`:

```yaml
commands:
  - script: |
      bash ../../common/prepare_placement_certs.sh
```

And `tests/kuttl/tests/ctlplane-tls-custom-route/03-deploy-openstack.yaml`:

```yaml
commands:
  - script: |
      oc kustomize ../../../../config/samples/tls/custom_route_cert | oc apply -n $NAMESPACE -f -
```

## Benefits

- ✅ **Kustomize-Native**: Uses kustomize replacements
- ✅ **No File Modification**: Doesn't modify source files
- ✅ **Debuggable**: Generated ConfigMap can be inspected
- ✅ **Clean Separation**: Script generates data, kustomize applies
- ✅ **Test-Friendly**: Easy to use in CI/CD pipelines

## Requirements

- `oc` or `kubectl` CLI
- Access to namespace with `placement-custom-route` secret
- `bash` for running prepare_placement_certs.sh

## Troubleshooting

### Certificate Not Found
```bash
oc get secret placement-custom-route -n $NAMESPACE
```

### View ConfigMap in Cluster
```bash
oc get configmap placement-cert-data -n $NAMESPACE -o yaml
```

### Test Kustomize Output
```bash
oc kustomize . | yq '.spec.placement.apiOverride.route.spec.tls'
```

## Related Documentation

For detailed information, see:
- [IMPLEMENTATION.md](./IMPLEMENTATION.md) - Complete implementation guide (includes ConfigMap details)

## Migration from Old Approach

If you were using `apply_with_certs.sh` (now removed), the new approach is:

**Old:**
```bash
bash apply_with_certs.sh
```

**New:**
```bash
bash ../../../tests/kuttl/common/prepare_placement_certs.sh
oc kustomize . | oc apply -n $NAMESPACE -f -
```

The new approach is more kustomize-native and doesn't modify source files during execution.
