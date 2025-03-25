#!/usr/bin/env bash
set -euo pipefail

# Script to verify Application Credential IDs are correctly configured in OpenStack service pods
# Usage: osp_check_appcred_id.sh <NAMESPACE> <SERVICE|all>

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 <NAMESPACE> <SERVICE|all>" >&2
    echo "  SERVICE: barbican (or 'all' for supported services)" >&2
    exit 1
fi

NAMESPACE="$1"
REQUESTED_SERVICE="$2"
DEBUG=${DEBUG:-0}

# Wait for control plane to be fully ready before checking pods, we have to add this manual sleep,
# because controlplane is switching to ready state before service pods catch up the AC change
echo "Waiting 60 seconds for OpenStack control plane to be fully ready..."
sleep 60

declare -a FAILED_CHECKS=()

# Service definitions: service_name -> "config_path|resource_type/resource_name:container,resource_type/resource_name:container"
# resource_type can be: deploy (deployment) or sts (statefulset)
declare -A SERVICES=(
    [barbican]="/etc/barbican/barbican.conf.d/00-default.conf|deploy/barbican-api:barbican-api"
    # TODO: Uncomment services here when they support application credentials:
    # [cinder]="/etc/cinder/cinder.conf.d/00-default.conf|sts/cinder-api:cinder-api,sts/cinder-scheduler:cinder-scheduler"
    # [glance]="/etc/glance/glance-api.conf.d/00-default.conf|sts/glance-default-external-api:glance-api,sts/glance-default-internal-api:glance-api"
    # [nova]="/etc/nova/nova.conf|sts/nova-api:nova-api,sts/nova-cell0-conductor:nova-conductor,sts/nova-cell1-conductor:nova-conductor,sts/nova-metadata:nova-metadata,sts/nova-scheduler:nova-scheduler"
    # [neutron]="/etc/neutron/neutron.conf|deploy/neutron:neutron-server"
    # [placement]="/etc/placement/placement.conf|deploy/placement:placement-api"
    # [swift]="/etc/swift/proxy-server.conf|deploy/swift-proxy:swift-proxy,sts/swift-storage:swift-storage"
    # [ceilometer]="/etc/ceilometer/ceilometer.conf|sts/ceilometer:ceilometer-central"
)

RESOURCE_TYPE="keystoneapplicationcredential"

debug() {
    [[ $DEBUG -ge 1 ]] && echo "[DEBUG] $*" >&2
}

error() {
    echo "[ERROR] $*" >&2
}

# Add a failed check to the global list
add_failed_check() {
    local service="$1" resource_spec="$2" container="$3" reason="$4"
    FAILED_CHECKS+=("$service: $resource_spec/$container - $reason")
}

# Convert resource type shorthand to full name
get_resource_type() {
    case "$1" in
        deploy) echo "deployment" ;;
        sts) echo "statefulset" ;;
        *) echo "$1" ;;
    esac
}

# Extract application_credential_id from config file in pod
get_app_cred_id_from_pod() {
    local resource_spec="$1" container="$2" config_path="$3"
    local output

    debug "Executing: oc exec -n $NAMESPACE $resource_spec -c $container -- sh -c \"grep '^[[:space:]]*application_credential_id[[:space:]]*=' '$config_path' | sed 's/^[^=]*=[[:space:]]*//' | sed 's/[[:space:]]*$//' | head -1\""

    if output=$(oc exec -n "$NAMESPACE" "$resource_spec" -c "$container" -- \
        sh -c "grep '^[[:space:]]*application_credential_id[[:space:]]*=' '$config_path' | sed 's/^[^=]*=[[:space:]]*//' | sed 's/[[:space:]]*$//' | head -1" 2>/dev/null); then
        debug "Successfully extracted ID from $resource_spec/$container: $output"
        echo "$output"
        return 0
    fi

    error "Failed to extract application_credential_id from $resource_spec/$container"
    return 1
}

