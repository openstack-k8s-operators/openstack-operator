package openstack

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"os"
	"time"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	certmgrmetav1 "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
	"github.com/openstack-k8s-operators/lib-common/modules/certmanager"
	"github.com/openstack-k8s-operators/lib-common/modules/common/condition"
	"github.com/openstack-k8s-operators/lib-common/modules/common/helper"
	"github.com/openstack-k8s-operators/lib-common/modules/common/secret"
	"github.com/openstack-k8s-operators/lib-common/modules/common/service"
	"github.com/openstack-k8s-operators/lib-common/modules/common/tls"
	"github.com/openstack-k8s-operators/lib-common/modules/common/util"
	"golang.org/x/exp/slices"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"

	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"

	ctrl "sigs.k8s.io/controller-runtime"
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

	bundle := newBundle()
	caOnlyBundle := newBundle()

	// load current CA bundle from secret if exist
	currentCASecret, _, err := secret.GetSecret(ctx, helper, tls.CABundleSecret, instance.Namespace)
	if err != nil && !k8s_errors.IsNotFound(err) {
		return ctrl.Result{}, err
	}
	if currentCASecret != nil {
		// full CA Bundle file
		if _, ok := currentCASecret.Data[tls.CABundleKey]; ok {
			err = bundle.getCertsFromPEM(currentCASecret.Data[tls.CABundleKey])
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		// only issuer CA bundle
		if _, ok := currentCASecret.Data[tls.InternalCABundleKey]; ok {
			err = caOnlyBundle.getCertsFromPEM(currentCASecret.Data[tls.InternalCABundleKey])
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	if instance.Status.TLS.Endpoint == nil {
		instance.Status.TLS.Endpoint = map[service.Endpoint]corev1.TLSCAStatus{}
	}
	// create RootCA cert and Issuer that uses the generated CA certificate to issue certs
	for endpoint, config := range instance.Spec.TLS.Endpoint {
		if config.Enabled {

			labels := map[string]string{}
			if endpoint == service.EndpointInternal {
				labels[certmanager.RootCAIssuerInternalLabel] = ""
			}
			caName := tls.DefaultCAPrefix + string(endpoint)
			// always create a root CA and issuer for the endpoint as we can
			// not expect that all services are yet configured to be provided with
			// a custom secret holding the cert/private key
			caCert, ctrlResult, err := createRootCACertAndIssuer(
				ctx,
				instance,
				helper,
				issuerReq,
				caName,
				labels,
			)
			if err != nil {
				return ctrlResult, err
			} else if (ctrlResult != ctrl.Result{}) {
				return ctrlResult, nil
			}

			err = bundle.getCertsFromPEM(caCert)
			if err != nil {
				return ctrl.Result{}, err
			}
			err = caOnlyBundle.getCertsFromPEM(caCert)
			if err != nil {
				return ctrl.Result{}, err
			}

			status := corev1.TLSCAStatus{
				Name: caName,
			}
			if len(caOnlyBundle.certs) > 0 {
				status.Expires = caOnlyBundle.certs[0].expire.String()
			}

			instance.Status.TLS.Endpoint[endpoint] = status
		}
	}
	instance.Status.Conditions.MarkTrue(corev1.OpenStackControlPlaneCAReadyCondition, corev1.OpenStackControlPlaneCAReadyMessage)

	// create/update combined CA secret
	if instance.Spec.TLS.CaBundleSecretName != "" {
		caSecret, _, err := secret.GetSecret(ctx, helper, instance.Spec.TLS.CaBundleSecretName, instance.Namespace)
		if err != nil {
			instance.Status.Conditions.Set(condition.FalseCondition(
				corev1.OpenStackControlPlaneCAReadyCondition,
				condition.ErrorReason,
				condition.SeverityWarning,
				corev1.OpenStackControlPlaneCAReadyErrorMessage,
				"secret",
				instance.Spec.TLS.CaBundleSecretName,
				err.Error()))
			if k8s_errors.IsNotFound(err) {
				timeout := time.Second * 10
				helper.GetLogger().Info(fmt.Sprintf("Certificate %s not found, reconcile in %s", instance.Spec.TLS.CaBundleSecretName, timeout.String()))

				return ctrl.Result{RequeueAfter: timeout}, nil
			}

			return ctrl.Result{}, err
		}

		for _, caCert := range caSecret.Data {
			err = bundle.getCertsFromPEM(caCert)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// get CA bundle from operator image. Downstream and upstream build use a different
	// base image, so the ca bundle cert file can be in different locations
	caBundle, err := getOperatorCABundle(tls.DownstreamTLSCABundlePath)
	if err != nil {
		// if the DownstreamTLSCABundlePath does not exist in the operator image,
		// check for UpstreamTLSCABundlePath
		if errors.Is(err, os.ErrNotExist) {
			helper.GetLogger().Info(fmt.Sprintf("Downstream CA bundle not found using: %s", tls.UpstreamTLSCABundlePath))
			caBundle, err = getOperatorCABundle(tls.UpstreamTLSCABundlePath)
			if err != nil {
				return ctrl.Result{}, err
			}
		} else {
			return ctrl.Result{}, err
		}
	}
	err = bundle.getCertsFromPEM(caBundle)
	if err != nil {
		return ctrl.Result{}, err
	}

	saSecretTemplate := []util.Template{
		{
			Name:               tls.CABundleSecret,
			Namespace:          instance.Namespace,
			Type:               util.TemplateTypeNone,
			InstanceType:       instance.Kind,
			AdditionalTemplate: nil,
			Annotations:        map[string]string{},
			Labels: map[string]string{
				tls.CABundleLabel: "",
			},
			ConfigOptions: nil,
			CustomData: map[string]string{
				tls.CABundleKey:         bundle.getBundlePEM(),
				tls.InternalCABundleKey: caOnlyBundle.getBundlePEM(),
			},
			SkipSetOwner: true, // TODO: (mschuppert) instead add e.g. keystoneapi to secret to prevent keystoneapi on cleanup to switch to not ready
		},
	}

	if err := secret.EnsureSecrets(ctx, helper, instance, saSecretTemplate, nil); err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneCAReadyErrorMessage,
			"secret",
			tls.CABundleSecret,
			err.Error()))

		return ctrlResult, err
	}

	instance.Status.TLS.CaBundleSecretName = tls.CABundleSecret

	return ctrl.Result{}, nil
}

func createRootCACertAndIssuer(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	selfsignedIssuerReq *certmgrv1.Issuer,
	caName string,
	labels map[string]string,
) ([]byte, ctrl.Result, error) {
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

		return nil, ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneCAReadyRunningMessage))

		return nil, ctrlResult, nil
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

		return nil, ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneCAReadyRunningMessage))

		return nil, ctrlResult, nil
	}

	caCert, ctrlResult, err := getCAFromSecret(ctx, instance, helper, caName)
	if err != nil {
		return nil, ctrl.Result{}, err
	} else if (ctrlResult != ctrl.Result{}) {
		return nil, ctrlResult, nil
	}

	return caCert, ctrl.Result{}, nil
}

