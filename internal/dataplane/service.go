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
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	yaml "gopkg.in/yaml.v3"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-playground/validator/v10"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/internal/dataplane/util"
)

// ServiceYAML struct for service YAML unmarshalling
type ServiceYAML struct {
	Kind     string
	Metadata yaml.Node
	Spec     yaml.Node
}

// MissingServiceError indicates that a requested service does not exist.
type MissingServiceError struct {
	NodeSet string
	Service string
	Err     error
}

func (e *MissingServiceError) Error() string {
	return e.Err.Error()
}

func (e *MissingServiceError) Unwrap() error {
	return e.Err
}

// ServiceCache stores reconcile-local service objects to avoid repeated API lookups.
type ServiceCache struct {
	services map[string]dataplanev1.OpenStackDataPlaneService
}

// NewServiceCache returns an empty reconcile-local service cache.
func NewServiceCache() *ServiceCache {
	return &ServiceCache{
		services: map[string]dataplanev1.OpenStackDataPlaneService{},
	}
}

// Get returns a cached service or fetches it once from the API.
func (c *ServiceCache) Get(ctx context.Context, helper *helper.Helper, service string) (dataplanev1.OpenStackDataPlaneService, error) {
	if cached, ok := c.services[service]; ok {
		return cached, nil
	}

	foundService, err := GetService(ctx, helper, service)
	if err != nil {
		return dataplanev1.OpenStackDataPlaneService{}, err
	}

	c.services[service] = foundService
	return foundService, nil
}

// DeployService service deployment
func (d *Deployer) DeployService(foundService dataplanev1.OpenStackDataPlaneService, aeeSpec *dataplanev1.AnsibleEESpec) error {
	err := dataplaneutil.AnsibleExecution(
		d.Ctx,
		d.Helper,
		d.Deployment,
		&foundService,
		d.AnsibleSSHPrivateKeySecrets,
		d.InventorySecrets,
		aeeSpec,
		d.NodeSet)
	if err != nil {
		d.Helper.GetLogger().Error(err, "Unable to execute Ansible", "service", foundService.Name)
		return err
	}

	return nil
}

// GetService return service
func GetService(ctx context.Context, helper *helper.Helper, service string) (dataplanev1.OpenStackDataPlaneService, error) {
	client := helper.GetClient()
	beforeObj := helper.GetBeforeObject()
	namespace := beforeObj.GetNamespace()
	foundService := &dataplanev1.OpenStackDataPlaneService{}
	err := client.Get(ctx, types.NamespacedName{Name: service, Namespace: namespace}, foundService)
	return *foundService, err
}

// EnsureServices - ensure the OpenStackDataPlaneServices exist
func EnsureServices(ctx context.Context, helper *helper.Helper, instance *dataplanev1.OpenStackDataPlaneNodeSet, validation *validator.Validate) error {
	servicesPath, found := os.LookupEnv("OPERATOR_SERVICES")
	if !found {
		servicesPath = "config/services"
		if err := os.Setenv("OPERATOR_SERVICES", servicesPath); err != nil {
			return fmt.Errorf("failed to set OPERATOR_SERVICES environment variable: %w", err)
		}
		util.LogForObject(
			helper, "OPERATOR_SERVICES not set in env when reconciling ", instance,
			"defaulting to ", servicesPath)
	}

	helper.GetLogger().Info("Ensuring services", "servicesPath", servicesPath)
	services, err := os.ReadDir(servicesPath)
	if err != nil {
		return err
	}

	for _, service := range services {

		servicePath := path.Join(servicesPath, service.Name())

		if !strings.HasSuffix(service.Name(), ".yaml") {
			helper.GetLogger().Info("Skipping ensuring service from file without .yaml suffix", "file", service.Name())
			continue
		}

		data, _ := os.ReadFile(servicePath)
		var serviceObj ServiceYAML
		err = yaml.Unmarshal(data, &serviceObj)
		if err != nil {
			helper.GetLogger().Info("Service YAML file Unmarshal error", "service YAML file", servicePath)
			return err
		}

		if serviceObj.Kind != "OpenStackDataPlaneService" {
			helper.GetLogger().Info("Skipping ensuring service since kind is not OpenStackDataPlaneService", "file", servicePath, "Kind", serviceObj.Kind)
			continue
		}

		serviceObjMeta := &metav1.ObjectMeta{}
		err = serviceObj.Metadata.Decode(serviceObjMeta)
		if err != nil {
			helper.GetLogger().Info("Service Metadata decode error")
			return err
		}
		// Check if service name matches RFC1123 for use in labels
		if err = validation.Var(serviceObjMeta.Name, "hostname_rfc1123"); err != nil {
			helper.GetLogger().Info("service name must follow RFC1123")
			return err
		}

		serviceObjSpec := &dataplanev1.OpenStackDataPlaneServiceSpec{}
		err = serviceObj.Spec.Decode(serviceObjSpec)
		if err != nil {
			helper.GetLogger().Info("Service Spec decode error")
			return err
		}

		ensureService := &dataplanev1.OpenStackDataPlaneService{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceObjMeta.Name,
				Namespace: instance.Namespace,
			},
		}
		_, err = controllerutil.CreateOrPatch(ctx, helper.GetClient(), ensureService, func() error {
			serviceObjSpec.DeepCopyInto(&ensureService.Spec)
			return nil
		})
		if err != nil {
			return fmt.Errorf("error ensuring service: %w", err)
		}

	}

	return nil
}

