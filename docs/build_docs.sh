#!/usr/bin/env bash
set -ex pipefail

CTLPLANE_FILES=()
CTLPLANE_PATHS=(
    "apis/client/v1beta1/openstackclient_types.go"
    "apis/core/v1beta1/openstackcontrolplane_types.go"
    "apis/core/v1beta1/openstackversion_types.go"
)
DATAPLANE_FILES=()
DATAPLANE_PATHS=(
    "apis/dataplane/v1beta1/openstackdataplanedeployment_types.go"
    "apis/dataplane/v1beta1/openstackdataplanenodeset_types.go"
    "apis/dataplane/v1beta1/openstackdataplaneservice_types.go"
    "apis/dataplane/v1beta1/common.go"
)

# Getting APIs from Services
SERVICE_PATH=($(MODCACHE=$(go env GOMODCACHE) awk '/openstack-k8s-operators/ && ! /lib-common/ && ! /openstack-operator/ && ! /infra/ && ! /replace/ {print ENVIRON["MODCACHE"] "/" $1 "@" $2 "/v1beta1/*_types.go"}' apis/go.mod))
for SERVICE in ${SERVICE_PATH[@]};do
    CTLPLANE_PATHS+=($(ls ${SERVICE}))
done

# Getting APIs from Infra
INFRA_PATH=($(MODCACHE=$(go env GOMODCACHE) awk '/openstack-k8s-operators/ && /infra/ {print ENVIRON["MODCACHE"] "/" $1 "@" $2 "/"}' apis/go.mod))
PATTERNS=("memcached/v1beta1/*_types.go"  "network/v1beta1/*_types.go"  "rabbitmq/v1beta1/*_types.go")
for INFRA in ${PATTERNS[@]};do
    ls ${INFRA_PATH}${INFRA}
    CTLPLANE_PATHS+=($(ls ${INFRA_PATH}${INFRA}))
done

# Adding -f to all API paths
for API_PATH in ${CTLPLANE_PATHS[@]};do
    CTLPLANE_FILES+=$(echo " -f $API_PATH")
done
for API_PATH in ${DATAPLANE_PATHS[@]};do
    DATAPLANE_FILES+=$(echo " -f $API_PATH")
done

# Build ctlplane docs from APIs
${CRD_ASCIIDOC} $CTLPLANE_FILES -n OpenStackClient -n OpenStackControlPlane -n OpenStackVersion > docs/assemblies/ctlplane_resources.adoc

# Build dataplane docs from APIs
${CRD_ASCIIDOC} $DATAPLANE_FILES -n OpenStackDataPlaneDeployment -n OpenStackDataPlaneNodeSet -n OpenStackDataPlaneService > docs/assemblies/dataplane_resources.adoc


# Render HTML
cd docs
${MAKE} html BUILD=upstream
${MAKE} html BUILD=downstream
