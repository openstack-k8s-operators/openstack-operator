#!/bin/bash

ROUTE_NAME=$1

EXPECTED_CERTIFICATE=$(oc get secret ${ROUTE_NAME}-custom-route -n $NAMESPACE -o jsonpath='{.data.tls\.crt}' | base64 -d)
EXPECTED_CA_CERTIFICATE=$(oc get secret ${ROUTE_NAME}-custom-route -n $NAMESPACE -o jsonpath='{.data.ca\.crt}' | base64 -d)

TLS_DATA=$(oc get route ${ROUTE_NAME}-public -n $NAMESPACE -o jsonpath='{.spec.tls}')

# Extract certificates from the route
ACTUAL_CERTIFICATE=$(echo "$TLS_DATA" | jq -r '.certificate')
ACTUAL_CA_CERTIFICATE=$(echo "$TLS_DATA" | jq -r '.caCertificate')

TRIMMED_EXPECTED_CERTIFICATE=$(echo "$EXPECTED_CERTIFICATE" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
TRIMMED_ACTUAL_CERTIFICATE=$(echo "$ACTUAL_CERTIFICATE" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

TRIMMED_EXPECTED_CA_CERTIFICATE=$(echo "$EXPECTED_CA_CERTIFICATE" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')
TRIMMED_ACTUAL_CA_CERTIFICATE=$(echo "$ACTUAL_CA_CERTIFICATE" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')

if [[ "$TRIMMED_EXPECTED_CERTIFICATE" != "$TRIMMED_ACTUAL_CERTIFICATE" ]]; then
    echo "Certificate does not match for route $ROUTE_NAME in namespace $NAMESPACE."
    exit 1
fi

if [[ "$TRIMMED_EXPECTED_CA_CERTIFICATE" != "$TRIMMED_ACTUAL_CA_CERTIFICATE" ]]; then
    echo "CA Certificate does not match for route $ROUTE_NAME in namespace $NAMESPACE."
    exit 1
fi

echo "TLS data matches for route $ROUTE_NAME in namespace $NAMESPACE."
