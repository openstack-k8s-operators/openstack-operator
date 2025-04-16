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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	slices "golang.org/x/exp/slices"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/iancoleman/strcase"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/pkg/dataplane/util"
)

// Deployer defines a data structure with all of the relevant objects required for a full deployment.
type Deployer struct {
	Ctx                         context.Context
	Helper                      *helper.Helper
	NodeSet                     *dataplanev1.OpenStackDataPlaneNodeSet
	Deployment                  *dataplanev1.OpenStackDataPlaneDeployment
	Status                      *dataplanev1.OpenStackDataPlaneDeploymentStatus
	AeeSpec                     *dataplanev1.AnsibleEESpec
	InventorySecrets            map[string]string
	AnsibleSSHPrivateKeySecrets map[string]string
	Version                     *openstackv1.OpenStackVersion
}

// Deploy function encapsulating primary deloyment handling
func (d *Deployer) Deploy(services []string) (*ctrl.Result, error) {
	log := d.Helper.GetLogger()

	var readyCondition condition.Type
	var readyMessage string
	var readyWaitingMessage string
	var readyErrorMessage string
	var deployName string

	// Save a copy of the original ExtraMounts so it can be reset after each
	// service deployment
	aeeSpecMounts := make([]storage.VolMounts, len(d.AeeSpec.ExtraMounts))
	copy(aeeSpecMounts, d.AeeSpec.ExtraMounts)
	// Deploy the composable services
	for _, service := range services {
		deployName = service
		readyCondition = condition.Type(fmt.Sprintf("Service%sDeploymentReady", strcase.ToCamel(service)))
		readyWaitingMessage = fmt.Sprintf(dataplanev1.NodeSetServiceDeploymentReadyWaitingMessage, deployName)
		readyMessage = fmt.Sprintf(dataplanev1.NodeSetServiceDeploymentReadyMessage, deployName)
		readyErrorMessage = fmt.Sprintf(dataplanev1.NodeSetServiceDeploymentErrorMessage, deployName) + " error %s"

		nsConditions := d.Status.NodeSetConditions[d.NodeSet.Name]
		log.Info("Deploying service", "service", service)
		foundService, err := GetService(d.Ctx, d.Helper, service)
		if err != nil {
			nsConditions.Set(condition.FalseCondition(
				readyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				readyErrorMessage,
				err.Error()))
			d.Status.NodeSetConditions[d.NodeSet.Name] = nsConditions
			return &ctrl.Result{}, err
		}

		containerImages := dataplaneutil.GetContainerImages(d.Version)
		if containerImages.AnsibleeeImage != nil {
			d.AeeSpec.OpenStackAnsibleEERunnerImage = *containerImages.AnsibleeeImage
		}
		if len(foundService.Spec.OpenStackAnsibleEERunnerImage) > 0 {
			d.AeeSpec.OpenStackAnsibleEERunnerImage = foundService.Spec.OpenStackAnsibleEERunnerImage
		}

		// Reset ExtraMounts to its original value, and then add in service
		// specific mounts.
		d.AeeSpec.ExtraMounts = make([]storage.VolMounts, len(aeeSpecMounts))
		copy(d.AeeSpec.ExtraMounts, aeeSpecMounts)
		d.AeeSpec, err = d.addServiceExtraMounts(foundService)
		if err != nil {
			nsConditions.Set(condition.FalseCondition(
				readyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				readyErrorMessage,
				err.Error()))
			d.Status.NodeSetConditions[d.NodeSet.Name] = nsConditions
			return &ctrl.Result{}, err
		}

		// Add certMounts
		if foundService.Spec.AddCertMounts {
			d.AeeSpec, err = d.addCertMounts(services)
			if err != nil {
				nsConditions.Set(condition.FalseCondition(
					readyCondition,
					condition.ErrorReason,
					condition.SeverityError,
					readyErrorMessage,
					err.Error()))
				d.Status.NodeSetConditions[d.NodeSet.Name] = nsConditions
				return &ctrl.Result{}, err
			}
		} else if len(foundService.Spec.CACerts) > 0 {
			// In general, we use the install-certs service (which has AddCertMounts=true
			// to copy the cacerts over.  This may not be early enough for some services
			// though.  In particular, we want the bootstrap service to copy the cacerts
			// to the default location on the compute node.
			d.AeeSpec, err = d.addCACertMount(foundService)
			if err != nil {
				nsConditions.Set(condition.FalseCondition(
					readyCondition,
					condition.ErrorReason,
					condition.SeverityError,
					readyErrorMessage,
					err.Error()))
				d.Status.NodeSetConditions[d.NodeSet.Name] = nsConditions
				return &ctrl.Result{}, err
			}
		}

		err = d.ConditionalDeploy(
			readyCondition,
			readyMessage,
			readyWaitingMessage,
			readyErrorMessage,
			deployName,
			foundService,
		)

		nsConditions = d.Status.NodeSetConditions[d.NodeSet.Name]
		if err != nil || !nsConditions.IsTrue(readyCondition) {
			log.Info(fmt.Sprintf("Condition %s not ready", readyCondition))
			return &ctrl.Result{}, err
		}

		log.Info(fmt.Sprintf("Condition %s ready", readyCondition))

		// (TODO) Only considers the container image values from the Version
		// for the time being. Can be expanded later to look at the actual
		// values used from the inventory, etc.
		if d.Version != nil {
			vContainerImages := reflect.ValueOf(d.Version.Status.ContainerImages)
			for _, cif := range foundService.Spec.ContainerImageFields {
				d.Deployment.Status.ContainerImages[cif] = reflect.Indirect(vContainerImages.FieldByName(cif)).String()
			}
		}

	}

	return nil, nil
}

