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

// Package openstackclient provides functionality for managing OpenStack client resources
package openstackclient

import (
	"context"
	"fmt"

	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/api/client/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClientPodSpec func
func ClientPodSpec(
	ctx context.Context,
	instance *clientv1.OpenStackClient,
	helper *helper.Helper,
	configHash string,
	mcpTLSCertSecret string,
) corev1.PodSpec {
	envVars := map[string]env.Setter{}
	envVars["OS_CLOUD"] = env.SetValue("default")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)
	envVars["PROMETHEUS_HOST"] = env.SetValue(fmt.Sprintf("%s-prometheus.%s.svc",
		telemetryv1.DefaultServiceName,
		instance.Namespace))
	envVars["PROMETHEUS_PORT"] = env.SetValue(fmt.Sprint(telemetryv1.DefaultPrometheusPort))
	metricStorage := &telemetryv1.MetricStorage{}
	err := helper.GetClient().Get(ctx, client.ObjectKey{
		Namespace: instance.Namespace,
		Name:      telemetryv1.DefaultServiceName,
	}, metricStorage)
	if err == nil && metricStorage.Spec.PrometheusTLS.Enabled() {
		envVars["PROMETHEUS_CA_CERT"] = env.SetValue(tls.DownstreamTLSCABundlePath)
	}

	if instance.Spec.Env != nil {
		for idx := range instance.Spec.Env {
			e := instance.Spec.Env[idx]
			envVars[e.Name] = func(env *corev1.EnvVar) {
				env.Value = e.Value
				env.ValueFrom = e.ValueFrom
			}
		}
	}

	// create Volume and VolumeMounts
	volumes := clientPodVolumes(instance)
	volumeMounts := clientPodVolumeMounts()

	// add CA cert if defined
	if instance.Spec.CaBundleSecretName != "" {
		volumes = append(volumes, instance.Spec.CreateVolume())
		volumeMounts = append(volumeMounts, instance.Spec.CreateVolumeMounts(nil)...)
	}

	podSpec := corev1.PodSpec{
		TerminationGracePeriodSeconds: ptr.To[int64](0),
		ServiceAccountName:            instance.RbacResourceName(),
		Volumes:                       volumes,
		Containers: []corev1.Container{
			{
				Name:    "openstackclient",
				Image:   instance.Spec.ContainerImage,
				Command: []string{"/bin/sleep"},
				Args:    []string{"infinity"},
				SecurityContext: &corev1.SecurityContext{
					RunAsUser:                ptr.To[int64](42401),
					RunAsGroup:               ptr.To[int64](42401),
					RunAsNonRoot:             ptr.To(true),
					AllowPrivilegeEscalation: ptr.To(false),
					Capabilities: &corev1.Capabilities{
						Drop: []corev1.Capability{
							"ALL",
						},
					},
				},
				Env:          env.MergeEnvs([]corev1.EnvVar{}, envVars),
				VolumeMounts: volumeMounts,
			},
		},
		Tolerations: []corev1.Toleration{
			{
				Key:               "node.kubernetes.io/not-ready",
				Operator:          corev1.TolerationOpExists,
				Effect:            corev1.TaintEffectNoExecute,
				TolerationSeconds: &[]int64{120}[0],
			},
			{
				Key:               "node.kubernetes.io/unreachable",
				Operator:          corev1.TolerationOpExists,
				Effect:            corev1.TaintEffectNoExecute,
				TolerationSeconds: &[]int64{120}[0],
			},
		},
	}

	if instance.Spec.MCP != nil && instance.Spec.MCP.Enabled {
		mcpVolumeMounts := []corev1.VolumeMount{
			{
				Name:      "mcp-config",
				MountPath: "/home/cloud-admin/.config/openstack/clouds.yaml",
				SubPath:   "clouds.yaml",
				ReadOnly:  true,
			},
			{
				Name:      "openstack-config-secret",
				MountPath: "/home/cloud-admin/.config/openstack/secure.yaml",
				SubPath:   "secure.yaml",
			},
			{
				Name:      "mcp-config",
				MountPath: "/tmp/mcp-config/config.yaml",
				SubPath:   "config.yaml",
				ReadOnly:  true,
			},
		}

		if instance.Spec.CaBundleSecretName != "" {
			mcpVolumeMounts = append(mcpVolumeMounts, instance.Spec.CreateVolumeMounts(nil)...)
		}

		if mcpTLSCertSecret != "" {
			mcpVolumeMounts = append(mcpVolumeMounts,
				corev1.VolumeMount{
					Name:      "mcp-tls-cert",
					MountPath: "/etc/pki/tls/certs/tls.crt",
					SubPath:   "tls.crt",
					ReadOnly:  true,
				},
				corev1.VolumeMount{
					Name:      "mcp-tls-cert",
					MountPath: "/etc/pki/tls/private/tls.key",
					SubPath:   "tls.key",
					ReadOnly:  true,
				},
			)
		}

		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "mcp-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Name + "-mcp-config",
					},
				},
			},
		})

		if mcpTLSCertSecret != "" {
			podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
				Name: "mcp-tls-cert",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName:  mcpTLSCertSecret,
						DefaultMode: ptr.To[int32](0444),
					},
				},
			})
		}

		mcpEnvVars := []corev1.EnvVar{
			{Name: "HOME", Value: "/home/cloud-admin"},
			{Name: "RHOS_MCPS_CONFIG", Value: "/tmp/mcp-config/config.yaml"},
			{Name: "OS_CLOUD", Value: "default"},
			{Name: "OS_CLIENT_CONFIG_FILE", Value: "/home/cloud-admin/.config/openstack/clouds.yaml"},
		}
		if instance.Spec.CaBundleSecretName != "" {
			mcpEnvVars = append(mcpEnvVars,
				corev1.EnvVar{Name: "OS_CACERT", Value: tls.DownstreamTLSCABundlePath},
				corev1.EnvVar{Name: "REQUESTS_CA_BUNDLE", Value: tls.DownstreamTLSCABundlePath},
			)
		}

		podSpec.Containers = append(podSpec.Containers, corev1.Container{
			Name:    "mcp-server",
			Image:   instance.Spec.MCPContainerImage,
			Command: []string{"rhos-ls-mcps"},
			Env:     mcpEnvVars,
			Ports: []corev1.ContainerPort{
				{Name: "mcp", ContainerPort: 8080, Protocol: corev1.ProtocolTCP},
			},
			SecurityContext: &corev1.SecurityContext{
				RunAsUser:                ptr.To[int64](42401),
				RunAsGroup:               ptr.To[int64](42401),
				RunAsNonRoot:             ptr.To(true),
				AllowPrivilegeEscalation: ptr.To(false),
				Capabilities: &corev1.Capabilities{
					Drop: []corev1.Capability{"ALL"},
				},
			},
			VolumeMounts: mcpVolumeMounts,
		})
	}

	if instance.Spec.NodeSelector != nil {
		podSpec.NodeSelector = *instance.Spec.NodeSelector
	}

	return podSpec
}

