package util //nolint:revive // util is an acceptable package name in this context

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	ocpconfigv1 "github.com/openshift/api/config/v1"
	mc "github.com/openshift/api/machineconfiguration/v1"
	ocpicsp "github.com/openshift/api/operator/v1alpha1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// machineConfigIgnition - holds the relevant parts of the ignition file that we need to create
// the registries.conf on the dataplane nodes.
type machineConfigIgnition struct {
	Ignition struct {
		Version string `json:"version"`
	} `json:"ignition"`
	Storage struct {
		Files []struct {
			Contents struct {
				Compression string `json:"compression,omitempty"`
				Source      string `json:"source"`
			} `json:"contents"`
			Mode      int    `json:"mode,omitempty"`
			Overwrite bool   `json:"overwrite,omitempty"`
			Path      string `json:"path"`
		} `json:"files"`
	} `json:"storage"`
}

// HasMirrorRegistries checks if OCP has IDMS/ICSP mirror registries configured.
// Note: The presence of IDMS/ICSP doesn't necessarily mean the cluster is disconnected.
// Mirror registries may be configured for other reasons (performance, policy, caching, etc.).
// Returns false without error if the CRDs don't exist (non-OpenShift cluster).
func HasMirrorRegistries(ctx context.Context, helper *helper.Helper) (bool, error) {
	// Check IDMS first (current API), then fall back to ICSP (deprecated)
	idmsList := &ocpconfigv1.ImageDigestMirrorSetList{}
	if err := helper.GetClient().List(ctx, idmsList); err != nil {
		if !IsNoMatchError(err) {
			return false, err
		}
		// CRD doesn't exist, continue to check ICSP
	} else if len(idmsList.Items) > 0 {
		return true, nil
	}

	icspList := &ocpicsp.ImageContentSourcePolicyList{}
	if err := helper.GetClient().List(ctx, icspList); err != nil {
		if !IsNoMatchError(err) {
			return false, err
		}
		// CRD doesn't exist, fall through to return false
	} else if len(icspList.Items) > 0 {
		return true, nil
	}

	return false, nil
}

// IsNoMatchError checks if the error indicates that a CRD/resource type doesn't exist
func IsNoMatchError(err error) bool {
	errStr := err.Error()
	// Check for "no matches for kind" type errors which indicate the CRD doesn't exist.
	// Also check for "no kind is registered" which occurs when the type isn't in the scheme.
	return strings.Contains(errStr, "no matches for kind") ||
		strings.Contains(errStr, "no kind is registered")
}

// GetMCRegistryConf - will unmarshal the MachineConfig ignition file the machineConfigIgnition object.
// This is then parsed and the base64 decoded string is returned.
func GetMCRegistryConf(ctx context.Context, helper *helper.Helper) (string, error) {
	var registriesConf string

	masterMachineConfig, err := getMachineConfig(ctx, helper)
	if err != nil {
		return registriesConf, err
	}

	config := machineConfigIgnition{}
	registriesConf, err = config.formatRegistriesConfString(&masterMachineConfig)
	if err != nil {
		return registriesConf, err
	}

	return registriesConf, nil
}

func (mci *machineConfigIgnition) removePrefixFromB64String() (string, error) {
	const b64Prefix string = "data:text/plain;charset=utf-8;base64,"
	if strings.HasPrefix(mci.Storage.Files[0].Contents.Source, b64Prefix) {
		return mci.Storage.Files[0].Contents.Source[len(b64Prefix):], nil
	}

	return "", fmt.Errorf("no b64prefix found in MachineConfig")
}

func (mci *machineConfigIgnition) formatRegistriesConfString(machineConfig *mc.MachineConfig) (string, error) {
	var (
		err             error
		rawConfigString string
		configString    []byte
	)

	err = json.Unmarshal([]byte(machineConfig.Spec.Config.Raw), &mci)
	if err != nil {
		return "", err
	}

	rawConfigString, err = mci.removePrefixFromB64String()
	if err != nil {
		return "", err
	}
	configString, err = base64.StdEncoding.DecodeString(rawConfigString)
	if err != nil {
		return "", err
	}

	return string(configString), nil
}

func masterMachineConfigBuilder(machineConfigRegistries string) mc.MachineConfig {
	return mc.MachineConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineConfigRegistries,
			Namespace: "",
		},
	}
}

func getMachineConfig(ctx context.Context, helper *helper.Helper) (mc.MachineConfig, error) {
	const machineConfigRegistries string = "99-master-generated-registries"

	masterMachineConfig := masterMachineConfigBuilder(machineConfigRegistries)

	err := helper.GetClient().Get(ctx,
		types.NamespacedName{
			Name: masterMachineConfig.Name, Namespace: masterMachineConfig.Namespace,
		}, &masterMachineConfig)
	if err != nil {
		return masterMachineConfig, err
	}

	return masterMachineConfig, nil
}

// GetMirrorRegistryCACerts retrieves CA certificates from image.config.openshift.io/cluster.
// Returns nil without error if:
//   - not on OpenShift (Image CRD doesn't exist)
//   - no additional CA is configured
//   - the referenced ConfigMap doesn't exist
func GetMirrorRegistryCACerts(ctx context.Context, helper *helper.Helper) (map[string]string, error) {
	imageConfig := &ocpconfigv1.Image{}
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "cluster"}, imageConfig); err != nil {
		if IsNoMatchError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get image.config.openshift.io/cluster: %w", err)
	}

	if imageConfig.Spec.AdditionalTrustedCA.Name == "" {
		return nil, nil
	}

	caConfigMap := &corev1.ConfigMap{}
	if err := helper.GetClient().Get(ctx, types.NamespacedName{
		Name:      imageConfig.Spec.AdditionalTrustedCA.Name,
		Namespace: "openshift-config",
	}, caConfigMap); err != nil {
		if k8s_errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get ConfigMap %s in openshift-config: %w",
			imageConfig.Spec.AdditionalTrustedCA.Name, err)
	}

	return caConfigMap.Data, nil
}
