# Build the manager binary
FROM golang:1.18 as builder

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

# FIXME(dprince): ARG doesn't work with FROM with buildah/podman? for each of the services below
#ARG KEYSTONE_BUNDLE=quay.io/openstack-k8s-operators/keystone-operator-bundle:latest
FROM quay.io/openstack-k8s-operators/keystone-operator-bundle:latest as keystone-bundle
FROM quay.io/openstack-k8s-operators/mariadb-operator-bundle:latest as mariadb-bundle

FROM golang:1.18 as merger
WORKDIR /workspace
COPY --from=builder /workspace/csv-merger .

# local operator manifests
COPY bundle/manifests /manifests/

# Custom Manifests
COPY --from=keystone-bundle /manifests/* /manifests/
COPY --from=mariadb-bundle /manifests/* /manifests/

RUN /workspace/csv-merger \
  --mariadb-csv=/manifests/mariadb-operator.clusterserviceversion.yaml \
  --keystone-csv=/manifests/keystone-operator.clusterserviceversion.yaml \
  --openstack-csv=/manifests/openstack-operator.clusterserviceversion.yaml | tee /openstack-operator.clusterserviceversion.yaml.new

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
COPY --from=merger /manifests /manifests/

# overwrite with the final merged CSV
COPY --from=merger /openstack-operator.clusterserviceversion.yaml.new /manifests/openstack-operator.clusterserviceversion.yaml
