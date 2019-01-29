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

| Metric                         | Description                                                                         |
|--------------------------------|-------------------------------------------------------------------------------------|
| `azurerm_insights_metric`      | General exporter stats                                                              |


HTTP Endpoints
--------------


| Endpoint                       | Description                                                                         |
|--------------------------------|-------------------------------------------------------------------------------------|
| `/metrics`                     | Default prometheus golang metrics                                                   |
| `/probe`                       | Probe metrics (see `azurerm_insights_metric`)                                       |


#### Probe endpoint parameters


| GET parameter          | Default   | Required | Description                                                          |
|------------------------|-----------|----------|----------------------------------------------------------------------|
| `subscription`         |           | **yes**  | Azure Subscription ID                                                |
| `target`               |           | **yes**  | Azure Resource URI                                                   |
| `timespan`             | `PT1M`    | no       | Metric timespan                                                      |
| `interval`             |           | no       | Metric timespan                                                      |
| `metric`               |           | no       | Metric name                                                          |
| `aggregation`          |           | no       | Metric aggregation (`minimum`, `maximum`, `average`)                 |
