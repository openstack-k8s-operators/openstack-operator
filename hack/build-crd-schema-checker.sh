#!/bin/bash
set -euxo pipefail

if [ -f "$INSTALL_DIR/crd-schema-checker" ]; then
    exit 0
fi

mkdir -p "$INSTALL_DIR/git-tmp"
git clone https://github.com/openshift/crd-schema-checker.git \
    -b "$CRD_SCHEMA_CHECKER_VERSION" "$INSTALL_DIR/git-tmp"
pushd "$INSTALL_DIR/git-tmp"
GOWORK=off make
cp crd-schema-checker "$INSTALL_DIR/"
popd
rm -rf "$INSTALL_DIR/git-tmp"
