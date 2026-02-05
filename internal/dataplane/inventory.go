/*
Copyright 2023.

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

package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v3"

	infranetworkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/ansible"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	utils "github.com/openstack-k8s-operators/lib-common/modules/common/util"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/internal/dataplane/util"
)

// getAnsibleVarsFrom gets ansible vars from ConfigMap/Secret
func getAnsibleVarsFrom(ctx context.Context, helper *helper.Helper, namespace string, ansible *dataplanev1.AnsibleOpts) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for _, dataSource := range ansible.AnsibleVarsFrom {
		configMap, secret, err := util.GetDataSourceCmSecret(ctx, helper, namespace, dataSource)
		if err != nil {
			return result, err
		}

		if configMap != nil {
			err := processConfigMapData(configMap.Data, dataSource.Prefix, result)
			if err != nil {
				return result, err
			}
		}

		if secret != nil {
			err := processSecretData(secret.Data, dataSource.Prefix, result)
			if err != nil {
				return result, err
			}
		}
	}
	return result, nil
}

// processConfigMapData processes the key-value pairs from ConfigMap and adds them to the result map.
func processConfigMapData(data map[string]string, prefix string, result map[string]any) error {

	var value any

	for k, v := range data {
		if len(prefix) > 0 {
			k = prefix + k
		}

		decoder := json.NewDecoder(strings.NewReader(v))
		decoder.UseNumber()

		err := decoder.Decode(&value)
		if err != nil {
			value = v
		}
		result[k] = value
	}

	return nil
}

// processSecretData processes the key-value pairs from Secret and adds them to the result map.
func processSecretData(data map[string][]byte, prefix string, result map[string]any) error {

	var value any

	for k, v := range data {
		if len(prefix) > 0 {
			k = prefix + k
		}

		decoder := json.NewDecoder(strings.NewReader(string(v[:])))
		decoder.UseNumber()

		err := decoder.Decode(&value)
		if err != nil {
			value = string(v)
		}
		result[k] = value
	}

	return nil
}

// GenerateNodeSetInventory yields a parsed Inventory for role
func GenerateNodeSetInventory(ctx context.Context, helper *helper.Helper,
	instance *dataplanev1.OpenStackDataPlaneNodeSet,
	allIPSets map[string]infranetworkv1.IPSet, dnsAddresses []string,
	containerImages openstackv1.ContainerImages,
	netServiceNetMap map[string]string,
) (string, error) {
	inventory := ansible.MakeInventory()
	nodeSetGroup := inventory.AddGroup(instance.Name)
	groupVars, err := getAnsibleVarsFrom(ctx, helper, instance.Namespace, &instance.Spec.NodeTemplate.Ansible)
	if err != nil {
		utils.LogErrorForObject(helper, err, "could not get ansible group vars from configMap/secret", instance)
		return "", err
	}
	for k, v := range groupVars {
		nodeSetGroup.Vars[k] = v
	}
	err = resolveGroupAnsibleVars(&instance.Spec.NodeTemplate,
		&nodeSetGroup, containerImages, netServiceNetMap)
	if err != nil {
		utils.LogErrorForObject(helper, err, "Could not resolve ansible group vars", instance)
		return "", err
	}

	// add the NodeSet name variable
	nodeSetGroup.Vars["edpm_nodeset_name"] = instance.Name

	hasMirrorRegistries, err := util.HasMirrorRegistries(ctx, helper)
	if err != nil {
		return "", err
	}

	if hasMirrorRegistries {
		registryConfig, err := util.GetMCRegistryConf(ctx, helper)
		if err != nil {
			// CRD not installed (non-OpenShift or no MCO) - log warning and continue.
			// This allows graceful degradation when running on non-OpenShift clusters.
			// Users can manually configure registries.conf via ansibleVars.
			if util.IsNoMatchError(err) {
				helper.GetLogger().Info("Disconnected environment detected but MachineConfig CRD not available. "+
					"Registry configuration will not be propagated to dataplane nodes. "+
					"You may need to configure registries.conf manually using ansibleVars "+
					"(edpm_podman_disconnected_ocp and edpm_podman_registries_conf).",
					"error", err.Error())
			} else {
				// CRD exists but resource not found, or other errors (network issues,
				// permissions, etc.) - return the error. If MCO is installed but the
				// registry MachineConfig doesn't exist, this indicates a misconfiguration.
				return "", fmt.Errorf("failed to get MachineConfig registry configuration: %w", err)
			}
		} else {
			helper.GetLogger().Info("Mirror registries detected via IDMS/ICSP. Using OCP registry configuration.")

			// Use OCP registries.conf for mirror registry deployments
			nodeSetGroup.Vars["edpm_podman_registries_conf"] = registryConfig
			nodeSetGroup.Vars["edpm_podman_disconnected_ocp"] = hasMirrorRegistries
		}
	}

	// add TLS ansible variable
	nodeSetGroup.Vars["edpm_tls_certs_enabled"] = instance.Spec.TLSEnabled
	if instance.Spec.Tags != nil {
		nodeSetGroup.Vars["nodeset_tags"] = instance.Spec.Tags
	}

	// add services list
	nodeSetGroup.Vars["edpm_services"] = instance.Spec.Services

	// add service types list
	serviceTypes := []string{}
	for _, serviceName := range instance.Spec.Services {
		service, err := GetService(ctx, helper, serviceName)
		if err != nil {
			helper.GetLogger().Error(err, fmt.Sprintf("could not get service %s, using name as EDPMServiceType", serviceName))
			serviceTypes = append(serviceTypes, serviceName)
		} else {
			serviceTypes = append(serviceTypes, service.Spec.EDPMServiceType)
		}
	}
	nodeSetGroup.Vars["edpm_service_types"] = serviceTypes

	nodeSetGroup.Vars["ansible_ssh_private_key_file"] = fmt.Sprintf("/runner/env/ssh_key/ssh_key_%s", instance.Name)

	for _, node := range instance.Spec.Nodes {
		host := nodeSetGroup.AddHost(strings.Split(node.HostName, ".")[0])
		hostVars, err := getAnsibleVarsFrom(ctx, helper, instance.Namespace, &node.Ansible)
		if err != nil {
			utils.LogErrorForObject(helper, err, "could not get ansible host vars from configMap/secret", instance)
			return "", err
		}
		for k, v := range hostVars {
			host.Vars[k] = v
		}
		// Use ansible_host if provided else use hostname. Fall back to
		// nodeName if all else fails.
		if node.Ansible.AnsibleHost != "" {
			host.Vars["ansible_host"] = node.Ansible.AnsibleHost
		} else {
			host.Vars["ansible_host"] = node.HostName
		}

		err = resolveHostAnsibleVars(&node, &host, netServiceNetMap)
		if err != nil {
			utils.LogErrorForObject(helper, err, "could not resolve ansible host vars", instance)
			return "", err
		}

		ipSet, ok := allIPSets[node.HostName]
		if !ok {
			err := fmt.Errorf("no IPSet found for host: %s", node.HostName)
			return "", err
		}
		populateInventoryFromIPAM(&ipSet, host, dnsAddresses, node.HostName)
	}

	invData, err := inventory.MarshalYAML()
	if err != nil {
		utils.LogErrorForObject(helper, err, "Could not parse NodeSet inventory", instance)
		return "", err
	}
	secretData := map[string]string{
		"inventory": string(invData),
	}
	secretName := fmt.Sprintf("dataplanenodeset-%s", instance.Name)
	labels := map[string]string{
		"openstack.org/operator-name": "dataplane",
		"openstackdataplanenodeset":   instance.Name,
		"inventory":                   "true",
	}
	for key, val := range instance.Labels {
		labels[key] = val
	}
	template := []utils.Template{
		// Secret
		{
			Name:         secretName,
			Namespace:    instance.Namespace,
			Type:         utils.TemplateTypeNone,
			InstanceType: instance.Kind,
			CustomData:   secretData,
			Labels:       labels,
		},
	}
	err = secret.EnsureSecrets(ctx, helper, instance, template, nil)
	if err == nil {
		instance.Status.InventorySecretName = secretName
	}
	return secretName, err
}

// populateInventoryFromIPAM populates inventory from IPAM
func populateInventoryFromIPAM(
	ipSet *infranetworkv1.IPSet, host ansible.Host,
	dnsAddresses []string, hostName string,
) {
	var dnsSearchDomains []string
	for _, res := range ipSet.Status.Reservation {
		// Build the vars for ips/routes etc
		entry := string(res.ServiceNetwork)
		host.Vars[entry+"_ip"] = res.Address
		_, ipnet, err := net.ParseCIDR(res.Cidr)
		if err == nil {
			netCidr, _ := ipnet.Mask.Size()
			host.Vars[entry+"_cidr"] = netCidr
		}
		if res.Vlan != nil {
			host.Vars[entry+"_vlan_id"] = res.Vlan
		}
		host.Vars[entry+"_mtu"] = res.MTU
		host.Vars[entry+"_gateway_ip"] = res.Gateway
		host.Vars[entry+"_host_routes"] = res.Routes

		if entry == dataplanev1.CtlPlaneNetwork {
			host.Vars[entry+"_dns_nameservers"] = dnsAddresses
			if dataplanev1.NodeHostNameIsFQDN(hostName) {
				host.Vars["canonical_hostname"] = hostName
				domain := strings.SplitN(hostName, ".", 2)[1]
				if domain != res.DNSDomain {
					dnsSearchDomains = append(dnsSearchDomains, domain)
				}
			} else {
				host.Vars["canonical_hostname"] = strings.Join([]string{hostName, res.DNSDomain}, ".")
			}
		}
		dnsSearchDomains = append(dnsSearchDomains, res.DNSDomain)
	}
	host.Vars["dns_search_domains"] = dnsSearchDomains
}

// set group ansible vars from NodeTemplate
func resolveGroupAnsibleVars(template *dataplanev1.NodeTemplate, group *ansible.Group,
	containerImages openstackv1.ContainerImages,
	netServiceNetMap map[string]string,
) error {
	if template.Ansible.AnsibleUser != "" {
		group.Vars["ansible_user"] = template.Ansible.AnsibleUser
	}
	if template.Ansible.AnsiblePort > 0 {
		group.Vars["ansible_port"] = strconv.Itoa(template.Ansible.AnsiblePort)
	}
	if template.ManagementNetwork != "" {
		group.Vars["management_network"] = template.ManagementNetwork
	}

	// Set the ansible variables for the container images if they are not
	// provided by the user in the spec.
	if template.Ansible.AnsibleVars["edpm_frr_image"] == nil {
		group.Vars["edpm_frr_image"] = containerImages.EdpmFrrImage
	}
	if template.Ansible.AnsibleVars["edpm_iscsid_image"] == nil {
		group.Vars["edpm_iscsid_image"] = containerImages.EdpmIscsidImage
	}
	if template.Ansible.AnsibleVars["edpm_logrotate_crond_image"] == nil {
		group.Vars["edpm_logrotate_crond_image"] = containerImages.EdpmLogrotateCrondImage
	}
	if template.Ansible.AnsibleVars["edpm_multipathd_image"] == nil {
		group.Vars["edpm_multipathd_image"] = containerImages.EdpmMultipathdImage
	}
	if template.Ansible.AnsibleVars["edpm_neutron_dhcp_image"] == nil {
		group.Vars["edpm_neutron_dhcp_image"] = containerImages.EdpmNeutronDhcpAgentImage
	}
	if template.Ansible.AnsibleVars["edpm_neutron_metadata_agent_image"] == nil {
		group.Vars["edpm_neutron_metadata_agent_image"] = containerImages.EdpmNeutronMetadataAgentImage
	}
	if template.Ansible.AnsibleVars["edpm_neutron_ovn_agent_image"] == nil {
		group.Vars["edpm_neutron_ovn_agent_image"] = containerImages.EdpmNeutronOvnAgentImage
	}
	if template.Ansible.AnsibleVars["edpm_neutron_sriov_agent_image"] == nil {
		group.Vars["edpm_neutron_sriov_image"] = containerImages.EdpmNeutronSriovAgentImage
	}
	if template.Ansible.AnsibleVars["edpm_nova_compute_image"] == nil {
		group.Vars["edpm_nova_compute_image"] = containerImages.NovaComputeImage
	}
	if template.Ansible.AnsibleVars["edpm_ovn_controller_agent_image"] == nil {
		group.Vars["edpm_ovn_controller_agent_image"] = containerImages.OvnControllerImage
	}
	if template.Ansible.AnsibleVars["edpm_ovn_bgp_agent_image"] == nil {
		group.Vars["edpm_ovn_bgp_agent_image"] = containerImages.EdpmOvnBgpAgentImage
	}
	if template.Ansible.AnsibleVars["edpm_telemetry_ceilometer_compute_image"] == nil {
		group.Vars["edpm_telemetry_ceilometer_compute_image"] = containerImages.CeilometerComputeImage
	}
	if template.Ansible.AnsibleVars["edpm_telemetry_ceilometer_ipmi_image"] == nil {
		group.Vars["edpm_telemetry_ceilometer_ipmi_image"] = containerImages.CeilometerIpmiImage
	}
	if template.Ansible.AnsibleVars["edpm_telemetry_node_exporter_image"] == nil {
		group.Vars["edpm_telemetry_node_exporter_image"] = containerImages.EdpmNodeExporterImage
	}
	if template.Ansible.AnsibleVars["edpm_telemetry_kepler_image"] == nil {
		group.Vars["edpm_telemetry_kepler_image"] = containerImages.EdpmKeplerImage
	}
	if template.Ansible.AnsibleVars["edpm_telemetry_podman_exporter_image"] == nil {
		group.Vars["edpm_telemetry_podman_exporter_image"] = containerImages.EdpmPodmanExporterImage
	}
	if template.Ansible.AnsibleVars["edpm_telemetry_openstack_network_exporter_image"] == nil {
		if containerImages.OpenstackNetworkExporterImage != nil {
			group.Vars["edpm_telemetry_openstack_network_exporter_image"] = containerImages.OpenstackNetworkExporterImage
		}
	}

	err := unmarshalAnsibleVars(template.Ansible.AnsibleVars, group.Vars)
	if err != nil {
		return err
	}
	if len(template.Networks) != 0 {
		nets, serviceNetMap := buildNetworkVars(template.Networks, netServiceNetMap)
		group.Vars["nodeset_networks"] = nets
		// We may get rid of networks_lower var after changing it's usage
		group.Vars["networks_lower"] = serviceNetMap
		group.Vars["network_servicenet_map"] = serviceNetMap
	}

	return nil
}

// set host ansible vars from NodeSection
func resolveHostAnsibleVars(node *dataplanev1.NodeSection,
	host *ansible.Host, netServiceNetMap map[string]string,
) error {
	if node.Ansible.AnsibleUser != "" {
		host.Vars["ansible_user"] = node.Ansible.AnsibleUser
	}
	if node.Ansible.AnsiblePort > 0 {
		host.Vars["ansible_port"] = strconv.Itoa(node.Ansible.AnsiblePort)
	}
	if node.ManagementNetwork != "" {
		host.Vars["management_network"] = node.ManagementNetwork
	}

	err := unmarshalAnsibleVars(node.Ansible.AnsibleVars, host.Vars)
	if err != nil {
		return err
	}
	if len(node.Networks) != 0 {
		nets, serviceNetMap := buildNetworkVars(node.Networks, netServiceNetMap)
		host.Vars["nodeset_networks"] = nets
		// We may get rid of networks_lower var after changing it's usage
		host.Vars["networks_lower"] = serviceNetMap
		host.Vars["network_servicenet_map"] = serviceNetMap
	}
	return nil
}

// unmarshal raw strings into an ansible vars dictionary
func unmarshalAnsibleVars(ansibleVars map[string]json.RawMessage,
	parsedVars map[string]interface{},
) error {
	for key, val := range ansibleVars {
		var v interface{}
		err := yaml.Unmarshal(val, &v)
		if err != nil {
			return err
		}
		parsedVars[key] = v
	}
	return nil
}

func buildNetworkVars(networks []infranetworkv1.IPSetNetwork,
	netServiceNetMap map[string]string,
) ([]string, map[string]string) {
	serviceNetMap := make(map[string]string)
	var nets []string
	for _, network := range networks {
		netName := string(network.Name)
		if netServiceNetMap[strings.ToLower(netName)] == dataplanev1.CtlPlaneNetwork {
			continue
		}
		nets = append(nets, netName)
		serviceNetMap[netName] = netServiceNetMap[strings.ToLower(netName)]
	}
	return nets, serviceNetMap
}
