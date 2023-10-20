package functional_test

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/uuid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap/zapcore"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	routev1 "github.com/openshift/api/route/v1"
	rabbitmqv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	openstackclientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	client_ctrl "github.com/openstack-k8s-operators/openstack-operator/controllers/client"
	core_ctrl "github.com/openstack-k8s-operators/openstack-operator/controllers/core"

	infra_test "github.com/openstack-k8s-operators/infra-operator/apis/test/helpers"
	keystone_test "github.com/openstack-k8s-operators/keystone-operator/api/test/helpers"
	certmanager_test "github.com/openstack-k8s-operators/lib-common/modules/certmanager/test/helpers"
	common_test "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	test "github.com/openstack-k8s-operators/lib-common/modules/test"
	mariadb_test "github.com/openstack-k8s-operators/mariadb-operator/api/test/helpers"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	cfg       *rest.Config
	k8sClient client.Client
	testEnv   *envtest.Environment
	ctx       context.Context
	cancel    context.CancelFunc
	logger    logr.Logger
	th        *common_test.TestHelper
	keystone  *keystone_test.TestHelper
	mariadb   *mariadb_test.TestHelper
	infra     *infra_test.TestHelper
	crtmgr    *certmanager_test.TestHelper
	namespace string
	names     Names
)

const (
	timeout = time.Second * 5

	SecretName = "test-osp-secret"

	interval = time.Millisecond * 200
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
		o.Development = true
		o.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))

	ctx, cancel = context.WithCancel(context.TODO())

	routev1CRDs, err := test.GetOpenShiftCRDDir("route/v1", "../../go.mod")
	Expect(err).ShouldNot(HaveOccurred())
	mariaDBCRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/mariadb-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	infraCRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/infra-operator/apis", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	cinderv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/cinder-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	glancev1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/glance-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	heatv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/heat-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	horizonv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/horizon-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	ironicv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/ironic-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	keystonev1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/keystone-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	manilav1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/manila-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	neutronv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/neutron-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	novav1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/nova-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	octaviav1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/octavia-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	ovnv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/ovn-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	placementv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/placement-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	swiftv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/swift-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	telemetryv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/telemetry-operator/api", "../../go.mod", "bases")
	Expect(err).ShouldNot(HaveOccurred())
	rabbitmqv2CRDs, err := test.GetCRDDirFromModule(
		"github.com/rabbitmq/cluster-operator/v2", "../../go.mod", "config/crd/bases")
	Expect(err).ShouldNot(HaveOccurred())
	certmgrv1CRDs, err := test.GetOpenShiftCRDDir("cert-manager/v1", "../../go.mod")
	Expect(err).ShouldNot(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "config", "crd", "bases"),
			routev1CRDs,
			mariaDBCRDs,
			infraCRDs,
			cinderv1CRDs,
			glancev1CRDs,
			heatv1CRDs,
			horizonv1CRDs,
			ironicv1CRDs,
			keystonev1CRDs,
			manilav1CRDs,
			neutronv1CRDs,
			novav1CRDs,
			octaviav1CRDs,
			ovnv1CRDs,
			placementv1CRDs,
			swiftv1CRDs,
			telemetryv1CRDs,
			rabbitmqv2CRDs,
			certmgrv1CRDs,
		},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "config", "webhook")},
			// NOTE(gibi): if localhost is resolved to ::1 (ipv6) then starting
			// the webhook fails as it try to parse the address as ipv4 and
			// failing on the colons in ::1
			LocalServingHost: "127.0.0.1",
		},
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	err = openstackclientv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = routev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = cinderv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = glancev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = heatv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = horizonv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = memcachedv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = redisv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = ironicv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = keystonev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = manilav1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = mariadbv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = neutronv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = novav1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = octaviav1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = ovnv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = placementv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = swiftv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = telemetryv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = rabbitmqv2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = certmgrv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = networkv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	logger = ctrl.Log.WithName("---Test---")

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
	th = common_test.NewTestHelper(ctx, k8sClient, timeout, interval, logger)
	Expect(th).NotTo(BeNil())
	keystone = keystone_test.NewTestHelper(ctx, k8sClient, timeout, interval, logger)
	Expect(keystone).NotTo(BeNil())
	mariadb = mariadb_test.NewTestHelper(ctx, k8sClient, timeout, interval, logger)
	Expect(mariadb).NotTo(BeNil())
	infra = infra_test.NewTestHelper(ctx, k8sClient, timeout, interval, logger)
	Expect(infra).NotTo(BeNil())
	crtmgr = certmanager_test.NewTestHelper(ctx, k8sClient, timeout, interval, logger)
	Expect(crtmgr).NotTo(BeNil())

	// Start the controller-manager if goroutine
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		// NOTE(gibi): disable metrics reporting in test to allow
		// parallel test execution. Otherwise each instance would like to
		// bind to the same port
		MetricsBindAddress: "0",
		Host:               webhookInstallOptions.LocalServingHost,
		Port:               webhookInstallOptions.LocalServingPort,
		CertDir:            webhookInstallOptions.LocalServingCertDir,
		LeaderElection:     false,
	})
	Expect(err).ToNot(HaveOccurred())

	kclient, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred(), "failed to create kclient")

	err = (&openstackclientv1.OpenStackClient{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())
	err = (&corev1.OpenStackControlPlane{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	openstackclientv1.SetupDefaults()
	corev1.SetupDefaults()

	err = (&client_ctrl.OpenStackClientReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackClient"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&core_ctrl.OpenStackControlPlaneReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenStackControlPlane"),
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred(), "failed to run manager")
	}()

	// wait for the webhook server to get ready
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	addrPort := fmt.Sprintf("%s:%d", webhookInstallOptions.LocalServingHost, webhookInstallOptions.LocalServingPort)
	Eventually(func() error {
		conn, err := tls.DialWithDialer(dialer, "tcp", addrPort, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return err
		}
		conn.Close()
		return nil
	}).Should(Succeed())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
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

	openstackControlplaneName := types.NamespacedName{
		Namespace: namespace,
		Name:      uuid.New().String()[:25],
	}

	names = CreateNames(openstackControlplaneName)
})
