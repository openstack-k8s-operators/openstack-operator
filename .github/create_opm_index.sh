#!/bin/bash
set -e

echo "Creating operator index image"
echo "${REGISTRY}"
echo "${GITHUB_SHA}"
echo "${INDEX_IMAGE}"
echo "${INDEX_IMAGE_TAG}"
echo "${BUNDLE_IMAGE}"
echo "${STORAGE_BUNDLE_IMAGE}"

echo "opm index add --bundles "${REGISTRY}/${BUNDLE_IMAGE}:${GITHUB_SHA}","${REGISTRY}/${STORAGE_BUNDLE_IMAGE}:${GITHUB_SHA}","quay.io/openstack-k8s-operators/rabbitmq-cluster-operator-bundle@sha256:9ff91ad3c9ef1797b232fce2f9adf6ede5c3421163bff5b8a2a462c6a2b3a68b" --tag "${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA}" -u podman --pull-tool podman"
opm index add --bundles "${REGISTRY}/${BUNDLE_IMAGE}:${GITHUB_SHA}","${REGISTRY}/${STORAGE_BUNDLE_IMAGE}:${GITHUB_SHA}","quay.io/openstack-k8s-operators/rabbitmq-cluster-operator-bundle@sha256:9ff91ad3c9ef1797b232fce2f9adf6ede5c3421163bff5b8a2a462c6a2b3a68b" --tag "${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA}" -u podman --pull-tool podman

echo "podman tag ${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA} ${REGISTRY}/${INDEX_IMAGE}:${INDEX_IMAGE_TAG}"
podman tag "${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA}" "${REGISTRY}/${INDEX_IMAGE}:${INDEX_IMAGE_TAG}"
