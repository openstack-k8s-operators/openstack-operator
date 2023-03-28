#!/bin/bash
set -ex

TMPDIR=${TMPDIR:-"/tmp/k8s-webhook-server/serving-certs"}
SKIP_CERT=${SKIP_CERT:-false}
CRC_IP=${CRC_IP:-$(/sbin/ip -o -4 addr list crc | awk '{print $4}' | cut -d/ -f1)}

#Open 9443
sudo firewall-cmd --zone=libvirt --add-port=9443/tcp
sudo firewall-cmd --runtime-to-permanent

# Generate the certs and the ca bundle
if [ "$SKIP_CERT" = false ] ; then
    mkdir -p ${TMPDIR}
    rm -rf ${TMPDIR}/* || true

    openssl req -newkey rsa:2048 -days 3650 -nodes -x509 \
    -subj "/CN=${HOSTNAME}" \
    -addext "subjectAltName = IP:${CRC_IP}" \
    -keyout ${TMPDIR}/tls.key \
    -out ${TMPDIR}/tls.crt

    cat ${TMPDIR}/tls.crt ${TMPDIR}/tls.key | base64 -w 0 > ${TMPDIR}/bundle.pem

fi

CA_BUNDLE=`cat ${TMPDIR}/bundle.pem`

# Patch the webhook(s)
cat >> ${TMPDIR}/patch_webhook_configurations.yaml <<EOF_CAT
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: vopenstackcontrolplane.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/validate-core-openstack-org-v1beta1-openstackcontrolplane
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: vopenstackcontrolplane.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - core.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackcontrolplanes
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mopenstackcontrolplane.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/mutate-core-openstack-org-v1beta1-openstackcontrolplane
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: mopenstackcontrolplane.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - core.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackcontrolplanes
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
EOF_CAT

oc apply -n openstack -f ${TMPDIR}/patch_webhook_configurations.yaml
