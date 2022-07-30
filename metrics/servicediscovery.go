package metrics

import (
	"context"
	"crypto/sha1" // #nosec G505
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	resourcegraph "github.com/Azure/azure-sdk-for-go/services/resourcegraph/mgmt/2019-04-01/resourcegraph"
	"github.com/webdevops/go-common/utils/to"
)

const (
	ResourceGraphQueryTop = 1000
)

type (
	AzureServiceDiscovery struct {
		prober *MetricProber
	}

	AzureResource struct {
		ID       string
		Location string
		Tags     map[string]string
	}
)

func (sd *AzureServiceDiscovery) ResourcesClient(subscriptionId string) *resources.Client {
	client := resources.NewClientWithBaseURI(sd.prober.AzureClient.Environment.ResourceManagerEndpoint, subscriptionId)
	sd.prober.decorateAzureAutoRest(&client.BaseClient.Client)

	return &client
}

func (sd *AzureServiceDiscovery) publishTargetList(targetList []MetricProbeTarget) {
	sd.prober.AddTarget(targetList...)
}

func (sd *AzureServiceDiscovery) fetchResourceList(subscriptionId, filter string) (resourceList []AzureResource, err error) {
	cacheKey := fmt.Sprintf(
		"%x",
		string(sha1.New().Sum([]byte(fmt.Sprintf("%v:%v", subscriptionId, filter)))),
	) // #nosec G401

	// try to fetch info from cache
	if cachedResourceList, ok := sd.fetchFromCache(cacheKey); !ok {
		list, azureErr := sd.ResourcesClient(subscriptionId).ListComplete(context.Background(), filter, "", nil)
		if azureErr != nil {
			err = fmt.Errorf("servicediscovery failed: %s", azureErr.Error())
			return
		}

		for list.NotDone() {
			val := list.Value()

			resourceList = append(
				resourceList,
				AzureResource{
					ID:   to.String(val.ID),
					Tags: to.StringMap(val.Tags),
				},
			)

			if list.NextWithContext(sd.prober.ctx) != nil {
				break
			}
		}

		// store to cache (if enabled)
		sd.saveToCache(cacheKey, resourceList)
	} else {
		sd.prober.logger.Debugf("using servicediscovery from cache")
		resourceList = cachedResourceList
	}

	return
}

func (sd *AzureServiceDiscovery) fetchFromCache(cacheKey string) (resourceList []AzureResource, status bool) {
	contextLogger := sd.prober.logger
	cache := sd.prober.serviceDiscoveryCache.cache

	if cache != nil {
		if v, ok := cache.Get(cacheKey); ok {
			if cacheData, ok := v.([]byte); ok {
				if err := json.Unmarshal(cacheData, &resourceList); err == nil {
					status = true
				} else {
					contextLogger.Debug("unable to parse cached servicediscovery")
				}
			}
		}
	}

	return
}

func (sd *AzureServiceDiscovery) saveToCache(cacheKey string, resourceList []AzureResource) {
	contextLogger := sd.prober.logger
	cache := sd.prober.serviceDiscoveryCache.cache
	cacheDuration := sd.prober.serviceDiscoveryCache.cacheDuration

	// store to cache (if enabled)
	if cache != nil {
		contextLogger.Debug("saving servicedisccovery to cache")
		if cacheData, err := json.Marshal(resourceList); err == nil {
			cache.Set(cacheKey, cacheData, *cacheDuration)
			contextLogger.Debugf("saved servicediscovery to cache for %s", cacheDuration.String())
		}
	}
}

func (sd *AzureServiceDiscovery) FindSubscriptionResources(subscriptionId, filter string) {
	var targetList []MetricProbeTarget

	if resourceList, err := sd.fetchResourceList(subscriptionId, filter); err == nil {
		for _, resource := range resourceList {
			targetList = append(
				targetList,
				MetricProbeTarget{
					ResourceId:   resource.ID,
					Metrics:      sd.prober.settings.Metrics,
					Aggregations: sd.prober.settings.Aggregations,
					Tags:         resource.Tags,
				},
			)
		}
	} else {
		sd.prober.logger.Error(err)
		return
	}

	sd.publishTargetList(targetList)
}

