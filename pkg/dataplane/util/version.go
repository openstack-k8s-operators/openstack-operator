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
	"errors"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetVersion - Get OpenStackVersion and assume at most 1 should exist
func GetVersion(ctx context.Context, helper *helper.Helper, namespace string) (*openstackv1.OpenStackVersion, error) {
	log := helper.GetLogger()
	var version *openstackv1.OpenStackVersion
	versions := &openstackv1.OpenStackVersionList{}
	opts := []client.ListOption{
		client.InNamespace(namespace),
	}
	if err := helper.GetClient().List(ctx, versions, opts...); err != nil {
		log.Error(err, "Unable to retrieve OpenStackVersions %w")
		return nil, err
	}
	if len(versions.Items) > 1 {
		errorMsg := "found multiple OpenStackVersions when at most 1 should exist"
		err := errors.New(errorMsg)
		log.Error(err, errorMsg)
		return nil, err
	} else if len(versions.Items) == 1 {
		version = &versions.Items[0]
	} else {
		version = nil
	}

	return version, nil
}

// GetContainerImages - get the container image values considering either the
// OpenStackVersion or the defaults
func GetContainerImages(version *openstackv1.OpenStackVersion) openstackv1.ContainerImages {
	var containerImages openstackv1.ContainerImages

	// Set the containerImages variable for the container images If there is an
	// OpenStackVersion, use the value from there, else use the default value.
	if version != nil {
		containerImages.AnsibleeeImage = version.Status.ContainerImages.AnsibleeeImage
		containerImages.BootcOsContainerImage = version.Status.ContainerImages.BootcOsContainerImage
		containerImages.CeilometerComputeImage = version.Status.ContainerImages.CeilometerComputeImage
		containerImages.CeilometerIpmiImage = version.Status.ContainerImages.CeilometerIpmiImage
		containerImages.EdpmFrrImage = version.Status.ContainerImages.EdpmFrrImage
		containerImages.EdpmIscsidImage = version.Status.ContainerImages.EdpmIscsidImage
		containerImages.EdpmLogrotateCrondImage = version.Status.ContainerImages.EdpmLogrotateCrondImage
		containerImages.EdpmMultipathdImage = version.Status.ContainerImages.EdpmMultipathdImage
		containerImages.EdpmNeutronDhcpAgentImage = version.Status.ContainerImages.EdpmNeutronDhcpAgentImage
		containerImages.EdpmNeutronMetadataAgentImage = version.Status.ContainerImages.EdpmNeutronMetadataAgentImage
		containerImages.EdpmNeutronOvnAgentImage = version.Status.ContainerImages.EdpmNeutronOvnAgentImage
		containerImages.EdpmNeutronSriovAgentImage = version.Status.ContainerImages.EdpmNeutronSriovAgentImage
		containerImages.EdpmNodeExporterImage = version.Status.ContainerImages.EdpmNodeExporterImage
		containerImages.EdpmKeplerImage = version.Status.ContainerImages.EdpmKeplerImage
		containerImages.EdpmPodmanExporterImage = version.Status.ContainerImages.EdpmPodmanExporterImage
		containerImages.EdpmOpenstackNetworkExporterImage = version.Status.ContainerImages.EdpmOpenstackNetworkExporterImage
		containerImages.EdpmOvnBgpAgentImage = version.Status.ContainerImages.EdpmOvnBgpAgentImage
		containerImages.NovaComputeImage = version.Status.ContainerImages.NovaComputeImage
		containerImages.OvnControllerImage = version.Status.ContainerImages.OvnControllerImage
		containerImages.OsContainerImage = version.Status.ContainerImages.OsContainerImage
		containerImages.AgentImage = version.Status.ContainerImages.AgentImage
		containerImages.ApacheImage = version.Status.ContainerImages.ApacheImage
	} else {
		containerImages.AnsibleeeImage = dataplanev1.ContainerImages.AnsibleeeImage
		containerImages.BootcOsContainerImage = dataplanev1.ContainerImages.BootcOsContainerImage
		containerImages.CeilometerComputeImage = dataplanev1.ContainerImages.CeilometerComputeImage
		containerImages.CeilometerIpmiImage = dataplanev1.ContainerImages.CeilometerIpmiImage
		containerImages.EdpmFrrImage = dataplanev1.ContainerImages.EdpmFrrImage
		containerImages.EdpmIscsidImage = dataplanev1.ContainerImages.EdpmIscsidImage
		containerImages.EdpmLogrotateCrondImage = dataplanev1.ContainerImages.EdpmLogrotateCrondImage
		containerImages.EdpmMultipathdImage = dataplanev1.ContainerImages.EdpmMultipathdImage
		containerImages.EdpmNeutronDhcpAgentImage = dataplanev1.ContainerImages.EdpmNeutronDhcpAgentImage
		containerImages.EdpmNeutronMetadataAgentImage = dataplanev1.ContainerImages.EdpmNeutronMetadataAgentImage
		containerImages.EdpmNeutronOvnAgentImage = dataplanev1.ContainerImages.EdpmNeutronOvnAgentImage
		containerImages.EdpmNeutronSriovAgentImage = dataplanev1.ContainerImages.EdpmNeutronSriovAgentImage
		containerImages.EdpmNodeExporterImage = dataplanev1.ContainerImages.EdpmNodeExporterImage
		containerImages.EdpmKeplerImage = dataplanev1.ContainerImages.EdpmKeplerImage
		containerImages.EdpmPodmanExporterImage = dataplanev1.ContainerImages.EdpmPodmanExporterImage
		containerImages.EdpmOpenstackNetworkExporterImage = dataplanev1.ContainerImages.EdpmOpenstackNetworkExporterImage
		containerImages.EdpmOvnBgpAgentImage = dataplanev1.ContainerImages.EdpmOvnBgpAgentImage
		containerImages.NovaComputeImage = dataplanev1.ContainerImages.NovaComputeImage
		containerImages.OvnControllerImage = dataplanev1.ContainerImages.OvnControllerImage
		containerImages.OsContainerImage = dataplanev1.ContainerImages.OsContainerImage
		containerImages.AgentImage = dataplanev1.ContainerImages.AgentImage
		containerImages.ApacheImage = dataplanev1.ContainerImages.ApacheImage

	}

	return containerImages
}
