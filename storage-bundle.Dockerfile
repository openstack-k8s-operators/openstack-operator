ARG GOLANG_CTX=golang:1.19
ARG GLANCE_BUNDLE=quay.io/openstack-k8s-operators/glance-operator-bundle:latest
ARG CINDER_BUNDLE=quay.io/openstack-k8s-operators/cinder-operator-bundle:latest
ARG MANILA_BUNDLE=quay.io/openstack-k8s-operators/manila-operator-bundle:latest

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

FROM $GLANCE_BUNDLE as glance-bundle
FROM $CINDER_BUNDLE as cinder-bundle
FROM $MANILA_BUNDLE as manila-bundle

FROM $GOLANG_CTX as merger
# this provides CSV_VERSION to the csv-merger
WORKDIR /workspace
COPY --from=builder /workspace/csv-merger .

# this makes creates the base /manifests dir
COPY --from=glance-bundle /manifests /manifests/
# copy in the rest
COPY --from=cinder-bundle /manifests/* /manifests/
COPY --from=manila-bundle /manifests/* /manifests/
COPY storage-operators.clusterserviceversion.yaml /manifests/

RUN /workspace/csv-merger \
  --export-env-file=/env-vars.yaml \
  --glance-csv=/manifests/glance-operator.clusterserviceversion.yaml \
  --cinder-csv=/manifests/cinder-operator.clusterserviceversion.yaml \
  --manila-csv=/manifests/manila-operator.clusterserviceversion.yaml \
  --base-csv=/manifests/storage-operators.clusterserviceversion.yaml | tee /openstack-operator-storage.clusterserviceversion.yaml.new

# remove all individual operator CSV's
RUN rm /manifests/*clusterserviceversion.yaml

RUN mkdir /metadata/
COPY bundle/metadata/annotations.yaml /metadata/
RUN sed -e "s|operators.operatorframework.io.bundle.package.v1.*|operators.operatorframework.io.bundle.package.v1: openstack-storage-operators|" -i /metadata/annotations.yaml

### Put everything together
FROM scratch

# Core bundle labels.
LABEL operators.operatorframework.io.bundle.mediatype.v1=registry+v1
LABEL operators.operatorframework.io.bundle.manifests.v1=manifests/
LABEL operators.operatorframework.io.bundle.metadata.v1=metadata/
LABEL operators.operatorframework.io.bundle.package.v1=openstack-storage-operators
LABEL operators.operatorframework.io.bundle.channels.v1=alpha
LABEL operators.operatorframework.io.metrics.builder=operator-sdk-v1.22.1
LABEL operators.operatorframework.io.metrics.mediatype.v1=metrics+v1
LABEL operators.operatorframework.io.metrics.project_layout=go.kubebuilder.io/v3

# Copy files to locations specified by labels.

# copy in manifests from operators
COPY --from=merger /manifests /manifests/
COPY --from=merger /metadata /metadata/

# overwrite with the final merged CSV
COPY --from=merger /openstack-operator-storage.clusterserviceversion.yaml.new /manifests/openstack-operator-storage.clusterserviceversion.yaml
COPY --from=merger /env-vars.yaml /env-vars.yaml
