#!/bin/bash
#
# Helper functions to create custom issuers and wildcard certificates for kuttl tests
#

# Wait for a resource to reach a specific state
function wait_for_state() {
    local object="$1"
    local state="$2"
    local timeout="$3"
    local namespace="${4:-openstack-kuttl-tests}"

    echo "Waiting for '${object}' in namespace '${namespace}' to become '${state}'..."
    oc wait --for=${state} --timeout=${timeout} ${object} -n="${namespace}"
    return $?
}

# Create a self-signed issuer (prerequisite for CA certificate)
function create_selfsigned_issuer() {
    local namespace="${1:-openstack-kuttl-tests}"

    echo "Creating self-signed issuer in namespace ${namespace}..."
    oc apply -f - << EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: selfsigned-issuer
  namespace: ${namespace}
spec:
  selfSigned: {}
EOF

    if wait_for_state "issuer/selfsigned-issuer" "condition=Ready" "2m" "${namespace}"; then
        echo "Self-signed issuer is ready"
    else
        echo "Failed to create self-signed issuer"
        oc describe issuer selfsigned-issuer -n ${namespace}
        return 1
    fi
}

# Create a custom root CA certificate and issuer
# Usage: create_custom_issuer <issuer-name> <namespace>
function create_custom_issuer() {
    local issuer_name="${1:-rootca-custom}"
    local namespace="${2:-openstack-kuttl-tests}"

    echo "Creating custom root CA and issuer: ${issuer_name} in namespace ${namespace}..."

    # Ensure self-signed issuer exists
    if ! oc get issuer selfsigned-issuer -n ${namespace} &>/dev/null; then
        create_selfsigned_issuer "${namespace}" || return 1
    fi

    # Create the root CA certificate
    oc apply -f - << EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ${issuer_name}
  namespace: ${namespace}
spec:
  commonName: ${issuer_name}
  isCA: true
  duration: 87600h  # 10 years
  privateKey:
    algorithm: ECDSA
    size: 256
  issuerRef:
    name: selfsigned-issuer
    kind: Issuer
  secretName: ${issuer_name}
EOF

    if wait_for_state "certificate/${issuer_name}" "condition=Ready" "5m" "${namespace}"; then
        echo "Root CA certificate ${issuer_name} is ready"
    else
        echo "Failed to create root CA certificate"
        oc describe certificate ${issuer_name} -n ${namespace}
        return 1
    fi

    # Create the issuer that uses this CA
    oc apply -f - << EOF
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: ${issuer_name}
  namespace: ${namespace}
spec:
  ca:
    secretName: ${issuer_name}
EOF

    if wait_for_state "issuer/${issuer_name}" "condition=Ready" "2m" "${namespace}"; then
        echo "Custom issuer ${issuer_name} is ready"
    else
        echo "Failed to create custom issuer"
        oc describe issuer ${issuer_name} -n ${namespace}
        return 1
    fi
}

# Create a wildcard certificate for ingress/routes
# Usage: create_wildcard_certificate <cert-name> <domain> <issuer-name> <namespace>
function create_wildcard_certificate() {
    local cert_name="${1}"
    local domain="${2}"
    local issuer_name="${3:-rootca-custom}"
    local namespace="${4:-openstack-kuttl-tests}"

    if [[ -z "${cert_name}" ]] || [[ -z "${domain}" ]]; then
        echo "Error: cert_name and domain are required"
        echo "Usage: create_wildcard_certificate <cert-name> <domain> [issuer-name] [namespace]"
        return 1
    fi

    echo "Creating wildcard certificate ${cert_name} for *.${domain}..."

    oc apply -f - << EOF
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: ${cert_name}
  namespace: ${namespace}
spec:
  commonName: "*.${domain}"
  dnsNames:
  - "*.${domain}"
  - "${domain}"
  usages:
  - server auth
  - client auth
  issuerRef:
    kind: Issuer
    name: ${issuer_name}
  secretName: ${cert_name}
  privateKey:
    algorithm: ECDSA
    size: 256
  duration: 8760h  # 1 year
  renewBefore: 720h  # 30 days
EOF

    if wait_for_state "certificate/${cert_name}" "condition=Ready" "5m" "${namespace}"; then
        echo "Wildcard certificate ${cert_name} is ready"
    else
        echo "Failed to create wildcard certificate"
        oc describe certificate ${cert_name} -n ${namespace}
        return 1
    fi
}

