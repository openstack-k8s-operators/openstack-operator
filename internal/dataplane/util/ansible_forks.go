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

const fallbackAnsibleForks int64 = 8

// defaultAnsibleForks uses the CPU request, capped by a lower CPU limit.
// Fractional cores are rounded up; an unspecified CPU falls back to eight.
func defaultAnsibleForks(resources corev1.ResourceRequirements) (int64, error) {
	request, hasRequest := resources.Requests[corev1.ResourceCPU]
	limit, hasLimit := resources.Limits[corev1.ResourceCPU]
	if !hasRequest && !hasLimit {
		return fallbackAnsibleForks, nil
	}

	// Validate both values even when the other value is the smaller one.
	maxCPU := resource.MustParse("2147483647")
	for name, cpu := range map[string]resource.Quantity{"requests.cpu": request, "limits.cpu": limit} {
		if (name == "requests.cpu" && !hasRequest) || (name == "limits.cpu" && !hasLimit) {
			continue
		}
		if cpu.Sign() <= 0 || cpu.Cmp(maxCPU) > 0 {
			return 0, fmt.Errorf("ansibleEEResources.%s must be positive and at most %s", name, maxCPU.String())
		}
	}
	chosen := request
	if !hasRequest || (hasLimit && limit.Cmp(request) < 0) {
		chosen = limit
	}
	// Bounded to int32 cores above, so MilliValue and the addition cannot overflow.
	milli := chosen.MilliValue()
	return (milli + 999) / 1000, nil
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
