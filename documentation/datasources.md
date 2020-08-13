# Working with data sources

This document describes how to create data sources.

## DataSource discovery

The operator uses a list of [set based selectors](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements) to discover dataSources by their [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/). The `dataSourceLabelSelector` property of the `Grafana` resource allows you to add selectors by which the dataSources will be filtered.

*NOTE*: If no `datasourceLabelSelector` is present, the operator will not discover any dataSources. The same goes for dataSources without labels, they will not be discovered by the operator.

Every selector can have a list of `matchLabels` and `matchExpressions`. The rules inside a single selector will be **AND**ed, while the list of selectors is evaluated with **OR**.

For example, the following selector:

```yaml
datasourceLabelSelector:
  - matchExpressions:
      - {key: app, operator: In, values: [grafana]}
      - {key: group, operator: In, values: [grafana]}
```

requires the dataSource to have two labels, `app` and `group` and each label is required to have a value of `grafana`.

To accept either, the `app` or the `group` label, you can write the selector in the following way:

```yaml
datasourceLabelSelector:
  - matchExpressions:
      - {key: app, operator: In, values: [grafana]}
  - matchExpressions:
      - {key: group, operator: In, values: [grafana]}
```


## Data source properties

Data sources are represented by the `GrafanaDataSource` custom resource. Examples can be found in `deploy/examples/datasources`.

The following properties are accepted in the `spec`:

* *datasources*: data source definitions. Check the [official documentation](https://grafana.com/docs/features/datasources/).

A data source accepts all properties listed [here](https://grafana.com/docs/administration/provisioning/#example-datasource-config-file), but does not support `apiVersion` and `deleteDatasources`.