#!/bin/bash

export OPENSTACK_RELEASE_VERSION=0.0.1-$(date +%s)
export RELATED_IMAGE_OPENSTACK_CLIENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-openstackclient:current-podified
export RELATED_IMAGE_RABBITMQ_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-rabbitmq:current-podified
export RELATED_IMAGE_KEYSTONE_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-keystone:current-podified
export RELATED_IMAGE_MARIADB_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-mariadb:current-podified
export RELATED_IMAGE_INFRA_MEMCACHED_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-memcached:current-podified
export RELATED_IMAGE_INFRA_REDIS_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-redis:current-podified
export RELATED_IMAGE_ANSIBLEEE_IMAGE_URL_DEFAULT=quay.io/openstack-k8s-operators/openstack-ansibleee-runner:current-podified
export RELATED_IMAGE_NOVA_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-nova-api:current-podified
export RELATED_IMAGE_NOVA_CONDUCTOR_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-nova-conductor:current-podified
export RELATED_IMAGE_NOVA_NOVNC_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-nova-novncproxy:current-podified
export RELATED_IMAGE_NOVA_SCHEDULER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-nova-scheduler:current-podified
export RELATED_IMAGE_NOVA_COMPUTE_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-nova-compute:current-podified
export RELATED_IMAGE_NEUTRON_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-neutron-server:current-podified
export RELATED_IMAGE_MANILA_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-manila-api:current-podified
export RELATED_IMAGE_MANILA_SCHEDULER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-manila-scheduler:current-podified
export RELATED_IMAGE_MANILA_SHARE_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-manila-share:current-podified
export RELATED_IMAGE_GLANCE_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-glance-api:current-podified
export RELATED_IMAGE_IRONIC_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ironic-api:current-podified
export RELATED_IMAGE_IRONIC_CONDUCTOR_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ironic-conductor:current-podified
export RELATED_IMAGE_IRONIC_INSPECTOR_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ironic-inspector:current-podified
export RELATED_IMAGE_IRONIC_PXE_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ironic-pxe:current-podified
export RELATED_IMAGE_IRONIC_NEUTRON_AGENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ironic-neutron-agent:current-podified
export RELATED_IMAGE_IRONIC_PYTHON_AGENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/ironic-python-agent:current-podified
export RELATED_IMAGE_OS_CONTAINER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/edpm-hardened-uefi:current-podified
export RELATED_IMAGE_AGENT_IMAGE_URL_DEFAULT=quay.io/openstack-k8s-operators/openstack-baremetal-operator-agent:current-podified
export RELATED_IMAGE_APACHE_IMAGE_URL_DEFAULT=registry.redhat.io/ubi9/httpd-24:latest
export OS_IMAGE_DEFAULT=edpm-hardened-uefi.qcow2
export RELATED_IMAGE_PLACEMENT_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-placement-api:current-podified
export RELATED_IMAGE_CEILOMETER_CENTRAL_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ceilometer-central:current-podified
export RELATED_IMAGE_CEILOMETER_COMPUTE_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ceilometer-compute:current-podified
export RELATED_IMAGE_CEILOMETER_NOTIFICATION_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ceilometer-notification:current-podified
export RELATED_IMAGE_CEILOMETER_IPMI_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ceilometer-ipmi:current-podified
export RELATED_IMAGE_CEILOMETER_SGCORE_IMAGE_URL_DEFAULT=quay.io/openstack-k8s-operators/sg-core:latest
export RELATED_IMAGE_CEILOMETER_MYSQLD_EXPORTER_IMAGE_URL_DEFAULT=quay.io/prometheus/mysqld-exporter:v0.15.1
export RELATED_IMAGE_KSM_IMAGE_URL_DEFAULT=registry.k8s.io/kube-state-metrics/kube-state-metrics:v2.15.0
export RELATED_IMAGE_AODH_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-aodh-api:current-podified
export RELATED_IMAGE_AODH_EVALUATOR_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-aodh-evaluator:current-podified
export RELATED_IMAGE_AODH_NOTIFIER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-aodh-notifier:current-podified
export RELATED_IMAGE_AODH_LISTENER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-aodh-listener:current-podified
export RELATED_IMAGE_OVN_NB_DBCLUSTER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ovn-nb-db-server:current-podified
export RELATED_IMAGE_OVN_SB_DBCLUSTER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ovn-sb-db-server:current-podified
export RELATED_IMAGE_OVN_NORTHD_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ovn-northd:current-podified
export RELATED_IMAGE_OVN_CONTROLLER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ovn-controller:current-podified
export RELATED_IMAGE_OVN_CONTROLLER_OVS_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ovn-base:current-podified
export RELATED_IMAGE_CINDER_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-cinder-api:current-podified
export RELATED_IMAGE_CINDER_BACKUP_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-cinder-backup:current-podified
export RELATED_IMAGE_CINDER_SCHEDULER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-cinder-scheduler:current-podified
export RELATED_IMAGE_CINDER_VOLUME_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-cinder-volume:current-podified
export RELATED_IMAGE_HORIZON_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-horizon:current-podified
export RELATED_IMAGE_HEAT_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-heat-api:current-podified
export RELATED_IMAGE_HEAT_CFNAPI_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-heat-api-cfn:current-podified
export RELATED_IMAGE_HEAT_ENGINE_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-heat-engine:current-podified
export RELATED_IMAGE_SWIFT_PROXY_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-swift-proxy-server:current-podified
export RELATED_IMAGE_SWIFT_ACCOUNT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-swift-account:current-podified
export RELATED_IMAGE_SWIFT_CONTAINER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-swift-container:current-podified
export RELATED_IMAGE_SWIFT_OBJECT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-swift-object:current-podified
export RELATED_IMAGE_NET_UTILS_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-netutils:current-podified
export RELATED_IMAGE_OCTAVIA_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-octavia-api:current-podified
export RELATED_IMAGE_OCTAVIA_HOUSEKEEPING_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-octavia-housekeeping:current-podified
export RELATED_IMAGE_OCTAVIA_HEALTHMANAGER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-octavia-health-manager:current-podified
export RELATED_IMAGE_OCTAVIA_WORKER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-octavia-worker:current-podified
export RELATED_IMAGE_OCTAVIA_RSYSLOG_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-rsyslog:current-podified
export RELATED_IMAGE_DESIGNATE_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-designate-api:current-podified
export RELATED_IMAGE_DESIGNATE_CENTRAL_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-designate-central:current-podified
export RELATED_IMAGE_DESIGNATE_MDNS_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-designate-mdns:current-podified
export RELATED_IMAGE_DESIGNATE_PRODUCER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-designate-producer:current-podified
export RELATED_IMAGE_DESIGNATE_WORKER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-designate-worker:current-podified
export RELATED_IMAGE_DESIGNATE_BACKENDBIND9_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-designate-backend-bind9:current-podified
export RELATED_IMAGE_DESIGNATE_UNBOUND_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-unbound:current-podified
export RELATED_IMAGE_BARBICAN_API_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-barbican-api:current-podified
export RELATED_IMAGE_BARBICAN_WORKER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-barbican-worker:current-podified
export RELATED_IMAGE_BARBICAN_KEYSTONE_LISTENER_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-barbican-keystone-listener:current-podified
export RELATED_IMAGE_EDPM_FRR_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-frr:current-podified
export RELATED_IMAGE_EDPM_ISCSID_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-iscsid:current-podified
export RELATED_IMAGE_EDPM_LOGROTATE_CROND_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-cron:current-podified
export RELATED_IMAGE_EDPM_MULTIPATHD_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-multipathd:current-podified
export RELATED_IMAGE_EDPM_NEUTRON_DHCP_AGENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-neutron-dhcp-agent:current-podified
export RELATED_IMAGE_EDPM_NEUTRON_METADATA_AGENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-neutron-metadata-agent-ovn:current-podified
export RELATED_IMAGE_EDPM_NEUTRON_OVN_AGENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-neutron-ovn-agent:current-podified
export RELATED_IMAGE_EDPM_NEUTRON_SRIOV_AGENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-neutron-sriov-agent:current-podified
export RELATED_IMAGE_EDPM_OVN_BGP_AGENT_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ovn-bgp-agent:current-podified
export RELATED_IMAGE_EDPM_NODE_EXPORTER_IMAGE_URL_DEFAULT=quay.io/prometheus/node-exporter:v1.5.0
export RELATED_IMAGE_OPENSTACK_NETWORK_EXPORTER_IMAGE_URL_DEFAULT=quay.io/openstack-k8s-operators/openstack-network-exporter:current-podified
export RELATED_IMAGE_EDPM_KEPLER_IMAGE_URL_DEFAULT=quay.io/sustainable_computing_io/kepler:release-0.7.12
export RELATED_IMAGE_EDPM_PODMAN_EXPORTER_IMAGE_URL_DEFAULT=quay.io/navidys/prometheus-podman-exporter:v1.10.1
export RELATED_IMAGE_TEST_TEMPEST_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-tempest-all:current-podified
#NOTE: TEST_ images below do not get released downstream. They should not be prefixed with RELATED
export TEST_TOBIKO_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-tobiko:current-podified
export TEST_ANSIBLETEST_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-ansible-tests:current-podified
export TEST_HORIZONTEST_IMAGE_URL_DEFAULT=quay.io/podified-antelope-centos9/openstack-horizontest:current-podified
export RELATED_IMAGE_OPENSTACK_MUST_GATHER_DEFAULT=quay.io/openstack-k8s-operators/openstack-must-gather:latest
