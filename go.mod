module github.com/openstack-k8s-operators/openstack-operator

go 1.20

require (
	github.com/cert-manager/cert-manager v1.11.5
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v1.4.1
	github.com/imdario/mergo v0.3.16
	github.com/onsi/ginkgo/v2 v2.15.0
	github.com/onsi/gomega v1.31.1
	github.com/openstack-k8s-operators/barbican-operator/api v0.0.0-20240216141430-461e99fa1b3c
	github.com/openstack-k8s-operators/cinder-operator/api v0.3.1-0.20240216172128-52771a306313
	github.com/openstack-k8s-operators/dataplane-operator/api v0.3.1-0.20240217035625-331533431892
	github.com/openstack-k8s-operators/designate-operator/api v0.0.0-20240216102618-e7d16424a045
	github.com/openstack-k8s-operators/glance-operator/api v0.3.1-0.20240216191912-f09285faf21b
	github.com/openstack-k8s-operators/heat-operator/api v0.3.1-0.20240216110209-c405c043e280
	github.com/openstack-k8s-operators/horizon-operator/api v0.3.1-0.20240217031606-5f5dac9bb5b7
	github.com/openstack-k8s-operators/infra-operator/apis v0.3.1-0.20240219153539-99face71256b
	github.com/openstack-k8s-operators/ironic-operator/api v0.3.1-0.20240216150409-296d5c6420a5
	github.com/openstack-k8s-operators/keystone-operator/api v0.3.1-0.20240216173228-eec429bcc776
	github.com/openstack-k8s-operators/lib-common/modules/certmanager v0.0.0-20240216173409-86913e6d5885
	github.com/openstack-k8s-operators/lib-common/modules/common v0.3.1-0.20240220172726-06e269f22402
	github.com/openstack-k8s-operators/lib-common/modules/test v0.3.1-0.20240216173409-86913e6d5885
	github.com/openstack-k8s-operators/manila-operator/api v0.3.1-0.20240216154248-d78aa8eb0e8a
	github.com/openstack-k8s-operators/mariadb-operator/api v0.3.1-0.20240215091212-cbf2ad281f43
	github.com/openstack-k8s-operators/neutron-operator/api v0.3.1-0.20240216172657-9188630d3435
	github.com/openstack-k8s-operators/nova-operator/api v0.3.1-0.20240216133845-7e1445f8cae4
	github.com/openstack-k8s-operators/octavia-operator/api v0.3.1-0.20240215100511-492a87cdffa3
	github.com/openstack-k8s-operators/openstack-ansibleee-operator/api v0.3.1-0.20240216162346-b1dca894860d
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.3.1-0.20240217021438-5918cf3aefdf
	github.com/openstack-k8s-operators/openstack-operator/apis v0.0.0-00010101000000-000000000000
	github.com/openstack-k8s-operators/ovn-operator/api v0.3.1-0.20240216200042-7835df58ed0c
	github.com/openstack-k8s-operators/placement-operator/api v0.3.1-0.20240216174613-3d349f26e681
	github.com/openstack-k8s-operators/swift-operator/api v0.3.1-0.20240216164023-80bb42077844
	github.com/openstack-k8s-operators/telemetry-operator/api v0.3.1-0.20240219111006-cce4d37e5187
	github.com/operator-framework/api v0.20.0
	github.com/rabbitmq/cluster-operator/v2 v2.7.0
	go.uber.org/zap v1.26.0
	golang.org/x/exp v0.0.0-20240213143201-ec583247a57a
	k8s.io/api v0.29.0
	k8s.io/apimachinery v0.29.0
	k8s.io/client-go v0.29.0
	sigs.k8s.io/controller-runtime v0.16.5
)

require (
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/pprof v0.0.0-20231229205709-960ae82b1e42 // indirect
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.4.0 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.5.0 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.4.0 // indirect
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.64.1-rhobs3 // indirect
	github.com/rhobs/observability-operator v0.0.20 // indirect
	golang.org/x/mod v0.15.0 // indirect
	golang.org/x/tools v0.18.0 // indirect
	sigs.k8s.io/gateway-api v0.6.0 // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0
	github.com/gophercloud/gophercloud v1.9.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.3.1-0.20240216173409-86913e6d5885 //indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.3.1-0.20240216173409-86913e6d5885 //indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/sirupsen/logrus v1.9.2 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	golang.org/x/net v0.21.0 // indirect
	golang.org/x/oauth2 v0.17.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/term v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.32.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.29.0 //indirect
	k8s.io/component-base v0.29.0 //indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240209001042-7a0d5b415232 //indirect
	k8s.io/utils v0.0.0-20240102154912-e7106e64919e
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd //indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

replace github.com/openstack-k8s-operators/openstack-operator/apis => ./apis

// mschuppert: map to latest commit from release-4.13 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230414143018-3367bc7e6ac7 //allow-merging
