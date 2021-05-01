package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/operationalinsights/v1/operationalinsights"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
)

type AzureLogAnalysticsMetrics struct {
	authorizer         *autorest.Authorizer
	prometheusRegistry *prometheus.Registry
}

type AzureLogAnalysticsMetricsResult struct {
	Result *operationalinsights.QueryResults
}

func NewAzureLogAnalysticsMetrics(registry *prometheus.Registry) *AzureLogAnalysticsMetrics {
	ret := AzureLogAnalysticsMetrics{}

	authorizer, err := auth.NewAuthorizerFromEnvironmentWithResource(AzureEnvironment.ResourceIdentifiers.OperationalInsights)
	if err != nil {
		panic(err)
	}

	ret.authorizer = &authorizer
	ret.prometheusRegistry = registry

	return &ret
}

func (m *AzureLogAnalysticsMetrics) QueryClient() *operationalinsights.QueryClient {
	client := operationalinsights.NewQueryClient()
	client.Authorizer = *m.authorizer
	client.ResponseInspector = m.azureResponseInspector()

	return &client
}

func (m *AzureLogAnalysticsMetrics) azureResponseInspector() autorest.RespondDecorator {
	return func(p autorest.Responder) autorest.Responder {
		return autorest.ResponderFunc(func(r *http.Response) error {
			return nil
		})
	}
}

func (m *AzureLogAnalysticsMetrics) Query(ctx context.Context, workspaceId string, query operationalinsights.QueryBody) (*AzureLogAnalysticsMetricsResult, error) {
	ret := AzureLogAnalysticsMetricsResult{}

	result, err := m.QueryClient().Execute(ctx, workspaceId, query)
	ret.Result = &result

	return &ret, err
}
