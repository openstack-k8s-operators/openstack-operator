module github.com/openstack-k8s-operators/openstack-operator/apis

go 1.19

require (
	github.com/onsi/ginkgo/v2 v2.13.1
	github.com/onsi/gomega v1.30.0
	github.com/openstack-k8s-operators/cinder-operator/api v0.3.1-0.20231114160640-3c5c40e6cc3a
	github.com/openstack-k8s-operators/designate-operator/api v0.0.0-20231113184257-f5f91f9b0986
	github.com/openstack-k8s-operators/glance-operator/api v0.3.1-0.20231115090205-264277ef61bb
	github.com/openstack-k8s-operators/heat-operator/api v0.3.1-0.20231115022322-40df8776a641
	github.com/openstack-k8s-operators/horizon-operator/api v0.3.1-0.20231103160944-413a8b455a4a
	github.com/openstack-k8s-operators/infra-operator/apis v0.3.1-0.20231113161711-d0c2a36fe46d
	github.com/openstack-k8s-operators/ironic-operator/api v0.3.1-0.20231103171435-bb14d998275c
	github.com/openstack-k8s-operators/keystone-operator/api v0.3.1-0.20231103151903-a1a0e540408d
	github.com/openstack-k8s-operators/lib-common/modules/common v0.3.1-0.20231114102008-65eb1b13d3a7
	github.com/openstack-k8s-operators/manila-operator/api v0.3.1-0.20231115215538-f222043fd9ca
	github.com/openstack-k8s-operators/mariadb-operator/api v0.3.1-0.20231103165533-ad766a5aa6b0
	github.com/openstack-k8s-operators/neutron-operator/api v0.3.1-0.20231115061135-28c36ae19f87
	github.com/openstack-k8s-operators/nova-operator/api v0.3.1-0.20231113083956-daa5310a0fef
	github.com/openstack-k8s-operators/octavia-operator/api v0.3.1-0.20231115115237-57226ee0d5ee
	github.com/openstack-k8s-operators/ovn-operator/api v0.3.1-0.20231116094235-e5276dc13ac1
	github.com/openstack-k8s-operators/placement-operator/api v0.3.1-0.20231115155722-c45baeea1005
	github.com/openstack-k8s-operators/swift-operator/api v0.3.1-0.20231114120226-5eaef7af17b4
	github.com/openstack-k8s-operators/telemetry-operator/api v0.3.1-0.20231114081411-5a92493f3843
	github.com/rabbitmq/cluster-operator/v2 v2.5.0
	k8s.io/apimachinery v0.27.4
	sigs.k8s.io/controller-runtime v0.15.1
)

require (
	github.com/go-logr/zapr v1.2.4 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/pprof v0.0.0-20230510103437-eeec1cb781c3 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.26.0 // indirect
	golang.org/x/exp v0.0.0-20231006140011-7918f672742d // indirect
	golang.org/x/tools v0.14.0 // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.0 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.4 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.4.0 // indirect
	github.com/gophercloud/gophercloud v1.7.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.3.1-0.20231114102008-65eb1b13d3a7 //indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.3.1-0.20231114102008-65eb1b13d3a7
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.16.0 // indirect
	github.com/prometheus/client_model v0.4.0 // indirect
	github.com/prometheus/common v0.44.0 // indirect
	github.com/prometheus/procfs v0.11.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/net v0.17.0 // indirect
	golang.org/x/oauth2 v0.10.0 // indirect
	golang.org/x/sys v0.14.0 // indirect
	golang.org/x/term v0.14.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.3.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/api v0.27.2
	k8s.io/apiextensions-apiserver v0.27.2 //indirect
	k8s.io/client-go v0.27.2
	k8s.io/component-base v0.27.2 //indirect
	k8s.io/klog/v2 v2.100.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230525220651-2546d827e515 //indirect
	k8s.io/utils v0.0.0-20230726121419-3b25d923346b //indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd //indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.3.0 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace ( //allow-merging
	// pin to k8s 0.26.x for now
	k8s.io/api => k8s.io/api v0.26.9
	k8s.io/apimachinery => k8s.io/apimachinery v0.26.9
	k8s.io/client-go => k8s.io/client-go v0.26.9
	sigs.k8s.io/controller-runtime => sigs.k8s.io/controller-runtime v0.14.6
)

// mschuppert: map to latest commit from release-4.13 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230414143018-3367bc7e6ac7 //allow-merging

// Bump golang.org/x/net to avoid Rapid Reset CVE
replace golang.org/x/net => golang.org/x/net v0.18.0 //allow-merging
