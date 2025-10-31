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

package deployment

import (
	"context"

	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane/util"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

// GetDeploymentHashesForService - Hash the ConfigMaps and Secrets for the provided service
func GetDeploymentHashesForService(
	ctx context.Context,
	helper *helper.Helper,
	namespace string,
	serviceName string,
	configMapHashes map[string]string,
	secretHashes map[string]string,
	nodeSets dataplanev1.OpenStackDataPlaneNodeSetList,
) error {

	namespacedName := types.NamespacedName{
		Name:      serviceName,
		Namespace: namespace,
	}
	service := &dataplanev1.OpenStackDataPlaneService{}
	err := helper.GetClient().Get(ctx, namespacedName, service)
	if err != nil {
		helper.GetLogger().Error(err, "Unable to retrieve OpenStackDataPlaneService %v")
		return err
	}

	for _, dataSource := range service.Spec.DataSources {
		cm, sec, err := dataplaneutil.GetDataSourceCmSecret(ctx, helper, namespace, dataSource)
		if err != nil {
			return err
		}

		if cm != nil {
			configMapHashes[cm.Name], err = configmap.Hash(cm)
			if err != nil {
				helper.GetLogger().Error(err, "Unable to hash ConfigMap %v")
			}
		}
		if sec != nil {
			secretHashes[sec.Name], err = secret.Hash(sec)
			if err != nil {
				helper.GetLogger().Error(err, "Unable to hash Secret %v")
			}
		}
	}

	if service.Spec.TLSCerts != nil {
		for certKey := range service.Spec.TLSCerts {
			var secrets *corev1.SecretList
			for _, nodeSet := range nodeSets.Items {
				labelSelectorMap := map[string]string{
					NodeSetLabel:    nodeSet.Name,
					ServiceLabel:    serviceName,
					ServiceKeyLabel: certKey,
				}
				secrets, err = secret.GetSecrets(ctx, helper, "", labelSelectorMap)
				if err != nil {
					helper.GetLogger().Error(err, "Unable to search for cert secrets %v")
					return err
				}
				for _, sec := range secrets.Items {
					// get secret?  or is it already there
					secretHashes[sec.Name], err = secret.Hash(&sec)
					if err != nil {
						helper.GetLogger().Error(err, "Unable to search for hash cert secrets %v")
						return err
					}

				}

			}
		}
	}

	return nil
}