# Create a wildcard certificate suitable for OpenStack service routes
# This creates both the certificate and extracts it to a route-compatible secret
# Usage: create_service_route_certificate <service-name> <ingress-domain> <namespace>
function create_service_route_certificate() {
    local service_name="${1}"
    local ingress_domain="${2}"
    local namespace="${3:-openstack-kuttl-tests}"
    local issuer_name="rootca-ingress-custom"
    local cert_name="${service_name}-custom-route-cert"
    local route_secret_name="${service_name}-custom-route"

    if [[ -z "${service_name}" ]] || [[ -z "${ingress_domain}" ]]; then
        echo "Error: service_name and ingress_domain are required"
        echo "Usage: create_service_route_certificate <service-name> <ingress-domain> [namespace]"
        return 1
    fi

    echo "Creating custom route certificate for ${service_name}..."

    # Ensure custom issuer exists
    if ! oc get issuer ${issuer_name} -n ${namespace} &>/dev/null; then
        echo "Custom issuer ${issuer_name} does not exist, creating..."
        create_custom_issuer "${issuer_name}" "${namespace}" || return 1
    fi

    # Create wildcard certificate
    create_wildcard_certificate "${cert_name}" "${ingress_domain}" "${issuer_name}" "${namespace}" || return 1

    # Wait a moment for the secret to be fully populated
    sleep 2

    # Extract certificate data and create route-compatible secret
    echo "Creating route-compatible secret ${route_secret_name}..."

    local tls_crt
    local tls_key
    local ca_crt
    tls_crt=$(oc get secret ${cert_name} -n ${namespace} -o jsonpath='{.data.tls\.crt}')
    tls_key=$(oc get secret ${cert_name} -n ${namespace} -o jsonpath='{.data.tls\.key}')
    ca_crt=$(oc get secret ${cert_name} -n ${namespace} -o jsonpath='{.data.ca\.crt}')

    if [[ -z "${tls_crt}" ]] || [[ -z "${tls_key}" ]]; then
        echo "Error: Failed to extract certificate data from secret ${cert_name}"
        return 1
    fi

    # Create the route secret with the certificate
    oc apply -f - << EOF
apiVersion: v1
kind: Secret
type: kubernetes.io/tls
metadata:
  name: ${route_secret_name}
  namespace: ${namespace}
data:
  tls.crt: ${tls_crt}
  tls.key: ${tls_key}
  ca.crt: ${ca_crt}
EOF

    echo "Route certificate secret ${route_secret_name} created successfully"
}

# Setup complete custom certificate infrastructure for kuttl tests
# This creates ingress and internal issuers for comprehensive testing
# Usage: setup_custom_certificate_infrastructure <namespace>
function setup_custom_certificate_infrastructure() {
    local namespace="${1:-openstack-kuttl-tests}"

    echo "Setting up custom certificate infrastructure in namespace ${namespace}..."

    # Create self-signed issuer (root of trust)
    create_selfsigned_issuer "${namespace}" || return 1

    # Create custom ingress issuer (for public routes)
    create_custom_issuer "rootca-ingress-custom" "${namespace}" || return 1

    # Create custom internal issuer (for internal services)
    create_custom_issuer "rootca-internal-custom" "${namespace}" || return 1

    echo "Custom certificate infrastructure setup complete"
}

# Create route certificates for barbican and placement
# Usage: create_barbican_placement_routes <ingress-domain> <namespace>
function create_barbican_placement_routes() {
    local ingress_domain="${1}"
    local namespace="${2:-openstack-kuttl-tests}"

    if [[ -z "${ingress_domain}" ]]; then
        echo "Error: ingress_domain is required"
        echo "Usage: create_barbican_placement_routes <ingress-domain> [namespace]"
        return 1
    fi

    echo "Creating route certificates for barbican and placement..."

    # Setup infrastructure if needed
    if ! oc get issuer rootca-ingress-custom -n ${namespace} &>/dev/null; then
        setup_custom_certificate_infrastructure "${namespace}" || return 1
    fi

    # Create certificates for both services
    create_service_route_certificate "barbican" "${ingress_domain}" "${namespace}" || return 1
    create_service_route_certificate "placement" "${ingress_domain}" "${namespace}" || return 1

    echo "Route certificates created successfully"
    echo "  - barbican-custom-route"
    echo "  - placement-custom-route"
}

# Cleanup custom certificates and issuers
# Usage: cleanup_custom_certificates <namespace>
function cleanup_custom_certificates() {
    local namespace="${1:-openstack-kuttl-tests}"

    echo "Cleaning up custom certificates and issuers in namespace ${namespace}..."

    # Delete certificates
    oc delete certificate --all -n ${namespace} --ignore-not-found=true

    # Delete issuers
    oc delete issuer --all -n ${namespace} --ignore-not-found=true

    # Delete route secrets
    oc delete secret -l custom-route=true -n ${namespace} --ignore-not-found=true

    echo "Cleanup complete"
}