// ConditionalDeploy function encapsulating primary deloyment handling with
// conditions.
func (d *Deployer) ConditionalDeploy(
	readyCondition condition.Type,
	readyMessage string,
	readyWaitingMessage string,
	readyErrorMessage string,
	deployName string,
	foundService dataplanev1.OpenStackDataPlaneService,
) error {
	var err error
	log := d.Helper.GetLogger()

	nsConditions := d.Status.NodeSetConditions[d.NodeSet.Name]
	if nsConditions.IsUnknown(readyCondition) {
		log.Info(fmt.Sprintf("%s Unknown, starting %s", readyCondition, deployName))
		err = d.DeployService(
			foundService)
		if err != nil {
			util.LogErrorForObject(d.Helper, err, fmt.Sprintf("Unable to %s for %s", deployName, d.NodeSet.Name), d.NodeSet)
			return err
		}
		nsConditions.Set(condition.FalseCondition(
			readyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			"%s", readyWaitingMessage))

	}

	var ansibleCondition batchv1.JobCondition
	if nsConditions.IsFalse(readyCondition) {
		var ansibleJob *batchv1.Job
		_, labelSelector := dataplaneutil.GetAnsibleExecutionNameAndLabels(&foundService, d.Deployment.Name, d.NodeSet.Name)
		ansibleJob, err = dataplaneutil.GetAnsibleExecution(d.Ctx, d.Helper, d.Deployment, labelSelector)
		if err != nil {
			// Return nil if we don't have AnsibleEE available yet
			if k8s_errors.IsNotFound(err) {
				log.Info(fmt.Sprintf("%s AnsibleEE job is not yet found", readyCondition))
				return nil
			}
			log.Error(err, fmt.Sprintf("Error getting ansibleJob job for %s", deployName))
			nsConditions.Set(condition.FalseCondition(
				readyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				readyErrorMessage,
				err.Error()))
		}
		if ansibleJob.Status.Succeeded > 0 {
			log.Info(fmt.Sprintf("Condition %s ready", readyCondition))
			nsConditions.Set(condition.TrueCondition(
				readyCondition,
				"%s", readyMessage))
		} else if ansibleJob.Status.Failed > *ansibleJob.Spec.BackoffLimit {
			errorMsg := fmt.Sprintf("execution.name %s execution.namespace %s failed pods: %d", ansibleJob.Name, ansibleJob.Namespace, ansibleJob.Status.Failed)
			for _, condition := range ansibleJob.Status.Conditions {
				if condition.Type == batchv1.JobFailed {
					ansibleCondition = condition
				}
			}
			if ansibleCondition.Reason == condition.JobReasonBackoffLimitExceeded {
				errorMsg = fmt.Sprintf("backoff limit reached for execution.name %s execution.namespace %s execution.condition.message: %s", ansibleJob.Name, ansibleJob.Namespace, ansibleCondition.Message)
			}
			log.Info(fmt.Sprintf("Condition %s error", readyCondition))
			err = fmt.Errorf("%s", errorMsg)
			nsConditions.Set(condition.FalseCondition(
				readyCondition,
				condition.Reason(ansibleCondition.Reason),
				condition.SeverityError,
				readyErrorMessage,
				err.Error()))
		} else {
			log.Info(fmt.Sprintf("AnsibleEE job is not yet completed: Execution: %s, Active pods: %d, Failed pods: %d", ansibleJob.Name, ansibleJob.Status.Active, ansibleJob.Status.Failed))
			nsConditions.Set(condition.FalseCondition(
				readyCondition,
				condition.RequestedReason,
				condition.SeverityInfo,
				"%s", readyWaitingMessage))
		}
	}
	d.Status.NodeSetConditions[d.NodeSet.Name] = nsConditions

	return err
}