// MCPConfigYAML returns the rhos-mcps config.yaml content for the MCP sidecar
func MCPConfigYAML(caBundleSecretName string, mcpTLSEnabled bool) string {
	caCert := ""
	if caBundleSecretName != "" {
		caCert = fmt.Sprintf("\n  ca_cert: %s", tls.DownstreamTLSCABundlePath)
	}
	mcpTLS := ""
	if mcpTLSEnabled {
		mcpTLS = `
tls:
  ssl_certfile: /etc/pki/tls/certs/tls.crt
  ssl_keyfile: /etc/pki/tls/private/tls.key`
	}
	allowedOrigins := `    - "http://*:*"`
	if mcpTLSEnabled {
		allowedOrigins = `    - "http://*:*"
    - "https://*:*"`
	}
	return fmt.Sprintf(`port: 8080
openstack:
  enabled: true
  allow_write: false%s
openshift:
  enabled: false
mcp_transport_security:
  enable_dns_rebinding_protection: false
  allowed_hosts:
    - "*:*"
  allowed_origins:
%s%s
`, caCert, allowedOrigins, mcpTLS)
}

// MCPCloudsYAML returns a clouds.yaml using the given auth URL for the MCP sidecar.
// When caBundleSecretName is set, a cacert path is included for TLS verification.
func MCPCloudsYAML(authURL, projectName, userName, region, caBundleSecretName string) string {
	caCert := ""
	if caBundleSecretName != "" {
		caCert = fmt.Sprintf("\n    cacert: %s", tls.DownstreamTLSCABundlePath)
	}
	return fmt.Sprintf(`clouds:
  default:
    auth:
      auth_url: %s
      project_name: %s
      username: %s
      user_domain_name: Default
      project_domain_name: Default
    region_name: %s%s
`, authURL, projectName, userName, region, caCert)
}

func clientPodVolumeMounts() []corev1.VolumeMount {
	return []corev1.VolumeMount{
		{
			Name:      "openstack-config",
			MountPath: "/home/cloud-admin/.config/openstack/clouds.yaml",
			SubPath:   "clouds.yaml",
		},
		{
			Name:      "openstack-config-secret",
			MountPath: "/home/cloud-admin/.config/openstack/secure.yaml",
			SubPath:   "secure.yaml",
		},
		{
			Name:      "openstack-config-secret",
			MountPath: "/home/cloud-admin/cloudrc",
			SubPath:   "cloudrc",
		},
	}
}

func clientPodVolumes(
	instance *clientv1.OpenStackClient,
) []corev1.Volume {
	return []corev1.Volume{
		{
			Name: "openstack-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: *instance.Spec.OpenStackConfigMap,
					},
				},
			},
		},
		{
			Name: "openstack-config-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: *instance.Spec.OpenStackConfigSecret,
				},
			},
		},
	}
}
