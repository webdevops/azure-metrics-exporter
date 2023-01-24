# Azure Monitor metrics exporter

[![license](https://img.shields.io/github/license/webdevops/azure-metrics-exporter.svg)](https://github.com/webdevops/azure-metrics-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fazure--metrics--exporter-blue)](https://hub.docker.com/r/webdevops/azure-metrics-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fazure--metrics--exporter-blue)](https://quay.io/repository/webdevops/azure-metrics-exporter)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/azure-metrics-exporter)](https://artifacthub.io/packages/search?repo=azure-metrics-exporter)

Prometheus exporter for Azure Monitor metrics.
Supports metrics fetching from all resource with one scrape (automatic service discovery), custom metric names with template system, full dimensions support and caching.

Configuration (except Azure connection) of this exporter is made entirely in Prometheus instead of a seperate configuration file, see examples below.

TOC:
* [Features](#Features)
* [Configuration](#configuration)
* [Metrics](#metrics)
    + [Azuretracing metrics](#azuretracing-metrics)
    + [Metric name and help template system](#metric-name-and-help-template-system)
        - [default template](#default-template)
        - [template `{name}_{metric}_{unit}`](#template-name_metric_unit)
        - [template `{name}_{metric}_{aggregation}_{unit}`](#template-name_metric_aggregation_unit)
* [HTTP Endpoints](#http-endpoints)
    + [/probe/metrics/resource parameters](#probemetricsresource-parameters)
    + [/probe/metrics/list parameters](#probemetricslist-parameters)
    + [/probe/metrics/scrape parameters](#probemetricsscrape-parameters)
* [Prometheus configuration examples](#prometheus-configuration-examples)
    * [Redis](#Redis)
    * [VirtualNetworkGateways](#virtualnetworkgateways)
    * [virtualNetworkGateway connections (dimension support)](#virtualnetworkgateway-connections-dimension-support)
    * [StorageAccount (metric namespace and dimension support)](#storageaccount-metric-namespace-and-dimension-support)
* [Development and testing query webui](#development-and-testing-query-webui)

## Features

- Uses of official [Azure SDK for go](https://github.com/Azure/azure-sdk-for-go)
- Supports all Azure environments (Azure public cloud, Azure governmant cloud, Azure china cloud, ...) via Azure SDK configuration
- Caching of Azure ServiceDiscovery to reduce Azure API calls
- Caching of fetched metrics (no need to request every minute from Azure Monitor API; you can keep scrape time of `30s` for metrics)
- Customizable metric names (with [template system with metric information](#metric-name-template-system))
- Ability to fetch metrics from one or more resources via `target` parameter  (see `/probe/metrics/resource`)
- Ability to fetch metrics from resources found with ServiceDiscovery via [Azure resources API based on $filter](https://docs.microsoft.com/en-us/rest/api/resources/resources/list) (see `/probe/metrics/list`)
- Ability to fetch metrics from resources found with ServiceDiscovery via [Azure resources API based on $filter](https://docs.microsoft.com/en-us/rest/api/resources/resources/list) with configuration inside Azure resource tags (see `/probe/metrics/scrape`)
- Ability to fetch metrics from resources found with ServiceDiscovery via [Azure ResourceGraph API based on Kusto query](https://docs.microsoft.com/en-us/azure/governance/resource-graph/overview) (see `/probe/metrics/resourcegraph`)
- Configuration based on Prometheus scraping config or ServiceMonitor manifest (Prometheus operator)
- Metric manipulation (adding, removing, updating or filtering of labels or metrics) can be done in scraping config (eg [`metric_relabel_configs`](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#metric_relabel_configs))
- Full metric [dimension support](#virtualnetworkgateway-connections-dimension-support)
- Docker image is based on [Google's distroless](https://github.com/GoogleContainerTools/distroless) static image to reduce attack surface (no shell, no other binaries inside image)
- Available via Docker Hub and Quay (see badges on top)
- Can run non-root and with readonly root filesystem, doesn't need any capabilities (you can safely use `drop: ["All"]`)
- Publishes Azure API rate limit metrics (when exporter sends Azure API requests, available via `/metrics`)

useful with additional exporters:

- [azure-resourcegraph-exporter](https://github.com/webdevops/azure-resourcegraph-exporter) for exporting Azure resource information from Azure ResourceGraph API with custom Kusto queries (get the tags from resources and ResourceGroups with this exporter)
- [azure-resourcemanager-exporter](https://github.com/webdevops/azure-resourcemanager-exporter) for exporting Azure subscription information (eg ratelimit, subscription quotas, ServicePrincipal expiry, RoleAssignments, resource health, ...)
- [azure-keyvault-exporter](https://github.com/webdevops/azure-keyvault-exporter) for exporting Azure KeyVault information (eg expiry date for secrets, certificates and keys)
- [azure-loganalytics-exporter](https://github.com/webdevops/azure-loganalytics-exporter) for exporting Azure LogAnalytics workspace information with custom Kusto queries (eg ingestion rate or application error count)

## Configuration

Normally no configuration is needed but can be customized using environment variables.

```
Usage:
  azure-metrics-exporter [OPTIONS]

Application Options:
      --log.debug                          debug mode [$LOG_DEBUG]
      --log.trace                          trace mode [$LOG_TRACE]
      --log.json                           Switch log output to json format [$LOG_JSON]
      --azure-environment=                 Azure environment name (default: AZUREPUBLICCLOUD) [$AZURE_ENVIRONMENT]
      --azure-ad-resource-url=             Specifies the AAD resource ID to use. If not set, it defaults to
                                           ResourceManagerEndpoint for operations with Azure Resource Manager
                                           [$AZURE_AD_RESOURCE]
      --azure.servicediscovery.cache=      Duration for caching Azure ServiceDiscovery of workspaces to reduce API calls
                                           (time.Duration) (default: 30m) [$AZURE_SERVICEDISCOVERY_CACHE]
      --azure.resource-tag=                Azure Resource tags (space delimiter) (default: owner) [$AZURE_RESOURCE_TAG]
      --metrics.template=                  Template for metric name (default: {name}) [$METRIC_TEMPLATE]
      --metrics.help=                      Metric help (with template support) (default: Azure monitor insight metric)
                                           [$METRIC_HELP]
      --concurrency.subscription=          Concurrent subscription fetches (default: 5) [$CONCURRENCY_SUBSCRIPTION]
      --concurrency.subscription.resource= Concurrent requests per resource (inside subscription requests) (default: 10)
                                           [$CONCURRENCY_SUBSCRIPTION_RESOURCE]
      --enable-caching                     Enable internal caching [$ENABLE_CACHING]
      --server.bind=                       Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=               Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write=              Server write timeout (default: 10s) [$SERVER_TIMEOUT_WRITE]

Help Options:
  -h, --help                               Show this help message
```

for Azure API authentication (using ENV vars) see following documentations:
- https://github.com/webdevops/go-common/blob/main/azuresdk/README.md
- https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication

## How to test

Enable the webui (`--development.webui`) to get a basic web frontend to query the exporter which helps you to find
the right settings for your configuration.

webui is available under url `/query`

## Metrics

| Metric                                   | Description                                                                                     |
|------------------------------------------|-------------------------------------------------------------------------------------------------|
| `azurerm_stats_metric_collecttime`       | General exporter stats                                                                          |
| `azurerm_stats_metric_requests`          | Counter of resource metric requests with result (error, success)                                |
| `azurerm_resource_metric` (customizable) | Resource metrics exported by probes (can be changed using `name` parameter and template system) |
| `azurerm_api_ratelimit`                  | Azure ratelimit metrics (only on /metrics, resets after query)                                  |
| `azurerm_api_request_*`                  | Azure request count and latency as histogram                                                    |

## AzureTracing metrics

(with 22.2.0 and later)

Azuretracing metrics collects latency and latency from azure-sdk-for-go and creates metrics and is controllable using
environment variables (eg. setting buckets, disabling metrics or disable autoreset).

| Metric                                   | Description                                                                            |
|------------------------------------------|----------------------------------------------------------------------------------------|
| `azurerm_api_ratelimit`                  | Azure ratelimit metrics (only on /metrics, resets after query due to limited validity) |
| `azurerm_api_request_*`                  | Azure request count and latency as histogram                                           |

### Settings

| Environment variable                     | Example                            | Description                                                    |
|------------------------------------------|------------------------------------|----------------------------------------------------------------|
| `METRIC_AZURERM_API_REQUEST_BUCKETS`     | `1, 2.5, 5, 10, 30, 60, 90, 120`   | Sets buckets for `azurerm_api_request` histogram metric        |
| `METRIC_AZURERM_API_REQUEST_ENABLE`      | `false`                            | Enables/disables `azurerm_api_request_*` metric                |
| `METRIC_AZURERM_API_REQUEST_LABELS`      | `apiEndpoint, method, statusCode`  | Controls labels of `azurerm_api_request_*` metric              |
| `METRIC_AZURERM_API_RATELIMIT_ENABLE`    | `false`                            | Enables/disables `azurerm_api_ratelimit` metric                |
| `METRIC_AZURERM_API_RATELIMIT_AUTORESET` | `false`                            | Enables/disables `azurerm_api_ratelimit` autoreset after fetch |


| `azurerm_api_request` label | Status             | Description                                                                                              |
|-----------------------------|--------------------|----------------------------------------------------------------------------------------------------------|
| `apiEndpoint`               | enabled by default | hostname of endpoint (max 3 parts)                                                                       |
| `routingRegion`             | enabled by default | detected region for API call, either routing region from Azure Management API or Azure resource location |
| `subscriptionID`            | enabled by default | detected subscriptionID                                                                                  |
| `tenantID`                  | enabled by default | detected tenantID (extracted from jwt auth token)                                                        |
| `resourceProvider`          | enabled by default | detected Azure Management API provider                                                                   |
| `method`                    | enabled by default | HTTP method                                                                                              |
| `statusCode`                | enabled by default | HTTP status code                                                                                         |

### Metric name and help template system

(with 21.5.3 and later)

By default Azure monitor metrics are generated with the name specified in the request (see parameter `name`).
This can be modified via environment variable `$METRIC_TEMPLATE` or as request parameter `template`.

HINT: Used templates are removed from labels!

Metric name recommendation: `{name}_{metric}_{aggregation}_{unit}`

Help recommendation: `Azure metrics for {metric} with aggregation {aggregation} as {unit}`

Following templates are available:

| Template        | Description                                                                               |
|-----------------|-------------------------------------------------------------------------------------------|
| `{name}`        | Name of template specified by request parameter `name`                                    |
| `{type}`        | The ResourceType or MetricNamespace specified in the request (not applicable to all APIs) |
| `{metric}`      | Name of Azure monitor metric                                                              |
| `{dimension}`   | Dimension value of Azure monitor metric (if dimension is used)                            |
| `{unit}`        | Unit name of Azure monitor metric (eg `count`, `percent`, ...)                            |
| `{aggregation}` | Aggregation of Azure monitor metric (eg `total`, `average`)                               |
| `{interval}`    | Interval of requested Azure monitor metric                                                |
| `{timespan}`    | Timespan of requested Azure monitor metric                                                |

#### default template

Prometheus config:
```yaml
- job_name: azure-metrics-keyvault
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["azure_metric_keyvault"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    filter: ["resourceType eq 'Microsoft.KeyVault/vaults'"]
    metric:
    - Availability
    - ServiceApiHit
    - ServiceApiLatency
    interval: ["PT15M"]
    timespan: ["PT15M"]
    aggregation:
    - average
    - total
  static_configs:
  - targets: ["azure-metrics:8080"]
```

generated metrics:
```
# HELP azure_metric_keyvault Azure monitor insight metric
# TYPE azure_metric_keyvault gauge
azure_metric_keyvault{aggregation="average",dimension="",interval="PT12H",metric="Availability",resourceID="/subscriptions/...",timespan="PT12H",unit="Percent"} 100
azure_metric_keyvault{aggregation="average",dimension="",interval="PT12H",metric="Availability",resourceID="/subscriptions/...",timespan="PT12H",unit="Percent"} 100
azure_metric_keyvault{aggregation="average",dimension="",interval="PT12H",metric="ServiceApiHit",resourceID="/subscriptions/...",timespan="PT12H",unit="Count"} 0
azure_metric_keyvault{aggregation="average",dimension="",interval="PT12H",metric="ServiceApiHit",resourceID="/subscriptions/...",timespan="PT12H",unit="Count"} 0
azure_metric_keyvault{aggregation="total",dimension="",interval="PT12H",metric="ServiceApiHit",resourceID="/subscriptions/...",timespan="PT12H",unit="Count"} 0
azure_metric_keyvault{aggregation="total",dimension="",interval="PT12H",metric="ServiceApiHit",resourceID="/subscriptions/...",timespan="PT12H",unit="Count"} 0
# HELP azurerm_ratelimit Azure ResourceManager ratelimit
# TYPE azurerm_ratelimit gauge
azurerm_ratelimit{scope="subscription",subscriptionID="...",type="read"} 11997
```


#### template `{name}_{metric}_{unit}`

Prometheus config:
```yaml
- job_name: azure-metrics-keyvault
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["azure_metric_keyvault"]
    template: ["{name}_{metric}_{unit}"]
    help: ["Custom help with {metric}"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    filter: ["resourceType eq 'Microsoft.KeyVault/vaults'"]
    metric:
    - Availability
    - ServiceApiHit
    - ServiceApiLatency
    interval: ["PT15M"]
    timespan: ["PT15M"]
    aggregation:
    - average
    - total
  static_configs:
  - targets: ["azure-metrics:8080"]
```

generated metrics:
```
# HELP azure_metric_keyvault_availability_percent Custom help with availability
# TYPE azure_metric_keyvault_availability_percent gauge
azure_metric_keyvault_availability_percent{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100
azure_metric_keyvault_availability_percent{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100

# HELP azure_metric_keyvault_serviceapihit_count Custom help with serviceapihit
# TYPE azure_metric_keyvault_serviceapihit_count gauge
azure_metric_keyvault_serviceapihit_count{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count{aggregation="total",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count{aggregation="total",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0

# HELP azurerm_ratelimit Azure ResourceManager ratelimit
# TYPE azurerm_ratelimit gauge
azurerm_ratelimit{scope="subscription",subscriptionID="...",type="read"} 11996
```

#### template `{name}_{metric}_{aggregation}_{unit}`

Prometheus config:
```yaml
- job_name: azure-metrics-keyvault
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["azure_metric_keyvault"]
    template: ["{name}_{metric}_{aggregation}_{unit}"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    filter: ["resourceType eq 'Microsoft.KeyVault/vaults'"]
    metric:
    - Availability
    - ServiceApiHit
    - ServiceApiLatency
    interval: ["PT15M"]
    timespan: ["PT15M"]
    aggregation:
    - average
    - total
  static_configs:
  - targets: ["azure-metrics:8080"]
```

generated metrics:
```
# HELP azure_metric_keyvault_availability_average_percent Azure monitor insight metric
# TYPE azure_metric_keyvault_availability_average_percent gauge
azure_metric_keyvault_availability_average_percent{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100
azure_metric_keyvault_availability_average_percent{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100
# HELP azure_metric_keyvault_availability_total_percent Azure monitor insight metric
# TYPE azure_metric_keyvault_availability_total_percent gauge
azure_metric_keyvault_availability_total_percent{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 9
# HELP azure_metric_keyvault_serviceapihit_average_count Azure monitor insight metric
# TYPE azure_metric_keyvault_serviceapihit_average_count gauge
azure_metric_keyvault_serviceapihit_average_count{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_average_count{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 1
# HELP azure_metric_keyvault_serviceapihit_total_count Azure monitor insight metric
# TYPE azure_metric_keyvault_serviceapihit_total_count gauge
azure_metric_keyvault_serviceapihit_total_count{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_total_count{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 9
# HELP azure_metric_keyvault_serviceapilatency_average_milliseconds Azure monitor insight metric
# TYPE azure_metric_keyvault_serviceapilatency_average_milliseconds gauge
azure_metric_keyvault_serviceapilatency_average_milliseconds{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 38.666666666666664
# HELP azure_metric_keyvault_serviceapilatency_total_milliseconds Azure monitor insight metric
# TYPE azure_metric_keyvault_serviceapilatency_total_milliseconds gauge
azure_metric_keyvault_serviceapilatency_total_milliseconds{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 348
# HELP azurerm_ratelimit Azure ResourceManager ratelimit
# TYPE azurerm_ratelimit gauge
azurerm_ratelimit{scope="subscription",subscriptionID="...",type="read"} 11999
```

## HTTP Endpoints

| Endpoint                      | Description                                                                                            |
|-------------------------------|--------------------------------------------------------------------------------------------------------|
| `/metrics`                    | Default prometheus golang metrics                                                                      |
| `/probe/metrics/resource`     | Probe metrics for one resource (see `azurerm_resource_metric`)                                         |
| `/probe/metrics/list`         | Probe metrics for list of resources (see `azurerm_resource_metric`)                                    |
| `/probe/metrics/scrape`       | Probe metrics for list of resources and config on resource by tag name (see `azurerm_resource_metric`) |
| `/probe/metrics/resourcegraph`        | Probe metrics for list of resources based on a kusto query and the resource graph API                  |

### /probe/metrics/resource parameters


| GET parameter          | Default                   | Required | Multiple | Description                                                          |
|------------------------|---------------------------|----------|----------|----------------------------------------------------------------------|
| `subscription`         |                           | **yes**  | **yes**  | Azure Subscription ID                                                |
| `target`               |                           | **yes**  | **yes**  | Azure Resource URI                                                   |
| `timespan`             | `PT1M`                    | no       | no       | Metric timespan                                                      |
| `interval`             |                           | no       | no       | Metric timespan                                                      |
| `metricNamespace`      |                           | no       | **yes**  | Metric namespace                  |
| `metric`               |                           | no       | **yes**  | Metric name                                                          |
| `aggregation`          |                           | no       | **yes**  | Metric aggregation (`minimum`, `maximum`, `average`, `total`, `count`, multiple possible separated with `,`) |
| `name`                 | `azurerm_resource_metric` | no       | no       | Prometheus metric name                                               |
| `metricFilter`         |                           | no       | no       | Prometheus metric filter (dimension support)                         |
| `metricTop`            |                           | no       | no       | Prometheus metric dimension count (dimension support)                |
| `metricOrderBy`        |                           | no       | no       | Prometheus metric order by (dimension support)                       |
| `cache`                | (same as timespan)        | no       | no       | Use of internal metrics caching                                      |
| `template`             | set to `$METRIC_TEMPLATE` | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |
| `help`                 | set to `$METRIC_HELP`     | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

### /probe/metrics/list parameters

HINT: service discovery information is cached for duration set by `$AZURE_SERVICEDISCOVERY_CACHE` (set to `0` to disable)

| GET parameter              | Default                   | Required | Multiple | Description                                                          |
|----------------------------|---------------------------|----------|----------|----------------------------------------------------------------------|
| `subscription`             |                           | **yes**  | **yes**  | Azure Subscription ID (or multiple separate by comma)                |
| `resourceType` or `filter` |                           | **yes**  | no       | Azure Resource type or filter query (https://docs.microsoft.com/en-us/rest/api/resources/resources/list) |
| `timespan`                 | `PT1M`                    | no       | no       | Metric timespan                                                      |
| `interval`                 |                           | no       | no       | Metric timespan                                                      |
| `metricNamespace`          |                           | no       | **yes**  | Metric namespace                  |
| `metric`                   |                           | no       | **yes**  | Metric name                                                          |
| `aggregation`              |                           | no       | **yes**  | Metric aggregation (`minimum`, `maximum`, `average`, `total`, `count`, multiple possible separated with `,`) |
| `name`                     | `azurerm_resource_metric` | no       | no       | Prometheus metric name                                               |
| `metricFilter`             |                           | no       | no       | Prometheus metric filter (dimension support)                         |
| `metricTop`                |                           | no       | no       | Prometheus metric dimension count (dimension support)                |
| `metricOrderBy`            |                           | no       | no       | Prometheus metric order by (dimension support)                       |
| `cache`                    | (same as timespan)        | no       | no       | Use of internal metrics caching                                      |
| `template`                 | set to `$METRIC_TEMPLATE` | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |
| `help`                     | set to `$METRIC_HELP`     | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

### /probe/metrics/scrape parameters

HINT: service discovery information is cached for duration set by `$AZURE_SERVICEDISCOVERY_CACHE` (set to `0` to disable)

| GET parameter               | Default                   | Required | Multiple | Description                                                          |
|----------------------------|---------------------------|----------|----------|----------------------------------------------------------------------|
| `subscription`             |                           | **yes**  | **yes**  | Azure Subscription ID  (or multiple separate by comma)               |
| `resourceType` or `filter` |                           | **yes**  | no       | Azure Resource type or filter query (https://docs.microsoft.com/en-us/rest/api/resources/resources/list) |
| `metricTagName`            |                           | **yes**  | no       | Resource tag name for getting "metrics" list                         |
| `aggregationTagName`       |                           | **yes**  | no       | Resource tag name for getting "aggregations" list                    |
| `timespan`                 | `PT1M`                    | no       | no       | Metric timespan                                                      |
| `interval`                 |                           | no       | no       | Metric timespan                                                      |
| `metricNamespace`          |                           | no       | **yes**  | Metric namespace                  |
| `metric`                   |                           | no       | **yes**  | Metric name                                                          |
| `aggregation`              |                           | no       | **yes**  | Metric aggregation (`minimum`, `maximum`, `average`, `total`, multiple possible separated with `,`)  |
| `name`                     | `azurerm_resource_metric` | no       | no       | Prometheus metric name                                               |
| `metricFilter`             |                           | no       | no       | Prometheus metric filter (dimension support)                         |
| `metricTop`                |                           | no       | no       | Prometheus metric dimension count (integer, dimension support)       |
| `metricOrderBy`            |                           | no       | no       | Prometheus metric order by (dimension support)                       |
| `cache`                    | (same as timespan)        | no       | no       | Use of internal metrics caching                                      |
| `template`                 | set to `$METRIC_TEMPLATE` | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |
| `help`                 | set to `$METRIC_HELP`     | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*


### /probe/metrics/resourcegraph parameters

This endpoint is using Azure ResoruceGraph API for servicediscovery (with 21.9.0 and later)

HINT: service discovery information is cached for duration set by `$AZURE_SERVICEDISCOVERY_CACHE` (set to `0` to disable)

| GET parameter              | Default                   | Required | Multiple | Description                                                          |
|----------------------------|---------------------------|----------|----------|----------------------------------------------------------------------|
| `subscription`             |                           | **yes**  | **yes**  | Azure Subscription ID (or multiple separate by comma)                |
| `resourceType`             |                           | **yes**  | no       | Azure Resource type                                                  |
| `filter`                   |                           | no       | no       | Additional Kusto query part (eg. `where id contains "/xzy/"`)        |
| `timespan`                 | `PT1M`                    | no       | no       | Metric timespan                                                      |
| `interval`                 |                           | no       | no       | Metric timespan                                                      |
| `metricNamespace`          |                           | no       | **yes**  | Metric namespace                  |
| `metric`                   |                           | no       | **yes**  | Metric name                                                          |
| `aggregation`              |                           | no       | **yes**  | Metric aggregation (`minimum`, `maximum`, `average`, `total`, `count`, multiple possible separated with `,`) |
| `name`                     | `azurerm_resource_metric` | no       | no       | Prometheus metric name                                               |
| `metricFilter`             |                           | no       | no       | Prometheus metric filter (dimension support)                         |
| `metricTop`                |                           | no       | no       | Prometheus metric dimension count (dimension support)                |
| `metricOrderBy`            |                           | no       | no       | Prometheus metric order by (dimension support)                       |
| `cache`                    | (same as timespan)        | no       | no       | Use of internal metrics caching                                      |
| `template`                 | set to `$METRIC_TEMPLATE` | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |
| `help`                     | set to `$METRIC_HELP`     | no       | no       | see [metric name and help template system](#metric-name-and-help-template-system)      |

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

## Prometheus configuration examples

### Redis

using target (single instances):

```yaml
- job_name: azure-metrics-redis
  scrape_interval: 1m
  metrics_path: /probe/metrics/resource
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    target:
    - /subscriptions/.../resourceGroups/.../providers/Microsoft.Cache/Redis/...
    - /subscriptions/.../resourceGroups/.../providers/Microsoft.Cache/Redis/...
    - /subscriptions/.../resourceGroups/.../providers/Microsoft.Cache/Redis/...
    - /subscriptions/.../resourceGroups/.../providers/Microsoft.Cache/Redis/...
    metric:
    - connectedclients
    - totalcommandsprocessed
    - cachehits
    - cachemisses
    - getcommands
    - setcommands
    - operationsPerSecond
    - evictedkeys
    - totalkeys
    - expiredkeys
    - usedmemory
    - usedmemorypercentage
    - usedmemoryRss
    - serverLoad
    - cacheWrite
    - cacheRead
    - percentProcessorTime
    - cacheLatency
    - errors
    interval: ["PT1M"]
    timespan: ["PT1M"]
    aggregation:
    - average
    - total
  static_configs:
  - targets: ["azure-metrics:8080"]
```

using ServiceDiscovery:
```yaml
- job_name: azure-metrics-redis
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    resourceType: ["Microsoft.Cache/Redis"]
    metric:
    - connectedclients
    - totalcommandsprocessed
    - cachehits
    - cachemisses
    - getcommands
    - setcommands
    - operationsPerSecond
    - evictedkeys
    - totalkeys
    - expiredkeys
    - usedmemory
    - usedmemorypercentage
    - usedmemoryRss
    - serverLoad
    - cacheWrite
    - cacheRead
    - percentProcessorTime
    - cacheLatency
    - errors
    interval: ["PT1M"]
    timespan: ["PT1M"]
    aggregation:
    - average
    - total
  static_configs:
  - targets: ["azure-metrics:8080"]
```

using ServiceDiscovery with custom resource filter query:
```yaml
- job_name: azure-metrics-redis
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    filter: ["resourceType eq 'Microsoft.Cache/Redis'"]
    metric:
    - connectedclients
    - totalcommandsprocessed
    - cachehits
    - cachemisses
    - getcommands
    - setcommands
    - operationsPerSecond
    - evictedkeys
    - totalkeys
    - expiredkeys
    - usedmemory
    - usedmemorypercentage
    - usedmemoryRss
    - serverLoad
    - cacheWrite
    - cacheRead
    - percentProcessorTime
    - cacheLatency
    - errors
    interval: ["PT1M"]
    timespan: ["PT1M"]
    aggregation:
    - average
    - total
  static_configs:
  - targets: ["azure-metrics:8080"]
```
### VirtualNetworkGateways

```yaml
- job_name: azure-metrics-virtualNetworkGateways
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    resourceType: ["Microsoft.Network/virtualNetworkGateways"]
    metric:
    - AverageBandwidth
    - P2SBandwidth
    - P2SConnectionCount
    - TunnelAverageBandwidth
    - TunnelEgressBytes
    - TunnelIngressBytes
    - TunnelEgressPackets
    - TunnelIngressPackets
    - TunnelEgressPacketDropTSMismatch
    - TunnelIngressPacketDropTSMismatch
    interval: ["PT5M"]
    timespan: ["PT5M"]
    aggregation:
    - average
    - total
  static_configs:
  - targets: ["azure-metrics:8080"]
```

### virtualNetworkGateway connections (dimension support)

Virtual Gateway connection metrics (dimension support)
```yaml
- job_name: azure-metrics-virtualNetworkGateways-connections
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    resourceType: ["Microsoft.Network/virtualNetworkGateways"]
    metric:
    - TunnelAverageBandwidth
    - TunnelEgressBytes
    - TunnelIngressBytes
    - TunnelEgressPackets
    - TunnelIngressPackets
    - TunnelEgressPacketDropTSMismatch
    - TunnelIngressPacketDropTSMismatch
    interval: ["PT5M"]
    timespan: ["PT5M"]
    aggregation:
    - average
    - total
    # by connection (dimension support)
    metricFilter: ["ConnectionName eq '*'"]
    metricTop: ["10"]
  static_configs:
  - targets: ["azure-metrics:8080"]
```

### StorageAccount (metric namespace and dimension support)

Virtual Gateway connection metrics (dimension support)
```yaml
- job_name: azure-metrics-virtualNetworkGateways-connections
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    resourceType: ["Microsoft.Storage/storageAccounts"]
    metricNamespace: ["Microsoft.Storage/storageAccounts/blobServices"]
    metric:
    - BlobCapacity
    interval: ["PT1H"]
    timespan: ["PT1H"]
    aggregation:
    - average
    - count
    # by blobtype (dimension support)
    metricFilter: ["BlobType eq '*'"]
    metricTop: ["10"]
  static_configs:
  - targets: ["azure-metrics:8080"]
```

In these examples all metrics are published with metric name `my_own_metric_name`.

The [List of supported metrics](https://docs.microsoft.com/en-us/azure/azure-monitor/platform/metrics-supported) is available in the Microsoft Azure docs.

### Development and testing query webui

(with 21.10.0-beta1 and later)

if azure-metrics-exporter is started with `--development.webui` there is a webui at `http://url-to-exporter/query`.
Here you can test different query settings and get the generated prometheus scrape_config.
