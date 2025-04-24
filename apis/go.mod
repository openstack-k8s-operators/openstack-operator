module github.com/openstack-k8s-operators/openstack-operator/apis

go 1.21

require (
	github.com/cert-manager/cert-manager v1.14.7
	github.com/go-playground/validator/v10 v10.25.0
	github.com/onsi/ginkgo/v2 v2.20.1
	github.com/onsi/gomega v1.34.1
	github.com/openstack-k8s-operators/barbican-operator/api v0.6.1-0.20250422103125-ccaa7fe59156
	github.com/openstack-k8s-operators/cinder-operator/api v0.6.1-0.20250423054052-76f811364c48
	github.com/openstack-k8s-operators/designate-operator/api v0.6.1-0.20250422103124-9adfe79423df
	github.com/openstack-k8s-operators/glance-operator/api v0.6.1-0.20250422163246-d10f72d2e220
	github.com/openstack-k8s-operators/heat-operator/api v0.6.1-0.20250423034234-f96998c6fdac
	github.com/openstack-k8s-operators/horizon-operator/api v0.6.1-0.20250421055257-62660c8c5cba
	github.com/openstack-k8s-operators/infra-operator/apis v0.6.1-0.20250424140239-2d89c1d9f3ec
	github.com/openstack-k8s-operators/ironic-operator/api v0.6.1-0.20250423054051-4b1b486312a5
	github.com/openstack-k8s-operators/keystone-operator/api v0.6.1-0.20250420193521-f2b86a82d7e8
	github.com/openstack-k8s-operators/lib-common/modules/common v0.6.1-0.20250423055245-3cb2ae8df6f0
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.6.1-0.20250423055245-3cb2ae8df6f0
	github.com/openstack-k8s-operators/manila-operator/api v0.6.1-0.20250421071803-b2385c65b8e5
	github.com/openstack-k8s-operators/mariadb-operator/api v0.6.1-0.20250415060817-dc849adfa27e
	github.com/openstack-k8s-operators/neutron-operator/api v0.6.1-0.20250422103409-9ddf83d0f545
	github.com/openstack-k8s-operators/nova-operator/api v0.6.1-0.20250422095814-99e0f2e04f20
	github.com/openstack-k8s-operators/octavia-operator/api v0.6.1-0.20250422141120-3b3a002bcbd6
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.6.1-0.20250422102839-2ae63e63226d
	github.com/openstack-k8s-operators/ovn-operator/api v0.6.1-0.20250422103124-2bd08f620f3f
	github.com/openstack-k8s-operators/placement-operator/api v0.6.1-0.20250421055256-80229e1ea7ad
	github.com/openstack-k8s-operators/swift-operator/api v0.6.1-0.20250422064723-b0252ffb3946
	github.com/openstack-k8s-operators/telemetry-operator/api v0.6.1-0.20250420191048-a2d55f55d5da
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.71.0-rhobs1 // indirect
	github.com/rhobs/observability-operator v0.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
	golang.org/x/tools v0.24.0 // indirect
	k8s.io/api v0.29.15
	k8s.io/apimachinery v0.29.15
	k8s.io/client-go v0.29.15
	k8s.io/utils v0.0.0-20240711033017-18e509b52bc8
	sigs.k8s.io/controller-runtime v0.17.6
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/go-logr/logr v1.4.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.0 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20240727154555-813a5fbdbec8 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gophercloud/gophercloud v1.14.1 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.6.3 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.5.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.6.1-0.20250423055245-3cb2ae8df6f0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.19.0 // indirect
	github.com/prometheus/client_model v0.6.0 // indirect
	github.com/prometheus/common v0.53.0 // indirect
	github.com/prometheus/procfs v0.13.0 // indirect
	github.com/rabbitmq/cluster-operator/v2 v2.11.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.18.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/term v0.29.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/appengine v1.6.8 // indirect
	google.golang.org/protobuf v1.34.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.29.15 // indirect
	k8s.io/component-base v0.29.15 // indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240322212309-b815d8309940 // indirect
	sigs.k8s.io/gateway-api v1.0.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

// mschuppert: map to latest commit from release-4.16 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20240830023148-b7d0481c9094 //allow-merging

// custom RabbitmqClusterSpecCore for OpenStackControlplane (v2.9.0_patches_tag)
replace github.com/rabbitmq/cluster-operator/v2 => github.com/openstack-k8s-operators/rabbitmq-cluster-operator/v2 v2.6.1-0.20241017142550-a3524acedd49 //allow-merging
