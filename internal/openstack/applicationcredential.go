package openstack

import (
	"context"
	"time"

	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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

// isACEnabled checks if AC should be enabled for a given service configuration
func isACEnabled(globalAC corev1beta1.ApplicationCredentialSection, serviceAC *corev1beta1.ServiceAppCredSection) bool {
	// Global AC must be enabled
	if !globalAC.Enabled {
		return false
	}
	// Service AC must be enabled
	return serviceAC != nil && serviceAC.Enabled
}

// EnsureApplicationCredentialForService handles AC creation for a single service.
// If service is not ready, AC creation is deferred
// If AC already exists and is ready, it's used immediately
// If AC doesn't exist and service is ready, AC is created
//
// Returns:
//   - acSecretName: name of the AC secret (from status), empty if not ready
//   - result: ctrl.Result with requeue if AC is being created/not ready
//   - err: any error that occurred
func EnsureApplicationCredentialForService(
	ctx context.Context,
	helper *helper.Helper,
	instance *corev1beta1.OpenStackControlPlane,
	serviceName string,
	serviceReady bool,
	secretName string,
	passwordSelector string,
	serviceUser string,
	acConfig *corev1beta1.ServiceAppCredSection,
) (acSecretName string, result ctrl.Result, err error) {
	Log := GetLogger(ctx)

	// Generate AC CR name
	acName := keystonev1.GetACCRName(serviceName)

	// Check if AC CR exists
	acCR := &keystonev1.KeystoneApplicationCredential{
		ObjectMeta: metav1.ObjectMeta{
			Name:      acName,
			Namespace: instance.Namespace,
		},
	}
	err = helper.GetClient().Get(ctx, types.NamespacedName{Name: acName, Namespace: instance.Namespace}, acCR)

	if err != nil && !k8s_errors.IsNotFound(err) {
		return "", ctrl.Result{}, err
	}
	acExists := err == nil

	// Check if AC is enabled for this service
	if !isACEnabled(instance.Spec.ApplicationCredential, acConfig) {
		// AC disabled for this service - delete AC CR if it exists
		if acExists {
			Log.Info("Application Credential disabled, deleting existing KeystoneApplicationCredential CR", "service", serviceName, "acName", acName)
			if err := helper.GetClient().Delete(ctx, acCR); err != nil && !k8s_errors.IsNotFound(err) {
				return "", ctrl.Result{}, err
			}
		}
		return "", ctrl.Result{}, nil
	}

	// Validate required fields are not empty
	if secretName == "" || passwordSelector == "" || serviceUser == "" {
		Log.Info("Skipping Application Credential creation: required fields not yet defaulted",
			"service", serviceName,
			"secretName", secretName,
			"passwordSelector", passwordSelector,
			"serviceUser", serviceUser)
		return "", ctrl.Result{}, nil
	}

	// Merge global and service-specific AC configuration
	merged := mergeAppCred(instance.Spec.ApplicationCredential, acConfig)

	// Check if AC CR exists and is ready
	if acExists {
		if acCR.IsReady() {
			Log.Info("Application Credential is ready", "service", serviceName, "acName", acName, "secretName", acCR.Status.SecretName)
			return acCR.Status.SecretName, ctrl.Result{}, nil
		}
		// Application Credential exists but not ready yet
		Log.Info("Application Credential not ready yet, requeuing", "service", serviceName, "acName", acName)
		return "", ctrl.Result{RequeueAfter: time.Second * 10}, nil
	}

	// AC doesn't exist
	if !serviceReady {
		// Service not ready, don't create Application Credential yet
		Log.Info("Service not ready, deferring Application Credential creation", "service", serviceName)
		return "", ctrl.Result{}, nil
	}

	// Service is ready, create Application Credential CR
	Log.Info("Service is ready, creating Application Credential", "service", serviceName, "acName", acName)

	err = reconcileApplicationCredential(ctx, helper, instance, acName, serviceUser, secretName, passwordSelector, merged)
	if err != nil {
		return "", ctrl.Result{}, err
	}

	// AC created, but not ready yet - requeue to check readiness
	return "", ctrl.Result{RequeueAfter: time.Second * 5}, nil
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
		log.Info("Reconciled Application Credential", "name", acName, "user", userName, "operation", op)
	}
	return nil
}
