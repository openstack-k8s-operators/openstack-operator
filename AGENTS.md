# AGENTS.md - openstack-operator

## Project overview

openstack-operator is a Kubernetes meta-operator that orchestrates the entire
[OpenStack](https://www.openstack.org/) control plane and data plane lifecycle
on OpenShift/Kubernetes. It is the top-level "umbrella" operator in the
[openstack-k8s-operators](https://github.com/openstack-k8s-operators) project,
managing all other OpenStack service operators (Keystone, Nova, Neutron, Cinder,
Glance, etc.) and coordinating data plane provisioning via Ansible.

Key domain concepts: **control plane** (all OpenStack services deployed as
pods), **data plane** (compute/network nodes provisioned via Ansible),
**node sets** (groups of data plane nodes), **deployments** (Ansible-based
provisioning runs), **services** (data plane service definitions).

### API groups

| API Group | Version |
|-----------|---------|
| `core.openstack.org` | `v1beta1` |
| `client.openstack.org` | `v1beta1` |
| `dataplane.openstack.org` | `v1beta1` |
| `operator.openstack.org` | `v1beta1` |

## Tech stack

| Layer | Technology |
|-------|------------|
| Language | Go (modules, multi-module workspace via `go.work`) |
| Scaffolding | [Kubebuilder v4](https://book.kubebuilder.io/) + [Operator SDK](https://sdk.operatorframework.io/) |
| CRD generation | controller-gen (DeepCopy, CRDs, RBAC, webhooks) |
| Config management | Kustomize |
| Packaging | OLM bundle |
| Testing | Ginkgo/Gomega + envtest (functional), KUTTL (integration) |
| Linting | golangci-lint (`.golangci.yaml`) |
| CI | Zuul (`zuul.d/`), Prow (`.ci-operator.yaml`), GitHub Actions |

## Custom Resources

CRD types and sub-resources are documented in
[docs/assemblies/ctlplane_resources.adoc](docs/assemblies/ctlplane_resources.adoc) (control plane) and
[docs/assemblies/dataplane_resources.adoc](docs/assemblies/dataplane_resources.adoc) (data plane).

The `OpenStackControlPlane` and `OpenStackDataPlaneNodeSet` CRs have defaulting
and validating admission webhooks.

## Directory structure

| Directory | Contents |
|-----------|----------|
| `api/{core,client,dataplane,operator}/v1beta1/` | CRD types, conditions, webhook markers — one subdir per API group |
| `internal/controller/` | Reconcilers organized by domain: `core/`, `client/`, `dataplane/`, `operator/` |
| `internal/openstack/` | Per-service control plane resource builders (one file per OpenStack service) |
| `internal/dataplane/` | Data plane logic: Ansible inventory, IPAM, certificates, baremetal, deployments |
| `internal/webhook/` | Webhook implementations organized by domain |
| `bindata/` | Embedded CRDs, RBAC, and manifests synced from dependent operators |
| `test/functional/` | envtest-based Ginkgo/Gomega tests, subdirs: `ctlplane/`, `dataplane/` |
| `docs/` | Architecture and configuration documentation ([design](docs/assemblies/design.adoc)) |

## Build commands

After modifying Go code, always run: `make generate manifests fmt vet`.

## Code style guidelines

- Follow standard openstack-k8s-operators conventions and lib-common patterns.
- Use `lib-common` modules for conditions, endpoints, TLS, storage, and other
  cross-cutting concerns rather than re-implementing them.
- CRD types are organized by API group under `api/{core,client,dataplane,operator}/v1beta1/`.
  Controller logic is organized under `internal/controller/{core,client,dataplane,operator}/`.
- Per-service resource builders (for the control plane) live in
  `internal/openstack/` with one file per OpenStack service.
- Data plane logic (inventory, IPAM, certificates, Ansible execution) lives in
  `internal/dataplane/`.
- Embedded data from dependent operators is managed in `bindata/`.
- Webhook logic is split between the kubebuilder markers in the API type files
  and the implementation in `internal/webhook/`.

## Testing

- Functional tests use the envtest framework with Ginkgo/Gomega and live in
  `test/functional/ctlplane/` (control plane) and `test/functional/dataplane/`
  (data plane).
- KUTTL integration tests live in `test/kuttl/`.
- Run all functional tests: `make test`.
- When adding a new field or feature, add corresponding test cases in the
  appropriate `test/functional/` subdirectory.

## Key dependencies

- [lib-common](https://github.com/openstack-k8s-operators/lib-common): shared modules for conditions, endpoints, database, TLS, secrets, Ansible, etc.
- All [openstack-k8s-operators](https://github.com/openstack-k8s-operators) service operators (barbican, cinder, designate, glance, heat, horizon, infra, ironic, keystone, manila, mariadb, neutron, nova, octavia, ovn, placement, swift, telemetry, watcher).
- [gophercloud](https://github.com/gophercloud/gophercloud): Go OpenStack SDK.
- [Control plane docs](docs/ctlplane.adoc) and [Data plane docs](docs/dataplane.adoc): architecture and configuration documentation.
