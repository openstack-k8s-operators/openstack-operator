#!/bin/bash
#
# Verify that route override certificates in OpenStackControlPlane match the secret
#

set -e

SERVICE_NAME=$1
NAMESPACE=${NAMESPACE:-openstack-kuttl-tests}

if [[ -z "${SERVICE_NAME}" ]]; then
    echo "ERROR: Service name is required"
    echo "Usage: $0 <service-name>"
    exit 1
fi

echo "Verifying ${SERVICE_NAME} route override certificates..."

# Get expected certificate values from the secret
EXPECTED_CERT=$(oc get secret ${SERVICE_NAME}-custom-route -n ${NAMESPACE} -o jsonpath='{.data.tls\.crt}' | base64 -d)
EXPECTED_KEY=$(oc get secret ${SERVICE_NAME}-custom-route -n ${NAMESPACE} -o jsonpath='{.data.tls\.key}' | base64 -d)
EXPECTED_CA=$(oc get secret ${SERVICE_NAME}-custom-route -n ${NAMESPACE} -o jsonpath='{.data.ca\.crt}' | base64 -d)

if [[ -z "${EXPECTED_CERT}" ]] || [[ -z "${EXPECTED_KEY}" ]] || [[ -z "${EXPECTED_CA}" ]]; then
    echo "ERROR: Failed to fetch certificate data from ${SERVICE_NAME}-custom-route secret"
    exit 1
fi

# Get actual certificate values from OpenStackControlPlane
ACTUAL_CERT=$(oc get openstackcontrolplane openstack -n ${NAMESPACE} -o jsonpath="{.spec.${SERVICE_NAME}.apiOverride.route.spec.tls.certificate}")
ACTUAL_KEY=$(oc get openstackcontrolplane openstack -n ${NAMESPACE} -o jsonpath="{.spec.${SERVICE_NAME}.apiOverride.route.spec.tls.key}")
ACTUAL_CA=$(oc get openstackcontrolplane openstack -n ${NAMESPACE} -o jsonpath="{.spec.${SERVICE_NAME}.apiOverride.route.spec.tls.caCertificate}")

if [[ -z "${ACTUAL_CERT}" ]] || [[ -z "${ACTUAL_KEY}" ]] || [[ -z "${ACTUAL_CA}" ]]; then
    echo "ERROR: Failed to fetch certificate data from OpenStackControlPlane ${SERVICE_NAME} override"
    exit 1
fi

# Trim whitespace for comparison
TRIMMED_EXPECTED_CERT=$(echo "$EXPECTED_CERT" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
TRIMMED_ACTUAL_CERT=$(echo "$ACTUAL_CERT" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

TRIMMED_EXPECTED_KEY=$(echo "$EXPECTED_KEY" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
TRIMMED_ACTUAL_KEY=$(echo "$ACTUAL_KEY" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

TRIMMED_EXPECTED_CA=$(echo "$EXPECTED_CA" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
TRIMMED_ACTUAL_CA=$(echo "$ACTUAL_CA" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

# Compare certificates
if [[ "$TRIMMED_EXPECTED_CERT" != "$TRIMMED_ACTUAL_CERT" ]]; then
    echo "ERROR: Certificate does not match for ${SERVICE_NAME} in OpenStackControlPlane"
    echo "Expected cert from secret (first 100 chars): ${TRIMMED_EXPECTED_CERT:0:100}"
    echo "Actual cert from controlplane (first 100 chars): ${TRIMMED_ACTUAL_CERT:0:100}"
    exit 1
fi

# Compare keys
if [[ "$TRIMMED_EXPECTED_KEY" != "$TRIMMED_ACTUAL_KEY" ]]; then
    echo "ERROR: Private key does not match for ${SERVICE_NAME} in OpenStackControlPlane"
    exit 1
fi

# Compare CA certificates
if [[ "$TRIMMED_EXPECTED_CA" != "$TRIMMED_ACTUAL_CA" ]]; then
    echo "ERROR: CA certificate does not match for ${SERVICE_NAME} in OpenStackControlPlane"
    echo "Expected CA from secret (first 100 chars): ${TRIMMED_EXPECTED_CA:0:100}"
    echo "Actual CA from controlplane (first 100 chars): ${TRIMMED_ACTUAL_CA:0:100}"
    exit 1
fi

echo "✓ All certificates match for ${SERVICE_NAME}"
