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

package util

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	ansibleeev1 "github.com/openstack-k8s-operators/openstack-ansibleee-operator/api/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
)

// AnsibleExecution creates a OpenStackAnsiblEE CR
func AnsibleExecution(
	ctx context.Context,
	helper *helper.Helper,
	deployment *dataplanev1.OpenStackDataPlaneDeployment,
	service *dataplanev1.OpenStackDataPlaneService,
	sshKeySecrets map[string]string,
	inventorySecrets map[string]string,
	aeeSpec *dataplanev1.AnsibleEESpec,
	nodeSet client.Object,
) error {
	var err error
	var cmdLineArguments strings.Builder
	var inventoryVolume corev1.Volume
	var inventoryName string
	var inventoryMountPath string
	var sshKeyName string
	var sshKeyMountPath string
	var sshKeyMountSubPath string

	ansibleEEMounts := storage.VolMounts{}

	executionName, labels := GetAnsibleExecutionNameAndLabels(service, deployment.GetName(), nodeSet.GetName())
	ansibleEE, err := GetAnsibleExecution(ctx, helper, deployment, labels)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	// Don't patch and re-run jobs if the job status is already completed.
	if ansibleEE != nil && ansibleEE.Status.JobStatus == ansibleeev1.JobStatusSucceeded {
		return nil
	}

	if ansibleEE == nil {
		ansibleEE = &ansibleeev1.OpenStackAnsibleEE{
			ObjectMeta: metav1.ObjectMeta{
				Name:      executionName,
				Namespace: deployment.GetNamespace(),
				Labels:    labels,
			},
		}
	}

	_, err = controllerutil.CreateOrPatch(ctx, helper.GetClient(), ansibleEE, func() error {
		ansibleEE.Spec.NetworkAttachments = aeeSpec.NetworkAttachments
		if aeeSpec.DNSConfig != nil {
			ansibleEE.Spec.DNSConfig = aeeSpec.DNSConfig
		}
		if len(aeeSpec.OpenStackAnsibleEERunnerImage) > 0 {
			ansibleEE.Spec.Image = aeeSpec.OpenStackAnsibleEERunnerImage
		}
		if len(aeeSpec.ExtraVars) > 0 {
			ansibleEE.Spec.ExtraVars = aeeSpec.ExtraVars
		}
		if len(aeeSpec.AnsibleTags) > 0 {
			fmt.Fprintf(&cmdLineArguments, "--tags %s ", aeeSpec.AnsibleTags)
		}
		if len(aeeSpec.AnsibleLimit) > 0 {
			fmt.Fprintf(&cmdLineArguments, "--limit %s ", aeeSpec.AnsibleLimit)
		}
		if len(aeeSpec.AnsibleSkipTags) > 0 {
			fmt.Fprintf(&cmdLineArguments, "--skip-tags %s ", aeeSpec.AnsibleSkipTags)
		}
		if len(aeeSpec.ServiceAccountName) > 0 {
			ansibleEE.Spec.ServiceAccountName = aeeSpec.ServiceAccountName
		}
		if cmdLineArguments.Len() > 0 {
			ansibleEE.Spec.CmdLine = strings.TrimSpace(cmdLineArguments.String())
		}

		if len(service.Spec.PlaybookContents) > 0 {
			ansibleEE.Spec.PlaybookContents = service.Spec.PlaybookContents
		}
		if len(service.Spec.Playbook) > 0 {
			ansibleEE.Spec.Playbook = service.Spec.Playbook
		}
		ansibleEE.Spec.BackoffLimit = deployment.Spec.BackoffLimit

		// If we have a service that ought to be deployed everywhere
		// substitute the existing play target with 'all'
		// Check if we have ExtraVars before accessing it
		if ansibleEE.Spec.ExtraVars == nil {
			ansibleEE.Spec.ExtraVars = make(map[string]json.RawMessage)
		}
		if service.Spec.DeployOnAllNodeSets {
			ansibleEE.Spec.ExtraVars["edpm_override_hosts"] = json.RawMessage([]byte("\"all\""))
			util.LogForObject(helper, fmt.Sprintf("for service %s, substituting existing ansible play host with 'all'.", service.Name), ansibleEE)
		} else {
			ansibleEE.Spec.ExtraVars["edpm_override_hosts"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", nodeSet.GetName())))
			util.LogForObject(helper,
				fmt.Sprintf("for service %s, substituting existing ansible play host with '%s'.", service.Name, nodeSet.GetName()), ansibleEE)
		}
		if service.Spec.EDPMServiceType != "" {
			ansibleEE.Spec.ExtraVars["edpm_service_type"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", service.Spec.EDPMServiceType)))
		} else {
			ansibleEE.Spec.ExtraVars["edpm_service_type"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", service.Name)))
		}

		if len(deployment.Spec.ServicesOverride) > 0 {
			ansibleEE.Spec.ExtraVars["edpm_services_override"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", deployment.Spec.ServicesOverride)))
		}

		// Sort keys of the ssh secret map
		sshKeys := make([]string, 0)
		for k := range sshKeySecrets {
			sshKeys = append(sshKeys, k)
		}
		sort.Strings(sshKeys)

		for _, sshKeyNodeName := range sshKeys {
			sshKeySecret := sshKeySecrets[sshKeyNodeName]
			if service.Spec.DeployOnAllNodeSets {
				sshKeyName = fmt.Sprintf("ssh-key-%s", sshKeyNodeName)
				sshKeyMountSubPath = fmt.Sprintf("ssh_key_%s", sshKeyNodeName)
				sshKeyMountPath = fmt.Sprintf("/runner/env/ssh_key/%s", sshKeyMountSubPath)
			} else {
				if sshKeyNodeName != nodeSet.GetName() {
					continue
				}
				sshKeyName = "ssh-key"
				sshKeyMountSubPath = "ssh_key"
				sshKeyMountPath = "/runner/env/ssh_key"
			}
			sshKeyVolume := corev1.Volume{
				Name: sshKeyName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: sshKeySecret,
						Items: []corev1.KeyToPath{
							{
								Key:  "ssh-privatekey",
								Path: sshKeyMountSubPath,
							},
						},
					},
				},
			}
			sshKeyMount := corev1.VolumeMount{
				Name:      sshKeyName,
				MountPath: sshKeyMountPath,
				SubPath:   sshKeyMountSubPath,
			}
			// Mount ssh secrets
			ansibleEEMounts.Mounts = append(ansibleEEMounts.Mounts, sshKeyMount)
			ansibleEEMounts.Volumes = append(ansibleEEMounts.Volumes, sshKeyVolume)
		}

		// order the inventory keys otherwise it could lead to changing order and mount order changing
		invKeys := make([]string, 0)
		for k := range inventorySecrets {
			invKeys = append(invKeys, k)
		}
		sort.Strings(invKeys)

		// Mounting inventory and secrets
		for inventoryIndex, nodeName := range invKeys {
			if service.Spec.DeployOnAllNodeSets {
				inventoryName = fmt.Sprintf("inventory-%d", inventoryIndex)
				inventoryMountPath = fmt.Sprintf("/runner/inventory/%s", inventoryName)
			} else {
				if nodeName != nodeSet.GetName() {
					continue
				}
				inventoryName = "inventory"
				inventoryMountPath = "/runner/inventory/hosts"
			}

			inventoryVolume = corev1.Volume{
				Name: inventoryName,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: inventorySecrets[nodeName],
						Items: []corev1.KeyToPath{
							{
								Key:  "inventory",
								Path: inventoryName,
							},
						},
					},
				},
			}
			inventoryMount := corev1.VolumeMount{
				Name:      inventoryName,
				MountPath: inventoryMountPath,
				SubPath:   inventoryName,
			}
			// Inventory mount
			ansibleEEMounts.Mounts = append(ansibleEEMounts.Mounts, inventoryMount)
			ansibleEEMounts.Volumes = append(ansibleEEMounts.Volumes, inventoryVolume)
		}

		ansibleEE.Spec.ExtraMounts = append(aeeSpec.ExtraMounts, []storage.VolMounts{ansibleEEMounts}...)
		ansibleEE.Spec.Env = aeeSpec.Env

		err := controllerutil.SetControllerReference(deployment, ansibleEE, helper.GetScheme())
		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		util.LogErrorForObject(helper, err, fmt.Sprintf("Unable to create AnsibleEE %s", ansibleEE.Name), ansibleEE)
		return err
	}

	return nil
}

// GetAnsibleExecution gets and returns an OpenStackAnsibleEE with the given
// labels where
// "openstackdataplaneservice":    <serviceName>,
// "openstackdataplanedeployment": <deploymentName>,
// "openstackdataplanenodeset":    <nodeSetName>,
// If none or more than one is found, return nil and error
func GetAnsibleExecution(ctx context.Context,
	helper *helper.Helper, obj client.Object, labelSelector map[string]string) (*ansibleeev1.OpenStackAnsibleEE, error) {
	var err error
	ansibleEEs := &ansibleeev1.OpenStackAnsibleEEList{}

	listOpts := []client.ListOption{
		client.InNamespace(obj.GetNamespace()),
	}
	if len(labelSelector) > 0 {
		labels := client.MatchingLabels(labelSelector)
		listOpts = append(listOpts, labels)
	}
	err = helper.GetClient().List(ctx, ansibleEEs, listOpts...)
	if err != nil {
		return nil, err
	}

	var ansibleEE *ansibleeev1.OpenStackAnsibleEE
	if len(ansibleEEs.Items) == 0 {
		return nil, k8serrors.NewNotFound(appsv1.Resource("OpenStackAnsibleEE"), fmt.Sprintf("with label %s", labelSelector))
	} else if len(ansibleEEs.Items) == 1 {
		ansibleEE = &ansibleEEs.Items[0]
	} else {
		return nil, fmt.Errorf("multiple OpenStackAnsibleEE's found with label %s", labelSelector)
	}

	return ansibleEE, nil
}

// getAnsibleExecutionNamePrefix compute the name of the AnsibleEE
func getAnsibleExecutionNamePrefix(serviceName string) string {
	var executionNamePrefix string
	AnsibleExecutionServiceNameLen := apimachineryvalidation.DNS1123LabelMaxLength - 10
	if len(serviceName) > AnsibleExecutionServiceNameLen {
		executionNamePrefix = serviceName[:AnsibleExecutionServiceNameLen]
	} else {
		executionNamePrefix = serviceName
	}
	return executionNamePrefix
}

// GetAnsibleExecutionNameAndLabels Name and Labels of AnsibleEE
func GetAnsibleExecutionNameAndLabels(service *dataplanev1.OpenStackDataPlaneService,
	deploymentName string,
	nodeSetName string) (string, map[string]string) {
	executionName := fmt.Sprintf("%s-%s", getAnsibleExecutionNamePrefix(service.Name), deploymentName)
	if !service.Spec.DeployOnAllNodeSets {
		executionName = fmt.Sprintf("%s-%s", executionName, nodeSetName)
	}

	if len(executionName) > apimachineryvalidation.DNS1123LabelMaxLength {
		executionName = strings.TrimRight(executionName[:apimachineryvalidation.DNS1123LabelMaxLength], "-.")
	}

	labels := map[string]string{
		"openstackdataplaneservice":    service.Name,
		"openstackdataplanedeployment": deploymentName,
		"openstackdataplanenodeset":    nodeSetName,
	}
	return executionName, labels
}
