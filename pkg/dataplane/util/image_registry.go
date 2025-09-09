package util

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
func IsDisconnectedOCP(ctx context.Context, helper *helper.Helper) (bool, error) {
	icspList := ocpicsp.ImageContentSourcePolicyList{}
	idmsList := ocpidms.ImageDigestMirrorSetList{}

	listOpts := []client.ListOption{}

	var err error
	err = helper.GetClient().List(ctx, &icspList, listOpts...)
	if err != nil {
		return false, err
	}
	err = helper.GetClient().List(ctx, &idmsList, listOpts...)
	if err != nil {
		return false, err
	}

	if len(icspList.Items) != 0 || len(idmsList.Items) != 0 {
		return true, err
	}

	return false, err
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
