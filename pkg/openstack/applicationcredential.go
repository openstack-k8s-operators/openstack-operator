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
		for _, svc := range []string{"glance", "nova", "swift", "ceilometer", "barbican", "cinder", "placement", "neutron"} {
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

	// Build a lookup with each service’s secret, selector, and service user name field:
	services := map[string]struct {
		SecretName       string
		PasswordSelector string
		ServiceUser      string
	}{
		"glance":     {instance.Spec.Glance.Template.Secret, instance.Spec.Glance.Template.PasswordSelectors.Service, instance.Spec.Glance.Template.ServiceUser},
		"nova":       {instance.Spec.Nova.Template.Secret, instance.Spec.Nova.Template.PasswordSelectors.Service, instance.Spec.Nova.Template.ServiceUser},
		"swift":      {instance.Spec.Swift.Template.SwiftProxy.Secret, instance.Spec.Swift.Template.SwiftProxy.PasswordSelectors.Service, instance.Spec.Swift.Template.SwiftProxy.ServiceUser},
		"ceilometer": {instance.Spec.Telemetry.Template.Ceilometer.Secret, instance.Spec.Telemetry.Template.Ceilometer.PasswordSelectors.CeilometerService, instance.Spec.Telemetry.Template.Ceilometer.ServiceUser},
		"barbican":   {instance.Spec.Barbican.Template.Secret, instance.Spec.Barbican.Template.PasswordSelectors.Service, instance.Spec.Barbican.Template.ServiceUser},
		"cinder":     {instance.Spec.Cinder.Template.Secret, instance.Spec.Cinder.Template.PasswordSelectors.Service, instance.Spec.Cinder.Template.ServiceUser},
		"placement":  {instance.Spec.Placement.Template.Secret, instance.Spec.Placement.Template.PasswordSelectors.Service, instance.Spec.Placement.Template.ServiceUser},
		"neutron":    {instance.Spec.Neutron.Template.Secret, instance.Spec.Neutron.Template.PasswordSelectors.Service, instance.Spec.Neutron.Template.ServiceUser},
	}

	// Collect each service’s enabled flag and AC section:
	type svcAC struct {
		Key       string
		Enabled   bool
		ACSection *corev1beta1.ServiceAppCredSection
	}
	svcs := []svcAC{
		{"glance", instance.Spec.Glance.Enabled, instance.Spec.Glance.ApplicationCredential},
		{"nova", instance.Spec.Nova.Enabled, instance.Spec.Nova.ApplicationCredential},
		{"swift", instance.Spec.Swift.Enabled, instance.Spec.Swift.ApplicationCredential},
		{"ceilometer", instance.Spec.Telemetry.Enabled, instance.Spec.Telemetry.ApplicationCredential},
		{"barbican", instance.Spec.Barbican.Enabled, instance.Spec.Barbican.ApplicationCredential},
		{"cinder", instance.Spec.Cinder.Enabled, instance.Spec.Cinder.ApplicationCredential},
		{"placement", instance.Spec.Placement.Enabled, instance.Spec.Placement.ApplicationCredential},
		{"neutron", instance.Spec.Neutron.Enabled, instance.Spec.Neutron.ApplicationCredential},
	}
	global := instance.Spec.ApplicationCredential

	// Loop, CreateOrPatch or delete each AC CR:
	for _, svc := range svcs {
		acName := fmt.Sprintf("ac-%s", svc.Key)
		acObj := &keystonev1.KeystoneApplicationCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name:      acName,
				Namespace: instance.Namespace,
			},
		}

		// merge flags
		effective := mergeAppCred(global, svc.ACSection)
		if !(svc.Enabled && effective.Enabled) {
			if res, err := EnsureDeleted(ctx, helper, acObj); err != nil {
				return res, err
			}
			continue
		}

		// create/patch
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), acObj, func() error {
			acObj.Spec.UserName = services[svc.Key].ServiceUser
			acObj.Spec.ExpirationDays = *effective.ExpirationDays
			acObj.Spec.GracePeriodDays = *effective.GracePeriodDays
			acObj.Spec.Secret = services[svc.Key].SecretName
			acObj.Spec.PasswordSelector = services[svc.Key].PasswordSelector
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
