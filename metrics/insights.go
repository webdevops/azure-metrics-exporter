package metrics

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/monitor/armmonitor"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/utils/to"
)

var (
	metricNamePlaceholders     = regexp.MustCompile(`{([^}]+)}`)
	metricNameNotAllowedChars  = regexp.MustCompile(`[^a-zA-Z0-9_:]`)
	metricLabelNotAllowedChars = regexp.MustCompile(`[^a-zA-Z0-9_]`)
	metricNameReplacer         = strings.NewReplacer("-", "_", " ", "_", "/", "_", ".", "_")
)

type (
	PrometheusMetricResult struct {
		Name   string
		Labels prometheus.Labels
		Value  float64
		Help   string
	}
)

func (p *MetricProber) MetricsClient(subscriptionId string) (*armmonitor.MetricsClient, error) {
	clientOpts := p.AzureClient.NewArmClientOptions()
	clientOpts.PerCallPolicies = append(
		clientOpts.PerCallPolicies,
		noCachePolicy{},
	)
	return armmonitor.NewMetricsClient(subscriptionId, p.AzureClient.GetCred(), clientOpts)
}

func (p *MetricProber) FetchMetricsFromTarget(client *armmonitor.MetricsClient, target MetricProbeTarget, metrics, aggregations []string) (AzureInsightMetricsResult, error) {
	ret := AzureInsightMetricsResult{
		AzureInsightBaseMetricsResult: AzureInsightBaseMetricsResult{
			prober: p,
		},
		target: &target,
	}

	resultType := armmonitor.ResultTypeData
	opts := armmonitor.MetricsClientListOptions{
		Interval:            p.settings.Interval,
		ResultType:          &resultType,
		Timespan:            to.StringPtr(p.settings.Timespan),
		Metricnames:         to.StringPtr(strings.Join(metrics, ",")),
		Top:                 p.settings.MetricTop,
		AutoAdjustTimegrain: to.BoolPtr(true),
		ValidateDimensions:  to.BoolPtr(p.settings.ValidateDimensions),
	}

	if len(aggregations) >= 1 {
		opts.Aggregation = to.StringPtr(strings.Join(aggregations, ","))
	}

	if len(p.settings.MetricFilter) >= 1 {
		opts.Filter = to.StringPtr(p.settings.MetricFilter)
	}

	if len(p.settings.MetricNamespace) >= 1 {
		opts.Metricnamespace = to.StringPtr(p.settings.MetricNamespace)
	}

	if len(p.settings.MetricOrderBy) >= 1 {
		opts.Orderby = to.StringPtr(p.settings.MetricOrderBy)
	}

	// Apply segment parameter for dimension splitting
	if len(p.settings.MetricSegment) >= 1 {
		segmentClause := fmt.Sprintf("$segment=%s", p.settings.MetricSegment)
		if len(p.settings.MetricFilter) >= 1 {
			// Combine existing filter with segment
			opts.Filter = to.StringPtr(p.settings.MetricFilter + " and " + segmentClause)
		} else {
			// Use segment only
			opts.Filter = to.StringPtr(segmentClause)
		}
	}

	resourceURI := target.ResourceId
	if strings.HasPrefix(strings.ToLower(p.settings.MetricNamespace), "microsoft.storage/storageaccounts/") {
		splitNamespace := strings.Split(p.settings.MetricNamespace, "/")
		// Storage accounts have an extra requirement that their ResourceURI include <type>/default
		storageAccountType := splitNamespace[len(splitNamespace)-1]
		resourceURI = resourceURI + fmt.Sprintf("/%s/default", storageAccountType)
	}

	result, err := client.List(
		p.ctx,
		resourceURI,
		&opts,
	)

	if err == nil {
		ret.Result = &result
	}

	return ret, err
}
