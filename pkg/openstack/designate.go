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

	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileDesignate -
func ReconcileDesignate(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	designate := &designatev1.Designate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "designate",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Designate.Enabled {
		if res, err := EnsureDeleted(ctx, helper, designate); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneDesignateReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeDesignateReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Designate.Template.DesignateAPI.Override.Service == nil {
			instance.Spec.Designate.Template.DesignateAPI.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Designate.Template.DesignateAPI.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Designate.Template.DesignateAPI.Override.Service[endpointType],
				designate.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "designate", Namespace: instance.Namespace}, designate); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	if designate.Status.Conditions.IsTrue(designatev1.DesignateAPIReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: designate.Name},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		instance.Spec.Designate.Template.DesignateAPI.Override.Service, ctrlResult, err = EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			designate,
			svcs,
			instance.Spec.Designate.Template.DesignateAPI.Override.Service,
			instance.Spec.Designate.APIOverride,
			corev1beta1.OpenStackControlPlaneExposeDesignateReadyCondition,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	helper.GetLogger().Info("Reconciling Designate", "Designate.Namespace", instance.Namespace, "Designate.Name", "designate")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), designate, func() error {
		instance.Spec.Designate.Template.DeepCopyInto(&designate.Spec)

		if designate.Spec.Secret == "" {
			designate.Spec.Secret = instance.Spec.Secret
		}
		if designate.Spec.NodeSelector == nil && instance.Spec.NodeSelector != nil {
			designate.Spec.NodeSelector = instance.Spec.NodeSelector
		}
		if designate.Spec.DatabaseInstance == "" {
			//designate.Spec.DatabaseInstance = instance.Name // name of MariaDB we create here
			designate.Spec.DatabaseInstance = "openstack" //FIXME: see above
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), designate, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneDesignateReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneDesignateReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Designate %s - %s", designate.Name, op))
	}

	if designate.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneDesignateReadyCondition, corev1beta1.OpenStackControlPlaneDesignateReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneDesignateReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneDesignateReadyRunningMessage))
	}

	return ctrl.Result{}, nil

}
