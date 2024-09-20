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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	job "github.com/openstack-k8s-operators/lib-common/modules/common/job"
	nad "github.com/openstack-k8s-operators/lib-common/modules/common/networkattachment"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
)

// AnsibleExecution creates a batchv1 Job to execute Ansible
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

	executionName, labels := GetAnsibleExecutionNameAndLabels(service, deployment.GetName(), nodeSet.GetName())

	existingAnsibleEE, err := GetAnsibleExecution(ctx, helper, deployment, labels)
	if err != nil && !k8serrors.IsNotFound(err) {
		return err
	}

	// Don't patch and re-run jobs if the job status is already completed.
	if existingAnsibleEE != nil && existingAnsibleEE.Status.Succeeded > 0 {
		return nil
	}

	ansibleEE := EEJob{
		Name:             executionName,
		Namespace:        deployment.GetNamespace(),
		Labels:           labels,
		EnvConfigMapName: "openstack-aee-default-env",
	}

	ansibleEE.NetworkAttachments = aeeSpec.NetworkAttachments
	ansibleEE.Annotations, err = nad.CreateNetworksAnnotation(deployment.Namespace, ansibleEE.NetworkAttachments)
	if err != nil {
		return fmt.Errorf("failed to create NetworkAttachment annotation. Error: %w", err)
	}

	ansibleEE.BuildAeeJobSpec(aeeSpec, deployment, service, nodeSet)

	ansibleEEMounts := storage.VolMounts{}
	SetAeeSSHMounts(nodeSet, service, sshKeySecrets, &ansibleEEMounts)
	SetAeeInvMounts(nodeSet, service, inventorySecrets, &ansibleEEMounts)

	ansibleEE.ExtraMounts = append(aeeSpec.ExtraMounts, []storage.VolMounts{ansibleEEMounts}...)
	ansibleEE.Env = aeeSpec.Env

	currentJobHash := deployment.Status.AnsibleEEHashes[ansibleEE.Name]
	jobDef, err := ansibleEE.JobForOpenStackAnsibleEE(helper)
	if err != nil {
		return err
	}

	ansibleeeJob := job.NewJob(
		jobDef,
		ansibleEE.Name,
		ansibleEE.PreserveJobs,
		time.Duration(5)*time.Second,
		currentJobHash,
	)

	_, err = ansibleeeJob.DoJob(
		ctx,
		helper,
	)

	if err != nil {
		return err
	}

	if ansibleeeJob.HasChanged() {
		deployment.Status.AnsibleEEHashes[ansibleEE.Name] = ansibleeeJob.GetHash()
	}

	return nil
}

