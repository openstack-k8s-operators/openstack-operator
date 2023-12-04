#!/bin/bash

# extract select data from bundles:
#  -env vars from all service operators
#  -dataplane-operator bundle is cached (in order to merge at build time)
set -ex

function extract_bundle {
    local IN_DIR=$1
    local OUT_DIR=$2
    for X in $(file ${IN_DIR}/* | grep gzip | cut -f 1 -d ':'); do
        tar xvf $X -C ${OUT_DIR}/;
    done
}

function extract_csv {
    local IN_DIR=$1
    local OUT_DIR=$2

    for X in $(file ${IN_DIR}/* | grep gzip | cut -f 1 -d ':'); do
        # NOTE(gibi): There might be multiple gzip in the bundle and
        # not all of them hand csv file in it. If none of them has
        # the csv then the build will fail at the csv-merger call
        tar xvf $X -C ${OUT_DIR} --wildcards --no-anchor '**/*clusterserviceversion.yaml' || true
    done
}

OUT_BUNDLE=bundle_extra_data
EXTRACT_DIR=tmp/bundle_extract

mkdir -p "$EXTRACT_DIR"
mkdir -p "$EXTRACT_DIR/csvs"
mkdir -p "$OUT_BUNDLE"

for BUNDLE in $(hack/pin-bundle-images.sh | tr "," " "); do
    skopeo copy "docker://$BUNDLE" dir:${EXTRACT_DIR}/tmp;
    if echo $BUNDLE | grep dataplane-operator &> /dev/null; then
        extract_bundle "${EXTRACT_DIR}/tmp" "${OUT_BUNDLE}/"
    else
        extract_csv "${EXTRACT_DIR}/tmp" "${EXTRACT_DIR}/csvs"
    fi
done

# Extract the ENV vars from all the CSVs
CSV_DIR="${EXTRACT_DIR}/csvs/manifests"
bin/csv-merger \
    --export-env-file=$OUT_BUNDLE/env-vars.yaml \
    --mariadb-csv=$CSV_DIR/mariadb-operator.clusterserviceversion.yaml \
    --infra-csv=$CSV_DIR/infra-operator.clusterserviceversion.yaml \
    --keystone-csv=$CSV_DIR/keystone-operator.clusterserviceversion.yaml \
    --placement-csv=$CSV_DIR/placement-operator.clusterserviceversion.yaml \
    --ovn-csv=$CSV_DIR/ovn-operator.clusterserviceversion.yaml \
    --neutron-csv=$CSV_DIR/neutron-operator.clusterserviceversion.yaml \
    --ansibleee-csv=$CSV_DIR/openstack-ansibleee-operator.clusterserviceversion.yaml \
    --nova-csv=$CSV_DIR/nova-operator.clusterserviceversion.yaml \
    --heat-csv=$CSV_DIR/heat-operator.clusterserviceversion.yaml \
    --ironic-csv=$CSV_DIR/ironic-operator.clusterserviceversion.yaml \
    --baremetal-csv=$CSV_DIR/openstack-baremetal-operator.clusterserviceversion.yaml \
    --horizon-csv=$CSV_DIR/horizon-operator.clusterserviceversion.yaml \
    --telemetry-csv=$CSV_DIR/telemetry-operator.clusterserviceversion.yaml \
    --glance-csv=$CSV_DIR/glance-operator.clusterserviceversion.yaml \
    --cinder-csv=$CSV_DIR/cinder-operator.clusterserviceversion.yaml \
    --manila-csv=$CSV_DIR/manila-operator.clusterserviceversion.yaml \
    --swift-csv=$CSV_DIR/swift-operator.clusterserviceversion.yaml \
    --octavia-csv=$CSV_DIR/octavia-operator.clusterserviceversion.yaml \
    --designate-csv=$CSV_DIR/designate-operator.clusterserviceversion.yaml \
    --barbican-csv=$CSV_DIR/barbican-operator.clusterserviceversion.yaml \
    --base-csv=config/manifests/bases/openstack-operator.clusterserviceversion.yaml | tee "$EXTRACT_DIR/out.yaml"

# cleanup our temporary dir used for extraction
rm -Rf "$EXTRACT_DIR"

# we only keep manifests from extracted (merged) bundles
rm -Rf "$OUT_BUNDLE/metadata"
rm -Rf "$OUT_BUNDLE/tests"
