package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	sdkazidentity "github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/webdevops/azure-metrics-exporter/config"
)

const graphResponseBodyLimit = 4096

type (
	AppRegistrationCredential struct {
		ID      string     `json:"id,omitempty"`
		Name    string     `json:"name,omitempty"`
		Created *time.Time `json:"created,omitempty"`
		Expires *time.Time `json:"expires,omitempty"`
	}

	AppRegistration struct {
		ID          string                      `json:"id"`
		Name        string                      `json:"name"`
		Credentials []AppRegistrationCredential `json:"credentials"`
	}

	graphCredential struct {
		ID        string `json:"keyId"`
		Name      string `json:"displayName"`
		CreatedAt string `json:"startDateTime"`
		ExpiresAt string `json:"endDateTime"`
	}

	graphApplication struct {
		ID                  string            `json:"appId"`
		Name                string            `json:"displayName"`
		Tags                []string          `json:"tags"`
		PasswordCredentials []graphCredential `json:"passwordCredentials"`
		KeyCredentials      []graphCredential `json:"keyCredentials"`
	}

	graphApplicationsResponse struct {
		Value    []graphApplication `json:"value"`
		NextLink string             `json:"@odata.nextLink"`
	}

	graphServicePrincipal struct {
		PasswordCredentials []graphCredential `json:"passwordCredentials"`
		KeyCredentials      []graphCredential `json:"keyCredentials"`
	}

	graphServicePrincipalsResponse struct {
		Value []graphServicePrincipal `json:"value"`
	}

	AppRegistrationExporter struct {
		credential      azcore.TokenCredential
		httpClient      *http.Client
		graphURL        *url.URL
		graphScope      string
		filterTags      map[string]struct{}
		refreshInterval time.Duration
		requestTimeout  time.Duration
		logger          *slog.Logger

		refreshMu sync.Mutex
		stateMu   sync.RWMutex
		apps      []AppRegistration
		lastError error
		lastRun   time.Time

		earliestExpiryDesc *prometheus.Desc
		credentialDesc     *prometheus.Desc
		refreshSuccessDesc *prometheus.Desc
		lastRefreshDesc    *prometheus.Desc
	}
)

func initAppRegistrationExporter() {
	if !Opts.Azure.AppRegistration.Enabled {
		return
	}

	credential, err := sdkazidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		logger.Fatal("unable to initialize Azure credential for app registrations", slog.Any("error", err))
	}

	AzureAppExporter, err = NewAppRegistrationExporter(
		credential,
		&http.Client{},
		Opts.Azure.AppRegistration.GraphURL,
		Opts.Azure.AppRegistration.GraphScope,
		Opts.Azure.AppRegistration.FilterTags,
		Opts.Azure.AppRegistration.RefreshInterval,
		Opts.Azure.AppRegistration.RequestTimeout,
		logger.Logger,
	)
	if err != nil {
		logger.Fatal("unable to initialize Azure app registration exporter", slog.Any("error", err))
	}

	prometheus.MustRegister(AzureAppExporter)
	AzureAppExporter.Start(context.Background())
}

