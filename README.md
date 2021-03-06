# Grafana Operator

It's based on the [integr8ly/grafana-operator](https://github.com/integr8ly/grafana-operator).

Grafana Operator creating and managing Grafana instances atop Kubernetes.

The operator itself is built with the [Operator framework](https://github.com/operator-framework/operator-sdk).

## Features

It can deploy and manage a Grafana instance on Kubernetes and OpenShift. The following features are supported:

* Create **Multiple** Grafana in the same namespace.
* Import Grafana dashboards from the same namespaces.
* Import Grafana datasources from the same namespace.
* Install Plugins (panels) defined as dependencies of dashboards.

## Supported Custom Resources

The following Grafana resources are supported:

* Grafana
* GrafanaDashboard
* GrafanaDatasource

all custom resources use the api group `monitor.kun` and version `v1alpha1`.

**Notes:** GrafanaDashboard and GrafanaDatasource do not support update, you can inject [ValidatingWebhook](/hack/webhook/README.md) to disable update the GrafanaDashboard and GrafanaDatasource.

### Grafana

Represents a Grafana instance. See [the documentation](./documentation/deploy_grafana.md) for a description of properties supported in the spec.

### GrafanaDashboard

Represents a Grafana dashboard and allows specifying required plugins. See [the documentation](./documentation/dashboards.md) for a description of properties supported in the spec.

### GrafanaDatasource

Represents a Grafana datasource. See [the documentation](./documentation/datasources.md) for a description of properties supported in the spec.

## Building the operator image

Init the submodules first to obtain grafonnet:

```sh
$ git submodule update --init
```

Then build the image using the operatpr-sdk:

```sh
$ operator-sdk build <registry>/<user>/grafana-operator:<tag>
```

## Running locally

You can run the Operator locally against a remote namespace using the operator-sdk:

Prerequisites:

* [operator-sdk](https://github.com/operator-framework/operator-sdk) installed
* kubectl pointing to the local context. [minikube](https://github.com/kubernetes/minikube) automatically sets the context to the local VM. If not you can use `kubectl config use <context>` or (if using the OpenShift CLI) `oc login -u <user> <url>`
* make sure to deploy the custom resource definition using the command ```kubectl create -f deploy/crds```

```sh
$ operator-sdk run local --namespace=<namespace> --operator-flags="<flags to pass>"
```

## Grafana features not yet supported in the operator

### Notifier provisioning

Grafana has provisioning support for multiple channels (notifiers) of alerts. The operator does currently not support this type of provisioning. An empty directory is mounted at the expected location to prevent a warning in the grafana log. This feature might be supported in the future. 
