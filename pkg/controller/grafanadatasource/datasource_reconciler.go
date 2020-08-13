package grafanadatasource

import (
	"github.com/go-logr/logr"
	grafanaClient "github.com/ucloud/grafana-operator/pkg/controller/grafanaclient"

	grafanav1alpha1 "github.com/ucloud/grafana-operator/pkg/apis/monitor/v1alpha1"
	"github.com/ucloud/grafana-operator/pkg/controller/common"
)

func (r *ReconcileGrafanaDataSource) reconcile(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDataSource) error {
	matchedGrafs, err := common.MatchGrafana(r.context, r.client, reqLogger, cr.Namespace, cr.Labels)
	if err != nil {
		reqLogger.Error(err, "matchGrafana failed.")
		return err
	}

	for _, graf := range matchedGrafs {
		reqLogger.V(3).Info("reconcile datasource for grafana", "grafanaName", graf.Name)
		// Read current state
		state := common.NewClusterState()
		if err = state.Read(r.context, graf, r.client); err != nil {
			reqLogger.Error(err, "error reading state")
			continue
		}

		client, err := common.NewGrafanaClient(graf, state)
		if err != nil {
			reqLogger.Error(err, "newGrafanaClient failed")
			r.manageError(cr, err)
			continue
		}

		if _, err := client.GetDatasourceByName(cr.Spec.Datasources.Name); err != nil && err == grafanaClient.NotFoundError {
			pipeline := NewDatasourcePipeline(cr)
			processed, err := pipeline.ProcessDatasource()

			if err != nil {
				reqLogger.Error(err, "cannot process datasource")
				r.manageError(cr, err)
				continue
			}

			if processed == nil {
				continue
			}

			_, err = client.CreateDatasource(processed)
			if err != nil {
				reqLogger.Error(err, "cannot submit datasource", "grafana", graf.Name)
				r.manageError(cr, err)
				continue
			}
		} else if err != nil {
			reqLogger.Error(err, "cannot get datasource", "grafana", graf.Name)
			r.manageError(cr, err)
			continue
		} else if err == nil {
			continue
		}

		r.manageSuccess(cr)
	}

	return nil
}

func (r *ReconcileGrafanaDataSource) reconcileDelete(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDataSource) error {
	matchedGrafs, err := common.MatchGrafana(r.context, r.client, reqLogger, cr.Namespace, cr.Labels)
	if err != nil {
		reqLogger.Error(err, "matchGrafana failed.")
		return err
	}
	for _, graf := range matchedGrafs {
		reqLogger.V(3).Info("delete datasource from grafana", "grafanaName", graf.Name)
		state := common.NewClusterState()
		if err = state.Read(r.context, graf, r.client); err != nil {
			reqLogger.Error(err, "error reading state")
			continue
		}

		client, err := common.NewGrafanaClient(graf, state)
		if err != nil {
			reqLogger.Error(err, "newGrafanaClient failed")
			continue
		}

		if _, err := client.GetDatasourceByName(cr.Spec.Datasources.Name); err != nil {
			if err == grafanaClient.NotFoundError {
				reqLogger.Info("datasource already be deleted or not installed", "grafana", graf.Name)
			}
			reqLogger.Error(err, "cannot get datasource", "grafana", graf.Name)
			continue
		}

		if _, err = client.DeleteDatasourceByName(cr.Spec.Datasources.Name); err != nil {
			if err == grafanaClient.NotFoundError {
				reqLogger.Info("datasource already be deleted", "grafana", graf.Name)
			}
			reqLogger.Error(err, "cannot delete datasource", "grafana", graf.Name)
			continue
		}
	}
	return nil
}

//finalize needs to do before the CR can be deleted.
func (r *ReconcileGrafanaDataSource) finalize(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDataSource) error {
	if err := r.reconcileDelete(reqLogger, cr); err != nil {
		reqLogger.Error(err, "Failed to finalize datasource")
		return err
	}
	reqLogger.Info("Successfully finalized GrafanaDataSource")
	return nil
}

func (r *ReconcileGrafanaDataSource) addFinalizer(reqLogger logr.Logger, cr *grafanav1alpha1.GrafanaDataSource) error {
	reqLogger.Info("Adding Finalizer for the datasource")
	cr.SetFinalizers(append(cr.GetFinalizers(), datasourceFinalizer))

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
