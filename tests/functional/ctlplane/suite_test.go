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
	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports
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
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	routev1 "github.com/openshift/api/route/v1"
	rabbitmqv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"

	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
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
	dataplanev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/dataplane/v1beta1"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"

	client_ctrl "github.com/openstack-k8s-operators/openstack-operator/controllers/client"
	core_ctrl "github.com/openstack-k8s-operators/openstack-operator/controllers/core"

	ocp_configv1 "github.com/openshift/api/config/v1"
	infra_test "github.com/openstack-k8s-operators/infra-operator/apis/test/helpers"
	topologyv1 "github.com/openstack-k8s-operators/infra-operator/apis/topology/v1beta1"
	keystone_test "github.com/openstack-k8s-operators/keystone-operator/api/test/helpers"
	certmanager_test "github.com/openstack-k8s-operators/lib-common/modules/certmanager/test/helpers"
	common_test "github.com/openstack-k8s-operators/lib-common/modules/common/test/helpers"
	test "github.com/openstack-k8s-operators/lib-common/modules/test"
	mariadb_test "github.com/openstack-k8s-operators/mariadb-operator/api/test/helpers"
	ovn_test "github.com/openstack-k8s-operators/ovn-operator/api/test/helpers"
	// +kubebuilder:scaffold:imports
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
	ovn       *ovn_test.TestHelper
	namespace string
	names     Names
)

const (
	timeout = time.Second * 10

	SecretName = "test-osp-secret"

	interval = time.Millisecond * 200
)

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "CtlPlane Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true), func(o *zap.Options) {
		o.Development = true
		o.TimeEncoder = zapcore.ISO8601TimeEncoder
	}))

	ctx, cancel = context.WithCancel(context.TODO())
	const gomod = "../../../go.mod"

	routev1CRDs, err := test.GetOpenShiftCRDDir("route/v1", gomod)
	Expect(err).ShouldNot(HaveOccurred())
	mariaDBCRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/mariadb-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	infraCRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/infra-operator/apis", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	cinderv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/cinder-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	glancev1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/glance-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	heatv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/heat-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	horizonv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/horizon-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	ironicv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/ironic-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	keystonev1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/keystone-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	manilav1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/manila-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	neutronv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/neutron-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	novav1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/nova-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	octaviav1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/octavia-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	ovnv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/ovn-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	placementv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/placement-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	swiftv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/swift-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	telemetryv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/telemetry-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	designatev1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/designate-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	barbicanv1CRDs, err := test.GetCRDDirFromModule(
		"github.com/openstack-k8s-operators/barbican-operator/api", gomod, "bases")
	Expect(err).ShouldNot(HaveOccurred())
	rabbitmqv2CRDs, err := test.GetCRDDirFromModule(
		"github.com/rabbitmq/cluster-operator/v2", gomod, "config/crd/bases")
	Expect(err).ShouldNot(HaveOccurred())
	certmgrv1CRDs, err := test.GetOpenShiftCRDDir("cert-manager/v1", gomod)
	Expect(err).ShouldNot(HaveOccurred())
	ocpconfigv1CRDs, err := test.GetOpenShiftCRDDir("config/v1", gomod)
	Expect(err).ShouldNot(HaveOccurred())

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{
			filepath.Join("..", "..", "..", "config", "crd", "bases"),
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
			designatev1CRDs,
			barbicanv1CRDs,
			rabbitmqv2CRDs,
			certmgrv1CRDs,
			ocpconfigv1CRDs,
		},
		ErrorIfCRDPathMissing: true,
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "..", "..", "config", "webhook")},
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
	err = dataplanev1beta1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = designatev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = barbicanv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = rabbitmqv2.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = certmgrv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = networkv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = ocp_configv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = topologyv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	logger = ctrl.Log.WithName("---CtlPlane Test---")

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
	ovn = ovn_test.NewTestHelper(ctx, k8sClient, timeout, interval, logger)
	Expect(ovn).NotTo(BeNil())

	th.CreateClusterNetworkConfig()

	// Start the controller-manager if goroutine
	webhookInstallOptions := &testEnv.WebhookInstallOptions
	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
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

	kclient, err := kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred(), "failed to create kclient")

	err = (&openstackclientv1.OpenStackClient{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())
	err = (&corev1.OpenStackVersion{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())
	err = (&corev1.OpenStackControlPlane{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())
	err = (&dataplanev1beta1.OpenStackDataPlaneNodeSet{}).SetupWebhookWithManager(k8sManager)
	Expect(err).NotTo(HaveOccurred())

	core_ctrl.SetupVersionDefaults()
	openstack.SetupServiceOperatorDefaults()
	openstackclientv1.SetupDefaults()
	corev1.SetupVersionDefaults()

	err = (&client_ctrl.OpenStackClientReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(ctx, k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&core_ctrl.OpenStackVersionReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(k8sManager)
	Expect(err).ToNot(HaveOccurred())

	err = (&core_ctrl.OpenStackControlPlaneReconciler{
		Client:  k8sManager.GetClient(),
		Scheme:  k8sManager.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(ctx, k8sManager)
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
