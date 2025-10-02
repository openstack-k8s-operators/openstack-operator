package openstack

import (
	"context"
	"fmt"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ServiceUserConfig defines a single user for a service that needs an ApplicationCredential
type ServiceUserConfig struct {
	UserName         string
	PasswordSelector string
	Suffix           string // For services that have multiple keystone service users (ironic)
}

// servicesWithMultipleUsers defines services that have multiple Keystone users
// The function returns nil if the service is not enabled or template is not initialized
var servicesWithMultipleUsers = map[string]func(*corev1beta1.OpenStackControlPlane) []ServiceUserConfig{
	"heat": func(instance *corev1beta1.OpenStackControlPlane) []ServiceUserConfig {
		if !instance.Spec.Heat.Enabled || instance.Spec.Heat.Template == nil {
			return nil
		}
		// Note: heat_stack_domain_admin is excluded from app cred support
		// as it requires special handling for domain-scoped authentication
		return []ServiceUserConfig{
			{
				UserName:         instance.Spec.Heat.Template.ServiceUser,
				PasswordSelector: instance.Spec.Heat.Template.PasswordSelectors.Service,
				Suffix:           "", // Main service, no suffix -> "ac-heat"
			},
		}
	},
	"ironic": func(instance *corev1beta1.OpenStackControlPlane) []ServiceUserConfig {
		if !instance.Spec.Ironic.Enabled || instance.Spec.Ironic.Template == nil {
			return nil
		}
		return []ServiceUserConfig{
			{
				UserName:         instance.Spec.Ironic.Template.ServiceUser,
				PasswordSelector: instance.Spec.Ironic.Template.PasswordSelectors.Service,
				Suffix:           "", // Main service, no suffix -> "ac-ironic"
			},
			{
				UserName:         instance.Spec.Ironic.Template.IronicInspector.ServiceUser,
				PasswordSelector: instance.Spec.Ironic.Template.IronicInspector.PasswordSelectors.Service,
				Suffix:           "inspector", // -> "ac-ironic-inspector"
			},
		}
	},
}

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
		for _, svc := range []string{"glance", "nova", "swift", "ceilometer", "barbican", "cinder", "placement", "neutron", "ironic", "heat", "octavia", "manila", "designate", "watcher", "aodh", "cloudkitty"} {
			if res, err := deleteServiceACs(ctx, helper, instance, svc); err != nil {
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
		{"ceilometer", instance.Spec.Telemetry.Enabled, instance.Spec.Telemetry.ApplicationCredentialCeilometer},
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
		{"aodh", instance.Spec.Telemetry.Enabled, instance.Spec.Telemetry.ApplicationCredentialAodh},
		{"cloudkitty", instance.Spec.Telemetry.Enabled, instance.Spec.Telemetry.ApplicationCredentialCloudKitty},
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
		case "cloudkitty":
			secretName = instance.Spec.Telemetry.Template.CloudKitty.Secret
			passwordSelector = instance.Spec.Telemetry.Template.CloudKitty.PasswordSelectors.CloudKittyService
			serviceUser = instance.Spec.Telemetry.Template.CloudKitty.ServiceUser
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
		// Check if service is enabled
		if !svc.Enabled {
			// Delete all possible AC CRs for this service
			if res, err := deleteServiceACs(ctx, helper, instance, svc.Key); err != nil {
				return res, err
			}
			continue
		}

		// Merge flags - only check AC enabled after verifying service is enabled
		effective := mergeAppCred(global, svc.ACSection)
		if !effective.Enabled {
			// Delete all possible AC CRs for this service
			if res, err := deleteServiceACs(ctx, helper, instance, svc.Key); err != nil {
				return res, err
			}
			continue
		}

		// Check if this service has multiple users
		if userConfigsFn, hasMultiple := servicesWithMultipleUsers[svc.Key]; hasMultiple {
			userConfigs := userConfigsFn(instance)
			if userConfigs == nil || len(userConfigs) == 0 {
				// Service enabled but template not ready yet
				log.Info("Skipping ApplicationCredential creation: Template not initialized", "service", svc.Key)
				continue
			}

			// Get base service info (secretName)
			svcInfo := getServiceInfo(svc.Key)
			if svcInfo.SecretName == "" {
				log.Info("Skipping ApplicationCredential creation: Secret name empty", "service", svc.Key)
				continue
			}

			// Create AC CR for each user
			for _, userCfg := range userConfigs {
				acName := fmt.Sprintf("ac-%s", svc.Key)
				if userCfg.Suffix != "" {
					acName = fmt.Sprintf("ac-%s-%s", svc.Key, userCfg.Suffix)
				}

				if err := reconcileApplicationCredential(
					ctx,
					helper,
					instance,
					acName,
					userCfg.UserName,
					svcInfo.SecretName,
					userCfg.PasswordSelector,
					effective,
				); err != nil {
					return ctrl.Result{}, err
				}
			}
		} else {
			// Single user service
			svcInfo := getServiceInfo(svc.Key)
			if svcInfo.SecretName == "" || svcInfo.PasswordSelector == "" {
				log.Info("Skipping ApplicationCredential creation: Template fields empty", "service", svc.Key)
				if res, err := deleteServiceACs(ctx, helper, instance, svc.Key); err != nil {
					return res, err
				}
				continue
			}

			acName := fmt.Sprintf("ac-%s", svc.Key)
			if err := reconcileApplicationCredential(
				ctx,
				helper,
				instance,
				acName,
				svcInfo.ServiceUser,
				svcInfo.SecretName,
				svcInfo.PasswordSelector,
				effective,
			); err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	return ctrl.Result{}, nil
}

// reconcileApplicationCredential creates or updates a single ApplicationCredential CR
func reconcileApplicationCredential(
	ctx context.Context,
	helper *helper.Helper,
	instance *corev1beta1.OpenStackControlPlane,
	acName string,
	userName string,
	secretName string,
	passwordSelector string,
	effective corev1beta1.ApplicationCredentialSection,
) error {
	log := GetLogger(ctx)

	acObj := &keystonev1.KeystoneApplicationCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      acName,
			Namespace: instance.Namespace,
		},
	}

	op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), acObj, func() error {
		acObj.Spec.UserName = userName
		acObj.Spec.ExpirationDays = *effective.ExpirationDays
		acObj.Spec.GracePeriodDays = *effective.GracePeriodDays
		acObj.Spec.Secret = secretName
		acObj.Spec.PasswordSelector = passwordSelector
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
		return err
	}
	if op != controllerutil.OperationResultNone {
		log.Info("Reconciled ApplicationCredential", "name", acName, "user", userName, "operation", op)
	}
	return nil
}

// deleteServiceACs deletes all AC CRs for a service (handles both single and multi-user)
func deleteServiceACs(
	ctx context.Context,
	helper *helper.Helper,
	instance *corev1beta1.OpenStackControlPlane,
	serviceKey string,
) (ctrl.Result, error) {
	// List of possible AC CR names for this service
	possibleNames := []string{
		fmt.Sprintf("ac-%s", serviceKey),
	}

	// Add additional names for multi-user services
	if serviceKey == "ironic" {
		possibleNames = append(possibleNames, "ac-ironic-inspector")
	}

	// Try to delete each possible CR
	for _, acName := range possibleNames {
		acObj := &keystonev1.KeystoneApplicationCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name:      acName,
				Namespace: instance.Namespace,
			},
		}
		if res, err := EnsureDeleted(ctx, helper, acObj); err != nil {
			return res, err
		}
	}

	return ctrl.Result{}, nil
}
