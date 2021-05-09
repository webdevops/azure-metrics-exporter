package config

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"time"
)

type (
	Opts struct {
		// logger
		Logger struct {
			Debug   bool `           long:"debug"        env:"DEBUG"    description:"debug mode"`
			Verbose bool `short:"v"  long:"verbose"      env:"VERBOSE"  description:"verbose mode"`
			LogJson bool `           long:"log.json"     env:"LOG_JSON" description:"Switch log output to json format"`
		}

		// azure
		Azure struct {
			Environment      *string `long:"azure-environment"            env:"AZURE_ENVIRONMENT"                description:"Azure environment name" default:"AZUREPUBLICCLOUD"`
			AdResourceUrl    *string `long:"azure-ad-resource-url"        env:"AZURE_AD_RESOURCE"                description:"Specifies the AAD resource ID to use. If not set, it defaults to ResourceManagerEndpoint for operations with Azure Resource Manager"`
			ServiceDiscovery struct {
				CacheDuration *time.Duration `long:"azure.servicediscovery.cache"            env:"AZURE_SERVICEDISCOVERY_CACHE"                description:"Duration for caching Azure ServiceDiscovery of workspaces to reduce API calls (time.Duration)" default:"30m"`
			}
		}

		// Prober settings
		Prober struct {
			ConcurrencySubscription         int  `long:"concurrency.subscription"          env:"CONCURRENCY_SUBSCRIPTION"           description:"Concurrent subscription fetches"                                  default:"5"`
			ConcurrencySubscriptionResource int  `long:"concurrency.subscription.resource" env:"CONCURRENCY_SUBSCRIPTION_RESOURCE"  description:"Concurrent requests per resource (inside subscription requests)"  default:"10"`
			Cache                           bool `long:"enable-caching"                    env:"ENABLE_CACHING"                     description:"Enable internal caching"`
		}

		// general options
		ServerBind string `long:"bind"     env:"SERVER_BIND"   description:"Server address"     default:":8080"`
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		log.Panic(err)
	}
	return jsonBytes
}
