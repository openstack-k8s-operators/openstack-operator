package openstack

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	"github.com/go-logr/logr"
	routev1 "github.com/openshift/api/route/v1"
	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/clusterdns"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/object"
	"github.com/openstack-k8s-operators/lib-common/modules/common/route"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"

	// corev1 "k8s.io/api/core/v1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ooSelector is used as a selector to the labels to differentiate from
	// any other possible labels added to services, because  e.g. the common.AppSelector
	// is also used by service operators.
	ooSelector = "osctlplane"

	// ooAppSelector service selector label added by the openstack-operator to service
	// overrides
	ooAppSelector = "osctlplane-service"

	// serviceCertSelector selector passed to cert-manager to set on the service cert secret
	serviceCertSelector = "service-cert"

	// caCertSelector selector passed to cert-manager to set on the ca cert secret
	caCertSelector = "ca-cert"
)

// GetLogger returns a logger object with a prefix of "controller.name" and additional controller context fields
func GetLogger(ctx context.Context) logr.Logger {
	return log.FromContext(ctx).WithName("Controllers").WithName("OpenstackControlPlane")
}

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

// GetServiceOpenStackOperatorLabel - returns the labels to be added to service override
func GetServiceOpenStackOperatorLabel(value string) map[string]string {
	return map[string]string{
		ooAppSelector: value,
		ooSelector:    "",
	}
}

// AddServiceOpenStackOperatorLabel - adds labels to the service override to be able to query
// the service labels to check for any route creation
func AddServiceOpenStackOperatorLabel(svcOverride service.RoutedOverrideSpec, value string) service.RoutedOverrideSpec {
	for s, v := range GetServiceOpenStackOperatorLabel(value) {
		svcOverride.AddLabel(map[string]string{s: v})
	}

	return svcOverride
}

// Endpoints represents service endpoint details and TLS CA configuration
type Endpoints struct {
	EndpointDetails map[service.Endpoint]EndpointDetail
	tls.Ca
}

// EndpointDetail - endpoint details
type EndpointDetail struct {
	Name        string
	Namespace   string
	Type        service.Endpoint
	Annotations map[string]string
	Labels      map[string]string
	Service     ServiceDetails
	Route       RouteDetails
	Hostname    *string
	Proto       service.Protocol
	EndpointURL string
}

// ServiceTLSDetails - tls settings for the endpoint
type ServiceTLSDetails struct {
	Enabled  bool
	CertName string
	tls.GenericService
	tls.Ca
}

// ServiceDetails - service details
type ServiceDetails struct {
	Spec         *k8s_corev1.Service
	OverrideSpec service.RoutedOverrideSpec
	TLS          ServiceTLSDetails
}

// RouteDetails - route details
type RouteDetails struct {
	Create       bool
	Route        *routev1.Route
	OverrideSpec route.OverrideSpec
	TLS          RouteTLSDetails
}

// RouteTLSDetails - tls settings for the endpoint
type RouteTLSDetails struct {
	Enabled    bool
	SecretName *string
	CertName   string
	IssuerName string
	tls.Ca
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
		err = fmt.Errorf("error listing routes for %s: %w", labelSelectorMap, err)
		return nil, err
	}

	return routeList, nil
}

// GetEndpointServiceOverrides -
func (e *Endpoints) GetEndpointServiceOverrides() map[service.Endpoint]service.RoutedOverrideSpec {
	overrides := map[service.Endpoint]service.RoutedOverrideSpec{}
	for endpt, endptCfg := range e.EndpointDetails {
		overrides[endpt] = *endptCfg.Service.OverrideSpec.DeepCopy()
	}

	return overrides
}

