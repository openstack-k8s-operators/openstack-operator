module github.com/openstack-k8s-operators/openstack-operator/apis

go 1.20

require (
	github.com/cert-manager/cert-manager v1.13.6
	github.com/go-playground/validator/v10 v10.21.0
	github.com/onsi/ginkgo/v2 v2.19.0
	github.com/onsi/gomega v1.33.1
	github.com/openstack-k8s-operators/barbican-operator/api v0.0.0-20240603141403-1ad477d065a2
	github.com/openstack-k8s-operators/cinder-operator/api v0.3.1-0.20240612161940-604036e71c7f
	github.com/openstack-k8s-operators/designate-operator/api v0.0.0-20240611223414-c013696ad9cc
	github.com/openstack-k8s-operators/glance-operator/api v0.3.1-0.20240610071145-9fc8ef9f3c7c
	github.com/openstack-k8s-operators/heat-operator/api v0.3.1-0.20240614091021-67d1c605b0e8
	github.com/openstack-k8s-operators/horizon-operator/api v0.3.1-0.20240604091541-cdb648f9a7ac
	github.com/openstack-k8s-operators/infra-operator/apis v0.3.1-0.20240604144138-996e41d1af19
	github.com/openstack-k8s-operators/ironic-operator/api v0.3.1-0.20240607124904-2f2e4f3dd090
	github.com/openstack-k8s-operators/keystone-operator/api v0.3.1-0.20240605055850-8ee0ece70906
	github.com/openstack-k8s-operators/lib-common/modules/common v0.3.1-0.20240612061748-03d1fc5e2fbe
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.3.1-0.20240612061748-03d1fc5e2fbe
	github.com/openstack-k8s-operators/manila-operator/api v0.3.1-0.20240610185533-3683da06b8eb
	github.com/openstack-k8s-operators/mariadb-operator/api v0.3.1-0.20240613130956-7d997dc5d972
	github.com/openstack-k8s-operators/neutron-operator/api v0.3.1-0.20240612151610-3a231baf6d66
	github.com/openstack-k8s-operators/nova-operator/api v0.3.1-0.20240613191819-c7395c1ce221
	github.com/openstack-k8s-operators/octavia-operator/api v0.3.1-0.20240614142111-2787761a35ad
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.3.1-0.20240604070904-cdec81ca1825
	github.com/openstack-k8s-operators/ovn-operator/api v0.3.1-0.20240614143745-cbe8d2bba3e4
	github.com/openstack-k8s-operators/placement-operator/api v0.3.1-0.20240614120333-5d0e0dfb84d1
	github.com/openstack-k8s-operators/swift-operator/api v0.3.1-0.20240612111911-209ad35b5db7
	github.com/openstack-k8s-operators/telemetry-operator/api v0.3.1-0.20240614144556-940a48eaf688
	github.com/rabbitmq/cluster-operator/v2 v2.6.0
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.69.0-rhobs1 // indirect
	github.com/rhobs/observability-operator v0.0.28 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20240409090435-93d18d7e34b8
	golang.org/x/tools v0.21.0 // indirect
	k8s.io/api v0.28.11
	k8s.io/apimachinery v0.28.11
	k8s.io/client-go v0.28.11
	sigs.k8s.io/controller-runtime v0.16.6
)

require k8s.io/utils v0.0.0-20240502163921-fe8a2dddb1d0

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.3 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240424215950-a892ee059fd6 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gophercloud/gophercloud v1.12.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.5.1 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.4.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.3.1-0.20240612061748-03d1fc5e2fbe // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	golang.org/x/crypto v0.23.0 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/oauth2 v0.16.0 // indirect
	golang.org/x/sys v0.20.0 // indirect
	golang.org/x/term v0.20.0 // indirect
	golang.org/x/text v0.15.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.28.11 // indirect
	k8s.io/component-base v0.28.11 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240228011516-70dd3763d340 // indirect
	sigs.k8s.io/gateway-api v0.8.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

// mschuppert: map to latest commit from release-4.13 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230414143018-3367bc7e6ac7 //allow-merging

// custom RabbitmqClusterSpecCore for OpenStackControlplane (v2.6.0_patches_tag)
replace github.com/rabbitmq/cluster-operator/v2 => github.com/openstack-k8s-operators/rabbitmq-cluster-operator/v2 v2.6.1-0.20240612145157-629e537392b3 //allow-merging
