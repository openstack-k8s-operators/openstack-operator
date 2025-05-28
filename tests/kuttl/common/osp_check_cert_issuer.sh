#!/bin/bash

set -x

EXPECTED_ISSUER="$1"
ENDPOINT_TYPE="$2"
ISSUER_MISMATCHES=""
ALL_MATCHED=1

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
    http_status=$(curl -s -o /dev/null -w "%{http_code}" "$endpoint_url")

    if [[ "$http_status" -ge 200 && "$http_status" -lt 400 ]]; then
        return 0
    else
        return 1
    fi
}

keystone_url=$(openstack endpoint list -c URL -f value | grep 'keystone-public')
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
for url in $(openstack endpoint list -c URL -f value | grep "$endpoint_filter"); do
    host_port=$(extract_host_port "$url")

    echo "Checking $host_port ..."
    if [[ "$ENDPOINT_TYPE" == "public" ]]; then
        ISSUER=$(echo | openssl s_client -connect "$host_port" 2>/dev/null | openssl x509 -noout -issuer | sed -n 's/^.*CN=\([^,]*\).*$/\1/p' | sed 's/ //g')
    else
        ISSUER=$(openssl s_client -connect $host_port </dev/null 2>/dev/null | openssl x509 -issuer -noout -in /dev/stdin | sed 's/ //g')
    fi

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
