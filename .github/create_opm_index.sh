#!/bin/bash
set -e

echo "Creating operator index image"
echo "${REGISTRY}"
echo "${GITHUB_SHA}"
echo "${INDEX_IMAGE}"
echo "${INDEX_IMAGE_TAG}"
echo "${BUNDLE_IMAGE}"

echo "opm index add --bundles ${REGISTRY}/${BUNDLE_IMAGE}:${GITHUB_SHA} --tag ${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA} -u podman --pull-tool podman"
opm index add --bundles "${REGISTRY}/${BUNDLE_IMAGE}:${GITHUB_SHA}" --tag "${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA}" -u podman --pull-tool podman

echo "podman tag ${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA} ${REGISTRY}/${INDEX_IMAGE}:${INDEX_IMAGE_TAG}"
podman tag "${REGISTRY}/${INDEX_IMAGE}:${GITHUB_SHA}" "${REGISTRY}/${INDEX_IMAGE}:${INDEX_IMAGE_TAG}"
