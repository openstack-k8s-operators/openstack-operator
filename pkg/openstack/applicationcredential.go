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

// mergeAppCred returns a new ApplicationCredentialSection
// by starting from the global defaults, then overriding
// only the fields the service user has explicitly set
func mergeAppCred(
	global corev1beta1.ApplicationCredentialSection,
	svc *corev1beta1.ServiceAppCredSection,
) corev1beta1.ApplicationCredentialSection {
	out := global

	if svc != nil {
		// always override Enabled, even if false
		out.Enabled = svc.Enabled

		// only override expiry/grace if the user actually set them
		if svc.ExpirationDays != nil {
			out.ExpirationDays = svc.ExpirationDays
		}
		if svc.GracePeriodDays != nil {
			out.GracePeriodDays = svc.GracePeriodDays
		}
	}

	return out
}

// ReconcileApplicationCredentials ensures that every OpenStack service which
// has AC enabled (both globally and per-service) has a corresponding
// keystone.openstack.org/v1beta1 ApplicationCredential CR, with proper
// ExpirationDays and GracePeriodDays inherited or overridden
func ReconcileApplicationCredentials(
	ctx context.Context,
	instance *corev1beta1.OpenStackControlPlane,
	_ *corev1beta1.OpenStackVersion,
	helper *helper.Helper,
) (ctrl.Result, error) {

	log := GetLogger(ctx)

	// If global AC is turned off, delete service AC CRs
	if !instance.Spec.ApplicationCredential.Enabled {
		log.Info("Global .spec.applicationCredential.enabled is false – deleting all per-service AC CRs")
		for _, svc := range []string{
			"glance", "nova", "swift", "ceilometer",
			"barbican", "cinder", "placement", "neutron",
		} {
			ac := &keystonev1.ApplicationCredential{
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

	// Build list of services to reconcile
	type svcAC struct {
		Name      string
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

	for _, svc := range svcs {
		acName := fmt.Sprintf("ac-%s", svc.Name)
		acObj := &keystonev1.ApplicationCredential{
			ObjectMeta: metav1.ObjectMeta{
				Name:      acName,
				Namespace: instance.Namespace,
			},
		}

		effective := mergeAppCred(global, svc.ACSection)
		// if either the service itself is disabled, or the merged AC.Enabled is false,
		// then ensure that CR is deleted
		if !(svc.Enabled && effective.Enabled) {
			if res, err := EnsureDeleted(ctx, helper, acObj); err != nil {
				return res, err
			}
			continue
		}

		// otherwise create or patch it to have exactly the merged values
		op, err := controllerutil.CreateOrPatch(ctx, helper.GetClient(), acObj, func() error {
			acObj.Spec.UserName = svc.Name
			acObj.Spec.ExpirationDays = *effective.ExpirationDays
			acObj.Spec.GracePeriodDays = *effective.GracePeriodDays
			return controllerutil.SetControllerReference(
				helper.GetBeforeObject(), acObj, helper.GetScheme(),
			)
		})
		if err != nil {
			return ctrl.Result{}, err
		}
		if op != controllerutil.OperationResultNone {
			log.Info("Reconciled ApplicationCredential", "service", svc.Name, "operation", op)
		}
	}

	return ctrl.Result{}, nil
}
