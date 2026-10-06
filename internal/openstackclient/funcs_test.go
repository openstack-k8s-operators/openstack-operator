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
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	"k8s.io/utils/ptr"
)

func TestMCPConfigYAMLTLSIsIndependentFromCABundle(t *testing.T) {
	t.Run("CA bundle configures only downstream trust", func(t *testing.T) {
		g := gomega.NewWithT(t)
		config := MCPConfigYAML("openstackclient", "openstack", "combined-ca-bundle", false, nil)

		g.Expect(config).To(gomega.ContainSubstring("ca_cert: " + tls.DownstreamTLSCABundlePath))
		g.Expect(config).NotTo(gomega.ContainSubstring("\ntls:\n"))
		g.Expect(config).NotTo(gomega.ContainSubstring("https://openstackclient-mcp.openstack.svc:8080"))
	})

	t.Run("MCP TLS does not require a downstream CA bundle", func(t *testing.T) {
		g := gomega.NewWithT(t)
		config := MCPConfigYAML("openstackclient", "openstack", "", true, nil)

		g.Expect(config).NotTo(gomega.ContainSubstring("ca_cert:"))
		g.Expect(config).To(gomega.ContainSubstring("\ntls:\n"))
		g.Expect(config).To(gomega.ContainSubstring("ssl_certfile: /etc/pki/tls/mcp/tls.crt"))
		g.Expect(config).To(gomega.ContainSubstring("https://openstackclient-mcp.openstack.svc:8080"))
	})
}

func TestMCPConfigYAMLPrometheus(t *testing.T) {
	t.Run("no MetricStorage omits the prometheus section", func(t *testing.T) {
		g := gomega.NewWithT(t)
		config := MCPConfigYAML("openstackclient", "openstack", "", false, nil)

		g.Expect(config).NotTo(gomega.ContainSubstring("prometheus:"))
	})

	t.Run("MetricStorage without TLS configures host/port but no ca_cert", func(t *testing.T) {
		g := gomega.NewWithT(t)
		metricStorage := &telemetryv1.MetricStorage{}
		config := MCPConfigYAML("openstackclient", "openstack", "", false, metricStorage)

		g.Expect(config).To(gomega.ContainSubstring("  prometheus:\n    host: metric-storage-prometheus.openstack.svc\n    port: 9090"))
		g.Expect(config).NotTo(gomega.ContainSubstring("prometheus:\n    host: metric-storage-prometheus.openstack.svc\n    port: 9090\n    ca_cert:"))
	})

	t.Run("MetricStorage with TLS enabled adds ca_cert under prometheus", func(t *testing.T) {
		g := gomega.NewWithT(t)
		metricStorage := &telemetryv1.MetricStorage{
			Spec: telemetryv1.MetricStorageSpec{
				PrometheusTLS: tls.SimpleService{
					GenericService: tls.GenericService{
						SecretName: ptr.To("prometheus-tls-secret"),
					},
				},
			},
		}
		config := MCPConfigYAML("openstackclient", "openstack", "", false, metricStorage)

		g.Expect(config).To(gomega.ContainSubstring("  prometheus:\n    host: metric-storage-prometheus.openstack.svc\n    port: 9090\n    ca_cert: " + tls.DownstreamTLSCABundlePath))
	})
}
