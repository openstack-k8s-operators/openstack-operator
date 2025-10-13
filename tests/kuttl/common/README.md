# Common Certificate Management Utilities

This directory contains shared utilities for creating and managing custom TLS certificates using cert-manager in OpenStack operator kuttl tests.

## Files

- **`create_custom_cert.sh`** - Main script with bash functions for certificate creation
- **`osp_check_route_cert.sh`** - Verification script for route certificates
- **`verify_route_override_certs.sh`** - Verification script for OpenStackControlPlane overrides
- **`prepare_placement_certs.sh`** - Helper script to create ConfigMap from certificates
- **`custom-ingress-issuer.yaml`** - YAML template for custom ingress issuer
- **`custom-internal-issuer.yaml`** - YAML template for custom internal issuer
- **`custom-barbican-route.yaml`** - Pre-generated barbican route secret
- **`custom-ca.yaml`** - Custom CA bundle for testing

## Quick Reference

### Create Certificates for Barbican and Placement

```bash
source ../../common/create_custom_cert.sh
INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"
```

### Main Functions

| Function | Usage | Description |
|----------|-------|-------------|
| `create_barbican_placement_routes` | `<ingress-domain> [namespace]` | One-command setup for barbican and placement |
| `create_service_route_certificate` | `<service-name> <ingress-domain> [namespace]` | Create certificate for any service |
| `create_custom_issuer` | `<issuer-name> [namespace]` | Create root CA and issuer |
| `create_wildcard_certificate` | `<cert-name> <domain> [issuer-name] [namespace]` | Create wildcard certificate |
| `setup_custom_certificate_infrastructure` | `[namespace]` | Setup complete cert infrastructure |
| `cleanup_custom_certificates` | `[namespace]` | Remove all custom certificates |

### Verification Functions

```bash
# Verify route certificate matches secret
bash ../../common/osp_check_route_cert.sh <service-name>

# Verify OpenStackControlPlane override matches secret
bash ../../common/verify_route_override_certs.sh <service-name>
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

## Usage in Kuttl Tests

```yaml
apiVersion: kuttl.dev/v1beta1
kind: TestStep
commands:
  - script: |
      source ../../common/create_custom_cert.sh
      INGRESS_DOMAIN=$(oc get ingresses.config.openshift.io cluster -o jsonpath='{.spec.domain}')
      create_barbican_placement_routes "${INGRESS_DOMAIN}" "${NAMESPACE}"
```

## Examples

### Create Certificates for Any Service

```bash
source ../../common/create_custom_cert.sh

# For keystone
create_service_route_certificate "keystone" "apps-crc.testing" "openstack-kuttl-tests"

# For glance
create_service_route_certificate "glance" "apps-crc.testing" "openstack-kuttl-tests"
```

### Setup Complete Infrastructure

```bash
# Creates both ingress and internal issuers
setup_custom_certificate_infrastructure "openstack-kuttl-tests"
```

### Cleanup

```bash
# Remove all custom certificates and issuers
cleanup_custom_certificates "openstack-kuttl-tests"
```

## Security Considerations

⚠️ **Important:** These certificates are for **TESTING PURPOSES ONLY**

## Complete Documentation

For detailed documentation including test flow, architecture, troubleshooting, and implementation details, see:

**[tests/kuttl/tests/ctlplane-tls-custom-route/README.md](../tests/ctlplane-tls-custom-route/README.md)**

## Related Documentation

- [Cert-Manager Documentation](https://cert-manager.io/docs/)
- [ctlplane-tls-custom-route Test](../tests/ctlplane-tls-custom-route/README.md)
