package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

type (
	AzureInsightBaseMetricsResult struct {
		settings *RequestMetricSettings
	}
)

func (r *AzureInsightBaseMetricsResult) buildMetric(labels prometheus.Labels, value float64) (metric PrometheusMetricResult) {
	// copy map to ensure we don't keep references
	metricLabels := prometheus.Labels{}
	for labelName, labelValue := range labels {
		metricLabels[labelName] = labelValue
	}

	metric = PrometheusMetricResult{
		Name:   r.settings.MetricTemplate,
		Labels: metricLabels,
		Value:  value,
	}

	// fallback if template is empty (should not be)
	if r.settings.MetricTemplate == "" {
		metric.Name = r.settings.Name
	}

	resourceType := r.settings.ResourceType
	// MetricNamespace is more descriptive than type
	if r.settings.MetricNamespace != "" {
		resourceType = r.settings.MetricNamespace
	}

	// set help
	metric.Help = r.settings.HelpTemplate
	if metricNamePlaceholders.MatchString(metric.Help) {
		metric.Help = metricNamePlaceholders.ReplaceAllStringFunc(
			metric.Help,
			func(fieldName string) string {
				fieldName = strings.Trim(fieldName, "{}")
				switch fieldName {
				case "name":
					return r.settings.Name
				case "type":
					return resourceType
				default:
					if fieldValue, exists := metric.Labels[fieldName]; exists {
						return fieldValue
					}
				}
				return ""
			},
		)
	}

	if metricNamePlaceholders.MatchString(metric.Name) {
		metric.Name = metricNamePlaceholders.ReplaceAllStringFunc(
			metric.Name,
			func(fieldName string) string {
				fieldName = strings.Trim(fieldName, "{}")
				switch fieldName {
				case "name":
					return r.settings.Name
				case "type":
					return resourceType
				default:
					if fieldValue, exists := metric.Labels[fieldName]; exists {
						// remove label, when we add it to metric name
						delete(metric.Labels, fieldName)
						return fieldValue
					}
				}
				return ""
			},
		)
	}

	// sanitize metric name
	metric.Name = metricNameReplacer.Replace(metric.Name)
	metric.Name = strings.ToLower(metric.Name)
	metric.Name = metricNameNotAllowedChars.ReplaceAllString(metric.Name, "")

	return
}
