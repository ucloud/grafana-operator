package grafanadatasource

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
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
)

const (
	ControllerName                 = "controller_grafanadatasource"
	datasourceFinalizer            = "finalizer.grafanadatasources.monitor.kun"
	defaultreconcileTime           = 10 * time.Second
	defaultMaxConcurrentReconciles = 10
)

var log = logf.Log.WithName(ControllerName)

// Add creates a new GrafanaDataSource Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, _ chan schema.GroupVersionKind) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	return &ReconcileGrafanaDataSource{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		context:  ctx,
		cancel:   cancel,
		recorder: mgr.GetEventRecorderFor(ControllerName),
		state:    common.ControllerState{},
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("grafanadatasource-controller", mgr, controller.Options{
		Reconciler:              r,
		MaxConcurrentReconciles: defaultMaxConcurrentReconciles,
	})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource GrafanaDataSource
	err = c.Watch(&source.Kind{Type: &grafanav1alpha1.GrafanaDataSource{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileGrafanaDataSource{}

// ReconcileGrafanaDataSource reconciles a GrafanaDataSource object
type ReconcileGrafanaDataSource struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	context  context.Context
	cancel   context.CancelFunc
	recorder record.EventRecorder
	state    common.ControllerState
}

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileGrafanaDataSource) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling GrafanaDataSource")

	// Fetch the GrafanaDataSource instance
	instance := &grafanav1alpha1.GrafanaDataSource{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Check if the GrafanaDataSource instance is marked to be deleted, which is
	// indicated by the deletion timestamp being set.
	isBackupMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isBackupMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), datasourceFinalizer) {
			// Run finalization logic for datasourceFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalize(reqLogger, instance); err != nil {
				return reconcile.Result{}, err
			}

			// Remove datasourceFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			instance.SetFinalizers(remove(instance.GetFinalizers(), datasourceFinalizer))
			err := r.client.Update(context.TODO(), instance)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), datasourceFinalizer) {
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

// Handle error case: update datasource with error message and status
func (r *ReconcileGrafanaDataSource) manageError(datasource *grafanav1alpha1.GrafanaDataSource, issue error) {
	r.recorder.Event(datasource, "Warning", "ProcessingError", issue.Error())

	// datasource deleted
	if datasource == nil {
		return
	}

	datasource.Status.Phase = grafanav1alpha1.PhaseFailing
	datasource.Status.Message = issue.Error()

	err := r.client.Status().Update(r.context, datasource)
	if err != nil {
		// Ignore conclicts. Resource might just be outdated.
		if errors.IsConflict(err) {
			return
		}
		log.Error(err, "error updating datasource status")
	}
}

// manage success case: datasource has been imported successfully and the configmap
// is updated
func (r *ReconcileGrafanaDataSource) manageSuccess(datasource *grafanav1alpha1.GrafanaDataSource) {
	log.Info(fmt.Sprintf("datasource %v/%v successfully imported",
		datasource.Namespace,
		datasource.Name))

	datasource.Status.Phase = grafanav1alpha1.PhaseReconciling
	datasource.Status.Message = "success"

	err := r.client.Status().Update(r.context, datasource)
	if err != nil {
		log.Error(err, "error updating datasource status")
		r.recorder.Event(datasource, "Warning", "UpdateError", err.Error())
	}
}
