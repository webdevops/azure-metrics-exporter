Azure Insights metrics exporter
===============================

[![license](https://img.shields.io/github/license/webdevops/azure-metrics-exporter.svg)](https://github.com/webdevops/azure-metrics-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/docker/cloud/automated/webdevops/azure-metrics-exporter)](https://hub.docker.com/r/webdevops/azure-metrics-exporter/)
[![Docker Build Status](https://img.shields.io/docker/cloud/build/webdevops/azure-metrics-exporter)](https://hub.docker.com/r/webdevops/azure-metrics-exporter/)

Prometheus exporter for Azure Insights metrics (on demand).
Supports metrics fetching from all resource with one scrape (automatic service discovery) and also supports dimensions.

Configuration (except Azure connection) of this exporter is made entirely in Prometheus instead of a seperate configuration file, see examples below.

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

```
Usage:
  azure-metrics-exporter [OPTIONS]

Application Options:
      --debug                              debug mode [$DEBUG]
  -v, --verbose                            verbose mode [$VERBOSE]
      --log.json                           Switch log output to json format [$LOG_JSON]
      --concurrency.subscription=          Concurrent subscription fetches (default: 5) [$CONCURRENCY_SUBSCRIPTION]
      --concurrency.subscription.resource= Concurrent requests per resource (inside subscription requests) (default:
                                           10) [$CONCURRENCY_SUBSCRIPTION_RESOURCE]
      --enable-caching                     Enable internal caching [$ENABLE_CACHING]
      --bind=                              Server address (default: :8080) [$SERVER_BIND]

Help Options:
  -h, --help                               Show this help message
```

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Metrics
-------

| Metric                              | Description                                                                    |
|-------------------------------------|--------------------------------------------------------------------------------|
| `azurerm_stats_metric_collecttime`  | General exporter stats                                                         |
| `azurerm_stats_metric_requests`     | Counter of resource metric requests with result (error, success)               |
| `azurerm_resource_metric`           | Resource metrics exported by probes (can be changed using `name` parameter)    |
| `azurerm_loganalytics_query_result` | LogAnalytics rows exported by probes                                           |


HTTP Endpoints
--------------

| Endpoint                       | Description                                                                         |
|--------------------------------|-------------------------------------------------------------------------------------|
| `/metrics`                     | Default prometheus golang metrics                                                   |
| `/probe/metrics/resource`      | Probe metrics for one resource (see `azurerm_resource_metric`)                      |
| `/probe/metrics/list`          | Probe metrics for list of resources (see `azurerm_resource_metric`)                 |
| `/probe/metrics/scrape`        | Probe metrics for list of resources and config on resource by tag name (see `azurerm_resource_metric`) |
| `/probe/loganalytics/query`    | Probe metrics from LogAnalytics query (see `azurerm_loganalytics_query_result`)     |


#### /probe/metrics/resource parameters


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

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

#### /probe/metrics/list parameters

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

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

#### /probe/metrics/scrape parameters

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

*Hint: Multiple values can be specified multiple times or with a comma in a single value.*

#### /probe/loganalytics/query parameters


| GET parameter          | Default   | Required | Description                                                          |
|------------------------|-----------|----------|----------------------------------------------------------------------|
| `workspace   `         |           | **yes**  | Azure LogAnalytics workspace ID                                      |
| `query`                |           | **yes**  | LogAnalytics query                                                   |
| `timespan`             |           | **yes**  | Query timespan                                                       |


Prometheus configuration
------------------------

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
