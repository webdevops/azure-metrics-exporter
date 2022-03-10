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

		Metrics struct {
			ResourceIdLowercase bool   `long:"metrics.resourceid.lowercase"   env:"METRIC_RESOURCEID_LOWERCASE"       description:"Publish lowercase Azure Resoruce ID in metrics"`
			SetTimestamp        bool   `long:"metrics.set-timestamp"          env:"METRIC_SET_TIMESTAMP"              description:"Set timestamp on scraped metrics"`
			Template            string `long:"metrics.template"               env:"METRIC_TEMPLATE"                   description:"Template for metric name"   default:"{name}"`
			Help                string `long:"metrics.help"                   env:"METRIC_HELP"                       description:"Metric help (with template support)"   default:"Azure monitor insight metric"`
		}

		// Prober settings
		Prober struct {
			ConcurrencySubscription         int  `long:"concurrency.subscription"          env:"CONCURRENCY_SUBSCRIPTION"           description:"Concurrent subscription fetches"                                  default:"5"`
			ConcurrencySubscriptionResource int  `long:"concurrency.subscription.resource" env:"CONCURRENCY_SUBSCRIPTION_RESOURCE"  description:"Concurrent requests per resource (inside subscription requests)"  default:"10"`
			Cache                           bool `long:"enable-caching"                    env:"ENABLE_CACHING"                     description:"Enable internal caching"`
		}

		// general options
		ServerBind string `long:"bind"     env:"SERVER_BIND"   description:"Server address"     default:":8080"`

		Development struct {
			WebUi bool `long:"development.webui"   env:"DEVELOPMENT_WEBUI"       description:"Enable webui on server bind socket, accessible with /query"`
		}
	}
)

func (o *Opts) GetJson() []byte {
	jsonBytes, err := json.Marshal(o)
	if err != nil {
		log.Panic(err)
	}
	return jsonBytes
}
