ARG GOLANG_CTX=golang:1.19
ARG INFRA_BUNDLE=quay.io/openstack-k8s-operators/infra-operator-bundle:latest
ARG KEYSTONE_BUNDLE=quay.io/openstack-k8s-operators/keystone-operator-bundle:latest
ARG MARIADB_BUNDLE=quay.io/openstack-k8s-operators/mariadb-operator-bundle:latest
ARG PLACEMENT_BUNDLE=quay.io/openstack-k8s-operators/placement-operator-bundle:latest
ARG OVN_BUNDLE=quay.io/openstack-k8s-operators/ovn-operator-bundle:latest
ARG NEUTRON_BUNDLE=quay.io/openstack-k8s-operators/neutron-operator-bundle:latest
ARG ANSIBLEEE_BUNDLE=quay.io/openstack-k8s-operators/openstack-ansibleee-operator-bundle:latest
ARG DATAPLANE_BUNDLE=quay.io/openstack-k8s-operators/dataplane-operator-bundle:latest
ARG NOVA_BUNDLE=quay.io/openstack-k8s-operators/nova-operator-bundle:latest
ARG HEAT_BUNDLE=quay.io/openstack-k8s-operators/heat-operator-bundle:latest
ARG IRONIC_BUNDLE=quay.io/openstack-k8s-operators/ironic-operator-bundle:latest
ARG BAREMETAL_BUNDLE=quay.io/openstack-k8s-operators/openstack-baremetal-operator-bundle:latest
ARG TELEMETRY_BUNDLE=quay.io/openstack-k8s-operators/telemetry-operator-bundle:latest
ARG HORIZON_BUNDLE=quay.io/openstack-k8s-operators/horizon-operator-bundle:latest
ARG GLANCE_BUNDLE=quay.io/openstack-k8s-operators/glance-operator-bundle:latest
ARG CINDER_BUNDLE=quay.io/openstack-k8s-operators/cinder-operator-bundle:latest
ARG MANILA_BUNDLE=quay.io/openstack-k8s-operators/manila-operator-bundle:latest
ARG SWIFT_BUNDLE=quay.io/openstack-k8s-operators/swift-operator-bundle:latest
ARG OCTAVIA_BUNDLE=quay.io/openstack-k8s-operators/octavia-operator-bundle:latest

# Build the manager binary
FROM $GOLANG_CTX as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum

COPY apis/ apis/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/csv-merger/csv-merger.go csv-merger.go
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o csv-merger csv-merger.go

FROM $INFRA_BUNDLE as infra-bundle
FROM $KEYSTONE_BUNDLE as keystone-bundle
FROM $MARIADB_BUNDLE as mariadb-bundle
FROM $PLACEMENT_BUNDLE as placement-bundle
FROM $OVN_BUNDLE as ovn-bundle
FROM $NEUTRON_BUNDLE as neutron-bundle
FROM $ANSIBLEEE_BUNDLE as openstack-ansibleee-bundle
FROM $DATAPLANE_BUNDLE as dataplane-bundle
FROM $NOVA_BUNDLE as nova-bundle
FROM $HEAT_BUNDLE as heat-bundle
FROM $IRONIC_BUNDLE as ironic-bundle
FROM $BAREMETAL_BUNDLE as baremetal-bundle
FROM $TELEMETRY_BUNDLE as telemetry-bundle
FROM $HORIZON_BUNDLE as horizon-bundle
FROM $GLANCE_BUNDLE as glance-bundle
FROM $CINDER_BUNDLE as cinder-bundle
FROM $MANILA_BUNDLE as manila-bundle
FROM $SWIFT_BUNDLE as swift-bundle
FROM $OCTAVIA_BUNDLE as octavia-bundle

FROM $GOLANG_CTX as merger
WORKDIR /workspace
COPY --from=builder /workspace/csv-merger .

# local operator manifests
COPY bundle/manifests /manifests/

