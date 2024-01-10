module github.com/openstack-k8s-operators/openstack-operator

go 1.19

require (
	github.com/cert-manager/cert-manager v1.11.5
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v1.4.1
	github.com/imdario/mergo v0.3.16
	github.com/onsi/ginkgo/v2 v2.13.2
	github.com/onsi/gomega v1.30.0
	github.com/openstack-k8s-operators/barbican-operator/api v0.0.0-20240104150405-d50607d50e9a
	github.com/openstack-k8s-operators/cinder-operator/api v0.3.1-0.20240104132718-a962cceb867d
	github.com/openstack-k8s-operators/dataplane-operator/api v0.3.1-0.20240104172704-238ed4f5b9f5
	github.com/openstack-k8s-operators/designate-operator/api v0.0.0-20240104144436-858a0383741c
	github.com/openstack-k8s-operators/glance-operator/api v0.3.1-0.20240109080016-338d4287e4ec
	github.com/openstack-k8s-operators/heat-operator/api v0.3.1-0.20240104130506-4f3841d6042d
	github.com/openstack-k8s-operators/horizon-operator/api v0.3.1-0.20240104144435-fdfef4b8a33f
	github.com/openstack-k8s-operators/infra-operator/apis v0.3.1-0.20240104150635-c4ffc51e0752
	github.com/openstack-k8s-operators/ironic-operator/api v0.3.1-0.20240104144719-8030e9e8c962
	github.com/openstack-k8s-operators/keystone-operator/api v0.3.1-0.20240104144437-5355d932c316
	github.com/openstack-k8s-operators/lib-common/modules/certmanager v0.0.0-20240110131857-e70e1dec4d14
	github.com/openstack-k8s-operators/lib-common/modules/common v0.3.1-0.20240110131857-e70e1dec4d14
	github.com/openstack-k8s-operators/lib-common/modules/test v0.3.1-0.20240104154954-dc504be0d9be
	github.com/openstack-k8s-operators/manila-operator/api v0.3.1-0.20240104144719-72b9a4ab968c
	github.com/openstack-k8s-operators/mariadb-operator/api v0.3.1-0.20240104162634-fe72003c6343
	github.com/openstack-k8s-operators/neutron-operator/api v0.3.1-0.20240104150349-c082ca19cafe
	github.com/openstack-k8s-operators/nova-operator/api v0.3.1-0.20240104123738-13980c2f529d
	github.com/openstack-k8s-operators/octavia-operator/api v0.3.1-0.20240104150350-1cb9656d2d92
	github.com/openstack-k8s-operators/openstack-ansibleee-operator/api v0.3.1-0.20240104130219-48e65da33a9a
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.3.1-0.20240104150636-35632735d92f
	github.com/openstack-k8s-operators/openstack-operator/apis v0.0.0-20230725141229-4ce90d0120fd
	github.com/openstack-k8s-operators/ovn-operator/api v0.3.1-0.20240104133234-31762c2b9fda
	github.com/openstack-k8s-operators/placement-operator/api v0.3.1-0.20240104123737-45f6dc371626
	github.com/openstack-k8s-operators/swift-operator/api v0.3.1-0.20240104130506-42419651f900
	github.com/openstack-k8s-operators/telemetry-operator/api v0.3.1-0.20240103003254-97178240dd81
	github.com/operator-framework/api v0.20.0
	github.com/rabbitmq/cluster-operator/v2 v2.5.0
	go.uber.org/zap v1.26.0
	golang.org/x/exp v0.0.0-20240103183307-be819d1f06fc
	k8s.io/api v0.28.1
	k8s.io/apimachinery v0.28.1
	k8s.io/client-go v0.28.1
	sigs.k8s.io/controller-runtime v0.15.1
)

require (
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20230510103437-eeec1cb781c3 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.4.0 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.3.1 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.2.0 // indirect
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.64.1-rhobs3 // indirect
	github.com/rhobs/observability-operator v0.0.20 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/tools v0.16.0 // indirect
	sigs.k8s.io/gateway-api v0.6.0 // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.0 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.5.0
	github.com/gophercloud/gophercloud v1.8.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.3.1-0.20240104154954-dc504be0d9be //indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.3.1-0.20240106101723-5f7aa263457f //indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.0 // indirect
	github.com/sirupsen/logrus v1.9.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/oauth2 v0.12.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/term v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.28.1 //indirect
	k8s.io/component-base v0.28.1 //indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230905202853-d090da108d2f //indirect
	k8s.io/utils v0.0.0-20240102154912-e7106e64919e
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd //indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.3.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace ( //allow-merging
	github.com/cert-manager/cert-manager => github.com/cert-manager/cert-manager v1.11.5
	github.com/google/gnostic => github.com/google/gnostic v0.6.9
	// pin to k8s 0.26.x for now
	k8s.io/api => k8s.io/api v0.26.11
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.11
	k8s.io/client-go => k8s.io/client-go v0.26.11
	k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20230327201221-f5883ff37f0c
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.14.7
	sigs.k8s.io/gateway-api => sigs.k8s.io/gateway-api v0.6.0
)

replace github.com/openstack-k8s-operators/openstack-operator/apis => ./apis

// mschuppert: map to latest commit from release-4.13 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230414143018-3367bc7e6ac7 //allow-merging

replace github.com/openstack-k8s-operators/keystone-operator/api => github.com/stuggi/keystone-operator/api v0.0.0-20240110132207-643df3216ef6

replace github.com/openstack-k8s-operators/neutron-operator/api => github.com/stuggi/neutron-operator/api v0.0.0-20240110132446-b7dd116f719a

replace github.com/openstack-k8s-operators/glance-operator/api => github.com/stuggi/glance-operator/api v0.0.0-20240110140133-e82ebb00ccb7

replace github.com/openstack-k8s-operators/cinder-operator/api => github.com/stuggi/cinder-operator/api v0.0.0-20240110132541-fed2378a8cb1
