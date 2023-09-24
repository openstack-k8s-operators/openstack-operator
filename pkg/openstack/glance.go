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

	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileGlance -
func ReconcileGlance(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	glance := &glancev1.Glance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "glance",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Glance.Enabled {
		if res, err := EnsureDeleted(ctx, helper, glance); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneGlanceReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeGlanceReadyCondition)
		return ctrl.Result{}, nil
	}

	serviceOverrides := map[service.Endpoint]service.RoutedOverrideSpec{}
	if instance.Spec.Glance.Template.GlanceAPIExternal.Override.Service != nil {
		serviceOverrides[service.EndpointPublic] = *instance.Spec.Glance.Template.GlanceAPIExternal.Override.Service
	}
	if instance.Spec.Glance.Template.GlanceAPIInternal.Override.Service != nil {
		serviceOverrides[service.EndpointInternal] = *instance.Spec.Glance.Template.GlanceAPIInternal.Override.Service
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		svcOverride := serviceOverrides[endpointType]
		serviceOverrides[endpointType] = AddServiceComponentLabel(svcOverride, glance.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "glance", Namespace: instance.Namespace}, glance); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	if glance.Status.Conditions.IsTrue(glancev1.GlanceAPIReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: glance.Name},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		serviceOverrides, ctrlResult, err = EnsureRoute(
			ctx,
			instance,
			helper,
			glance,
			svcs,
			serviceOverrides,
			instance.Spec.Glance.APIOverride.Route,
			corev1beta1.OpenStackControlPlaneExposeGlanceReadyCondition,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	helper.GetLogger().Info("Reconciling Glance", "Glance.Namespace", instance.Namespace, "Glance.Name", "glance")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), glance, func() error {
		instance.Spec.Glance.Template.DeepCopyInto(&glance.Spec)
		glance.Spec.GlanceAPIExternal.Override.Service = ptr.To(serviceOverrides[service.EndpointPublic])
		glance.Spec.GlanceAPIInternal.Override.Service = ptr.To(serviceOverrides[service.EndpointInternal])

		if glance.Spec.Secret == "" {
			glance.Spec.Secret = instance.Spec.Secret
		}
		if glance.Spec.DatabaseInstance == "" {
			glance.Spec.DatabaseInstance = "openstack"
		}
		if glance.Spec.StorageClass == "" {
			glance.Spec.StorageClass = instance.Spec.StorageClass
		}
		// if already defined at service level (template section), we don't merge
		// with the global defined extra volumes
		if len(glance.Spec.ExtraMounts) == 0 {

			var glanceVolumes []glancev1.GlanceExtraVolMounts

			for _, ev := range instance.Spec.ExtraMounts {
				glanceVolumes = append(glanceVolumes, glancev1.GlanceExtraVolMounts{
					Name:      ev.Name,
					Region:    ev.Region,
					VolMounts: ev.VolMounts,
				})
			}
			glance.Spec.ExtraMounts = glanceVolumes
		}
		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), glance, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneGlanceReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneGlanceReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("glance %s - %s", glance.Name, op))
	}

	if glance.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneGlanceReadyCondition, corev1beta1.OpenStackControlPlaneGlanceReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneGlanceReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneGlanceReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
