#!/bin/bash
set -euxo pipefail

CHECKER=$INSTALL_DIR/crd-schema-checker

TMP_DIR=$(mktemp -d)

function cleanup {
    rm -rf "$TMP_DIR"
}

trap cleanup EXIT


for crd in config/crd/bases/*.yaml; do
    mkdir -p "$(dirname "$TMP_DIR/$crd")"
    if git show "$BASE_REF:$crd" > "$TMP_DIR/$crd"; then
        $CHECKER check-manifests \
            --existing-crd-filename="$TMP_DIR/$crd" \
            --new-crd-filename="$crd"
    fi
done