func NewAppRegistrationExporter(
	credential azcore.TokenCredential,
	httpClient *http.Client,
	graphURL string,
	graphScope string,
	filterTags []string,
	refreshInterval time.Duration,
	requestTimeout time.Duration,
	logger *slog.Logger,
) (*AppRegistrationExporter, error) {
	if credential == nil {
		return nil, errors.New("Azure credential is required")
	}
	if httpClient == nil {
		return nil, errors.New("HTTP client is required")
	}

	parsedGraphURL, err := url.Parse(strings.TrimRight(graphURL, "/"))
	if err != nil || parsedGraphURL.Scheme == "" || parsedGraphURL.Host == "" {
		return nil, fmt.Errorf("invalid Microsoft Graph URL %q", graphURL)
	}
	if graphScope == "" {
		return nil, errors.New("Microsoft Graph scope is required")
	}
	if refreshInterval <= 0 {
		return nil, errors.New("app registration refresh interval must be positive")
	}
	if requestTimeout <= 0 {
		return nil, errors.New("app registration request timeout must be positive")
	}
	if logger == nil {
		logger = slog.Default()
	}

	configuredTags := make(map[string]struct{}, len(filterTags))
	for _, tag := range filterTags {
		if tag = strings.TrimSpace(tag); tag != "" {
			configuredTags[tag] = struct{}{}
		}
	}

	return &AppRegistrationExporter{
		credential:      credential,
		httpClient:      httpClient,
		graphURL:        parsedGraphURL,
		graphScope:      graphScope,
		filterTags:      configuredTags,
		refreshInterval: refreshInterval,
		requestTimeout:  requestTimeout,
		logger:          logger,
		earliestExpiryDesc: prometheus.NewDesc(
			"azure_app_earliest_expiry",
			"Returns earliest credential expiry in unix time (seconds)",
			[]string{"app_id", "app_name"}, nil,
		),
		credentialDesc: prometheus.NewDesc(
			"azure_app_credential_expiry",
			"Returns the expiration in unix time (seconds) for each credential of an app",
			[]string{"app_id", "app_name", "credential_id", "credential_name"}, nil,
		),
		refreshSuccessDesc: prometheus.NewDesc(
			"azure_app_exporter_refresh_success",
			"Whether the last Microsoft Graph refresh completed successfully",
			nil, nil,
		),
		lastRefreshDesc: prometheus.NewDesc(
			"azure_app_exporter_last_refresh_timestamp_seconds",
			"Unix timestamp of the last successful Microsoft Graph refresh",
			nil, nil,
		),
	}, nil
}

func (e *AppRegistrationExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- e.earliestExpiryDesc
	ch <- e.credentialDesc
	ch <- e.refreshSuccessDesc
	ch <- e.lastRefreshDesc
}

func (e *AppRegistrationExporter) Collect(ch chan<- prometheus.Metric) {
	apps, lastRun, lastErr := e.snapshot()

	for _, app := range apps {
		var earliest *time.Time
		for _, credential := range app.Credentials {
			if credential.Expires == nil {
				continue
			}
			if earliest == nil || credential.Expires.Before(*earliest) {
				expiry := *credential.Expires
				earliest = &expiry
			}

			credentialID := credential.ID
			if credentialID == "" {
				credentialID = "unknown"
			}
			credentialName := credential.Name
			if credentialName == "" {
				credentialName = "unnamed"
			}

			ch <- prometheus.MustNewConstMetric(
				e.credentialDesc,
				prometheus.GaugeValue,
				float64(credential.Expires.Unix()),
				app.ID, app.Name, credentialID, credentialName,
			)
		}
		if earliest != nil {
			ch <- prometheus.MustNewConstMetric(
				e.earliestExpiryDesc,
				prometheus.GaugeValue,
				float64(earliest.Unix()),
				app.ID, app.Name,
			)
		}
	}

	success := 1.0
	if lastErr != nil || lastRun.IsZero() {
		success = 0
	}
	ch <- prometheus.MustNewConstMetric(e.refreshSuccessDesc, prometheus.GaugeValue, success)
	if !lastRun.IsZero() {
		ch <- prometheus.MustNewConstMetric(e.lastRefreshDesc, prometheus.GaugeValue, float64(lastRun.Unix()))
	}
}

func (e *AppRegistrationExporter) Start(ctx context.Context) {
	go func() {
		e.refreshAndLog(ctx)

		ticker := time.NewTicker(e.refreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				e.refreshAndLog(ctx)
			}
		}
	}()
}

func (e *AppRegistrationExporter) refreshAndLog(ctx context.Context) {
	if err := e.Refresh(ctx); err != nil {
		e.logger.Error("unable to refresh Azure app registrations", slog.Any("error", err))
		return
	}
	apps, _, _ := e.snapshot()
	e.logger.Info("refreshed Azure app registrations", slog.Int("applications", len(apps)))
}

func (e *AppRegistrationExporter) Refresh(ctx context.Context) error {
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	apps, err := e.fetchAll(ctx)
	e.stateMu.Lock()
	defer e.stateMu.Unlock()
	if err != nil {
		e.lastError = err
		return err
	}

	e.apps = cloneAppRegistrations(apps)
	e.lastError = nil
	e.lastRun = time.Now()
	return nil
}

