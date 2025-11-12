package openstack

import (
	"context"
	"fmt"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// mergeAppCred returns a new ApplicationCredentialSection by overlaying
// service-specific values on top of the global defaults.
func mergeAppCred(
	global corev1beta1.ApplicationCredentialSection,
	svc *corev1beta1.ServiceAppCredSection,
) corev1beta1.ApplicationCredentialSection {
	out := global
	if svc != nil {
		out.Enabled = svc.Enabled

		// only override expiry/grace if specified
		if svc.ExpirationDays != nil {
			out.ExpirationDays = svc.ExpirationDays
		}
		if svc.GracePeriodDays != nil {
			out.GracePeriodDays = svc.GracePeriodDays
		}

		// only override Roles if user set them
		if len(svc.Roles) > 0 {
			out.Roles = svc.Roles
		}
		// only override Unrestricted if user set it
		if svc.Unrestricted != nil {
			out.Unrestricted = svc.Unrestricted
		}
		// only override AccessRules if user set them
		if len(svc.AccessRules) > 0 {
			out.AccessRules = svc.AccessRules
		}
	}

	return out
}

// ReconcileApplicationCredentials ensures an AC CR per enabled service,
// propagating its secret name, passwordSelector, and serviceUser fields.
func ReconcileApplicationCredentials(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	_ *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
) (ctrl.Result, error) {
	log := GetLogger(ctx)

	// If global disabled, delete all ACs:
	if !instance.Spec.ApplicationCredential.Enabled {
		log.Info("Global AC disabled; deleting per-service AC CRs")
		for _, svc := range []string{"glance", "nova", "swift", "ceilometer", "barbican", "cinder", "placement", "neutron", "ironic", "heat", "octavia", "manila", "designate", "watcher", "aodh"} {
			ac := &keystonev1.KeystoneApplicationCredential{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("ac-%s", svc),
					Namespace: instance.Namespace,
				},
			}
			if res, err := EnsureDeleted(ctx, helper, ac); err != nil {
				return res, err
			}
		}
		return ctrl.Result{}, nil
	}

	// Type definition for service AC configuration
	type svcAC struct {
		Key       string
		Enabled   bool
		ACSection *corev1beta1.ServiceAppCredSection
	}

	// Collect each service's enabled flag and AC section
	svcs := []svcAC{
		{"glance", instance.Spec.Glance.Enabled, instance.Spec.Glance.ApplicationCredential},
		{"nova", instance.Spec.Nova.Enabled, instance.Spec.Nova.ApplicationCredential},
		{"swift", instance.Spec.Swift.Enabled, instance.Spec.Swift.ApplicationCredential},
		{"ceilometer", instance.Spec.Telemetry.Enabled, instance.Spec.Telemetry.ApplicationCredential},
		{"barbican", instance.Spec.Barbican.Enabled, instance.Spec.Barbican.ApplicationCredential},
		{"cinder", instance.Spec.Cinder.Enabled, instance.Spec.Cinder.ApplicationCredential},
		{"placement", instance.Spec.Placement.Enabled, instance.Spec.Placement.ApplicationCredential},
		{"neutron", instance.Spec.Neutron.Enabled, instance.Spec.Neutron.ApplicationCredential},
		{"ironic", instance.Spec.Ironic.Enabled, instance.Spec.Ironic.ApplicationCredential},
		{"heat", instance.Spec.Heat.Enabled, instance.Spec.Heat.ApplicationCredential},
		{"octavia", instance.Spec.Octavia.Enabled, instance.Spec.Octavia.ApplicationCredential},
		{"manila", instance.Spec.Manila.Enabled, instance.Spec.Manila.ApplicationCredential},
		{"designate", instance.Spec.Designate.Enabled, instance.Spec.Designate.ApplicationCredential},
		{"watcher", instance.Spec.Watcher.Enabled, instance.Spec.Watcher.ApplicationCredential},
		{"aodh", instance.Spec.Telemetry.Enabled, instance.Spec.Telemetry.AodhApplicationCredential},
	}
	global := instance.Spec.ApplicationCredential

	// Helper functions to safely access Watcher's pointer fields
	getWatcherSecret := func() string {
		if instance.Spec.Watcher.Template != nil && instance.Spec.Watcher.Template.Secret != nil {
			return *instance.Spec.Watcher.Template.Secret
		}
		return ""
	}
	getWatcherServiceUser := func() string {
		if instance.Spec.Watcher.Template != nil && instance.Spec.Watcher.Template.ServiceUser != nil {
			return *instance.Spec.Watcher.Template.ServiceUser
		}
		return ""
	}
	getWatcherPasswordSelector := func() string {
		if instance.Spec.Watcher.Template != nil &&
			instance.Spec.Watcher.Template.PasswordSelectors.Service != nil {
			return *instance.Spec.Watcher.Template.PasswordSelectors.Service
		}
		return ""
	}

	// getServiceInfo retrieves service info from Template
	// When service is enabled, webhook ensures Template is initialized with defaults
	// This function is only called after verifying service is enabled
	getServiceInfo := func(key string) struct {
		SecretName       string
		PasswordSelector string
		ServiceUser      string
	} {
		var secretName, passwordSelector, serviceUser string

		switch key {
		case "glance":
			secretName = instance.Spec.Glance.Template.Secret
			passwordSelector = instance.Spec.Glance.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Glance.Template.ServiceUser
		case "nova":
			secretName = instance.Spec.Nova.Template.Secret
			passwordSelector = instance.Spec.Nova.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Nova.Template.ServiceUser
		case "swift":
			secretName = instance.Spec.Swift.Template.SwiftProxy.Secret
			passwordSelector = instance.Spec.Swift.Template.SwiftProxy.PasswordSelectors.Service
			serviceUser = instance.Spec.Swift.Template.SwiftProxy.ServiceUser
		case "ceilometer":
			secretName = instance.Spec.Telemetry.Template.Ceilometer.Secret
			passwordSelector = instance.Spec.Telemetry.Template.Ceilometer.PasswordSelectors.CeilometerService
			serviceUser = instance.Spec.Telemetry.Template.Ceilometer.ServiceUser
		case "barbican":
			secretName = instance.Spec.Barbican.Template.Secret
			passwordSelector = instance.Spec.Barbican.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Barbican.Template.ServiceUser
		case "cinder":
			secretName = instance.Spec.Cinder.Template.Secret
			passwordSelector = instance.Spec.Cinder.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Cinder.Template.ServiceUser
		case "placement":
			secretName = instance.Spec.Placement.Template.Secret
			passwordSelector = instance.Spec.Placement.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Placement.Template.ServiceUser
		case "neutron":
			secretName = instance.Spec.Neutron.Template.Secret
			passwordSelector = instance.Spec.Neutron.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Neutron.Template.ServiceUser
		case "ironic":
			secretName = instance.Spec.Ironic.Template.Secret
			passwordSelector = instance.Spec.Ironic.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Ironic.Template.ServiceUser
		case "heat":
			secretName = instance.Spec.Heat.Template.Secret
			passwordSelector = instance.Spec.Heat.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Heat.Template.ServiceUser
		case "octavia":
			secretName = instance.Spec.Octavia.Template.Secret
			passwordSelector = instance.Spec.Octavia.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Octavia.Template.ServiceUser
		case "manila":
			secretName = instance.Spec.Manila.Template.Secret
			passwordSelector = instance.Spec.Manila.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Manila.Template.ServiceUser
		case "designate":
			secretName = instance.Spec.Designate.Template.Secret
			passwordSelector = instance.Spec.Designate.Template.PasswordSelectors.Service
			serviceUser = instance.Spec.Designate.Template.ServiceUser
		case "watcher":
			secretName = getWatcherSecret()
			passwordSelector = getWatcherPasswordSelector()
			serviceUser = getWatcherServiceUser()
		case "aodh":
			secretName = instance.Spec.Telemetry.Template.Autoscaling.Aodh.Secret
			passwordSelector = instance.Spec.Telemetry.Template.Autoscaling.Aodh.PasswordSelectors.AodhService
			serviceUser = instance.Spec.Telemetry.Template.Autoscaling.Aodh.ServiceUser
		default:
			return struct {
				SecretName       string
				PasswordSelector string
				ServiceUser      string
			}{}
		}

		// If service-specific Secret is empty, use top-level Secret
		if secretName == "" {
			secretName = instance.Spec.Secret
		}

		return struct {
			SecretName       string
			PasswordSelector string
			ServiceUser      string
		}{secretName, passwordSelector, serviceUser}
	}

	// Loop, CreateOrPatch or delete each AC CR:
	for _, svc := range svcs {
		acName := fmt.Sprintf("ac-%s", svc.Key)
		acObj := &keystonev1.KeystoneApplicationCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name:      acName,
				Namespace: instance.Namespace,
			},
		}

		// Only create AC if service is enabled AND AC is enabled
		// Check service enabled first to avoid accessing Template for disabled services
		if !svc.Enabled {
			if res, err := EnsureDeleted(ctx, helper, acObj); err != nil {
				return res, err
			}
			continue
		}

		// merge flags - only check AC enabled after verifying service is enabled
		effective := mergeAppCred(global, svc.ACSection)
		if !effective.Enabled {
			if res, err := EnsureDeleted(ctx, helper, acObj); err != nil {
				return res, err
			}
			continue
		}

		// Get service info - when service is enabled, webhook ensures Template is initialized
		svcInfo := getServiceInfo(svc.Key)
		if svcInfo.SecretName == "" || svcInfo.PasswordSelector == "" {
			// This should not happen if webhook ran correctly, but handle gracefully
			log.Info("Skipping AC creation: Template fields empty", "service", svc.Key)
			if res, err := EnsureDeleted(ctx, helper, acObj); err != nil {
				return res, err
			}
			continue
		}

		// create/patch
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), acObj, func() error {
			acObj.Spec.UserName = svcInfo.ServiceUser
			acObj.Spec.ExpirationDays = *effective.ExpirationDays
			acObj.Spec.GracePeriodDays = *effective.GracePeriodDays
			acObj.Spec.Secret = svcInfo.SecretName
			acObj.Spec.PasswordSelector = svcInfo.PasswordSelector
			acObj.Spec.Roles = effective.Roles
			acObj.Spec.Unrestricted = *effective.Unrestricted

			if len(effective.AccessRules) > 0 {
				kr := make([]keystonev1.ACRule, 0, len(effective.AccessRules))
				for _, r := range effective.AccessRules {
					kr = append(kr, keystonev1.ACRule{
						Service: r.Service,
						Path:    r.Path,
						Method:  r.Method,
					})
				}
				acObj.Spec.AccessRules = kr
			}

			return controllerutil.SetControllerReference(
				helper.GetBeforeObject(), acObj, helper.GetScheme(),
			)
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			log.Info("Reconciled ApplicationCredential", "service", svc.Key, "operation", op)
		}
	}

	return ctrl.Result{}, nil
}