# Custom Manifests
COPY --from=keystone-bundle /manifests/* /manifests/
COPY --from=mariadb-bundle /manifests/* /manifests/
COPY --from=infra-bundle /manifests/* /manifests/
COPY --from=placement-bundle /manifests/* /manifests/
COPY --from=ovn-bundle /manifests/* /manifests/
COPY --from=neutron-bundle /manifests/* /manifests/
COPY --from=openstack-ansibleee-bundle /manifests/* /manifests/
COPY --from=dataplane-bundle /manifests/* /manifests/
COPY --from=nova-bundle /manifests/* /manifests/
COPY --from=heat-bundle /manifests/* /manifests/
COPY --from=ironic-bundle /manifests/* /manifests/
COPY --from=baremetal-bundle /manifests/* /manifests/
COPY --from=telemetry-bundle /manifests/* /manifests/
COPY --from=horizon-bundle /manifests/* /manifests/
COPY --from=glance-bundle /manifests/* /manifests/
COPY --from=cinder-bundle /manifests/* /manifests/
COPY --from=manila-bundle /manifests/* /manifests/
COPY --from=swift-bundle /manifests/* /manifests/
COPY --from=octavia-bundle /manifests/* /manifests/

# extract all the env vars (NOTE/FIXME: base-csv is unused below to be refactored)
RUN /workspace/csv-merger \
  --export-env-file=/env-vars.yaml \
  --mariadb-csv=/manifests/mariadb-operator.clusterserviceversion.yaml \
  --infra-csv=/manifests/infra-operator.clusterserviceversion.yaml \
  --keystone-csv=/manifests/keystone-operator.clusterserviceversion.yaml \
  --placement-csv=/manifests/placement-operator.clusterserviceversion.yaml \
  --ovn-csv=/manifests/ovn-operator.clusterserviceversion.yaml \
  --neutron-csv=/manifests/neutron-operator.clusterserviceversion.yaml \
  --ansibleee-csv=/manifests/openstack-ansibleee-operator.clusterserviceversion.yaml \
  --dataplane-csv=/manifests/dataplane-operator.clusterserviceversion.yaml \
  --nova-csv=/manifests/nova-operator.clusterserviceversion.yaml \
  --heat-csv=/manifests/heat-operator.clusterserviceversion.yaml \
  --ironic-csv=/manifests/ironic-operator.clusterserviceversion.yaml \
  --baremetal-csv=/manifests/openstack-baremetal-operator.clusterserviceversion.yaml \
  --horizon-csv=/manifests/horizon-operator.clusterserviceversion.yaml \
  --telemetry-csv=/manifests/telemetry-operator.clusterserviceversion.yaml \
  --glance-csv=/manifests/glance-operator.clusterserviceversion.yaml \
  --cinder-csv=/manifests/cinder-operator.clusterserviceversion.yaml \
  --manila-csv=/manifests/manila-operator.clusterserviceversion.yaml \
  --swift-csv=/manifests/swift-operator.clusterserviceversion.yaml \
  --octavia-csv=/manifests/octavia-operator.clusterserviceversion.yaml \
  --base-csv=/manifests/openstack-operator.clusterserviceversion.yaml | tee /fixme-required-for-now-but-will-can-made-optional.yaml

# apply all the ENV vars to the actual base-csv
RUN /workspace/csv-merger \
  --import-env-files=/env-vars.yaml \
  --dataplane-csv=/manifests/dataplane-operator.clusterserviceversion.yaml \
  --base-csv=/manifests/openstack-operator.clusterserviceversion.yaml | tee /openstack-operator.clusterserviceversion.yaml.new

# remove all individual operator CSV's
RUN rm /manifests/*clusterserviceversion.yaml

### Put everything together
FROM scratch

# Core bundle labels.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=openstack-operator
LABEL operators.operatorframework.io.bundle.channels.v1=alpha
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.22.1
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v3

# Labels for testing.
LABEL operators.operatorframework.io.test.mediatype.v1=scorecard+v1
LABEL operators.operatorframework.io.test.config.v1=tests/scorecard/

# Copy files to locations specified by labels.
COPY bundle/metadata /metadata/
COPY bundle/tests/scorecard /tests/scorecard/

# copy in manifests from operators
COPY bundle/manifests /manifests/
COPY --from=merger /manifests/dataplane.openstack.* /manifests/

# overwrite with the final merged CSV
COPY --from=merger /openstack-operator.clusterserviceversion.yaml.new /manifests/openstack-operator.clusterserviceversion.yaml