// DedupeServices deduplicates services to deploy and builds execution
// levels based on service dependencies. The returned map contains a
// leveled execution plan for each nodeset: each level is a list of
// services whose dependencies are all in earlier levels.
func DedupeServices(ctx context.Context, helper *helper.Helper,
	nodesets []dataplanev1.OpenStackDataPlaneNodeSet,
	serviceOverride []string,
	fallbackToListOrder bool,
) (map[string][][]string, *ServiceCache, error) {
	nodeSetServiceMap := make(map[string][][]string)
	var globalServices []string
	var services []string
	var err error
	serviceCache := NewServiceCache()

	for i := range nodesets {
		nodeset := &nodesets[i]
		var nodeSetServices []string
		if len(serviceOverride) != 0 {
			nodeSetServices = serviceOverride
		} else {
			nodeSetServices = nodeset.Spec.Services
		}
		services, globalServices, err = dedupe(ctx, helper, serviceCache, nodeset.Name, nodeSetServices, globalServices)
		if err != nil {
			return nil, nil, err
		}
		levels, err := BuildServiceLevels(ctx, helper, serviceCache, services, fallbackToListOrder)
		if err != nil {
			return nil, nil, fmt.Errorf("error building service dependency graph for nodeset %s: %w", nodeset.Name, err)
		}
		nodeSetServiceMap[nodeset.Name] = levels
	}
	helper.GetLogger().Info("Current global services", "services", globalServices)
	return nodeSetServiceMap, serviceCache, nil
}

func dedupe(ctx context.Context, helper *helper.Helper,
	serviceCache *ServiceCache,
	nodeSetName string,
	services []string, globalServices []string) ([]string, []string, error) {
	var dedupedServices []string
	var nodeSetServiceTypes []string
	updatedglobalServices := globalServices
	for _, svc := range services {
		service, err := serviceCache.Get(ctx, helper, svc)
		if err != nil {
			if !k8s_errors.IsNotFound(err) {
				return dedupedServices, updatedglobalServices, err
			}
			return dedupedServices, updatedglobalServices, &MissingServiceError{
				NodeSet: nodeSetName,
				Service: svc,
				Err:     err,
			}
		}

		serviceType := service.Spec.EDPMServiceType
		if serviceType == "" {
			serviceType = service.Name
		}
		if !slices.Contains(nodeSetServiceTypes, serviceType) && !slices.Contains(dedupedServices, svc) {
			if service.Spec.DeployOnAllNodeSets {
				if !slices.Contains(globalServices, svc) {
					updatedglobalServices = append(globalServices, svc)
				} else {
					continue
				}
			}
			nodeSetServiceTypes = append(nodeSetServiceTypes, serviceType)
			dedupedServices = append(dedupedServices, svc)
		}
	}
	return dedupedServices, updatedglobalServices, nil
}
