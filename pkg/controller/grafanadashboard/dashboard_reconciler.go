package grafanadashboard

import (
	"github.com/go-logr/logr"
	grafanaClient "github.com/ucloud/grafana-operator/v3/pkg/controller/grafanaclient"

	grafanav1alpha1 "github.com/ucloud/grafana-operator/v3/pkg/apis/monitor/v1alpha1"
	"github.com/ucloud/grafana-operator/v3/pkg/controller/common"
)

func (r *ReconcileGrafanaDashboard) reconcile(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDashboard) error {
	matchedGrafs, err := common.MatchGrafana(r.context, r.client, reqLogger, cr.Namespace, cr.Labels)
	if err != nil {
		reqLogger.Error(err, "matchGrafana failed.")
		return err
	}

	for _, graf := range matchedGrafs {
		reqLogger.V(3).Info("reconcile dashboard for grafana", "grafanaName", graf.Name)
		// Read current state
		state := common.NewClusterState()
		err = state.Read(r.context, graf, r.client)
		if err != nil {
			reqLogger.Error(err, "error reading state")
			continue
		}

		client, err := common.NewGrafanaClient(graf, state)
		if err != nil {
			reqLogger.Error(err, "newGrafanaClient failed")
			continue
		}

		dashboards, err := client.GetDashboardsByName(cr.DashboardName())
		if err != nil {
			reqLogger.Error(err, "cannot get dashboard")
			r.manageError(cr, err)
			continue
		}

		if len(dashboards) > 0 {
			continue
		}

		pipeline := NewDashboardPipeline(r.client, cr)
		processed, err := pipeline.ProcessDashboard()

		if err != nil {
			reqLogger.Error(err, "cannot process dashboard")
			r.manageError(cr, err)
			continue
		}

		if processed == nil {
			continue
		}

		folder, err := client.GetOrCreateNamespaceFolder(cr.Namespace)
		if err != nil {
			reqLogger.Error(err, "failed to get or create namespace folder")
			r.manageError(cr, err)
			continue
		}

		var folderID int64 = 0
		if folder.ID != nil {
			folderID = *folder.ID
		}

		_, err = client.CreateOrUpdateDashboard(processed, folderID)
		if err != nil {
			log.Error(err, "cannot submit dashboard")
			r.manageError(cr, err)
			continue
		}
		r.manageSuccess(cr)
	}

	return nil
}

func (r *ReconcileGrafanaDashboard) reconcileDelete(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDashboard) error {
	matchedGrafs, err := common.MatchGrafana(r.context, r.client, reqLogger, cr.Namespace, cr.Labels)
	if err != nil {
		reqLogger.Error(err, "matchGrafana failed.")
		return err
	}

	for _, graf := range matchedGrafs {
		reqLogger.V(3).Info("delete dashboard from grafana", "grafanaName", graf.Name)
		// Read current state
		state := common.NewClusterState()
		err = state.Read(r.context, graf, r.client)
		if err != nil {
			reqLogger.Error(err, "error reading state")
			continue
		}

		client, err := common.NewGrafanaClient(graf, state)
		if err != nil {
			reqLogger.Error(err, "newGrafanaClient failed")
			continue
		}

		dashboards, err := client.GetDashboardsByName(cr.DashboardName())
		if err != nil {
			reqLogger.Error(err, "cannot get dashboard")
			r.manageError(cr, err)
			continue
		}

		if len(dashboards) == 0 {
			continue
		}

		if len(dashboards) > 1 {
			reqLogger.Info("GetDashboardsByName", "dashboardNum", len(dashboards))
		}

		if _, err = client.DeleteDashboardByUID(*dashboards[0].UID); err != nil {
			if err == grafanaClient.NotFoundError {
				reqLogger.Info("dashboard already be deleted", "grafana", graf.Name)
			} else {
				reqLogger.Error(err, "cannot delete dashboard", "grafana", graf.Name)
			}
			continue
		}
	}
	return nil
}

//finalize needs to do before the CR can be deleted.
func (r *ReconcileGrafanaDashboard) finalize(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDashboard) error {
	if err := r.reconcileDelete(reqLogger, cr); err != nil {
		reqLogger.Error(err, "Failed to finalize datasource")
		return err
	}
	reqLogger.Info("Successfully finalized GrafanaDataSource")
	return nil
}

func (r *ReconcileGrafanaDashboard) addFinalizer(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDashboard) error {
	reqLogger.Info("Adding Finalizer for the datasource")
	cr.SetFinalizers(append(cr.GetFinalizers(), dashboardFinalizer))

	// Update CR
	err := r.client.Update(r.context, cr)
	if err != nil {
		reqLogger.Error(err, "Failed to update GrafanaDataSource with finalizer")
		return err
	}
	return nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func remove(list []string, s string) []string {
	for i, v := range list {
		if v == s {
			list = append(list[:i], list[i+1:]...)
		}
	}
	return list
}
