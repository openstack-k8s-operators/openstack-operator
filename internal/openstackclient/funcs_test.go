/*
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

package openstackclient

import (
	"testing"

	"github.com/onsi/gomega"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
)

func TestMCPConfigYAMLTLSIsIndependentFromCABundle(t *testing.T) {
	t.Run("CA bundle configures only downstream trust", func(t *testing.T) {
		g := gomega.NewWithT(t)
		config := MCPConfigYAML("openstackclient", "openstack", "combined-ca-bundle", false)

		g.Expect(config).To(gomega.ContainSubstring("ca_cert: " + tls.DownstreamTLSCABundlePath))
		g.Expect(config).NotTo(gomega.ContainSubstring("\ntls:\n"))
		g.Expect(config).NotTo(gomega.ContainSubstring("https://openstackclient-mcp.openstack.svc:8080"))
	})

	t.Run("MCP TLS does not require a downstream CA bundle", func(t *testing.T) {
		g := gomega.NewWithT(t)
		config := MCPConfigYAML("openstackclient", "openstack", "", true)

		g.Expect(config).NotTo(gomega.ContainSubstring("ca_cert:"))
		g.Expect(config).To(gomega.ContainSubstring("\ntls:\n"))
		g.Expect(config).To(gomega.ContainSubstring("ssl_certfile: /etc/pki/tls/mcp/tls.crt"))
		g.Expect(config).To(gomega.ContainSubstring("https://openstackclient-mcp.openstack.svc:8080"))
	})
}
