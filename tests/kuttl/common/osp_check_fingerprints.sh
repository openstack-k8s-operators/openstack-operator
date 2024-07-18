#!/bin/bash
set -x

# Check if all services from before are present in after and have valid fingerprints
while IFS= read -r before; do
    eval $(echo "$before" | awk '{print "service_name="$1" fp_before="$2}')
    fp_after=$(grep -F "$service_name" /tmp/endpoint_fingerprints_after | awk '{ print $2}')

    echo -n "Endpoint $service_name - "

    if [ -z "$fp_after" ]; then
        echo "not found in endpoint_fingerprints_after"
        exit 1
    fi

    if [ "$fp_before" = "$fp_after" ]; then
        echo "ERROR cert not rotated - before: $fp_before - after: $fp_after"
        exit 1
    fi

    echo "OK cert rotated - before: $fp_before - after: $fp_after"
done < /tmp/endpoint_fingerprints_before

exit 0
