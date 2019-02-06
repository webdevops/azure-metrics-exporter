package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/services/operationalinsights/v1/operationalinsights"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"sync"
)

type AzureLogAnalysticsMetrics struct {
	client      *operationalinsights.QueryClient
	clientMutex sync.Mutex
}

type AzureLogAnalysticsMetricsResult struct {
	Result *operationalinsights.QueryResults
}

func NewAzureLogAnalysticsMetrics() *AzureLogAnalysticsMetrics {
	ret := AzureLogAnalysticsMetrics{}
	return &ret
}

func (m *AzureLogAnalysticsMetrics) QueryClient() *operationalinsights.QueryClient {
	if m.client == nil {
		m.clientMutex.Lock()
		keyvaultAuth, err := auth.NewAuthorizerFromEnvironmentWithResource("https://api.loganalytics.io")
		if err != nil {
			panic(err)
		}

		client := operationalinsights.NewQueryClient()
		client.Authorizer = keyvaultAuth
		m.client = &client
		m.clientMutex.Unlock()
	}

	return m.client
}

func (m *AzureLogAnalysticsMetrics) Query(ctx context.Context, workspaceId string, query operationalinsights.QueryBody) (*AzureLogAnalysticsMetricsResult, error) {
	ret := AzureLogAnalysticsMetricsResult{}

	result, err := m.QueryClient().Execute(ctx, workspaceId, query)
	ret.Result = &result

	return &ret, err
}
