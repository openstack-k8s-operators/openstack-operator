module github.com/openstack-k8s-operators/openstack-operator

go 1.19

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v1.2.4
	github.com/imdario/mergo v0.3.15
	github.com/openstack-k8s-operators/cinder-operator/api v0.0.0-20230519170041-e9e2440531f1
	github.com/openstack-k8s-operators/dataplane-operator/api v0.0.0-20230519144246-bdb565811532
	github.com/openstack-k8s-operators/glance-operator/api v0.0.0-20230517053641-880c11fddd03
	github.com/openstack-k8s-operators/horizon-operator/api v0.0.0-20230519070602-0041d1c04491
	github.com/openstack-k8s-operators/infra-operator/apis v0.0.0-20230515135008-28b4b15c5915
	github.com/openstack-k8s-operators/ironic-operator/api v0.0.0-20230516124248-417f6ebce386
	github.com/openstack-k8s-operators/keystone-operator/api v0.0.0-20230515212624-62d8100c3ead
	github.com/openstack-k8s-operators/lib-common/modules/common v0.0.0-20230516091855-e51abe875764
	github.com/openstack-k8s-operators/manila-operator/api v0.0.0-20230517165807-ec17d8a9467d
	github.com/openstack-k8s-operators/mariadb-operator/api v0.0.0-20230517052836-8478be8b3aaa
	github.com/openstack-k8s-operators/neutron-operator/api v0.0.0-20230515091936-6fca6f80d9f1
	github.com/openstack-k8s-operators/nova-operator/api v0.0.0-20230517142127-c170e2201e05
	github.com/openstack-k8s-operators/openstack-ansibleee-operator/api v0.0.0-20230511195906-07c2e0aa9a63
	github.com/openstack-k8s-operators/openstack-baremetal-operator/api v0.0.0-20230518053009-43a436586a6c
	github.com/openstack-k8s-operators/openstack-operator/apis v0.0.0-20230518063227-7c3db0fa5069
	github.com/openstack-k8s-operators/ovn-operator/api v0.0.0-20230517053349-0237d13e5f1c
	github.com/openstack-k8s-operators/ovs-operator/api v0.0.0-20230517053110-2821a8dc05fd
	github.com/openstack-k8s-operators/placement-operator/api v0.0.0-20230517142510-5ebb403dc2f7
	github.com/operator-framework/api v0.17.3
	github.com/rabbitmq/cluster-operator v1.14.0
	go.uber.org/zap v1.24.0
	k8s.io/api v0.26.3
	k8s.io/apimachinery v0.26.3
	k8s.io/client-go v0.26.3
	sigs.k8s.io/controller-runtime v0.14.6
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.10.2 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.6.0 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.6 // indirect
	github.com/go-openapi/jsonreference v0.20.2 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gophercloud/gophercloud v1.2.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/metal3-io/baremetal-operator/apis v0.2.0 // indirect
	github.com/metal3-io/baremetal-operator/pkg/hardwareutils v0.1.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.0.0-20230516091855-e51abe875764 // indirect; indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.0.0-20230516091855-e51abe875764 // indirect; indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.15.0 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.9.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/net v0.10.0 // indirect
	golang.org/x/oauth2 v0.7.0 // indirect
	golang.org/x/sys v0.8.0 // indirect
	golang.org/x/term v0.8.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.30.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.26.3 // indirect; indirect // indirect // indirect
	k8s.io/component-base v0.26.3 // indirect; indirect // indirect // indirect
	k8s.io/klog/v2 v2.90.1 // indirect
	k8s.io/kube-openapi v0.0.0-20230327201221-f5883ff37f0c // indirect; indirect // indirect // indirect // indirect // indirect // indirect
	k8s.io/utils v0.0.0-20230406110748-d93618cff8a2 // indirect; indirect // indirect // indirect // indirect
	sigs.k8s.io/json v0.0.0-20221116044647-bc3834ca7abd // indirect; indirect // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/openstack-k8s-operators/openstack-operator/apis => ./apis
