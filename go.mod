module github.com/openstack-k8s-operators/openstack-operator

go 1.24.4

require (
	github.com/cert-manager/cert-manager v1.16.5
	github.com/go-logr/logr v1.4.3
	github.com/go-playground/validator/v10 v10.28.0
	github.com/google/uuid v1.6.0
	github.com/iancoleman/strcase v0.3.0
	github.com/k8snetworkplumbingwg/network-attachment-definition-client v1.7.7
	github.com/onsi/ginkgo/v2 v2.27.2
	github.com/onsi/gomega v1.38.3
	github.com/openshift/api v3.9.0+incompatible
	github.com/openstack-k8s-operators/barbican-operator/api v0.6.1-0.20251125115107-f489fa5ceb3c
	github.com/openstack-k8s-operators/cinder-operator/api v0.6.1-0.20251204132059-3dda695e96ae
	github.com/openstack-k8s-operators/designate-operator/api v0.6.1-0.20251203145024-0f6b7a8e7dc5
	github.com/openstack-k8s-operators/glance-operator/api v0.6.1-0.20251203100349-4a406668b8c7
	github.com/openstack-k8s-operators/heat-operator/api v0.6.1-0.20251125115646-26b110b9f3e7
	github.com/openstack-k8s-operators/horizon-operator/api v0.6.1-0.20251125145341-8bc80a35f9c5
	github.com/openstack-k8s-operators/infra-operator/apis v0.6.1-0.20251205192058-5cfbada0ab96
	github.com/openstack-k8s-operators/ironic-operator/api v0.6.1-0.20251203164336-97b491f161c0
	github.com/openstack-k8s-operators/keystone-operator/api v0.6.1-0.20251128160419-8b3a77972a77
	github.com/openstack-k8s-operators/lib-common/modules/ansible v0.6.1-0.20251122131503-b76943960b6c
	github.com/openstack-k8s-operators/lib-common/modules/certmanager v0.6.1-0.20251122131503-b76943960b6c
	github.com/openstack-k8s-operators/lib-common/modules/common v0.6.1-0.20251122131503-b76943960b6c
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.6.1-0.20251122131503-b76943960b6c
	github.com/openstack-k8s-operators/lib-common/modules/test v0.6.1-0.20251122131503-b76943960b6c
	github.com/openstack-k8s-operators/manila-operator/api v0.6.1-0.20251206062608-c96d36e3726a
	github.com/openstack-k8s-operators/mariadb-operator/api v0.6.1-0.20251202153403-32849708ca7a
	github.com/openstack-k8s-operators/neutron-operator/api v0.6.1-0.20251125150830-633e42336356
	github.com/openstack-k8s-operators/nova-operator/api v0.6.1-0.20251127143706-407c63ad016a
	github.com/openstack-k8s-operators/octavia-operator/api v0.6.1-0.20251127161151-38d49bbc1c5d
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.6.1-0.20251203114842-732f07ff0113
	github.com/openstack-k8s-operators/openstack-operator/api v0.0.0-00010101000000-000000000000
	github.com/openstack-k8s-operators/ovn-operator/api v0.6.1-0.20251127135801-f3d54911d811
	github.com/openstack-k8s-operators/placement-operator/api v0.6.1-0.20251125174406-42e6ab0985af
	github.com/openstack-k8s-operators/swift-operator/api v0.6.1-0.20251204110720-8ff616797937
	github.com/openstack-k8s-operators/telemetry-operator/api v0.6.1-0.20251204094249-d41273755bc1
	github.com/openstack-k8s-operators/test-operator/api v0.6.1-0.20251127114833-80f76294e428
	github.com/openstack-k8s-operators/watcher-operator/api v0.6.1-0.20251208042300-1636bedad09b
	github.com/pkg/errors v0.9.1
	github.com/rabbitmq/cluster-operator/v2 v2.16.0
	github.com/stretchr/testify v1.11.1
	go.uber.org/zap v1.27.1
	gopkg.in/yaml.v3 v3.0.1
	k8s.io/api v0.31.14
	k8s.io/apimachinery v0.31.14
	k8s.io/client-go v0.31.14
	k8s.io/utils v0.0.0-20250820121507-0af2bda4dd1d
	sigs.k8s.io/controller-runtime v0.19.7
)

