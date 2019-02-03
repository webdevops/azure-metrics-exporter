Azure Insights metrics exporter
===============================

[![license](https://img.shields.io/github/license/webdevops/azure-metrics-exporter.svg)](https://github.com/webdevops/azure-metrics-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fazure--metrics--exporter-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/azure-metrics-exporter/)
[![Docker Build Status](https://img.shields.io/docker/build/webdevops/azure-metrics-exporter.svg)](https://hub.docker.com/r/webdevops/azure-metrics-exporter/)

Prometheus exporter for Azure Insights metrics (on demand)

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable              | DefaultValue                | Description                                        |
|-----------------------------------|-----------------------------|----------------------------------------------------|
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                    |

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Metrics
-------

| Metric                             | Description                                                                     |
|------------------------------------|---------------------------------------------------------------------------------|
| `azurerm_stats_metric_collecttime` | General exporter stats                                                          |
| `azurerm_resource_metric`          | Resource metrics exported by probes                                             |


HTTP Endpoints
--------------

| Endpoint                       | Description                                                                         |
|--------------------------------|-------------------------------------------------------------------------------------|
| `/metrics`                     | Default prometheus golang metrics                                                   |
| `/probe/resource`              | Probe metrics for one resource (see `azurerm_resource_metric`)                      |
| `/probe/list`                  | Probe metrics for list of resources (see `azurerm_resource_metric`)                 |
| `/probe/scrape`                | Probe metrics for list of resources and config on resource by tag name (see `azurerm_resource_metric`) |


#### /probe/metrics/resource parameters


| GET parameter          | Default   | Required | Description                                                          |
|------------------------|-----------|----------|----------------------------------------------------------------------|
| `subscription`         |           | **yes**  | Azure Subscription ID                                                |
| `target`               |           | **yes**  | Azure Resource URI                                                   |
| `timespan`             | `PT1M`    | no       | Metric timespan                                                      |
| `interval`             |           | no       | Metric timespan                                                      |
| `metric`               |           | no       | Metric name                                                          |
| `aggregation`          |           | no       | Metric aggregation (`minimum`, `maximum`, `average`)                 |


#### /probe/metrics/list parameters

| GET parameter          | Default   | Required | Description                                                          |
|------------------------|-----------|----------|----------------------------------------------------------------------|
| `subscription`         |           | **yes**  | Azure Subscription ID                                                |
| `filter`               |           | **yes**  | Azure Resource filter (https://docs.microsoft.com/en-us/rest/api/resources/resources/list)                                              |
| `timespan`             | `PT1M`    | no       | Metric timespan                                                      |
| `interval`             |           | no       | Metric timespan                                                      |
| `metric`               |           | no       | Metric name                                                          |
| `aggregation`          |           | no       | Metric aggregation (`minimum`, `maximum`, `average`)                 |


#### /probe/metrics/scrape parameters

| GET parameter          | Default   | Required | Description                                                          |
|------------------------|-----------|----------|----------------------------------------------------------------------|
| `subscription`         |           | **yes**  | Azure Subscription ID                                                |
| `filter`               |           | **yes**  | Azure Resource filter (https://docs.microsoft.com/en-us/rest/api/resources/resources/list)                                              |
| `metricTagName`        |           | **yes**  | Resource tag name for getting "metric" list                                                                                             |
| `aggregationTagName`   |           | **yes**  | Resource tag name for getting "aggregation" list                                         |
| `timespan`             | `PT1M`    | no       | Metric timespan                                                      |
| `interval`             |           | no       | Metric timespan                                                      |
| `metric`               |           | no       | Metric name                                                          |
| `aggregation`          |           | no       | Metric aggregation (`minimum`, `maximum`, `average`)                 |