func (sd *AzureServiceDiscovery) FindSubscriptionResourcesWithScrapeTags(ctx context.Context, subscriptionId, filter, metricTagName, aggregationTagName string) {
	var targetList []MetricProbeTarget

	if resourceList, err := sd.fetchResourceList(subscriptionId, filter); err == nil {
		for _, resource := range resourceList {
			if metrics, ok := resource.Tags[metricTagName]; ok && metrics != "" {
				if aggregations, ok := resource.Tags[aggregationTagName]; ok && aggregations != "" {
					targetList = append(
						targetList,
						MetricProbeTarget{
							ResourceId:   resource.ID,
							Metrics:      stringToStringList(metrics, ","),
							Aggregations: stringToStringList(aggregations, ","),
						},
					)

				}
			}
		}
	} else {
		sd.prober.logger.Error(err)
		return
	}

	sd.publishTargetList(targetList)
}

func (sd *AzureServiceDiscovery) FindResourceGraph(ctx context.Context, subscriptions []string, resourceType, filter string) {
	var targetList []MetricProbeTarget

	// Create and authorize a ResourceGraph client
	resourcegraphClient := resourcegraph.NewWithBaseURI(sd.prober.AzureClient.Environment.ResourceManagerEndpoint)
	sd.prober.decorateAzureAutoRest(&resourcegraphClient.Client)

	if filter != "" {
		filter = "| " + filter
	}

	queryTemplate := `
Resources
| where type =~ "%s"
%s
| project id, tags
`

	query := strings.TrimSpace(fmt.Sprintf(
		queryTemplate,
		strings.ReplaceAll(resourceType, "'", "\\'"),
		filter,
	))

	sd.prober.logger.WithField("query", query).Debugf("using Kusto query")

	requestQueryTop := int32(ResourceGraphQueryTop)
	requestQuerySkip := int32(0)

	// Set options
	RequestOptions := resourcegraph.QueryRequestOptions{
		ResultFormat: "objectArray",
		Top:          &requestQueryTop,
		Skip:         &requestQuerySkip,
	}

	// Run the query and get the results
	resultTotalRecords := int32(0)
	for {
		// Create the query request
		Request := resourcegraph.QueryRequest{
			Subscriptions: &subscriptions,
			Query:         &query,
			Options:       &RequestOptions,
		}

		var results, queryErr = resourcegraphClient.Resources(ctx, Request)
		if results.TotalRecords != nil {
			resultTotalRecords = int32(*results.TotalRecords)
		}

		if queryErr == nil {
			if resultList, ok := results.Data.([]interface{}); ok {
				// check if we got data, otherwise break the for loop
				if len(resultList) == 0 {
					break
				}

				for _, v := range resultList {
					if resultRow, ok := v.(map[string]interface{}); ok {
						if val, ok := resultRow["id"]; ok && val != "" {
							if resourceId, ok := val.(string); ok {
								targetList = append(
									targetList,
									MetricProbeTarget{
										ResourceId:   resourceId,
										Metrics:      sd.prober.settings.Metrics,
										Aggregations: sd.prober.settings.Aggregations,
										Tags:         sd.resourceTagsToStringMap(resultRow["tags"]),
									},
								)
							}
						}
					}
				}
			}
		}

		*RequestOptions.Skip += requestQueryTop
		if *RequestOptions.Skip >= resultTotalRecords {
			break
		}
	}

	sd.publishTargetList(targetList)
}

func (sd *AzureServiceDiscovery) resourceTagsToStringMap(tags interface{}) (ret map[string]string) {
	ret = map[string]string{}

	switch tagMap := tags.(type) {
	case map[string]interface{}:
		for tag, value := range tagMap {
			switch v := value.(type) {
			case string:
				ret[tag] = v
			case *string:
				ret[tag] = to.String(v)
			}
		}
	case map[string]string:
		ret = tagMap
	case map[string]*string:
		ret = to.StringMap(tagMap)
	case map[*string]*string:
		for tag, value := range tagMap {
			ret[to.String(tag)] = to.String(value)
		}
	}

	return ret
}
