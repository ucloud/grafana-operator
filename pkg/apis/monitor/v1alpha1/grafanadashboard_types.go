package v1alpha1

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const GrafanaDashboardKind = "GrafanaDashboard"

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GrafanaDashboardSpec defines the desired state of GrafanaDashboard
type GrafanaDashboardSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Json         string                       `json:"json"`
	Jsonnet      string                       `json:"jsonnet"`
	Name         string                       `json:"name"`
	Plugins      PluginList                   `json:"plugins,omitempty"`
	Url          string                       `json:"url,omitempty"`
	ConfigMapRef *corev1.ConfigMapKeySelector `json:"configMapRef,omitempty"`
	Datasources  []GrafanaDashboardDatasource `json:"datasources,omitempty"`
}

type GrafanaDashboardDatasource struct {
	InputName      string `json:"inputName"`
	DatasourceName string `json:"datasourceName"`
}

// Used to keep a dashboard reference without having access to the dashboard
// struct itself
type GrafanaDashboardRef struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	UID       string `json:"uid"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GrafanaDashboard is the Schema for the grafanadashboards API
// +k8s:openapi-gen=true
type GrafanaDashboard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec GrafanaDashboardSpec `json:"spec,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GrafanaDashboardList contains a list of GrafanaDashboard
type GrafanaDashboardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GrafanaDashboard `json:"items"`
}

type GrafanaDashboardStatusMessage struct {
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

func init() {
	SchemeBuilder.Register(&GrafanaDashboard{}, &GrafanaDashboardList{})
}

func (in *GrafanaDashboard) Hash() string {
	var datasources strings.Builder
	for _, input := range in.Spec.Datasources {
		datasources.WriteString(input.DatasourceName)
		datasources.WriteString(input.InputName)
	}

	hash := sha256.New()
	io.WriteString(hash, in.Spec.Json)
	io.WriteString(hash, in.Spec.Url)
	io.WriteString(hash, in.Spec.Jsonnet)
	io.WriteString(hash, in.Namespace)

	if in.Spec.ConfigMapRef != nil {
		io.WriteString(hash, in.Spec.ConfigMapRef.Name)
		io.WriteString(hash, in.Spec.ConfigMapRef.Key)
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

func (in *GrafanaDashboard) Parse(optional string) (map[string]interface{}, error) {
	var dashboardBytes = []byte(in.Spec.Json)
	if optional != "" {
		dashboardBytes = []byte(optional)
	}

	var parsed = make(map[string]interface{})
	err := json.Unmarshal(dashboardBytes, &parsed)
	return parsed, err
}

func (in *GrafanaDashboard) UID() string {
	content, err := in.Parse("")
	if err == nil {
		// Check if the user has defined an uid and if that's the
		// case, use that
		if content["uid"] != nil && content["uid"] != "" {
			return content["uid"].(string)
		}
	}

	// Use sha1 to keep the hash limit at 40 bytes which is what
	// Grafana allows for UIDs
	return fmt.Sprintf("%x", sha1.Sum([]byte(in.Namespace+in.Name)))
}

func (in *GrafanaDashboard) DashboardName() string {
	content, err := in.Parse("")
	if err == nil {
		// Check if the user has defined an uid and if that's the
		// case, use that
		if content["title"] != nil && content["title"] != "" {
			return content["title"].(string)
		}
	}

	return in.Name
}
