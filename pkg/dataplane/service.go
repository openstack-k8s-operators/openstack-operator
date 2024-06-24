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
	"strings"

	"golang.org/x/exp/slices"
	yaml "gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/go-playground/validator/v10"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	dataplaneutil "github.com/openstack-k8s-operators/openstack-operator/pkg/dataplane/util"
)

// ServiceYAML struct for service YAML unmarshalling
type ServiceYAML struct {
	Kind     string
	Metadata yaml.Node
	Spec     yaml.Node
}

// DeployService service deployment
func (d *Deployer) DeployService(foundService dataplanev1.OpenStackDataPlaneService) error {
	err := dataplaneutil.AnsibleExecution(
		d.Ctx,
		d.Helper,
		d.Deployment,
		&foundService,
		d.AnsibleSSHPrivateKeySecrets,
		d.InventorySecrets,
		d.AeeSpec,
		d.NodeSet)

	if err != nil {
		d.Helper.GetLogger().Error(err, fmt.Sprintf("Unable to execute Ansible for %s", foundService.Name))
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
		os.Setenv("OPERATOR_SERVICES", servicesPath)
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
			ensureService.DefaultLabels()
			if ensureService.Spec.EDPMServiceType == "" {
				ensureService.Spec.EDPMServiceType = serviceObjMeta.Name
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("error ensuring service: %w", err)
		}

	}

	return nil
}

// Dedup services for deploy
// Multiple Services of same ServiceType/ServiceName in a nodeset
// Global Services in multiple NodeSets for a deployment
func DedupServices(ctx context.Context, helper *helper.Helper, nodesets []dataplanev1.OpenStackDataPlaneNodeSet) (map[string][]string, error) {
	var globalServices []string
	var nodeSetServiceMap = make(map[string][]string, 0)
	for _, nodeset := range nodesets {
		var services []string
		var nodeSetServiceTypes []string
		for _, svc := range nodeset.Spec.Services {
			service, err := GetService(ctx, helper, svc)
			if err != nil {
				helper.GetLogger().Error(err, fmt.Sprintf("Configured service %s does not exist", svc))
				return nil, err
			}
			if !slices.Contains(nodeSetServiceTypes, service.Spec.EDPMServiceType) && !slices.Contains(services, svc) {
				nodeSetServiceTypes = append(nodeSetServiceTypes, service.Spec.EDPMServiceType)
				services = append(services, svc)
			}
			if service.Spec.DeployOnAllNodeSets && !slices.Contains(globalServices, svc) {
				globalServices = append(globalServices, svc)
				services = append(services, svc)
			}
		}
		nodeSetServiceMap[nodeset.Name] = services
	}
	return nodeSetServiceMap, nil
}
