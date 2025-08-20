#!/bin/bash

# Script to add replace directives to go.mod for OpenStack operator dependencies
# Usage: add-replace-directive.sh <fork_name> <branch_name>
# Example: add-replace-directive.sh dprince/keystone-operator drop_kube_rbac_proxy

set -e

if [ $# -ne 2 ]; then
    echo "Usage: $0 <fork_name> <branch_name>"
    echo "Example: $0 dprince/keystone-operator drop_kube_rbac_proxy"
    exit 1
fi

FORK_NAME="$1"
BRANCH_NAME="$2"

# Convert fork name to official openstack-k8s-operators equivalent
# e.g., dprince/keystone-operator -> keystone-operator
REPO_NAME=$(echo "$FORK_NAME" | sed 's|.*/||')
OFFICIAL_REPO="github.com/openstack-k8s-operators/$REPO_NAME"

echo "Converting fork $FORK_NAME to official repo: $OFFICIAL_REPO"

# Find all OpenStack dependencies that match the repo pattern
echo "Finding matching OpenStack dependencies..."
MATCHING_DEPS=$(go list -mod=readonly -m -json all | jq -r --arg repo "$REPO_NAME" '. | select(.Path | contains("openstack")) | .Replace // . | .Path | select(. | contains($repo))')

if [ -z "$MATCHING_DEPS" ]; then
    echo "No matching dependencies found for $REPO_NAME"
    exit 1
fi

echo "Found matching dependencies:"
echo "$MATCHING_DEPS"

# Get the latest commit hash for the branch
echo "Fetching latest commit for branch $BRANCH_NAME from fork github.com/$FORK_NAME..."
COMMIT_HASH=$(git ls-remote "https://github.com/$FORK_NAME.git" "refs/heads/$BRANCH_NAME" | cut -f1)

if [ -z "$COMMIT_HASH" ]; then
    echo "Error: Could not find branch $BRANCH_NAME in fork github.com/$FORK_NAME"
    exit 1
fi

echo "Latest commit hash: $COMMIT_HASH"

# Get commit timestamp using shallow clone
echo "Fetching commit timestamp..."
TEMP_DIR=$(mktemp -d)
pushd "$TEMP_DIR"
git clone --depth=1 --branch "$BRANCH_NAME" "https://github.com/$FORK_NAME.git" repo >/dev/null 2>&1
if [ $? -ne 0 ]; then
    rm -rf "$TEMP_DIR"
    echo "Error: Could not clone branch $BRANCH_NAME from github.com/$FORK_NAME"
    exit 1
fi

pushd repo
COMMIT_TIMESTAMP=$(git log -1 --format="%ct")
popd
popd
rm -rf "$TEMP_DIR"

# Convert timestamp to the format needed for pseudoversion (YYYYMMDDHHMMSS)
FORMATTED_TIMESTAMP=$(date -u -d "@$COMMIT_TIMESTAMP" +%Y%m%d%H%M%S)

# Create pseudoversion in format: v0.0.0-YYYYMMDDHHMMSS-abcdefabcdef
PSEUDOVERSION="v0.0.0-${FORMATTED_TIMESTAMP}-${COMMIT_HASH:0:12}"

echo "Generated pseudoversion: $PSEUDOVERSION"

# Add replace directives for each matching dependency
echo "Adding replace directives to go.mod..."
while IFS= read -r dep; do
    if [ -n "$dep" ]; then
        echo "Adding replace directive for: $dep"
        # Check if replace directive already exists
        if grep -q "^replace $dep =>" go.mod; then
            echo "Replace directive already exists for $dep, skipping..."
        else
            # Extract the suffix from the dependency (e.g., /api from github.com/openstack-k8s-operators/keystone-operator/api)
            SUFFIX=$(echo "$dep" | sed "s|github.com/openstack-k8s-operators/$REPO_NAME||")
            REPLACEMENT_TARGET="github.com/$FORK_NAME$SUFFIX"
            
            go mod edit -replace="$dep=$REPLACEMENT_TARGET@$PSEUDOVERSION"
            pushd apis
            go mod edit -replace="$dep=$REPLACEMENT_TARGET@$PSEUDOVERSION"
            popd
            echo "Added: replace $dep => $REPLACEMENT_TARGET@$PSEUDOVERSION"
        fi
    fi
done <<< "$MATCHING_DEPS"

echo "Successfully added replace directives to go.mod"
echo "Run 'go mod tidy' to update go.sum"
