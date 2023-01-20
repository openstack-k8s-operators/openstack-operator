module github.com/openstack-k8s-operators/openstack-operator

go 1.18

require (
	github.com/ghodss/yaml v1.0.0
	github.com/go-logr/logr v1.2.3
	github.com/imdario/mergo v0.3.13
	github.com/openstack-k8s-operators/cinder-operator/api v0.0.0-20230116172529-ad6adb43ab7f
	github.com/openstack-k8s-operators/glance-operator/api v0.0.0-20230117132931-a7c3745ebcf5
	github.com/openstack-k8s-operators/keystone-operator/api v0.0.0-20230120095729-d9c56b54cc8d
	github.com/openstack-k8s-operators/lib-common/modules/common v0.0.0-20230120104300-c5aa132b34d6
	github.com/openstack-k8s-operators/mariadb-operator/api v0.0.0-20230118093207-9764a3d19c85
	github.com/openstack-k8s-operators/neutron-operator/api v0.0.0-20221130112258-2d20ea7691aa
	github.com/openstack-k8s-operators/nova-operator/api v0.0.0-20230117122627-555970b72139
	github.com/openstack-k8s-operators/openstack-operator/apis v0.0.0-20230119191250-f6f88d501bff
	github.com/openstack-k8s-operators/ovn-operator/api v0.0.0-20221130085454-49cd8b9c841c
	github.com/openstack-k8s-operators/ovs-operator/api v0.0.0-20221221132409-30464f27d7d5
	github.com/openstack-k8s-operators/placement-operator/api v0.0.0-20230110163707-b184c1b7a7ea
	github.com/operator-framework/api v0.17.3
	github.com/rabbitmq/cluster-operator v1.14.0
	go.uber.org/zap v1.24.0
	k8s.io/api v0.25.5
	k8s.io/apimachinery v0.26.0
	k8s.io/client-go v0.25.4
	sigs.k8s.io/controller-runtime v0.13.1
)

require (
	cloud.google.com/go v0.99.0 // indirect
	github.com/Azure/go-autorest v14.2.0+incompatible // indirect
	github.com/Azure/go-autorest/autorest v0.11.28 // indirect
	github.com/Azure/go-autorest/autorest/adal v0.9.21 // indirect
	github.com/Azure/go-autorest/autorest/date v0.3.0 // indirect
	github.com/Azure/go-autorest/logger v0.2.1 // indirect
	github.com/Azure/go-autorest/tracing v0.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v4 v4.4.2 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.5.9 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/gophercloud/gophercloud v1.0.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/openshift/api v3.9.0+incompatible // indirect
	github.com/openstack-k8s-operators/lib-common/modules/openstack v0.0.0-20220928054455-66e396fb480e // indirect
	github.com/openstack-k8s-operators/lib-common/modules/storage v0.0.0-20221212162305-ec57ccd85ad5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.13.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.8.0 // indirect
	golang.org/x/crypto v0.0.0-20220722155217-630584e8d5aa // indirect
	golang.org/x/net v0.3.1-0.20221206200815-1e63c2f08a10 // indirect
	golang.org/x/oauth2 v0.0.0-20220909003341-f21342109be1 // indirect
	golang.org/x/sys v0.3.0 // indirect
	golang.org/x/term v0.3.0 // indirect
	golang.org/x/text v0.5.0 // indirect
	golang.org/x/time v0.0.0-20220922220347-f3bd1da661af // indirect
	gomodules.xyz/jsonpatch/v2 v2.2.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiextensions-apiserver v0.25.2 // indirect
	k8s.io/component-base v0.25.2 // indirect
	k8s.io/klog/v2 v2.80.1 // indirect
	k8s.io/kube-openapi v0.0.0-20221012153701-172d655c2280 // indirect
	k8s.io/utils v0.0.0-20221107191617-1a15be271d1d // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)

replace github.com/openstack-k8s-operators/openstack-operator/apis => ./apis
