/*
Copyright 2026.

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

package assistant

import (
	"bytes"
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	common_secret "github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"

	assistantv1 "github.com/openstack-k8s-operators/openstack-operator/api/assistant/v1beta1"
)

const (
	openShiftServiceCAConfigMapName = "openshift-service-ca.crt"
	openShiftServiceCAKey           = "service-ca.crt"
	assistantCABundleSuffix         = "-ca-bundle"
)

// assistantCABundleSecretName returns a bounded, deterministic name for the
// controller-owned CA bundle. The namespace and CR name are part of the hash
// used by clusterRBACName, so assistants cannot collide.
func assistantCABundleSecretName(instance *assistantv1.OpenStackAssistant) string {
	return clusterRBACName(instance) + assistantCABundleSuffix
}

// mergeCABundles creates a normalized PEM bundle while avoiding an exact
// duplicate of the additional bundle when it is already present in the source.
func mergeCABundles(source, additional []byte) []byte {
	source = bytes.TrimSpace(source)
	additional = bytes.TrimSpace(additional)

	merged := make([]byte, 0, len(source)+len(additional)+2)
	if len(source) > 0 {
		merged = append(merged, source...)
		merged = append(merged, '\n')
	}
	if len(additional) > 0 && !bytes.Contains(source, additional) {
		merged = append(merged, additional...)
		merged = append(merged, '\n')
	}

	return merged
}

// reconcileCABundle creates the CA bundle mounted by the assistant pod. The
// configured Secret remains user-owned; the generated Secret combines it with
// the OpenShift service-serving CA when that CA is available in the namespace.
// On Kubernetes clusters without the OpenShift-injected ConfigMap, the source
// bundle is copied unchanged.
func (r *OpenStackAssistantReconciler) reconcileCABundle(
	ctx context.Context,
	instance *assistantv1.OpenStackAssistant,
) (string, string, error) {
	sourceName := instance.Spec.CaBundleSecretName
	if sourceName == "" {
		return "", "", nil
	}

	generatedName := assistantCABundleSecretName(instance)
	if sourceName == generatedName {
		return "", "", fmt.Errorf(
			"CA bundle source Secret %s/%s conflicts with the controller-owned bundle name",
			instance.Namespace,
			generatedName,
		)
	}

	source := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: sourceName, Namespace: instance.Namespace}, source); err != nil {
		return "", "", err
	}
	sourceBundle, ok := source.Data[tls.CABundleKey]
	if !ok {
		return "", "", fmt.Errorf(
			"field %s not found in Secret %s/%s",
			tls.CABundleKey,
			instance.Namespace,
			sourceName,
		)
	}

	var serviceCABundle []byte
	serviceCA := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      openShiftServiceCAConfigMapName,
		Namespace: instance.Namespace,
	}, serviceCA)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return "", "", err
	}
	if err == nil {
		serviceCABundle = []byte(serviceCA.Data[openShiftServiceCAKey])
	}

	generated := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generatedName,
			Namespace: instance.Namespace,
		},
	}
	op, err := controllerutil.CreateOrPatch(ctx, r.Client, generated, func() error {
		generated.Labels = map[string]string{
			common.AppSelector: "openstackassistant",
		}
		generated.Type = corev1.SecretTypeOpaque
		generated.Data = map[string][]byte{
			tls.CABundleKey: mergeCABundles(sourceBundle, serviceCABundle),
		}
		return controllerutil.SetControllerReference(instance, generated, r.Scheme)
	})
	if err != nil {
		return "", "", fmt.Errorf("error reconciling assistant CA bundle Secret: %w", err)
	}

	bundleHash, err := common_secret.Hash(generated)
	if err != nil {
		return "", "", fmt.Errorf("error hashing assistant CA bundle Secret: %w", err)
	}

	if op != controllerutil.OperationResultNone {
		r.GetLogger(ctx).Info(
			"Assistant CA bundle reconciled",
			"secret", generatedName,
			"operation", op,
		)
	}

	return generatedName, bundleHash, nil
}