// EnsureEndpointConfig -
func EnsureEndpointConfig(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	owner metav1.Object,
	svcs *k8s_corev1.ServiceList,
	svcOverrides map[service.Endpoint]service.RoutedOverrideSpec,
	ingressOverride corev1.Override,
	condType condition.Type,
	serviceTLSDisabled bool,
	tlsConfig tls.API,
) (Endpoints, ctrl.Result, error) {
	endpoints := Endpoints{
		EndpointDetails: map[service.Endpoint]EndpointDetail{},
	}
	clusterDomain := clusterdns.GetDNSClusterDomain()

	for _, svc := range svcs.Items {
		ed := EndpointDetail{
			Name:      svc.Name,
			Namespace: svc.Namespace,
			Type:      service.Endpoint(svc.Annotations[service.AnnotationEndpointKey]),
			Service: ServiceDetails{
				Spec: &svc,
			},
		}

		ed.Service.OverrideSpec = svcOverrides[ed.Type]
		// TLS on the pod level is enabled if
		// * TLS is enabled for pod level
		// * the particular service has not TLS.Disabled set to true
		if instance.Spec.TLS.PodLevel.Enabled && !serviceTLSDisabled {
			ed.Service.TLS.Enabled = true
			ed.Service.TLS.CertName = fmt.Sprintf("%s-svc", ed.Name)
		} else {
			ed.Service.TLS.Enabled = false
		}

		switch ed.Type {
		case service.EndpointPublic:
			// If the service has the create ingress annotation and its a default ClusterIP service -> create route
			ed.Route.Create = svc.Annotations[service.AnnotationIngressCreateKey] == "true" &&
				svc.Spec.Type == k8s_corev1.ServiceTypeClusterIP

			if ingressOverride.Route != nil {
				ed.Route.OverrideSpec = *ingressOverride.Route
			}

			// if TLS termination at the ingress (route) is enabled
			if instance.Spec.TLS.Ingress.Enabled {
				// TLS for route enabled if public endpoint TLS is true
				ed.Route.TLS.Enabled = true
				ed.Route.TLS.CertName = fmt.Sprintf("%s-route", ed.Name)

				// if a custom cert secret was provided we'll use this for
				// the route, otherwise the issuer is used to request one
				// for the endpoint.
				if ingressOverride.TLS != nil && ingressOverride.TLS.SecretName != "" {
					ed.Route.TLS.SecretName = ptr.To(ingressOverride.TLS.SecretName)
					validateSecret := &tls.GenericService{SecretName: ed.Route.TLS.SecretName}
					_, err := validateSecret.ValidateCertSecret(ctx, helper, instance.GetNamespace())
					if err != nil {
						if k8s_errors.IsNotFound(err) {
							instance.Status.Conditions.Set(condition.FalseCondition(
								corev1.OpenStackControlPlaneCustomTLSReadyCondition,
								condition.RequestedReason,
								condition.SeverityInfo,
								corev1.OpenStackControlPlaneCustomTLSReadyWaitingMessage,
								ingressOverride.TLS.SecretName))
							return endpoints, ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
						}
						instance.Status.Conditions.Set(condition.FalseCondition(
							corev1.OpenStackControlPlaneCustomTLSReadyCondition,
							condition.ErrorReason,
							condition.SeverityWarning,
							corev1.OpenStackControlPlaneCustomTLSReadyErrorMessage,
							ingressOverride.TLS.SecretName,
							err.Error()))
						return endpoints, ctrl.Result{}, err
					}
					instance.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneCustomTLSReadyCondition,
						corev1.OpenStackControlPlaneCustomTLSReadyMessage)
				} else {
					ed.Route.TLS.IssuerName = instance.GetPublicIssuer()
				}
			}

			if ed.Service.TLS.Enabled {
				ed.Service.TLS.CaBundleSecretName = tls.CABundleSecret
				// if a custom cert secret was provided and ed.Route.Create == false
				// we'll use this for the service, otherwise issue a cert. This is for
				// use case where you deploy without ingress/routes and also use
				// a LoadBalancer (MetalLB) for the public endpoints.
				if !ed.Route.Create && (tlsConfig.API.Public.SecretName != nil && *tlsConfig.API.Public.SecretName != "") {
					ed.Service.TLS.SecretName = tlsConfig.API.Public.SecretName
					_, err := ed.Service.TLS.ValidateCertSecret(ctx, helper, instance.GetNamespace())
					if err != nil {
						if k8s_errors.IsNotFound(err) {
							return endpoints, ctrl.Result{RequeueAfter: time.Duration(10) * time.Second}, nil
						}
						return endpoints, ctrl.Result{}, err
					}
					// Delete the issued certificate if it exists
					cert := certmanager.NewCertificate(
						&certmgrv1.Certificate{
							ObjectMeta: metav1.ObjectMeta{
								Name:      ed.Route.TLS.CertName,
								Namespace: ed.Namespace,
							},
						},
						5*time.Second,
					)
					err = cert.Delete(ctx, helper)
					if err != nil {
						return endpoints, ctrl.Result{}, err
					}
				} else {
					// issue a certificate for public pod virthost
					certRequest := certmanager.CertificateRequest{
						IssuerName: instance.GetPublicIssuer(),
						CertName:   ed.Service.TLS.CertName,
						Hostnames: []string{
							fmt.Sprintf("%s.%s.svc", ed.Name, instance.Namespace),
							fmt.Sprintf("%s.%s.svc.%s", ed.Name, instance.Namespace, clusterDomain),
						},
						Ips:         nil,
						Annotations: ed.Annotations,
						Labels:      util.MergeMaps(ed.Labels, map[string]string{serviceCertSelector: ""}),
						Usages:      nil,
					}

					addSubjNames := util.GetStringListFromMap(svc.Annotations, tls.AdditionalSubjectNamesKey)
					if len(addSubjNames) > 0 {
						certRequest.Hostnames = append(certRequest.Hostnames, addSubjNames...)
					}
					if instance.Spec.TLS.Ingress.Cert.Duration != nil {
						certRequest.Duration = &instance.Spec.TLS.Ingress.Cert.Duration.Duration
					}
					if instance.Spec.TLS.Ingress.Cert.RenewBefore != nil {
						certRequest.RenewBefore = &instance.Spec.TLS.Ingress.Cert.RenewBefore.Duration
					}
					certSecret, ctrlResult, err := certmanager.EnsureCert(
						ctx,
						helper,
						certRequest,
						nil)
					if err != nil {
						return endpoints, ctrlResult, err
					} else if (ctrlResult != ctrl.Result{}) {
						return endpoints, ctrlResult, nil
					}

					ed.Service.TLS.SecretName = &certSecret.Name
				}
			}

			ctrlResult, err := ed.ensureRoute(ctx, instance, helper, &svc, owner, condType)
			if err != nil {
				return endpoints, ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return endpoints, ctrlResult, nil
			}

		case service.EndpointInternal:
			if ed.Service.TLS.Enabled {
				ed.Service.TLS.CaBundleSecretName = tls.CABundleSecret
				// create certificate for internal pod virthost
				// request certificate
				certRequest := certmanager.CertificateRequest{
					IssuerName: instance.GetInternalIssuer(),
					CertName:   ed.Service.TLS.CertName,
					Hostnames: []string{
						fmt.Sprintf("%s.%s.svc", ed.Name, instance.Namespace),
						fmt.Sprintf("%s.%s.svc.%s", ed.Name, instance.Namespace, clusterDomain),
					},
					Ips:         nil,
					Annotations: ed.Annotations,
					Labels:      util.MergeMaps(ed.Labels, map[string]string{serviceCertSelector: ""}),
					Usages:      nil,
				}

				addSubjNames := util.GetStringListFromMap(svc.Annotations, tls.AdditionalSubjectNamesKey)
				if len(addSubjNames) > 0 {
					certRequest.Hostnames = append(certRequest.Hostnames, addSubjNames...)
				}
				if instance.Spec.TLS.PodLevel.Internal.Cert.Duration != nil {
					certRequest.Duration = &instance.Spec.TLS.PodLevel.Internal.Cert.Duration.Duration
				}
				if instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore != nil {
					certRequest.RenewBefore = &instance.Spec.TLS.PodLevel.Internal.Cert.RenewBefore.Duration
				}
				certSecret, ctrlResult, err := certmanager.EnsureCert(
					ctx,
					helper,
					certRequest,
					nil)
				if err != nil {
					return endpoints, ctrlResult, err
				} else if (ctrlResult != ctrl.Result{}) {
					return endpoints, ctrlResult, nil
				}

				ed.Service.TLS.SecretName = &certSecret.Name
			}
		}

		// update override for the service with the endpoint url
		if ed.EndpointURL != "" {
			// Any trailing path will be added on the service-operator level.
			ed.Service.OverrideSpec.EndpointURL = &ed.EndpointURL
			instance.Status.Conditions.MarkTrue(condType, corev1.OpenStackControlPlaneExposeServiceReadyMessage, owner.GetName())
		}

		endpoints.EndpointDetails[ed.Type] = ed
	}

	return endpoints, ctrl.Result{}, nil
}

