#!/bin/bash
set -euxo pipefail

CHECKER=$INSTALL_DIR/crd-schema-checker

DISABLED_VALIDATORS=NoMaps,NoBools,ListsMustHaveSSATags # TODO: https://issues.redhat.com/browse/OSPRH-12254

CHECKER_ARGS=""
if [[ ${DISABLED_VALIDATORS:+x} ]]; then
    CHECKER_ARGS="$CHECKER_ARGS "
    for check in ${DISABLED_VALIDATORS//,/ }; do
        CHECKER_ARGS+=" --disabled-validators $check"
    done
fi

TMP_DIR=$(mktemp -d)

function cleanup {
    rm -rf "$TMP_DIR"
}

trap cleanup EXIT


for crd in config/crd/bases/*.yaml; do
    mkdir -p "$(dirname "$TMP_DIR/$crd")"
    if git show "$BASE_REF:$crd" > "$TMP_DIR/$crd"; then
        OUTPUT=$($CHECKER check-manifests \
            $CHECKER_ARGS \
            --existing-crd-filename="$TMP_DIR/$crd" \
            --new-crd-filename="$crd" 2>&1) && rc=0 || rc=$?
        if [[ $rc -ne 0 ]] && ! grep -q '^ERROR:' <<< "$OUTPUT"; then
            echo "$OUTPUT"
            exit "$rc"
        fi

        # Filter out NoNewRequiredFields errors for entirely new top-level
        # spec sections. When a new service (e.g. cyborg) is added, all
        # required fields inherited from shared types are flagged, but
        # these are not backwards compatibility concerns since the entire
        # section is new.
        FILTERED=""
        while IFS= read -r line; do
            if [[ "$line" == ERROR:*\"NoNewRequiredFields\"* ]]; then
                field=$(echo "$line" | sed 's/.*field\/\(.*\) is new.*/\1/')
                section=$(echo "$field" | cut -d. -f3)
                if [[ -n "$section" && "$field" != "$line" ]] && \
                   ! grep -q "^\s*${section}:" "$TMP_DIR/$crd" 2>/dev/null; then
                    continue
                fi
            fi
            FILTERED+="$line"$'\n'
        done <<< "$OUTPUT"

        if echo "$FILTERED" | grep -q '^ERROR:'; then
            echo "$FILTERED"
            exit 1
        fi
        if [[ -n "$FILTERED" ]]; then
            echo "$FILTERED"
        fi
    fi
done
