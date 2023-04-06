module github.com/openstack-k8s-operators/openstack-operator/apis

go 1.19

require (
	github.com/openstack-k8s-operators/cinder-operator/api v0.0.0-20230404123906-31a141ed31a7
	github.com/openstack-k8s-operators/glance-operator/api v0.0.0-20230404134424-48d1d0cc1ce8
	github.com/openstack-k8s-operators/infra-operator/apis v0.0.0-20230324141445-d1ac945900ed
	github.com/openstack-k8s-operators/ironic-operator/api v0.0.0-20230404074453-a45327c3502f
	github.com/openstack-k8s-operators/keystone-operator/api v0.0.0-20230403180130-0c0446d0c308
	github.com/openstack-k8s-operators/lib-common/modules/common v0.0.0-20230404094840-40d9ff1586e1
	github.com/openstack-k8s-operators/manila-operator/api v0.0.0-20230403172743-c62cbab42bda
	github.com/openstack-k8s-operators/mariadb-operator/api v0.0.0-20230331101132-3a320b926e09
	github.com/openstack-k8s-operators/neutron-operator/api v0.0.0-20230404161401-11f2c91e84b2
	github.com/openstack-k8s-operators/nova-operator/api v0.0.0-20230404133348-7e9aa75b8a51
	github.com/openstack-k8s-operators/ovn-operator/api v0.0.0-20230404065948-c761b84761fa
	github.com/openstack-k8s-operators/placement-operator/api v0.0.0-20230404093611-4c8cf1203823
	github.com/rabbitmq/cluster-operator v1.14.0
	k8s.io/apimachinery v0.26.3
	sigs.k8s.io/controller-runtime v0.14.6
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.1 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.1 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gophercloud/gophercloud v1.2.0 // indirect
	github.com/imdario/mergo v0.3.15 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.0.0-20230404094840-40d9ff1586e1 // indirect; indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.0.0-20230404094840-40d9ff1586e1
	github.com/openstack-k8s-operators/ovs-operator/api v0.0.0-20230404065949-bea95aca292c
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.14.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.8.0 // indirect
	golang.org/x/oauth2 v0.0.0-20220909003341-f21342109be1 // indirect
	golang.org/x/sys v0.6.0 // indirect
	golang.org/x/term v0.6.0 // indirect
	golang.org/x/text v0.8.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.26.3 // indirect // indirect // indirect
	k8s.io/apiextensions-apiserver v0.26.3 // indirect; indirect // indirect // indirect
	k8s.io/client-go v0.26.3 // indirect; indirect // indirect // indirect
	k8s.io/component-base v0.26.3 // indirect; indirect // indirect // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230327201221-f5883ff37f0c // indirect; indirect // indirect // indirect // indirect // indirect // indirect
	k8s.io/utils v0.0.0-20230313181309-38a27ef9d749 // indirect; indirect // indirect // indirect // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect; indirect // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
