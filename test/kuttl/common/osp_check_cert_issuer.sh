#!/bin/bash

set -x

EXPECTED_ISSUER="$1"
ENDPOINT_TYPE="$2"
ISSUER_MISMATCHES=""
ALL_MATCHED=1
KEYSTONE_NAMESPACE="$(cat /var/run/secrets/kubernetes.io/serviceaccount/namespace)"
KEYSTONE_AUTH_URL="https://keystone-internal.${KEYSTONE_NAMESPACE}.svc:5000/v3"
KEYSTONE_SOURCE_CONFIG="${OS_CLIENT_CONFIG_FILE:-/home/cloud-admin/.config/openstack/clouds.yaml}"
KEYSTONE_SECURE_CONFIG="${OS_CLIENT_SECURE_FILE:-/home/cloud-admin/.config/openstack/secure.yaml}"
KEYSTONE_CLIENT_CONFIG="$(mktemp)"
cp "$KEYSTONE_SOURCE_CONFIG" "$KEYSTONE_CLIENT_CONFIG"
sed -i -E "s|(^[[:space:]]*auth_url:[[:space:]]*).*|\1${KEYSTONE_AUTH_URL}|" "$KEYSTONE_CLIENT_CONFIG"
trap 'rm -f "$KEYSTONE_CLIENT_CONFIG"' EXIT

openstack_with_internal_auth() {
    OS_CLOUD=default \
    OS_CLIENT_CONFIG_FILE="$KEYSTONE_CLIENT_CONFIG" \
    OS_CLIENT_SECURE_FILE="$KEYSTONE_SECURE_CONFIG" \
    OS_IDENTITY_API_VERSION=3 \
    OS_INTERFACE=internal \
    OS_ENDPOINT_TYPE=internal \
    OS_AUTH_URL="$KEYSTONE_AUTH_URL" \
        openstack "$@"
}

function extract_host_port {
    local endpoint_url=$1
    local host_port

    if [[ "$ENDPOINT_TYPE" == "public" ]]; then
        # Extract the hostname and port for public endpoints
        host_port=$(echo "$endpoint_url" | sed -E 's|^[^:/]+://([^:/]+).*|\1|')
    else
        # Extract the hostname and port for internal endpoints, keeping the port if specified
        host_port=$(echo "$endpoint_url" | sed -E 's|^[^:/]+://([^:/]+(:[0-9]+)?).*|\1|')
    fi

    # If no port is specified, add :443
    if [[ ! "$host_port" =~ :[0-9]+$ ]]; then
        host_port="${host_port}:443"
    fi

    echo "$host_port"
}

function check_keystone_endpoint {
    local endpoint_url=$1

    echo "Checking Keystone endpoint $endpoint_url ..."
    # Route hosts are not reliably resolvable from CRC client pods. Keep the
    # route hostname for Host/SNI, but connect through the in-cluster router.
    local host_port
    host_port=$(extract_host_port "$endpoint_url")
    local host="${host_port%:*}"
    local port="${host_port##*:}"
    http_status=$(curl --connect-to "${host}:${port}:router-internal-default.openshift-ingress.svc.cluster.local.:${port}" \
        -s -o /dev/null -w "%{http_code}" "$endpoint_url")

    if [[ "$http_status" -ge 200 && "$http_status" -lt 400 ]]; then
        return 0
    else
        return 1
    fi
}

get_issuer_cn() {
    local host_port="$1"
    local output

    if [[ "$ENDPOINT_TYPE" == "public" ]]; then
        local host="${host_port%:*}"
        local port="${host_port##*:}"
        output=$(openssl s_client \
            -connect "router-internal-default.openshift-ingress.svc.cluster.local.:${port}" \
            -servername "$host" </dev/null 2>/dev/null |
            openssl x509 -noout -issuer 2>/dev/null)
    else
        output=$(openssl s_client -connect "$host_port" </dev/null 2>/dev/null |
            openssl x509 -noout -issuer 2>/dev/null)
    fi

    echo "$output" | sed -n 's/^.*CN[[:space:]]*=[[:space:]]*\([^,]*\).*$/\1/p'
}

keystone_url=$(openstack_with_internal_auth endpoint list -c URL -f value | grep 'keystone-public')
keystone_host_port=$(extract_host_port "$keystone_url")

if ! check_keystone_endpoint "$keystone_url"; then
    echo "Failed to connect to Keystone public endpoint."
    exit 1
fi

# Determine endpoint filter
if [[ "$ENDPOINT_TYPE" == "public" ]]; then
    endpoint_filter='public'
else
    endpoint_filter='svc'
fi

# Check endpoints for the expected issuer
for url in $(openstack_with_internal_auth endpoint list -c URL -f value | grep "$endpoint_filter"); do
    host_port=$(extract_host_port "$url")

    echo "Checking $host_port ..."
    for retry in {1..5}; do
        echo "Retrying $retry on getting issuer $host_port..."
        ISSUER=$(get_issuer_cn "$host_port")
        if [[ -n "$ISSUER" ]]; then
            break
        fi
        sleep 20
    done

    if [[ "$ISSUER" != "$EXPECTED_ISSUER" ]]; then
        ISSUER_MISMATCHES+="$host_port issued by $ISSUER, expected $EXPECTED_ISSUER\n"
        ALL_MATCHED=0
    fi
done

if [ "$ALL_MATCHED" -eq 1 ]; then
    echo "All certificates match the custom issuer $EXPECTED_ISSUER"
    exit 0
else
    echo -e "Mismatched issuers found:\n$ISSUER_MISMATCHES"
    exit 1
fi