// addCertMounts adds the cert mounts to the aeeSpec for the install-certs service
func (d *Deployer) addCertMounts(
	services []string,
) (*dataplanev1.AnsibleEESpec, error) {
	log := d.Helper.GetLogger()
	client := d.Helper.GetClient()
	for _, svc := range services {
		service, err := GetService(d.Ctx, d.Helper, svc)
		if err != nil {
			return nil, err
		}

		if service.Spec.CertsFrom != "" && service.Spec.TLSCerts == nil && service.Spec.CACerts == "" {
			if slices.Contains(services, service.Spec.CertsFrom) {
				continue
			}
			service, err = GetService(d.Ctx, d.Helper, service.Spec.CertsFrom)
			if err != nil {
				return nil, err
			}
		}

		if service.Spec.EDPMServiceType != service.Name && service.Spec.TLSCerts == nil {
			if slices.Contains(services, service.Spec.EDPMServiceType) {
				continue
			}
			service, err = GetService(d.Ctx, d.Helper, service.Spec.EDPMServiceType)
			if err != nil {
				return nil, err
			}
		}

		if service.Spec.TLSCerts != nil && d.NodeSet.Spec.TLSEnabled {
			// sort cert list to ensure mount list is consistent
			certKeyList := make([]string, 0, len(service.Spec.TLSCerts))
			for ckey := range service.Spec.TLSCerts {
				certKeyList = append(certKeyList, ckey)
			}
			sort.Strings(certKeyList)

			for _, certKey := range certKeyList {
				log.Info("Mounting TLS cert for service", "service", svc)
				volMounts := storage.VolMounts{}

				// add mount for certs and keys
				secretName := GetServiceCertsSecretName(d.NodeSet, service.Name, certKey, 0) // Need to get the number of secrets
				certSecret := &corev1.Secret{}
				err := client.Get(d.Ctx, types.NamespacedName{Name: secretName, Namespace: service.Namespace}, certSecret)
				if err != nil {
					return d.AeeSpec, err
				}
				numberOfSecrets, _ := strconv.Atoi(certSecret.Labels["numberOfSecrets"])
				projectedVolumeSource := corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{},
				}
				for i := 0; i < numberOfSecrets; i++ {
					secretName := GetServiceCertsSecretName(d.NodeSet, service.Name, certKey, i)
					certSecret := &corev1.Secret{}
					err := client.Get(d.Ctx, types.NamespacedName{Name: secretName, Namespace: service.Namespace}, certSecret)
					if err != nil {
						return d.AeeSpec, err
					}
					volumeProjection := corev1.VolumeProjection{
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: secretName,
							},
						},
					}
					projectedVolumeSource.Sources = append(projectedVolumeSource.Sources, volumeProjection)
				}
				volumeName := GetServiceCertsSecretName(d.NodeSet, service.Name, certKey, 0)
				if len(volumeName) > apimachineryvalidation.DNS1123LabelMaxLength {
					hash := sha256.Sum224([]byte(volumeName))
					volumeName = "cert" + hex.EncodeToString(hash[:])
				}
				certVolume := storage.Volume{
					Name: volumeName,
					VolumeSource: storage.VolumeSource{
						Projected: &projectedVolumeSource,
					},
				}

				certMountDir := service.Spec.TLSCerts[certKey].EDPMRoleServiceName
				if certMountDir == "" {
					certMountDir = service.Spec.EDPMServiceType
				}

				certVolumeMount := corev1.VolumeMount{
					Name:      volumeName,
					MountPath: path.Join(CertPaths, certMountDir, certKey),
				}
				volMounts.Volumes = append(volMounts.Volumes, certVolume)
				volMounts.Mounts = append(volMounts.Mounts, certVolumeMount)
				d.AeeSpec.ExtraMounts = append(d.AeeSpec.ExtraMounts, volMounts)
			}
		}

		// add mount for cacert bundle, even if TLS-E is not enabled
		if len(service.Spec.CACerts) > 0 {
			d.AeeSpec, err = d.addCACertMount(service)
			if err != nil {
				return d.AeeSpec, err
			}
		}
	}

	return d.AeeSpec, nil
}

