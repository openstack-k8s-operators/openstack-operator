#!/bin/bash
set -ex

oc delete validatingwebhookconfiguration vopenstackcontrolplane.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration mopenstackcontrolplane.kb.io --ignore-not-found
oc delete validatingwebhookconfiguration/vopenstackclient.kb.io --ignore-not-found
oc delete mutatingwebhookconfiguration/mopenstackclient.kb.io --ignore-not-found
