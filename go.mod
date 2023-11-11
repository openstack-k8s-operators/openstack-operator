module github.com/openstack-k8s-operators/openstack-operator

go 1.19

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v1.2.4
	github.com/imdario/mergo v0.3.16
	github.com/onsi/ginkgo/v2 v2.12.1
	github.com/onsi/gomega v1.28.0
	github.com/openstack-k8s-operators/cinder-operator/api v0.3.1-0.20231108214420-8ed92ee7b839
	github.com/openstack-k8s-operators/dataplane-operator/api v0.2.1-0.20231110152421-750e53d351d8
	github.com/openstack-k8s-operators/glance-operator/api v0.2.1-0.20231108214137-bbf3476632b8
	github.com/openstack-k8s-operators/heat-operator/api v0.2.1-0.20231109132642-ec106cb7501f
	github.com/openstack-k8s-operators/horizon-operator/api v0.2.1-0.20231108230321-4baed63eb4e9
	github.com/openstack-k8s-operators/infra-operator/apis v0.2.1-0.20231110151746-93d3b52adb82
	github.com/openstack-k8s-operators/ironic-operator/api v0.2.1-0.20231108224935-c7926207d742
	github.com/openstack-k8s-operators/keystone-operator/api v0.2.1-0.20231108214421-4c26401bf967
	github.com/openstack-k8s-operators/lib-common/modules/common v0.2.0
	github.com/openstack-k8s-operators/manila-operator/api v0.2.1-0.20231108230037-cc79f5559974
	github.com/openstack-k8s-operators/mariadb-operator/api v0.2.1-0.20231108230037-0751fe887640
	github.com/openstack-k8s-operators/neutron-operator/api v0.2.1-0.20231108211656-019b675c1d09
	github.com/openstack-k8s-operators/nova-operator/api v0.3.1-0.20231109153403-a7dcff3b218a
	github.com/openstack-k8s-operators/octavia-operator/api v0.3.1-0.20231108230036-1b53d614bb0b
	github.com/openstack-k8s-operators/openstack-ansibleee-operator/api v0.2.1-0.20231111000310-b6a099c83479
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.3.1-0.20231109134311-9ce7c9365645
	github.com/openstack-k8s-operators/openstack-operator/apis v0.0.0-20230725141229-4ce90d0120fd
	github.com/openstack-k8s-operators/ovn-operator/api v0.2.1-0.20231108225506-26592569dcb4
	github.com/openstack-k8s-operators/placement-operator/api v0.3.1-0.20231109132642-f4906e746627
	github.com/openstack-k8s-operators/swift-operator/api v0.3.1-0.20231108224935-77cf9e4a7d24
	github.com/openstack-k8s-operators/telemetry-operator/api v0.2.1-0.20231109132641-b9318803e632
	github.com/operator-framework/api v0.17.6
	github.com/rabbitmq/cluster-operator/v2 v2.5.0
	go.uber.org/zap v1.26.0
	k8s.io/api v0.27.2
	k8s.io/apimachinery v0.27.4
	k8s.io/client-go v0.27.2
	sigs.k8s.io/controller-runtime v0.15.1
)

require (
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20230510103437-eeec1cb781c3 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.3.1 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.2.0 // indirect
	golang.org/x/exp v0.0.0-20230905200255-921286631fa9 // indirect
	golang.org/x/tools v0.13.0 // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.1 // indirect
	github.com/gophercloud/gophercloud v1.7.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.1.1-0.20231001084618-12369665b166 //indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.2.0 //indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.0 // indirect
	github.com/sirupsen/logrus v1.9.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.15.0 // indirect
	golang.org/x/oauth2 v0.10.0 // indirect
	golang.org/x/sys v0.13.0 // indirect
	golang.org/x/term v0.13.0 // indirect
	golang.org/x/text v0.13.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.27.2 //indirect
	k8s.io/component-base v0.27.2 //indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230525220651-2546d827e515 //indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd //indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.3.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace (
	// pin to k8s 0.26.x for now
	k8s.io/api => k8s.io/api v0.26.9
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.9
	k8s.io/client-go => k8s.io/client-go v0.26.9
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.14.6

)

// Bump golang.org/x/net to avoid Rapid Reset CVE
replace golang.org/x/net => golang.org/x/net v0.17.0 //allow-merging

replace github.com/openstack-k8s-operators/openstack-operator/apis => ./apis

// mschuppert: map to latest commit from release-4.13 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230414143018-3367bc7e6ac7 //allow-merging
