/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functional_test

import (
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	openstackclientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
)

type Names struct {
	Namespace                 string
	OpenStackControlplaneName types.NamespacedName
	KeystoneAPIName           types.NamespacedName
	MemcachedName             types.NamespacedName
	DBName                    types.NamespacedName
	DBCell1Name               types.NamespacedName
	RabbitMQName              types.NamespacedName
	RabbitMQCell1Name         types.NamespacedName
	ServiceAccountName        types.NamespacedName
	RoleName                  types.NamespacedName
	RoleBindingName           types.NamespacedName
	RootCAPublicName          types.NamespacedName
	RootCAInternalName        types.NamespacedName
	SelfSignedIssuerName      types.NamespacedName
	CABundleName              types.NamespacedName
}

func CreateNames(openstackControlplaneName types.NamespacedName) Names {
	return Names{
		Namespace:                 openstackControlplaneName.Namespace,
		OpenStackControlplaneName: openstackControlplaneName,
		RootCAPublicName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rootca-public"},
		RootCAInternalName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rootca-internal"},
		SelfSignedIssuerName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "selfsigned-issuer"},
		CABundleName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "combined-ca-bundle"},
		ServiceAccountName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      openstackControlplaneName.Name},
		RoleName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      openstackControlplaneName.Name + "-role"},
		RoleBindingName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      openstackControlplaneName.Name + "-rolebinding"},
		KeystoneAPIName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "keystone",
		},
		MemcachedName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "memcached",
		},
		DBName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "openstack",
		},
		DBCell1Name: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "openstack-cell1",
		},
		RabbitMQName: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rabbitmq",
		},
		RabbitMQCell1Name: types.NamespacedName{
			Namespace: openstackControlplaneName.Namespace,
			Name:      "rabbitmq-cell1",
		},
	}
}

func GetDefaultOpenStackClientSpec() map[string]interface{} {
	return map[string]interface{}{}
}

func CreateOpenStackClient(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "client.openstack.org/v1beta1",
		"kind":       "OpenStackClient",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func GetOpenStackClient(name types.NamespacedName) *openstackclientv1.OpenStackClient {
	instance := &openstackclientv1.OpenStackClient{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}

func CreateOpenStackControlPlane(name types.NamespacedName, spec map[string]interface{}) client.Object {

	raw := map[string]interface{}{
		"apiVersion": "core.openstack.org/v1beta1",
		"kind":       "OpenStackControlPlane",
		"metadata": map[string]interface{}{
			"name":      name.Name,
			"namespace": name.Namespace,
		},
		"spec": spec,
	}
	return th.CreateUnstructured(raw)
}

func GetDefaultOpenStackControlPlaneSpec() map[string]interface{} {
	memcachedTemplate := map[string]interface{}{
		"memcached": map[string]interface{}{
			"replicas": 1,
		},
	}
	rabbitTemplate := map[string]interface{}{
		names.RabbitMQName.Name: map[string]interface{}{
			"replicas": 1,
		},
		names.RabbitMQCell1Name.Name: map[string]interface{}{
			"replicas": 1,
		},
	}
	galeraTemplate := map[string]interface{}{
		names.DBName.Name: map[string]interface{}{
			"storageRequest": "500M",
		},
		names.DBCell1Name.Name: map[string]interface{}{
			"storageRequest": "500M",
		},
	}
	keystoneTemplate := map[string]interface{}{
		"databaseInstance": names.KeystoneAPIName.Name,
		"secret":           "osp-secret",
	}
	ironicTemplate := map[string]interface{}{
		"ironicConductors": []interface{}{},
	}

	tlsTemplate := map[string]interface{}{
		"endpoint": map[string]interface{}{
			"public": map[string]interface{}{
				"enabled": true,
			},
		},
	}

	return map[string]interface{}{
		"secret":       "osp-secret",
		"storageClass": "local-storage",
		"tls":          tlsTemplate,
		"galera": map[string]interface{}{
			"enabled":   true,
			"templates": galeraTemplate,
		},
		"rabbitmq": map[string]interface{}{
			"enabled":   true,
			"templates": rabbitTemplate,
		},
		"memcached": map[string]interface{}{
			"enabled":   true,
			"templates": memcachedTemplate,
		},
		"keystone": map[string]interface{}{
			"enabled":  true,
			"template": keystoneTemplate,
		},
		"placement": map[string]interface{}{
			"enabled": false,
		},
		"glance": map[string]interface{}{
			"enabled": false,
		},
		"cinder": map[string]interface{}{
			"enabled": false,
		},
		"ovn": map[string]interface{}{
			"enabled": false,
		},
		"neutron": map[string]interface{}{
			"enabled": false,
		},
		"swift": map[string]interface{}{
			"enabled": false,
		},
		"nova": map[string]interface{}{
			"enabled": false,
		},
		"redis": map[string]interface{}{
			"enabled": false,
		},
		"ironic": map[string]interface{}{
			"enabled":  false,
			"template": ironicTemplate,
		},
		"designate": map[string]interface{}{
			"enabled": false,
		},
	}
}

func GetOpenStackControlPlane(name types.NamespacedName) *corev1.OpenStackControlPlane {
	instance := &corev1.OpenStackControlPlane{}
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, name, instance)).Should(Succeed())
	}, timeout, interval).Should(Succeed())
	return instance
}
