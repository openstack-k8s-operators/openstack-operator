#!/bin/bash
set -e

# This script can be executed in 2 modes. If DOCKERFILE is set then we replace the image locations there with pinned SHA version.
# If no DOCKERFILE is set the script just echo's a list of bundle dependencies to stout as a single common separated line. This
# is suitable for use with OPM catalog/index creation
DOCKERFILE=${DOCKERFILE:-""}
IMAGENAMESPACE=${IMAGENAMESPACE:-"openstack-k8s-operators"}
IMAGEREGISTRY=${IMAGEREGISTRY:-"quay.io"}
IMAGEBASE=${IMAGEBASE:-}
LOCAL_REGISTRY=${LOCAL_REGISTRY:-0}

if [ -n "$DOCKERFILE" ]; then
    cp "$DOCKERFILE" "${DOCKERFILE}.pinned"
    set -ex #in DOCKERFILE mode we like extra logging
fi

#loop over each openstack-k8s-operators go.mod entry
for MOD_PATH in $(go list -mod=readonly -m -json all | jq -r '. | select(.Path | contains("openstack")) | .Replace // . |.Path' | grep -v openstack-operator | grep -v lib-common); do
    if [[ "$MOD_PATH" == "./apis" ]]; then
        continue
    fi
    MOD_VERSION=$(go list -mod=readonly -m -json all | jq -r ". | select(.Path | contains(\"openstack\")) | .Replace // . | select( .Path == \"$MOD_PATH\") | .Version")

    BASE=$(echo $MOD_PATH | sed -e 's|github.com/.*/\(.*\)-operator/.*|\1|')

    GIT_REPO=${MOD_PATH%"/apis"}
    GIT_REPO=${GIT_REPO%"/api"}
    REF=$(echo $MOD_VERSION | sed -e 's|v[0-9]*.[0-9]*.[0-9]*-.*[0-9]*-\(.*\)$|\1|')
    if [[ "$REF" == v* ]]; then
        REF=$(git ls-remote https://${GIT_REPO} | grep ${REF} | awk 'NR==1{print $1}')
    fi
    GITHUB_USER=$(echo $MOD_PATH | sed -e 's|github.com/\(.*\)/.*-operator/.*$|\1|')
    CURL_REGISTRY="quay.io"
    REPO_CURL_URL="https://${CURL_REGISTRY}/api/v1/repository/openstack-k8s-operators"
    REPO_URL="${CURL_REGISTRY}/openstack-k8s-operators"
    if [[ "$GITHUB_USER" != "openstack-k8s-operators" || "$BASE" == *"$IMAGEBASE"* ]]; then
        if [[ "$IMAGENAMESPACE" != "openstack-k8s-operators" || "${IMAGEREGISTRY}" != "quay.io" ]]; then
            REPO_URL="${IMAGEREGISTRY}/${IMAGENAMESPACE}"
            CURL_REGISTRY="${IMAGEREGISTRY}"
            # Quay registry v2 api does not return all the tags that's why keeping v1 for quay and v2
            # for local registry
            if [[ ${LOCAL_REGISTRY} -eq 1 ]]; then
                REPO_CURL_URL="${CURL_REGISTRY}/v2/${IMAGENAMESPACE}"
            else
                REPO_CURL_URL="https://${CURL_REGISTRY}/api/v1/repository/${IMAGENAMESPACE}"
            fi
        else
            REPO_CURL_URL="https://${CURL_REGISTRY}/api/v1/repository/${GITHUB_USER}"
            REPO_URL="${CURL_REGISTRY}/${GITHUB_USER}"
        fi
    fi

    if [[ ${LOCAL_REGISTRY} -eq 1 && ( "$GITHUB_USER" != "openstack-k8s-operators" || "$BASE" == *"$IMAGEBASE"* ) ]]; then
        SHA=$(curl -s ${REPO_CURL_URL}/$BASE-operator-bundle/tags/list | jq -r .tags[] | sort -u | grep $REF)
    elif [[ "${CURL_REGISTRY}" != "quay.io" ]]; then
        # quay.rdoproject.io doesn't support filter_tag_name, so increase limit to 100
        SHA=$(curl -s ${REPO_CURL_URL}/$BASE-operator-bundle/tag/?limit=100 | jq -r .tags[].name | sort -u | grep $REF)
    else
        SHA=$(curl -s ${REPO_CURL_URL}/$BASE-operator-bundle/tag/?onlyActiveTags=true\&filter_tag_name=like:$REF | jq -r .tags[].name)
    fi

    if [ -z "$SHA" ]; then
        echo "Error: SHA is empty, Could not find container with tag $REF in $REPO_CURL_URL"
        exit 1
    fi

    if [ -n "$DOCKERFILE" ]; then
        sed -i "${DOCKERFILE}.pinned" -e "s|quay.io/openstack-k8s-operators/${BASE}-operator-bundle.*|${REPO_URL}/${BASE}-operator-bundle:$SHA|"
    else
        echo -n ",${REPO_URL}/${BASE}-operator-bundle:$SHA"
    fi
done
# append the rabbitmq URL only if we aren't in Dockerfile mode
if [ -z "$DOCKERFILE" ]; then
    echo -n ",quay.io/openstack-k8s-operators/rabbitmq-cluster-operator-bundle@sha256:1501babc66e91c5427d0773d3df82f67569a0bd33d1ebbdf8ffa3959f75d9095"
fi
