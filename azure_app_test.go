package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/prometheus/client_golang/prometheus"
)

type staticTokenCredential struct{}

func (staticTokenCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "test-token", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

type graphTestHandler struct {
	t                        *testing.T
	serverURL                string
	failApplications         atomic.Bool
	applicationRequests      atomic.Int32
	servicePrincipalRequests atomic.Int32
}

func (h *graphTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.t.Helper()
	if r.Header.Get("Authorization") != "Bearer test-token" {
		h.t.Errorf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
	}
	w.Header().Set("Content-Type", "application/json")

	switch r.URL.Path {
	case "/v1.0/applications":
		h.applicationRequests.Add(1)
		if h.failApplications.Load() {
			http.Error(w, "temporary Graph failure", http.StatusServiceUnavailable)
			return
		}
		h.writeApplications(w)
	case "/v1.0/applications-next":
		h.applicationRequests.Add(1)
		fmt.Fprint(w, `{"value":[{"appId":"app-b","displayName":"Application B","tags":["monitored"]}]}`)
	case "/v1.0/servicePrincipals":
		h.servicePrincipalRequests.Add(1)
		h.writeServicePrincipals(w, r)
	case "/v1.0/service-principals-next":
		h.servicePrincipalRequests.Add(1)
		fmt.Fprint(w, `{"value":[{"appId":"app-b","passwordCredentials":[]}]}`)
	default:
		h.t.Errorf("unexpected request path: %s", r.URL.Path)
		http.NotFound(w, r)
	}
}

func (h *graphTestHandler) writeApplications(w http.ResponseWriter) {
	fmt.Fprintf(w, `{
		"value":[
			{"appId":"app-a","displayName":"Application A","tags":["monitored"],"passwordCredentials":[{"keyId":"credential-1","displayName":"rotation","endDateTime":"2026-10-31T00:00:00Z"}]},
			{"appId":"excluded","displayName":"Excluded","tags":["other"]}
		],
		"@odata.nextLink":%q
	}`, h.serverURL+"/v1.0/applications-next")
}

func (h *graphTestHandler) writeServicePrincipals(w http.ResponseWriter, r *http.Request) {
	if got := r.URL.Query().Get(graphSelectParameter); got != graphServicePrincipalsSelectFields {
		h.t.Errorf("service principal select = %q, want %q", got, graphServicePrincipalsSelectFields)
	}
	fmt.Fprintf(w, `{
		"value":[{"appId":"app-a","passwordCredentials":[
			{"keyId":"credential-1","displayName":"rotation","endDateTime":"2026-10-31T00:00:00Z"},
			{"keyId":"credential-2","displayName":"rotation","endDateTime":"2026-09-30T00:00:00Z"},
			{"keyId":"credential-3","displayName":"certificate-without-expiry"}
		]}],
		"@odata.nextLink":%q
	}`, h.serverURL+"/v1.0/service-principals-next")
}

func newGraphTestServer(t *testing.T) (*httptest.Server, *graphTestHandler) {
	t.Helper()
	handler := &graphTestHandler{t: t}
	server := httptest.NewServer(handler)
	handler.serverURL = server.URL
	t.Cleanup(server.Close)
	return server, handler
}

func newTestAppRegistrationExporter(t *testing.T, server *httptest.Server, filterTags []string) *AppRegistrationExporter {
	t.Helper()
	exporter, err := NewAppRegistrationExporter(
		staticTokenCredential{},
		server.Client(),
		AppRegistrationExporterOptions{
			GraphURL:        server.URL + "/v1.0",
			GraphScope:      server.URL + "/.default",
			FilterTags:      filterTags,
			RefreshInterval: time.Hour,
			RequestTimeout:  5 * time.Second,
		},
		slog.Default(),
	)
	if err != nil {
		t.Fatalf("NewAppRegistrationExporter() error = %v", err)
	}
	return exporter
}

func TestAppRegistrationExporterRefreshAndMetrics(t *testing.T) {
	t.Parallel()
	server, graphHandler := newGraphTestServer(t)
	exporter := newTestAppRegistrationExporter(t, server, []string{" monitored ", ""})

	if err := exporter.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	assertApplications(t, exporter.Applications())
	assertMetrics(t, exporter)

	if got := graphHandler.applicationRequests.Load(); got != 2 {
		t.Errorf("application requests = %d, want 2 paginated requests", got)
	}
	if got := graphHandler.servicePrincipalRequests.Load(); got != 2 {
		t.Errorf("service principal requests = %d, want 2 paginated bulk requests", got)
	}
}

func assertApplications(t *testing.T, apps []AppRegistration) {
	t.Helper()
	if len(apps) != 2 {
		t.Fatalf("got %d applications, want 2", len(apps))
	}
	if apps[0].ID != "app-a" || apps[1].ID != "app-b" {
		t.Fatalf("unexpected applications: %#v", apps)
	}
	if got := len(apps[0].Credentials); got != 3 {
		t.Fatalf("got %d credentials for app-a, want 3", got)
	}
	if apps[0].Credentials[0].ID != "credential-1" || apps[0].Credentials[1].ID != "credential-2" {
		t.Fatalf("credentials were not deduplicated by ID: %#v", apps[0].Credentials)
	}
}

func assertMetrics(t *testing.T, exporter *AppRegistrationExporter) {
	t.Helper()
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(exporter); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}

	metricCounts := map[string]int{}
	metricValues := map[string]float64{}
	for _, family := range metricFamilies {
		metricCounts[family.GetName()] = len(family.Metric)
		if len(family.Metric) > 0 && family.Metric[0].Gauge != nil {
			metricValues[family.GetName()] = family.Metric[0].Gauge.GetValue()
		}
	}
	if metricCounts["azure_app_earliest_expiry"] != 1 {
		t.Errorf("earliest expiry metric count = %d, want 1", metricCounts["azure_app_earliest_expiry"])
	}
	if metricCounts["azure_app_credential_expiry"] != 2 {
		t.Errorf("credential expiry metric count = %d, want 2", metricCounts["azure_app_credential_expiry"])
	}
	if metricValues["azure_app_exporter_refresh_success"] != 1 {
		t.Errorf("refresh success = %v, want 1", metricValues["azure_app_exporter_refresh_success"])
	}
}

