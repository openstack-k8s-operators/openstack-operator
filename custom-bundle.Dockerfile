ARG GOLANG_CTX=golang:1.19

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

# Build the csv-merger
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o csv-merger csv-merger.go

FROM $GOLANG_CTX as merger
WORKDIR /workspace
COPY --from=builder /workspace/csv-merger .

# local operator manifests
COPY bundle/manifests /manifests/
COPY bundle_extra_data /bundle_extra_data
RUN cp -a /bundle_extra_data/manifests/* /manifests/

# Merge things into our openstack-operator CSV:
#  -dataplane-operator CSV
#  -ENV vars from all operators (for webhooks)
RUN /workspace/csv-merger \
  --import-env-files=/bundle_extra_data/env-vars.yaml \
  --dataplane-csv=/bundle_extra_data/manifests/dataplane-operator.clusterserviceversion.yaml \
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
COPY --from=merger /manifests/* /manifests/

# overwrite with the final merged CSV
COPY --from=merger /openstack-operator.clusterserviceversion.yaml.new /manifests/openstack-operator.clusterserviceversion.yaml
