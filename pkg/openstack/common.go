package openstack

import (
	"context"
	"fmt"
	"time"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/route"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// EnsureDeleted - Delete the object which in turn will clean the sub resources
func EnsureDeleted(ctx context.Context, helper *helper.Helper, obj client.Object) (ctrl.Result, error) {
	key := client.ObjectKeyFromObject(obj)
	if err := helper.GetClient().Get(ctx, key, obj); err != nil {
		if k8s_errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Delete the object
	if obj.GetDeletionTimestamp().IsZero() {
		if err := helper.GetClient().Delete(ctx, obj); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil

}

// AddServiceComponentLabel - adds component label to the service override to be able to query
// the service labels to check for any route creation
func AddServiceComponentLabel(svcOverride service.RoutedOverrideSpec, value string) service.RoutedOverrideSpec {
	if svcOverride.EmbeddedLabelsAnnotations == nil {
		svcOverride.EmbeddedLabelsAnnotations = &service.EmbeddedLabelsAnnotations{}
	}
	svcOverride.EmbeddedLabelsAnnotations.Labels = util.MergeStringMaps(
		svcOverride.EmbeddedLabelsAnnotations.Labels, map[string]string{common.AppSelector: value})

	return svcOverride
}

// RouteDetails - route details
type RouteDetails struct {
	RouteName         string
	Namespace         string
	Endpoint          service.Endpoint
	RouteOverrideSpec *route.OverrideSpec
	ServiceLabel      map[string]string
	ServiceSpec       *k8s_corev1.Service
	endpointURL       string
	hostname          *string
	route             *routev1.Route
}

// GetRoutesListWithLabel - Get all routes in namespace of the obj matching label selector
func GetRoutesListWithLabel(
	ctx context.Context,
	h *helper.Helper,
	namespace string,
	labelSelectorMap map[string]string,
) (*routev1.RouteList, error) {
	routeList := &routev1.RouteList{}
	listOpts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(labelSelectorMap),
	}

	if err := h.GetClient().List(ctx, routeList, listOpts...); err != nil {
		err = fmt.Errorf("Error listing routes for %s: %w", labelSelectorMap, err)
		return nil, err
	}

	return routeList, nil
}

// EnsureRoute -
func EnsureRoute(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	owner metav1.Object,
	svcs *k8s_corev1.ServiceList,
	svcOverrides map[service.Endpoint]service.RoutedOverrideSpec,
	overrideSpec *route.OverrideSpec,
	condType condition.Type,
) (map[service.Endpoint]service.RoutedOverrideSpec, ctrl.Result, error) {

	cleanCondition := true

	for _, svc := range svcs.Items {
		rd := RouteDetails{
			RouteName:         svc.Name,
			Namespace:         svc.Namespace,
			Endpoint:          service.Endpoint(svc.Annotations[service.AnnotationEndpointKey]),
			RouteOverrideSpec: overrideSpec,
			ServiceSpec:       &svc,
		}
		svcOverride := svcOverrides[rd.Endpoint]

		// check if there is already a route with common.AppSelector from the service
		if svcLabelVal, ok := svc.Labels[common.AppSelector]; ok {
			routes, err := GetRoutesListWithLabel(
				ctx,
				helper,
				instance.Namespace,
				map[string]string{common.AppSelector: svcLabelVal},
			)
			if err != nil {
				return svcOverrides, ctrl.Result{}, err
			}

			// check the routes if name changed where we are the owner
			for _, r := range routes.Items {
				instanceRef := metav1.OwnerReference{
					APIVersion:         instance.APIVersion,
					Kind:               instance.Kind,
					Name:               instance.GetName(),
					UID:                instance.GetUID(),
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
				}

				owner := metav1.GetControllerOf(&r.ObjectMeta)

				// Delete the route if the service was changed not to expose a route
				if svc.ObjectMeta.Annotations[service.AnnotationIngressCreateKey] == "false" &&
					r.Spec.To.Name == svc.Name &&
					owner != nil && owner.UID == instance.GetUID() {
					// Delete any other owner refs from ref list to not block deletion until owners are gone
					r.SetOwnerReferences([]metav1.OwnerReference{instanceRef})

					// Delete route
					err := helper.GetClient().Delete(ctx, &r)
					if err != nil && !k8s_errors.IsNotFound(err) {
						err = fmt.Errorf("Error deleting route %s: %w", r.Name, err)
						return svcOverrides, ctrl.Result{}, err
					}

					if svcOverride.EndpointURL != nil {
						svcOverride.EndpointURL = nil
						helper.GetLogger().Info(fmt.Sprintf("Service %s override endpointURL removed", svc.Name))
					}
				}
			}
		}

		// If the service has the create ingress annotation and its a default ClusterIP service -> create route
		if svc.ObjectMeta.Annotations[service.AnnotationIngressCreateKey] == "true" && svc.Spec.Type == k8s_corev1.ServiceTypeClusterIP {
			if instance.Status.Conditions.Get(condType) == nil {
				instance.Status.Conditions.Set(condition.UnknownCondition(
					condType,
					condition.InitReason,
					corev1.OpenStackControlPlaneExposeServiceReadyInitMessage,
					owner.GetName(),
					svc.Name,
				))
			}

			if svcOverride.EmbeddedLabelsAnnotations == nil {
				svcOverride.EmbeddedLabelsAnnotations = &service.EmbeddedLabelsAnnotations{}
			}

			if labelVal, ok := svcOverride.EmbeddedLabelsAnnotations.Labels[common.AppSelector]; ok {
				rd.ServiceLabel = map[string]string{common.AppSelector: labelVal}
			}

			ctrlResult, err := rd.CreateRoute(ctx, helper, owner)
			if err != nil {
				instance.Status.Conditions.Set(condition.FalseCondition(
					condType,
					condition.ErrorReason,
					condition.SeverityWarning,
					corev1.OpenStackControlPlaneExposeServiceReadyErrorMessage,
					owner.GetName(),
					rd.RouteName,
					err.Error()))

				return svcOverrides, ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return svcOverrides, ctrlResult, nil
			}

			cleanCondition = false

			// update override for the service with the route endpoint url
			if rd.endpointURL != "" {
				// Any trailing path will be added on the service-operator level.
				svcOverride.EndpointURL = &rd.endpointURL
				instance.Status.Conditions.MarkTrue(condType, corev1.OpenStackControlPlaneExposeServiceReadyMessage, owner.GetName())
			}
		}

		svcOverrides[rd.Endpoint] = svcOverride
	}

	if cleanCondition {
		instance.Status.Conditions.Remove(condType)
	}

	return svcOverrides, ctrl.Result{}, nil
}

// CreateRoute -
func (rd *RouteDetails) CreateRoute(
	ctx context.Context,
	helper *helper.Helper,
	owner metav1.Object,
) (ctrl.Result, error) {
	// TODO TLS
	route, err := route.NewRoute(
		route.GenericRoute(&route.GenericRouteDetails{
			Name:           rd.RouteName,
			Namespace:      rd.Namespace,
			Labels:         rd.ServiceLabel,
			ServiceName:    rd.ServiceSpec.Name,
			TargetPortName: rd.ServiceSpec.Name,
		}),
		time.Duration(5)*time.Second,
		rd.RouteOverrideSpec,
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	route.OwnerReferences = append(route.OwnerReferences, owner)

	ctrlResult, err := route.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	rd.hostname = ptr.To(route.GetHostname())
	rd.endpointURL = "http://" + *rd.hostname
	rd.route = route.GetRoute()

	return ctrl.Result{}, nil
}
