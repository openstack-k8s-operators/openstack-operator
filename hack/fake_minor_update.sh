# A quick way to test a fake minor update. Run this script, and then once the
# openstackversion reconciles (the CSV will redeploy the controller-manager)
# you can set targetVersion == 0.0.2 for a quick test
VERSION=0.4
CURRENT=${VERSION}.0
UPDATE=${VERSION}.1
oc get csv openstack-operator.v${CURRENT} -o yaml -n openstack-operators  > csv.yaml
# bump them to current-podified
sed -i csv.yaml -e "s|value: .*quay.io/podified-antelope-centos9/\(.*\)@.*|value: quay.io/podified-antelope-centos9/\1:current-podified|g"
# also bump the OPENSTACK_RELEASE_VERSION value (it is the only field set like this)
sed -i csv.yaml -e "s|value: ${CURRENT}|value: ${UPDATE}|"
oc apply -n openstack-operators -f csv.yaml
