#!/usr/bin/env bash
set -ex pipefail

BAREMETAL=docs/assemblies/ref_example-OpenStackDataPlaneNodeSet-CR-for-bare-metal-nodes.adoc
FOOTER=$(sed '0,/----/d' $BAREMETAL | sed -e '0,/----/d')
sed -i '/----/q' $BAREMETAL
sed -i 's/preprovisioned/baremetal/' config/samples/dataplane/no_vars_from/kustomization.yaml
$LOCALBIN/oc kustomize --load-restrictor LoadRestrictionsNone config/samples/dataplane/no_vars_from | $LOCALBIN/yq ' select(.kind == "OpenStackDataPlaneNodeSet")' >> $BAREMETAL
sed -i 's/\/baremetal/\/preprovisioned/' config/samples/dataplane/no_vars_from/kustomization.yaml
echo -e "----\n$FOOTER" >> $BAREMETAL

COUNT=1
CALLOUTS=(
    "baremetalSetTemplate"
    "env"
    "networkAttachments"
    "nodeTemplate"
    "ansibleUser"
    "ansibleVars"
    "edpm_network_config_template"
    "ansibleSSHPrivateKeySecret"
    "networks"
    "edpm-compute-0"
    "services"
)
for callout in "${CALLOUTS[@]}";do
    sed -i "/$callout:/ s/$/ #<$COUNT>/" $BAREMETAL
    COUNT=$((COUNT + 1))
done


PREPROVISIONED=docs/assemblies/ref_example-OpenStackDataPlaneNodeSet-CR-for-preprovisioned-nodes.adoc
FOOTER=$(sed '0,/----/d' $PREPROVISIONED | sed -e '0,/----/d')
sed -i '/----/q' $PREPROVISIONED
$LOCALBIN/oc kustomize --load-restrictor LoadRestrictionsNone config/samples/dataplane/no_vars_from | $LOCALBIN/yq ' select(.kind == "OpenStackDataPlaneNodeSet")' >> $PREPROVISIONED
echo -e "----\n$FOOTER" >> $PREPROVISIONED

COUNT=1
CALLOUTS=(
    "env"
    "networkAttachments"
    "nodeTemplate"
    "ansibleUser"
    "ansibleVars"
    "edpm_network_config_template"
    "ansibleSSHPrivateKeySecret"
    "edpm-compute-0"
    "preProvisioned"
    "services"
)
for callout in "${CALLOUTS[@]}";do
    sed -i "/$callout:/ s/$/ #<$COUNT>/" $PREPROVISIONED
    COUNT=$((COUNT + 1))
done