# Check a single service
check_service() {
    local service="$1"
    local service_def="${SERVICES[$service]}"
    local config_path="${service_def%%|*}"
    local targets="${service_def##*|}"

    echo "Checking service: $service"

    # Get expected Application Credential ID from Kubernetes resource
    local cr_name="ac-$service"
    local expected_id
    if ! expected_id=$(oc get "$RESOURCE_TYPE" "$cr_name" -n "$NAMESPACE" -o jsonpath='{.status.acID}' 2>/dev/null); then
        error "Failed to get Application Credential ID from $RESOURCE_TYPE/$cr_name"
        add_failed_check "$service" "$cr_name" "N/A" "Failed to get Application Credential ID from Kubernetes resource"
        return 1
    fi

    if [[ -z "$expected_id" ]]; then
        error "$RESOURCE_TYPE/$cr_name has empty .status.acID"
        add_failed_check "$service" "$cr_name" "N/A" "Empty .status.acID in Kubernetes resource"
        return 1
    fi

    echo "  Expected ID: $expected_id"

    local failed=0

    # Check each resource/container pair
    IFS=',' read -ra TARGET_LIST <<< "$targets"
    for target in "${TARGET_LIST[@]}"; do
        local resource_spec="${target%%:*}"  # e.g., "deploy/barbican-api" or "sts/cinder-api"
        local container="${target##*:}"      # e.g., "barbican-api"

        # Parse resource type and name
        local resource_type_short="${resource_spec%%/*}"  # e.g., "deploy" or "sts"
        local resource_name="${resource_spec##*/}"        # e.g., "barbican-api"
        local resource_type_full=$(get_resource_type "$resource_type_short")

        # Skip if resource doesn't exist
        if ! oc get "$resource_type_full" "$resource_name" -n "$NAMESPACE" >/dev/null 2>&1; then
            echo "  Skipping $resource_type_full/$resource_name (not found)"
            continue
        fi

        local actual_id
        if ! actual_id=$(get_app_cred_id_from_pod "$resource_type_full/$resource_name" "$container" "$config_path"); then
            add_failed_check "$service" "$resource_spec" "$container" "Failed to extract application_credential_id from pod"
            failed=1
            continue
        fi

        if [[ -z "$actual_id" ]]; then
            error "  $resource_spec/$container: application_credential_id not found in $config_path"
            add_failed_check "$service" "$resource_spec" "$container" "application_credential_id not found in config file"
            failed=1
        elif [[ "$actual_id" != "$expected_id" ]]; then
            error "  $resource_spec/$container: ID mismatch (got: $actual_id, expected: $expected_id)"
            add_failed_check "$service" "$resource_spec" "$container" "ID mismatch (got: $actual_id, expected: $expected_id)"
            failed=1
        else
            echo "  $resource_spec/$container: ID matches"
        fi
    done

    return $failed
}

# Print summary of failed checks
print_failed_checks_summary() {
    if [[ ${#FAILED_CHECKS[@]} -eq 0 ]]; then
        return
    fi

    echo
    echo "FAILED APPLICATION CREDENTIAL ID CHECKS:"
    for failure in "${FAILED_CHECKS[@]}"; do
        echo "  $failure"
    done
}

# Main execution
main() {
    local services_to_check=()

    if [[ "$REQUESTED_SERVICE" == "all" ]]; then
        services_to_check=("${!SERVICES[@]}")
    else
        if [[ -z "${SERVICES[$REQUESTED_SERVICE]:-}" ]]; then
            error "Service '$REQUESTED_SERVICE' is not supported"
            echo "Supported services: ${!SERVICES[*]}" >&2
            exit 1
        fi
        services_to_check=("$REQUESTED_SERVICE")
    fi

    local overall_failed=0

    for service in "${services_to_check[@]}"; do
        if ! check_service "$service"; then
            overall_failed=1
        fi
        echo
    done

    # Print summary of failures
    print_failed_checks_summary

    if [[ $overall_failed -eq 0 ]]; then
        echo "All Application Credential ID checks passed"
    else
        echo "Some Application Credential ID checks failed"
    fi

    exit $overall_failed
}

main
