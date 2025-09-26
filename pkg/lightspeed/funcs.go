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

// Package lightspeed provides utilities and functions for OpenStack Lightspeed operations
package lightspeed

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	common_helper "github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	lightspeedv1 "github.com/openstack-k8s-operators/openstack-operator/apis/lightspeed/v1beta1"
	corev1 "k8s.io/api/core/v1"
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

	// OpenStackLightspeedJobName - name of the pod that is used to discover environment variables inside of the RAG
	// container image
	OpenStackLightspeedJobName = "openstack-lightspeed"

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

	if len(OLSConfigList.Items) > 0 {
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
	helper *common_helper.Helper,
	instance *lightspeedv1.OpenStackLightspeed,
	olsConfig *uns.Unstructured,
	indexID string,
) error {
	// 1. Patch the Providers section
	providersPatch := []any{
		map[string]any{
			"credentialsSecretRef": map[string]any{
				"name": instance.Spec.LLMCredentials,
			},
			"models": []any{
				map[string]any{
					"name":       instance.Spec.ModelName,
					"parameters": map[string]any{},
				},
			},
			"name": OpenStackLightspeedDefaultProvider,
			"type": instance.Spec.LLMEndpointType,
			"url":  instance.Spec.LLMEndpoint,
		},
	}
	if err := uns.SetNestedSlice(olsConfig.Object, providersPatch, "spec", "llm", "providers"); err != nil {
		return err
	}

	// 2. Patch the RAG section
	openstackRAG := []any{
		map[string]any{
			"image":     instance.Spec.RAGImage,
			"indexID":   indexID,
			"indexPath": OpenStackLightspeedVectorDBPath,
		},
	}

	if err := uns.SetNestedSlice(olsConfig.Object, openstackRAG, "spec", "ols", "rag"); err != nil {
		return err
	}

	if instance.Spec.TLSCACertBundle != "" {
		tlsCaCertBundle := instance.Spec.TLSCACertBundle
		err := uns.SetNestedField(olsConfig.Object, tlsCaCertBundle, "spec", "ols", "additionalCAConfigMapRef", "name")
		if err != nil {
			return err
		}
	}

	modelName := instance.Spec.ModelName
	err := uns.SetNestedField(olsConfig.Object, modelName, "spec", "ols", "defaultModel")
	if err != nil {
		return err
	}

	err = uns.SetNestedField(olsConfig.Object, OpenStackLightspeedDefaultProvider, "spec", "ols", "defaultProvider")
	if err != nil {
		return err
	}

	// 3. Add info which OpenStackLightspeed instance owns the OLSConfig
	labels := olsConfig.GetLabels()
	updatedLabels := map[string]any{
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

// ResolveIndexID - returns index ID for the data stored in the vector DB container image. The discovery of the
// index ID is done through spawning a pod with the rag-content image and looking at the INDEX_NAME env variable value.
func ResolveIndexID(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *lightspeedv1.OpenStackLightspeed,
) (string, ctrl.Result, error) {
	result, err := createOLSJob(ctx, helper, instance)
	if err != nil {
		return "", result, err
	}

	podList := &corev1.PodList{}
	labelSelector := client.MatchingLabels{"app": OpenStackLightspeedJobName}
	if err := helper.GetClient().List(ctx, podList, client.InNamespace(instance.Namespace), labelSelector); err != nil {
		return "", ctrl.Result{}, err
	}

	var OLSPod *corev1.Pod
	for _, pod := range podList.Items {
		if pod.Spec.Containers[0].Image == instance.Spec.RAGImage {
			OLSPod = &pod
			break
		}
	}
	if OLSPod == nil {
		return requeueWaitingPod(helper, instance)
	}

	switch OLSPod.Status.Phase {
	case corev1.PodSucceeded:
		indexName, err := extractEnvFromPodLogs(ctx, OLSPod, "INDEX_NAME")
		if err != nil && k8s_errors.IsNotFound(err) {
			return requeueWaitingPod(helper, instance)
		}
		return indexName, ctrl.Result{}, err
	case corev1.PodFailed:
		return "", ctrl.Result{}, fmt.Errorf("failed to start OpenStack Lightpseed RAG pod")
	default:
		return requeueWaitingPod(helper, instance)
	}
}

// extractEnvFromPodLogs - discovers an environment variable value from the pod logs. The pod must be started using
// createOLSJob.
func extractEnvFromPodLogs(ctx context.Context, pod *corev1.Pod, envVarName string) (string, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		return "", err
	}

	k8sClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return "", err
	}

	req := k8sClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{})
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = podLogs.Close() }()

	buf := new(strings.Builder)
	_, err = io.Copy(buf, podLogs)
	if err != nil {
		return "", fmt.Errorf("error in copying logs: %w", err)
	}

	logs := buf.String()
	for envLine := range strings.SplitSeq(logs, "\n") {
		parts := strings.Split(envLine, "=")
		if len(parts) != 2 {
			continue
		}

		if parts[0] == envVarName {
			return parts[1], nil
		}
	}

	return "", fmt.Errorf("env var not discovered: %s", envVarName)
}

// createOLSJob - starts OLS pod with entrypoint that lists environment variables after the start of the pod. It used
// to discover INDEX_NAME value.
func createOLSJob(
	ctx context.Context,
	helper *common_helper.Helper,
	instance *lightspeedv1.OpenStackLightspeed,
) (ctrl.Result, error) {
	imageHash := sha256.Sum256([]byte(instance.Spec.RAGImage))
	imageHashStr := fmt.Sprintf("%x", imageHash)
	imageHashStr = imageHashStr[len(imageHashStr)-9:]
	imageName := fmt.Sprintf("%s-%s", OpenStackLightspeedJobName, imageHashStr)

	ttlSecondsAfterFinished := int32(600) // 10 mins
	activeDeadlineSeconds := int64(1200)  // 20 mins
	OLSPod := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      imageName,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app": OpenStackLightspeedJobName,
			},
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			ActiveDeadlineSeconds:   &activeDeadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": OpenStackLightspeedJobName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "rag-content",
							Image:   instance.Spec.RAGImage,
							Command: []string{"/bin/sh", "-c"},
							Args:    []string{"env"},
						},
					},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(instance, OLSPod, helper.GetScheme()); err != nil {
		return ctrl.Result{}, err
	}

	err := helper.GetClient().Create(ctx, OLSPod)
	if err != nil && !k8s_errors.IsAlreadyExists(err) {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func requeueWaitingPod(helper *common_helper.Helper, instance *lightspeedv1.OpenStackLightspeed) (string, ctrl.Result, error) {
	instance.Status.Conditions.Set(condition.FalseCondition(
		lightspeedv1.OpenStackLightspeedReadyCondition,
		condition.RequestedReason,
		condition.SeverityInfo,
		lightspeedv1.OpenStackLightspeedWaitingVectorDBMessage,
	))
	helper.GetLogger().Info(lightspeedv1.OpenStackLightspeedReadyMessage)
	return "", ctrl.Result{RequeueAfter: 5 * time.Second}, nil
}
