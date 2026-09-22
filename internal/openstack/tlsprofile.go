package openstack

import (
	"context"
	"fmt"
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
//
// lib-common merges that ConfigMap underneath the ConfigOptions of every
// Template that EnsureConfigMaps/EnsureSecrets renders, so the keys reach the
// service operators with no code of their own. Acting on them is still opt-in
// per service: the Template has to ask for the common template it wants
// (Template.CommonTemplates, e.g. "ssl.conf") and the service's httpd config
// has to Include the rendered file. Until a service does both, the keys are
// merged and ignored and it stays on its own built-in defaults.
//
// NOTE: openstack-operator owns the object's whole lifecycle; lib-common only
// consumes it, and renders built-in defaults when it is not present.
//
// When the control plane does not inherit the cluster TLS profile, nothing
// is resolved: any previously published ConfigMap is withdrawn and the
// TLSProfileReady condition is removed, so the status does not describe a
// feature that is off.
func ReconcileTLSProfile(ctx context.Context, instance *corev1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	Log := GetLogger(ctx)

	// Inheritance disabled: the services stay on the built-in defaults.
	// Withdrawing the ConfigMap (a no-op when it is not there) and dropping
	// the condition makes "inheritance was switched off" converge to the
	// same state as "it was never on".
	if !instance.Spec.TLS.InheritClusterProfile {
		if err := configmap.DeleteConfigMapWithName(ctx, helper, util.TLSProfileConfigMap, instance.Namespace); err != nil {
			return ctrl.Result{}, err
		}
		// a no-op now that InitConditions() only registers the condition when
		// inheritance is on, kept so this does not silently depend on that
		instance.Status.Conditions.Remove(corev1.OpenStackControlPlaneTLSProfileReadyCondition)
		return ctrl.Result{}, nil
	}

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

	cipherSuite, sslProtocol, minTLSVersion, err := resolveTLSProfile(apiServer.Spec.TLSSecurityProfile)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneTLSProfileReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneTLSProfileReadyErrorMessage,
			err.Error()))
		return ctrl.Result{}, err
	}

	Log.Info("Resolved TLS profile from APIServer",
		"MinTLSVersion", minTLSVersion, "SSLCipherSuite", cipherSuite, "SSLProtocol", sslProtocol)

	// Write the retrieved data to a ConfigMap controlled by openstack-operator
	// This is used to push/inject a global configuration to the underlying
	// openstack services
	cm := &k8s_corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      util.TLSProfileConfigMap,
			Namespace: instance.Namespace,
		},
		Data: map[string]string{
			// Apache httpd directives, rendered for lib-common's ssl.conf.
			"SSLCipherSuite": cipherSuite,
			"SSLProtocol":    sslProtocol,

			// The minimum version in neutral form ("VersionTLS12"), passed
			// through as the cluster set it. openstack-operator holds the only
			// RBAC grant on config.openshift.io/apiservers in RHOSO, so galera,
			// rabbitmq, ovn and haproxy cannot learn it any other way, and each
			// needs its own syntax for it: SSLProtocol is useless to them.
			"MinTLSVersion": string(minTLSVersion),
		},
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

	instance.Status.Conditions.MarkTrue(
		corev1.OpenStackControlPlaneTLSProfileReadyCondition,
		corev1.OpenStackControlPlaneTLSProfileReadyMessage)

	return ctrl.Result{}, nil
}

// resolveTLSProfile resolves a TLSSecurityProfile into the Apache httpd
// directives and the minimum TLS version the cluster mandates. If the profile
// is nil, the OpenShift default (Intermediate) is used.
//
// The resolved *TLSProfileSpec deliberately does not escape: for the named
// profiles it points into configv1.TLSProfiles, a package-level var, so handing
// it to callers would leak an alias to process-global state.
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

	return strings.Join(spec.Ciphers, ":"), sslProtocol, spec.MinTLSVersion, nil
}
