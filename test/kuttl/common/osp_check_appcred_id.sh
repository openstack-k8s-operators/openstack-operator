#!/usr/bin/env bash
set -euo pipefail

# Script to verify Application Credential IDs and AC secrets are correctly configured in OpenStack service pods
# Usage: osp_check_appcred_id.sh <NAMESPACE> <SERVICE|all>

if [[ $# -lt 2 ]]; then
    echo "Usage: $0 <NAMESPACE> <SERVICE|all>" >&2
    echo "  SERVICE: barbican, glance, swift, cinder, manila (or 'all' for supported services)" >&2
    exit 1
fi

NAMESPACE="$1"
REQUESTED_SERVICE="$2"
DEBUG=${DEBUG:-0}

declare -a FAILED_CHECKS=()

# Service definitions: service_name -> "config_path|resource_type/resource_name:container,resource_type/resource_name:container"
# resource_type can be: deploy (deployment) or sts (statefulset)
declare -A SERVICES=(
    [barbican]="/etc/barbican/barbican.conf.d/00-default.conf|deploy/barbican-api:barbican-api"
    [cinder]="/etc/cinder/cinder.conf.d/00-default.conf|sts/cinder-api:cinder-api,sts/cinder-scheduler:cinder-scheduler"
    [glance]="/etc/glance/glance-api.conf.d/00-default.conf|sts/glance-default-external-api:glance-api,sts/glance-default-internal-api:glance-api"
    [swift]="/etc/swift/proxy-server.conf.d/00-proxy-server.conf|deploy/swift-proxy:proxy-server"
    [manila]="/etc/manila/manila.conf.d/00-config.conf|sts/manila-api:manila-api,sts/manila-scheduler:manila-scheduler"
    # TODO: Add remaining services when they support application credentials in their operators:
    # [nova]="/etc/nova/nova.conf|sts/nova-api:nova-api,sts/nova-cell0-conductor:nova-conductor,sts/nova-cell1-conductor:nova-conductor,sts/nova-metadata:nova-metadata,sts/nova-scheduler:nova-scheduler"
    [neutron]="/etc/neutron/neutron.conf.d/01-default.conf|deploy/neutron:neutron-server"
    [placement]="/etc/placement/placement.conf|deploy/placement:placement-api"
    # [ceilometer]="/etc/ceilometer/ceilometer.conf|sts/ceilometer:ceilometer-central"
    # [aodh]="/etc/aodh/aodh.conf|sts/aodh-api:aodh-api"
    # [heat]="/etc/heat/heat.conf|sts/heat-api:heat-api"
    # [ironic]="/etc/ironic/ironic.conf|sts/ironic-api:ironic-api"
    [octavia]="/etc/octavia/octavia.conf|sts/octavia-api:octavia-api"
    # [designate]="/etc/designate/designate.conf|sts/designate-api:designate-api"
    # [watcher]="/etc/watcher/watcher.conf|sts/watcher-api:watcher-api"
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

# Extract application credential field from config file in pod
get_app_cred_field_from_pod() {
    local resource_spec="$1" container="$2" config_path="$3" field_name="$4"
    local output

    debug "Executing: oc exec -n $NAMESPACE $resource_spec -c $container -- sh -c \"grep '^[[:space:]]*${field_name}[[:space:]]*=' '$config_path' | sed 's/^[^=]*=[[:space:]]*//' | sed 's/[[:space:]]*$//' | head -1\""

    if output=$(oc exec -n "$NAMESPACE" "$resource_spec" -c "$container" -- \
        sh -c "grep '^[[:space:]]*${field_name}[[:space:]]*=' '$config_path' | sed 's/^[^=]*=[[:space:]]*//' | sed 's/[[:space:]]*$//' | head -1" 2>/dev/null); then
        debug "Successfully extracted $field_name from $resource_spec/$container: $output"
        echo "$output"
        return 0
    fi

    error "Failed to extract $field_name from $resource_spec/$container"
    return 1
}

# Check a single service
check_service() {
    local service="$1"
    local service_def="${SERVICES[$service]}"
    local config_path="${service_def%%|*}"
    local targets="${service_def##*|}"

    echo "Checking service: $service"

    # Get expected Application Credential data from Kubernetes resources
    local cr_name="ac-$service"
    local expected_id expected_secret

    # Get AC ID from the KeystoneApplicationCredential status
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

    # Get AC Secret from the associated secret (base64 decoded)
    local secret_name
    if ! secret_name=$(oc get "$RESOURCE_TYPE" "$cr_name" -n "$NAMESPACE" -o jsonpath='{.status.secretName}' 2>/dev/null); then
        error "Failed to get secret name from $RESOURCE_TYPE/$cr_name"
        add_failed_check "$service" "$cr_name" "N/A" "Failed to get secret name from Kubernetes resource"
        return 1
    fi

    if [[ -z "$secret_name" ]]; then
        error "$RESOURCE_TYPE/$cr_name has empty .status.secretName"
        add_failed_check "$service" "$cr_name" "N/A" "Empty .status.secretName in Kubernetes resource"
        return 1
    fi

    # Get and decode the AC_SECRET from the Kubernetes secret
    if ! expected_secret=$(oc get secret "$secret_name" -n "$NAMESPACE" -o jsonpath='{.data.AC_SECRET}' 2>/dev/null | base64 -d); then
        error "Failed to get AC_SECRET from secret/$secret_name"
        add_failed_check "$service" "$secret_name" "N/A" "Failed to get AC_SECRET from Kubernetes secret"
        return 1
    fi

    if [[ -z "$expected_secret" ]]; then
        error "secret/$secret_name has empty AC_SECRET"
        add_failed_check "$service" "$secret_name" "N/A" "Empty AC_SECRET in Kubernetes secret"
        return 1
    fi

    echo "  Expected ID: $expected_id"
    echo "  Expected Secret: ${expected_secret:0:20}..."

    local failed=0

    # Check each resource/container pair
    IFS=',' read -ra TARGET_LIST <<< "$targets"
    for target in "${TARGET_LIST[@]}"; do
        local resource_spec="${target%%:*}"  # e.g., "deploy/swift-proxy"
        local container="${target##*:}"      # e.g., "proxy-server"

        # Parse resource type and name
        local resource_type_short="${resource_spec%%/*}"  # e.g., "deploy"
        local resource_name="${resource_spec##*/}"        # e.g., "swift-proxy"
        local resource_type_full=$(get_resource_type "$resource_type_short")

        # Skip if resource doesn't exist
        if ! oc get "$resource_type_full" "$resource_name" -n "$NAMESPACE" >/dev/null 2>&1; then
            echo "  Skipping $resource_type_full/$resource_name (not found)"
            continue
        fi

        # Check Application Credential ID
        local actual_id
        if ! actual_id=$(get_app_cred_field_from_pod "$resource_type_full/$resource_name" "$container" "$config_path" "application_credential_id"); then
            add_failed_check "$service" "$resource_spec" "$container" "Failed to extract application_credential_id from pod"
            failed=1
        elif [[ -z "$actual_id" ]]; then
            error "  $resource_spec/$container: application_credential_id not found in $config_path"
            add_failed_check "$service" "$resource_spec" "$container" "application_credential_id not found in config file"
            failed=1
        elif [[ "$actual_id" != "$expected_id" ]]; then
            echo "  $resource_spec/$container: Found ID: $actual_id"
            error "  $resource_spec/$container: ID mismatch (got: $actual_id, expected: $expected_id)"
            add_failed_check "$service" "$resource_spec" "$container" "ID mismatch (got: $actual_id, expected: $expected_id)"
            failed=1
        else
            echo "  $resource_spec/$container: Found ID: $actual_id"
            echo "  $resource_spec/$container: ID matches"
        fi

        # Check Application Credential Secret
        local actual_secret
        if ! actual_secret=$(get_app_cred_field_from_pod "$resource_type_full/$resource_name" "$container" "$config_path" "application_credential_secret"); then
            add_failed_check "$service" "$resource_spec" "$container" "Failed to extract application_credential_secret from pod"
            failed=1
        elif [[ -z "$actual_secret" ]]; then
            error "  $resource_spec/$container: application_credential_secret not found in $config_path"
            add_failed_check "$service" "$resource_spec" "$container" "application_credential_secret not found in config file"
            failed=1
        elif [[ "$actual_secret" != "$expected_secret" ]]; then
            echo "  $resource_spec/$container: Found Secret: ${actual_secret:0:20}..."
            error "  $resource_spec/$container: Secret mismatch (got: ${actual_secret:0:20}..., expected: ${expected_secret:0:20}...)"
            add_failed_check "$service" "$resource_spec" "$container" "Secret mismatch"
            failed=1
        else
            echo "  $resource_spec/$container: Found Secret: ${actual_secret:0:20}..."
            echo "  $resource_spec/$container: Secret matches"
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
    echo "FAILED APPLICATION CREDENTIAL CHECKS:"
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
        echo "All Application Credential checks passed"
    else
        echo "Some Application Credential checks failed"
    fi

    exit $overall_failed
}

main
