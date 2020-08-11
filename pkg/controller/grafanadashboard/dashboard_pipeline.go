package grafanadashboard

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/go-jsonnet"
	"github.com/ucloud/grafana-operator/v3/pkg/apis/monitor/v1alpha1"
	"github.com/ucloud/grafana-operator/v3/pkg/controller/config"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

type SourceType int

const (
	SourceTypeJson    SourceType = 1
	SourceTypeJsonnet SourceType = 2
	SourceTypeUnknown SourceType = 3
)

type DashboardPipeline interface {
	ProcessDashboard() ([]byte, error)
}

type DashboardPipelineImpl struct {
	Client    client.Client
	Dashboard *v1alpha1.GrafanaDashboard
	JSON      string
	Board     map[string]interface{}
	Logger    logr.Logger
}

func NewDashboardPipeline(client client.Client, dashboard *v1alpha1.GrafanaDashboard) DashboardPipeline {
	return &DashboardPipelineImpl{
		Client:    client,
		Dashboard: dashboard,
		JSON:      "",
		Logger:    logf.Log.WithName(fmt.Sprintf("dashboard-%v", dashboard.Name)),
	}
}

func (r *DashboardPipelineImpl) ProcessDashboard() ([]byte, error) {
	err := r.obtainJson()
	if err != nil {
		return nil, err
	}

	// Datasource inputs to resolve?
	err = r.resolveDatasources()
	if err != nil {
		return nil, err
	}

	// Dashboard valid?
	err = r.validateJson()
	if err != nil {
		return nil, err
	}

	// Dashboards are never expected to come with an ID, it is
	// always assigned by Grafana. If there is one, we ignore it
	r.Board["id"] = nil
	// Overwrite in case any user provided uid exists
	r.Board["uid"] = r.Dashboard.UID()
	raw, err := json.Marshal(r.Board)
	if err != nil {
		return nil, err
	}

	return bytes.TrimSpace(raw), nil
}

// Make sure the dashboard contains valid JSON
func (r *DashboardPipelineImpl) validateJson() error {
	contents, err := r.Dashboard.Parse(r.JSON)
	r.Board = contents
	return err
}

// Try to get the dashboard json definition either from a provided URL or from the
// raw json in the dashboard resource. The priority is as follows:
// 1) try to fetch from url if provided
// 2) url fails or not provided: try to fetch from configmap ref
// 3) no configmap specified: try to use embedded json
// 4) no json specified: try to use embedded jsonnet
func (r *DashboardPipelineImpl) obtainJson() error {
	if r.Dashboard.Spec.Url != "" {
		err := r.loadDashboardFromURL()
		if err != nil {
			r.Logger.Error(err, "failed to request dashboard url, falling back to raw json")
		} else {
			return nil
		}
	}

	if r.Dashboard.Spec.ConfigMapRef != nil {
		err := r.loadDashboardFromConfigMap()
		if err != nil {
			r.Logger.Error(err, "failed to get config map, falling back to raw json")
		} else {
			return nil
		}
	}

	if r.Dashboard.Spec.Json != "" {
		r.JSON = r.Dashboard.Spec.Json
		return nil
	}

	if r.Dashboard.Spec.Jsonnet != "" {
		json, err := r.loadJsonnet(r.Dashboard.Spec.Jsonnet)
		if err != nil {
			r.Logger.Error(err, "failed to parse jsonnet")
		} else {
			r.JSON = json
			return nil
		}
	}

	return errors.New("unable to obtain dashboard contents")
}

// Compiles jsonnet to json and makes the grafonnet library available to
// the template
func (r *DashboardPipelineImpl) loadJsonnet(source string) (string, error) {
	cfg := config.GetControllerConfig()
	jsonnetLocation := cfg.GetConfigString(config.ConfigJsonnetBasePath, config.JsonnetBasePath)

	vm := jsonnet.MakeVM()

	vm.Importer(&jsonnet.FileImporter{
		JPaths: []string{jsonnetLocation},
	})

	return vm.EvaluateSnippet(r.Dashboard.Name, source)
}

// Try to obtain the dashboard json from a provided url
func (r *DashboardPipelineImpl) loadDashboardFromURL() error {
	url, err := url.ParseRequestURI(r.Dashboard.Spec.Url)
	if err != nil {
		return errors.New(fmt.Sprintf("invalid url %v", r.Dashboard.Spec.Url))
	}

	resp, err := http.Get(r.Dashboard.Spec.Url)
	if err != nil {
		return errors.New(fmt.Sprintf("cannot request %v", r.Dashboard.Spec.Url))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("request failed with status %v", resp.StatusCode)
	}

	body, _ := ioutil.ReadAll(resp.Body)
	sourceType := r.getFileType(url.Path)

	switch sourceType {
	case SourceTypeJson, SourceTypeUnknown:
		// If unknown, assume json
		r.JSON = string(body)
	case SourceTypeJsonnet:
		json, err := r.loadJsonnet(string(body))
		if err != nil {
			return err
		}
		r.JSON = json
	}

	return nil
}

// Try to determine the type (json or grafonnet) or a remote file by looking
// at the filename extension
func (r *DashboardPipelineImpl) getFileType(path string) SourceType {
	fragments := strings.Split(path, ".")
	if len(fragments) == 0 {
		return SourceTypeUnknown
	}

	extension := strings.TrimSpace(fragments[len(fragments)-1])
	switch strings.ToLower(extension) {
	case "json":
		return SourceTypeJson
	case "grafonnet":
		return SourceTypeJsonnet
	case "jsonnet":
		return SourceTypeJsonnet
	default:
		return SourceTypeUnknown
	}
}

// Try to obtain the dashboard json from a config map
func (r *DashboardPipelineImpl) loadDashboardFromConfigMap() error {
	ctx := context.Background()
	objectKey := client.ObjectKey{Name: r.Dashboard.Spec.ConfigMapRef.Name, Namespace: r.Dashboard.Namespace}

	var cm corev1.ConfigMap
	err := r.Client.Get(ctx, objectKey, &cm)
	if err != nil {
		return err
	}

	r.JSON = cm.Data[r.Dashboard.Spec.ConfigMapRef.Key]

	return nil
}

func (r *DashboardPipelineImpl) resolveDatasources() error {
	if len(r.Dashboard.Spec.Datasources) == 0 {
		return nil
	}

	currentJson := r.JSON
	for _, input := range r.Dashboard.Spec.Datasources {
		if input.DatasourceName == "" || input.InputName == "" {
			msg := fmt.Sprintf("invalid datasource input rule, input or datasource empty")
			r.Logger.Info(msg)
			return errors.New(msg)
		}

		searchValue := fmt.Sprintf("${%s}", input.InputName)
		currentJson = strings.ReplaceAll(currentJson, searchValue, input.DatasourceName)
		r.Logger.Info(fmt.Sprintf("resolving input %s to %s", input.InputName, input.DatasourceName))
	}

	r.JSON = currentJson
	return nil
}
