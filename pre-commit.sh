#!/bin/bash
set -ex

BASE_DIR="$(dirname $0)"
#cd "${BASE_DIR}/../.."
cd "${BASE_DIR}"

# Run pre-commit on all-files
PRE_COMMIT_HOME=/tmp pre-commit run --all-files --show-diff-on-failure