func (ed *EndpointDetail) ensureRoute(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	svc *k8s_corev1.Service,
	owner metav1.Object,
	condType condition.Type,
) (ctrl.Result, error) {
	// check if there is already a route with ooAppSelector from the service
	if svcLabelVal, ok := svc.Labels[ooAppSelector]; ok {
		routes, err := GetRoutesListWithLabel(
			ctx,
			helper,
			instance.Namespace,
			map[string]string{ooAppSelector: svcLabelVal},
		)
		if err != nil {
			return ctrl.Result{}, err
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
			if svc.Annotations[service.AnnotationIngressCreateKey] == "false" &&
				r.Spec.To.Name == ed.Name &&
				owner != nil && owner.UID == instance.GetUID() {
				// Delete any other owner refs from ref list to not block deletion until owners are gone
				r.SetOwnerReferences([]metav1.OwnerReference{instanceRef})

				// Delete certificate for the route
				if ed.Service.TLS.Enabled {
					err = DeleteCertificate(ctx, helper, instance.Namespace, ed.Route.TLS.CertName)
					if err != nil && !k8s_errors.IsNotFound(err) {
						err = fmt.Errorf("error deleting route certificate %s: %w", ed.Route.TLS.CertName, err)
						return ctrl.Result{}, err
					}
				}

				// Delete route
				err := helper.GetClient().Delete(ctx, &r)
				if err != nil && !k8s_errors.IsNotFound(err) {
					err = fmt.Errorf("error deleting route %s: %w", r.Name, err)
					return ctrl.Result{}, err
				}

				if ed.Service.OverrideSpec.EndpointURL != nil {
					ed.Service.OverrideSpec.EndpointURL = nil
					helper.GetLogger().Info(fmt.Sprintf("Service %s override endpointURL removed", svc.Name))
				}
			}
		}
	}

	if ed.Route.Create {
		if instance.Status.Conditions.Get(condType) == nil {
			instance.Status.Conditions.Set(condition.UnknownCondition(
				condType,
				condition.InitReason,
				corev1.OpenStackControlPlaneExposeServiceReadyInitMessage,
				owner.GetName(),
				svc.Name,
			))
		}
		if ed.Service.OverrideSpec.EmbeddedLabelsAnnotations == nil {
			ed.Service.OverrideSpec.EmbeddedLabelsAnnotations = &service.EmbeddedLabelsAnnotations{}
		}

		if labelVal, ok := ed.Service.OverrideSpec.Labels[ooAppSelector]; ok {
			ed.Labels = map[string]string{ooAppSelector: labelVal}
		}

		ctrlResult, err := ed.CreateRoute(ctx, instance, helper, owner)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				condType,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1.OpenStackControlPlaneExposeServiceReadyErrorMessage,
				owner.GetName(),
				ed.Name,
				err.Error()))

			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		return ctrl.Result{}, nil
	}
	instance.Status.Conditions.Remove(condType)

	return ctrl.Result{}, nil
}

