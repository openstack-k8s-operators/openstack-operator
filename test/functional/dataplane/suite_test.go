/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package functional

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	infrav1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	openstackv1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	dataplanev1 "github.com/openstack-k8s-operators/openstack-operator/api/dataplane/v1beta1"
	dataplanecontrollers "github.com/openstack-k8s-operators/openstack-operator/internal/controller/dataplane"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"

	machineconfig "github.com/openshift/api/machineconfiguration/v1"
	ocp_image "github.com/openshift/api/operator/v1alpha1"

	//revive:disable-next-line:dot-imports
	. "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	test "github.com/openstack-k8s-operators/lib-common/modules/test"

	corewebhook "github.com/openstack-k8s-operators/openstack-operator/internal/webhook/core/v1beta1"
	dataplanewebhook "github.com/openstack-k8s-operators/openstack-operator/internal/webhook/dataplane/v1beta1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	k8sClient client.Client // You'll be using this client in your tests.
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
	logger    logr.Logger
	th        *TestHelper
	namespace string
)

const (
	SecretName           = "test-secret"
	MessageBusSecretName = "rabbitmq-secret"
	ContainerImage       = "test://nova"
	timeout              = 40 * time.Second
	// have maximum 100 retries before the timeout hits
	interval = timeout / 100
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "DataPlane Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	const gomod = "../../../go.mod"

	baremetalCRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/openstack-baremetal-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	infraCRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/infra-operator/apis", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	certmgrv1CRDs, err := test.GetOpenShiftCRDDir("cert-manager/v1", gomod)
	Expect(err).ShouldNot(HaveOccurred())
	openstackCRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/openstack-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	imageContentSourcePolicyCRDs, err := test.GetCRDDirFromModule("github.com/openshift/api", gomod, "operator/v1alpha1/zz_generated.crd-manifests/")
	Expect(err).ShouldNot(HaveOccurred())
	imageDigestMirrorSetCRDs, err := test.GetCRDDirFromModule("github.com/openshift/api", gomod, "config/v1/zz_generated.crd-manifests/0000_10_config-operator_01_imagedigestmirrorsets.crd.yaml")
	Expect(err).ShouldNot(HaveOccurred())
	machineConfigCRDs, err := test.GetCRDDirFromModule("github.com/openshift/api", gomod, "machineconfiguration/v1/zz_generated.crd-manifests/0000_80_machine-config_01_machineconfigs.crd.yaml")
	Expect(err).ShouldNot(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
			baremetalCRDs,
			infraCRDs,
			certmgrv1CRDs,
			openstackCRDs,
			imageContentSourcePolicyCRDs,
			imageDigestMirrorSetCRDs,
			machineConfigCRDs,
		},
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook")},
			// NOTE(gibi): if localhost is resolved to ::1 (ipv6) then starting
			// the webhook fails as it try to parse the address as ipv4 and
			// failing on the colons in ::1
			LocalServingHost: "127.0.0.1",
		},
		ErrorIfCRDPathMissing:    true,
		ControlPlaneStartTimeout: 2 * time.Minute,
		ControlPlaneStopTimeout:  2 * time.Minute,
		CRDInstallOptions: envtest.CRDInstallOptions{
			MaxTime: 5 * time.Minute,
		},
		ControlPlane: envtest.ControlPlane{
			APIServer: &envtest.APIServer{
				Args: []string{
					"--request-timeout=5m",
					"--max-requests-inflight=800",
					"--max-mutating-requests-inflight=400",
				},
			},
			Etcd: &envtest.Etcd{
				Args: []string{
					"--quota-backend-bytes=8589934592", // 8GB
				},
			},
		},
	}

	// cfg is defined in this file globally.
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	// NOTE(gibi): Need to add all API schemas our operator can own.
	// Keep this in synch with SetupWithManager, otherwise the reconciler loop
	// will silently not start in the test env.
	err = dataplanev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = batchv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = appsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = baremetalv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = infrav1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = rabbitmqv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = openstackv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = certmgrv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = machineconfig.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = ocp_image.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	logger = ctrl.Log.WithName("---DataPlane Test---")

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
	th = NewTestHelper(ctx, k8sClient, timeout, interval, logger)
	Expect(th).NotTo(BeNil())

	// Start the controller-manager if goroutine
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		WebhookServer: webhook.NewServer(
			webhook.Options{
				Host:    webhookInstallOptions.LocalServingHost,
				Port:    webhookInstallOptions.LocalServingPort,
				CertDir: webhookInstallOptions.LocalServingCertDir,
			}),
		LeaderElection: false,
	})
	Expect(err).ToNot(HaveOccurred())

	err = dataplanewebhook.SetupOpenStackDataPlaneNodeSetWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&dataplanev1.OpenStackDataPlaneDeployment{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = (&dataplanev1.OpenStackDataPlaneService{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = corewebhook.SetupOpenStackVersionWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	err = corewebhook.SetupOpenStackControlPlaneWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	kclient, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred(), "failed to create kclient")
	err = (&dataplanecontrollers.OpenStackDataPlaneNodeSetReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(ctx, k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&dataplanecontrollers.OpenStackDataPlaneDeploymentReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()
})

var _ = BeforeEach(func() {
	// NOTE(gibi): We need to create a unique namespace for each test run
	// as namespaces cannot be deleted in a locally running envtest. See
	// https://book.kubebuilder.io/reference/envtest.html#namespace-usage-limitation
	namespace = uuid.New().String()
	th.CreateNamespace(namespace)
	// We still request the delete of the Namespace to properly cleanup if
	// we run the test in an existing cluster.
	DeferCleanup(th.DeleteNamespace, namespace)
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})
