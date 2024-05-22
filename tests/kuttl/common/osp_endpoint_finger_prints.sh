#!/bin/bash

set -x

for url in $(openstack endpoint list -c URL -f value | awk -F/ '{print $3}'); do
  # Extract the hostname and port
  host_port=$(echo "$url" | sed -E 's|^[^:/]+://([^:/]+)(:([0-9]+))?.*|\1:\3|')

  # If no port is specified, add :443
  if [[ ! "$host_port" =~ :[0-9]+$ ]]; then
    host_port="${host_port}:443"
  fi

  echo -n "$host_port - "; openssl s_client -connect $host_port < /dev/null 2>/dev/null | openssl x509 -fingerprint -noout -in /dev/stdin
done
