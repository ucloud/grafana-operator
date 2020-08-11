package model

const (
	GrafanaImage                    = "grafana/grafana"
	GrafanaVersion                  = "7.1.1"
	grafanaServiceAccountName       = "grafana-serviceaccount"
	grafanaServiceName              = "grafana-service"
	grafanaDataStorageName          = "grafana-pvc"
	grafanaConfigName               = "grafana-config"
	grafanaConfigFileName           = "grafana.ini"
	grafanaIngressName              = "grafana-ingress"
	grafanaRouteName                = "grafana-route"
	grafanaDeploymentName           = "grafana-deployment"
	GrafanaPluginsVolumeName        = "grafana-plugins"
	GrafanaInitContainerName        = "grafana-plugins-init"
	GrafanaLogsVolumeName           = "grafana-logs"
	GrafanaDataVolumeName           = "grafana-data"
	GrafanaHealthEndpoint           = "/api/health"
	GrafanaPodLabel                 = "grafana"
	LastConfigAnnotation            = "last-config"
	LastConfigEnvVar                = "LAST_CONFIG"
	LastDatasourcesConfigEnvVar     = "LAST_DATASOURCES"
	grafanaAdminSecretName          = "grafana-admin-credentials"
	DefaultAdminUser                = "admin"
	GrafanaAdminUserEnvVar          = "GF_SECURITY_ADMIN_USER"
	GrafanaAdminPasswordEnvVar      = "GF_SECURITY_ADMIN_PASSWORD"
	GrafanaHttpPort             int = 3000
	GrafanaHttpPortName             = "grafana"
)