func (d *Deployer) addCACertMount(
	service dataplanev1.OpenStackDataPlaneService,
) (*dataplanev1.AnsibleEESpec, error) {
	log := d.Helper.GetLogger()
	client := d.Helper.GetClient()

	log.Info("Mounting CA cert bundle for service", "service", service)
	volMounts := storage.VolMounts{}
	cacertSecret := &corev1.Secret{}
	err := client.Get(d.Ctx, types.NamespacedName{Name: service.Spec.CACerts, Namespace: service.Namespace}, cacertSecret)
	if err != nil {
		return d.AeeSpec, err
	}
	volumeName := fmt.Sprintf("%s-%s", service.Name, service.Spec.CACerts)
	if len(volumeName) > apimachineryvalidation.DNS1123LabelMaxLength {
		hash := sha256.Sum224([]byte(volumeName))
		volumeName = "cacert" + hex.EncodeToString(hash[:])
	}
	cacertVolume := storage.Volume{
		Name: volumeName,
		VolumeSource: storage.VolumeSource{
			Secret: &corev1.SecretVolumeSource{
				SecretName: service.Spec.CACerts,
			},
		},
	}

	cacertVolumeMount := corev1.VolumeMount{
		Name:      volumeName,
		MountPath: path.Join(CACertPaths, service.Spec.EDPMServiceType),
	}

	volMounts.Volumes = append(volMounts.Volumes, cacertVolume)
	volMounts.Mounts = append(volMounts.Mounts, cacertVolumeMount)
	d.AeeSpec.ExtraMounts = append(d.AeeSpec.ExtraMounts, volMounts)
	return d.AeeSpec, nil
}

// addServiceExtraMounts adds the service configs as ExtraMounts to aeeSpec
func (d *Deployer) addServiceExtraMounts(
	service dataplanev1.OpenStackDataPlaneService,
) (*dataplanev1.AnsibleEESpec, error) {
	baseMountPath := path.Join(ConfigPaths, service.Spec.EDPMServiceType)

	var configMaps []*corev1.ConfigMap
	var secrets []*corev1.Secret

	for _, dataSource := range service.Spec.DataSources {
		_cm, _secret, err := dataplaneutil.GetDataSourceCmSecret(d.Ctx, d.Helper, service.Namespace, dataSource)
		if err != nil {
			return nil, err
		}

		if _cm != nil {
			configMaps = append(configMaps, _cm)
		}
		if _secret != nil {
			secrets = append(secrets, _secret)
		}
	}

	for _, cm := range configMaps {

		volMounts := storage.VolMounts{}

		keys := []string{}
		for key := range cm.Data {
			keys = append(keys, key)
		}
		for key := range cm.BinaryData {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for idx, key := range keys {
			volumeName := fmt.Sprintf("%s-%s", cm.Name, strconv.Itoa(idx))
			if len(volumeName) > apimachineryvalidation.DNS1123LabelMaxLength {
				hash := sha256.Sum224([]byte(volumeName))
				volumeName = "cm" + hex.EncodeToString(hash[:])
			}
			volume := storage.Volume{
				Name: volumeName,
				VolumeSource: storage.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: cm.Name,
						},
						Items: []corev1.KeyToPath{
							{
								Key:  key,
								Path: key,
							},
						},
					},
				},
			}

			volumeMount := corev1.VolumeMount{
				Name:      volumeName,
				MountPath: path.Join(baseMountPath, key),
				SubPath:   key,
			}

			volMounts.Volumes = append(volMounts.Volumes, volume)
			volMounts.Mounts = append(volMounts.Mounts, volumeMount)

		}

		d.AeeSpec.ExtraMounts = append(d.AeeSpec.ExtraMounts, volMounts)
	}

	for _, sec := range secrets {

		volMounts := storage.VolMounts{}
		keys := []string{}
		for key := range sec.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for idx, key := range keys {
			volumeName := fmt.Sprintf("%s-%s", sec.Name, strconv.Itoa(idx))
			if len(volumeName) > apimachineryvalidation.DNS1123LabelMaxLength {
				hash := sha256.Sum224([]byte(volumeName))
				volumeName = "sec" + hex.EncodeToString(hash[:])
			}
			volume := storage.Volume{
				Name: volumeName,
				VolumeSource: storage.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: sec.Name,
						Items: []corev1.KeyToPath{
							{
								Key:  key,
								Path: key,
							},
						},
					},
				},
			}

			volumeMount := corev1.VolumeMount{
				Name:      volumeName,
				MountPath: path.Join(baseMountPath, key),
				SubPath:   key,
			}

			volMounts.Volumes = append(volMounts.Volumes, volume)
			volMounts.Mounts = append(volMounts.Mounts, volumeMount)

		}

		d.AeeSpec.ExtraMounts = append(d.AeeSpec.ExtraMounts, volMounts)
	}

	return d.AeeSpec, nil
}
