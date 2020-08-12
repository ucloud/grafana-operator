package grafanaClient

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

const (
	deleteDashboardByUIDUrl    = "%s/api/dashboards/uid/%s"
	createOrUpdateDashboardUrl = "%s/api/dashboards/db"
	deleteDatasourceByNameUrl  = "%s/api/datasources/name/%s"
	createDatasourceUrl        = "%s/api/datasources"
	createOrUpdateFolderUrl    = "%s/api/folders"
	healthInfoUrl              = "%s/api/health"
)

const (
	NonNamespacedFolderName = "Non-Namespaced"
)

var (
	ConflictError = fmt.Errorf("conflict")
	NotFoundError = fmt.Errorf("not found")
)

type GrafanaRequest struct {
	Dashboard json.RawMessage `json:"dashboard"`
	FolderId  int64           `json:"folderId"`
	Overwrite bool            `json:"overwrite"`
}

type GrafanaResponse struct {
	ID      *uint   `json:"id"`
	OrgID   *uint   `json:"orgId"`
	Message *string `json:"message"`
	Slug    *string `json:"slug"`
	Version *int    `json:"version"`
	Status  *string `json:"resp"`
	UID     *string `json:"uid"`
	URL     *string `json:"url"`
}

type GrafanaFolderRequest struct {
	Title string `json:"title"`
}

type GrafanaFolderResponse struct {
	ID    *int64 `json:"id"`
	Title string `json:"title"`
}

type GrafanaClient interface {
	CheckGrafanaHealth() error
	GetDashboardsByName(name string) ([]GrafanaResponse, error)
	CreateOrUpdateDashboard(dashboard []byte, folderId int64) (GrafanaResponse, error)
	DeleteDashboardByUID(UID string) (GrafanaResponse, error)
	GetOrCreateNamespaceFolder(namespace string) (GrafanaFolderResponse, error)
	GetDatasourceByName(name string) (GrafanaResponse, error)
	CreateDatasource(datasource []byte) (GrafanaResponse, error)
	DeleteDatasourceByName(name string) (GrafanaResponse, error)
}

type GrafanaClientImpl struct {
	url      string
	user     string
	password string
	client   *http.Client
}

func setHeaders(req *http.Request) {
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "grafana-operator")
}

func NewGrafanaClient(url, user, password string, timeout time.Duration) GrafanaClient {
	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
		DisableKeepAlives: true,
	}

	client := &http.Client{
		Transport: &transport,
		Timeout:   timeout,
	}

	return &GrafanaClientImpl{
		url:      url,
		user:     user,
		password: password,
		client:   client,
	}
}