func TestAppRegistrationAPIUsesCachedFilteredSnapshot(t *testing.T) {
	t.Parallel()
	server, graphHandler := newGraphTestServer(t)
	exporter := newTestAppRegistrationExporter(t, server, []string{"monitored"})
	if err := exporter.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}
	requestCount := graphHandler.applicationRequests.Load() + graphHandler.servicePrincipalRequests.Load()

	handler := newAppRegistrationsHandler(exporter)
	listResponse := httptest.NewRecorder()
	handler.ServeHTTP(listResponse, httptest.NewRequest(http.MethodGet, "/api/apps", nil))
	if listResponse.Code != http.StatusOK {
		t.Fatalf("GET /api/apps status = %d, want %d", listResponse.Code, http.StatusOK)
	}
	var apps []AppRegistration
	if err := json.NewDecoder(listResponse.Body).Decode(&apps); err != nil {
		t.Fatalf("decode GET /api/apps response: %v", err)
	}
	assertApplications(t, apps)

	excludedResponse := httptest.NewRecorder()
	handler.ServeHTTP(excludedResponse, httptest.NewRequest(http.MethodGet, "/api/apps/excluded", nil))
	if excludedResponse.Code != http.StatusNotFound {
		t.Errorf("GET filtered application status = %d, want %d", excludedResponse.Code, http.StatusNotFound)
	}

	applicationResponse := httptest.NewRecorder()
	handler.ServeHTTP(applicationResponse, httptest.NewRequest(http.MethodGet, "/api/apps/app-a", nil))
	if applicationResponse.Code != http.StatusOK {
		t.Errorf("GET cached application status = %d, want %d", applicationResponse.Code, http.StatusOK)
	}

	methodResponse := httptest.NewRecorder()
	handler.ServeHTTP(methodResponse, httptest.NewRequest(http.MethodPost, "/api/apps", nil))
	if methodResponse.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST /api/apps status = %d, want %d", methodResponse.Code, http.StatusMethodNotAllowed)
	}
	if got := graphHandler.applicationRequests.Load() + graphHandler.servicePrincipalRequests.Load(); got != requestCount {
		t.Errorf("API triggered Graph requests: got %d total, want %d", got, requestCount)
	}
}

func TestAppRegistrationExporterPreservesSnapshotAfterRefreshFailure(t *testing.T) {
	t.Parallel()
	server, graphHandler := newGraphTestServer(t)
	exporter := newTestAppRegistrationExporter(t, server, []string{"monitored"})
	if err := exporter.Refresh(context.Background()); err != nil {
		t.Fatalf("initial Refresh() error = %v", err)
	}

	graphHandler.failApplications.Store(true)
	if err := exporter.Refresh(context.Background()); err == nil {
		t.Fatal("Refresh() error = nil, want Graph failure")
	}
	assertApplications(t, exporter.Applications())
	assertRefreshFailureMetric(t, exporter)
}

func assertRefreshFailureMetric(t *testing.T, exporter *AppRegistrationExporter) {
	t.Helper()
	registry := prometheus.NewPedanticRegistry()
	if err := registry.Register(exporter); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error = %v", err)
	}
	found := false
	for _, family := range metricFamilies {
		if family.GetName() == "azure_app_exporter_refresh_success" {
			found = true
			if family.Metric[0].Gauge.GetValue() != 0 {
				t.Errorf("refresh success = %v, want 0", family.Metric[0].Gauge.GetValue())
			}
		}
	}
	if !found {
		t.Error("azure_app_exporter_refresh_success metric was not collected")
	}
}

func TestAppRegistrationExporterRejectsCrossOriginNextLink(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"value":[],"@odata.nextLink":"https://attacker.invalid/applications"}`)
	}))
	t.Cleanup(server.Close)
	exporter := newTestAppRegistrationExporter(t, server, nil)

	err := exporter.Refresh(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpected origin") {
		t.Fatalf("Refresh() error = %v, want unexpected origin error", err)
	}
}
