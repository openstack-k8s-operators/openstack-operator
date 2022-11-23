#!/bin/bash
set -e
cp custom-bundle.Dockerfile custom-bundle.Dockerfile.pinned
#loop over each openstack-k8s-operators go.mod entry
for X in $(go list -m -json all | jq -r '. | select(.Path | contains("openstack")) | "\(.Path)=\(.Version)"'); do
  #example: github.com/openstack-k8s-operators/placement-operator/api=v0.0.0-20221007105015-13dce7450573
  BASE=$(echo $X | sed -e 's|github.com/openstack-k8s-operators/\([^-\)]*\).*|\1|')
  REF=$(echo $X | sed -e 's|github.com/[^\=]*=v0.0.0-[0-9]*-\(.*\)$|\1|')
  if  ! echo "$BASE" | grep -e "lib" -e "openstack" &> /dev/null; then
      SHA=$(curl -s https://quay.io/api/v1/repository/openstack-k8s-operators/$BASE-operator-bundle/tag/ \
            | jq -r .tags[].name | grep $REF)
      sed -i custom-bundle.Dockerfile.pinned -e "s|FROM quay.io/openstack-k8s-operators/${BASE}-operator-bundle.*|FROM quay.io/openstack-k8s-operators/${BASE}-operator-bundle:$SHA as ${BASE}-bundle|"
  fi
done
