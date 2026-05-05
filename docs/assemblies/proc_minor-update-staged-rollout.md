# Performing a Staged Update of OpenStack

A minor update of an OpenStack environment proceeds through a fixed sequence of stages.
By default the update runs all stages automatically. The
`core.openstack.org/update-target-stage` annotation on the `OpenStackVersion` CR lets you
pause the update after any stage so you can validate the environment, coordinate maintenance
windows, or advance one stage at a time.

When a pause is active, the `OpenStackVersion` controller sets the next stage's condition to
`False` with a gated message, and the `OpenStackControlPlane` controller skips reconciling
control-plane components for stages beyond the annotation target until you advance or remove
the annotation.

## Examples to use staged rollouts

- You want to verify OVN networking is healthy before allowing the rest of the update to
  proceed.
- Your organisation requires a sign-off after each major component is updated.
- You are performing the update in phases across a maintenance window and need to stop at a
  known safe point.

## Understanding the update pipeline

The update always runs stages in this order. Each stage must complete before the next one
starts.

| Stage              | What gets updated                       | Requires manual action?                                 |
|--------------------|-----------------------------------------|---------------------------------------------------------|
| `ovn-controlplane` | OVN control plane images                | No                                                      |
| `ovn-dataplane`    | OVN controller data plane images on compute nodes  | **Yes** — create an OVN `OpenStackDataPlaneDeployment`  |
| `rabbitmq`         | RabbitMQ images                         | No                                                      |
| `mariadb`          | MariaDB/Galera images                   | No                                                      |
| `memcached`        | Memcached images                        | No                                                      |
| `keystone`         | Keystone API images                     | No                                                      |
| `controlplane`     | All remaining control-plane services    | No                                                      |
| *(completion)*     | Data-plane services on compute nodes    | **Yes** — create a full `OpenStackDataPlaneDeployment`  |

