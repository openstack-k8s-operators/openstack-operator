package openstack

import (
	"context"
	"time"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"

	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// CombinedCASecret -
	CombinedCASecret = "combined-ca-bundle"
	// DefaultPublicCAName -
	DefaultPublicCAName = "rootca-" + string(service.EndpointPublic)
	// DefaultInternalCAName -
	DefaultInternalCAName = "rootca-" + string(service.EndpointInternal)
)

// ReconcileCAs -
func ReconcileCAs(ctx context.Context, instance *corev1.OpenStackControlPlane, helper *helper.Helper) (ctrl.Result, error) {
	// create selfsigned-issuer
	issuerReq := certmanager.SelfSignedIssuer(
		"selfsigned-issuer",
		instance.GetNamespace(),
		map[string]string{},
	)
	/*
		// Cleanuo?
		if !instance.Spec.TLS.Enabled {
			if err := cert.Delete(ctx, helper); err != nil {
				return ctrl.Result{}, err
			}
			instance.Status.Conditions.Remove(corev1beta1.OpenStackControlPlaneCAsReadyCondition)

			return ctrl.Result{}, nil
		}
	*/

	helper.GetLogger().Info("Reconciling CAs", "Namespace", instance.Namespace, "Name", issuerReq.Name)

	issuer := certmanager.NewIssuer(issuerReq, 5)
	ctrlResult, err := issuer.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneCAReadyErrorMessage,
			issuerReq.Kind,
			issuerReq.GetName(),
			err.Error()))

		return ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneCAReadyRunningMessage))

		return ctrlResult, nil
	}

	caCerts := map[string]string{}

	// create RootCA cert and Issuer that uses the generated CA certificate to issue certs
	if instance.Spec.TLS.PublicEndpoints.Enabled && instance.Spec.TLS.PublicEndpoints.Issuer == nil {
		caCert, ctrlResult, err := createRootCACertAndIssuer(
			ctx,
			instance,
			helper,
			issuerReq,
			DefaultPublicCAName,
			map[string]string{},
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		caCerts[DefaultPublicCAName] = string(caCert)
	}
	if instance.Spec.TLS.InternalEndpoints.Enabled {
		caCert, ctrlResult, err := createRootCACertAndIssuer(
			ctx,
			instance,
			helper,
			issuerReq,
			DefaultInternalCAName,
			map[string]string{
				certmanager.RootCAIssuerInternalLabel: "",
			},
		)
		if err != nil {
			return ctrlResult, err
		} else if (ctrlResult != ctrl.Result{}) {
			return ctrlResult, nil
		}

		caCerts[DefaultInternalCAName] = string(caCert)
	}
	instance.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneCAReadyCondition, corev1.OpenStackControlPlaneCAReadyMessage)

	// create/update combined CA secret
	if instance.Spec.TLS.CaSecretName != "" {
		caSecret, _, err := secret.GetSecret(ctx, helper, instance.Spec.TLS.CaSecretName, instance.Namespace)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1.OpenStackControlPlaneCAReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1.OpenStackControlPlaneCAReadyErrorMessage,
				"secret",
				instance.Spec.TLS.CaSecretName,
				err.Error()))

			return ctrlResult, err
		}

		for key, ca := range caSecret.Data {
			key := instance.Spec.TLS.CaSecretName + "-" + key
			caCerts[key] = string(ca)
		}
	}

	saSecretTemplate := []util.Template{
		{
			Name:               CombinedCASecret,
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeNone,
			InstanceType:       instance.Kind,
			AdditionalTemplate: nil,
			Annotations:        map[string]string{},
			Labels: map[string]string{
				CombinedCASecret: "",
			},
			ConfigOptions: nil,
			CustomData:    caCerts,
		},
	}

	if err := secret.EnsureSecrets(ctx, helper, instance, saSecretTemplate, nil); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneCAReadyErrorMessage,
			"secret",
			CombinedCASecret,
			err.Error()))

		return ctrlResult, err
	}

	return ctrl.Result{}, nil
}

func createRootCACertAndIssuer(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	selfsignedIssuerReq *certmgrv1.Issuer,
	caName string,
	labels map[string]string,
) (string, ctrl.Result, error) {
	var caCert string
	// create RootCA Certificate used to sign certificates
	caCertReq := certmanager.Cert(
		caName,
		instance.Namespace,
		map[string]string{},
		certmgrv1.CertificateSpec{
			IsCA:       true,
			CommonName: caName,
			SecretName: caName,
			PrivateKey: &certmgrv1.CertificatePrivateKey{
				Algorithm: "ECDSA",
				Size:      256,
			},
			IssuerRef: certmgrmetav1.ObjectReference{
				Name:  selfsignedIssuerReq.Name,
				Kind:  selfsignedIssuerReq.Kind,
				Group: selfsignedIssuerReq.GroupVersionKind().Group,
			},
		})
	cert := certmanager.NewCertificate(caCertReq, 5)

	ctrlResult, err := cert.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneCAReadyErrorMessage,
			caCertReq.Kind,
			caCertReq.Name,
			err.Error()))

		return caCert, ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneCAReadyRunningMessage))

		return caCert, ctrlResult, nil
	}

	// create Issuer that uses the generated CA certificate to issue certs
	issuerReq := certmanager.CAIssuer(
		caCertReq.Name,
		instance.GetNamespace(),
		labels,
		caCertReq.Name,
	)

	issuer := certmanager.NewIssuer(issuerReq, 5)
	ctrlResult, err = issuer.CreateOrPatch(ctx, helper)
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneCAReadyErrorMessage,
			issuerReq.Kind,
			issuerReq.GetName(),
			err.Error()))

		return caCert, ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneCAReadyRunningMessage))

		return caCert, ctrlResult, nil
	}

	caCert, ctrlResult, err = getCAFromSecret(ctx, instance, helper, caName)
	if err != nil {
		return caCert, ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return caCert, ctrlResult, nil
	}

	return caCert, ctrl.Result{}, nil
}

func getCAFromSecret(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	caName string,
) (string, ctrl.Result, error) {
	caSecret, ctrlResult, err := secret.GetDataFromSecret(ctx, helper, caName, time.Duration(5), "ca.crt")
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneCAReadyErrorMessage,
			"secret",
			caName,
			err.Error()))

		return caSecret, ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneCAReadyRunningMessage))

		return caSecret, ctrlResult, nil
	}

	return caSecret, ctrl.Result{}, nil
}
