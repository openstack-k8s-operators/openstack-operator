module github.com/openstack-k8s-operators/openstack-operator/apis

go 1.19

require (
	github.com/onsi/ginkgo/v2 v2.11.0
	github.com/onsi/gomega v1.27.10
	github.com/openstack-k8s-operators/cinder-operator/api v0.1.1-0.20230814191049-79f552f43dd8
	github.com/openstack-k8s-operators/glance-operator/api v0.1.1-0.20230814183119-345713ec4219
	github.com/openstack-k8s-operators/heat-operator/api v0.1.1-0.20230810060039-f3690dc88146
	github.com/openstack-k8s-operators/horizon-operator/api v0.1.1-0.20230809035503-bcb0b055d9e8
	github.com/openstack-k8s-operators/infra-operator/apis v0.1.1-0.20230808142958-b6c74f5e1faf
	github.com/openstack-k8s-operators/ironic-operator/api v0.1.1-0.20230814160640-38f450fd80b1
	github.com/openstack-k8s-operators/keystone-operator/api v0.1.1-0.20230814191321-13dac2395fc0
	github.com/openstack-k8s-operators/lib-common/modules/common v0.1.1-0.20230811131408-7be84c6eae21
	github.com/openstack-k8s-operators/manila-operator/api v0.1.1-0.20230802043308-28bd5a6022fd
	github.com/openstack-k8s-operators/mariadb-operator/api v0.1.1-0.20230809092129-21af8a109394
	github.com/openstack-k8s-operators/neutron-operator/api v0.1.1-0.20230803070221-d6bbc917719e
	github.com/openstack-k8s-operators/nova-operator/api v0.1.2-0.20230811100810-17d91ed8846c
	github.com/openstack-k8s-operators/octavia-operator/api v0.0.0-20230810181000-456ef27e6770
	github.com/openstack-k8s-operators/ovn-operator/api v0.1.1-0.20230808050446-6b9663935676
	github.com/openstack-k8s-operators/placement-operator/api v0.1.1-0.20230811170444-6e7a00937764
	github.com/openstack-k8s-operators/swift-operator/api v0.1.1-0.20230814135725-9ae5a519264a
	github.com/openstack-k8s-operators/telemetry-operator/api v0.1.1-0.20230802123857-67049bf7e81a
	github.com/rabbitmq/cluster-operator v1.14.0
	k8s.io/apimachinery v0.26.7
	sigs.k8s.io/controller-runtime v0.14.6
)

require (
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20230510103437-eeec1cb781c3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.25.0 // indirect
	golang.org/x/exp v0.0.0-20230522175609-2e198f4a06a1 // indirect
	golang.org/x/tools v0.9.3 // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.2.4 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gophercloud/gophercloud v1.5.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.1.0 //indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.1.1-0.20230811131408-7be84c6eae21
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.12.0 // indirect
	golang.org/x/oauth2 v0.9.0 // indirect
	golang.org/x/sys v0.10.0 // indirect
	golang.org/x/term v0.10.0 // indirect
	golang.org/x/text v0.11.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.26.7
	k8s.io/apiextensions-apiserver v0.26.7 //indirect
	k8s.io/client-go v0.26.7
	k8s.io/component-base v0.26.7 //indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230515203736-54b630e78af5 //indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b //indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd //indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

// mschuppert: map to latest commit from release-4.13 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230414143018-3367bc7e6ac7 //allow-merging

replace github.com/openstack-k8s-operators/lib-common/modules/common => github.com/stuggi/lib-common/modules/common v0.0.0-20230817071545-78d401546fbd

replace github.com/openstack-k8s-operators/keystone-operator/api => github.com/stuggi/keystone-operator/api v0.0.0-20230817071801-e9a76286a0ee

replace github.com/openstack-k8s-operators/glance-operator/api => github.com/stuggi/glance-operator/api v0.0.0-20230817072026-6e8bbc772eae

replace github.com/openstack-k8s-operators/placement-operator/api => github.com/stuggi/placement-operator/api v0.0.0-20230817072416-d8488931388d

replace github.com/openstack-k8s-operators/cinder-operator/api => github.com/stuggi/cinder-operator/api v0.0.0-20230817072747-5c2625f5bea3

replace github.com/openstack-k8s-operators/neutron-operator/api => github.com/stuggi/neutron-operator/api v0.0.0-20230817072919-cf05e6dfc75a

replace github.com/openstack-k8s-operators/nova-operator/api => github.com/stuggi/nova-operator/api v0.0.0-20230817142637-d4a8cfba0ff8

replace github.com/openstack-k8s-operators/heat-operator/api => github.com/stuggi/heat-operator/api v0.0.0-20230817073252-11201e3ea71a

replace github.com/openstack-k8s-operators/horizon-operator/api => github.com/stuggi/horizon-operator/api v0.0.0-20230817073634-eb4a021174e8

replace github.com/openstack-k8s-operators/manila-operator/api => github.com/stuggi/manila-operator/api v0.0.0-20230817073920-95554b5ef1e7

replace github.com/openstack-k8s-operators/swift-operator/api => github.com/stuggi/swift-operator/api v0.0.0-20230817074054-423c7a596540
