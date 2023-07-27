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
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// GetExternalEnpointDetailsForEndpoint - returns the MetalLBData for the endpoint, if not specified nil.
func GetExternalEnpointDetailsForEndpoint(
	externalEndpoints []corev1.MetalLBConfig,
	endpt service.Endpoint,
) *corev1.MetalLBConfig {
	for _, metallbcfg := range externalEndpoints {
		if metallbcfg.Endpoint == endpt {
			return metallbcfg.DeepCopy()
		}
	}

	return nil
}

// ServiceDetails - service details to create service.Service with overrides
type ServiceDetails struct {
	ServiceName         string
	Namespace           string
	Endpoint            service.Endpoint
	ServiceOverrideSpec map[string]service.OverrideSpec
	RouteOverrideSpec   *route.OverrideSpec
	endpointName        string
	endpointURL         string
	hostname            *string
	route               *routev1.Route
}

// GetEndpointName - returns the name of the endpoint
func (sd *ServiceDetails) GetEndpointName() string {
	return sd.endpointName
}

// GetEndpointURL - returns the URL of the endpoint (proto://hostname/path)
func (sd *ServiceDetails) GetEndpointURL() string {
	return sd.endpointURL
}

// public endpoint
// 1) Default (service override nil)
// - create route + pass in service selector
// 2) service override provided == LoadBalancer service type
// - do nothing let the service operator handle it
// 3) service override provided == ClusterIP service type
// - create route , merge override
//
// internal endpoint
// 1) Default (service override nil)
// - service operator will create default service
// 2) service override provided
// - do nothing let the service operator handle it

// CreateEndpointServiceOverride -
func (sd *ServiceDetails) CreateEndpointServiceOverride() (*service.Service, error) {

	// get service overrides for the endpoint
	svcOverride := ptr.To(sd.ServiceOverrideSpec[string(sd.Endpoint)])

	// set the endpoint name to service name - endpoint type
	sd.endpointName = sd.ServiceName + "-" + string(sd.Endpoint)

	// if there is no override for the internal endpoint return
	if svcOverride == nil && sd.Endpoint == service.EndpointInternal {
		return nil, nil
	}

	// initialize the service override if not specified via the CR
	if svcOverride == nil {
		svcOverride = &service.OverrideSpec{}
	}

	// Create generic service definitions for either MetalLB, or generic service
	svcDef := &k8s_corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sd.endpointName,
			Namespace: sd.Namespace,
		},
		Spec: k8s_corev1.ServiceSpec{
			Type: k8s_corev1.ServiceTypeClusterIP,
		},
	}

	// Create the service
	svc, err := service.NewService(
		svcDef,
		time.Duration(5)*time.Second, //not really required as we don't create the service here
		svcOverride,
	)
	if err != nil {
		return nil, err
	}

	// add annotation to register service name in dnsmasq if it is a LoadBalancer service
	if svc.GetServiceType() == k8s_corev1.ServiceTypeLoadBalancer {
		svc.AddAnnotation(map[string]string{
			service.AnnotationHostnameKey: svc.GetServiceHostname(),
		})
	}

	return svc, nil
}

// CreateRoute -
func (sd *ServiceDetails) CreateRoute(
	ctx context.Context,
	helper *helper.Helper,
	svc service.Service,
) (ctrl.Result, error) {
	// TODO TLS
	route, err := route.NewRoute(
		route.GenericRoute(&route.GenericRouteDetails{
			Name:      sd.ServiceName,
			Namespace: sd.Namespace,
			Labels: map[string]string{
				common.AppSelector: sd.ServiceName,
			},
			ServiceName:    sd.endpointName,
			TargetPortName: sd.endpointName,
		}),
		time.Duration(5)*time.Second,
		sd.RouteOverrideSpec,
	)
	if err != nil {
		return ctrl.Result{}, err
	}

	ctrlResult, err := route.CreateOrPatch(ctx, helper)
	if err != nil {
		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		return ctrlResult, nil
	}

	sd.hostname = ptr.To(route.GetHostname())
	sd.endpointURL = "http://" + *sd.hostname
	sd.route = route.GetRoute()

	return ctrl.Result{}, nil
}

// AddOwnerRef - adds owner to the OwnerReference list of the route
// Add the service CR to the ownerRef list of the route to prevent the route being deleted
// before the service is deleted. Otherwise this can result cleanup issues which require
// the endpoint to be reachable.
// If ALL objects in the list have been deleted, this object will be garbage collected.
// https://github.com/kubernetes/apimachinery/blob/15d95c0b2af3f4fcf46dce24105e5fbb9379af5a/pkg/apis/meta/v1/types.go#L240-L247
func (sd *ServiceDetails) AddOwnerRef(
	ctx context.Context,
	helper *helper.Helper,
	owner metav1.Object,
) error {
	if sd.route != nil {
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), sd.route, func() error {
			err := controllerutil.SetOwnerReference(owner, sd.route, helper.GetScheme())
			if err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
		if op != controllerutil.OperationResultNone {
			helper.GetLogger().Info(fmt.Sprintf("%s route %s owner reference - %s", sd.ServiceName, sd.route.Name, op))
		}
	}

	return nil
}

// CreateRouteAndServiceOverride -
func (sd *ServiceDetails) CreateRouteAndServiceOverride(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
) (*service.OverrideSpec, ctrl.Result, error) {

	svc, err := sd.CreateEndpointServiceOverride()
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1beta1.OpenStackControlPlaneServiceOverrideReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1beta1.OpenStackControlPlaneServiceOverrideReadyErrorMessage,
			sd.ServiceName,
			string(sd.Endpoint),
			err.Error()))

		return nil, ctrl.Result{}, err
	}

	var svcOverride *service.OverrideSpec
	if svc != nil {
		// Create the route if it is public endpoint and the service type is ClusterIP
		if sd.Endpoint == service.EndpointPublic &&
			svc.GetServiceType() == k8s_corev1.ServiceTypeClusterIP {
			//TODO create TLS cert
			ctrlResult, err := sd.CreateRoute(ctx, helper, *svc)
			if err != nil {
				instance.Status.Conditions.Set(condition.FalseCondition(
					condition.ExposeServiceReadyCondition,
					condition.ErrorReason,
					condition.SeverityWarning,
					condition.ExposeServiceReadyErrorMessage,
					err.Error()))
				return nil, ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return nil, ctrlResult, nil
			}

			instance.Status.Conditions.MarkTrue(condition.ExposeServiceReadyCondition, condition.ExposeServiceReadyMessage)
		}

		// convert ServiceSpec to OverrideServiceSpec
		overrideServiceSpec, err := svc.ToOverrideServiceSpec()
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1beta1.OpenStackControlPlaneServiceOverrideReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1beta1.OpenStackControlPlaneServiceOverrideReadyErrorMessage,
				sd.ServiceName,
				string(sd.Endpoint),
				err.Error()))

			return nil, ctrl.Result{}, err
		}

		svcOverride = &service.OverrideSpec{
			EmbeddedLabelsAnnotations: &service.EmbeddedLabelsAnnotations{
				Annotations: svc.GetAnnotations(),
				Labels:      svc.GetLabels(),
			},
			Spec: overrideServiceSpec,
		}

		if sd.GetEndpointURL() != "" {
			svcOverride.EndpointURL = ptr.To(sd.GetEndpointURL())
		}
	}

	return svcOverride, ctrl.Result{}, nil
}
