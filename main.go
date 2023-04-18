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

package main

import (
	"flag"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	dataplanev1beta1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1alpha1"
	clientv1 "github.com/openstack-k8s-operators/infra-operator/apis/client/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	ansibleeev1 "github.com/openstack-k8s-operators/openstack-ansibleee-operator/api/v1alpha1"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	ovsv1 "github.com/openstack-k8s-operators/ovs-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	rabbitmqclusterv1 "github.com/rabbitmq/cluster-operator/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	corev1beta1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"
	corecontrollers "github.com/openstack-k8s-operators/openstack-operator/controllers/core"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1beta1.AddToScheme(scheme))
	utilruntime.Must(keystonev1.AddToScheme(scheme))
	utilruntime.Must(mariadbv1.AddToScheme(scheme))
	utilruntime.Must(memcachedv1.AddToScheme(scheme))
	utilruntime.Must(rabbitmqclusterv1.AddToScheme(scheme))
	utilruntime.Must(placementv1.AddToScheme(scheme))
	utilruntime.Must(glancev1.AddToScheme(scheme))
	utilruntime.Must(cinderv1.AddToScheme(scheme))
	utilruntime.Must(novav1.AddToScheme(scheme))
	utilruntime.Must(baremetalv1.AddToScheme(scheme))
	utilruntime.Must(ironicv1.AddToScheme(scheme))
	utilruntime.Must(ovnv1.AddToScheme(scheme))
	utilruntime.Must(ovsv1.AddToScheme(scheme))
	utilruntime.Must(neutronv1.AddToScheme(scheme))
	utilruntime.Must(dataplanev1beta1.AddToScheme(scheme))
	utilruntime.Must(ansibleeev1.AddToScheme(scheme))
	utilruntime.Must(rabbitmqv1.AddToScheme(scheme))
	utilruntime.Must(clientv1.AddToScheme(scheme))
	utilruntime.Must(manilav1.AddToScheme(scheme))
	utilruntime.Must(horizonv1.AddToScheme(scheme))
	utilruntime.Must(telemetryv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	devMode, err := strconv.ParseBool(os.Getenv("DEV_MODE"))
	if err != nil {
		devMode = false
	}
	opts := zap.Options{
		Development: devMode,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "40ba705e.openstack.org",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	cfg, err := config.GetConfig()
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}
	kclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		setupLog.Error(err, "")
		os.Exit(1)
	}

	if err = (&corecontrollers.OpenStackControlPlaneReconciler{
		Client:                        mgr.GetClient(),
		Scheme:                        mgr.GetScheme(),
		Kclient:                       kclient,
		Log:                           ctrl.Log.WithName("controllers").WithName("OpenStackControlPlane"),
		OpenStackClientContainerImage: os.Getenv("OPENSTACKCLIENT_IMAGE_URL_DEFAULT"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackControlPlane")
		os.Exit(1)
	}

	// Defaults for service operators
	setupServiceOperatorDefaults()

	// Webhooks
	if strings.ToLower(os.Getenv("ENABLE_WEBHOOKS")) != "false" {
		if err = (&corev1beta1.OpenStackControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackControlPlane")
			os.Exit(1)
		}
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

// Set up any defaults used by service operator defaulting logic
func setupServiceOperatorDefaults() {
	// Acquire environmental defaults and initialize service operators that
	// require each respective default

	// Cinder
	cinderDefaults := cinderv1.CinderDefaults{
		APIContainerImageURL:       os.Getenv("CINDER_API_IMAGE_URL_DEFAULT"),
		BackupContainerImageURL:    os.Getenv("CINDER_BACKUP_IMAGE_URL_DEFAULT"),
		SchedulerContainerImageURL: os.Getenv("CINDER_SCHEDULER_IMAGE_URL_DEFAULT"),
		VolumeContainerImageURL:    os.Getenv("CINDER_VOLUME_IMAGE_URL_DEFAULT"),
	}

	cinderv1.SetupCinderDefaults(cinderDefaults)

	// Glance
	glanceDefaults := glancev1.GlanceDefaults{
		ContainerImageURL: os.Getenv("GLANCE_API_IMAGE_URL_DEFAULT"),
	}

	glancev1.SetupGlanceDefaults(glanceDefaults)

	// Ironic
	ironicDefaults := ironicv1.IronicDefaults{
		APIContainerImageURL:       os.Getenv("IRONIC_API_IMAGE_URL_DEFAULT"),
		ConductorContainerImageURL: os.Getenv("IRONIC_CONDUCTOR_IMAGE_URL_DEFAULT"),
		InspectorContainerImageURL: os.Getenv("IRONIC_INSPECTOR_IMAGE_URL_DEFAULT"),
		PXEContainerImageURL:       os.Getenv("IRONIC_PXE_IMAGE_URL_DEFAULT"),
		INAContainerImageURL:       os.Getenv("IRONIC_NEUTRON_AGENT_IMAGE_URL_DEFAULT"),
	}

	ironicv1.SetupIronicDefaults(ironicDefaults)

	// Keystone
	keystoneDefaults := keystonev1.KeystoneAPIDefaults{
		ContainerImageURL: os.Getenv("KEYSTONE_API_IMAGE_URL_DEFAULT"),
	}

	keystonev1.SetupKeystoneAPIDefaults(keystoneDefaults)

	// Manila
	manilaDefaults := manilav1.ManilaDefaults{
		APIContainerImageURL:       os.Getenv("MANILA_API_IMAGE_URL_DEFAULT"),
		SchedulerContainerImageURL: os.Getenv("MANILA_SCHEDULER_IMAGE_URL_DEFAULT"),
		ShareContainerImageURL:     os.Getenv("MANILA_SHARE_IMAGE_URL_DEFAULT"),
	}

	manilav1.SetupManilaDefaults(manilaDefaults)

	// MariaDB
	mariadbDefaults := mariadbv1.MariaDBDefaults{
		ContainerImageURL: os.Getenv("MARIADB_IMAGE_URL_DEFAULT"),
	}

	mariadbv1.SetupMariaDBDefaults(mariadbDefaults)

	// Memcached
	memcachedDefaults := memcachedv1.MemcachedDefaults{
		ContainerImageURL: os.Getenv("INFRA_MEMCACHED_IMAGE_URL_DEFAULT"),
	}

	memcachedv1.SetupMemcachedDefaults(memcachedDefaults)

	// Neutron
	neutronAPIDefaults := neutronv1.NeutronAPIDefaults{
		ContainerImageURL: os.Getenv("NEUTRON_API_IMAGE_URL_DEFAULT"),
	}

	neutronv1.SetupNeutronAPIDefaults(neutronAPIDefaults)

	// Nova
	novav1.SetupDefaults()

	// OpenStackClient
	openStackClientDefaults := clientv1.OpenStackClientDefaults{
		ContainerImageURL: os.Getenv("INFRA_CLIENT_IMAGE_URL_DEFAULT"),
	}

	clientv1.SetupOpenStackClientDefaults(openStackClientDefaults)

	// OVN
	ovnDbClusterDefaults := ovnv1.OVNDBClusterDefaults{
		NBContainerImageURL: os.Getenv("OVN_NB_DBCLUSTER_IMAGE_URL_DEFAULT"),
		SBContainerImageURL: os.Getenv("OVN_SB_DBCLUSTER_IMAGE_URL_DEFAULT"),
	}

	ovnv1.SetupOVNDBClusterDefaults(ovnDbClusterDefaults)

	ovnNorthdDefaults := ovnv1.OVNNorthdDefaults{
		ContainerImageURL: os.Getenv("OVN_NORTHD_IMAGE_URL_DEFAULT"),
	}

	ovnv1.SetupOVNNorthdDefaults(ovnNorthdDefaults)

	// OVS
	ovsDefaults := ovsv1.OvsDefaults{
		OvsContainerImageURL: os.Getenv("OVS_IMAGE_URL_DEFAULT"),
		OvnContainerImageURL: os.Getenv("OVN_IMAGE_URL_DEFAULT"),
	}

	ovsv1.SetupOvsDefaults(ovsDefaults)

	// Placement
	placementAPIDefaults := placementv1.PlacementAPIDefaults{
		ContainerImageURL: os.Getenv("PLACEMENT_API_IMAGE_URL_DEFAULT"),
	}

	placementv1.SetupPlacementAPIDefaults(placementAPIDefaults)

	// Redis
	redisDefaults := redisv1.RedisDefaults{
		ContainerImageURL: os.Getenv("INFRA_REDIS_IMAGE_URL_DEFAULT"),
	}

	redisv1.SetupRedisDefaults(redisDefaults)

	// Telemetry
	telemetryDefaults := telemetryv1.TelemetryDefaults{
		CentralContainerImageURL:      os.Getenv("CEILOMETER_CENTRAL_IMAGE_URL_DEFAULT"),
		CentralInitContainerImageURL:  os.Getenv("CEILOMETER_CENTRAL_INIT_IMAGE_URL_DEFAULT"),
		ComputeContainerImageURL:      os.Getenv("CEILOMETER_COMPUTE_IMAGE_URL_DEFAULT"),
		ComputeInitContainerImageURL:  os.Getenv("CEILOMETER_COMPUTE_INIT_IMAGE_URL_DEFAULT"),
		NotificationContainerImageURL: os.Getenv("CEILOMETER_NOTIFICATION_IMAGE_URL_DEFAULT"),
		SgCoreContainerImageURL:       os.Getenv("CEILOMETER_SGCORE_INIT_IMAGE_URL_DEFAULT"),
	}

	telemetryv1.SetupTelemetryDefaults(telemetryDefaults)
}
