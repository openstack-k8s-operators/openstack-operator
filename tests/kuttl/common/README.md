# Custom Certificate Management for Kuttl Tests

This directory contains utilities for creating and managing custom TLS certificates using cert-manager for OpenStack operator kuttl tests.

## Files

- **`create_custom_cert.sh`** - Main script with bash functions for certificate creation
- **`osp_check_route_cert.sh`** - Verification script for route certificates
- **`verify_route_override_certs.sh`** - Verification script for OpenStackControlPlane overrides
- **`prepare_placement_certs.sh`** - Helper script to create ConfigMap from certificates
- **`custom-ingress-issuer.yaml`** - YAML template for custom ingress issuer
- **`custom-internal-issuer.yaml`** - YAML template for custom internal issuer
- **`custom-barbican-route.yaml`** - Pre-generated barbican route secret
- **`custom-ca.yaml`** - Custom CA bundle for testing

## Quick Start

### One-Command Setup for Barbican and Placement

```bash
source ../../common/create_custom_cert.sh && \
INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}') && \
create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"
```

This creates:
- `rootca-ingress-custom` (CA and Issuer)
- `barbican-custom-route` (Secret with TLS cert)
- `placement-custom-route` (Secret with TLS cert)

### Use in Kuttl Test

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
  - script: |
      source ../../common/create_custom_cert.sh
      INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
      create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"
```

### Verify Certificates

```bash
# Check route certificates
bash ../../common/osp_check_route_cert.sh barbican
bash ../../common/osp_check_route_cert.sh placement

# Verify OpenStackControlPlane overrides
bash ../../common/verify_route_override_certs.sh placement
```

## Available Functions

### Main Functions

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
# For keystone
create_service_route_certificate "keystone" "apps-crc.testing" "openstack-kuttl-tests"

# For glance
create_service_route_certificate "glance" "apps-crc.testing" "openstack-kuttl-tests"
```

#### Create Custom Issuer

```bash
# For internal services
create_custom_issuer "rootca-internal-custom" "openstack-kuttl-tests"
```

#### Setup Complete Infrastructure

```bash
# Creates both ingress and internal issuers
setup_custom_certificate_infrastructure "openstack-kuttl-tests"
```

## Architecture

### Certificate Hierarchy

```
selfsigned-issuer (Self-signed Issuer)
  └─> rootca-ingress-custom (CA Certificate)
       └─> rootca-ingress-custom (CA Issuer)
            └─> barbican-custom-route-cert (Wildcard Certificate)
                 └─> barbican-custom-route (Route Secret)
            └─> placement-custom-route-cert (Wildcard Certificate)
                 └─> placement-custom-route (Route Secret)
```

### Certificate Types

1. **Self-Signed Issuer** - Root of trust for all custom certificates
2. **Root CA Certificate and Issuer** - 10-year validity, ECDSA 256-bit keys
3. **Wildcard Certificates** (`*.<domain>`) - 1-year validity, 30-day renewal window

## Using with OpenStackControlPlane

### Barbican (Secret Reference)

```yaml
spec:
  barbican:
    apiOverride:
      tls:
        secretName: barbican-custom-route
```

### Placement (Inline Certificates via Kustomize)

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

The placement certificates are injected using kustomize replacements. See `config/samples/tls/custom_route_cert/` for details.

## Verification

### Check Certificate Status

```bash
# View all certificates
oc get certificate -n openstack-kuttl-tests

# Describe specific certificate
oc describe certificate barbican-custom-route-cert -n openstack-kuttl-tests

# View secret content
oc get secret barbican-custom-route -n openstack-kuttl-tests -o yaml
```

### Decode Certificate

```bash
oc get secret barbican-custom-route -n openstack-kuttl-tests \
  -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -text -noout
```

### Verify Route TLS

