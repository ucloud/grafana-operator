package model

import (
	"crypto/rand"
	"encoding/base64"

	"github.com/ucloud/grafana-operator/pkg/apis/monitor/v1alpha1"
)

func generateRandomBytes(n int) []byte {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return b
}

func RandStringRunes(s int) string {
	b := generateRandomBytes(s)
	return base64.URLEncoding.EncodeToString(b)
}

func MergeAnnotations(requested map[string]string, existing map[string]string) map[string]string {
	if existing == nil {
		return requested
	}

	for k, v := range requested {
		existing[k] = v
	}
	return existing
}

func getLabels(cr *v1alpha1.Grafana) map[string]string {
	var labels = map[string]string{}
	if cr.Spec.Deployment != nil && cr.Spec.Deployment.Labels != nil {
		labels = cr.Spec.Deployment.Labels
	}
	labels["app"] = GrafanaPodLabel
	labels["grafana"] = cr.Name
	return labels
}
