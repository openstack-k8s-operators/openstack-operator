# TLS Custom Route Certificate Test

## Overview

This kuttl test validates custom TLS certificate management for OpenStackControlPlane using dynamically generated certificates from cert-manager. The test specifically validates:

- **Barbican** - Uses secret reference for TLS configuration
- **Placement** - Uses inline certificates injected via Kustomize replacements

The test uses a hybrid approach combining cert-manager, bash scripts, ConfigMaps, and Kustomize replacements to achieve dynamic certificate injection without modifying source files.

## Quick Start

### Running the Test

```bash
export NAMESPACE="openstack-kuttl-tests"
kubectl kuttl test --test ctlplane-tls-custom-route
```

### Manual Certificate Setup

```bash
# Step 1: Create certificates
source ../../common/create_custom_cert.sh
INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"

# Step 2: Create ConfigMap for kustomize
bash ../../common/prepare_placement_certs.sh

# Step 3: Apply OpenStackControlPlane with kustomize
oc kustomize ../../../../config/samples/tls/custom_route_cert | oc apply -n $NAMESPACE -f -

# Step 4: Verify certificates
bash ../../common/osp_check_route_cert.sh barbican
bash ../../common/osp_check_route_cert.sh placement
bash ../../common/verify_route_override_certs.sh placement
```

## Test Structure

```
tests/kuttl/tests/ctlplane-tls-custom-route/
├── 01-deploy-openstack.yaml              # Initial OpenStack deployment
├── 01-assert-deploy-openstack.yaml
├── 02-deploy-custom-route-secret.yaml    # Create certificates and ConfigMap
├── 02-assert-custom-route-secret.yaml
├── 03-deploy-openstack.yaml              # Apply with custom certs via kustomize
├── 03-assert-deploy-openstack.yaml
├── 04-assert-custom-route-cert.yaml      # Verify routes and certificates
├── 04-errors-cleanup.yaml
└── README.md                             # This file
```

## How It Works

### Architecture Overview

```
1. cert-manager generates certificates
   └─> barbican-custom-route (Secret) - used by reference
   └─> placement-custom-route (Secret) - extracted for inline injection

2. prepare_placement_certs.sh extracts placement certs
   └─> placement-cert-data.yaml (File for kustomize)
   └─> placement-cert-data (ConfigMap in cluster for verification)

3. Kustomize includes ConfigMap file and uses replacements to inject
   └─> OpenStackControlPlane with real certificates

4. OpenStack operators create Routes with certificates
   └─> barbican-public (Route) - from secret reference
   └─> placement-public (Route) - from inline certificates

5. Verification scripts validate end-to-end
```

### Certificate Hierarchy

```
selfsigned-issuer (Self-signed Issuer)
  └─> rootca-ingress-custom (CA Certificate)
       └─> rootca-ingress-custom (CA Issuer)
            ├─> barbican-custom-route-cert (Wildcard Certificate)
            │    └─> barbican-custom-route (Secret)
            └─> placement-custom-route-cert (Wildcard Certificate)
                 └─> placement-custom-route (Secret)
                      └─> placement-cert-data.yaml (File + ConfigMap)
```

### Step-by-Step Test Flow

#### Step 1: Initial Deployment
Deploy OpenStackControlPlane without custom certificates.

#### Step 2: Create Certificates and ConfigMap
```yaml
commands:
  - script: |
      source ../../common/create_custom_cert.sh
      INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
      create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"
```

This creates:
- `rootca-ingress-custom` (CA and Issuer)
- `barbican-custom-route` (Secret with TLS cert)
- `placement-custom-route` (Secret with TLS cert)

Then creates the ConfigMap:
```yaml
  - script: |
      bash ../../common/prepare_placement_certs.sh
```

This creates:
- `placement-cert-data.yaml` (File for kustomize)
- `placement-cert-data` (ConfigMap in cluster for verification)

#### Step 3: Apply with Custom Certificates
```yaml
commands:
  - script: |
      oc kustomize ../../../../config/samples/tls/custom_route_cert | oc apply -n $NAMESPACE -f -
```

The kustomize configuration:
1. Includes the ConfigMap file (`placement-cert-data.yaml`) as a resource
2. Patches the OpenStackControlPlane with empty certificate fields
3. Uses `replacements` to inject certificate data from the ConfigMap

#### Step 4: Verify Certificates
```yaml
commands:
  - script: bash ../../common/osp_check_route_cert.sh barbican
  - script: bash ../../common/osp_check_route_cert.sh placement
  - script: bash ../../common/verify_route_override_certs.sh placement
```

## Certificate Creation Utilities

### Available Functions

The `../../common/create_custom_cert.sh` script provides:

| Function | Usage | Description |
|----------|-------|-------------|
| `create_barbican_placement_routes` | `<ingress-domain> [namespace]` | One-command setup for barbican and placement |
| `create_service_route_certificate` | `<service-name> <ingress-domain> [namespace]` | Create certificate for any service |
| `create_custom_issuer` | `<issuer-name> [namespace]` | Create root CA and issuer |
| `create_wildcard_certificate` | `<cert-name> <domain> [issuer-name] [namespace]` | Create wildcard certificate |
| `setup_custom_certificate_infrastructure` | `[namespace]` | Setup complete cert infrastructure |
| `cleanup_custom_certificates` | `[namespace]` | Remove all custom certificates |

### Examples

#### Create Certificates for Any Service

```bash
source ../../common/create_custom_cert.sh

# For keystone
create_service_route_certificate "keystone" "apps-crc.testing" "openstack-kuttl-tests"

# For glance
create_service_route_certificate "glance" "apps-crc.testing" "openstack-kuttl-tests"
```

#### Setup Complete Infrastructure

```bash
# Creates both ingress and internal issuers
setup_custom_certificate_infrastructure "openstack-kuttl-tests"
```

## Kustomize Implementation Details

### Location

The kustomize configuration is in: `config/samples/tls/custom_route_cert/`

### Files

- **`kustomization.yaml`** - Kustomize configuration with replacements
- **`patch.yaml`** - Patch file with Barbican and Placement TLS configuration
- **`placement-cert-data.yaml`** - Generated ConfigMap file (created by `prepare_placement_certs.sh`)

### How Replacements Work

The kustomization includes the ConfigMap file as a resource:

```yaml
resources:
- placement-cert-data.yaml
```

Then uses replacements to inject the certificate data:

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

This replaces the empty `certificate: ""` field in the patch with the actual certificate data from the ConfigMap resource.

### Key Features

- ✅ Uses dynamically generated certificates (no hardcoded values)
- ✅ Kustomize-native approach with replacements
- ✅ ConfigMap file is generated (not checked into git)
- ✅ ConfigMap also applied to cluster for inspection/debugging
- ✅ Clean separation: script generates data, kustomize applies

### OpenStackControlPlane Configuration

#### Barbican (Secret Reference)

```yaml
spec:
  barbican:
    apiOverride:
      tls:
        secretName: barbican-custom-route
```

The operator reads the secret directly and applies it to the route.

#### Placement (Inline Certificates via Kustomize)

```yaml
spec:
  placement:
    apiOverride:
      route:
        spec:
          tls:
            certificate: <from-configmap>
            key: <from-configmap>
            caCertificate: <from-configmap>
            termination: reencrypt
```

The certificates are injected inline using kustomize replacements from the ConfigMap.

## Verification and Debugging

### View Generated ConfigMap File

```bash
cat config/samples/tls/custom_route_cert/placement-cert-data.yaml
```

### View ConfigMap in Cluster

```bash
oc get configmap placement-cert-data -n $NAMESPACE -o yaml
```

### Test Kustomize Output

```bash
# See the full output
oc kustomize config/samples/tls/custom_route_cert | less

# Check just the placement TLS config
oc kustomize config/samples/tls/custom_route_cert | yq '.spec.placement.apiOverride.route.spec.tls'
```

### Check Certificate Status

```bash
# View all certificates
oc get certificate -n $NAMESPACE

# Describe specific certificate
oc describe certificate barbican-custom-route-cert -n $NAMESPACE

# View secret content
oc get secret barbican-custom-route -n $NAMESPACE -o yaml
```

### Decode Certificate

```bash
oc get secret barbican-custom-route -n $NAMESPACE \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
```

### Verify Route TLS

```bash
# Check route configuration
oc get route barbican-public -n $NAMESPACE -o jsonpath='{.spec.tls}' | jq

# Verify certificates match (uses dynamic validation)
bash ../../common/osp_check_route_cert.sh barbican
bash ../../common/osp_check_route_cert.sh placement
```

### Verify OpenStackControlPlane Overrides

```bash
# Verify placement certificates in OpenStackControlPlane match the secret
bash ../../common/verify_route_override_certs.sh placement
```

## Certificate Specifications

### Root CA Certificate
- **Algorithm:** ECDSA P-256
- **Duration:** 87600h (10 years)
- **Usage:** Certificate Sign, CRL Sign
- **IsCA:** true

### Service Certificates
- **Algorithm:** ECDSA P-256
- **Duration:** 8760h (1 year)
- **Renewal:** 720h (30 days) before expiration
- **Usage:** Server Auth, Client Auth
- **DNS Names:** `*.domain.com`, `domain.com`

## Troubleshooting

### Certificate Not Ready

**Problem:** Certificate remains in pending state

**Solution:**
```bash
# Check certificate status
oc describe certificate <cert-name> -n $NAMESPACE

# Check cert-manager logs
oc logs -n cert-manager -l app=cert-manager --tail=50
```

### ConfigMap File Not Found During Kustomize

**Error:** Kustomize fails with `placement-cert-data.yaml` not found

**Solution:** Run `prepare_placement_certs.sh` first to generate the ConfigMap file:
```bash
bash ../../common/prepare_placement_certs.sh
```

