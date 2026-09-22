package openstack

import (
	"context"
	"fmt"
	"slices"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/configmap"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	k8s_corev1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Let's map the httpd config with the TLSProfile gathered from the APIServer
var tlsVersionToSSLProtocol = map[configv1.TLSProtocolVersion]string{
	configv1.VersionTLS10: "all -SSLv2 -SSLv3",
	configv1.VersionTLS11: "all -SSLv2 -SSLv3 -TLSv1",
	configv1.VersionTLS12: "all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1",
	configv1.VersionTLS13: "all -SSLv2 -SSLv3 -TLSv1 -TLSv1.1 -TLSv1.2",
}

// ReconcileTLSProfile reads the OpenShift APIServer TLS security profile and
// publishes the resolved SSLCipherSuite and SSLProtocol values in the
// util.TLSProfileConfigMap ConfigMap.
// lib-common merges that ConfigMap underneath the ConfigOptions of every
// Template that EnsureConfigMaps/EnsureSecrets renders, so the keys reach the
// service operators.
//
// NOTE: openstack-operator owns the object's whole lifecycle; lib-common only
// consumes it, and renders built-in defaults when it is not present.
// None withdraws any published ConfigMap and removes the TLSProfileReady
// condition, so the status does not describe a feature that is off. Inherit
// takes whatever the cluster declares; a profile named in the spec is
// resolved without consulting the cluster.
func ReconcileTLSProfile(ctx context.Context, instance *corev1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	Log := GetLogger(ctx)

	// Withdrawing the ConfigMap (a no-op when absent) and dropping the
	// condition makes "set back to None" converge on "never set at all".
	if !instance.Spec.TLS.Profile.IsEnabled() {
		if err := configmap.DeleteConfigMapWithName(ctx, helper, util.TLSProfileConfigMap, instance.Namespace); err != nil {
			return ctrl.Result{}, err
		}
		instance.Status.TLS.Profile = nil
		// a no-op while InitConditions() registers it only when enabled,
		// kept so this does not silently depend on that
		instance.Status.Conditions.Remove(corev1.OpenStackControlPlaneTLSProfileReadyCondition)
		return ctrl.Result{}, nil
	}

	// Default to the profile named in the spec; Inherit replaces it below.
	profile := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileType(instance.Spec.TLS.Profile),
	}

	if instance.Spec.TLS.Profile == corev1.TLSProfileInherit {
		apiServer := &configv1.APIServer{}
		if err := helper.GetClient().Get(ctx, types.NamespacedName{Name: "cluster"}, apiServer); err != nil {
			if k8s_errors.IsNotFound(err) {
				Log.Info("APIServer CR not found, skipping TLS profile resolution (defaults will apply)")

				// The cluster declares no APIServer CR, so there is no profile to
				// inherit. Drop any ConfigMap published earlier and fall back to
				// the built-in defaults. If there's a pre-existing leftover, it
				// would keep pinning services to a profile the cluster no longer
				// declares
				if err := configmap.DeleteConfigMapWithName(
					ctx, helper, util.TLSProfileConfigMap, instance.Namespace,
				); err != nil {
					instance.Status.Conditions.Set(condition.FalseCondition(
						corev1.OpenStackControlPlaneTLSProfileReadyCondition,
						condition.ErrorReason,
						condition.SeverityWarning,
						corev1.OpenStackControlPlaneTLSProfileReadyErrorMessage,
						err.Error()))
					return ctrl.Result{}, err
				}

				instance.Status.TLS.Profile = nil

				// Nothing was published here: the cluster mandates no profile, so
				// report that rather than claiming a ConfigMap was created.

				instance.Status.Conditions.MarkTrue(
					corev1.OpenStackControlPlaneTLSProfileReadyCondition,
					corev1.OpenStackControlPlaneTLSProfileReadyNoProfileMessage)
				return ctrl.Result{}, nil
			}
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1.OpenStackControlPlaneTLSProfileReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1.OpenStackControlPlaneTLSProfileReadyErrorMessage,
				err.Error()))
			return ctrl.Result{}, err
		}
		profile = apiServer.Spec.TLSSecurityProfile
	}

	// Covers both cases: the cluster's profile under Inherit, and the
	// Intermediate/Modern named in the spec, which reach the
	// configv1.TLSProfiles lookup because the enum spells them as OpenShift does.
	cipherSuite, sslProtocol, minTLSVersion, err := resolveTLSProfile(profile)
	if err != nil {
		// Only a human editing spec.tls.profile or the APIServer CR can fix
		// this, and both are watched, so requeueing would spin and hold every
		// service behind it. Any ConfigMap already published is left in place.
		Log.Error(err, "Unresolvable TLS profile, not publishing",
			"Profile", instance.Spec.TLS.Profile)
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneTLSProfileReadyCondition,
			condition.ErrorReason,
			condition.SeverityError,
			corev1.OpenStackControlPlaneTLSProfileReadyInvalidMessage,
			err.Error()))
		return ctrl.Result{}, nil
	}

	Log.Info("Resolved TLS profile",
		"Profile", instance.Spec.TLS.Profile, "MinTLSVersion", minTLSVersion,
		"SSLCipherSuite", cipherSuite, "SSLProtocol", sslProtocol)

	data := map[string]string{
		"MinTLSVersion": string(minTLSVersion),
		// Apache httpd directive, rendered for lib-common's ssl.conf.
		"SSLProtocol": sslProtocol,
	}

	if cipherSuite != "" {
		data["SSLCipherSuite"] = cipherSuite
	}

	// Write the retrieved data to a ConfigMap controlled by openstack-operator
	// This is used to push/inject a global configuration to the underlying
	// openstack services
	cm := &k8s_corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.TLSProfileConfigMap,
			Namespace: instance.Namespace,
		},
		Data: data,
	}

	// The control plane is set as controller owner, so the ConfigMap is
	// garbage-collected when the control plane is deleted.
	if _, _, err := configmap.CreateOrPatchRawConfigMap(ctx, helper, instance, cm, false); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneTLSProfileReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneTLSProfileReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	// Record only successful publication. Errors above retain the last status,
	// just as they retain the last published ConfigMap.
	publishedType := configv1.TLSProfileIntermediateType
	if profile != nil {
		publishedType = profile.Type
	}
	source := "Spec"
	if instance.Spec.TLS.Profile == corev1.TLSProfileInherit {
		source = "Cluster"
	}
	instance.Status.TLS.Profile = &corev1.TLSProfileStatus{
		Type:          publishedType,
		Source:        source,
		MinTLSVersion: minTLSVersion,
	}

	instance.Status.Conditions.MarkTrue(
		corev1.OpenStackControlPlaneTLSProfileReadyCondition,
		corev1.OpenStackControlPlaneTLSProfileReadyMessage)

	return ctrl.Result{}, nil
}

