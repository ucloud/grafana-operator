package grafanadatasource

import (
	"encoding/json"
	"github.com/ucloud/grafana-operator/pkg/apis/monitor/v1alpha1"
)

type DatasourcePipeline interface {
	ProcessDatasource() ([]byte, error)
}

type DatasourcePipelineImpl struct {
	datasource *v1alpha1.GrafanaDataSource
}

func NewDatasourcePipeline(ds *v1alpha1.GrafanaDataSource) DatasourcePipeline {
	return &DatasourcePipelineImpl{
		datasource: ds,
	}
}

func (i *DatasourcePipelineImpl) ProcessDatasource() ([]byte, error) {
	return json.Marshal(i.datasource.Spec.Datasources)
}
