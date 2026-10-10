package util //nolint:revive // util is an acceptable package name in this context

import (
	"context"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const fallbackAnsibleForks int64 = 5

// defaultAnsibleForks uses the CPU request, capped by a lower CPU limit.
// Fractional cores are rounded up; an unspecified CPU falls back to five.
func defaultAnsibleForks(resources corev1.ResourceRequirements) (int64, bool) {
	request, hasRequest := usableCPU(resources.Requests)
	limit, hasLimit := usableCPU(resources.Limits)
	chosen, ok := limit, hasLimit
	if hasRequest && (!ok || request.Cmp(limit) < 0) {
		chosen, ok = request, true
	}
	if !ok {
		return 0, false
	}
	// Quantity.Value() is the ceiling of the core count, which is what forks want.
	return chosen.Value(), true
}

// usableCPU checks if a resource value (resources or limits was set)
func usableCPU(list corev1.ResourceList) (resource.Quantity, bool) {
	cpu, found := list[corev1.ResourceCPU]
	return cpu, found && cpu.Sign() > 0
}

// addDefaultAnsibleForks leaves explicit NodeSet Env and ConfigMap settings
// untouched (including empty/invalid user values). Env takes precedence over
// EnvFrom at kubelet startup, so only add an explicit default when both lack it.
func (a *EEJob) addDefaultAnsibleForks(ctx context.Context, k8sClient client.Client) error {
	for _, env := range a.Env {
		if env.Name == "ANSIBLE_FORKS" {
			return nil
		}
	}
	configMap := &corev1.ConfigMap{}
	err := k8sClient.Get(ctx, types.NamespacedName{Namespace: a.Namespace, Name: a.EnvConfigMapName}, configMap)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to read Ansible EE environment ConfigMap %s/%s: %w", a.Namespace, a.EnvConfigMapName, err)
	}
	if err == nil {
		if _, present := configMap.Data["ANSIBLE_FORKS"]; present {
			return nil
		}
	}
	forks, err := defaultAnsibleForks(a.Resources)
	if err != nil {
		return fmt.Errorf("failed to derive Ansible forks: %w", err)
	}
	a.Env = append(append([]corev1.EnvVar(nil), a.Env...), corev1.EnvVar{Name: "ANSIBLE_FORKS", Value: strconv.FormatInt(forks, 10)})
	return nil
}
