/*
Copyright 2025.

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

package lightspeed

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	lightspeedv1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/lightspeed/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// OpenStackLightspeedDefaultProvider - contains default name for the provider created in OLSConfig
	// by openstack-operator.
	OpenStackLightspeedDefaultProvider = "openstack-lightspeed-provider"

	// OpenStackLightspeedOwnerIDLabel - name of a label that contains ID of OpenStackLightspeed instance
	// that manages the OLSConfig.
	OpenStackLightspeedOwnerIDLabel = "openstack.org/lightspeed-owner-id"

	// OpenStackLightspeedVectorDBPath - path inside of the container image where the vector DB are
	// located
	OpenStackLightspeedVectorDBPath = "/rag/vector_db/os_product_docs"

	// OLSConfigName - OLS forbids other name for OLSConfig instance than OLSConfigName
	OLSConfigName = "cluster"
)

// GetOLSConfig returns OLSConfig if there is one present in the cluster.
func GetOLSConfig(ctx context.Context, helper *common_helper.Helper) (uns.Unstructured, error) {
	OLSConfigGVR := schema.GroupVersionResource{
		Group:    "ols.openshift.io",
		Version:  "v1alpha1",
		Resource: "olsconfigs",
	}

	OLSConfigList := &uns.UnstructuredList{}
	OLSConfigList.SetGroupVersionKind(OLSConfigGVR.GroupVersion().WithKind("OLSConfig"))
	err := helper.GetClient().List(ctx, OLSConfigList)
	if err != nil {
		return uns.Unstructured{}, err
	}

	if OLSConfigList.Items != nil && len(OLSConfigList.Items) > 0 {
		return OLSConfigList.Items[0], nil
	}

	return uns.Unstructured{}, k8s_errors.NewNotFound(
		schema.GroupResource{Group: "ols.openshifg.io", Resource: "olsconfigs"},
		"OLSConfig")
}

// IsOLSOperatorInstalled checks whether OLS Operator is already running in the cluster.
func IsOLSOperatorInstalled(ctx context.Context, helper *common_helper.Helper) (bool, error) {
	csvGVR := schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "clusterserviceversions",
	}

	csvList := &uns.UnstructuredList{}
	csvList.SetGroupVersionKind(csvGVR.GroupVersion().WithKind("clusterserviceversion"))
	err := helper.GetClient().List(ctx, csvList)
	if err != nil {
		return false, err
	}

	for _, csv := range csvList.Items {
		if strings.HasPrefix(csv.GetName(), "lightspeed-operator") {
			return true, nil
		}
	}

	return false, nil
}

// PatchOLSConfig patches OLSConfig with information from OpenStackLightspeed instance.
func PatchOLSConfig(
	olsConfig *uns.Unstructured,
	instance *lightspeedv1beta1.OpenStackLightspeed,
	helper *common_helper.Helper,
) error {
	// 1. Patch the Providers section
	providersPatch := []interface{}{
		map[string]interface{}{
			"credentialsSecretRef": map[string]interface{}{
				"name": StringPtrValue(instance.Spec.LLMCredentials),
			},
			"models": []interface{}{
				map[string]interface{}{
					"name":       StringPtrValue(instance.Spec.ModelName),
					"parameters": map[string]interface{}{},
				},
			},
			"name": OpenStackLightspeedDefaultProvider,
			"type": StringPtrValue(instance.Spec.LLMEndpointType),
			"url":  StringPtrValue(instance.Spec.LLMEndpoint),
		},
	}

	if err := uns.SetNestedSlice(olsConfig.Object, providersPatch, "spec", "llm", "providers"); err != nil {
		return err
	}

	indexID, err := getIndexID(instance.Spec.RAGImage)
	if err != nil {
		return err
	}

	// 2. Patch the RAG section
	openstackRAG := []interface{}{
		map[string]interface{}{
			"image":     instance.Spec.RAGImage,
			"indexID":   indexID,
			"indexPath": OpenStackLightspeedVectorDBPath,
		},
	}

	if err := uns.SetNestedSlice(olsConfig.Object, openstackRAG, "spec", "ols", "rag"); err != nil {
		return err
	}

	tlsCaCertBundle := StringPtrValue(instance.Spec.TLSCACertBundle)
	err = uns.SetNestedField(olsConfig.Object, tlsCaCertBundle, "spec", "ols", "additionalCAConfigMapRef", "name")
	if err != nil {
		return err
	}

	modelName := StringPtrValue(instance.Spec.ModelName)
	err = uns.SetNestedField(olsConfig.Object, modelName, "spec", "ols", "defaultModel")
	if err != nil {
		return err
	}

	err = uns.SetNestedField(olsConfig.Object, OpenStackLightspeedDefaultProvider, "spec", "ols", "defaultProvider")
	if err != nil {
		return err
	}

	// 3. Add info which OpenStackLightspeed instance owns the OLSConfig
	labels := olsConfig.GetLabels()
	updatedLabels := map[string]interface{}{
		OpenStackLightspeedOwnerIDLabel: string(instance.GetUID()),
	}
	for k, v := range labels {
		updatedLabels[k] = v
	}

	err = uns.SetNestedField(olsConfig.Object, updatedLabels, "metadata", "labels")
	if err != nil {
		return err
	}

	// 4. Add OpenStack finalizers
	if !controllerutil.AddFinalizer(olsConfig, helper.GetFinalizer()) && instance.Status.Conditions == nil {
		return fmt.Errorf("cannot add finalizer")
	}

	return nil
}

// IsOLSConfigReady returns true if required conditions are true for OLSConfig
func IsOLSConfigReady(ctx context.Context, helper *common_helper.Helper) (bool, error) {
	olsConfig, err := GetOLSConfig(ctx, helper)
	if err != nil {
		return false, err
	}

	olsConfigStatusList, found, err := uns.NestedSlice(olsConfig.Object, "status", "conditions")
	if !found {
		return false, err
	}

	jsonData, err := json.Marshal(olsConfigStatusList)
	if err != nil {
		return false, fmt.Errorf("failed to marshal OLSConfig status: %w", err)
	}

	var OLSConfigConditions []metav1.Condition
	err = json.Unmarshal(jsonData, &OLSConfigConditions)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshal JSON containing condition.Conditions: %w", err)
	}

	requiredConditionTypes := []string{"ConsolePluginReady", "CacheReady", "ApiReady", "Reconciled"}
	for _, OLSConfigCondition := range OLSConfigConditions {
		for _, requiredConditionType := range requiredConditionTypes {
			if OLSConfigCondition.Type == requiredConditionType && OLSConfigCondition.Status != metav1.ConditionTrue {
				return false, nil
			}
		}
	}

	return true, nil
}

// getIndexID - returns index ID for the data stored in the vector DB container image.
// It expects that the index ID is equal to the image tag.
func getIndexID(imageName string) (string, error) {
	imageNameSections := strings.Split(imageName, ":")
	if len(imageNameSections) != 2 {
		return "", fmt.Errorf("failed to discvoer index ID")
	}

	return imageNameSections[1], nil
}

// StringPtrValue - dereference safely string pointer
func StringPtrValue(s *string) string {
	if s == nil {
		return ""
	}

	return *s
}
