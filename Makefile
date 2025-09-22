# VERSION defines the project version for the bundle.
# Update this value when you upgrade the version of your project.
# To re-generate a bundle for another specific version without changing the standard setup, you can:
# - use the VERSION as arg of the bundle target (e.g make bundle VERSION=0.0.2)
# - use environment variables to overwrite this value (e.g export VERSION=0.0.2)
VERSION ?= 0.4.0

OPENSTACK_RELEASE_VERSION ?= $(VERSION)-$(shell date +%s)

# CHANNELS define the bundle channels used in the bundle.
# Add a new line here if you would like to change its default config. (E.g CHANNELS = "candidate,fast,stable")
# To re-generate a bundle for other specific channels without changing the standard setup, you can:
# - use the CHANNELS as arg of the bundle target (e.g make bundle CHANNELS=candidate,fast,stable)
# - use environment variables to overwrite this value (e.g export CHANNELS="candidate,fast,stable")
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif

# DEFAULT_CHANNEL defines the default channel used in the bundle.
# Add a new line here if you would like to change its default config. (E.g DEFAULT_CHANNEL = "stable")
# To re-generate a bundle for any other default channel without changing the default setup, you can:
# - use the DEFAULT_CHANNEL as arg of the bundle target (e.g make bundle DEFAULT_CHANNEL=stable)
# - use environment variables to overwrite this value (e.g export DEFAULT_CHANNEL="stable")
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# IMAGE_TAG_BASE defines the docker.io namespace and part of the image name for remote images.
# This variable is used to construct full image tags for bundle and catalog images.
#
# For example, running 'make bundle-build bundle-push catalog-build catalog-push' will build and push both
# openstack.org/openstack-operator-bundle:$VERSION and openstack.org/openstack-operator-catalog:$VERSION.
IMAGE_TAG_BASE ?= quay.io/$(USER)/openstack-operator

# BUNDLE_IMG defines the image:tag used for the bundle.
# You can use it as an arg. (E.g make bundle-build BUNDLE_IMG=<some-registry>/<project-name-bundle>:<tag>)
BUNDLE_IMG ?= $(IMAGE_TAG_BASE)-bundle:v$(VERSION)

# BUNDLE_GEN_FLAGS are the flags passed to the operator-sdk generate bundle command
BUNDLE_GEN_FLAGS ?= -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)

# USE_IMAGE_DIGESTS defines if images are resolved via tags or digests
# You can enable this value if you would like to use SHA Based Digests
# To enable set flag to true
USE_IMAGE_DIGESTS ?= false
ifeq ($(USE_IMAGE_DIGESTS), true)
	BUNDLE_GEN_FLAGS += --use-image-digests
endif

# Set the Operator SDK version to use. By default, what is installed on the system is used.
# This is useful for CI or a project to utilize a specific version of the operator-sdk toolkit.
OPERATOR_SDK_VERSION ?= v1.31.0

# Image URL to use all building/pushing image targets
DEFAULT_IMG ?= quay.io/openstack-k8s-operators/openstack-operator:latest
IMG ?= $(DEFAULT_IMG)
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
ENVTEST_K8S_VERSION = 1.31

CRDDESC_OVERRIDE ?= :maxDescLen=0

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

# Extra vars which will be passed to the Docker-build
DOCKER_BUILD_ARGS ?=

.PHONY: all
all: build

##@ Docs

.PHONY: docs-dependencies
docs-dependencies: .bundle

.PHONY: .bundle
.bundle:
	if ! type bundle; then \
		echo "Bundler not found. On Linux run 'sudo dnf install /usr/bin/bundle' to install it."; \
		exit 1; \
	fi

	bundle config set --local path 'local/bundle'; bundle install

.PHONY: docs
docs: manifests docs-dependencies crd-to-markdown  docs-kustomize-examples ## Build docs
	CRD_MARKDOWN=$(CRD_MARKDOWN) MAKE=$(MAKE) ./docs/build_docs.sh

.PHONY: docs-preview
docs-preview: docs
	cd docs; $(MAKE) open-html

.PHONY: docs-watch
docs-watch: docs-preview
	cd docs; $(MAKE) watch-html

.PHONY: docs-clean
docs-clean:
	rm -r docs_build

.PHONY: docs-examples
docs-kustomize-examples: oc yq ## Generate updated docs from examples using kustomize
	LOCALBIN=$(LOCALBIN) ./docs/kustomize_to_docs.sh

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