func (e *AppRegistrationExporter) Applications() []AppRegistration {
	apps, _, _ := e.snapshot()
	return apps
}

func (e *AppRegistrationExporter) Get(ctx context.Context, appID string) (AppRegistration, error) {
	query := url.Values{}
	query.Set("$filter", fmt.Sprintf("appId eq '%s'", strings.ReplaceAll(appID, "'", "''")))
	query.Set("$select", "appId,displayName,tags,passwordCredentials,keyCredentials")
	query.Set("$top", "1")

	var response graphApplicationsResponse
	if err := e.getJSON(ctx, e.graphEndpoint("applications", query), &response); err != nil {
		return AppRegistration{}, err
	}
	if len(response.Value) == 0 {
		return AppRegistration{}, fmt.Errorf("application with app id %q not found", appID)
	}
	return e.mapApplication(ctx, response.Value[0])
}

func (e *AppRegistrationExporter) fetchAll(ctx context.Context) ([]AppRegistration, error) {
	query := url.Values{}
	query.Set("$select", "appId,displayName,tags,passwordCredentials,keyCredentials")
	query.Set("$top", "999")
	nextURL := e.graphEndpoint("applications", query)
	apps := make([]AppRegistration, 0)

	for nextURL != "" {
		if err := e.validateGraphURL(nextURL); err != nil {
			return nil, err
		}

		var response graphApplicationsResponse
		if err := e.getJSON(ctx, nextURL, &response); err != nil {
			return nil, err
		}
		for _, application := range response.Value {
			if !e.matchesTagFilter(application.Tags) {
				continue
			}
			mapped, err := e.mapApplication(ctx, application)
			if err != nil {
				return nil, err
			}
			apps = append(apps, mapped)
		}
		nextURL = response.NextLink
	}

	sort.Slice(apps, func(i, j int) bool { return apps[i].ID < apps[j].ID })
	return apps, nil
}

func (e *AppRegistrationExporter) mapApplication(ctx context.Context, application graphApplication) (AppRegistration, error) {
	servicePrincipalCredentials, err := e.fetchServicePrincipalCredentials(ctx, application.ID)
	if err != nil {
		return AppRegistration{}, fmt.Errorf("fetch service principal credentials for app %q: %w", application.ID, err)
	}

	credentials := make([]graphCredential, 0, len(application.PasswordCredentials)+len(application.KeyCredentials)+len(servicePrincipalCredentials))
	credentials = append(credentials, application.PasswordCredentials...)
	credentials = append(credentials, application.KeyCredentials...)
	credentials = append(credentials, servicePrincipalCredentials...)

	seen := make(map[string]struct{}, len(credentials))
	mappedCredentials := make([]AppRegistrationCredential, 0, len(credentials))
	for _, credential := range credentials {
		identity := credential.ID
		if identity == "" {
			identity = strings.Join([]string{credential.Name, credential.CreatedAt, credential.ExpiresAt}, "\x00")
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}

		created, err := parseGraphTime(credential.CreatedAt)
		if err != nil {
			return AppRegistration{}, fmt.Errorf("parse credential %q start time: %w", credential.ID, err)
		}
		expires, err := parseGraphTime(credential.ExpiresAt)
		if err != nil {
			return AppRegistration{}, fmt.Errorf("parse credential %q expiry time: %w", credential.ID, err)
		}
		mappedCredentials = append(mappedCredentials, AppRegistrationCredential{
			ID:      credential.ID,
			Name:    credential.Name,
			Created: created,
			Expires: expires,
		})
	}

	return AppRegistration{
		ID:          application.ID,
		Name:        application.Name,
		Credentials: mappedCredentials,
	}, nil
}

