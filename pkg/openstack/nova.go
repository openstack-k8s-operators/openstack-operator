/*
Copyright 2022.

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

package openstack

import (
	"context"
	"fmt"

	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileNova -
func ReconcileNova(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	nova := &novav1.Nova{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "nova",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Nova.Enabled {
		if res, err := EnsureDeleted(ctx, helper, nova); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneNovaReadyCondition)
		return ctrl.Result{}, nil
	}

	// Create service overrides to pass into the service CR
	// and expose the public endpoint using a route per default.
	// Any trailing path will be added on the service-operator level.
	apiServiceOverrides := map[string]service.OverrideSpec{}
	serviceDetails := []ServiceDetails{}

	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		sd := ServiceDetails{
			ServiceName:         novav1.GetAPIServiceName(),
			Namespace:           instance.Namespace,
			Endpoint:            endpointType,
			ServiceOverrideSpec: instance.Spec.Nova.Template.APIServiceTemplate.Override.Service,
			RouteOverrideSpec:   instance.Spec.Nova.APIOverride.Route,
		}

		svcOverride, ctrlResult, err := sd.CreateRouteAndServiceOverride(ctx, instance, helper)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		serviceDetails = append(
			serviceDetails,
			sd,
		)

		apiServiceOverrides[string(endpointType)] = *svcOverride
	}

	// Create service overrides to pass into the service CR
	// and expose the public endpoint using a route per default.
	// Any trailing path will be added on the service-operator level.
	metadataServiceOverrides := map[string]map[string]service.OverrideSpec{}
	novncproxyServiceOverrides := map[string]map[string]service.OverrideSpec{}

	// cell service override
	for cellName, template := range instance.Spec.Nova.Template.CellTemplates {
		cellOverride := instance.Spec.Nova.CellOverride[cellName]

		// metadata
		metadataServiceName := novav1.GetMetadataServiceName(&cellName)
		metadataSD := ServiceDetails{
			ServiceName:         metadataServiceName,
			Namespace:           instance.Namespace,
			Endpoint:            service.EndpointInternal,
			ServiceOverrideSpec: template.MetadataServiceTemplate.Override.Service,
			RouteOverrideSpec:   nil,
		}

		metadataSVCOverride, ctrlResult, err := metadataSD.CreateRouteAndServiceOverride(ctx, instance, helper)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		serviceDetails = append(
			serviceDetails,
			metadataSD,
		)

		if metadataServiceOverrides[cellName] == nil {
			metadataServiceOverrides[cellName] = map[string]service.OverrideSpec{}
		}

		metadataServiceOverrides[cellName][string(service.EndpointInternal)] = *metadataSVCOverride
		// metadata - end

		// novncproxy
		for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
			// no novncproxy for cell0
			if cellName == novav1.Cell0Name {
				break
			}
			novncProxySD := ServiceDetails{
				ServiceName:         novav1.GetNoVNCProxyServiceName(&cellName),
				Namespace:           instance.Namespace,
				Endpoint:            endpointType,
				ServiceOverrideSpec: template.NoVNCProxyServiceTemplate.Override.Service,
				RouteOverrideSpec:   cellOverride.NoVNCProxy.Route,
			}

			novncSVCOverride, ctrlResult, err := novncProxySD.CreateRouteAndServiceOverride(ctx, instance, helper)
			if err != nil {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}

			serviceDetails = append(
				serviceDetails,
				novncProxySD,
			)

			if novncproxyServiceOverrides[cellName] == nil {
				novncproxyServiceOverrides[cellName] = map[string]service.OverrideSpec{}
			}

			novncproxyServiceOverrides[cellName][string(endpointType)] = *novncSVCOverride
		}
		// novncproxy - end
	}
	instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneServiceOverrideReadyCondition, corev1beta1.OpenStackControlPlaneServiceOverrideReadyMessage)

	helper.GetLogger().Info("Reconciling Nova", "Nova.Namespace", instance.Namespace, "Nova.Name", nova.Name)
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), nova, func() error {
		// 1)
		// Nova.Spec.APIDatabaseInstance and each NovaCell.CellDatabaseInstance
		// are defaulted to "openstack" in nova-operator and the MariaDB created
		// by openstack-operator is also named "openstack". This works but
		// in production we might want to have separate DB service instances
		// per cell.
		//
		// 2)
		// Each NovaCell.CellMessageBusInstance in defaulted to "rabbitmq" by
		// nova-operator and openstack-operator creates RabbitMQCluster named
		// "rabbitmq" as well. This will not work as sharing rabbitmq
		// between cells will prevent the nova-computes to register itself
		// for the proper cell. Basically each cell will be merged to one,
		// cell0 but cell0 should not have compute nodes registered. Eventually
		// we need to support either rabbitmq vhosts or deploy a separate
		// RabbitMQCluster per nova cell.
		instance.Spec.Nova.Template.DeepCopyInto(&nova.Spec)
		nova.Spec.APIServiceTemplate.Override.Service = apiServiceOverrides

		for cellName, cellTemplate := range nova.Spec.CellTemplates {
			if override, exist := metadataServiceOverrides[cellName]; exist {
				cellTemplate.MetadataServiceTemplate.Override.Service = override
			}
			if override, exist := novncproxyServiceOverrides[cellName]; exist {
				cellTemplate.NoVNCProxyServiceTemplate.Override.Service = override
			}
			nova.Spec.CellTemplates[cellName] = cellTemplate
		}

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), nova, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNovaReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneNovaReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Nova %s - %s", nova.Name, op))
	}

	if nova.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneNovaReadyCondition, corev1beta1.OpenStackControlPlaneNovaReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneNovaReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneNovaReadyRunningMessage))
	}

	for _, sd := range serviceDetails {
		// Add the service CR to the ownerRef list of the route to prevent the route being deleted
		// before the service is deleted. Otherwise this can result cleanup issues which require
		// the endpoint to be reachable.
		// If ALL objects in the list have been deleted, this object will be garbage collected.
		// https://github.com/kubernetes/apimachinery/blob/15d95c0b2af3f4fcf46dce24105e5fbb9379af5a/pkg/apis/meta/v1/types.go#L240-L247
		err = sd.AddOwnerRef(ctx, helper, nova)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}
