#!/bin/bash
set -x

OSCTLPLANE=${1:-}
SERVICE_LIST=${2:-}
SERVICE_ENABLED=${3:-true}

if [ -z "$OSCTLPLANE" ]; then
    echo "ERROR: OpenStackControlPlane name (arg 1) is required"
    exit 1
fi

SERVICE_PATCH='['

IFS=',' read -ra array <<< "$SERVICE_LIST"
for i in "${!array[@]}"; do
    if [ "$i" -gt 0 ]; then
        SERVICE_PATCH+=','
    fi
    SERVICE_PATCH+='{"op":"replace","path":"/spec/'"${array[$i]}"'/enabled",'
    SERVICE_PATCH+='"value":'"${SERVICE_ENABLED}"'}'
done

SERVICE_PATCH+=']'

if [ "$SERVICE_PATCH" != '[]' ]; then
    oc patch openstackcontrolplane "${OSCTLPLANE}" -n "${NAMESPACE}" --type=json -p="${SERVICE_PATCH}"
fi