```bash
# Check route configuration
oc get route barbican-public -n openstack-kuttl-tests -o jsonpath='{.spec.tls}' | jq

# Verify certificates match
bash ../../common/osp_check_route_cert.sh barbican
```

## Troubleshooting

### Certificate Not Ready

**Problem:** Certificate remains in pending state

**Solution:**
```bash
# Check certificate status
oc describe certificate <cert-name> -n openstack-kuttl-tests

# Check cert-manager logs
oc logs -n cert-manager -l app=cert-manager --tail=50
```

### Route Not Using Custom Certificate

**Problem:** Route is using wrong certificate

**Solution:**
```bash
# Verify secret exists
oc get secret <service>-custom-route -n openstack-kuttl-tests

# Check route configuration
oc get route <service>-public -n openstack-kuttl-tests -o yaml

# Verify OpenStackControlPlane configuration
oc get openstackcontrolplane -n openstack-kuttl-tests -o yaml | grep -A 10 "secretName:"
```

### Issuer Not Ready

**Problem:** Issuer shows "Ready" condition as "False"

**Solution:**
```bash
# Check issuer details
oc describe issuer <issuer-name> -n openstack-kuttl-tests

# Ensure CA secret exists
oc get secret <issuer-name> -n openstack-kuttl-tests

# Verify self-signed issuer
oc get issuer selfsigned-issuer -n openstack-kuttl-tests
```

### Script Returns Error: "Secret not found"

**Problem:** `prepare_placement_certs.sh` fails

**Solution:**
The script waits up to 3 attempts (30 seconds) for secrets to be created. Ensure:
```bash
# Check if secret exists
oc get secret placement-custom-route -n openstack-kuttl-tests

# If not, create it first
source ../../common/create_custom_cert.sh
INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"
```

## Complete Kuttl Test Example

### Test Structure

```
tests/ctlplane-tls-custom-route/
├── 01-deploy-openstack.yaml
├── 01-assert-deploy-openstack.yaml
├── 02-deploy-custom-route-secret.yaml  # Create certificates
├── 02-assert-custom-route-secret.yaml  # Verify certificates
├── 03-deploy-openstack.yaml            # Apply with custom certs
├── 03-assert-deploy-openstack.yaml
├── 04-assert-custom-route-cert.yaml    # Verify routes
└── 04-errors-cleanup.yaml
```

### Example: Create Certificates (02-deploy-custom-route-secret.yaml)

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
  - script: |
      source ../../common/create_custom_cert.sh
      INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
      create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"

  - script: |
      bash ../../common/prepare_placement_certs.sh
```

### Example: Verify Certificates (04-assert-custom-route-cert.yaml)

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestAssert
commands:
  - script: bash ../../common/osp_check_route_cert.sh barbican
  - script: bash ../../common/osp_check_route_cert.sh placement
  - script: bash ../../common/verify_route_override_certs.sh placement
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

## Cleanup

```bash
# Remove all custom certificates and issuers
cleanup_custom_certificates "openstack-kuttl-tests"

# Or manually
oc delete certificate --all -n openstack-kuttl-tests
oc delete issuer --all -n openstack-kuttl-tests
oc delete secret -l cert-manager.io/common-name -n openstack-kuttl-tests
```

## Security Considerations

⚠️ **Important:** These certificates are for **TESTING PURPOSES ONLY**

- Self-signed CAs should not be used in production
- Private keys are stored in Kubernetes secrets
- Certificates have relatively short lifetimes for testing
- ECDSA P-256 is safe for testing but verify for production requirements

## Related Documentation

- [Cert-Manager Documentation](https://cert-manager.io/docs/)
- [config/samples/tls/custom_route_cert/README.md](../../../config/samples/tls/custom_route_cert/README.md) - Kustomize certificate injection
- [tests/ctlplane-tls-custom-route/README_DYNAMIC_CERTS.md](../tests/ctlplane-tls-custom-route/README_DYNAMIC_CERTS.md) - Test documentation