// resolveTLSProfile resolves a TLSSecurityProfile into the Apache httpd
// directives and the minimum TLS version the cluster mandates.
//   - For the named profiles, *TLSProfileSpec points into configv1.TLSProfiles.
//   - non-httpd services can process minTLSVersion to take decisions according
//     to their syntax.
func resolveTLSProfile(profile *configv1.TLSSecurityProfile) (
	cipherSuite string, sslProtocol string, minTLSVersion configv1.TLSProtocolVersion, err error,
) {
	// pick the spec the cluster asked for: a Custom profile carries its own,
	// the named ones come from the table OpenShift ships
	var spec *configv1.TLSProfileSpec
	switch {
	case profile == nil:
		spec = configv1.TLSProfiles[configv1.TLSProfileIntermediateType]

	case profile.Type == configv1.TLSProfileCustomType:
		if profile.Custom == nil {
			return "", "", "", fmt.Errorf("custom TLS profile type specified but no custom profile provided")
		}
		spec = &profile.Custom.TLSProfileSpec

	default:
		var ok bool
		spec, ok = configv1.TLSProfiles[profile.Type]
		if !ok {
			return "", "", "", fmt.Errorf("unknown TLS profile type: %s", profile.Type)
		}
	}

	// render it as the pair of httpd directives ssl.conf templates in
	sslProtocol, ok := tlsVersionToSSLProtocol[spec.MinTLSVersion]
	if !ok {
		return "", "", "", fmt.Errorf("unsupported minimum TLS version: %s", spec.MinTLSVersion)
	}
	cipherSuite = strings.Join(spec.Ciphers, ":")
	tls13 := spec.MinTLSVersion == configv1.VersionTLS13
	if cipherSuite != "" {
		// OpenSSL skips the names that do not belong in the directive it gets and
		// fails it once none are left, which stops httpd from starting.
		usable := slices.ContainsFunc(spec.Ciphers, func(c string) bool {
			return strings.HasPrefix(c, "TLS_") == tls13
		})
		if !usable {
			return "", "", "", fmt.Errorf(
				"TLS profile lists no cipher usable at %s", spec.MinTLSVersion)
		}
		// With min TLS 1.3 httpd needs the "TLSv1.3" keyword so the TLS_* suites reach
		// the TLS 1.3 cipher list, else the plain directive fails with "no cipher match":
		// https://httpd.apache.org/docs/2.4/mod/mod_ssl.html#sslciphersuite
		if tls13 {
			cipherSuite = "TLSv1.3 " + cipherSuite
		}
	}
	return cipherSuite, sslProtocol, spec.MinTLSVersion, nil
}