require (
	github.com/Masterminds/semver/v3 v3.4.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/fsnotify/fsnotify v1.9.0 // indirect
	github.com/fxamacker/cbor/v2 v2.9.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.10 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-openapi/jsonpointer v0.21.1 // indirect
	github.com/go-openapi/jsonreference v0.21.0 // indirect
	github.com/go-openapi/swag v0.23.1 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-task/slim-sprig/v3 v3.0.0 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/google/cel-go v0.20.1 // indirect
	github.com/google/gnostic-models v0.7.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/pprof v0.0.0-20250403155104-27863c87afa6 // indirect
	github.com/gophercloud/gophercloud/v2 v2.8.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mailru/easyjson v0.9.0 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.9.3 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.5.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.3-0.20250322232337-35a7c28c31ee // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.6.1-0.20251103072528-9eb684fef4ef // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.2 // indirect
	github.com/prometheus/common v0.65.0 // indirect
	github.com/prometheus/procfs v0.16.1 // indirect
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.71.0-rhobs1 // indirect
	github.com/rhobs/observability-operator v0.3.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.58.0 // indirect
	go.opentelemetry.io/otel v1.34.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.28.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.27.0 // indirect
	go.opentelemetry.io/otel/metric v1.34.0 // indirect
	go.opentelemetry.io/otel/sdk v1.34.0 // indirect
	go.opentelemetry.io/otel/trace v1.34.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.yaml.in/yaml/v2 v2.4.2 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/exp v0.0.0-20241217172543-b2144cdd0a67 // indirect
	golang.org/x/mod v0.28.0 // indirect
	golang.org/x/net v0.45.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
	golang.org/x/term v0.36.0 // indirect
	golang.org/x/text v0.30.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/tools v0.37.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.5.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250115164207-1a7da9e5054f // indirect
	google.golang.org/grpc v1.71.1 // indirect
	google.golang.org/protobuf v1.36.7 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	k8s.io/apiextensions-apiserver v0.33.2 // indirect
	k8s.io/apiserver v0.33.2 // indirect
	k8s.io/component-base v0.33.2 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20250902184714-7fc278399c7f // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.30.3 // indirect
	sigs.k8s.io/gateway-api v1.2.1 // indirect
	sigs.k8s.io/json v0.0.0-20241014173422-cfa47c3a1cc8 // indirect
	sigs.k8s.io/randfill v1.0.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.7.0 // indirect
	sigs.k8s.io/yaml v1.6.0 // indirect
)

replace github.com/openstack-k8s-operators/openstack-operator/api => ./api //allow-merging

// mschuppert: map to latest commit from release-4.18 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20250711200046-c86d80652a9e //allow-merging

// custom RabbitmqClusterSpecCore for OpenStackControlplane (v2.16.0_patches)
replace github.com/rabbitmq/cluster-operator/v2 => github.com/openstack-k8s-operators/rabbitmq-cluster-operator/v2 v2.6.1-0.20250929174222-a0d328fa4dec //allow-merging

// pin to support rabbitmq 2.16.0 rebase
replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20250627150254-e9823e99808e //allow-merging

replace k8s.io/apimachinery => k8s.io/apimachinery v0.31.13 //allow-merging

replace k8s.io/api => k8s.io/api v0.31.13 //allow-merging

replace k8s.io/apiserver => k8s.io/apiserver v0.31.13 //allow-merging

replace k8s.io/client-go => k8s.io/client-go v0.31.13 //allow-merging

replace k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.31.13 //allow-merging

replace k8s.io/cli-runtime => k8s.io/cli-runtime v0.31.13 //allow-merging

replace k8s.io/code-generator => k8s.io/code-generator v0.31.13 //allow-merging

replace k8s.io/component-base => k8s.io/component-base v0.31.13 //allow-merging

replace github.com/cert-manager/cmctl/v2 => github.com/cert-manager/cmctl/v2 v2.1.2-0.20241127223932-88edb96860cf //allow-merging
