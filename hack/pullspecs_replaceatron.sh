#!/bin/bash
set -euo pipefail

# Script to update RELATED_IMAGE_ environment variables in ClusterServiceVersion file
# with values from the current bash environment

CSV_FILE="bundle/manifests/openstack-operator.clusterserviceversion.yaml"

# Check if CSV file exists
if [[ ! -f "$CSV_FILE" ]]; then
    echo "Error: ClusterServiceVersion file not found at $CSV_FILE"
    echo "Please run 'make bundle' first to generate the bundle directory"
    exit 1
fi

# Create a backup of the original file
cp "$CSV_FILE" "${CSV_FILE}.backup"

echo "Updating RELATED_IMAGE_ environment variables in $CSV_FILE..."

# Extract all RELATED_IMAGE_ env var names from the CSV file
RELATED_IMAGE_VARS=$(grep -o 'RELATED_IMAGE_[A-Z_]*' "$CSV_FILE" | sort -u)

# Track if any errors occurred
ERRORS=0

# Process each RELATED_IMAGE_ variable
for var_name in $RELATED_IMAGE_VARS; do
    # Check if the environment variable exists in the current bash environment
    if [[ -n "${!var_name:-}" ]]; then
        current_value="${!var_name}"
        echo "Updating $var_name with value: $current_value"

        # Use sed to replace all occurrences of the current value in the CSV file
        # First, we need to get the current value from the CSV file
        current_csv_value=$(grep -A1 "name: $var_name" "$CSV_FILE" | grep "value:" | sed 's/.*value: //' | tr -d '"')

        if [[ -n "$current_csv_value" ]]; then
            # Escape special characters for sed
            escaped_current=$(printf '%s\n' "$current_csv_value" | sed 's/[[\.*^$()+?{|]/\\&/g')
            escaped_new=$(printf '%s\n' "$current_value" | sed 's/[[\.*^$()+?{|]/\\&/g')

            # Replace all occurrences of the current value with the new value
            sed -i "s|$escaped_current|$escaped_new|g" "$CSV_FILE"
        else
            echo "Warning: Could not find current value for $var_name in CSV file"
        fi
    else
        echo "Error: Environment variable $var_name is not set"
        ERRORS=$((ERRORS + 1))
    fi
done

if [[ $ERRORS -gt 0 ]]; then
    echo ""
    echo "Error: $ERRORS environment variable(s) were not found"
    echo "Please set all required RELATED_IMAGE_ environment variables before running this script"
    echo "Restoring original file from backup..."
    mv "${CSV_FILE}.backup" "$CSV_FILE"
    exit 1
else
    echo ""
    echo "Successfully updated all RELATED_IMAGE_ environment variables"
    echo "Backup saved as ${CSV_FILE}.backup"
    rm -f "${CSV_FILE}.backup"
fi
