# Dynamic Certificate Management for ctlplane-tls-custom-route Test

## Overview

This test has been updated to use dynamically generated certificates instead of hardcoded placeholder values. This ensures the test works with real cert-manager generated certificates.

## Changes Made

### 1. Updated Certificate Validation Script

**File:** `../../common/osp_check_route_cert.sh`

Changed from hardcoded certificate values to dynamic fetching:

```bash
# OLD: Hardcoded certificate values
EXPECTED_CERTIFICATE="-----BEGIN CERTIFICATE-----..."

# NEW: Fetch from secret
EXPECTED_CERTIFICATE=$(oc get secret ${ROUTE_NAME}-custom-route -n $NAMESPACE -o jsonpath='{.data.tls\.crt}' | base64 -d)
```

This allows the script to validate certificates for any service by fetching them from the `<service>-custom-route` secret.

### 2. Created Dynamic Application Script

**File:** `../../../../config/samples/tls/custom_route_cert/apply_with_certs.sh`

New script that:
- Fetches certificate data from `placement-custom-route` secret
- Generates a patch with the actual certificate content
- Applies the OpenStackControlPlane with real certificates

**Usage:**
```bash
bash ../../../../config/samples/tls/custom_route_cert/apply_with_certs.sh
```

### 3. Created Route Override Verification Script

**File:** `../../common/verify_route_override_certs.sh`

New script that verifies certificates in OpenStackControlPlane match the secret:
- Compares certificate from secret vs. OpenStackControlPlane spec
- Compares private key from secret vs. OpenStackControlPlane spec
- Compares CA certificate from secret vs. OpenStackControlPlane spec

**Usage:**
```bash
bash ../../common/verify_route_override_certs.sh placement
```

### 4. Updated Test Steps

**File:** `02-deploy-custom-route-secret.yaml`

Added ConfigMap generation after creating secrets:

```yaml
commands:
  - script: |
      source ../../common/create_custom_cert.sh
      INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
      create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"

  - script: |
      # Generate ConfigMap for kustomize
      bash ../../common/prepare_placement_certs.sh
```

**File:** `03-deploy-openstack.yaml`

Simplified to pure kustomize apply:

```yaml
commands:
  - script: |
      # ConfigMap created in step 02
      oc kustomize ../../../../config/samples/tls/custom_route_cert | oc apply -n $NAMESPACE -f -
```

**File:** `03-assert-deploy-openstack.yaml`

Removed hardcoded certificate placeholders:

```yaml
# OLD:
placement:
  apiOverride:
    route:
      spec:
        tls:
          certificate: |
            CERT123
          key: |
            KEY123
          caCertificate: |
            CACERT123
          termination: reencrypt

# NEW:
placement:
  apiOverride:
    route:
      spec:
        tls:
          termination: reencrypt
```

**File:** `04-assert-custom-route-cert.yaml`

Added dynamic certificate verification:

```yaml
commands:
  - script: |
      echo "Checking barbican custom route certificate..."
      bash ../../common/osp_check_route_cert.sh "barbican"

  - script: |
      echo "Checking placement custom route certificate..."
      bash ../../common/osp_check_route_cert.sh "placement"

  - script: |
      echo "Verifying placement route override certificates in OpenStackControlPlane..."
      bash ../../common/verify_route_override_certs.sh "placement"
```

## Test Flow

1. **Step 02:**
   - Deploy custom route secrets (barbican-custom-route, placement-custom-route)
   - Generate ConfigMap from placement-custom-route secret
2. **Step 03:** Apply OpenStackControlPlane with kustomize (using ConfigMap from step 02)
3. **Step 04:** Verify certificates:
   - Check barbican route has correct certificate from secret
   - Check placement route has correct certificate from secret
   - Verify placement OpenStackControlPlane override matches secret

## Benefits

✅ **No hardcoded certificates** - Uses real cert-manager generated certificates

✅ **Reusable** - Works with any dynamically generated certificate

✅ **Maintainable** - No need to update hardcoded values when certificates change

