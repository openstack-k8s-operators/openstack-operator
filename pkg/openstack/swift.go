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

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// ReconcileSwift -
func ReconcileSwift(ctx context.Context, instance *corev1beta1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	swift := &swiftv1.Swift{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "swift",
			Namespace: instance.Namespace,
		},
	}

	if !instance.Spec.Swift.Enabled {
		if res, err := EnsureDeleted(ctx, helper, swift); err != nil {
			return res, err
		}
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneSwiftReadyCondition)
		instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneExposeSwiftReadyCondition)
		return ctrl.Result{}, nil
	}

	// add selector to service overrides
	for _, endpointType := range []service.Endpoint{service.EndpointPublic, service.EndpointInternal} {
		if instance.Spec.Swift.Template.SwiftProxy.Override.Service == nil {
			instance.Spec.Swift.Template.SwiftProxy.Override.Service = map[service.Endpoint]service.RoutedOverrideSpec{}
		}
		instance.Spec.Swift.Template.SwiftProxy.Override.Service[endpointType] =
			AddServiceComponentLabel(
				instance.Spec.Swift.Template.SwiftProxy.Override.Service[endpointType],
				swift.Name)
	}

	// When component services got created check if there is the need to create a route
	if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "swift", Namespace: instance.Namespace}, swift); err != nil {
		if !k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
	}

	if swift.Status.Conditions.IsTrue(swiftv1.SwiftProxyReadyCondition) {
		svcs, err := service.GetServicesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{common.AppSelector: swift.Name},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		var ctrlResult reconcile.Result
		instance.Spec.Swift.Template.SwiftProxy.Override.Service, ctrlResult, err = EnsureEndpointConfig(
			ctx,
			instance,
			helper,
			swift,
			svcs,
			instance.Spec.Swift.Template.SwiftProxy.Override.Service,
			instance.Spec.Swift.ProxyOverride,
			corev1beta1.OpenStackControlPlaneExposeSwiftReadyCondition,
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}
	}

	helper.GetLogger().Info("Reconciling Swift", "Swift.Namespace", instance.Namespace, "Swift.Name", "swift")
	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), swift, func() error {
		instance.Spec.Swift.Template.DeepCopyInto(&swift.Spec)

		err := controllerutil.SetControllerReference(helper.GetBeforeObject(), swift, helper.GetScheme())
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneSwiftReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneSwiftReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}
	if op != controllerutil.OperationResultNone {
		helper.GetLogger().Info(fmt.Sprintf("Swift %s - %s", swift.Name, op))
	}

	if swift.IsReady() {
		instance.Status.Conditions.MarkTrue(corev1beta1.OpenStackControlPlaneSwiftReadyCondition, corev1beta1.OpenStackControlPlaneSwiftReadyMessage)
	} else {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneSwiftReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1beta1.OpenStackControlPlaneSwiftReadyRunningMessage))
	}

	return ctrl.Result{}, nil
}
