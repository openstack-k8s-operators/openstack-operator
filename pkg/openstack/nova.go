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

	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileNova -
func ReconcileNova(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, version *corev1beta1.OpenStackVersion, helper *helper.Helper) (ctrl.Result, error) {
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

	// When component services got created check if there is the need to create routes and certificates
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "nova", Namespace: instance.Namespace}, nova); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	// Add selectors and CA bundle to service overrides for api, metadata and novncproxy
	// NovaAPI
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Nova.Template.APIServiceTemplate.Override.Service == nil {
			instance.Spec.Nova.Template.APIServiceTemplate.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Nova.Template.APIServiceTemplate.Override.Service[endpointType] =
			AddServiceOpenStackOperatorLabel(
				instance.Spec.Nova.Template.APIServiceTemplate.Override.Service[endpointType],
				nova.Name+"-api")
	}
	// preserve any previously set TLS certs,set CA cert
	if instance.Spec.TLS.PodLevel.Enabled {
		instance.Spec.Nova.Template.APIServiceTemplate.TLS = nova.Spec.APIServiceTemplate.TLS
	}
	instance.Spec.Nova.Template.APIServiceTemplate.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName

	// NovaMetadata
	if metadataEnabled(instance.Spec.Nova.Template.MetadataServiceTemplate) {
		if instance.Spec.Nova.Template.MetadataServiceTemplate.Override.Service == nil {
			instance.Spec.Nova.Template.MetadataServiceTemplate.Override.Service = &service.OverrideSpec{}
		}
		instance.Spec.Nova.Template.MetadataServiceTemplate.Override.Service.AddLabel(centralMetadataLabelMap(nova.Name))

		// preserve any previously set TLS certs,set CA cert
		if instance.Spec.TLS.PodLevel.Enabled {
			instance.Spec.Nova.Template.MetadataServiceTemplate.TLS = nova.Spec.MetadataServiceTemplate.TLS
		}
		instance.Spec.Nova.Template.MetadataServiceTemplate.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
	}
	// Cells
	for cellName, cellTemplate := range instance.Spec.Nova.Template.CellTemplates {
		// add override where novncproxy enabled is not specified or explicitely set to true
		if noVNCProxyEnabled(cellTemplate.NoVNCProxyServiceTemplate) {
			if cellTemplate.NoVNCProxyServiceTemplate.Override.Service == nil {
				cellTemplate.NoVNCProxyServiceTemplate.Override.Service = &service.RoutedOverrideSpec{}
			}
			cellTemplate.NoVNCProxyServiceTemplate.Override.Service.AddLabel(getNoVNCProxyLabelMap(nova.Name, cellName))

			// preserve any previously set TLS certs,set CA cert
			if instance.Spec.TLS.PodLevel.Enabled {
				cellTemplate.NoVNCProxyServiceTemplate.TLS = nova.Spec.CellTemplates[cellName].NoVNCProxyServiceTemplate.TLS
			}
			cellTemplate.NoVNCProxyServiceTemplate.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
		}

		// add override where metadata enabled is set to true
		if metadataEnabled(cellTemplate.MetadataServiceTemplate) {
			if cellTemplate.MetadataServiceTemplate.Override.Service == nil {
				cellTemplate.MetadataServiceTemplate.Override.Service = &service.OverrideSpec{}
			}
			cellTemplate.MetadataServiceTemplate.Override.Service.AddLabel(cellMetadataLabelMap(nova.Name, cellName))

			// preserve any previously set TLS certs,set CA cert
			if instance.Spec.TLS.PodLevel.Enabled {
				cellTemplate.MetadataServiceTemplate.TLS = nova.Spec.CellTemplates[cellName].MetadataServiceTemplate.TLS
			}
			cellTemplate.MetadataServiceTemplate.TLS.CaBundleSecretName = instance.Status.TLS.CaBundleSecretName
		}
		instance.Spec.Nova.Template.CellTemplates[cellName] = cellTemplate
	}

	// Nova API
	svcs, err := service.GetServicesListWithLabel(
		ctx,
		helper,
		instance.Namespace,
		GetServiceOpenStackOperatorLabel(nova.Name+"-api"),
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	// make sure to get to EndpointConfig when all service got created
	if len(svcs.Items) == len(instance.Spec.Nova.Template.APIServiceTemplate.Override.Service) {
		endpointDetails, ctrlResult, err := EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			nova,
			svcs,
			instance.Spec.Nova.Template.APIServiceTemplate.Override.Service,
			instance.Spec.Nova.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeNovaReadyCondition,
			false, // TODO (mschuppert) could be removed when all integrated service support TLS
			instance.Spec.Nova.Template.APIServiceTemplate.TLS,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
		// set service overrides
		instance.Spec.Nova.Template.APIServiceTemplate.Override.Service = endpointDetails.GetEndpointServiceOverrides()
		// set NovaAPI TLS cert secret
		instance.Spec.Nova.Template.APIServiceTemplate.TLS.API.Public.SecretName =
			endpointDetails.GetEndptCertSecret(service.EndpointPublic)
		instance.Spec.Nova.Template.APIServiceTemplate.TLS.API.Internal.SecretName =
			endpointDetails.GetEndptCertSecret(service.EndpointInternal)
	}

	// create certificate for central Metadata agent if internal TLS and Metadata are enabled
	if instance.Spec.TLS.PodLevel.Enabled &&
		metadataEnabled(instance.Spec.Nova.Template.MetadataServiceTemplate) {
		certScrt, ctrlResult, err := certmanager.EnsureCertForServiceWithSelector(
			ctx,
			helper,
			nova.Namespace,
			instance.Spec.Nova.Template.MetadataServiceTemplate.Override.Service.Labels,
			instance.GetInternalIssuer(),
			nil)
		if err != nil && !k8s_errors.IsNotFound(err) {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		// update NovaMetadata cert secret
		instance.Spec.Nova.Template.MetadataServiceTemplate.TLS.SecretName = ptr.To(certScrt)
	}

	// cell Metadata and NoVNCProxy
	for cellName, cellTemplate := range instance.Spec.Nova.Template.CellTemplates {
		// create certificate for Metadata agend if internal TLS and Metadata per cell is enabled
		if instance.Spec.TLS.PodLevel.Enabled &&
			metadataEnabled(cellTemplate.MetadataServiceTemplate) {

			certScrt, ctrlResult, err := certmanager.EnsureCertForServiceWithSelector(
				ctx,
				helper,
				nova.Namespace,
				cellTemplate.MetadataServiceTemplate.Override.Service.Labels,
				instance.GetInternalIssuer(),
				nil)
			if err != nil && !k8s_errors.IsNotFound(err) {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}
			// update NovaMetadata cert secret
			cellTemplate.MetadataServiceTemplate.TLS.SecretName = ptr.To(certScrt)
		}

		// NoVNCProxy check for/creating route if service is enabled
		if noVNCProxyEnabled(cellTemplate.NoVNCProxyServiceTemplate) {
			if cellTemplate.NoVNCProxyServiceTemplate.Override.Service == nil {
				cellTemplate.NoVNCProxyServiceTemplate.Override.Service = &service.RoutedOverrideSpec{}
			}

			svcs, err := service.GetServicesListWithLabel(
				ctx,
				helper,
				instance.Namespace,
				getNoVNCProxyLabelMap(nova.Name, cellName),
			)
			if err != nil {
				return ctrl.Result{}, err
			}

			// make sure to get to EndpointConfig when all service got created
			if len(svcs.Items) == 1 {
				endpointDetails, ctrlResult, err := EnsureEndpointConfig(
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
					false, // TODO (mschuppert) could be removed when all integrated service support TLS
					tls.API{
						API: tls.APIService{
							Public: tls.GenericService{
								SecretName: cellTemplate.NoVNCProxyServiceTemplate.TLS.SecretName,
							},
						},
					},
				)
				if err != nil {
					return ctrlResult, err
				} else if (ctrlResult != ctrl.Result{}) {
					return ctrlResult, nil
				}
				// set service overrides
				routedOverrideSpec := endpointDetails.GetEndpointServiceOverrides()
				cellTemplate.NoVNCProxyServiceTemplate.Override.Service = ptr.To(routedOverrideSpec[service.EndpointPublic])
				// update NoVNCProxy cert secret
				cellTemplate.NoVNCProxyServiceTemplate.TLS.SecretName =
					endpointDetails.GetEndptCertSecret(service.EndpointPublic)
			}
		}

		instance.Spec.Nova.Template.CellTemplates[cellName] = cellTemplate
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

		nova.Spec.NovaImages.APIContainerImageURL = *version.Status.ContainerImages.NovaAPIImage
		nova.Spec.NovaImages.NovaComputeContainerImageURL = *version.Status.ContainerImages.NovaComputeImage
		nova.Spec.NovaImages.ConductorContainerImageURL = *version.Status.ContainerImages.NovaConductorImage
		nova.Spec.NovaImages.MetadataContainerImageURL = *version.Status.ContainerImages.NovaAPIImage //metadata uses novaAPI image
		nova.Spec.NovaImages.SchedulerContainerImageURL = *version.Status.ContainerImages.NovaSchedulerImage
		nova.Spec.NovaImages.NoVNCContainerImageURL = *version.Status.ContainerImages.NovaNovncImage

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

	if nova.IsReady() { //FIXME ObservedGeneration
		instance.Status.ContainerImages.NovaAPIImage = version.Status.ContainerImages.NovaAPIImage
		instance.Status.ContainerImages.NovaComputeImage = version.Status.ContainerImages.NovaComputeImage
		instance.Status.ContainerImages.NovaConductorImage = version.Status.ContainerImages.NovaConductorImage
		instance.Status.ContainerImages.NovaNovncImage = version.Status.ContainerImages.NovaNovncImage
		instance.Status.ContainerImages.NovaSchedulerImage = version.Status.ContainerImages.NovaSchedulerImage
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

func getNoVNCProxyLabelMap(name string, cellName string) map[string]string {
	return util.MergeMaps(
		GetServiceOpenStackOperatorLabel(name+"-novncproxy"),
		map[string]string{"cell": cellName},
	)
}

func getMetadataLabelMap(name string, instType string) map[string]string {
	return util.MergeMaps(
		GetServiceOpenStackOperatorLabel(name+"-metadata"),
		map[string]string{"type": instType},
	)
}

func centralMetadataLabelMap(name string) map[string]string {
	return getMetadataLabelMap(name, "central")
}

func cellMetadataLabelMap(name string, cell string) map[string]string {
	lm := getMetadataLabelMap(name, "cell")
	lm["cell"] = cell
	return lm
}

func metadataEnabled(metadata novav1.NovaMetadataTemplate) bool {
	return metadata.Enabled != nil && *metadata.Enabled == true
}

func noVNCProxyEnabled(vncproxy novav1.NovaNoVNCProxyTemplate) bool {
	return vncproxy.Enabled != nil && *vncproxy.Enabled == true
}
