module github.com/openstack-k8s-operators/openstack-operator/api

go 1.26.3

require (
	github.com/cert-manager/cert-manager v1.18.6
	github.com/go-playground/validator/v10 v10.30.3
	github.com/onsi/ginkgo/v2 v2.32.0
	github.com/onsi/gomega v1.42.1
	github.com/openstack-k8s-operators/barbican-operator/api v0.6.1-0.20260718084238-021ced1d93a9
	github.com/openstack-k8s-operators/cinder-operator/api v0.6.1-0.20260718115004-37472c33e009
	github.com/openstack-k8s-operators/designate-operator/api v0.6.1-0.20260718084238-e8741ebff581
	github.com/openstack-k8s-operators/glance-operator/api v0.6.1-0.20260718115005-27b6f8ace0ed
	github.com/openstack-k8s-operators/heat-operator/api v0.6.1-0.20260718115005-9efaaa68b557
	github.com/openstack-k8s-operators/horizon-operator/api v0.6.1-0.20260718084237-d7faaaa31246
	github.com/openstack-k8s-operators/infra-operator/apis v0.6.1-0.20260718084237-5df87de62106
	github.com/openstack-k8s-operators/ironic-operator/api v0.6.1-0.20260718115004-150f47ca20d3
	github.com/openstack-k8s-operators/keystone-operator/api v0.6.1-0.20260718115006-465f0b877bbe
	github.com/openstack-k8s-operators/lib-common/modules/common v0.6.1-0.20260717092345-ab1ee7b97c67
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.6.1-0.20260717092345-ab1ee7b97c67
	github.com/openstack-k8s-operators/manila-operator/api v0.6.1-0.20260718115007-533286bb763c
	github.com/openstack-k8s-operators/mariadb-operator/api v0.6.1-0.20260718115003-b917b0d72e8a
	github.com/openstack-k8s-operators/neutron-operator/api v0.6.1-0.20260718115000-279634c924ae
	github.com/openstack-k8s-operators/nova-operator/api v0.6.1-0.20260718115002-683cc0aeb38a
	github.com/openstack-k8s-operators/octavia-operator/api v0.6.1-0.20260718114958-9a5bde8cdc6f
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.6.1-0.20260718115006-7d7c29ae7d5a
	github.com/openstack-k8s-operators/ovn-operator/api v0.6.1-0.20260718115004-d33324e13623
	github.com/openstack-k8s-operators/swift-operator/api v0.6.1-0.20260718115003-4e3c990fa737
	github.com/openstack-k8s-operators/telemetry-operator/api v0.6.1-0.20260718115006-ce78f7e49f5c
	github.com/openstack-k8s-operators/watcher-operator/api v0.6.1-0.20260718115834-fae3078ef033
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.77.1-rhobs1 // indirect
	github.com/rhobs/observability-operator v1.0.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.28.0 // indirect
	golang.org/x/exp v0.0.0-20260611194520-c48552f49976
	golang.org/x/tools v0.48.0 // indirect
	k8s.io/api v0.33.13
	k8s.io/apimachinery v0.33.13
	k8s.io/client-go v0.33.13
	k8s.io/utils v0.0.0-20260210185600-b8788abfbbc2
	sigs.k8s.io/controller-runtime v0.21.0
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.13.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.13 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.1 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
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
	github.com/gophercloud/gophercloud/v2 v2.13.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.11.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.6.1-0.20260717092345-ab1ee7b97c67 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v2 v2.4.3 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/mod v0.38.0 // indirect
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/oauth2 v0.36.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/term v0.45.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	k8s.io/apiextensions-apiserver v0.33.13 // indirect
	k8s.io/klog/v2 v2.140.0 // indirect
	k8s.io/kube-openapi v0.0.0-20250902184714-7fc278399c7f // indirect
	sigs.k8s.io/gateway-api v1.2.1 // indirect
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