# (dprince) FIXME: controller-gen crd didn't seem to like multiple paths so I didn't split it. So we can continue using kubebuilder
# I did split out the rbac for both binaries so we can use separate roles
.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	mkdir -p config/operator/rbac && \
	$(CONTROLLER_GEN) crd$(CRDDESC_OVERRIDE) output:crd:artifacts:config=config/crd/bases webhook paths="./..." && \
	$(CONTROLLER_GEN) rbac:roleName=manager-role paths="{./apis/lightspeed/...,./apis/client/...,./apis/core/...,./apis/dataplane/...,./controllers/lightspeed/...,./controllers/client/...,./controllers/core/...,./controllers/dataplane/...,./pkg/...}" output:dir=config/rbac && \
	$(CONTROLLER_GEN) rbac:roleName=operator-role paths="./controllers/operator/..." paths="./apis/operator/..." output:dir=config/operator/rbac && \
	rm -f apis/bases/* && cp -a config/crd/bases apis/

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: bindata
bindata: export LOCAL_BINARIES=$(LOCALBIN)
bindata: kustomize yq ## Call sync bindata script
	mkdir -p bindata/crds bindata/rbac bindata/operator
	$(KUSTOMIZE) build config/crd > bindata/crds/crds.yaml
	$(KUSTOMIZE) build config/default > bindata/operator/operator.yaml
	sed -i bindata/operator/operator.yaml -e "s|replicas:.*|replicas: {{ .OpenStackOperator.Deployment.Replicas }}|"
	sed -i bindata/operator/operator.yaml -e "/envCustom/c\\{{- range .OpenStackOperator.Deployment.Manager.Env }}\n        - name: '{{ .Name }}'\n          value: '{{ .Value }}'\n{{- end }}"
	sed -i bindata/operator/operator.yaml -e "/customLimits/c\\            cpu: {{ .OpenStackOperator.Deployment.Manager.Resources.Limits.CPU }}\n            memory: {{ .OpenStackOperator.Deployment.Manager.Resources.Limits.Memory }}"
	sed -i bindata/operator/operator.yaml -e "/customRequests/c\\            cpu: {{ .OpenStackOperator.Deployment.Manager.Resources.Requests.CPU }}\n            memory: {{ .OpenStackOperator.Deployment.Manager.Resources.Requests.Memory }}"
	sed -i bindata/operator/operator.yaml -e "s|kube-rbac-proxy:replace_me.*|'{{ .OpenStackOperator.Deployment.KubeRbacProxy.Image }}'|"
	sed -i bindata/operator/operator.yaml -e "/customTolerations/c\\      tolerations:\n{{- range .OpenStackOperator.Deployment.Tolerations }}\n      - key: \"{{ .Key }}\"\n{{- if .Operator }}\n        operator: \"{{ .Operator }}\"\n{{- end }}\n{{- if .Value }}\n        value: \"{{ .Value }}\"\n{{- end }}\n{{- if .Effect }}\n        effect: \"{{ .Effect }}\"\n{{- end }}\n{{- if .TolerationSeconds }}\n        tolerationSeconds: {{ .TolerationSeconds }}\n{{- end }}\n{{- end }}"
	cp config/operator/managers.yaml bindata/operator/
	cp config/operator/rabbit.yaml bindata/operator/
	$(KUSTOMIZE) build config/rbac > bindata/rbac/rbac.yaml
	/bin/bash hack/sync-bindata.sh

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: gowork ## Run go vet against code.
	go vet ./...
	go vet ./apis/...

BRANCH ?= main
.PHONY: force-bump
force-bump: ## Force bump after tagging
	for dep in $$(cat go.mod | grep openstack-k8s-operators | grep -vE -- 'indirect|openstack-operator|^replace' | awk '{print $$1}'); do \
		go get $$dep@$(BRANCH) ; \
	done
	for dep in $$(cat apis/go.mod | grep openstack-k8s-operators | grep -vE -- 'indirect|openstack-operator|^replace' | awk '{print $$1}'); do \
		cd ./apis && go get $$dep@$(BRANCH) && cd .. ; \
	done

.PHONY: tidy
tidy: ## Run go mod tidy on every mod file in the repo
	go mod tidy
	cd ./apis && go mod tidy

GOLANGCI_LINT_VERSION ?= v2.4.0
.PHONY: golangci-lint
golangci-lint:
	test -s $(LOCALBIN)/golangci-lint || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s $(GOLANGCI_LINT_VERSION)
	$(LOCALBIN)/golangci-lint run --fix

MAX_PROCS := 5
NUM_PROCS := $(shell expr $(shell nproc --ignore 2) / 2)
PROCS ?= $(shell if [ $(NUM_PROCS) -gt $(MAX_PROCS) ]; then echo $(MAX_PROCS); else echo $(NUM_PROCS); fi)
PROC_CMD = --procs $(PROCS)

.PHONY: test
test: manifests generate gowork fmt vet envtest ginkgo ginkgo-run ## Run ginkgo tests with dependencies.

.PHONY: ginkgo-run
ginkgo-run: ## Run ginkgo.
	source hack/export_related_images.sh && \
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) -v debug --bin-dir $(LOCALBIN) use $(ENVTEST_K8S_VERSION) -p path)" \
	OPERATOR_TEMPLATES="$(PWD)/templates" \
	$(GINKGO) --trace --cover --coverpkg=./pkg/...,./controllers/...,./apis/... --coverprofile cover.out --covermode=atomic ${PROC_CMD} $(GINKGO_ARGS) $(GINKGO_TESTS)

.PHONY: test-all
test-all: test golint golangci golangci-lint ## Run all tests.

.PHONY: cover
cover: test ## Run tests and display functional test coverage
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" go tool cover -html=cover.out

##@ Build

.PHONY: build
build: generate fmt vet ## Build manager binary.
	go build -o bin/manager main.go
	go build -o bin/operator cmd/operator/main.go

.PHONY: run
run: export METRICS_PORT?=8080
run: export HEALTH_PORT?=8081
run: export PPROF_PORT?=8082
run: export ENABLE_WEBHOOKS?=false
run: manifests generate fmt vet ## Run a controller from your host.
	/bin/bash hack/clean_local_webhook.sh
	source hack/export_related_images.sh && \
		go run ./main.go -metrics-bind-address ":$(METRICS_PORT)" -health-probe-bind-address ":$(HEALTH_PORT)" -pprof-bind-address ":$(PPROF_PORT)"

.PHONY: run-operator
run-operator: export METRICS_PORT?=8080
run-operator: export HEALTH_PORT?=8081
run: export PPROF_PORT?=8082
run-operator: export ENABLE_WEBHOOKS?=false
run-operator: export BASE_BINDATA?=bindata
run-operator: export OPERATOR_IMAGE_URL=${IMG}
run-operator: manifests generate fmt vet ## Run a controller from your host.
	source hack/export_related_images.sh && \
	source hack/export_operator_related_images.sh && \
	go run ./cmd/operator/main.go -metrics-bind-address ":$(METRICS_PORT)" -health-probe-bind-address ":$(HEALTH_PORT)" -pprof-bind-address ":$(PPROF_PORT)"

.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	podman build -t ${IMG} . ${DOCKER_BUILD_ARGS}

.PHONY: docker-push
docker-push: ## Push docker image with the manager.
	podman push ${IMG}

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> than the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx:  ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- docker buildx create --name project-v3-builder
	docker buildx use project-v3-builder
	- docker buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- docker buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | kubectl delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest
CRD_MARKDOWN ?= $(LOCALBIN)/crd-to-markdown
GINKGO ?= $(LOCALBIN)/ginkgo
GINKGO_TESTS ?= ./tests/... ./apis/client/... ./apis/core/... ./apis/dataplane/... ./pkg/...

KUTTL ?= $(LOCALBIN)/kubectl-kuttl

## Tool Versions
KUSTOMIZE_VERSION ?= v5.5.0 #(dprince: bumped to aquire new features like --load-restrictor)
CONTROLLER_TOOLS_VERSION ?= v0.18.0
CRD_MARKDOWN_VERSION ?= v0.0.3
KUTTL_VERSION ?= 0.17.0
GOTOOLCHAIN_VERSION ?= go1.24.0
OC_VERSION ?= 4.16.0

KUSTOMIZE_INSTALL_SCRIPT ?= "https://raw.githubusercontent.com/kubernetes-sigs/kustomize/master/hack/install_kustomize.sh"
.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || { curl -Ss $(KUSTOMIZE_INSTALL_SCRIPT) | bash -s -- $(subst v,,$(KUSTOMIZE_VERSION)) $(LOCALBIN); }

.PHONY: controller-gen
controller-gen: gowork $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: crd-to-markdown
crd-to-markdown: $(CRD_MARKDOWN) ## Download crd-to-markdown locally if necessary.
$(CRD_MARKDOWN): $(LOCALBIN)
	test -s $(LOCALBIN)/crd-to-markdown || GOBIN=$(LOCALBIN) go install github.com/clamoriniere/crd-to-markdown@$(CRD_MARKDOWN_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: ginkgo
ginkgo: $(GINKGO) ## Download ginkgo locally if necessary.
$(GINKGO): $(LOCALBIN)
	test -s $(LOCALBIN)/ginkgo || GOBIN=$(LOCALBIN) go install github.com/onsi/ginkgo/v2/ginkgo

.PHONY: kuttl-test
kuttl-test: ## Run kuttl tests
	$(LOCALBIN)/kubectl-kuttl test --config kuttl-test.yaml tests/kuttl/tests $(KUTTL_ARGS)

.PHONY: kuttl
kuttl: $(KUTTL) ## Download kubectl-kuttl locally if necessary.
$(KUTTL): $(LOCALBIN)
	test -s $(LOCALBIN)/kubectl-kuttl || curl -L -o $(LOCALBIN)/kubectl-kuttl https://github.com/kudobuilder/kuttl/releases/download/v$(KUTTL_VERSION)/kubectl-kuttl_$(KUTTL_VERSION)_linux_x86_64
	chmod +x $(LOCALBIN)/kubectl-kuttl

.PHONY: operator-sdk
OPERATOR_SDK ?= $(LOCALBIN)/operator-sdk
operator-sdk: ## Download operator-sdk locally if necessary.
ifeq (,$(wildcard $(OPERATOR_SDK)))
ifeq (, $(shell which operator-sdk 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPERATOR_SDK)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPERATOR_SDK) https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk_$${OS}_$${ARCH} ;\
	chmod +x $(OPERATOR_SDK) ;\
	}
else
OPERATOR_SDK = $(shell which operator-sdk)
endif
endif

.PHONY: bundle
bundle: manifests kustomize operator-sdk ## Generate bundle manifests and metadata, then validate generated files.
	$(OPERATOR_SDK) generate kustomize manifests -q
	cd config/operator/deployment/ && $(KUSTOMIZE) edit set image controller=$(IMG) && \
	$(KUSTOMIZE) edit add patch --kind Deployment --name openstack-operator-controller-operator --namespace system --patch "[{\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/1/env/0\", \"value\": {\"name\": \"OPENSTACK_RELEASE_VERSION\", \"value\": \"$(OPENSTACK_RELEASE_VERSION)\"}}]" && \
	$(KUSTOMIZE) edit add patch --kind Deployment --name openstack-operator-controller-operator --namespace system --patch "[{\"op\": \"replace\", \"path\": \"/spec/template/spec/containers/1/env/1\", \"value\": {\"name\": \"OPERATOR_IMAGE_URL\", \"value\": \"$(IMG)\"}}]"
	$(KUSTOMIZE) build config/operator --load-restrictor='LoadRestrictionsNone' | $(OPERATOR_SDK) generate bundle $(BUNDLE_GEN_FLAGS)
	$(OPERATOR_SDK) bundle validate ./bundle

.PHONY: bundle-build
bundle-build: bundle ## Build the bundle image.
	podman build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

.PHONY: bundle-push
bundle-push: ## Push the bundle image.
	$(MAKE) docker-push IMG=$(BUNDLE_IMG)

.PHONY: opm
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/v1.29.0/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

.PHONY: yq
yq: $(LOCALBIN) ## Download and install yq in local env
	test -s $(LOCALBIN)/yq || ( cd $(LOCALBIN) &&\
	wget https://github.com/mikefarah/yq/releases/latest/download/yq_linux_amd64.tar.gz -O - |\
	tar xz && mv yq_linux_amd64 $(LOCALBIN)/yq )

.PHONY: oc
oc: $(LOCALBIN) ## Download and install oc in local env
	test -s $(LOCALBIN)/oc || ( cd $(LOCALBIN) &&\
	wget https://mirror.openshift.com/pub/openshift-v4/x86_64/clients/ocp/$(OC_VERSION)/openshift-client-linux-$(OC_VERSION).tar.gz -O - |\
	tar xz)

# Build make variables to export for shell
MAKE_ENV := $(shell echo '$(.VARIABLES)' | awk -v RS=' ' '/^(IMAGE)|.*?(REGISTRY)$$/')
SHELL_EXPORT = $(foreach v,$(MAKE_ENV),$(v)='$($(v))')

# These images MUST exist in a registry and be pull-able.
BUNDLE_IMGS = "$(BUNDLE_IMG)$(shell $(SHELL_EXPORT) /bin/bash hack/pin-bundle-images.sh || echo bundle-fail-tag)"

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-index:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

# Build a catalog image by adding bundle images to an empty catalog using the operator package manager tool, 'opm'.
# This recipe invokes 'opm' in 'semver' bundle add mode. For more information on add modes, see:
# https://github.com/operator-framework/community-operators/blob/7f1438c/docs/packaging-operator.md#updating-your-existing-operator
.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	# FIXME: hardcoded bundle below should use go.mod pinned version for manila bundle
	$(OPM) index add --container-tool podman --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	$(MAKE) docker-push IMG=$(CATALOG_IMG)


# CI tools repo for running tests
CI_TOOLS_REPO := https://github.com/openstack-k8s-operators/openstack-k8s-operators-ci
CI_TOOLS_REPO_DIR = $(shell pwd)/CI_TOOLS_REPO
.PHONY: get-ci-tools
get-ci-tools:
	if [ -d  "$(CI_TOOLS_REPO_DIR)" ]; then \
		echo "Ci tools exists"; \
		pushd "$(CI_TOOLS_REPO_DIR)"; \
		git pull --rebase; \
		popd; \
	else \
		git clone $(CI_TOOLS_REPO) "$(CI_TOOLS_REPO_DIR)"; \
	fi

# Run go fmt against code
gofmt: get-ci-tools
	GOWORK=off $(CI_TOOLS_REPO_DIR)/test-runner/gofmt.sh
	GOWORK=off $(CI_TOOLS_REPO_DIR)/test-runner/gofmt.sh ./apis

# Run go vet against code
govet: get-ci-tools
	GOWORK=off $(CI_TOOLS_REPO_DIR)/test-runner/govet.sh
	GOWORK=off $(CI_TOOLS_REPO_DIR)/test-runner/govet.sh  ./apis

# Run go test against code
gotest: test

# Run golangci-lint test against code
golangci: get-ci-tools
	GOWORK=off $(CI_TOOLS_REPO_DIR)/test-runner/golangci.sh
	GOWORK=off $(CI_TOOLS_REPO_DIR)/test-runner/golangci.sh  ./apis

# Run go lint against code
golint: get-ci-tools
	GOWORK=off PATH=$(GOBIN):$(PATH); $(CI_TOOLS_REPO_DIR)/test-runner/golint.sh
	GOWORK=off PATH=$(GOBIN):$(PATH); $(CI_TOOLS_REPO_DIR)/test-runner/golint.sh ./apis

.PHONY: gowork
gowork: ## Generate go.work file to support our multi module repository
	test -f go.work || GOTOOLCHAIN=$(GOTOOLCHAIN_VERSION) go work init
	go work use .
	go work use ./apis
	go work sync

.PHONY: operator-lint
operator-lint: gowork ## Runs operator-lint
	GOBIN=$(LOCALBIN) go install github.com/gibizer/operator-lint@v0.3.0
	go vet -vettool=$(LOCALBIN)/operator-lint ./... ./apis/...

# Used for webhook testing
# The configure_local_webhook.sh script below will remove any OLM webhooks
# for the operator and also scale its deployment replicas down to 0 so that
# the operator can run locally.
# We will attempt to catch SIGINT/SIGTERM and clean up the local webhooks,
# but it may be necessary to manually run ./hack/clean_local_webhook.sh
# before deploying with OLM again for other untrappable signals.
SKIP_CERT ?=false
.PHONY: run-with-webhook
run-with-webhook: export METRICS_PORT?=8080
run-with-webhook: export HEALTH_PORT?=8081
run-with-webhook: export PPROF_PORT?=8082
run-with-webhook: export WEBHOOK_PORT?=9443
run-with-webhook: manifests generate fmt vet ## Run a controller from your host.
	/bin/bash hack/run_with_local_webhook.sh

.PHONY: webhook-cleanup
webhook-cleanup:
	/bin/bash hack/clean_local_webhook.sh

CRD_SCHEMA_CHECKER_VERSION ?= release-4.16

PHONY: crd-schema-check
crd-schema-check: manifests
	INSTALL_DIR=$(LOCALBIN) CRD_SCHEMA_CHECKER_VERSION=$(CRD_SCHEMA_CHECKER_VERSION) hack/build-crd-schema-checker.sh
	INSTALL_DIR=$(LOCALBIN) BASE_REF="$${PULL_BASE_SHA:-$(BRANCH)}" hack/crd-schema-checker.sh
