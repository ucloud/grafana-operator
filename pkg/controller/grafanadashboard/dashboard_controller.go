package grafanadashboard

import (
	"context"
	"fmt"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"

	grafanav1alpha1 "github.com/ucloud/grafana-operator/pkg/apis/monitor/v1alpha1"
	"github.com/ucloud/grafana-operator/pkg/controller/common"
	"github.com/ucloud/grafana-operator/pkg/controller/config"
)

const (
	ControllerName                 = "controller_grafanadashboard"
	dashboardFinalizer             = "finalizer.grafanadashboards.monitor.kun"
	defaultreconcileTime           = 10 * time.Second
	defaultMaxConcurrentReconciles = 10
)

var log = logf.Log.WithName(ControllerName)

// Add creates a new GrafanaDashboard Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, _ chan schema.GroupVersionKind) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	return &ReconcileGrafanaDashboard{
		client:   mgr.GetClient(),
		config:   config.GetControllerConfig(),
		context:  ctx,
		cancel:   cancel,
		recorder: mgr.GetEventRecorderFor(ControllerName),
		state:    common.ControllerState{},
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("grafanadashboard-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource GrafanaDashboard
	err = c.Watch(&source.Kind{Type: &grafanav1alpha1.GrafanaDashboard{}}, &handler.EnqueueRequestForObject{})
	if err == nil {
		log.Info("Starting dashboard controller")
	}

	return err
}

var _ reconcile.Reconciler = &ReconcileGrafanaDashboard{}

// ReconcileGrafanaDashboard reconciles a GrafanaDashboard object
type ReconcileGrafanaDashboard struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	config   *config.ControllerConfig
	context  context.Context
	cancel   context.CancelFunc
	recorder record.EventRecorder
	state    common.ControllerState
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileGrafanaDashboard) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling GrafanaDashboard")

	// Fetch the GrafanaDashboard instance
	instance := &grafanav1alpha1.GrafanaDashboard{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Check if the GrafanaDashboard instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isBackupMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isBackupMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), dashboardFinalizer) {
			// Run finalization logic for dashboardFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalize(reqLogger, instance); err != nil {
				return reconcile.Result{}, err
			}

			// Remove dashboardFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			instance.SetFinalizers(remove(instance.GetFinalizers(), dashboardFinalizer))
			err := r.client.Update(context.TODO(), instance)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), dashboardFinalizer) {
		if err := r.addFinalizer(reqLogger, instance); err != nil {
			r.manageError(instance, err)
			return reconcile.Result{}, err
		}
	}

	// Reconcile all data sources
	if err := r.reconcile(reqLogger, instance); err != nil {
		r.manageError(instance, err)
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: defaultreconcileTime}, nil
}

// check if the labels on a namespace match a given label selector
func (r *ReconcileGrafanaDashboard) checkNamespaceLabels(dashboard *grafanav1alpha1.GrafanaDashboard) (bool, error) {
	key := client.ObjectKey{
		Name: dashboard.Namespace,
	}
	ns := &v1.Namespace{}
	err := r.client.Get(r.context, key, ns)
	if err != nil {
		return false, err
	}
	selector, err := metav1.LabelSelectorAsSelector(r.state.DashboardNamespaceSelector)
	if err != nil {
		return false, err
	}
	return selector.Empty() || selector.Matches(labels.Set(ns.Labels)), nil
}

// Handle success case: update dashboard metadata (id, uid) and update the list
// of plugins
func (r *ReconcileGrafanaDashboard) manageSuccess(dashboard *grafanav1alpha1.GrafanaDashboard) {
	msg := fmt.Sprintf("dashboard %v/%v successfully submitted",
		dashboard.Namespace,
		dashboard.Name)
	r.recorder.Event(dashboard, "Normal", "Success", msg)
	log.Info(msg)
	r.config.AddDashboard(dashboard)
	r.config.SetPluginsFor(dashboard)
}

// Handle error case: update dashboard with error message and status
func (r *ReconcileGrafanaDashboard) manageError(dashboard *grafanav1alpha1.GrafanaDashboard, issue error) {
	r.recorder.Event(dashboard, "Warning", "ProcessingError", issue.Error())

	// Ignore conclicts. Resource might just be outdated.
	if errors.IsConflict(issue) {
		return
	}
	log.Error(issue, "error updating dashboard")
}
