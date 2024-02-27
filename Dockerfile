ARG GOLANG_BUILDER=registry.access.redhat.com/ubi9/go-toolset:1.20
ARG OPERATOR_BASE_IMAGE=registry.access.redhat.com/ubi9/ubi-minimal:latest
# Build the manager binary
FROM $GOLANG_BUILDER AS builder

#Arguments required by OSBS build system
ARG CACHITO_ENV_FILE=/remote-source/cachito.env

ARG REMOTE_SOURCE=.
ARG REMOTE_SOURCE_DIR=/remote-source
ARG REMOTE_SOURCE_SUBDIR=
ARG DEST_ROOT=/dest-root

ARG GO_BUILD_EXTRA_ARGS="-tags strictfipsruntime"
ARG GO_BUILD_EXTRA_ENV_ARGS="CGO_ENABLED=1 GO111MODULE=on"

COPY $REMOTE_SOURCE $REMOTE_SOURCE_DIR
WORKDIR $REMOTE_SOURCE_DIR/$REMOTE_SOURCE_SUBDIR

USER root
RUN mkdir -p ${DEST_ROOT}/usr/local/bin/

# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN if [ ! -f $CACHITO_ENV_FILE ]; then go mod download ; fi

# Build manager
RUN if [ -f $CACHITO_ENV_FILE ] ; then source $CACHITO_ENV_FILE ; fi ; env ${GO_BUILD_EXTRA_ENV_ARGS} go build ${GO_BUILD_EXTRA_ARGS} -a -o ${DEST_ROOT}/manager main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM $OPERATOR_BASE_IMAGE

ARG DEST_ROOT=/dest-root
# NONROOT default id https://github.com/GoogleContainerTools/distroless/blob/main/base/base.bzl#L8=
ARG USER_ID=65532

ARG IMAGE_COMPONENT="openstack-operator-container"
ARG IMAGE_NAME="openstack-operator"
ARG IMAGE_VERSION="1.0.0"
ARG IMAGE_SUMMARY="Openstack Operator"
ARG IMAGE_DESC="This image includes the openstack-operator"
ARG IMAGE_TAGS="cn-openstack openstack"

### DO NOT EDIT LINES BELOW
# Auto generated using CI tools from
# https://github.com/openstack-k8s-operators/openstack-k8s-operators-ci

# Labels required by upstream and osbs build system
LABEL com.redhat.component="${IMAGE_COMPONENT}" \
	name="${IMAGE_NAME}" \
	version="${IMAGE_VERSION}" \
	summary="${IMAGE_SUMMARY}" \
	io.k8s.name="${IMAGE_NAME}" \
	io.k8s.description="${IMAGE_DESC}" \
	io.openshift.tags="${IMAGE_TAGS}"
### DO NOT EDIT LINES ABOVE

ENV USER_UID=$USER_ID

WORKDIR /

# Install operator binary to WORKDIR
COPY --from=builder ${DEST_ROOT}/manager .

USER $USER_ID

ENV PATH="/:${PATH}"

ENTRYPOINT ["/manager"]
