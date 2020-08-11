package common

import (
	"context"
	stdErr "errors"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	grafanav1alpha1 "github.com/ucloud/grafana-operator/v3/pkg/apis/monitor/v1alpha1"
	grafanaClient "github.com/ucloud/grafana-operator/v3/pkg/controller/grafanaclient"
	"github.com/ucloud/grafana-operator/v3/pkg/controller/model"
)

const DefaultClientTimeout = 5 * time.Second

func matchesSelector(l map[string]string, s *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(s)
	if err != nil {
		return false, err
	}

	return selector.Empty() || selector.Matches(labels.Set(l)), nil
}

// MatchesSelectors Check if the labels matches at least one of the selectors
func MatchesSelectors(l map[string]string, s []*metav1.LabelSelector) (bool, error) {
	result := false

	for _, selector := range s {
		match, err := matchesSelector(l, selector)
		if err != nil {
			return false, err
		}

		result = result || match
	}

	return result, nil
}

func MatchGrafana(ctx context.Context, kubeclient client.Client, reqLogger logr.Logger, namespace string, label map[string]string) ([]*grafanav1alpha1.Grafana, error) {
	foundGrafanas := &grafanav1alpha1.GrafanaList{}
	err := kubeclient.List(ctx, foundGrafanas, client.InNamespace(namespace))
	if err != nil {
		return nil, err
	}

	var result []*grafanav1alpha1.Grafana
	for _, item := range foundGrafanas.Items {
		match, err := MatchesSelectors(label, item.Spec.DashboardLabelSelector)
		if err != nil {
			return nil, err
		}
		if match {
			grafanaDeployment := &appsv1.Deployment{}
			if err := kubeclient.Get(ctx, types.NamespacedName{
				Namespace: namespace,
				Name:      model.GetGrafanaDeploymentName(&item),
			}, grafanaDeployment); err != nil {
				if errors.IsNotFound(err) {
					continue
				}
				return nil, err
			}
			if grafanaDeployment.Status.ReadyReplicas != *grafanaDeployment.Spec.Replicas {
				reqLogger.V(4).Info("grafanaDeployment not ready", "deployment", model.GetGrafanaDeploymentName(&item),
					"readyReplicas", grafanaDeployment.Status.ReadyReplicas, "expectReplicas", *grafanaDeployment.Spec.Replicas)
				continue
			}
			result = append(result, item.DeepCopy())
		}
	}

	return result, nil
}

func getGrafanaAdminUrl(cr *grafanav1alpha1.Grafana, state *ClusterState) (string, error) {
	// If preferService is true, we skip the routes and try to access grafana
	// by using the service.
	preferService := false
	if cr.Spec.Client != nil {
		preferService = cr.Spec.Client.PreferService
	}

	// First try to use the route if it exists. Prefer the route because it also works
	// when running the operator outside of the cluster
	if state.GrafanaRoute != nil && !preferService {
		return fmt.Sprintf("https://%v", state.GrafanaRoute.Spec.Host), nil
	}

	// Try the ingress first if on vanilla Kubernetes
	if state.GrafanaIngress != nil && !preferService {
		// If provided, use the hostname from the CR
		if cr.Spec.Ingress != nil && cr.Spec.Ingress.Hostname != "" {
			return fmt.Sprintf("https://%v", cr.Spec.Ingress.Hostname), nil
		}

		// Otherwise try to find something suitable, hostname or IP
		for _, ingress := range state.GrafanaIngress.Status.LoadBalancer.Ingress {
			if ingress.Hostname != "" {
				return fmt.Sprintf("https://%v", ingress.Hostname), nil
			}
			return fmt.Sprintf("https://%v", ingress.IP), nil
		}
	}

	var servicePort = int32(model.GetGrafanaPort(cr))

	// Otherwise rely on the service
	if state.GrafanaService != nil && state.GrafanaService.Spec.ClusterIP != "" {
		return fmt.Sprintf("http://%v:%d", state.GrafanaService.Spec.ClusterIP,
			servicePort), nil
	} else if state.GrafanaService != nil {
		return fmt.Sprintf("http://%v:%d", state.GrafanaService.Name,
			servicePort), nil
	}

	return "", stdErr.New("failed to find admin url")
}

func NewGrafanaClient(cr *grafanav1alpha1.Grafana, state *ClusterState) (grafanaClient.GrafanaClient, error) {
	username := string(state.AdminSecret.Data[model.GrafanaAdminUserEnvVar])
	password := string(state.AdminSecret.Data[model.GrafanaAdminPasswordEnvVar])
	url, err := getGrafanaAdminUrl(cr, state)
	if err != nil {
		return nil, err
	}

	if url == "" {
		return nil, stdErr.New("cannot get grafana admin url")
	}
	if username == "" {
		return nil, stdErr.New("invalid credentials (username)")
	}
	if password == "" {
		return nil, stdErr.New("invalid credentials (password)")
	}

	return grafanaClient.NewGrafanaClient(url, username, password, DefaultClientTimeout), nil
}
