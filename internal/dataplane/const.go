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

const (
	// ValidateNetworkLabel for ValidateNetwork OpenStackAnsibleEE
	ValidateNetworkLabel = "validate-network"

	// InstallOSLabel for InstallOS OpenStackAnsibleEE
	InstallOSLabel = "install-os"

	// ConfigureOSLabel for ConfigureOS OpenStackAnsibleEE
	ConfigureOSLabel = "configure-os"

	// RunOSLabel for RunOS OpenStackAnsibleEE
	RunOSLabel = "run-os"

	// InstallOpenStackLabel for InstallOpenStack OpenStackAnsibleEE
	InstallOpenStackLabel = "install-openstack"

	// ConfigureOpenStackLabel for ConfigureOpenStack OpenStackAnsibleEE
	ConfigureOpenStackLabel = "configure-openstack"

	// RunOpenStackLabel for RunOpenStack OpenStackAnsibleEE
	RunOpenStackLabel = "run-openstack"

	// NicConfigTemplateFile is the custom nic config file we use when user provided network config templates are provided.
	NicConfigTemplateFile = "/runner/network/nic-config-template"

	// ConfigPaths base path for volume mounts in OpenStackAnsibleEE pod
	ConfigPaths = "/var/lib/openstack/configs"

	// CertPaths base path for cert volume mount in OpenStackAnsibleEE pod
	CertPaths = "/var/lib/openstack/certs"

	// CACertPaths base path for CA cert volume mount in OpenStackAnsibleEE pod
	CACertPaths = "/var/lib/openstack/cacerts"

	// DNSNamesStr value for setting dns values in a cert
	DNSNamesStr = "dnsnames"

	// IPValuesStr value for setting ip addresses in a cert
	IPValuesStr = "ips"

	// NodeSetLabel label for marking secrets to be watched for changes
	NodeSetLabel = "osdpns"

	//ServiceLabel label for marking secrets to be watched for changes
	ServiceLabel = "osdp-service"

	//ServiceKeyLabel label for marking secrets to be watched for changes
	ServiceKeyLabel = "osdp-service-cert-key"

	//HostnameLabel label for marking secrets to be watched for changes
	HostnameLabel = "hostname"
)
