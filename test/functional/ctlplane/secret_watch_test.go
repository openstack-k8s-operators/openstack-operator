package functional_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2" //revive:disable:dot-imports
	. "github.com/onsi/gomega"    //revive:disable:dot-imports

	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	corev1 "github.com/openstack-k8s-operators/openstack-operator/api/core/v1beta1"
	core_ctrl "github.com/openstack-k8s-operators/openstack-operator/internal/controller/core"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// reconcileRecorder wraps the client of an OpenStackControlPlaneReconciler. It
// records every OpenStackControlPlane reconcile request and makes the reconcile
// end right away (NotFound -> nothing to do). This allows to assert what the
// watches registered by SetupWithManager enqueue, without the periodic
// requeues of a real reconcile.
type reconcileRecorder struct {
	client.Client
	mu    sync.Mutex
	count int
	last  time.Time
}

func (r *reconcileRecorder) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*corev1.OpenStackControlPlane); ok {
		r.mu.Lock()
		r.count++
		r.last = time.Now()
		r.mu.Unlock()
		return k8s_errors.NewNotFound(schema.GroupResource{Group: corev1.GroupVersion.Group, Resource: "openstackcontrolplanes"}, key.Name)
	}
	return r.Client.Get(ctx, key, obj, opts...)
}

func (r *reconcileRecorder) stats() (int, time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.count, r.last
}

// startReconcileRecorder starts a second manager, limited to the test
// namespace, running the real OpenStackControlPlane controller setup with a
// recording no-op reconcile.
func startReconcileRecorder() *reconcileRecorder {
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:  scheme.Scheme,
		Metrics: metricsserver.Options{BindAddress: "0"},
		Cache:   cache.Options{DefaultNamespaces: map[string]cache.Config{namespace: {}}},
		// the real controller of the suite manager has the same name
		Controller: config.Controller{SkipNameValidation: ptr.To(true)},
	})
	Expect(err).ToNot(HaveOccurred())

	rec := &reconcileRecorder{Client: mgr.GetClient()}
	Expect((&core_ctrl.OpenStackControlPlaneReconciler{
		Client: rec,
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr)).To(Succeed())

	mgrCtx, mgrCancel := context.WithCancel(ctx)
	DeferCleanup(mgrCancel)
	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(mgrCtx)).To(Succeed())
	}()
	Expect(mgr.GetCache().WaitForCacheSync(mgrCtx)).To(BeTrue())

	return rec
}
