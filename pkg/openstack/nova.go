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

	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
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
	Log := GetLogger(ctx)

	if !instance.Spec.Nova.Enabled {
		if res, err := EnsureDeleted(ctx, helper, nova); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneNovaReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeNovaReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		// NovaAPI
		if instance.Spec.Nova.Template.APIServiceTemplate.Override.Service == nil {
			instance.Spec.Nova.Template.APIServiceTemplate.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Nova.Template.APIServiceTemplate.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Nova.Template.APIServiceTemplate.Override.Service[endpointType],
				nova.Name+"-api")

		// cell NoVNCProxy service override
		for cellName, cellTemplate := range instance.Spec.Nova.Template.CellTemplates {
			// skip adding override for all the cells where novncproxy is disabled
			if cellTemplate.NoVNCProxyServiceTemplate.Enabled == ptr.To(false) {
				continue
			}
			if cellTemplate.NoVNCProxyServiceTemplate.Override.Service == nil {
				cellTemplate.NoVNCProxyServiceTemplate.Override.Service = &service.RoutedOverrideSpec{}
			}

			*cellTemplate.NoVNCProxyServiceTemplate.Override.Service =
				AddServiceComponentLabel(
					*cellTemplate.NoVNCProxyServiceTemplate.Override.Service,
					getNoVNCProxyServiceLabel(nova.Name, cellName))

			instance.Spec.Nova.Template.CellTemplates[cellName] = cellTemplate
		}
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "nova", Namespace: instance.Namespace}, nova); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Nova API
	var apiServiceEndpointDetails = Endpoints{}
	if nova.Status.Conditions.IsTrue(novav1.NovaAPIReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: nova.Name + "-api"},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		apiServiceEndpointDetails, ctrlResult, err = EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			nova,
			svcs,
			instance.Spec.Nova.Template.APIServiceTemplate.Override.Service,
			instance.Spec.Nova.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeNovaReadyCondition,
			true, // TODO: (mschuppert) disable TLS for now until implemented
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		instance.Spec.Nova.Template.APIServiceTemplate.Override.Service = apiServiceEndpointDetails.GetEndpointServiceOverrides()
	}

	if nova.Status.Conditions.IsTrue(novav1.NovaAllCellsReadyCondition) {
		// cell NoVNCProxy
		for cellName, cellTemplate := range instance.Spec.Nova.Template.CellTemplates {
			// skip checking for/creating route if service is not enabled
			if cellTemplate.NoVNCProxyServiceTemplate.Enabled == ptr.To(false) {
				continue
			}

			if cellTemplate.NoVNCProxyServiceTemplate.Override.Service == nil {
				cellTemplate.NoVNCProxyServiceTemplate.Override.Service = &service.RoutedOverrideSpec{}
			}

			svcs, err := service.GetServicesListWithLabel(
				ctx,
				helper,
				instance.Namespace,
				map[string]string{
					common.AppSelector: getNoVNCProxyServiceLabel(nova.Name, cellName),
				},
			)
			if err != nil {
				return ctrl.Result{}, err
			}

			var ctrlResult reconcile.Result
			var cellServiceEndpointDetails = Endpoints{}
			cellServiceEndpointDetails, ctrlResult, err = EnsureEndpointConfig(
				ctx,
				instance,
				helper,
				nova,
				svcs,
				map[service.Endpoint]service.RoutedOverrideSpec{
					service.EndpointPublic: *cellTemplate.NoVNCProxyServiceTemplate.Override.Service,
				},
				instance.Spec.Nova.CellOverride[cellName].NoVNCProxy,
				corev1beta1.OpenStackControlPlaneExposeNovaReadyCondition,
				true, // TODO: (mschuppert) disable TLS for now until implemented
			)
			if err != nil {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}

			routedOverrideSpec := cellServiceEndpointDetails.GetEndpointServiceOverrides()
			cellTemplate.NoVNCProxyServiceTemplate.Override.Service = ptr.To(routedOverrideSpec[service.EndpointPublic])

			instance.Spec.Nova.Template.CellTemplates[cellName] = cellTemplate

		}
	}

	Log.Info("Reconciling Nova", "Nova.Namespace", instance.Namespace, "Nova.Name", nova.Name)
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
		Log.Info(fmt.Sprintf("Nova %s - %s", nova.Name, op))
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

	return ctrl.Result{}, nil
}

func getNoVNCProxyServiceLabel(name string, cellName string) string {
	return name + "-novncproxy-" + cellName
}
