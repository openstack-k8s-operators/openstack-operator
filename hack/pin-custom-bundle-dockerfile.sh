#!/bin/bash
set -ex

IMAGENAMESPACE=${IMAGENAMESPACE:-"openstack-k8s-operators"}
IMAGEREGISTRY=${IMAGEREGISTRY:-"quay.io"}


cp custom-bundle.Dockerfile custom-bundle.Dockerfile.pinned

#loop over each openstack-k8s-operators go.mod entry
for MOD_PATH in $(go list -m -json all | jq -r '. | select(.Path | contains("openstack")) | .Replace // . |.Path' | grep -v apis | grep -v openstack-operator | grep -v lib-common); do
  MOD_VERSION=$(go list -m -json all | jq -r ". | select(.Path | contains(\"openstack\")) | .Replace // . | select( .Path == \"$MOD_PATH\") | .Version")

  BASE=$(echo $MOD_PATH | sed -e 's|github.com/.*/\(.*\)-operator/.*|\1|')

  REF=$(echo $MOD_VERSION | sed -e 's|v0.0.0-[0-9]*-\(.*\)$|\1|')
  GITHUB_USER=$(echo $MOD_PATH | sed -e 's|github.com/\(.*\)/.*-operator/.*$|\1|')
  REPO_CURL_URL="https://quay.io/api/v1/repository/openstack-k8s-operators"
  REPO_URL="quay.io/openstack-k8s-operators"
  if [[ "$GITHUB_USER" != "openstack-k8s-operators" ]]; then
      if [[ "$IMAGENAMESPACE" != "openstack-k8s-operators" || "${IMAGEREGISTRY}" != "quay.io" ]]; then
          REPO_CURL_URL="https://${IMAGEREGISTRY}/api/v1/repository/${IMAGENAMESPACE}"
          REPO_URL="${IMAGEREGISTRY}/${IMAGENAMESPACE}"
      else
          REPO_CURL_URL="https://quay.io/api/v1/repository/${GITHUB_USER}"
          REPO_URL="quay.io/${GITHUB_USER}"
      fi
  fi

  SHA=$(curl -s ${REPO_CURL_URL}/$BASE-operator-bundle/tag/ | jq -r .tags[].name | sort -u | grep $REF)
  sed -i custom-bundle.Dockerfile.pinned -e "s|quay.io/openstack-k8s-operators/${BASE}-operator-bundle.*|${REPO_URL}/${BASE}-operator-bundle:$SHA|"
done
