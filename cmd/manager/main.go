package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"

	routev1 "github.com/openshift/api/route/v1"
	"github.com/operator-framework/operator-sdk/pkg/k8sutil"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/metrics"
	sdkVersion "github.com/operator-framework/operator-sdk/version"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	"github.com/ucloud/grafana-operator/pkg/apis"
	grafanav1alpha1 "github.com/ucloud/grafana-operator/pkg/apis/monitor/v1alpha1"
	"github.com/ucloud/grafana-operator/pkg/controller"
	"github.com/ucloud/grafana-operator/pkg/controller/common"
	config2 "github.com/ucloud/grafana-operator/pkg/controller/config"
	"github.com/ucloud/grafana-operator/version"
)

var log = logf.Log.WithName("cmd")
var flagImage string
var flagImageTag string
var flagPluginsInitContainerImage string
var flagPluginsInitContainerTag string
var flagJsonnetLocation string

var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8080
)

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
	log.Info(fmt.Sprintf("operator-sdk Version: %v", sdkVersion.Version))
	log.Info(fmt.Sprintf("operator Version: %v", version.Version))
}

func init() {
	flagset := flag.CommandLine
	flagset.StringVar(&flagImage, "grafana-image", "", "Overrides the default Grafana image")
	flagset.StringVar(&flagImageTag, "grafana-image-tag", "", "Overrides the default Grafana image tag")
	flagset.StringVar(&flagPluginsInitContainerImage, "grafana-plugins-init-container-image", "", "Overrides the default Grafana Plugins Init Container image")
	flagset.StringVar(&flagPluginsInitContainerTag, "grafana-plugins-init-container-tag", "", "Overrides the default Grafana Plugins Init Container tag")
	flagset.StringVar(&flagJsonnetLocation, "jsonnet-location", "", "Overrides the base path of the jsonnet libraries")
	flagset.Parse(os.Args[1:])
}

func main() {
	// The logger instantiated here can be changed to any logger
	// implementing the logr.Logger interface. This logger will
	// be propagated through the whole operator, generating
	// uniform and structured logs.
	logf.SetLogger(logf.ZapLogger(false))

	printVersion()

	namespace, err := k8sutil.GetWatchNamespace()
	if err != nil {
		log.Error(err, "failed to get watch namespace")
		os.Exit(1)
	}

	// Controller configuration
	controllerConfig := config2.GetControllerConfig()
	controllerConfig.AddConfigItem(config2.ConfigGrafanaImage, flagImage)
	controllerConfig.AddConfigItem(config2.ConfigGrafanaImageTag, flagImageTag)
	controllerConfig.AddConfigItem(config2.ConfigPluginsInitContainerImage, flagPluginsInitContainerImage)
	controllerConfig.AddConfigItem(config2.ConfigPluginsInitContainerTag, flagPluginsInitContainerTag)
	controllerConfig.AddConfigItem(config2.ConfigOperatorNamespace, namespace)
	controllerConfig.AddConfigItem(config2.ConfigDashboardLabelSelector, "")
	controllerConfig.AddConfigItem(config2.ConfigJsonnetBasePath, flagJsonnetLocation)

	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Become the leader before proceeding
	leader.Become(context.TODO(), "grafana-operator-lock")

	// Create a new Cmd to provide shared dependencies and start components
	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          namespace,
		MetricsBindAddress: fmt.Sprintf("%s:%d", metricsHost, metricsPort),
	})
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	log.Info("Registering Components.")

	// Starting the resource auto-detection for the grafana controller
	autodetect, err := common.NewAutoDetect(mgr)
	if err != nil {
		log.Error(err, "failed to start the background process to auto-detect the operator capabilities")
	} else {
		autodetect.Start()
		defer autodetect.Stop()
	}

	// Setup Scheme for all resources
	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup Scheme for OpenShift routes
	if err := routev1.AddToScheme(mgr.GetScheme()); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr, autodetect.SubscriptionChannel); err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	servicePorts := []v1.ServicePort{
		{
			Name:       metrics.OperatorPortName,
			Protocol:   v1.ProtocolTCP,
			Port:       metricsPort,
			TargetPort: intstr.IntOrString{Type: intstr.Int, IntVal: metricsPort},
		},
	}
	_, err = metrics.CreateMetricsService(context.TODO(), cfg, servicePorts)
	if err != nil {
		log.Error(err, "error starting metrics service")
	}

	if os.Getenv("ENABLE_WEBHOOKS") == "true" {
		startWebHook(mgr)
	}

	log.Info("Starting the Cmd.")

	signalHandler := signals.SetupSignalHandler()

	if err := mgr.Start(signalHandler); err != nil {
		log.Error(err, "manager exited non-zero")
		os.Exit(1)
	}
}

func startWebHook(mgr manager.Manager) {
	log.Info("Starting the WebHook.")
	ws := mgr.GetWebhookServer()
	ws.CertDir = "/etc/webhook/certs"
	ws.Port = 7443
	if err := (&grafanav1alpha1.GrafanaDashboard{}).SetupWebhookWithManager(mgr); err != nil {
		log.Error(err, "unable to create webHook", "webHook", "GrafanaDashboard")
		os.Exit(1)
	}
	if err := (&grafanav1alpha1.GrafanaDataSource{}).SetupWebhookWithManager(mgr); err != nil {
		log.Error(err, "unable to create webHook", "webHook", "GrafanaDataSource")
		os.Exit(1)
	}
}

//func serveProfiling() {
//	log.Info("Starting the Profiling.")
//	mux := mux.NewPathRecorderMux("grafana-operator")
//	routes.Profiling{}.Install(mux)
//	go wait.Until(func() {
//		err := http.ListenAndServe("[::]:10269", mux)
//		if err != nil {
//			log.Error(err, "starting metrics server failed")
//			os.Exit(1)
//		}
//	}, 5*time.Second, wait.NeverStop)
//}
