# A quick way to test a fake minor update. Run this script, and then once the
# openstackversion reconciles (the CSV will redeploy the controller-manager)
# you can set targetVersion == 0.0.2 for a quick test
oc get csv openstack-operator.v0.4.0 -o yaml -n openstack-operators  > csv.yaml
# bump them to current-podified
sed -i csv.yaml -e "s|value: .*quay.io/podified-antelope-centos9/\(.*\)@.*|value: quay.io/podified-antelope-centos9/\1:current-podified|g"
# also bump the OPENSTACK_RELEASE_VERSION value (it is the only field set like this)
sed -i csv.yaml -e "s|value: 0.4.0|value: 0.4.1|"
oc apply -n openstack-operators -f csv.yaml
