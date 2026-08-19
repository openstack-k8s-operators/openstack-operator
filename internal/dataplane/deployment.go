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
	"slices"
	"sort"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	apimachineryvalidation "k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"

	"github.com/iancoleman/strcase"
	condition "github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"github.com/openstack-k8s-operators/lib-common/modules/storage"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane/util"
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
	ServiceCache                *ServiceCache
	Version                     *openstackv1.OpenStackVersion
}

// Deploy function encapsulating primary deployment handling.
// It accepts a leveled execution plan: each level is a group of services
// whose dependencies are all satisfied by earlier levels.
func (d *Deployer) Deploy(serviceLevels [][]string) (*ctrl.Result, error) {
	log := d.Helper.GetLogger()
	if d.ServiceCache == nil {
		d.ServiceCache = NewServiceCache()
	}

	// Flatten levels into a single list for cert-mount lookups.
	var allServices []string
	for _, level := range serviceLevels {
		allServices = append(allServices, level...)
	}

	for levelIdx, level := range serviceLevels {
		log.Info("Deploying service level", "level", levelIdx, "services", level)
		levelServices := make(map[string]dataplanev1.OpenStackDataPlaneService, len(level))

		// Dispatch all services in this level
		for _, service := range level {
			deployName := service
			readyCondition := condition.Type(fmt.Sprintf("Service%sDeploymentReady", strcase.ToCamel(service)))
			readyWaitingMessage := fmt.Sprintf(dataplanev1.NodeSetServiceDeploymentReadyWaitingMessage, deployName)
			readyMessage := fmt.Sprintf(dataplanev1.NodeSetServiceDeploymentReadyMessage, deployName)
			readyErrorMessage := fmt.Sprintf(dataplanev1.NodeSetServiceDeploymentErrorMessage, deployName) + " error %s"

			log.Info("Deploying service", "service", service, "level", levelIdx)
			foundService, err := d.ServiceCache.Get(d.Ctx, d.Helper, service)
			if err != nil {
				d.setServiceError(readyCondition, readyErrorMessage, err)
				return &ctrl.Result{}, err
			}
			levelServices[service] = foundService

			serviceAeeSpec := d.AeeSpec.DeepCopy()
			containerImages := dataplaneutil.GetContainerImages(d.Version)
			if containerImages.AnsibleeeImage != nil {
				serviceAeeSpec.OpenStackAnsibleEERunnerImage = *containerImages.AnsibleeeImage
			}
			if len(foundService.Spec.OpenStackAnsibleEERunnerImage) > 0 {
				serviceAeeSpec.OpenStackAnsibleEERunnerImage = foundService.Spec.OpenStackAnsibleEERunnerImage
			}

			err = d.addServiceExtraMounts(serviceAeeSpec, foundService)
			if err != nil {
				d.setServiceError(readyCondition, readyErrorMessage, err)
				return &ctrl.Result{}, err
			}

			// Add certMounts
			if foundService.Spec.AddCertMounts {
				err = d.addCertMounts(serviceAeeSpec, allServices)
				if err != nil {
					d.setServiceError(readyCondition, readyErrorMessage, err)
					return &ctrl.Result{}, err
				}
			} else if len(foundService.Spec.CACerts) > 0 {
				err = d.addCACertMount(serviceAeeSpec, foundService)
				if err != nil {
					d.setServiceError(readyCondition, readyErrorMessage, err)
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
				serviceAeeSpec,
			)

			if err != nil {
				log.Info("Condition error in service level", "condition", readyCondition, "level", levelIdx)
				return &ctrl.Result{}, err
			}
		}

		// After dispatching all services in this level, check that every
		// service in the level is ready before advancing to the next level.
		for _, service := range level {
			readyCondition := condition.Type(fmt.Sprintf("Service%sDeploymentReady", strcase.ToCamel(service)))
			nsConditions := d.Status.NodeSetConditions[d.NodeSet.Name]
			if !nsConditions.IsTrue(readyCondition) {
				log.Info("Condition not ready in service level, waiting", "condition", readyCondition, "level", levelIdx)
				return &ctrl.Result{}, nil
			}
			log.Info("Condition ready", "condition", readyCondition)

			foundService := levelServices[service]
			// (TODO) Only considers the container image values from the Version
			// for the time being.
			if d.Version != nil {
				vContainerImages := reflect.ValueOf(d.Version.Status.ContainerImages)
				for _, cif := range foundService.Spec.ContainerImageFields {
					d.Deployment.Status.ContainerImages[cif] = reflect.Indirect(vContainerImages.FieldByName(cif)).String()
				}
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
	aeeSpec *dataplanev1.AnsibleEESpec,
) error {
	var err error
	log := d.Helper.GetLogger()

	nsConditions := d.Status.NodeSetConditions[d.NodeSet.Name]
	if nsConditions.IsUnknown(readyCondition) {
		log.Info("Condition unknown, starting deploy", "condition", readyCondition, "service", deployName)
		err = d.DeployService(
			foundService, aeeSpec)
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
				log.Info("AnsibleEE job not yet found", "condition", readyCondition)
				return nil
			}
			log.Error(err, "Error getting ansibleJob job", "service", deployName)
			nsConditions.Set(condition.FalseCondition(
				readyCondition,
				condition.ErrorReason,
				condition.SeverityError,
				readyErrorMessage,
				err.Error()))
		}
		if ansibleJob.Status.Succeeded > 0 {
			d.storeExecutionSummary(ansibleJob)
			log.Info("Condition ready", "condition", readyCondition)
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
			d.storeExecutionSummary(ansibleJob)
			log.Info("Condition error", "condition", readyCondition)
			err = fmt.Errorf("%s", errorMsg)
			nsConditions.Set(condition.FalseCondition(
				readyCondition,
				condition.Reason(ansibleCondition.Reason),
				condition.SeverityError,
				readyErrorMessage,
				err.Error()))
		} else {
			log.Info("AnsibleEE job not yet completed",
				"execution", ansibleJob.Name,
				"activePods", ansibleJob.Status.Active,
				"failedPods", ansibleJob.Status.Failed)
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

// storeExecutionSummary fetches and stores the ansible execution summary for a
// completed or failed Job into the deployment status.
func (d *Deployer) storeExecutionSummary(ansibleJob *batchv1.Job) {
	log := d.Helper.GetLogger()
	summary, err := dataplaneutil.GetAnsibleExecutionSummary(d.Ctx, d.Helper, ansibleJob)
	if err != nil {
		log.Error(err, "Unable to get ansible execution summary", "execution", ansibleJob.Name)
		return
	}
	if summary == nil {
		return
	}
	if d.Status.AnsibleExecutionSummaries == nil {
		d.Status.AnsibleExecutionSummaries = make(map[string]dataplanev1.AnsibleExecutionSummary)
	}
	d.Status.AnsibleExecutionSummaries[ansibleJob.Name] = *summary
}

func (d *Deployer) setServiceError(
	readyCondition condition.Type,
	readyErrorMessage string,
	err error,
) {
	nsConditions := d.Status.NodeSetConditions[d.NodeSet.Name]
	nsConditions.Set(condition.FalseCondition(
		readyCondition,
		condition.ErrorReason,
		condition.SeverityError,
		readyErrorMessage,
		err.Error()))
	d.Status.NodeSetConditions[d.NodeSet.Name] = nsConditions
}

// addCertMounts adds the cert mounts to the aeeSpec for the install-certs service
func (d *Deployer) addCertMounts(
	aeeSpec *dataplanev1.AnsibleEESpec,
	services []string,
) error {
	log := d.Helper.GetLogger()
	client := d.Helper.GetClient()
	for _, svc := range services {
		service, err := d.ServiceCache.Get(d.Ctx, d.Helper, svc)
		if err != nil {
			return err
		}

		if service.Spec.CertsFrom != "" && service.Spec.TLSCerts == nil && service.Spec.CACerts == "" {
			if slices.Contains(services, service.Spec.CertsFrom) {
				continue
			}
			service, err = d.ServiceCache.Get(d.Ctx, d.Helper, service.Spec.CertsFrom)
			if err != nil {
				return err
			}
		}

		if service.Spec.EDPMServiceType != service.Name && service.Spec.TLSCerts == nil {
			if slices.Contains(services, service.Spec.EDPMServiceType) {
				continue
			}
			service, err = d.ServiceCache.Get(d.Ctx, d.Helper, service.Spec.EDPMServiceType)
			if err != nil {
				return err
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
					return err
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
						return err
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
				aeeSpec.ExtraMounts = append(aeeSpec.ExtraMounts, volMounts)
			}
		}

		// add mount for cacert bundle, even if TLS-E is not enabled
		if len(service.Spec.CACerts) > 0 {
			err = d.addCACertMount(aeeSpec, service)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (d *Deployer) addCACertMount(
	aeeSpec *dataplanev1.AnsibleEESpec,
	service dataplanev1.OpenStackDataPlaneService,
) error {
	log := d.Helper.GetLogger()
	client := d.Helper.GetClient()

	log.Info("Mounting CA cert bundle for service", "service", service)
	volMounts := storage.VolMounts{}
	cacertSecret := &corev1.Secret{}
	err := client.Get(d.Ctx, types.NamespacedName{Name: service.Spec.CACerts, Namespace: service.Namespace}, cacertSecret)
	if err != nil {
		return err
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
	aeeSpec.ExtraMounts = append(aeeSpec.ExtraMounts, volMounts)
	return nil
}

// addServiceExtraMounts adds the service configs as ExtraMounts to aeeSpec
func (d *Deployer) addServiceExtraMounts(
	aeeSpec *dataplanev1.AnsibleEESpec,
	service dataplanev1.OpenStackDataPlaneService,
) error {
	baseMountPath := path.Join(ConfigPaths, service.Spec.EDPMServiceType)

	var configMaps []*corev1.ConfigMap
	var secrets []*corev1.Secret

	for _, dataSource := range service.Spec.DataSources {
		_cm, _secret, err := dataplaneutil.GetDataSourceCmSecret(d.Ctx, d.Helper, service.Namespace, dataSource)
		if err != nil {
			return err
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

		aeeSpec.ExtraMounts = append(aeeSpec.ExtraMounts, volMounts)
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

		aeeSpec.ExtraMounts = append(aeeSpec.ExtraMounts, volMounts)
	}

	return nil
}