✅ **Flexible** - Easy to extend for additional services

✅ **Validated** - Multiple levels of certificate verification

## How It Works

### Certificate Flow

```
1. cert-manager generates certificates
   └─> placement-custom-route (Secret)
       ├─ tls.crt
       ├─ tls.key
       └─ ca.crt

2. apply_with_certs.sh fetches certificate from secret
   └─> Creates dynamic patch with actual certificate data

3. OpenStackControlPlane applied with real certificates
   └─> spec.placement.apiOverride.route.spec.tls
       ├─ certificate: <actual cert>
       ├─ key: <actual key>
       ├─ caCertificate: <actual CA>
       └─ termination: reencrypt

4. Operator creates Route with certificate
   └─> placement-public (Route)
       └─ spec.tls
           ├─ certificate: <from OpenStackControlPlane>
           ├─ key: <from OpenStackControlPlane>
           └─ caCertificate: <from OpenStackControlPlane>

5. Verification scripts validate:
   ✓ Route certificate matches secret
   ✓ OpenStackControlPlane override matches secret
```

## Extending for Other Services

To add dynamic certificate support for another service (e.g., keystone):

1. **Create the secret:**
   ```bash
   source ../../common/create_custom_cert.sh
   create_service_route_certificate "keystone" "${INGRESS_DOMAIN}" "${NAMESPACE}"
   ```

2. **Update apply_with_certs.sh** to include keystone section:
   ```bash
   KEYSTONE_CERT=$(oc get secret keystone-custom-route -n "${NAMESPACE}" -o jsonpath='{.data.tls\.crt}' | base64 -d)
   # ... add to patch
   ```

3. **Add verification:**
   ```yaml
   - script: |
       bash ../../common/osp_check_route_cert.sh "keystone"
   - script: |
       bash ../../common/verify_route_override_certs.sh "keystone"
   ```

## Troubleshooting

### Certificate Not Found

**Error:** `ERROR: Failed to fetch certificate data from placement-custom-route secret`

**Solution:**
- Ensure the secret exists: `oc get secret placement-custom-route -n $NAMESPACE`
- Check secret has correct data: `oc get secret placement-custom-route -n $NAMESPACE -o yaml`

### Certificate Mismatch

**Error:** `ERROR: Certificate does not match for placement in OpenStackControlPlane`

**Solution:**
- Verify secret data: `oc get secret placement-custom-route -n $NAMESPACE -o jsonpath='{.data.tls\.crt}' | base64 -d`
- Check OpenStackControlPlane: `oc get openstackcontrolplane openstack -n $NAMESPACE -o yaml`
- Reapply: `bash ../../../../config/samples/tls/custom_route_cert/apply_with_certs.sh`

### Script Execution Issues

**Error:** `Permission denied`

**Solution:**
```bash
chmod +x ../../../../config/samples/tls/custom_route_cert/apply_with_certs.sh
chmod +x ../../common/verify_route_override_certs.sh
```

## Files Summary

| File | Purpose |
|------|---------|
| `apply_with_certs.sh` | Fetch certs from secrets and apply OpenStackControlPlane |
| `verify_route_override_certs.sh` | Verify OpenStackControlPlane override matches secret |
| `osp_check_route_cert.sh` | Verify Route certificate matches secret |
| `03-deploy-openstack.yaml` | Updated to use apply_with_certs.sh |
| `03-assert-deploy-openstack.yaml` | Removed hardcoded cert placeholders |
| `04-assert-custom-route-cert.yaml` | Added verification commands |

## Testing Locally

```bash
# Set namespace
export NAMESPACE="openstack-kuttl-tests"

# Run the test
kubectl kuttl test --test ctlplane-tls-custom-route

# Or run individual steps
cd tests/kuttl/tests/ctlplane-tls-custom-route/
bash 03-deploy-openstack.yaml
bash ../../common/verify_route_override_certs.sh placement
```

## References

- [Custom Certificate Creation Guide](../../common/README.md)