This creates `config/samples/tls/custom_route_cert/placement-cert-data.yaml` and also applies it to the cluster.

### Failed to Fetch Certificate Data

**Error:** `ERROR: Failed to fetch certificate data from placement-custom-route secret`

**Solution:** Ensure `placement-custom-route` secret exists:
```bash
oc get secret placement-custom-route -n $NAMESPACE

# If not, create it:
source ../../common/create_custom_cert.sh
INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"
```

### Certificates Not Populated

**Solution:**
1. Check that prepare_placement_certs.sh ran successfully
2. Verify the ConfigMap file was created:
   ```bash
   cat config/samples/tls/custom_route_cert/placement-cert-data.yaml
   ```
3. Verify the ConfigMap was applied to the cluster:
   ```bash
   oc get configmap placement-cert-data -n $NAMESPACE -o yaml
   ```
4. Ensure namespace in ConfigMap matches your target namespace
5. Check that the ConfigMap has the expected data keys: `tls.crt`, `tls.key`, `ca.crt`

### Route Not Using Custom Certificate

**Problem:** Route is using wrong certificate

**Solution:**
```bash
# Verify secret exists
oc get secret <service>-custom-route -n $NAMESPACE

# Check route configuration
oc get route <service>-public -n $NAMESPACE -o yaml

# Verify OpenStackControlPlane configuration
oc get openstackcontrolplane -n $NAMESPACE -o yaml | grep -A 10 "secretName:"
```

### Certificate Mismatch

**Error:** `ERROR: Certificate does not match for placement in OpenStackControlPlane`

**Solution:**
```bash
# Verify secret data
oc get secret placement-custom-route -n $NAMESPACE -o jsonpath='{.data.tls\.crt}' | base64 -d

# Check OpenStackControlPlane
oc get openstackcontrolplane openstack -n $NAMESPACE -o yaml

# Reapply with kustomize
oc kustomize config/samples/tls/custom_route_cert | oc apply -n $NAMESPACE -f -
```

### Issuer Not Ready

**Problem:** Issuer shows "Ready" condition as "False"

**Solution:**
```bash
# Check issuer details
oc describe issuer <issuer-name> -n $NAMESPACE

# Ensure CA secret exists
oc get secret <issuer-name> -n $NAMESPACE

# Verify self-signed issuer
oc get issuer selfsigned-issuer -n $NAMESPACE
```

### Script Execution Issues

**Error:** `Permission denied`

**Solution:**
```bash
chmod +x ../../common/prepare_placement_certs.sh
chmod +x ../../common/verify_route_override_certs.sh
chmod +x ../../common/osp_check_route_cert.sh
```

## Extending for Other Services

To add dynamic certificate support for another service (e.g., keystone):

1. **Create the secret:**
   ```bash
   source ../../common/create_custom_cert.sh
   INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
   create_service_route_certificate "keystone" "${INGRESS_DOMAIN}" "${NAMESPACE}"
   ```

2. **Update kustomize patch** to include keystone TLS configuration

3. **Update prepare_*.sh script** if needed for inline certificates

4. **Add verification:**
   ```yaml
   - script: |
       bash ../../common/osp_check_route_cert.sh "keystone"
   ```

## Cleanup

```bash
# Remove all custom certificates and issuers
source ../../common/create_custom_cert.sh
cleanup_custom_certificates "$NAMESPACE"

# Remove generated ConfigMap file
rm -f config/samples/tls/custom_route_cert/placement-cert-data.yaml

# Or manually
oc delete certificate --all -n $NAMESPACE
oc delete issuer --all -n $NAMESPACE
oc delete secret -l cert-manager.io/common-name -n $NAMESPACE
oc delete configmap placement-cert-data -n $NAMESPACE
```

## Security Considerations

⚠️ **Important:** These certificates are for **TESTING PURPOSES ONLY**

## Files Reference

### Test Files
- `01-deploy-openstack.yaml` - Initial deployment
- `02-deploy-custom-route-secret.yaml` - Create certificates and ConfigMap
- `03-deploy-openstack.yaml` - Apply with kustomize
- `04-assert-custom-route-cert.yaml` - Verify certificates

### Common Utilities
- `../../common/create_custom_cert.sh` - Certificate creation functions
- `../../common/prepare_placement_certs.sh` - ConfigMap generation
- `../../common/osp_check_route_cert.sh` - Route certificate validation
- `../../common/verify_route_override_certs.sh` - OpenStackControlPlane validation

### Kustomize Files
- `../../../../config/samples/tls/custom_route_cert/kustomization.yaml` - Kustomize config
- `../../../../config/samples/tls/custom_route_cert/patch.yaml` - TLS patch

## Related Documentation

- [Cert-Manager Documentation](https://cert-manager.io/docs/)
- [Kustomize Documentation](https://kustomize.io/)
- OpenStack Operator TLS Documentation
