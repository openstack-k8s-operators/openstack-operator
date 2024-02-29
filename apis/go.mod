module github.com/openstack-k8s-operators/openstack-operator/apis

go 1.20

require (
	github.com/onsi/ginkgo/v2 v2.15.0
	github.com/onsi/gomega v1.31.1
	github.com/openstack-k8s-operators/barbican-operator/api v0.0.0-20240226165142-cb4e05f187dc
	github.com/openstack-k8s-operators/cinder-operator/api v0.3.1-0.20240226142027-fd2c289db7cb
	github.com/openstack-k8s-operators/designate-operator/api v0.0.0-20240227172900-6b5267549089
	github.com/openstack-k8s-operators/glance-operator/api v0.3.1-0.20240227173143-0edb7845d02d
	github.com/openstack-k8s-operators/heat-operator/api v0.3.1-0.20240227102739-4385489fe06b
	github.com/openstack-k8s-operators/horizon-operator/api v0.3.1-0.20240227193241-8c506107e152
	github.com/openstack-k8s-operators/infra-operator/apis v0.3.1-0.20240221083751-49edc0df8a12
	github.com/openstack-k8s-operators/ironic-operator/api v0.3.1-0.20240227192424-196de12b349c
	github.com/openstack-k8s-operators/keystone-operator/api v0.3.1-0.20240226160457-b1b853eb4600
	github.com/openstack-k8s-operators/lib-common/modules/common v0.3.1-0.20240227142955-7f3f84347d80
	github.com/openstack-k8s-operators/manila-operator/api v0.3.1-0.20240225212614-b19b1bc5e38f
	github.com/openstack-k8s-operators/mariadb-operator/api v0.3.1-0.20240222094307-76fef735f093
	github.com/openstack-k8s-operators/neutron-operator/api v0.3.1-0.20240227164744-a5d059fe3866
	github.com/openstack-k8s-operators/nova-operator/api v0.3.1-0.20240227141648-d0fc19939cd0
	github.com/openstack-k8s-operators/octavia-operator/api v0.3.1-0.20240226131611-9e1c9e20aeec
	github.com/openstack-k8s-operators/ovn-operator/api v0.3.1-0.20240227150317-d42793e452c2
	github.com/openstack-k8s-operators/placement-operator/api v0.3.1-0.20240226114810-c3d2988d777c
	github.com/openstack-k8s-operators/swift-operator/api v0.3.1-0.20240227123655-fb2f23c4ad89
	github.com/openstack-k8s-operators/telemetry-operator/api v0.3.1-0.20240227142952-0d4f1ece1c02
	github.com/rabbitmq/cluster-operator/v2 v2.5.0
	k8s.io/apimachinery v0.29.0
	sigs.k8s.io/controller-runtime v0.16.5
)

require (
	github.com/evanphx/json-patch v5.7.0+incompatible // indirect
	github.com/go-logr/zapr v1.3.0 // indirect
	github.com/go-task/slim-sprig v0.0.0-20230315185526-52ccab3ef572 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/pprof v0.0.0-20230510103437-eeec1cb781c3 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rhobs/obo-prometheus-operator/pkg/apis/monitoring v0.64.1-rhobs3 // indirect
	github.com/rhobs/observability-operator v0.0.20 // indirect
	github.com/rogpeppe/go-internal v1.12.0 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/exp v0.0.0-20240213143201-ec583247a57a // indirect
	golang.org/x/tools v0.18.0 // indirect
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/emicklei/go-restful/v3 v3.11.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.0 // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gophercloud/gophercloud v1.9.0 // indirect
	github.com/imdario/mergo v0.3.16 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.3.1-0.20240227142955-7f3f84347d80 //indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.3.1-0.20240227142955-7f3f84347d80
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.18.0 // indirect
	github.com/prometheus/client_model v0.5.0 // indirect
	github.com/prometheus/common v0.46.0 // indirect
	github.com/prometheus/procfs v0.12.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
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
	k8s.io/api v0.29.0
	k8s.io/apiextensions-apiserver v0.29.0 //indirect
	k8s.io/client-go v0.29.0
	k8s.io/component-base v0.29.0 //indirect
	k8s.io/klog/v2 v2.120.1 // indirect
	k8s.io/kube-openapi v0.0.0-20240209001042-7a0d5b415232 //indirect
	k8s.io/utils v0.0.0-20240102154912-e7106e64919e //indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd //indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.1 // indirect
	sigs.k8s.io/yaml v1.4.0 // indirect
)

// mschuppert: map to latest commit from release-4.13 tag
// must consistent within modules and service operators
replace github.com/openshift/api => github.com/openshift/api v0.0.0-20230414143018-3367bc7e6ac7 //allow-merging
