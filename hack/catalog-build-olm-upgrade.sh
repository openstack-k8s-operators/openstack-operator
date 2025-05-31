#!/bin/bash
#NOTE: this script is used by the catalog-openstack-operator-upgrades.yaml
set -ex

MAIN_VERSION=${MAIN_VERSION:-"0.3.0"}
FEATURE_RELEASE_VERSION=${FEATURE_RELEASE_VERSION:-"0.2.0"}
FEATURE_RELEASE_BRANCH=${FEATURE_RELEASE_BRANCH:-"18.0-fr2"}
BUNDLE=${BUNDLE:-"quay.io/openstack-k8s-operators/openstack-operator-bundle:latest"}

[ -d "catalog" ] && rm -Rf catalog
[ -e "catalog.Dockerfile" ] && rm catalog.Dockerfile
mkdir catalog

opm generate dockerfile ./catalog -i registry.redhat.io/openshift4/ose-operator-registry-rhel9:v4.18
opm init openstack-operator --default-channel=stable-v1.0 --output yaml > catalog/index.yaml

opm render ${BUNDLE} --output yaml >> catalog/index.yaml
# always default to use the FR release from openstack-k8s-operators
opm render quay.io/openstack-k8s-operators/openstack-operator-bundle:${FEATURE_RELEASE_BRANCH}-latest --output yaml >> catalog/index.yaml

  cat >> catalog/index.yaml << EOF_CAT
---
schema: olm.channel
package: openstack-operator
name: stable-v1.0
entries:
  - name: openstack-operator.v${FEATURE_RELEASE_VERSION}
  - name: openstack-operator.v${MAIN_VERSION}
    replaces: openstack-operator.v${FEATURE_RELEASE_VERSION}
EOF_CAT
opm validate catalog
