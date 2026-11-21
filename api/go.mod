module github.com/openstack-k8s-operators/openstack-operator/api

go 1.26.3

require (
	github.com/cert-manager/cert-manager v1.19.6
	github.com/go-playground/validator/v10 v10.30.3
	github.com/onsi/ginkgo/v2 v2.34.0
	github.com/onsi/gomega v1.44.0
	github.com/openstack-k8s-operators/barbican-operator/api v0.6.1-0.20261121072501-b9da9a9aad5b
	github.com/openstack-k8s-operators/cinder-operator/api v0.6.1-0.20261120181937-b36fa3191a57
	github.com/openstack-k8s-operators/designate-operator/api v0.6.1-0.20261120170339-d45e0324602f
	github.com/openstack-k8s-operators/glance-operator/api v0.6.1-0.20261121072501-5d502dc997ed
	github.com/openstack-k8s-operators/heat-operator/api v0.6.1-0.20261120154202-c4261a8d892b
	github.com/openstack-k8s-operators/horizon-operator/api v0.6.1-0.20261121072502-3e248235ee2a
	github.com/openstack-k8s-operators/infra-operator/apis v0.6.1-0.20261118064744-5c00118ba512
	github.com/openstack-k8s-operators/ironic-operator/api v0.6.1-0.20261120075741-e000c3c799fd
	github.com/openstack-k8s-operators/keystone-operator/api v0.6.1-0.20261120164743-068ce5bda2dc
	github.com/openstack-k8s-operators/lib-common/modules/common v0.6.1-0.20261120143313-344125315151
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.6.1-0.20261120143313-344125315151
	github.com/openstack-k8s-operators/manila-operator/api v0.6.1-0.20261120180932-3bb798e7adc6
	github.com/openstack-k8s-operators/mariadb-operator/api v0.6.1-0.20261119151634-76daf34914b1
	github.com/openstack-k8s-operators/neutron-operator/api v0.6.1-0.20261120165714-19e180ad6657
	github.com/openstack-k8s-operators/nova-operator/api v0.6.1-0.20261120181434-cc8f372923fd
	github.com/openstack-k8s-operators/octavia-operator/api v0.6.1-0.20261120154201-34729aeac0d0
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.6.1-0.20261120012958-fd313b891603
	github.com/openstack-k8s-operators/ovn-operator/api v0.6.1-0.20261120172338-9eeb818a5bd9
	github.com/openstack-k8s-operators/swift-operator/api v0.6.1-0.20261120181435-bf80eed26dbb
	github.com/openstack-k8s-operators/telemetry-operator/api v0.6.1-0.20261117232707-e5e294827d56
	github.com/openstack-k8s-operators/watcher-operator/api v0.6.1-0.20261117071146-345d75395c74
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.77.1-rhobs1 // indirect
	github.com/rhobs/observability-operator v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	golang.org/x/exp v0.0.0-20260611194520-c48552f49976
	golang.org/x/tools v0.52.0 // indirect
	k8s.io/api v0.34.1
	k8s.io/apimachinery v0.34.1
	k8s.io/client-go v0.34.1
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2
	sigs.k8s.io/controller-runtime v0.22.3
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.22.1 // indirect
	github.com/go-openapi/jsonreference v0.21.2 // indirect
	github.com/go-openapi/swag v0.25.4 // indirect
	github.com/go-openapi/swag/cmdutils v0.25.4 // indirect
	github.com/go-openapi/swag/conv v0.25.4 // indirect
	github.com/go-openapi/swag/fileutils v0.25.4 // indirect
	github.com/go-openapi/swag/jsonname v0.25.4 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.4 // indirect
	github.com/go-openapi/swag/loading v0.25.4 // indirect
	github.com/go-openapi/swag/mangling v0.25.4 // indirect
	github.com/go-openapi/swag/netutils v0.25.4 // indirect
	github.com/go-openapi/swag/stringutils v0.25.4 // indirect
	github.com/go-openapi/swag/typeutils v0.25.4 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.4 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/pprof v0.0.0-20260402051712-545e8a4df936 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gophercloud/gophercloud/v2 v2.16.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.11.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.6.1-0.20261113070913-5152d1d626f9 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.23.2 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.66.1 // indirect
	github.com/prometheus/procfs v0.17.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/stretchr/objx v0.5.3 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.59.0 // indirect
	golang.org/x/mod v0.42.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.25.0 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
	golang.org/x/text v0.44.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.13.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apiextensions-apiserver v0.34.1 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20250910181357-589584f1c912 // indirect
	sigs.k8s.io/gateway-api v1.4.0 // indirect
	sigs.k8s.io/json v0.0.0-20250730193827-2d320260d730 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

// mschuppert: map to latest commit from release-4.20 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20260710141509-36dec0bfafe4 //allow-merging

replace k8s.io/apimachinery => k8s.io/apimachinery v0.33.13 //allow-merging

replace k8s.io/api => k8s.io/api v0.33.13 //allow-merging

replace k8s.io/apiserver => k8s.io/apiserver v0.33.13 //allow-merging

replace k8s.io/client-go => k8s.io/client-go v0.33.13 //allow-merging

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.33.13 //allow-merging

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.33.13 //allow-merging

replace k8s.io/code-generator => k8s.io/code-generator v0.33.13 //allow-merging

replace k8s.io/component-base => k8s.io/component-base v0.33.13 //allow-merging

replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20250627150254-e9823e99808e //allow-merging

replace github.com/cert-manager/cmctl/v2 => github.com/cert-manager/cmctl/v2 v2.3.0 //allow-merging
