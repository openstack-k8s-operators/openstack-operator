#!/bin/bash

set -ex

CONFIG_DIR=${CONFIG_DIR:-"../config/samples"}
CRD_NAME=${CRD_NAME:-"core_v1beta1_openstackcontrolplane_generated.yaml"}

# Containers Vars
SERVICE_REGISTRY=${SERVICE_REGISTRY:-"quay.io"}
SERVICE_ORG=${SERVICE_ORG:-"tripleozedcentos9"}
CONTAINER_TAG=${CONTAINER_TAG:-"current-tripleo"}
PREFIX=${PREFIX:-"openstack"}

# Services containers
KEYSTONEAPI_IMG=${KEYSTONEAPI_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-keystone:${CONTAINER_TAG}"}
MARIADB_DEPL_IMG=${MARIADB_DEPL_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-mariadb:${CONTAINER_TAG}"}
PLACEMENTAPI_IMG=${PLACEMENTAPI_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-placement-api:${CONTAINER_TAG}"}
GLANCEAPI_IMG=${GLANCEAPI_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-glance-api:${CONTAINER_TAG}"}
CINDERAPI_IMG=${CINDERAPI_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-cinder-api:${CONTAINER_TAG}"}
CINDER_SCHEDULER_IMG=${CINDER_SCHEDULER_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-cinder-schedular:${CONTAINER_TAG}"}
CINDER_BACKUP_IMG=${CINDER_BACKUP_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-cinder-backup:${CONTAINER_TAG}"}
CINDER_VOLUME_IMG=${CINDER_VOLUME_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-cinder-volume:${CONTAINER_TAG}"}
OVN_NB_DB_SERVER_IMG=${OVN_NB_DB_SERVER_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-ovn-nb-db-server:${CONTAINER_TAG}"}
OVN_SB_DB_SERVER_IMG=${OVN_SB_DB_SERVER_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-ovn-sb-db-server:${CONTAINER_TAG}"}
OVN_NORTHD_IMG=${OVN_NORTHD_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-ovn-northd:${CONTAINER_TAG}"}
OVS_IMG=${OVS_IMG:-"${SERVICE_REGISTRY}/skaplons/ovs:latest"}
OVN_CONTROLLER_IMG=${OVN_CONTROLLER_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-ovn-controller:${CONTAINER_TAG}"}
NEUTRON_SERVER_IMG=${NEUTRON_SERVER_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-neutron-server:${CONTAINER_TAG}"}
NEUTRONAPI_IMG=${NEUTRONAPI_IMG:-"${SERVICE_REGISTRY}/${SERVICE_ORG}/${PREFIX}-neutron-api:${CONTAINER_TAG}"}

# CRD content
cat <<EOF >${CONFIG_DIR}/${CRD_NAME}
apiVersion: core.openstack.org/v1beta1
kind: OpenStackControlPlane
metadata:
  name: openstack
spec:
  secret: osp-secret
  storageClass: local-storage
  keystone:
    template:
      containerImage: ${KEYSTONEAPI_IMG}
      databaseInstance: openstack
      secret: osp-secret
  mariadb:
    template:
      containerImage: ${MARIADB_DEPL_IMG}
      storageRequest: 500M
  rabbitmq:
    templates:
      rabbitmq:
        replicas: 1
        #resources:
        #  requests:
        #    cpu: 500m
        #    memory: 1Gi
        #  limits:
        #    cpu: 800m
        #    memory: 1Gi
  placement:
    template:
      databaseInstance: openstack
      containerImage: ${PLACEMENTAPI_IMG}
      secret: osp-secret
  glance:
    template:
      databaseInstance: openstack
      containerImage: ${GLANCEAPI_IMG}
      storageClass: ""
      storageRequest: 10G
      glanceAPIInternal:
        containerImage: ${GLANCEAPI_IMG}
      glanceAPIExternal:
        containerImage: ${GLANCEAPI_IMG}
  cinder:
    template:
      cinderAPI:
        replicas: 1
        containerImage: ${CINDERAPI_IMG}
      cinderScheduler:
        replicas: 1
        containerImage: ${CINDER_SCHEDULER_IMG}
      cinderBackup:
        replicas: 1
        containerImage: ${CINDER_BACKUP_IMG}
      cinderVolumes:
        volume1:
          containerImage: ${CINDER_VOLUME_IMG}
          replicas: 1
  ovn:
    template:
      ovnDBCluster:
        ovndbcluster-nb:
          replicas: 1
          containerImage: ${OVN_NB_DB_SERVER_IMG}
          dbType: NB
          storageRequest: 10G
        ovndbcluster-sb:
          replicas: 1
          containerImage: ${OVN_SB_DB_SERVER_IMG}
          dbType: SB
          storageRequest: 10G
      ovnNorthd:
        replicas: 1
        containerImage: ${OVN_NORTHD_IMG}
  ovs:
    template:
      ovsContainerImage: ${OVS_IMG}
      ovnContainerImage: ${OVN_CONTROLLER_IMG}
      external-ids:
        system-id: "random"
        ovn-bridge: "br-int"
        ovn-encap-type: "geneve"
  neutron:
    template:
      databaseInstance: openstack
      containerImage: ${NEUTRONAPI_IMG}
      secret: osp-secret
  nova:
    template:
      secret: osp-secret
EOF
