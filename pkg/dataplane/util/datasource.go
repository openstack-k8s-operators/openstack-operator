/*
Copyright 2024.

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

package util

import (
	"context"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	utils "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
)

// GetDataSourceCmSecrets gets the ConfigMaps and Secrets from a DataSource
func GetDataSourceCmSecret(ctx context.Context, helper *helper.Helper, namespace string, dataSource dataplanev1.DataSource) (*v1.ConfigMap, *v1.Secret, error) {

	var configMap *v1.ConfigMap
	var secret *v1.Secret

	client := helper.GetClient()

	switch {
	case dataSource.ConfigMapRef != nil:
		cm := dataSource.ConfigMapRef
		optional := cm.Optional != nil && *cm.Optional
		configMap = &v1.ConfigMap{}
		err := client.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: namespace}, configMap)
		if err != nil {
			if k8s_errors.IsNotFound(err) && optional {
				// ignore error when marked optional
				utils.LogForObject(helper, "Optional ConfigMap not found", configMap)
				return nil, nil, nil
			}
			utils.LogErrorForObject(helper, err, "Required ConfigMap not found", configMap)
			return configMap, secret, err
		}

	case dataSource.SecretRef != nil:
		s := dataSource.SecretRef
		optional := s.Optional != nil && *s.Optional
		secret = &v1.Secret{}
		err := client.Get(ctx, types.NamespacedName{Name: s.Name, Namespace: namespace}, secret)
		if err != nil {
			if k8s_errors.IsNotFound(err) && optional {
				// ignore error when marked optional
				utils.LogForObject(helper, "Optional Secret not found", secret)
				return nil, nil, nil
			}
			utils.LogErrorForObject(helper, err, "Required Secret not found", secret)
			return configMap, secret, err
		}

	}

	return configMap, secret, nil
}