func (r *GrafanaClientImpl) getAllFolders() ([]GrafanaFolderResponse, error) {
	rawUrl := fmt.Sprintf(createOrUpdateFolderUrl, r.url)
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("GET", parsed.String(), nil)
	if err != nil {
		return nil, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.New(fmt.Sprintf(
			"error creating folder, expected status 200 but got %v",
			resp.StatusCode))
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var folders []GrafanaFolderResponse
	err = json.Unmarshal(data, &folders)
	return folders, err
}

// CheckGrafanaHealth check health information about Grafana.
func (r *GrafanaClientImpl) CheckGrafanaHealth() error {
	rawUrl := fmt.Sprintf(healthInfoUrl, r.url)

	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("GET", parsed.String(), nil)
	if err != nil {
		return err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf(
			"error get health information about Grafana, expected status 200 but got %v",
			resp.StatusCode)
	}

	return nil
}

func (r *GrafanaClientImpl) GetOrCreateNamespaceFolder(namespace string) (GrafanaFolderResponse, error) {
	response := newFolderResponse()

	folders, err := r.getAllFolders()
	if err != nil {
		return response, err
	}

	for _, folder := range folders {
		if folder.Title == namespace {
			return folder, nil
		}
	}

	rawUrl := fmt.Sprintf(createOrUpdateFolderUrl, r.url)
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return response, err
	}

	var title = namespace
	if title == "" {
		title = NonNamespacedFolderName
	}

	raw, err := json.Marshal(GrafanaFolderRequest{
		Title: title,
	})
	if err != nil {
		return response, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("POST", parsed.String(), bytes.NewBuffer(raw))
	if err != nil {
		return response, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return response, fmt.Errorf(
			"error creating folder, expected status 200 but got %v",
			resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(data, &response)
	return response, err
}

// GetDashboardsByName get dashboards given by name.
func (r *GrafanaClientImpl) GetDashboardsByName(name string) ([]GrafanaResponse, error) {
	rawUrl := fmt.Sprintf("%s/api/search?query=%s", r.url, url.QueryEscape(name))
	var response []GrafanaResponse

	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return response, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("GET", parsed.String(), nil)
	if err != nil {
		return response, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return response, NotFoundError
		}
		return response, fmt.Errorf(
			"error get datasource, expected status 200 but got %v",
			resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(data, &response)
	return response, err
}

// Submit dashboard json to grafana
func (r *GrafanaClientImpl) CreateOrUpdateDashboard(dashboard []byte, folderId int64) (GrafanaResponse, error) {
	rawUrl := fmt.Sprintf(createOrUpdateDashboardUrl, r.url)
	response := newResponse()

	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return response, err
	}

	// Grafana expects some additional data along with the dashboard
	raw, err := json.Marshal(GrafanaRequest{
		Dashboard: dashboard,

		FolderId: folderId,

		// We always want to set `overwrite` because the uids in the CRs map
		// directly to dashboards in grafana
		Overwrite: true,
	})
	if err != nil {
		return response, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("POST", parsed.String(), bytes.NewBuffer(raw))
	if err != nil {
		return response, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return response, fmt.Errorf(
			"error creating dashboard, expected status 200 but got %v",
			resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(data, &response)
	return response, err
}

// Delete a dashboard given by a UID
func (r *GrafanaClientImpl) DeleteDashboardByUID(UID string) (GrafanaResponse, error) {
	rawUrl := fmt.Sprintf(deleteDashboardByUIDUrl, r.url, UID)
	response := newResponse()

	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return response, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("DELETE", parsed.String(), nil)
	if err != nil {
		return response, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return response, fmt.Errorf(
			"error deleting dashboard, expected status 200 but got %v",
			resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(data, &response)
	return response, err
}

func newFolderResponse() GrafanaFolderResponse {
	var id int64 = 0
	return GrafanaFolderResponse{
		ID: &id,
	}
}

func newResponse() GrafanaResponse {
	var id uint = 0
	var orgId uint = 0
	var version int = 0
	var status = "(empty)"
	var message = "(empty)"
	var slug string
	var uid string
	var url string

	return GrafanaResponse{
		ID:      &id,
		OrgID:   &orgId,
		Message: &message,
		Slug:    &slug,
		Version: &version,
		Status:  &status,
		UID:     &uid,
		URL:     &url,
	}
}

// CreateDatasource Submit datasource json to grafana.
func (r *GrafanaClientImpl) CreateDatasource(datasource []byte) (GrafanaResponse, error) {
	rawUrl := fmt.Sprintf(createDatasourceUrl, r.url)
	response := newResponse()

	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return response, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("POST", parsed.String(), bytes.NewBuffer(datasource))
	if err != nil {
		return response, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusConflict {
			return response, ConflictError
		}
		return response, fmt.Errorf(
			"error creating datasource, expected status 200 but got %v",
			resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(data, &response)
	return response, err
}

// DeleteDatasourceByName Delete a datasource given by name.
func (r *GrafanaClientImpl) DeleteDatasourceByName(name string) (GrafanaResponse, error) {
	rawUrl := fmt.Sprintf(deleteDatasourceByNameUrl, r.url, name)
	response := newResponse()

	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return response, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("DELETE", parsed.String(), nil)
	if err != nil {
		return response, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return response, NotFoundError
		}
		return response, fmt.Errorf(
			"error deleting datasource, expected status 200 but got %v",
			resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(data, &response)
	return response, err
}

// GetDatasourceByName get a datasource given by name.
func (r *GrafanaClientImpl) GetDatasourceByName(name string) (GrafanaResponse, error) {
	rawUrl := fmt.Sprintf(deleteDatasourceByNameUrl, r.url, name)
	response := newResponse()

	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return response, err
	}

	parsed.User = url.UserPassword(r.user, r.password)
	req, err := http.NewRequest("GET", parsed.String(), nil)
	if err != nil {
		return response, err
	}

	setHeaders(req)

	resp, err := r.client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusNotFound {
			return response, NotFoundError
		}
		return response, fmt.Errorf(
			"error get datasource, expected status 200 but got %v",
			resp.StatusCode)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	err = json.Unmarshal(data, &response)
	return response, err
}
