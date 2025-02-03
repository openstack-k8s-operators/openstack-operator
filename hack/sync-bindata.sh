#!/bin/bash

# extract select data from bundles:
#  -CSV's
#  -TODO: role data
set -ex

BINDATA_GIT_ADD=${BINDATA_GIT_ADD:-""}
OUT_DATA=bindata
EXTRACT_DIR=tmp/bindata
LOCAL_BINARIES=${LOCAL_BINARIES:?}

mkdir -p "$EXTRACT_DIR"
mkdir -p "$OUT_DATA/crds"

function extract_bundle {
    local IN_DIR=$1
    local OUT_DIR=$2
    for X in $(file ${IN_DIR}/* | grep gzip | cut -f 1 -d ':'); do
        tar xvf $X -C ${OUT_DIR}/;
    done
}


function extract_webhooks {
local CSV_FILENAME=$1
local OPERATOR_NAME=$2
local TYPE=$3

cat $CSV_FILENAME | $LOCAL_BINARIES/yq -r ".spec.webhookdefinitions.[] | select(.type == \"$TYPE\")" | \
    sed -e '/^containerPort:/d' | \
    sed -e '/^deploymentName:/d' | \
    sed -e '/^targetPort:/d' | \
    sed -e '/^type:/d' | \
    sed -e 's|^|  |' | sed -e 's|.*admissionReviewVersions:|- admissionReviewVersions:|' | \
    sed -e 's|.*generateName:|  name:|' | \
    sed -e 's|    - v1|  - v1|' | \
    sed -e "s|.*webhookPath:|  clientConfig:\n    service:\n      name: ${OPERATOR_NAME}-webhook-service\n      namespace: '{{ .OperatorNamespace }}'\n      path:|"

}


function write_webhooks {
local CSV_FILENAME=$1
local OPERATOR_NAME=$2

MUTATING_WEBHOOKS=$(extract_webhooks "$CSV_FILENAME" "$OPERATOR_NAME" "MutatingAdmissionWebhook")
VALIDATING_WEBHOOKS=$(extract_webhooks "$CSV_FILENAME" "$OPERATOR_NAME" "ValidatingAdmissionWebhook")

cat > operator/$OPERATOR_NAME-webhooks.yaml <<EOF_CAT
apiVersion: v1
kind: Service
metadata:
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: openstack-operator
    app.kubernetes.io/instance: webhook-service
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: service
    app.kubernetes.io/part-of: $OPERATOR_NAME
  name: $OPERATOR_NAME-webhook-service
  namespace: '{{ .OperatorNamespace }}'
spec:
  ports:
  - port: 443
    protocol: TCP
    targetPort: 9443
  selector:
    openstack.org/operator-name: ${OPERATOR_NAME//-operator}
---
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  labels:
    app.kubernetes.io/component: certificate
    app.kubernetes.io/created-by: openstack-operator
    app.kubernetes.io/instance: serving-cert
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: certificate
    app.kubernetes.io/part-of: $OPERATOR_NAME
  name: $OPERATOR_NAME-serving-cert
  namespace: '{{ .OperatorNamespace }}'
spec:
  dnsNames:
  - $OPERATOR_NAME-webhook-service.{{ .OperatorNamespace }}.svc
  - $OPERATOR_NAME-webhook-service.{{ .OperatorNamespace }}.svc.cluster.local
  issuerRef:
    kind: Issuer
    name: $OPERATOR_NAME-selfsigned-issuer
  secretName: $OPERATOR_NAME-webhook-server-cert
---
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  labels:
    app.kubernetes.io/component: certificate
    app.kubernetes.io/created-by: openstack-operator
    app.kubernetes.io/instance: selfsigned-issuer
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: issuer
    app.kubernetes.io/part-of: $OPERATOR_NAME
  name: $OPERATOR_NAME-selfsigned-issuer
  namespace: '{{ .OperatorNamespace }}'
spec:
  selfSigned: {}
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  annotations:
    cert-manager.io/inject-ca-from: '{{ .OperatorNamespace }}/$OPERATOR_NAME-serving-cert'
  creationTimestamp: null
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: openstack-operator
    app.kubernetes.io/instance: mutating-webhook-configuration
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: mutatingwebhookconfiguration
    app.kubernetes.io/part-of: $OPERATOR_NAME
  name: $OPERATOR_NAME-mutating-webhook-configuration
webhooks:
${MUTATING_WEBHOOKS}
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  annotations:
    cert-manager.io/inject-ca-from: '{{ .OperatorNamespace }}/$OPERATOR_NAME-serving-cert'
  creationTimestamp: null
  labels:
    app.kubernetes.io/component: webhook
    app.kubernetes.io/created-by: openstack-operator
    app.kubernetes.io/instance: validating-webhook-configuration
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/name: validatingwebhookconfiguration
    app.kubernetes.io/part-of: $OPERATOR_NAME
  name: $OPERATOR_NAME-validating-webhook-configuration
webhooks:
${VALIDATING_WEBHOOKS}
EOF_CAT

}

for BUNDLE in $(hack/pin-bundle-images.sh | tr "," " "); do
    skopeo copy "docker://$BUNDLE" dir:${EXTRACT_DIR}/tmp;
    extract_bundle "${EXTRACT_DIR}/tmp" "${OUT_DATA}/"
done

cd "$OUT_DATA"
# copy CRDS into crds basedir
grep -l CustomResourceDefinition manifests/* | xargs -I % sh -c 'cp % ./crds/'

# extract role, clusterRole, and deployment from CSV's
for X in $(ls manifests/*clusterserviceversion.yaml); do
        OPERATOR_NAME=$(echo $X | sed -e "s|manifests\/\([^\.]*\)\..*|\1|")
        echo $OPERATOR_NAME
        LEADER_ELECTION_ROLE_RULES=$(cat $X | $LOCAL_BINARIES/yq -r .spec.install.spec.permissions | sed -e 's|- rules:|rules:|' | sed -e 's|    ||' | sed -e '/  serviceAccountName.*/d'
)
        CLUSTER_ROLE_RULES=$(cat $X | $LOCAL_BINARIES/yq -r .spec.install.spec.clusterPermissions| sed -e 's|- rules:|rules:|' | sed -e 's|    ||' | sed -e '/  serviceAccountName.*/d'
)

if [[ "$OPERATOR_NAME" == "infra-operator" ]]; then
    write_webhooks "$X" "$OPERATOR_NAME"
fi

mkdir -p rbac
cat > rbac/$OPERATOR_NAME-rbac.yaml <<EOF_CAT
# NOTE: this file is automatically generated by hack/sync-bindata.sh!
#
apiVersion: v1
kind: ServiceAccount
metadata:
  name: ${OPERATOR_NAME}-controller-manager
  namespace: '{{ .OperatorNamespace }}'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: ${OPERATOR_NAME}-leader-election-role
  namespace: '{{ .OperatorNamespace }}'
${LEADER_ELECTION_ROLE_RULES}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  creationTimestamp: null
  name: ${OPERATOR_NAME}-manager-role
${CLUSTER_ROLE_RULES}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: ${OPERATOR_NAME}-leader-election-rolebinding
  namespace: '{{ .OperatorNamespace }}'
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: ${OPERATOR_NAME}-leader-election-role
subjects:
- kind: ServiceAccount
  name: ${OPERATOR_NAME}-controller-manager
  namespace: '{{ .OperatorNamespace }}'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${OPERATOR_NAME}-manager-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${OPERATOR_NAME}-manager-role
subjects:
- kind: ServiceAccount
  name: ${OPERATOR_NAME}-controller-manager
  namespace: '{{ .OperatorNamespace }}'
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: ${OPERATOR_NAME}-proxy-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${OPERATOR_NAME}-proxy-role
subjects:
- kind: ServiceAccount
  name: ${OPERATOR_NAME}-controller-manager
  namespace: '{{ .OperatorNamespace }}'
---
apiVersion: v1
kind: Service
metadata:
  labels:
    control-plane: controller-manager
  name: ${OPERATOR_NAME}-controller-manager-metrics-service
  namespace: '{{ .OperatorNamespace }}'
spec:
  ports:
  - name: https
    port: 8443
    protocol: TCP
    targetPort: https
  selector:
    openstack.org/operator-name: ${OPERATOR_NAME}
EOF_CAT
done

# generate config/operator/manager_operator_images.yaml
cat > ../config/operator/manager_operator_images.yaml <<EOF_CAT
# NOTE: this file is automatically generated by hack/sync-bindata.sh!
#
# This patch inject custom ENV settings to the manager container
# Used to set our operator locations
apiVersion: apps/v1
kind: Deployment
metadata:
  name: openstack-operator-controller-operator
  namespace: system
spec:
  template:
    spec:
      containers:
      - name: operator
        env:
EOF_CAT

cat > ../hack/export_operator_related_images.sh <<EOF_CAT
# NOTE: this file is automatically generated by hack/sync-bindata.sh!

EOF_CAT

for X in $(ls manifests/*clusterserviceversion.yaml); do
        OPERATOR_NAME=$(echo $X | sed -e "s|manifests\/\([^\.]*\)\..*|\1|" | sed -e "s|-|_|g" | tr '[:lower:]' '[:upper:]' )
        echo $OPERATOR_NAME
        if [[ $OPERATOR_NAME == "RABBITMQ_CLUSTER_OPERATOR" ]]; then
            IMAGE=$(cat $X | $LOCAL_BINARIES/yq -r .spec.install.spec.deployments[0].spec.template.spec.containers[0].image)
        else
            IMAGE=$(cat $X | $LOCAL_BINARIES/yq -r .spec.install.spec.deployments[0].spec.template.spec.containers[1].image)
        fi
        echo $IMAGE


cat >> ../config/operator/manager_operator_images.yaml <<EOF_CAT
        - name: RELATED_IMAGE_${OPERATOR_NAME}_MANAGER_IMAGE_URL
          value: ${IMAGE}
EOF_CAT

cat >> ../hack/export_operator_related_images.sh <<EOF_CAT
export RELATED_IMAGE_${OPERATOR_NAME}_MANAGER_IMAGE_URL=${IMAGE}
EOF_CAT

done

cd ..

# cleanup
rm -Rf "$EXTRACT_DIR"
rm -Rf "$OUT_DATA/manifests"
rm -Rf "$OUT_DATA/metadata"
rm -Rf "$OUT_DATA/tests"

# stage new files for addition via git
if [[ "$BINDATA_GIT_ADD" == "true" ]]; then
    git ls-files -o --exclude-standard | xargs --no-run-if-empty git add
fi
