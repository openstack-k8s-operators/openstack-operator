# openstack-operator

[![CodeQL](https://github.com/openstack-k8s-operators/openstack-operator/actions/workflows/codeql.yml/badge.svg)](https://github.com/openstack-k8s-operators/openstack-operator/actions/workflows/codeql.yml)
[![CRD sync check main](https://github.com/openstack-k8s-operators/openstack-operator/actions/workflows/crd-sync-check.yaml/badge.svg)](https://github.com/openstack-k8s-operators/openstack-operator/actions/workflows/crd-sync-check.yaml)
[![CRD sync check olive](https://github.com/openstack-k8s-operators/openstack-operator/actions/workflows/crd-sync-check-olive.yaml/badge.svg)](https://github.com/openstack-k8s-operators/openstack-operator/actions/workflows/crd-sync-check-olive.yaml)

This is the primary operator for OpenStack. It is a "meta" operator, meaning it
serves to coordinate the other operators for OpenStack by watching and configuring
their CustomResources (CRs). Additionally installing this operator will automatically
install all required operator dependencies for installing/managing OpenStack.

## Description

This project is built, modeled, and maintained with [operator-sdk] (https://github.com/operator-framework/operator-sdk).

## Getting Started
You’ll need a Kubernetes cluster to run against. You can use [KIND](https://sigs.k8s.io/kind) to get a local cluster for testing, or run against a remote cluster.
**Note:** Your controller will automatically use the current context in your kubeconfig file (i.e. whatever cluster `kubectl cluster-info` shows).

### Running on the cluster
1. Install Instances of Custom Resources:

```sh
kubectl apply -f config/samples/
```

2. Build and push your image to the location specified by `IMG`:

```sh
make docker-build docker-push IMG=<some-registry>/openstack-operator:tag
```

3. Deploy the controller to the cluster with the image specified by `IMG`:

```sh
make deploy IMG=<some-registry>/openstack-operator:tag
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

### Building your own bundle, index images
The OpenStack operator uses multiple bundles to minimize the number of
deployment artifacts we have in the OLM catalog while also providing enough
space for our CRs (this is a big project). As such the build order for local
bundles is a bit different than normal.

1. Run make:bundle. This pins down dependencies to version used in the go.mod and
 and also string replaces the URL for any dependant bundles (storage, etc) that
 we will build below. Additionally a dependency.yaml is added to the generated bundle
 so that we require any dependencies. This sets the stage for everything below.

```sh
make bundle
```

2. Run dep-bundle-build-push. This creates any *dependency* bundles required by this project.
It builds and pushes them to a registry as this is required to be able to build the main
bundle.

```sh
make dep-bundle-build-push
```

3. Run bundle-build. This will execute podman to build the bundle.Dockerfile.

```sh
make bundle-build
```

4. Run bundle-push. This pushes the resulting bundle image to the registry.

```sh
make bundle-push
```

5. Run catalog-build.  At this point you can generate your index image so that it contains both of the above bundle images. Because we use dependencies in the openstack-operator's main bundle it will
 automatically install the CSV contained in the dependant (storage, etc) bundle.

```sh
make catalog-build
```

6. Run catalog-push. Push the catalog to your registry.

```sh
make catalog-push
```

### Uninstall CRDs
To delete the CRDs from the cluster:

```sh
make uninstall
```

### Undeploy controller
UnDeploy the controller to the cluster:

```sh
make undeploy
```

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

### How it works
This project aims to follow the Kubernetes [Operator pattern](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)

It uses [Controllers](https://kubernetes.io/docs/concepts/architecture/controller/)
which provides a reconcile function responsible for synchronizing resources untile the desired state is reached on the cluster

### Test It Out
1. Install the CRDs into the cluster:

```sh
make install
```

2. Run your controller (this will run in the foreground, so switch to a new terminal if you want to leave it running):

```sh
make run
```

**NOTE:** You can also run this in one step by running: `make install run`

### Modifying the API definitions
If you are editing the API definitions, generate the manifests such as CRs or CRDs using:

```sh
make manifests
```

**NOTE:** Run `make --help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