// CreateRoute -
func (ed *EndpointDetail) CreateRoute(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	owner metav1.Object,
) (ctrl.Result, error) {
	// initialize the route with any custom provided route override
	// per default use the service name as targetPortName if we don't have the annotation.
	targetPortName := ed.Service.Spec.Name
	if name, ok := ed.Service.Spec.Annotations[service.AnnotationIngressTargetPortNameKey]; ok && name != "" {
		targetPortName = name
	}
	enptRoute, err := route.NewRoute(
		route.GenericRoute(&route.GenericRouteDetails{
			Name:           ed.Name,
			Namespace:      ed.Namespace,
			Labels:         ed.Labels,
			ServiceName:    ed.Service.Spec.Name,
			TargetPortName: targetPortName,
		}),
		time.Duration(5)*time.Second,
		[]route.OverrideSpec{ed.Route.OverrideSpec},
	)
	if err != nil {
		return ctrl.Result{}, err
	}
	// in EnvTest the UID of the created service CRs are empty
	if owner.GetUID() != "" {
		enptRoute.OwnerReferences = append(enptRoute.OwnerReferences, owner)
	}

	// if route TLS is disabled -> create the route
	// if TLS is enabled and the route does not yet exist -> create the route
	// to get the hostname for creating the cert
	serviceRoute := &routev1.Route{}
	err = helper.GetClient().Get(ctx, types.NamespacedName{Name: ed.Name, Namespace: ed.Namespace}, serviceRoute)
	if !ed.Route.TLS.Enabled || (ed.Route.TLS.Enabled && err != nil && k8s_errors.IsNotFound(err)) {
		ctrlResult, err := enptRoute.CreateOrPatch(ctx, helper)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		ed.Hostname = ptr.To(enptRoute.GetHostname())
	} else if err != nil {
		return ctrl.Result{}, err
	} else {
		ed.Hostname = &serviceRoute.Spec.Host
	}

	// if TLS is enabled for the route
	if ed.Route.TLS.Enabled {
		var ctrlResult reconcile.Result

		certSecret := &k8s_corev1.Secret{}

		// if a custom cert secret was provided, check if it exist
		// and has the required cert, key and cacert
		// Right now there is no check if certificate is valid for
		// the hostname of the route. If the referenced secret is
		// there and has the required files it is just being used.
		if ed.Route.TLS.SecretName != nil {
			certSecret, _, err = secret.GetSecret(ctx, helper, *ed.Route.TLS.SecretName, ed.Namespace)
			if err != nil {
				if k8s_errors.IsNotFound(err) {
					return ctrl.Result{}, fmt.Errorf("certificate secret %s not found: %w", *ed.Route.TLS.SecretName, err)
				}

				return ctrl.Result{}, err
			}

			// check if secret has the expected entries tls.crt, tls.key and ca.crt
			if certSecret != nil {
				for _, key := range []string{"tls.crt", "tls.key", "ca.crt"} {
					if _, exist := certSecret.Data[key]; !exist {
						return ctrl.Result{}, fmt.Errorf("certificate secret %s does not provide %s", *ed.Route.TLS.SecretName, key)
					}
				}
			}
		}

		if ed.Route.TLS.SecretName == nil && !hasCertInOverrideSpec(ed.Route.OverrideSpec) {
			certRequest := certmanager.CertificateRequest{
				IssuerName:  ed.Route.TLS.IssuerName,
				CertName:    ed.Route.TLS.CertName,
				Hostnames:   []string{*ed.Hostname},
				Ips:         nil,
				Annotations: ed.Annotations,
				Labels:      util.MergeMaps(ed.Labels, map[string]string{serviceCertSelector: ""}),
				Usages:      nil,
			}
			if instance.Spec.TLS.Ingress.Cert.Duration != nil {
				certRequest.Duration = &instance.Spec.TLS.Ingress.Cert.Duration.Duration
			}
			if instance.Spec.TLS.Ingress.Cert.RenewBefore != nil {
				certRequest.RenewBefore = &instance.Spec.TLS.Ingress.Cert.RenewBefore.Duration
			}
			//create the cert using default issuer for the endpoint
			certSecret, ctrlResult, err = certmanager.EnsureCert(
				ctx,
				helper,
				certRequest,
				nil)
			if err != nil {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}
		}
		// create default TLS route override
		tlsConfig := &routev1.TLSConfig{
			Termination:                   routev1.TLSTerminationEdge,
			Certificate:                   string(certSecret.Data[tls.CertKey]),
			Key:                           string(certSecret.Data[tls.PrivateKey]),
			CACertificate:                 string(certSecret.Data[tls.CAKey]),
			InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyRedirect,
		}

		// for internal TLS (TLSE) use routev1.TLSTerminationReencrypt
		if ed.Service.TLS.Enabled && (ed.Service.TLS.SecretName != nil || hasCertInOverrideSpec(ed.Route.OverrideSpec)) {
			// get the TLSInternalCABundleFile to add it to the route
			// to be able to validate public/internal service endpoints
			tlsConfig.DestinationCACertificate, ctrlResult, err = secret.GetDataFromSecret(
				ctx, helper, ed.Service.TLS.CaBundleSecretName, 5, tls.InternalCABundleKey)
			if err != nil {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}

			tlsConfig.Termination = routev1.TLSTerminationReencrypt
		}

		enptRoute, err = route.NewRoute(
			enptRoute.GetRoute(),
			time.Duration(5)*time.Second,
			[]route.OverrideSpec{
				{
					Spec: &route.Spec{
						TLS: tlsConfig,
					},
				},
				ed.Route.OverrideSpec,
			},
		)
		if err != nil {
			return ctrl.Result{}, err
		}

		ctrlResult, err = enptRoute.CreateOrPatch(ctx, helper)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		// Delete the issued certificate if it exists and custom cert secret or direct TLS data was provided
		if ed.Route.TLS.SecretName != nil || hasCertInOverrideSpec(ed.Route.OverrideSpec) {
			cert := certmanager.NewCertificate(
				&certmgrv1.Certificate{
					ObjectMeta: metav1.ObjectMeta{
						Name:      ed.Route.TLS.CertName,
						Namespace: ed.Namespace,
					},
				},
				5*time.Second,
			)
			err := cert.Delete(ctx, helper)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
		ed.Proto = service.ProtocolHTTPS
	} else {
		ed.Proto = service.ProtocolHTTP
	}

	ed.EndpointURL = ed.Proto.String() + "://" + *ed.Hostname

	return ctrl.Result{}, nil
}

// GetEndptCertSecret -
func (e *Endpoints) GetEndptCertSecret(endpt service.Endpoint) *string {
	var endptTLSSecret *string

	if endptCfg, ok := e.EndpointDetails[endpt]; ok && endptCfg.Service.TLS.Enabled {
		endptTLSSecret = endptCfg.Service.TLS.SecretName
	}

	return endptTLSSecret
}

// SetupServiceOperatorDefaults sets up any defaults used by service operator defaulting logic
func SetupServiceOperatorDefaults() {
	// Acquire environmental defaults and initialize service operators that
	// require each respective default

	// Cinder
	cinderv1.SetupDefaults()

	// Glance
	glancev1.SetupDefaults()

	// Ironic
	ironicv1.SetupDefaults()

	// Keystone
	keystonev1.SetupDefaults()

	// Manila
	manilav1.SetupDefaults()

	// MariaDB
	mariadbv1.SetupDefaults()

	// Memcached
	memcachedv1.SetupDefaults()

	// Neutron
	neutronv1.SetupDefaults()

	// Nova
	novav1.SetupDefaults()

	// OVN
	ovnv1.SetupDefaults()

	// Placement
	placementv1.SetupDefaults()

	// Heat
	heatv1.SetupDefaults()

	// Redis
	redisv1.SetupDefaults()

	// DNS
	networkv1.SetupDefaults()

	// Telemetry
	telemetryv1.SetupDefaultsTelemetry()

	// Swift
	swiftv1.SetupDefaults()

	// Octavia
	octaviav1.SetupDefaults()

	// Designate
	designatev1.SetupDefaults()

	//  Barbican
	barbicanv1.SetupDefaults()
}

// GetIssuerCertSecret retrieves the certificate secret from the issuer
func GetIssuerCertSecret(
	ctx context.Context,
	helper *helper.Helper,
	name string,
	namespace string,
) (string, error) {
	// get  issuer
	issuer, err := certmanager.GetIssuerByName(
		ctx,
		helper,
		name,
		namespace,
	)
	if err != nil {
		return "", err
	}
	return issuer.Spec.CA.SecretName, nil
}

func hasCertInOverrideSpec(overrideSpec route.OverrideSpec) bool {
	if overrideSpec.Spec == nil {
		return false
	}
	if overrideSpec.Spec.TLS == nil {
		return false
	}
	return overrideSpec.Spec.TLS.CACertificate != "" &&
		overrideSpec.Spec.TLS.Certificate != "" &&
		overrideSpec.Spec.TLS.Key != ""
}

func serviceExists(route string, services *k8s_corev1.ServiceList) bool {
	for _, svc := range services.Items {
		if svc.Name == route {
			return true
		}
	}
	return false
}

// DeleteCertificate deletes a certificate from the specified namespace
func DeleteCertificate(
	ctx context.Context,
	helper *helper.Helper,
	namespace string,
	certName string) error {

	cert := certmanager.NewCertificate(
		&certmgrv1.Certificate{
			ObjectMeta: metav1.ObjectMeta{
				Name:      certName,
				Namespace: namespace,
			},
		},
		5*time.Second,
	)

	helper.GetLogger().Info(fmt.Sprintf("Deleting cert %s", certName))
	return cert.Delete(ctx, helper)
}

// DeleteCertsAndRoutes deletes certificates and routes for the OpenStack control plane
func DeleteCertsAndRoutes(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
) (ctrl.Result, error) {

	log := GetLogger(ctx)

	// Retrieve all routes, certs and services in the namespace
	routes, err := GetRoutesListWithLabel(ctx, helper, instance.Namespace, nil)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("could not get routes: %w", err)
	}

	certs := &certmgrv1.CertificateList{}
	if err := helper.GetClient().List(ctx, certs, client.InNamespace(instance.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("could not get certificates: %w", err)
	}

	services := &k8s_corev1.ServiceList{}
	if err := helper.GetClient().List(ctx, services, client.InNamespace(instance.Namespace)); err != nil {
		return ctrl.Result{}, fmt.Errorf("could not get services: %w", err)
	}

	var delErrs []error
	for _, route := range routes.Items {

		if !object.CheckOwnerRefExist(instance.GetUID(), route.OwnerReferences) {
			continue
		}

		if serviceExists(route.Spec.To.Name, services) {
			continue
		}

		// Delete certs by service and route-name
		for _, cert := range certs.Items {
			if _, ok := cert.Labels[serviceCertSelector]; ok && strings.Contains(cert.Name, route.Name) {
				if object.CheckOwnerRefExist(instance.GetUID(), cert.OwnerReferences) {
					log.Info("Deleting certificate", ":", cert.Name)
					err := DeleteCertificate(ctx, helper, instance.Namespace, cert.Name)
					if err != nil {
						delErrs = append(delErrs, fmt.Errorf("cert deletion for '%s' failed, because: %w", cert.Name, err))
					}
				}
			}

			// NOTE(auniyal): This is specifically to cleanup novncproxy certs, others service certs do not use commonName as of now
			// TODO: this can be removed once we can map service, route and certs with `osctlplane-service` label
			if strings.Contains(cert.Spec.CommonName, route.Name) {
				if object.CheckOwnerRefExist(instance.GetUID(), cert.OwnerReferences) {
					log.Info("Deleting certificate", ":", cert.Name)
					err := DeleteCertificate(ctx, helper, instance.Namespace, cert.Name)
					if err != nil {
						delErrs = append(delErrs, fmt.Errorf("cert deletion for '%s' failed, because: %w", cert.Name, err))
					}
				}
			}
		}

		log.Info("Deleting route", ":", route.Name)
		_, err := EnsureDeleted(ctx, helper, &route)
		if err != nil {
			delErrs = append(delErrs, fmt.Errorf("route deletion for '%s' failed, because: %w", route.Name, err))
		}
	}

	if len(delErrs) > 0 {
		return ctrl.Result{}, errors.Join(delErrs...)
	}

	return ctrl.Result{}, nil
}
