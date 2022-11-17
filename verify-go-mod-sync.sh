#!/bin/bash

deps="
github.com/openstack-k8s-operators/cinder-operator/api
github.com/openstack-k8s-operators/glance-operator/api
github.com/openstack-k8s-operators/keystone-operator/api
github.com/openstack-k8s-operators/mariadb-operator/api
github.com/openstack-k8s-operators/placement-operator/api
github.com/rabbitmq/cluster-operator
"
result=0
for dep in $deps;
do
        if [ $(grep $dep go.mod ./apis/go.mod | tr $'\t' ' ' | cut -d ' ' -f3 | sort -u | wc -l) -ne 1 ]; then
                echo "go.mod and ./apis/go.mod has different versions from $dep"
                ((result+=1))
        fi
done
exit $result
