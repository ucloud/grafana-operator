package grafana

import (
	"context"
	"fmt"

	routev1 "github.com/openshift/api/route/v1"
	v12 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	v1beta12 "k8s.io/api/extensions/v1beta1"
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

	grafanav1alpha1 "github.com/ucloud/grafana-operator/v3/pkg/apis/monitor/v1alpha1"
	"github.com/ucloud/grafana-operator/v3/pkg/controller/common"
	"github.com/ucloud/grafana-operator/v3/pkg/controller/config"
)

const ControllerName = "grafana-controller"
const DefaultClientTimeoutSeconds = 5

var log = logf.Log.WithName(ControllerName)

// Add creates a new Grafana Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, autodetectChannel chan schema.GroupVersionKind) error {
	return add(mgr, newReconciler(mgr), autodetectChannel)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	return &ReconcileGrafana{
		client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		plugins:  newPluginsHelper(),
		context:  ctx,
		cancel:   cancel,
		config:   config.GetControllerConfig(),
		recorder: mgr.GetEventRecorderFor(ControllerName),
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler, autodetectChannel chan schema.GroupVersionKind) error {
	// Create a new controller
	c, err := controller.New("grafana-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Grafana
	err = c.Watch(&source.Kind{Type: &grafanav1alpha1.Grafana{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	if err = watchSecondaryResource(c, &v12.Deployment{}); err != nil {
		return err
	}

	if err = watchSecondaryResource(c, &v1beta12.Ingress{}); err != nil {
		return err
	}

	if err = watchSecondaryResource(c, &v1.ConfigMap{}); err != nil {
		return err
	}

	if err = watchSecondaryResource(c, &v1.Service{}); err != nil {
		return err
	}

	if err = watchSecondaryResource(c, &v1.ServiceAccount{}); err != nil {
		return err
	}

	go func() {
		for gvk := range autodetectChannel {
			cfg := config.GetControllerConfig()

			// Route already watched?
			if cfg.GetConfigBool(config.ConfigRouteWatch, false) == true {
				return
			}

			// Watch routes if they exist on the cluster
			if gvk.String() == routev1.SchemeGroupVersion.WithKind(common.RouteKind).String() {
				if err = watchSecondaryResource(c, &routev1.Route{}); err != nil {
					log.Error(err, fmt.Sprintf("error adding secondary watch for %v", common.RouteKind))
				} else {
					cfg.AddConfigItem(config.ConfigRouteWatch, true)
					log.Info(fmt.Sprintf("added secondary watch for %v", common.RouteKind))
				}
			}
		}
	}()

	return nil
}

var _ reconcile.Reconciler = &ReconcileGrafana{}

// ReconcileGrafana reconciles a Grafana object
type ReconcileGrafana struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client   client.Client
	scheme   *runtime.Scheme
	plugins  *PluginsHelperImpl
	context  context.Context
	cancel   context.CancelFunc
	config   *config.ControllerConfig
	recorder record.EventRecorder
}

func watchSecondaryResource(c controller.Controller, resource runtime.Object) error {
	return c.Watch(&source.Kind{Type: resource}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &grafanav1alpha1.Grafana{},
	})
}

// Reconcile reads that state of the cluster for a Grafana object and makes changes based on the state read
// and what is in the Grafana.Spec
func (r *ReconcileGrafana) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	instance := &grafanav1alpha1.Grafana{}
	err := r.client.Get(r.context, request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Stop the dashboard controller from reconciling when grafana is not installed
			r.config.RemoveConfigItem(config.ConfigDashboardLabelSelector)
			r.config.Cleanup(true)

			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	cr := instance.DeepCopy()

	// Read current state
	currentState := common.NewClusterState()
	err = currentState.Read(r.context, cr, r.client)
	if err != nil {
		log.Error(err, "error reading state")
		return r.manageError(cr, err)
	}

	// Get the actions required to reach the desired state
	reconciler := NewGrafanaReconciler()
	desiredState := reconciler.Reconcile(currentState, cr)

	// Run the actions to reach the desired state
	actionRunner := common.NewClusterActionRunner(r.context, r.client, r.scheme, cr)
	err = actionRunner.RunAll(desiredState)
	if err != nil {
		return r.manageError(cr, err)
	}

	// Run the config map reconciler to discover jsonnet libraries
	err = reconcileConfigMaps(cr, r)
	if err != nil {
		return r.manageError(cr, err)
	}

	return r.manageSuccess(cr, currentState)
}

func (r *ReconcileGrafana) manageError(cr *grafanav1alpha1.Grafana, issue error) (reconcile.Result, error) {
	r.recorder.Event(cr, "Warning", "ProcessingError", issue.Error())
	cr.Status.Phase = grafanav1alpha1.PhaseFailing
	cr.Status.Message = issue.Error()

	err := r.client.Status().Update(r.context, cr)
	if err != nil {
		// Ignore conflicts, resource might just be outdated.
		if errors.IsConflict(err) {
			err = nil
		}
		return reconcile.Result{}, err
	}

	return reconcile.Result{RequeueAfter: config.RequeueDelay}, nil
}

func (r *ReconcileGrafana) manageSuccess(cr *grafanav1alpha1.Grafana, state *common.ClusterState) (reconcile.Result, error) {
	cr.Status.Phase = grafanav1alpha1.PhaseReconciling
	cr.Status.Message = "success"

	r.updateStatus(cr)

	err := r.client.Status().Update(r.context, cr)
	if err != nil {
		return r.manageError(cr, err)
	}

	log.Info("desired cluster state met")

	return reconcile.Result{RequeueAfter: config.RequeueDelay}, nil
}

func (r *ReconcileGrafana) updateStatus(cr *grafanav1alpha1.Grafana) error {
	var installedDashboards []*grafanav1alpha1.GrafanaDashboardRef
	dashboards := &grafanav1alpha1.GrafanaDashboardList{}
	r.client.List(r.context, dashboards, client.InNamespace(cr.Namespace))
	for _, dashboard := range dashboards.Items {
		if match, err := common.MatchesSelectors(dashboard.Labels, cr.Spec.DashboardLabelSelector); err != nil {
			return err
		} else if match {
			installedDashboards = append(installedDashboards, &grafanav1alpha1.GrafanaDashboardRef{
				Name: dashboard.Name,
			})
		}
	}

	var installedDataSources []*grafanav1alpha1.GrafanaDatasourceRef
	dataSources := &grafanav1alpha1.GrafanaDataSourceList{}
	r.client.List(r.context, dataSources, client.InNamespace(cr.Namespace))
	for _, dataSource := range dataSources.Items {
		if match, err := common.MatchesSelectors(dataSource.Labels, cr.Spec.DatasourceLabelSelector); err != nil {
			return err
		} else if match {
			installedDataSources = append(installedDataSources, &grafanav1alpha1.GrafanaDatasourceRef{
				Name: dataSource.Name,
			})
		}
	}

	cr.Status.InstalledDashboards = installedDashboards
	cr.Status.InstalledDatasources = installedDataSources
	return nil
}