// GetAnsibleExecution gets and returns a batchv1 Job with the given
// labels where
// "openstackdataplaneservice":    <serviceName>,
// "openstackdataplanedeployment": <deploymentName>,
// "openstackdataplanenodeset":    <nodeSetName>,
// If none or more than one is found, return nil and error
func GetAnsibleExecution(ctx context.Context,
	helper *helper.Helper, obj client.Object, labelSelector map[string]string,
) (*batchv1.Job, error) {
	var err error
	ansibleEEs := &batchv1.JobList{}

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

	var ansibleEE *batchv1.Job
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
	nodeSetName string,
) (string, map[string]string) {
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

func (a *EEJob) BuildAeeJobSpec(
	aeeSpec *dataplanev1.AnsibleEESpec,
	deployment *dataplanev1.OpenStackDataPlaneDeployment,
	service *dataplanev1.OpenStackDataPlaneService,
	nodeSet client.Object,
) {
	if aeeSpec.DNSConfig != nil {
		a.DNSConfig = aeeSpec.DNSConfig
	}
	if len(aeeSpec.ServiceAccountName) > 0 {
		a.ServiceAccountName = aeeSpec.ServiceAccountName
	}

	if len(service.Spec.PlaybookContents) > 0 {
		a.PlaybookContents = service.Spec.PlaybookContents
	}
	if len(service.Spec.Playbook) > 0 {
		a.Playbook = service.Spec.Playbook
	}

	a.BackoffLimit = deployment.Spec.BackoffLimit
	a.PreserveJobs = deployment.Spec.PreserveJobs
	a.FormatAEECmdLineArguments(aeeSpec)
	a.FormatAEEExtraVars(aeeSpec, service, deployment, nodeSet)
	a.DetermineAeeImage(aeeSpec)
}

func (a *EEJob) FormatAEECmdLineArguments(aeeSpec *dataplanev1.AnsibleEESpec) {
	var cmdLineArguments strings.Builder

	if len(aeeSpec.AnsibleTags) > 0 {
		fmt.Fprintf(&cmdLineArguments, "--tags %s ", aeeSpec.AnsibleTags)
	}
	if len(aeeSpec.AnsibleLimit) > 0 {
		fmt.Fprintf(&cmdLineArguments, "--limit %s ", aeeSpec.AnsibleLimit)
	}
	if len(aeeSpec.AnsibleSkipTags) > 0 {
		fmt.Fprintf(&cmdLineArguments, "--skip-tags %s ", aeeSpec.AnsibleSkipTags)
	}

	if cmdLineArguments.Len() > 0 {
		a.CmdLine = strings.TrimSpace(cmdLineArguments.String())
	}
}

func (a *EEJob) FormatAEEExtraVars(
	aeeSpec *dataplanev1.AnsibleEESpec,
	service *dataplanev1.OpenStackDataPlaneService,
	deployment *dataplanev1.OpenStackDataPlaneDeployment,
	nodeSet client.Object,
) {
	if len(aeeSpec.ExtraVars) > 0 {
		a.ExtraVars = aeeSpec.ExtraVars
	}

	// If we have a service that ought to be deployed everywhere
	// substitute the existing play target with 'all'
	// Check if we have ExtraVars before accessing it
	if a.ExtraVars == nil {
		a.ExtraVars = make(map[string]json.RawMessage)
	}
	if service.Spec.DeployOnAllNodeSets {
		a.ExtraVars["edpm_override_hosts"] = json.RawMessage([]byte("\"all\""))
	} else {
		a.ExtraVars["edpm_override_hosts"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", nodeSet.GetName())))
	}
	if service.Spec.EDPMServiceType != "" {
		a.ExtraVars["edpm_service_type"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", service.Spec.EDPMServiceType)))
	} else {
		a.ExtraVars["edpm_service_type"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", service.Name)))
	}

	if len(deployment.Spec.ServicesOverride) > 0 {
		a.ExtraVars["edpm_services_override"] = json.RawMessage([]byte(fmt.Sprintf("\"%s\"", deployment.Spec.ServicesOverride)))
	}
}

func (a *EEJob) DetermineAeeImage(aeeSpec *dataplanev1.AnsibleEESpec) {
	if len(aeeSpec.OpenStackAnsibleEERunnerImage) > 0 {
		a.Image = aeeSpec.OpenStackAnsibleEERunnerImage
	} else {
		a.Image = *dataplanev1.ContainerImageDefaults.AnsibleeeImage
	}
}

// SetAeeSshMounts - Using the information provided from the NodeSet, Service and AnsibleEE Spec. We determine the required
// ssh key mounts that are required for the Ansible Execution Job. This function takes a pointer to the storage.VolMounts
// struct and updates them as per the required ssh key related mounts.
func SetAeeSSHMounts(
	nodeSet client.Object,
	service *dataplanev1.OpenStackDataPlaneService,
	sshKeySecrets map[string]string,
	ansibleEEMounts *storage.VolMounts,
) {
	var (
		sshKeyName         string
		sshKeyMountPath    string
		sshKeyMountSubPath string
	)

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

		CreateVolume(ansibleEEMounts, sshKeyName, sshKeyMountSubPath, sshKeySecret, "ssh-privatekey")
		CreateVolumeMount(ansibleEEMounts, sshKeyName, sshKeyMountPath, sshKeyMountSubPath)
	}
}

func SetAeeInvMounts(
	nodeSet client.Object,
	service *dataplanev1.OpenStackDataPlaneService,
	inventorySecrets map[string]string,
	ansibleEEMounts *storage.VolMounts,
) {
	var (
		inventoryName      string
		inventoryMountPath string
	)

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

		CreateVolume(ansibleEEMounts, inventoryName, inventoryName, inventorySecrets[nodeName], "inventory")
		CreateVolumeMount(ansibleEEMounts, inventoryName, inventoryMountPath, inventoryName)
	}
}

func CreateVolume(ansibleEEMounts *storage.VolMounts, volumeName string, volumeMountPath string, secretName string, keyToPathKey string) {
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: secretName,
				Items: []corev1.KeyToPath{
					{
						Key:  keyToPathKey,
						Path: volumeMountPath,
					},
				},
			},
		},
	}

	ansibleEEMounts.Volumes = append(ansibleEEMounts.Volumes, volume)
}

func CreateVolumeMount(ansibleEEMounts *storage.VolMounts, volumeMountName string, volumeMountPath string, volumeMountSubPath string) {
	volumeMount := corev1.VolumeMount{
		Name:      volumeMountName,
		MountPath: volumeMountPath,
		SubPath:   volumeMountSubPath,
	}

	ansibleEEMounts.Mounts = append(ansibleEEMounts.Mounts, volumeMount)
}
