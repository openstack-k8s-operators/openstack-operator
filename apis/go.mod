module github.com/openstack-k8s-operators/openstack-operator/apis

go 1.24

require (
	github.com/cert-manager/cert-manager v1.14.7
	github.com/go-playground/validator/v10 v10.25.0
	github.com/onsi/ginkgo/v2 v2.20.1
	github.com/onsi/gomega v1.34.1
	github.com/openstack-k8s-operators/barbican-operator/api v0.6.1-0.20250916162815-94a54170a2be
	github.com/openstack-k8s-operators/cinder-operator/api v0.6.1-0.20250916151837-4aab51f61506
	github.com/openstack-k8s-operators/designate-operator/api v0.6.1-0.20250916093535-a95d6d012ab5
	github.com/openstack-k8s-operators/glance-operator/api v0.6.1-0.20250913143437-ca03bf1e106a
	github.com/openstack-k8s-operators/heat-operator/api v0.6.1-0.20250916093004-20a0505e2327
	github.com/openstack-k8s-operators/horizon-operator/api v0.6.1-0.20250911092040-f829125f6046
	github.com/openstack-k8s-operators/infra-operator/apis v0.6.1-0.20250919143843-b81500a16049
	github.com/openstack-k8s-operators/ironic-operator/api v0.6.1-0.20250915220807-7ac28d765257
	github.com/openstack-k8s-operators/keystone-operator/api v0.6.1-0.20250916093250-82a76386143d
	github.com/openstack-k8s-operators/lib-common/modules/common v0.6.1-0.20250922082314-c83d83092a04
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.6.1-0.20250922082314-c83d83092a04
	github.com/openstack-k8s-operators/manila-operator/api v0.6.1-0.20250919144954-99877452c35b
	github.com/openstack-k8s-operators/mariadb-operator/api v0.6.1-0.20250919091623-97dfb39624e3
	github.com/openstack-k8s-operators/neutron-operator/api v0.6.1-0.20250916093251-7cfe04efd126
	github.com/openstack-k8s-operators/nova-operator/api v0.6.1-0.20250917121202-3d3857d13c4c
	github.com/openstack-k8s-operators/octavia-operator/api v0.6.1-0.20250916093536-0423d6aadfb4
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.6.1-0.20250912113640-0e6ddd45ee52
	github.com/openstack-k8s-operators/ovn-operator/api v0.6.1-0.20250918124008-71fb50adbeb5
	github.com/openstack-k8s-operators/placement-operator/api v0.6.1-0.20250905081434-85ee919b626c
	github.com/openstack-k8s-operators/swift-operator/api v0.6.1-0.20250909184158-7873da060764
	github.com/openstack-k8s-operators/telemetry-operator/api v0.6.1-0.20250917072532-5152878cf152
	github.com/openstack-k8s-operators/watcher-operator/api v0.6.1-0.20250919081639-e7427981ed8c
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.71.0-rhobs1 // indirect
	github.com/rhobs/observability-operator v0.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20240719175910-8a7402abbf56
	golang.org/x/tools v0.24.0 // indirect
	k8s.io/api v0.31.12
	k8s.io/apimachinery v0.31.12
	k8s.io/client-go v0.31.12
	k8s.io/utils v0.0.0-20250820121507-0af2bda4dd1d
	sigs.k8s.io/controller-runtime v0.19.7
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.12.0 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.8 // indirect
	github.com/go-logr/logr v1.4.3 // indirect
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
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.6.1-0.20250823121217-7e1cd2e3dd03 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.19.1 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.55.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rabbitmq/cluster-operator/v2 v2.9.0 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stretchr/testify v1.11.1 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	golang.org/x/crypto v0.33.0 // indirect
	golang.org/x/net v0.34.0 // indirect
	golang.org/x/oauth2 v0.21.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
	golang.org/x/term v0.29.0 // indirect
	golang.org/x/text v0.22.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/protobuf v1.34.2 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.31.12 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240322212309-b815d8309940 // indirect
	sigs.k8s.io/gateway-api v1.0.0 // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

// mschuppert: map to latest commit from release-4.18 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20250711200046-c86d80652a9e //allow-merging

// custom RabbitmqClusterSpecCore for OpenStackControlplane (v2.9.0_patches_tag_n)
replace github.com/rabbitmq/cluster-operator/v2 => github.com/openstack-k8s-operators/rabbitmq-cluster-operator/v2 v2.6.1-0.20250717122149-12f70b7f3d8d //allow-merging
