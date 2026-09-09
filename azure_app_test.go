package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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

func TestAppRegistrationExporterRefreshAndMetrics(t *testing.T) {
	t.Parallel()

	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected Authorization header: %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/v1.0/applications":
			fmt.Fprintf(w, `{
				"value": [
					{
						"appId": "app-a",
						"displayName": "Application A",
						"tags": ["monitored"],
						"passwordCredentials": [
							{"keyId": "credential-1", "displayName": "rotation", "endDateTime": "2026-10-31T00:00:00Z"}
						]
					},
					{"appId": "excluded", "displayName": "Excluded", "tags": ["other"]}
				],
				"@odata.nextLink": %q
			}`, server.URL+"/v1.0/next")
		case "/v1.0/next":
			fmt.Fprint(w, `{
				"value": [
					{"appId": "app-b", "displayName": "Application B", "tags": ["monitored"]}
				]
			}`)
		case "/v1.0/servicePrincipals":
			filter := r.URL.Query().Get("$filter")
			switch filter {
			case "appId eq 'app-a'":
				fmt.Fprint(w, `{
					"value": [{
						"passwordCredentials": [
							{"keyId": "credential-1", "displayName": "rotation", "endDateTime": "2026-10-31T00:00:00Z"},
							{"keyId": "credential-2", "displayName": "rotation", "endDateTime": "2026-09-30T00:00:00Z"},
							{"keyId": "credential-3", "displayName": "certificate-without-expiry"}
						]
					}]
				}`)
			case "appId eq 'app-b'":
				fmt.Fprint(w, `{"value": []}`)
			default:
				t.Errorf("unexpected service principal filter: %q", filter)
				http.Error(w, "unexpected filter", http.StatusBadRequest)
			}
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	exporter, err := NewAppRegistrationExporter(
		staticTokenCredential{},
		server.Client(),
		server.URL+"/v1.0",
		server.URL+"/.default",
		[]string{" monitored ", ""},
		time.Hour,
		5*time.Second,
		slog.Default(),
	)
	if err != nil {
		t.Fatalf("NewAppRegistrationExporter() error = %v", err)
	}
	if err := exporter.Refresh(context.Background()); err != nil {
		t.Fatalf("Refresh() error = %v", err)
	}

	apps := exporter.Applications()
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

func TestAppRegistrationExporterRejectsCrossOriginNextLink(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"value": [], "@odata.nextLink": "https://attacker.invalid/applications"}`)
	}))
	defer server.Close()

	exporter, err := NewAppRegistrationExporter(
		staticTokenCredential{}, server.Client(), server.URL, server.URL+"/.default", nil,
		time.Hour, 5*time.Second, slog.Default(),
	)
	if err != nil {
		t.Fatalf("NewAppRegistrationExporter() error = %v", err)
	}
	err = exporter.Refresh(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unexpected origin") {
		t.Fatalf("Refresh() error = %v, want unexpected origin error", err)
	}
}

func TestAppRegistrationExporterGetEscapesAppID(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/applications" {
			if got := r.URL.Query().Get("$filter"); got != "appId eq 'app''id'" {
				t.Errorf("filter = %q, want escaped OData string", got)
			}
			fmt.Fprint(w, `{"value": [{"appId": "app'id", "displayName": "Application"}]}`)
			return
		}
		if r.URL.Path == "/servicePrincipals" {
			fmt.Fprint(w, `{"value": []}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	graphURL, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	exporter, err := NewAppRegistrationExporter(
		staticTokenCredential{}, server.Client(), graphURL.String(), server.URL+"/.default", nil,
		time.Hour, 5*time.Second, slog.Default(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exporter.Get(context.Background(), "app'id"); err != nil {
		t.Fatalf("Get() error = %v", err)
	}
}
