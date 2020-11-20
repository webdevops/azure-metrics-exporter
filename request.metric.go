package main

import (
	iso8601 "github.com/ChannelMeter/iso8601duration"
	log "github.com/sirupsen/logrus"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type (
	RequestMetricSettings struct {
		Name          string
		Subscriptions []string
		Filter        string
		Timespan      string
		Interval      *string
		Metric        []string
		Aggregation   []string
		Target        []string

		// needed for dimension support
		MetricTop     *int32
		MetricFilter  string
		MetricOrderBy string

		// cache
		Cache *time.Duration
	}
)

func NewRequestMetricSettings(r *http.Request) (RequestMetricSettings, error) {
	ret := RequestMetricSettings{}
	params := r.URL.Query()

	// param name
	ret.Name = paramsGetWithDefault(params, "name", PROMETHEUS_METRIC_NAME)

	// param subscription
	if subscriptionList, err := paramsGetListRequired(params, "subscription"); err == nil {
		for _, subscription := range subscriptionList {
			subscription = strings.TrimSpace(subscription)
			ret.Subscriptions = append(ret.Subscriptions, subscription)
		}
	} else {
		return ret, err
	}

	// param filter
	if filter, err := paramsGetRequired(params, "filter"); err == nil {
		ret.Filter = filter
	} else {
		return ret, err
	}

	// param timespan
	ret.Timespan = paramsGetWithDefault(params, "timespan", "PT1M")

	// param interval
	if val := params.Get("interval"); val != "" {
		ret.Interval = &val
	}

	// param metric
	if val, err := paramsGetList(params, "metric"); err == nil {
		ret.Metric = val
	} else {
		return ret, err
	}

	// param aggregation
	if val, err := paramsGetList(params, "aggregation"); err == nil {
		ret.Aggregation = val
	} else {
		return ret, err
	}

	// param target
	if val, err := paramsGetList(params, "target"); err == nil {
		ret.Target = val
	} else {
		return ret, err
	}

	// param metricTop
	if val := params.Get("metricTop"); val != "" {
		valInt64, err := strconv.ParseInt(val, 10, 32)
		if err != nil {
			return ret, err
		}
		valInt32 := int32(valInt64)
		ret.MetricTop = &valInt32
	}

	// param metricFilter
	ret.MetricFilter = paramsGetWithDefault(params, "metricFilter", "")

	// param metricOrderBy
	ret.MetricOrderBy = paramsGetWithDefault(params, "metricOrderBy", "")

	// param cache (timespan as default)
	if opts.Prober.Cache {
		cacheDefaultDuration, err := iso8601.FromString(ret.Timespan)
		cacheDefaultDurationString := ""
		if err == nil {
			cacheDefaultDurationString = cacheDefaultDuration.ToDuration().String()
		}

		// get value from query (with default from timespan)
		cacheDurationString := paramsGetWithDefault(params, "cache", cacheDefaultDurationString)
		// only enable caching if value is set
		if cacheDurationString != "" {
			if val, err := time.ParseDuration(cacheDurationString); err == nil {
				ret.Cache = &val
			} else {
				log.Error(err.Error())
				return ret, err
			}
		}
	}

	return ret, nil
}

func (s *RequestMetricSettings) CacheDuration(requestTime time.Time) (ret *time.Duration) {
	if s.Cache != nil {
		bufferDuration := time.Duration(2 * time.Second)
		cachedUntilTime := requestTime.Add(*s.Cache).Add(-bufferDuration)
		cacheDuration := time.Until(cachedUntilTime)
		if cacheDuration.Seconds() > 0 {
			ret = &cacheDuration
		}
	}
	return
}

func (s *RequestMetricSettings) SetMetrics(val string) {
	s.Metric = strings.Split(val, ",")
}

func (s *RequestMetricSettings) SetAggregations(val string) {
	s.Aggregation = strings.Split(val, ",")
}
