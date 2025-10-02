#!/bin/bash
#NOTE: this script is used by the catalog-openstack-operator-upgrades.yaml
set -ex

function toDigestURL {
local URL=$1

DIGEST=$(skopeo inspect --format '{{.Digest}}' docker://$URL)
echo $URL | sed -e "s|:.*|@$DIGEST|"

}

# These variables are mandatory. The script will exit if they are not set.
MAIN_VERSION=${MAIN_VERSION:?"Error: MAIN_VERSION must be set."}
FEATURE_RELEASE_VERSION=${FEATURE_RELEASE_VERSION:?"Error: FEATURE_RELEASE_VERSION must be set."}
FEATURE_RELEASE_BRANCH=${FEATURE_RELEASE_BRANCH:?"Error: FEATURE_RELEASE_BRANCH must be set."}

BUNDLE=${BUNDLE:-"quay.io/openstack-k8s-operators/openstack-operator-bundle:latest"}

[ -d "catalog" ] && rm -Rf catalog
[ -e "catalog.Dockerfile" ] && rm catalog.Dockerfile
mkdir catalog

opm generate dockerfile ./catalog -i registry.redhat.io/openshift4/ose-operator-registry-rhel9:v4.18
opm init openstack-operator --default-channel=alpha --output yaml > catalog/index.yaml

#opm render ${BUNDLE} --output yaml >> catalog/index.yaml
opm render $(toDigestURL $BUNDLE) --output yaml >> catalog/index.yaml
# always default to use the FR release from openstack-k8s-operators
opm render $(toDigestURL quay.io/openstack-k8s-operators/openstack-operator-bundle:${FEATURE_RELEASE_BRANCH}-latest) --output yaml >> catalog/index.yaml

  cat >> catalog/index.yaml << EOF_CAT
---
schema: olm.channel
package: openstack-operator
name: alpha
entries:
  - name: openstack-operator.v${FEATURE_RELEASE_VERSION}
  - name: openstack-operator.v${MAIN_VERSION}
    replaces: openstack-operator.v${FEATURE_RELEASE_VERSION}
EOF_CAT
opm validate catalog
