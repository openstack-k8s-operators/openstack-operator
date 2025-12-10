#!/bin/bash

NAMESPACE=${NAMESPACE}

declare -A services_secrets=(
    ["ceilometer-internal"]="cert-ceilometer-internal-svc"
    ["ovsdbserver-nb-0"]="cert-ovndbcluster-nb-ovndbs"
    ["ovsdbserver-sb-0"]="cert-ovndbcluster-sb-ovndbs"
    ["rabbitmq"]="cert-rabbitmq-svc"
    ["rabbitmq-cell1"]="cert-rabbitmq-cell1-svc"
)

declare -A database_secrets=(
    ["openstack"]="cert-galera-openstack-svc"
    ["openstack-cell1"]="cert-galera-openstack-cell1-svc"
)

mismatched_services=()

# Gather the ClusterIP and ports for general services
for service in "${!services_secrets[@]}"; do
    secret="${services_secrets[$service]}"

    service_info=$(oc get service "$service" -n "$NAMESPACE" -o jsonpath="{.spec.clusterIP} {.spec.ports[*].port}")
    cluster_ip=$(echo "$service_info" | awk '{print $1}')
    ports=$(echo "$service_info" | cut -d' ' -f2-)

    echo "Checking service: $service (ClusterIP: $cluster_ip, Ports: $ports)"

    # Fetch the certificate from the secret and decode it
    secret_cert=$(oc get secret "$secret" -n "$NAMESPACE" -o jsonpath="{.data['tls\.crt']}" | base64 --decode 2>&1)
    if [[ -z "$secret_cert" ]]; then
        echo "Error retrieving or decoding certificate from secret $secret for service $service."
        continue
    fi

    for port in $ports; do
        echo "Connecting to $service on port $port..."

        # Use different secret for metrics port 1981
        current_secret="$secret"
        if [[ "$port" == "1981" && ("$service" == "ovsdbserver-nb-0" || "$service" == "ovsdbserver-sb-0") ]]; then
            current_secret="cert-ovn-metrics"
            # Fetch the certificate from the metrics secret
            current_secret_cert=$(oc get secret "$current_secret" -n "$NAMESPACE" -o jsonpath="{.data['tls\.crt']}" | base64 --decode 2>&1)
        else
            current_secret_cert="$secret_cert"
        fi

        if [[ -z "$current_secret_cert" ]]; then
            echo "Error retrieving or decoding certificate from secret $current_secret for service $service port $port."
            continue
        fi

        # Captures the certificate section from the openssl output
        pod_cert=$(oc rsh -n "$NAMESPACE" openstackclient openssl s_client -connect "$cluster_ip:$port" -servername "$cluster_ip" </dev/null 2>/dev/null | sed -ne '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p')

        if [[ -z "$pod_cert" ]]; then
            echo "Error retrieving certificate from $service at $cluster_ip:$port in namespace $NAMESPACE."
            continue
        fi

        if [[ "$pod_cert" == "$current_secret_cert" ]]; then
            echo "Certificates for $service on port $port match the secret."
        else
            echo "Certificates for $service on port $port DO NOT match the secret."
            mismatched_services+=("$service on port $port")
        fi
    done
done

# Gather the ClusterIP and ports for databases
for database in "${!database_secrets[@]}"; do
    secret="${database_secrets[$database]}"

    database_info=$(oc get service "$database" -n "$NAMESPACE" -o jsonpath="{.spec.clusterIP} {.spec.ports[*].port}")
    cluster_ip=$(echo "$database_info" | awk '{print $1}')
    ports=$(echo "$database_info" | cut -d' ' -f2-)

    echo "Checking database: $database (ClusterIP: $cluster_ip, Ports: $ports)"

    # Fetch the certificate from the secret and decode it
    secret_cert=$(oc get secret "$secret" -n "$NAMESPACE" -o jsonpath="{.data['tls\.crt']}" | base64 --decode 2>&1)
    if [[ -z "$secret_cert" ]]; then
        echo "Error retrieving or decoding certificate from secret $secret for database $database."
        continue
    fi

    for port in $ports; do
        echo "Connecting to $database on port $port..."

        pod_cert=$(oc rsh -n "$NAMESPACE" openstackclient openssl s_client -starttls mysql -connect "$cluster_ip:$port" </dev/null 2>/dev/null | sed -ne '/-----BEGIN CERTIFICATE-----/,/-----END CERTIFICATE-----/p')

        if [[ -z "$pod_cert" ]]; then
            echo "Error retrieving certificate from $database at $cluster_ip:$port."
            continue
        fi

        if [[ "$pod_cert" == "$secret_cert" ]]; then
            echo "Certificates for $database on port $port match the secret."
        else
            echo "Certificates for $database on port $port DO NOT match the secret."
            mismatched_services+=("$database on port $port")
        fi
    done
done

if [[ ${#mismatched_services[@]} -ne 0 ]]; then
    echo "The following services had certificate mismatches:"
    for mismatch in "${mismatched_services[@]}"; do
        echo " - $mismatch"
    done
    exit 1
fi
