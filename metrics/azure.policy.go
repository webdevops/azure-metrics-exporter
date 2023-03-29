package metrics

import (
	"net/http"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type noCachePolicy struct{}

func (p noCachePolicy) Do(req *policy.Request) (*http.Response, error) {
	// Mutate/process request.
	req.Raw().Header.Set("cache-control", "no-cache")

	// replace encoded %2C to ,
	req.Raw().URL.RawQuery = strings.ReplaceAll(req.Raw().URL.RawQuery, "%2C", ",")

	// Forward the request to the next policy in the pipeline.
	return req.Next()
}
