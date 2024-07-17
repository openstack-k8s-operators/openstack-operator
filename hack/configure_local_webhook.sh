#!/bin/bash
set -ex

TMPDIR=${TMPDIR:-"/tmp/k8s-webhook-server/serving-certs"}
SKIP_CERT=${SKIP_CERT:-false}
CRC_IP=${CRC_IP:-$(/sbin/ip -o -4 addr list crc | awk '{print $4}' | cut -d/ -f1)}

#Open 9443
sudo firewall-cmd --zone=libvirt --add-port=9443/tcp
sudo firewall-cmd --runtime-to-permanent

# Generate the certs and the ca bundle
if [ "$SKIP_CERT" = false ]; then
	mkdir -p ${TMPDIR}
	rm -rf ${TMPDIR}/* || true

	openssl req -newkey rsa:2048 -days 3650 -nodes -x509 \
		-subj "/CN=${HOSTNAME}" \
		-addext "subjectAltName = IP:${CRC_IP}" \
		-keyout ${TMPDIR}/tls.key \
		-out ${TMPDIR}/tls.crt

	cat ${TMPDIR}/tls.crt ${TMPDIR}/tls.key | base64 -w 0 >${TMPDIR}/bundle.pem

fi

CA_BUNDLE=$(cat ${TMPDIR}/bundle.pem)

# Patch the webhook(s)
cat >>${TMPDIR}/patch_webhook_configurations.yaml <<EOF_CAT
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
kind: ValidatingWebhookConfiguration
metadata:
  name: vopenstackclient.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/validate-client-openstack-org-v1beta1-openstackclient
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: vopenstackclient.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - client.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackclients
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
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mopenstackclient.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/mutate-client-openstack-org-v1beta1-openstackclient
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: mopenstackclient.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - client.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackclients
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: vopenstackdataplanenodeset.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/validate-dataplane-openstack-org-v1beta1-openstackdataplanenodeset
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: vopenstackdataplanenodeset.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - dataplane.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackdataplanenodesets
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mopenstackdataplanenodeset.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/mutate-dataplane-openstack-org-v1beta1-openstackdataplanenodeset
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: mopenstackdataplanenodeset.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - dataplane.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackdataplanenodesets
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: vopenstackdataplanedeployment.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/validate-dataplane-openstack-org-v1beta1-openstackdataplanedeployment
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: vopenstackdataplanedeployment.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - dataplane.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackdataplanedeployments
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mopenstackdataplanedeployment.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/mutate-dataplane-openstack-org-v1beta1-openstackdataplanedeployment
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: mopenstackdataplanedeployment.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - dataplane.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackdataplanedeployments
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: vopenstackdataplaneservice.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/validate-dataplane-openstack-org-v1beta1-openstackdataplaneservice
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: vopenstackdataplaneservice.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - dataplane.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackdataplaneservices
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mopenstackdataplaneservice.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/mutate-dataplane-openstack-org-v1beta1-openstackdataplaneservice
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: mopenstackdataplaneservice.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - dataplane.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackdataplaneservices
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: vopenstackansibleee.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/validate-ansibleee-openstack-org-v1beta1-openstackansibleee
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: vopenstackansibleee.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - ansibleee.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackansibleees
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
---
apiVersion: admissionregistration.k8s.io/v1
kind: MutatingWebhookConfiguration
metadata:
  name: mopenstackansibleee.kb.io
webhooks:
- admissionReviewVersions:
  - v1
  clientConfig:
    caBundle: ${CA_BUNDLE}
    url: https://${CRC_IP}:9443/mutate-ansibleee-openstack-org-v1beta1-openstackansibleee
  failurePolicy: Fail
  matchPolicy: Equivalent
  name: mopenstackansibleee.kb.io
  objectSelector: {}
  rules:
  - apiGroups:
    - ansibleee.openstack.org
    apiVersions:
    - v1beta1
    operations:
    - CREATE
    - UPDATE
    resources:
    - openstackansibleees
    scope: '*'
  sideEffects: None
  timeoutSeconds: 10
EOF_CAT

oc apply -n openstack -f ${TMPDIR}/patch_webhook_configurations.yaml
