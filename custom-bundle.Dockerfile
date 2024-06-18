ARG GOLANG_CTX=registry.access.redhat.com/ubi9/go-toolset:1.20

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
USER root
COPY cmd/csv-merger/csv-merger.go csv-merger.go
COPY pkg/ pkg/

# Build the csv-merger
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o csv-merger csv-merger.go

USER $USER_ID

FROM $GOLANG_CTX as merger
WORKDIR /workspace
COPY --from=builder /workspace/csv-merger .

USER root
# local operator manifests
COPY bundle/manifests /manifests/
COPY bundle_extra_data /bundle_extra_data

# Merge things into our openstack-operator CSV:
#  -openstack-operator CSV
#  -ENV vars from all operators (for webhooks)
RUN /workspace/csv-merger \
	--import-env-files=/bundle_extra_data/env-vars.yaml \
	--base-csv=/manifests/openstack-operator.clusterserviceversion.yaml | tee /openstack-operator.clusterserviceversion.yaml.new

# remove all individual operator CSV's
RUN rm /manifests/*clusterserviceversion.yaml

USER $USER_ID

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
USER root
COPY bundle/metadata /metadata/
COPY bundle/tests/scorecard /tests/scorecard/

# copy in manifests from operators
COPY bundle/manifests /manifests/
COPY --from=merger /manifests/* /manifests/

# overwrite with the final merged CSV
COPY --from=merger /openstack-operator.clusterserviceversion.yaml.new /manifests/openstack-operator.clusterserviceversion.yaml

USER $USER_ID
