package util //nolint:revive // util is an acceptable package name in this context

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	ocpidms "github.com/openshift/api/config/v1"
	mc "github.com/openshift/api/machineconfiguration/v1"
	ocpicsp "github.com/openshift/api/operator/v1alpha1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

// IsDisconnectedOCP - Will retrieve a CR's related to disconnected OCP deployments. If the list is not
// empty, we can infer that the OCP cluster is a disconnected deployment.
// Returns false without error if the CRDs don't exist (non-OpenShift cluster).
func IsDisconnectedOCP(ctx context.Context, helper *helper.Helper) (bool, error) {
	icspList := ocpicsp.ImageContentSourcePolicyList{}
	idmsList := ocpidms.ImageDigestMirrorSetList{}

	listOpts := []client.ListOption{}

	var icspCount, idmsCount int

	err := helper.GetClient().List(ctx, &icspList, listOpts...)
	if err != nil {
		// If the CRD doesn't exist, this is not an OpenShift cluster or ICSP is not available
		// This is not an error condition - just means we're not in a disconnected environment
		if IsNoMatchError(err) {
			helper.GetLogger().Info("ImageContentSourcePolicy CRD not available, assuming not a disconnected environment")
		} else {
			return false, err
		}
	} else {
		icspCount = len(icspList.Items)
	}

	err = helper.GetClient().List(ctx, &idmsList, listOpts...)
	if err != nil {
		// If the CRD doesn't exist, this is not an OpenShift cluster or IDMS is not available
		if IsNoMatchError(err) {
			helper.GetLogger().Info("ImageDigestMirrorSet CRD not available, assuming not a disconnected environment")
		} else {
			return false, err
		}
	} else {
		idmsCount = len(idmsList.Items)
	}

	if icspCount != 0 || idmsCount != 0 {
		return true, nil
	}

	return false, nil
}

// IsNoMatchError checks if the error indicates that a CRD/resource type doesn't exist
func IsNoMatchError(err error) bool {
	errStr := err.Error()
	// Check for "no matches for kind" type errors which indicate the CRD doesn't exist.
	// Also check for "no kind is registered" which occurs when the type isn't in the scheme.
	// This is specifically needed for functional tests where the fake client returns a different
	// error type than a real Kubernetes API server when CRDs are not installed.
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
