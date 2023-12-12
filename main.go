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
	"crypto/tls"
	"flag"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap/zapcore"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	certmgrv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	barbicanv1 "github.com/openstack-k8s-operators/barbican-operator/api/v1beta1"
	cinderv1 "github.com/openstack-k8s-operators/cinder-operator/api/v1beta1"
	dataplanev1beta1 "github.com/openstack-k8s-operators/dataplane-operator/api/v1beta1"
	designatev1 "github.com/openstack-k8s-operators/designate-operator/api/v1beta1"
	glancev1 "github.com/openstack-k8s-operators/glance-operator/api/v1beta1"
	heatv1 "github.com/openstack-k8s-operators/heat-operator/api/v1beta1"
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	memcachedv1 "github.com/openstack-k8s-operators/infra-operator/apis/memcached/v1beta1"
	networkv1 "github.com/openstack-k8s-operators/infra-operator/apis/network/v1beta1"
	rabbitmqv1 "github.com/openstack-k8s-operators/infra-operator/apis/rabbitmq/v1beta1"
	redisv1 "github.com/openstack-k8s-operators/infra-operator/apis/redis/v1beta1"
	ironicv1 "github.com/openstack-k8s-operators/ironic-operator/api/v1beta1"
	keystonev1 "github.com/openstack-k8s-operators/keystone-operator/api/v1beta1"
	manilav1 "github.com/openstack-k8s-operators/manila-operator/api/v1beta1"
	mariadbv1 "github.com/openstack-k8s-operators/mariadb-operator/api/v1beta1"
	neutronv1 "github.com/openstack-k8s-operators/neutron-operator/api/v1beta1"
	novav1 "github.com/openstack-k8s-operators/nova-operator/api/v1beta1"
	octaviav1 "github.com/openstack-k8s-operators/octavia-operator/api/v1beta1"
	ansibleeev1 "github.com/openstack-k8s-operators/openstack-ansibleee-operator/api/v1beta1"
	baremetalv1 "github.com/openstack-k8s-operators/openstack-baremetal-operator/api/v1beta1"
	ovnv1 "github.com/openstack-k8s-operators/ovn-operator/api/v1beta1"
	placementv1 "github.com/openstack-k8s-operators/placement-operator/api/v1beta1"
	swiftv1 "github.com/openstack-k8s-operators/swift-operator/api/v1beta1"
	telemetryv1 "github.com/openstack-k8s-operators/telemetry-operator/api/v1beta1"
	rabbitmqclusterv2 "github.com/rabbitmq/cluster-operator/v2/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	routev1 "github.com/openshift/api/route/v1"
	clientv1 "github.com/openstack-k8s-operators/openstack-operator/apis/client/v1beta1"
	corev1 "github.com/openstack-k8s-operators/openstack-operator/apis/core/v1beta1"

	clientcontrollers "github.com/openstack-k8s-operators/openstack-operator/controllers/client"
	corecontrollers "github.com/openstack-k8s-operators/openstack-operator/controllers/core"
	"github.com/openstack-k8s-operators/openstack-operator/pkg/openstack"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(corev1.AddToScheme(scheme))
	utilruntime.Must(keystonev1.AddToScheme(scheme))
	utilruntime.Must(mariadbv1.AddToScheme(scheme))
	utilruntime.Must(memcachedv1.AddToScheme(scheme))
	utilruntime.Must(rabbitmqclusterv2.AddToScheme(scheme))
	utilruntime.Must(placementv1.AddToScheme(scheme))
	utilruntime.Must(glancev1.AddToScheme(scheme))
	utilruntime.Must(cinderv1.AddToScheme(scheme))
	utilruntime.Must(novav1.AddToScheme(scheme))
	utilruntime.Must(baremetalv1.AddToScheme(scheme))
	utilruntime.Must(heatv1.AddToScheme(scheme))
	utilruntime.Must(ironicv1.AddToScheme(scheme))
	utilruntime.Must(ovnv1.AddToScheme(scheme))
	utilruntime.Must(neutronv1.AddToScheme(scheme))
	utilruntime.Must(octaviav1.AddToScheme(scheme))
	utilruntime.Must(designatev1.AddToScheme(scheme))
	utilruntime.Must(dataplanev1beta1.AddToScheme(scheme))
	utilruntime.Must(ansibleeev1.AddToScheme(scheme))
	utilruntime.Must(rabbitmqv1.AddToScheme(scheme))
	utilruntime.Must(manilav1.AddToScheme(scheme))
	utilruntime.Must(horizonv1.AddToScheme(scheme))
	utilruntime.Must(networkv1.AddToScheme(scheme))
	utilruntime.Must(telemetryv1.AddToScheme(scheme))
	utilruntime.Must(swiftv1.AddToScheme(scheme))
	utilruntime.Must(clientv1.AddToScheme(scheme))
	utilruntime.Must(redisv1.AddToScheme(scheme))
	utilruntime.Must(routev1.AddToScheme(scheme))
	utilruntime.Must(certmgrv1.AddToScheme(scheme))
	utilruntime.Must(barbicanv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enableHTTP2 bool
	flag.BoolVar(&enableHTTP2, "enable-http2", enableHTTP2, "If HTTP/2 should be enabled for the metrics and webhook servers.")
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	devMode, err := strconv.ParseBool(os.Getenv("DEV_MODE"))
	if err != nil {
		devMode = true
	}
	opts := zap.Options{
		Development: devMode,
		TimeEncoder: zapcore.ISO8601TimeEncoder,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	disableHTTP2 := func(c *tls.Config) {
		if enableHTTP2 {
			return
		}
		c.NextProtos = []string{"http/1.1"}
	}

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
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackControlPlane")
		os.Exit(1)
	}

	if err = (&clientcontrollers.OpenStackClientReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Kclient: kclient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackClient")
		os.Exit(1)
	}

	if err = (&corecontrollers.OpenStackVersionReconciler{
		Client:  mgr.GetClient(),
		Scheme:  mgr.GetScheme(),
		Kclient: kclient,
		Log:     ctrl.Log.WithName("controllers").WithName("OpenVersion"),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "OpenStackVersion")
		os.Exit(1)
	}
	corecontrollers.SetupVersionDefaults()

	// Defaults for service operators
	openstack.SetupServiceOperatorDefaults()

	// Defaults for OpenStackClient
	clientv1.SetupDefaults()

	// Defaults for anything else that was not covered by OpenStackClient nor service operator defaults
	corev1.SetupDefaults()

	// Webhooks
	checker := healthz.Ping
	if strings.ToLower(os.Getenv("ENABLE_WEBHOOKS")) != "false" {
		// overriding the default values
		srv := mgr.GetWebhookServer()
		srv.TLSOpts = []func(config *tls.Config){disableHTTP2}

		if err = (&corev1.OpenStackControlPlane{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackControlPlane")
			os.Exit(1)
		}
		if err = (&clientv1.OpenStackClient{}).SetupWebhookWithManager(mgr); err != nil {
			setupLog.Error(err, "unable to create webhook", "webhook", "OpenStackClient")
			os.Exit(1)
		}
		checker = mgr.GetWebhookServer().StartedChecker()
	}

	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", checker); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", checker); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