func getCAFromSecret(
	ctx context.Context,
	instance *corev1.OpenStackControlPlane,
	helper *helper.Helper,
	secretName string,
) ([]byte, ctrl.Result, error) {
	caSecret, ctrlResult, err := secret.GetDataFromSecret(ctx, helper, secretName, time.Duration(5), "ca.crt")
	if err != nil {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.ErrorReason,
			condition.SeverityWarning,
			corev1.OpenStackControlPlaneCAReadyErrorMessage,
			"secret",
			secretName,
			err.Error()))

		return nil, ctrlResult, err
	} else if (ctrlResult != ctrl.Result{}) {
		instance.Status.Conditions.Set(condition.FalseCondition(
			corev1.OpenStackControlPlaneCAReadyCondition,
			condition.RequestedReason,
			condition.SeverityInfo,
			corev1.OpenStackControlPlaneCAReadyRunningMessage))

		return nil, ctrlResult, nil
	}

	return []byte(caSecret), ctrl.Result{}, nil
}

func getOperatorCABundle(caFile string) ([]byte, error) {
	contents, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("File reading error %w", err)
	}

	return contents, nil
}

func days(t time.Time) int {
	return int(math.Round(time.Since(t).Hours() / 24))
}

type caBundle struct {
	certs []caCert
}

type caCert struct {
	hash   string
	cert   *x509.Certificate
	expire time.Time
}

// newBundle returns a new, empty Bundle
func newBundle() *caBundle {
	return &caBundle{
		certs: make([]caCert, 0),
	}
}

func (cab *caBundle) getCertsFromPEM(PEMdata []byte) error {
	if PEMdata == nil {
		return fmt.Errorf("certificate data can't be nil")
	}

	for {
		var block *pem.Block
		block, PEMdata = pem.Decode(PEMdata)

		if block == nil {
			break
		}

		if block.Type != "CERTIFICATE" {
			// only certificates are allowed in a bundle
			return fmt.Errorf("invalid PEM block in bundle: only CERTIFICATE blocks are permitted but found '%s'", block.Type)
		}

		if len(block.Headers) != 0 {
			return fmt.Errorf("invalid PEM block in bundle; blocks are not permitted to have PEM headers")
		}

		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			// the presence of an invalid cert (including things which aren't certs)
			// should cause the bundle to be rejected
			return fmt.Errorf("invalid PEM block in bundle; invalid PEM certificate: %w", err)
		}

		if certificate == nil {
			return fmt.Errorf("failed appending a certificate: certificate is nil")
		}

		// validate if the CA expired
		if -days(certificate.NotAfter) <= 0 {
			continue
		}

		blockHash, err := util.ObjectHash(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed calc hash of PEM block : %w", err)
		}

		// if cert is not already in bundle list add it
		// validate of nextip is already in a reservation and its not us
		f := func(c caCert) bool {
			return c.hash == blockHash
		}
		idx := slices.IndexFunc(cab.certs, f)
		if idx == -1 {
			cab.certs = append(cab.certs,
				caCert{
					hash:   blockHash,
					cert:   certificate,
					expire: certificate.NotAfter,
				})
		}
	}

	return nil
}

// Create PEM bundle from certificates
func (cab *caBundle) getBundlePEM() string {
	var bundleData string

	for _, cert := range cab.certs {
		bundleData += "# " + cert.cert.Issuer.CommonName + "\n" +
			string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.cert.Raw}))
	}

	return bundleData
}
