package main

var WebUiIndexHtml = `
<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">

    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-1BmE4kWBq78iYhFldvKuhfTAU6auU8tT94WrHftjDbrCEXSU1oBoqyl2QvZ6jIW3" crossorigin="anonymous">

	<style>
		div.row.hidden {
			display: none;
		}

		.navbar-brand {
			font-size: 1rem;
		}
		small {
			font-size: 0.5em;
		}
	</style>
    <title>azure-metrics-exporter</title>
  </head>
  <body>

  <nav class="navbar navbar-expand-sm navbar-dark bg-dark" aria-label="navbar">
    <div class="container-fluid">
      <a class="navbar-brand" href="#">azure-metrics-exporter query tester <small>(beta)</small></a>
      <button class="navbar-toggler" type="button" data-bs-toggle="collapse" data-bs-target="#navbar" aria-controls="navbar" aria-expanded="false" aria-label="Toggle navigation">
        <span class="navbar-toggler-icon"></span>
      </button>

      <div class="collapse navbar-collapse" id="navbar">
        <ul class="navbar-nav me-auto mb-2 mb-sm-0">
          <li class="nav-item dropdown">
            <a class="nav-link dropdown-toggle" href="#" id="dropdown03" data-bs-toggle="dropdown" aria-expanded="false">Examples</a>
            <ul class="dropdown-menu" aria-labelledby="dropdown03">
              <li><a class="dropdown-item" href="/query?#eyJlbmRwb2ludCI6Ii9wcm9iZS9tZXRyaWNzL2xpc3QiLCJtZXRyaWNOYW1lIjoiYXp1cmVfbWV0cmljIiwic3Vic2NyaXB0aW9uIjoieHh4eHh4eHgteHh4eC14eHh4LXh4eHgteHh4eHh4eHh4eHh4IiwidGFyZ2V0IjoiIiwicmVzb3VyY2VUeXBlIjoiTWljcm9zb2Z0LktleVZhdWx0L3ZhdWx0cyIsImZpbHRlciI6IiIsInJlc291cmNlU3ViUGF0aCI6IiIsIm1ldHJpY05hbWVzcGFjZSI6IiIsIm1ldHJpYyI6IkF2YWlsYWJpbGl0eVxuU2VydmljZUFwaUhpdFxuU2VydmljZUFwaUxhdGVuY3kiLCJpbnRlcnZhbCI6IlBUMUgiLCJ0aW1lc3BhbiI6IlBUMUgiLCJhZ2dyZWdhdGlvbiI6ImF2ZXJhZ2VcbnRvdGFsXG5jb3VudCIsIm1ldHJpY0ZpbHRlciI6IiIsIm1ldHJpY1RvcCI6IjEwIiwidGVtcGxhdGUiOiJ7bmFtZX1fe21ldHJpY31fe2FnZ3JlZ2F0aW9ufV97dW5pdH0iLCJjYWNoZSI6IiIsInNlbmRRdWVyeSI6IiJ9">Azure KeyVault</a></li>
            </ul>
          </li>
        </ul>
      </div>
    </div>
  </nav>

    <main class="container">
      <div class="bg-light p-5 rounded">
        <h1>
			Query settings
		</h1>

        <form class="query">

          <div class="mb-3 row">
			<h3>General</h3>
          </div>

          <div class="mb-3 row">
            <label for="endpoint" class="col-sm-2 col-form-label">endpoint</label>
            <div class="col-sm-10">
                <select id="endpoint" class="form-select" aria-label="endpoint">
                  <option selected value="">- select endpoint -</option>
                  <option>/probe/metrics/resource</option>
                  <option>/probe/metrics/list</option>
                  <option>/probe/metrics/scrape</option>
                  <option>/probe/metrics/resourcegraph</option>
                </select>
                <div class="form-text">azure-metrics-exporter query endpoint</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="metricName" class="col-sm-2 col-form-label">name</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="metricName" value="azure_metric">
              <div class="form-text">Name of metric</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="template" class="col-sm-2 col-form-label">template</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="template" value="10" value="{name}_{metric}_{aggregation}_{unit}">
            <div class="form-text">Metric template support</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="cache" class="col-sm-2 col-form-label">cache</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="cache">
            <div class="form-text">Specifies how long metric result should be cached (if caching is enabled)</div>
            </div>
          </div>

          <div class="mb-3 row">
			<h3>Service Discovery</h3>
          </div>

          <div class="mb-3 row">
            <label for="subscription" class="col-sm-2 col-form-label">subscription</label>
            <div class="col-sm-10">
              <textarea class="form-control" id="subscription" rows="3"></textarea>
              <div class="form-text">List of Azure subscriptions</div>
            </div>
          </div>

          <div class="mb-3 row" query-endpoint="/probe/metrics/resource">
            <label for="target" class="col-sm-2 col-form-label">target</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="target">
                <div class="form-text">Static target (for /probe/metrics/resource)</div>
            </div>
          </div>

          <div class="mb-3 row" query-endpoint-exclude="/probe/metrics/resource">
            <label for="resourceType" class="col-sm-2 col-form-label">resourceType</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="resourceType">
            <div class="form-text">Azure Resource Type query eg <code>Microsoft.KeyVault/vaults</code> (for service discovery)</div>
            </div>
          </div>

          <div class="mb-3 row" query-endpoint-exclude="/probe/metrics/resource">
            <label for="filter" class="col-sm-2 col-form-label">filter</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="filter">
            <div class="form-text">Additional filter statement (Kusto statement for /probe/metrics/resourcegraph; <a href="https://docs.microsoft.com/de-de/rest/api/resources/resources/list" target="_blank">Azure API Resource List $filter</a> for rest)</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="resourceSubPath" class="col-sm-2 col-form-label">resourceSubPath</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="resourceSubPath">
            <div class="form-text">Additional path for namespaced metrics (eg. Azure StorageAccount sub metrics)</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="metricNamespace" class="col-sm-2 col-form-label">metricNamespace</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="metricNamespace">
            <div class="form-text">Additional metric namespace for namespaced metrics (eg. Azure StorageAccount sub metrics)</div>
            </div>
          </div>


          <div class="mb-3 row">
			<h3>Metric settings</h3>
          </div>

          <div class="mb-3 row">
            <label for="metric" class="col-sm-2 col-form-label">metric</label>
            <div class="col-sm-10">
              <textarea class="form-control" id="metric" rows="3"></textarea>
            <div class="form-text">Specifies which <a href="https://docs.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported" target="_blank">Azure metrics</a> should be fetched</div>
            </div>
          </div>


          <div class="mb-3 row">
            <label for="interval" class="col-sm-2 col-form-label">interval</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="interval" value="PT1H">
            <div class="form-text">Metric interval</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="timespan" class="col-sm-2 col-form-label">timespan</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="timespan" value="PT1H">
            <div class="form-text">Metric timeframe</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="aggregation" class="col-sm-2 col-form-label">aggregation</label>
            <div class="col-sm-10">
              <textarea class="form-control" id="aggregation" rows="3">average
total
count</textarea>
            <div class="form-text">Metric aggregation</div>
            </div>
          </div>


          <div class="mb-3 row">
			<h3>Dimension support</h3>
          </div>

          <div class="mb-3 row">
            <label for="metricFilter" class="col-sm-2 col-form-label">metricFilter</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="metricFilter">
            <div class="form-text">Dimension support: filter for metric splitting (eg <code>ConnectionName eq '*'</code>)</div>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="metricTop" class="col-sm-2 col-form-label">metricTop</label>
            <div class="col-sm-10">
              <input type="text" class="form-control" id="metricTop" value="10">
            <div class="form-text">Dimension support: number of fetched dimension rows</div>
            </div>
          </div>

          <div class="mb-3 row">
            <div class="offset-sm-2 col-sm-10">
               <button type="button" class="btn btn-primary mb-3" id="sendQuery">Execute query</button>
            </div>
          </div>
        </form>
      </div>

      <div class="bg-light p-5 rounded queryResult">
        <h2>Result</h2>

          <div class="mb-3 row">
            <label for="metricTop" class="col-sm-2 col-form-label">HTTP status</label>
            <div class="col-sm-10">
              <code id="exporterResponseStatus"></code>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="metricTop" class="col-sm-2 col-form-label">Response body</label>
            <div class="col-sm-10">
              <code id="exporterResponseBody"></code>
            </div>
          </div>

          <div class="mb-3 row">
            <label for="metricTop" class="col-sm-2 col-form-label">Caching status</label>
            <div class="col-sm-10">
              <code id="exporterResponseCache"></code>
            </div>
          </div>
      </div>

      <div class="bg-light p-5 rounded">
        <h2>Prometheus scrape_config</h2>

          <div class="mb-3 row">
            <label for="metricTop" class="col-sm-2 col-form-label">scrape_config</label>
            <div class="col-sm-10">
              <code id="exporterPrometheusScrapeConfig" style="white-space: pre;"></code>
            </div>
          </div>
      </div>


    </main>

<script
  src="https://ajax.aspnetcdn.com/ajax/jQuery/jquery-3.6.0.min.js"
  integrity="sha256-/xUj+3OJU5yExlq6GSYGSHk7tPXikynS7ogEvDej/m4="
  crossorigin="anonymous"></script>
<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/js/bootstrap.bundle.min.js" integrity="sha384-ka7Sk0Gln4gmtz2MlQnikT1wXgYsOg+OMhuP+IlRH9sENBO0LRn5q+8nbTov4+1p" crossorigin="anonymous"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/js-yaml/4.1.0/js-yaml.min.js" integrity="sha512-CSBhVREyzHAjAFfBlIBakjoRUKp5h7VSweP0InR/pAJyptH7peuhCsqAI/snV+TwZmXZqoUklpXp6R6wMnYf5Q==" crossorigin="anonymous" referrerpolicy="no-referrer"></script>

<script>
$( document ).ready(function() {
    let formSaveToHash = () => {
        let formData = {};
        $("form :input").each((num, el) => {
            let formEl = $(el);
            let fieldName = formEl.attr("id");
            let fieldValue = formEl.val();
            fieldValue = fieldValue.trim();

            formData[fieldName] = fieldValue;
        });

        let hashString = btoa(JSON.stringify(formData));
        window.location.hash = hashString;
    };

    $(document).on("change", "form :input", () => {
        formSaveToHash();
    });

    let loadFromHash = () => {
        try {
            if (window.location.hash && window.location.hash.length >= 2) {
                let hashString = window.location.hash.substring(1);
                let formData = jQuery.parseJSON(atob(hashString));

                console.log(formData);
                Object.keys(formData).forEach((fieldName) => {
                    $("#" + fieldName + ":input").val(formData[fieldName]);
                });
            }
        } catch(e) {}

		formSetVisibility();
    };

	let formSetVisibility = () => {
		let endpoint = $("#endpoint:input").val().trim();
		$("form.query div.row").removeClass("hidden");
		$("form.query div.row[query-endpoint]:not([query-endpoint*=\"" + endpoint + "\"])").addClass("hidden");
		$("form.query div.row[query-endpoint-exclude][query-endpoint-exclude*=\"" + endpoint + "\"]").addClass("hidden");
	};

	let buildPrometheusScrapeConfig = (queryEndpoint, queryParams) => {
		let scrapeConfig = {
			scrape_configs: [
				{
					job_name: "azure-metrics-example",
					scrape_interval: "1m",
					metrics_path: queryEndpoint,
					params: queryParams,
					static_configs: [
						{targets: ["url-to-your-azure-metrics-exporter-instance"]}
					],
				}
			]
		}

		let yamlOpts = {
			noRefs: true,
			lineWidth: -1,
		};



		$("#exporterPrometheusScrapeConfig").text( jsyaml.dump(scrapeConfig, yamlOpts) );
	};

	window.onhashchange = () => {
		loadFromHash();
	}
    loadFromHash();

    $(document).on("change", "#endpoint:input", formSetVisibility);

    $(document).on("click", "#sendQuery", () => {
        let queryParams = {};
        let queryParamsForPrometheus = {};
        let queryEndpoint = false

        $("form :input:visible").each((num, el) => {
            let formEl = $(el);
            let fieldName = formEl.attr("id");
            let fieldValue = formEl.val();
            fieldValue = fieldValue.trim();

            switch (fieldName) {
                case "endpoint":
                    queryEndpoint = fieldValue;
                    break;
                default:
					// split by newline
                    fieldValue = fieldValue.split(/\r?\n/);
					// filter empty values
					fieldValue = fieldValue.filter(e =>  e);
                    if (fieldValue.length >= 1) {
                        queryParams[fieldName] = fieldValue.join(",")
						queryParamsForPrometheus[fieldName] = fieldValue
                    }
                    break;
            }
        });

        if (queryEndpoint) {
			$(".queryResult code").text("");

            let jqxhr = $.ajax({
              url: queryEndpoint,
              data: queryParams,
              dataType: "text",
              traditional: false
            }).always(function() {
                $("#exporterResponseStatus").text("HTTP " + jqxhr.status + " " + jqxhr.statusText);
                $("#exporterResponseBody").text(jqxhr.responseText);

                let cachedUntil = jqxhr.getResponseHeader("X-Metrics-Cached-Until");
                let cacheActive = jqxhr.getResponseHeader("X-Metrics-Cached");
                if (cachedUntil) {
                    $("#exporterResponseCache").text("cached until: " + cachedUntil);
                } else if (cacheActive) {
                    $("#exporterResponseCache").text("cached result");
                } else {
                    $("#exporterResponseCache").text("");
                }

				buildPrometheusScrapeConfig(queryEndpoint, queryParamsForPrometheus);
            });
        } else {
            alert("endpoint not selected");
        }
    });

});
</script>

  </body>
</html>
`
