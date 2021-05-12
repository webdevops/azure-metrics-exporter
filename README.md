# Azure Insights metrics exporter

[![license](https://img.shields.io/github/license/webdevops/azure-metrics-exporter.svg)](https://github.com/webdevops/azure-metrics-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fazure--metrics--exporter-blue)](https://hub.docker.com/r/webdevops/azure-metrics-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fazure--metrics--exporter-blue)](https://quay.io/repository/webdevops/azure-metrics-exporter)

Prometheus exporter for Azure Insights metrics (on demand).
Supports metrics fetching from all resource with one scrape (automatic service discovery) and also supports dimensions.

Configuration (except Azure connection) of this exporter is made entirely in Prometheus instead of a seperate configuration file, see examples below.

WARNING: LogAnalytics metrics are deprecated, please migrate to [azure-loganalytics-exporter](https://github.com/webdevops/azure-loganalytics-exporter)

TOC:
* [Configuration](#configuration)
* [Metrics](#metrics)
    + [Metric name template system](#metric-name-template-system)
        - [default template](#default-template)
        - [template `{name}_{metric}_{unit}`](#template-name_metric_unit)
        - [template `{name}_{metric}_{unit}_{aggregation}`](#template-name_metric_unit_aggregation)
* [HTTP Endpoints](#http-endpoints)
    + [/probe/metrics/resource parameters](#probemetricsresource-parameters)
    + [/probe/metrics/list parameters](#probemetricslist-parameters)
    + [/probe/metrics/scrape parameters](#probemetricsscrape-parameters)
    + [/probe/loganalytics/query parameters **deprecated**](#probeloganalyticsquery-parameters-deprecated)
* [Prometheus configuration](#prometheus-configuration)

## Configuration

Normally no configuration is needed but can be customized using environment variables.

```
Usage:
  azure-metrics-exporter [OPTIONS]

Application Options:
      --debug                              debug mode [$DEBUG]
  -v, --verbose                            verbose mode [$VERBOSE]
      --log.json                           Switch log output to json format [$LOG_JSON]
      --azure-environment=                 Azure environment name (default: AZUREPUBLICCLOUD)
                                           [$AZURE_ENVIRONMENT]
      --azure-ad-resource-url=             Specifies the AAD resource ID to use. If not set, it defaults to
                                           ResourceManagerEndpoint for operations with Azure Resource Manager
                                           [$AZURE_AD_RESOURCE]
      --azure.servicediscovery.cache=      Duration for caching Azure ServiceDiscovery of workspaces to reduce
                                           API calls (time.Duration) (default: 30m)
                                           [$AZURE_SERVICEDISCOVERY_CACHE]
      --metrics.template=                  Template for metric name (default: {name}) [$METRIC_TEMPLATE]
      --concurrency.subscription=          Concurrent subscription fetches (default: 5)
                                           [$CONCURRENCY_SUBSCRIPTION]
      --concurrency.subscription.resource= Concurrent requests per resource (inside subscription requests)
                                           (default: 10) [$CONCURRENCY_SUBSCRIPTION_RESOURCE]
      --enable-caching                     Enable internal caching [$ENABLE_CACHING]
      --bind=                              Server address (default: :8080) [$SERVER_BIND]

Help Options:
  -h, --help                               Show this help message
```

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

## Metrics

| Metric                                   | Description                                                                    |
|------------------------------------------|--------------------------------------------------------------------------------|
| `azurerm_stats_metric_collecttime`       | General exporter stats                                                         |
| `azurerm_stats_metric_requests`          | Counter of resource metric requests with result (error, success)               |
| `azurerm_resource_metric` (customizable) | Resource metrics exported by probes (can be changed using `name` parameter and template system) |
| `azurerm_loganalytics_query_result`      | LogAnalytics rows exported by probes                                           |
| `azurerm_ratelimit`                      | Azure ratelimit metric (only available for uncached /probe requests)           |

### Metric name template system

(with 21.5.3 and later)

By default Azure monitor metrics are generated with the name specified in the request (see parameter `name`).
This can be modified via environment variable `$METRIC_TEMPLATE` or as request parameter `template`.

HINT: Used templates are removed from labels!

Following templates are available:

| Template         |  Description                                                                                      |
|------------------|---------------------------------------------------------------------------------------------------|
| `{name}`         | Name of template specified by request parameter `name`                                            |
| `{metric}`       | Name of Azure monitor metric                                                                      |
| `{dimension}`    | Dimension value of Azure monitor metric (if dimension is used)                                    |
| `{unit}`         | Unit name of Azure monitor metric (eg `count`, `percent`, ...)                                    |
| `{aggregation}`  | Aggregation of Azure monitor metric (eg `total`, `average`)                                       |
| `{interval}`     | Interval of requested Azure monitor metric                                                        |
| `{timespan}`     | Timespan of requested Azure monitor metric                                                        |

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
# HELP azure_metric_keyvault_availability_percent Azure monitor insight metric
# TYPE azure_metric_keyvault_availability_percent gauge
azure_metric_keyvault_availability_percent{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100
azure_metric_keyvault_availability_percent{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100

# HELP azure_metric_keyvault_serviceapihit_count Azure monitor insight metric
# TYPE azure_metric_keyvault_serviceapihit_count gauge
azure_metric_keyvault_serviceapihit_count{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count{aggregation="average",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count{aggregation="total",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count{aggregation="total",dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0

# HELP azurerm_ratelimit Azure ResourceManager ratelimit
# TYPE azurerm_ratelimit gauge
azurerm_ratelimit{scope="subscription",subscriptionID="...",type="read"} 11996
```

#### template `{name}_{metric}_{unit}_{aggregation}`

Prometheus config:
```yaml
- job_name: azure-metrics-keyvault
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["azure_metric_keyvault"]
    template: ["{name}_{metric}_{unit}_{aggregation}"]
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
# HELP azure_metric_keyvault_availability_percent_average Azure monitor insight metric
# TYPE azure_metric_keyvault_availability_percent_average gauge
azure_metric_keyvault_availability_percent_average{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100
azure_metric_keyvault_availability_percent_average{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 100
# HELP azure_metric_keyvault_serviceapihit_count_average Azure monitor insight metric
# TYPE azure_metric_keyvault_serviceapihit_count_average gauge
azure_metric_keyvault_serviceapihit_count_average{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count_average{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
# HELP azure_metric_keyvault_serviceapihit_count_total Azure monitor insight metric
# TYPE azure_metric_keyvault_serviceapihit_count_total gauge
azure_metric_keyvault_serviceapihit_count_total{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
azure_metric_keyvault_serviceapihit_count_total{dimension="",interval="PT12H",resourceID="/subscriptions/...",timespan="PT12H"} 0
# HELP azurerm_ratelimit Azure ResourceManager ratelimit
# TYPE azurerm_ratelimit gauge
azurerm_ratelimit{scope="subscription",subscriptionID="...",type="read"} 11992
```

## HTTP Endpoints

| Endpoint                       | Description                                                                         |
|--------------------------------|-------------------------------------------------------------------------------------|
| `/metrics`                     | Default prometheus golang metrics                                                   |
| `/probe/metrics/resource`      | Probe metrics for one resource (see `azurerm_resource_metric`)                      |
| `/probe/metrics/list`          | Probe metrics for list of resources (see `azurerm_resource_metric`)                 |
| `/probe/metrics/scrape`        | Probe metrics for list of resources and config on resource by tag name (see `azurerm_resource_metric`) |
| `/probe/loganalytics/query`    | **deprecated** Probe metrics from LogAnalytics query (see `azurerm_loganalytics_query_result`)   |


### /probe/metrics/resource parameters


| GET parameter          | Default                   | Required | Multiple | Description                                                          |
|------------------------|---------------------------|----------|----------|----------------------------------------------------------------------|
| `subscription`         |                           | **yes**  | **yes**  | Azure Subscription ID                                                |
| `target`               |                           | **yes**  | **yes**  | Azure Resource URI                                                   |
| `timespan`             | `PT1M`                    | no       | no       | Metric timespan                                                      |
| `interval`             |                           | no       | no       | Metric timespan                                                      |
| `metric`               |                           | no       | **yes**  | Metric name                                                          |
| `aggregation`          |                           | no       | **yes**  | Metric aggregation (`minimum`, `maximum`, `average`, `total`, `count`, multiple possible separated with `,`) |
| `name`                 | `azurerm_resource_metric` | no       | no       | Prometheus metric name                                               |
| `metricFilter`         |                           | no       | no       | Prometheus metric filter (dimension support)                         |
| `metricTop`            |                           | no       | no       | Prometheus metric dimension count (dimension support)                |
| `metricOrderBy`        |                           | no       | no       | Prometheus metric order by (dimension support)                       |
| `cache`                | (same as timespan)        | no       | no       | Use of internal metrics caching                                      |
| `template`             | set to `$METRIC_TEMPLATE` | no       | no       | see [metric name template system](#metric-name-template-system)      |

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

### /probe/metrics/list parameters

HINT: service discovery information is cached for duration set by `$AZURE_SERVICEDISCOVERY_CACHE` (set to `0` to disable)

| GET parameter          | Default                   | Required | Multiple | Description                                                          |
|------------------------|---------------------------|----------|----------|----------------------------------------------------------------------|
| `subscription`         |                           | **yes**  | **yes**  | Azure Subscription ID (or multiple separate by comma)                |
| `filter`               |                           | **yes**  | no       | Azure Resource filter (https://docs.microsoft.com/en-us/rest/api/resources/resources/list)                                              |
| `timespan`             | `PT1M`                    | no       | no       | Metric timespan                                                      |
| `interval`             |                           | no       | no       | Metric timespan                                                      |
| `metric`               |                           | no       | **yes**  | Metric name                                                          |
| `aggregation`          |                           | no       | **yes**  | Metric aggregation (`minimum`, `maximum`, `average`, `total`, `count`, multiple possible separated with `,`) |
| `name`                 | `azurerm_resource_metric` | no       | no       | Prometheus metric name                                               |
| `metricFilter`         |                           | no       | no       | Prometheus metric filter (dimension support)                         |
| `metricTop`            |                           | no       | no       | Prometheus metric dimension count (dimension support)                |
| `metricOrderBy`        |                           | no       | no       | Prometheus metric order by (dimension support)                       |
| `cache`                | (same as timespan)        | no       | no       | Use of internal metrics caching                                      |
| `template`             | set to `$METRIC_TEMPLATE` | no       | no       | see [metric name template system](#metric-name-template-system)      |

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

### /probe/metrics/scrape parameters

HINT: service discovery information is cached for duration set by `$AZURE_SERVICEDISCOVERY_CACHE` (set to `0` to disable)

| GET parameter          | Default                   | Required | Multiple | Description                                                          |
|------------------------|---------------------------|----------|----------|----------------------------------------------------------------------|
| `subscription`         |                           | **yes**  | **yes**  | Azure Subscription ID  (or multiple separate by comma)               |
| `filter`               |                           | **yes**  | no       | Azure Resource filter (https://docs.microsoft.com/en-us/rest/api/resources/resources/list)                                              |
| `metricTagName`        |                           | **yes**  | no       | Resource tag name for getting "metric" list                                                                                             |
| `aggregationTagName`   |                           | **yes**  | no       | Resource tag name for getting "aggregation" list                     |
| `timespan`             | `PT1M`                    | no       | no       | Metric timespan                                                      |
| `interval`             |                           | no       | no       | Metric timespan                                                      |
| `metric`               |                           | no       | **yes**  | Metric name                                                          |
| `aggregation`          |                           | no       | **yes**  | Metric aggregation (`minimum`, `maximum`, `average`, `total`, multiple possible separated with `,`)        |
| `name`                 | `azurerm_resource_metric` | no       | no       | Prometheus metric name                                               |
| `metricFilter`         |                           | no       | no       | Prometheus metric filter (dimension support)                         |
| `metricTop`            |                           | no       | no       | Prometheus metric dimension count (integer, dimension support)       |
| `metricOrderBy`        |                           | no       | no       | Prometheus metric order by (dimension support)                       |
| `cache`                | (same as timespan)        | no       | no       | Use of internal metrics caching                                      |
| `template`             | set to `$METRIC_TEMPLATE` | no       | no       | see [metric name template system](#metric-name-template-system)      |

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

### /probe/loganalytics/query parameters **deprecated**

WARNING: LogAnalytics metrics are deprecated, please migrate to [azure-loganalytics-exporter](https://github.com/webdevops/azure-loganalytics-exporter)

| GET parameter          | Default   | Required | Description                                                          |
|------------------------|-----------|----------|----------------------------------------------------------------------|
| `workspace   `         |           | **yes**  | Azure LogAnalytics workspace ID                                      |
| `query`                |           | **yes**  | LogAnalytics query                                                   |
| `timespan`             |           | **yes**  | Query timespan                                                       |


## Prometheus configuration

Azure Redis metrics

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

Virtual Gateway metrics
```yaml
- job_name: azure-metrics-virtualNetworkGateways
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    filter: ["resourceType eq 'Microsoft.Network/virtualNetworkGateways'"]
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

Virtual Gateway connection metrics (dimension support)
```yaml
- job_name: azure-metrics-virtualNetworkGateways-connections
  scrape_interval: 1m
  metrics_path: /probe/metrics/list
  params:
    name: ["my_own_metric_name"]
    subscription:
    - xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
    filter: ["resourceType eq 'Microsoft.Network/virtualNetworkGateways'"]
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

In these examples all metrics are published with metric name `my_own_metric_name`.

The [List of supported metrics](https://docs.microsoft.com/en-us/azure/azure-monitor/platform/metrics-supported) is available in the Microsoft Azure docs.