func (e *AppRegistrationExporter) fetchServicePrincipalCredentials(ctx context.Context, appID string) ([]graphCredential, error) {
	if appID == "" {
		return nil, nil
	}

	query := url.Values{}
	query.Set("$filter", fmt.Sprintf("appId eq '%s'", strings.ReplaceAll(appID, "'", "''")))
	query.Set("$select", "keyCredentials,passwordCredentials")
	query.Set("$top", "1")

	var response graphServicePrincipalsResponse
	if err := e.getJSON(ctx, e.graphEndpoint("servicePrincipals", query), &response); err != nil {
		return nil, err
	}
	if len(response.Value) == 0 {
		return nil, nil
	}

	credentials := make([]graphCredential, 0, len(response.Value[0].KeyCredentials)+len(response.Value[0].PasswordCredentials))
	credentials = append(credentials, response.Value[0].KeyCredentials...)
	credentials = append(credentials, response.Value[0].PasswordCredentials...)
	return credentials, nil
}

func (e *AppRegistrationExporter) getJSON(ctx context.Context, requestURL string, destination any) error {
	if err := e.validateGraphURL(requestURL); err != nil {
		return err
	}

	requestContext, cancel := context.WithTimeout(ctx, e.requestTimeout)
	defer cancel()

	token, err := e.credential.GetToken(requestContext, policy.TokenRequestOptions{Scopes: []string{e.graphScope}})
	if err != nil {
		return fmt.Errorf("get Microsoft Graph access token: %w", err)
	}

	request, err := http.NewRequestWithContext(requestContext, http.MethodGet, requestURL, nil)
	if err != nil {
		return fmt.Errorf("create Microsoft Graph request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+token.Token)
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", UserAgent+gitTag)

	response, err := e.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("request Microsoft Graph: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(response.Body, graphResponseBodyLimit))
		return fmt.Errorf("Microsoft Graph returned %s: %s", response.Status, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(response.Body).Decode(destination); err != nil {
		return fmt.Errorf("decode Microsoft Graph response: %w", err)
	}
	return nil
}

func (e *AppRegistrationExporter) graphEndpoint(resource string, query url.Values) string {
	endpoint := *e.graphURL
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/" + resource
	endpoint.RawQuery = query.Encode()
	return endpoint.String()
}

func (e *AppRegistrationExporter) validateGraphURL(candidate string) error {
	parsed, err := url.Parse(candidate)
	if err != nil {
		return fmt.Errorf("invalid Microsoft Graph pagination URL: %w", err)
	}
	if !strings.EqualFold(parsed.Scheme, e.graphURL.Scheme) || !strings.EqualFold(parsed.Host, e.graphURL.Host) {
		return fmt.Errorf("refusing Microsoft Graph pagination URL on unexpected origin %q", parsed.Host)
	}
	return nil
}

func (e *AppRegistrationExporter) matchesTagFilter(tags []string) bool {
	if len(e.filterTags) == 0 {
		return true
	}
	for _, tag := range tags {
		if _, ok := e.filterTags[tag]; ok {
			return true
		}
	}
	return false
}

func (e *AppRegistrationExporter) snapshot() ([]AppRegistration, time.Time, error) {
	e.stateMu.RLock()
	defer e.stateMu.RUnlock()
	return cloneAppRegistrations(e.apps), e.lastRun, e.lastError
}

func cloneAppRegistrations(apps []AppRegistration) []AppRegistration {
	cloned := make([]AppRegistration, len(apps))
	for index, app := range apps {
		cloned[index] = app
		cloned[index].Credentials = append([]AppRegistrationCredential(nil), app.Credentials...)
	}
	return cloned
}

func parseGraphTime(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func appRegistrationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	encoder := json.NewEncoder(w)

	if r.URL.Path == config.AppRegistrationsURL {
		if err := AzureAppExporter.Refresh(r.Context()); err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if err := encoder.Encode(AzureAppExporter.Applications()); err != nil {
			logger.Error("unable to encode Azure app registrations", slog.Any("error", err))
		}
		return
	}

	appID, err := url.PathUnescape(strings.TrimPrefix(r.URL.Path, config.AppRegistrationURLPrefix))
	if err != nil || appID == "" || strings.Contains(appID, "/") {
		http.Error(w, "invalid app id", http.StatusBadRequest)
		return
	}
	application, err := AzureAppExporter.Get(r.Context(), appID)
	if err != nil {
		status := http.StatusBadGateway
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		http.Error(w, err.Error(), status)
		return
	}
	if err := encoder.Encode(application); err != nil {
		logger.Error("unable to encode Azure app registration", slog.Any("error", err))
	}
}