> **Note:** Two stages require you to create an `OpenStackDataPlaneDeployment` manually.
> The `ovn-dataplane` stage and the final data-plane completion step do not self-drive —
> the controller waits for the corresponding deployment to finish before advancing.
> See [Required manual deployments](#required-manual-deployments) below.

## Prerequisites

- A running cluster with a deployed OpenStack environment.
- `OpenStackControlPlane` and `OpenStackVersion` are both `Ready`.
- `status.deployedVersion` is set on the `OpenStackVersion` CR.
- A newer version is available: `status.availableVersion` differs from
  `status.deployedVersion`.

The examples below use:
- Namespace: `openstack`
- `OpenStackVersion` CR name: `openstack`

---

## Performing a fully staged update

The recommended approach is to set the annotation to the first stage before bumping
`targetVersion`, then advance the annotation one stage at a time after you have validated
each step. If you start the update without the annotation, you cannot add it later at a
stage earlier than rollout progress already reached; set a later stage or remove the
annotation to run to completion.

### Step 1 — Confirm an update is available

```bash
oc get openstackversion openstack -n openstack \
  -o jsonpath='Available: {.status.availableVersion}  Deployed: {.status.deployedVersion}{"\n"}'
```

Note the `availableVersion` value — this is `<new-version>` in the commands below.

### Step 2 — Set the initial pause point

Choose the stage after which you want the first pause. To pause after OVN control-plane:

```bash
oc annotate openstackversion openstack \
  core.openstack.org/update-target-stage=ovn-controlplane \
  -n openstack
```

### Step 3 — Start the update

```bash
oc patch openstackversion openstack -n openstack \
  --type=merge -p '{"spec":{"targetVersion":"<new-version>"}}'
```

The update begins immediately. The controller runs the `ovn-controlplane` stage and then
pauses. The `MinorUpdateOVNControlplane` condition becomes `True` and the
`MinorUpdateOVNDataplane` condition shows:

```
Minor update progression stopped after stage: ovn-controlplane. Set annotation to any stage after ovn-controlplane to resume OpenStack update or remove the annotation to run to completion.
```

### Step 4 — Validate and advance stage by stage

After each pause, check the environment is healthy, then advance to the next stage.

#### Checking the current update status

```bash
oc get openstackversion openstack -n openstack \
  -o jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\t"}{.message}{"\n"}{end}' \
  | grep MinorUpdate
```

Completed stages show `True`. The currently blocked stage shows `False` with a message
telling you which stage just finished and what to set next.

#### Advancing to the next stage

Update the annotation value to the stage you want to run next. For example, after
validating the OVN control-plane, advance to `ovn-dataplane`:

> **Before advancing to `ovn-dataplane`**, create the OVN dataplane deployment first —
> see [Required manual deployments](#required-manual-deployments).

```bash
oc annotate openstackversion openstack \
  core.openstack.org/update-target-stage=ovn-dataplane \
  --overwrite -n openstack
```

Continue advancing through the remaining stages as needed:

| To run through…    | Set annotation to… |
|--------------------|--------------------|
| RabbitMQ           | `rabbitmq`         |
| MariaDB            | `mariadb`          |
| Memcached          | `memcached`        |
| Keystone           | `keystone`         |
| Full control-plane | `controlplane`     |

### Step 5 — Complete the update

When you are ready to run the final data-plane update on compute nodes, first create the
full dataplane deployment (see [Required manual deployments](#required-manual-deployments)),
then remove the annotation to let the update finish:

```bash
oc annotate openstackversion openstack \
  core.openstack.org/update-target-stage- \
  -n openstack
```

> The trailing `-` removes the annotation entirely.

The controller runs the remaining stages and, once complete, sets
`status.deployedVersion` to the new version.

### Step 6 — Confirm completion

```bash
oc get openstackversion openstack -n openstack \
  -o jsonpath='{.status.deployedVersion}'
```

The output should show `<new-version>`.

---

## Required manual deployments

Two stages in the process do not self-start. You must create an
`OpenStackDataPlaneDeployment` before (or at the same time as) advancing past each of them.

### OVN data-plane deployment

Required before the `ovn-dataplane` stage can complete. This deployment updates only the
OVN-related services on compute nodes.

```yaml
apiVersion: dataplane.openstack.org/v1beta1
kind: OpenStackDataPlaneDeployment
metadata:
  name: edpm-deployment-ovn-update
  namespace: openstack
spec:
  nodeSets:
    - openstack-edpm-ipam
  servicesOverride:
    - ovn
```

```bash
oc apply -f edpm-deployment-ovn-update.yaml
```

### Full data-plane update deployment

Required before the final completion step can finish. This deployment updates all remaining
services on compute nodes.

```yaml
apiVersion: dataplane.openstack.org/v1beta1
kind: OpenStackDataPlaneDeployment
metadata:
  name: edpm-deployment-update
  namespace: openstack
spec:
  nodeSets:
    - openstack-edpm-ipam
  servicesOverride:
    - update
```

```bash
oc apply -f edpm-deployment-update.yaml
```

---

## Pausing a running update

If you need to pause an update that is already in progress, add the annotation at any time.
The controller completes whichever stage is currently running, then stops after the stage you
named.

```bash
oc annotate openstackversion openstack \
  core.openstack.org/update-target-stage=<stage> \
  -n openstack
```

Replace `<stage>` with the name of the last stage you want to run before pausing.

---

## Running the full update without pausing

If you do not need staged control, omit the annotation entirely and let the controller run
all stages automatically. You still need to create both dataplane deployments at the right
time:

1. Create the OVN dataplane deployment before or immediately after starting the update.
2. Create the dataplane update deployment before the final completion step.

```bash
oc patch openstackversion openstack -n openstack \
  --type=merge -p '{"spec":{"targetVersion":"<new-version>"}}'
```

---

## Troubleshooting

### The update appears stuck

Check whether the blocked condition message contains `"stopped after stage"`. If it does,
the update is intentionally paused — advance or remove the annotation to continue.

```bash
oc get openstackversion openstack -n openstack -o json | \
  jq '[.status.conditions[] | select(.reason=="Requested" and .status=="False")]'
```

### `MinorUpdateOVNDataplane` or `MinorUpdateDataplane` stays `False`

These stages wait for an `OpenStackDataPlaneDeployment` to complete. Check whether the
required deployment exists and is running:

```bash
oc get openstackdataplanedeployment -n openstack
```

If the deployment is missing, create it as described in
[Required manual deployments](#required-manual-deployments).

### Checking overall update progress

```bash
watch -n 5 "oc get openstackversion openstack -n openstack \
  -o jsonpath='{range .status.conditions[*]}{.type}{\"\t\"}{.status}{\"\t\"}{.message}{\"\n\"}{end}' \
  | grep MinorUpdate"
```
