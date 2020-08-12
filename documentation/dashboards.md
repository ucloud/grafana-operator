# Working with dashboards

This document describes how to create dashboards and manage plugins (panels).

## Dashboard properties

Dashboards are represented by the `GrafanaDashboard` custom resource. Examples can be found in `deploy/examples/dashboards`.

The following properties are accepted in the `spec`:

* *name*: The filename of the dashboard that gets mounted into a volume in the grafana instance. Not to be confused with `metadata.name`.
* *json*: Raw json string with the dashboard contents. Check the [official documentation](https://grafana.com/docs/reference/dashboard/#dashboard-json).
* *jsonnet*: Jsonnet source. The [Grafonnet](https://grafana.github.io/grafonnet-lib/) library is made available automatically and can be imported.
* *url*: Url address to download a json or jsonnet string with the dashboard contents. This will take priority over the json field in case the download is successful.
* *plugins*: A list of plugins required by the dashboard. They will be installed by the operator if not already present.
* *datasources*: A list of datasources to be used as inputs. See [datasource inputs](#datasource-inputs).
* *configMapRef*: Import dashboards from config maps. See [config map refreences](#config-map-references).

## Creating a new dashboard

The operator import dashboards for Grafana instance from the Grafana's same namespaces.

To create a dashboard in the `Grafana instance` namespace run:

```sh
$ kubectl create -f deploy/examples/dashboards/SimpleDashboard.yaml -n namespace_where_has_Grafana
```

## Dashboard UIDs

Grafana allows users to define the UIDs of dashboards. If an uid is present on a dashbaord, the operator will use it and not assign a generated one. This is often used to guarantee predictable dashboard URLs for interlinking.

## Plugins

Dashboards can specify plugins (panels) they depend on. The operator will automatically install them.

You need to provide a name and a version for every plugin, e.g.:

```yaml
spec:
  name: "dummy"
  json: "{}"
  plugins:
    - name: "grafana-piechart-panel"
      version: "1.3.6"
    - name: "grafana-clock-panel"
      version: "1.0.2"
```

Plugins are installed from the [Grafana plugin registry](https://grafana.com/plugins).

## Dashboard discovery

The operator uses a list of [set based selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements) to discover dashboards by their [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/). The `dashboardLabelSelector` property of the `Grafana` resource allows you to add selectors by which the dashboards will be filtered.

*NOTE*: If no `dashboardLabelSelector` is present, the operator will not discover any dashboards. The same goes for dashboards without labels, they will not be discovered by the operator. 

Every selector can have a list of `matchLabels` and `matchExpressions`. The rules inside a single selector will be **AND**ed, while the list of selectors is evaluated with **OR**. 

For example, the following selector:

```yaml
dashboardLabelSelector:
  - matchExpressions:
      - {key: app, operator: In, values: [grafana]}
      - {key: group, operator: In, values: [grafana]}
```

requires the dashboard to have two labels, `app` and `group` and each label is required to have a value of `grafana`.

To accept either, the `app` or the `group` label, you can write the selector in the following way:

```yaml
dashboardLabelSelector:
  - matchExpressions:
      - {key: app, operator: In, values: [grafana]}
  - matchExpressions:
      - {key: group, operator: In, values: [grafana]}          
```

## Datasource inputs

Dashboards may rely on certain datasources to be present. When a dashboard is exported, Grafana will populate an `__inputs` array with required datasources. When importing such a dashboard, the required datasources have to be mapped to datasources existing in the Grafana instance. For example, consider the following dashboard:

```json
{
"__inputs": [
  {
    "name": "DS_PROMETHEUS",
    "label": "Prometheus",
    "description": "",
    "type": "datasource",
    "pluginId": "prometheus",
    "pluginName": "Prometheus"
  }
],
"title": ...
"panels": ...
}
```

A Prometheus datasource is expected and will be referred to as `DS_PROMETHEUS` in the dashboard. To map this to an existing datasource with the name `Prometheus`, add the following `datasources` section to the dashboard:

```yaml
...
spec:
  datasources:
    - inputName: "DS_PROMETHEUS"
      datasourceName: "Prometheus"
...
```

This will allow the operator to replace all occurrences of the datasource variable `DS_PROMETHEUS` with the actual name of the datasource. An example for this is `dashboards/KeycloakDashboard.yaml`.

## Config map references

The json contents of a dashboard can be defined in a config map with the dashboard CR pointing to that config map.

```yaml
...
spec:
  name: grafana-dashboard-from-config-map.json
  configMapRef:
    name: <config map name>
    key: <key of the entry containing the json contents>
...
```