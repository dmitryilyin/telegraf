package logstash

import (
	"encoding/json"
	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/internal/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
	"net/http"
	"net/url"
	"time"

	jsonparser "github.com/influxdata/telegraf/plugins/parsers/json"
)

const sampleConfig = `
  ## This plugin reads metrics exposed by Logstash Monitoring API.
  ## https://www.elastic.co/guide/en/logstash/current/monitoring.html

  ## The URL of the exposed Logstash API endpoint
  url = "http://127.0.0.1:9600"

  ## Enable Logstash 6+ multi-pipeline statistics support
  multi_pipeline = true

  ## Should the general process statistics be gathered
  collect_process_stats = true

  ## Should the JVM specific statistics be gathered
  collect_jvm_stats = true

  ## Should the event pipelines statistics be gathered
  collect_pipelines_stats = true

  ## Should the plugin statistics be gathered
  collect_plugins_stats = true

  ## Timeout for HTTP requests
  timeout = "5s"

  ## HTTP Basic Auth credentials
  # username = "username"
  # password = "pa$$word"

  ## Optional TLS Config
  # tls_ca = "/etc/telegraf/ca.pem"
  # tls_cert = "/etc/telegraf/cert.pem"
  # tls_key = "/etc/telegraf/key.pem"
  ## Use TLS but skip chain & host verification
  # insecure_skip_verify = false
`

const jvmStats = "/_node/stats/jvm"
const processStats = "/_node/stats/process"
const pipelineStats = "/_node/stats/pipeline"
const pipelinesStats = "/_node/stats/pipelines"

type Logstash struct {
	URL string `toml:"url"`

	MultiPipeline         bool `toml:"multi_pipeline"`
	CollectProcessStats   bool `toml:"collect_process_stats"`
	CollectJVMStats       bool `toml:"collect_jvm_stats"`
	CollectPipelinesStats bool `toml:"collect_pipelines_stats"`
	CollectPluginsStats   bool `toml:"collect_plugins_stats"`

	Username string            `toml:"username"`
	Password string            `toml:"password"`
	Timeout  internal.Duration `toml:"timeout"`

	tls.ClientConfig
	client *http.Client
}

func NewLogstash() *Logstash {
	return &Logstash{
		URL:                   "http://127.0.0.1:9600",
		MultiPipeline:         true,
		CollectProcessStats:   true,
		CollectJVMStats:       true,
		CollectPipelinesStats: true,
		CollectPluginsStats:   true,
		Timeout:               internal.Duration{Duration: time.Second * 5},
	}
}

type JVMStats struct {
	ID      string      `json:"id"`
	JVM     interface{} `json:"jvm"`
	Name    string      `json:"name"`
	Host    string      `json:"host"`
	Version string      `json:"version"`
}

type ProcessStats struct {
	ID      string      `json:"id"`
	Process interface{} `json:"process"`
	Name    string      `json:"name"`
	Host    string      `json:"host"`
	Version string      `json:"version"`
}

type PluginEvents struct {
	QueuePushDurationInMillis float64 `json:"queue_push_duration_in_millis"`
	DurationInMillis          float64 `json:"duration_in_millis"`
	In                        float64 `json:"in"`
	Out                       float64 `json:"out"`
}

type Plugin struct {
	ID     string       `json:"id"`
	Events PluginEvents `json:"events"`
	Name   string       `json:"name"`
}

type PipelinePlugins struct {
	Inputs  []Plugin `json:"inputs"`
	Filters []Plugin `json:"filters"`
	Outputs []Plugin `json:"outputs"`
}

type PipelineQueue struct {
	Events   float64     `json:"events"`
	Qtype    string      `json:"type"`
	Capacity interface{} `json:"capacity"`
	Data     interface{} `json:"data"`
}

type Pipeline struct {
	Events  interface{}     `json:"events"`
	Plugins PipelinePlugins `json:"plugins"`
	Reloads interface{}     `json:"reloads"`
	Queue   PipelineQueue   `json:"queue"`
}

type PipelineStats struct {
	ID       string   `json:"id"`
	Pipeline Pipeline `json:"pipeline"`
	Name     string   `json:"name"`
	Host     string   `json:"host"`
	Version  string   `json:"version"`
}

type PipelinesStats struct {
	ID        string              `json:"id"`
	Pipelines map[string]Pipeline `json:"pipelines"`
	Name      string              `json:"name"`
	Host      string              `json:"host"`
	Version   string              `json:"version"`
}

//Description returns short info about plugin
func (logstash *Logstash) Description() string { return "Read metrics exposed by Logstash" }

//SampleConfig returns details how to configure plugin
func (logstash *Logstash) SampleConfig() string { return sampleConfig }

//createHttpClient create clients to access API
func (logstash *Logstash) createHTTPClient() (*http.Client, error) {
	tlsCfg, err := logstash.ClientConfig.TLSConfig()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
		Timeout: logstash.Timeout.Duration,
	}

	return client, nil
}

func (logstash *Logstash) gatherJsonData(url string, v interface{}) (err error) {

	request, err := http.NewRequest("GET", url, nil)

	if (logstash.Username != "") || (logstash.Password != "") {
		request.SetBasicAuth(logstash.Username, logstash.Password)
	}

	response, err := logstash.client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if err = json.NewDecoder(response.Body).Decode(v); err != nil {
		return err
	}

	return nil
}

func (logstash *Logstash) gatherJVMStats(url string, accumulator telegraf.Accumulator) error {
	JVMStats := &JVMStats{}

	err := logstash.gatherJsonData(url, JVMStats)
	if err != nil {
		return err
	}

	tags := map[string]string{
		"node_id":      JVMStats.ID,
		"node_name":    JVMStats.Name,
		"node_host":    JVMStats.Host,
		"node_version": JVMStats.Version,
	}

	stats := map[string]interface{}{
		"jvm": JVMStats.JVM,
	}

	for part, stat := range stats {
		flattener := jsonparser.JSONFlattener{}
		// parse Json, ignoring strings and booleans
		err := flattener.FlattenJSON("", stat)
		if err != nil {
			return err
		}
		accumulator.AddFields("logstash_"+part, flattener.Fields, tags)
	}

	return nil
}

func (logstash *Logstash) gatherProcessStats(url string, accumulator telegraf.Accumulator) error {
	ProcessStats := &ProcessStats{}

	err := logstash.gatherJsonData(url, ProcessStats)
	if err != nil {
		return err
	}

	tags := map[string]string{
		"node_id":      ProcessStats.ID,
		"node_name":    ProcessStats.Name,
		"node_host":    ProcessStats.Host,
		"node_version": ProcessStats.Version,
	}

	stats := map[string]interface{}{
		"process": ProcessStats.Process,
	}

	for part, stat := range stats {
		flattener := jsonparser.JSONFlattener{}
		// parse Json, ignoring strings and booleans
		err := flattener.FlattenJSON("", stat)
		if err != nil {
			return err
		}
		accumulator.AddFields("logstash_"+part, flattener.Fields, tags)
	}

	return nil
}

func (logstash *Logstash) gatherPipelineStats(url string, accumulator telegraf.Accumulator) error {
	PipelineStats := &PipelineStats{}

	err := logstash.gatherJsonData(url, PipelineStats)
	if err != nil {
		return err
	}

	tags := map[string]string{
		"node_id":      PipelineStats.ID,
		"node_name":    PipelineStats.Name,
		"node_host":    PipelineStats.Host,
		"node_version": PipelineStats.Version,
	}

	stats := map[string]interface{}{
		"events": PipelineStats.Pipeline.Events,
	}

	// Events
	for part, stat := range stats {
		flattener := jsonparser.JSONFlattener{}
		// parse Json, ignoring strings and booleans
		err := flattener.FlattenJSON("", stat)
		if err != nil {
			return err
		}
		accumulator.AddFields("logstash_"+part, flattener.Fields, tags)
	}

	if logstash.CollectPluginsStats {
		// Input Plugins
		for _, plugin := range PipelineStats.Pipeline.Plugins.Inputs {
			//plugin := &plugin
			fields := map[string]interface{}{
				"queue_push_duration_in_millis": plugin.Events.QueuePushDurationInMillis,
				"duration_in_millis":            plugin.Events.DurationInMillis,
				"in":                            plugin.Events.In,
				"out":                           plugin.Events.Out,
			}
			tags := map[string]string{
				"node_id":      PipelineStats.ID,
				"node_name":    PipelineStats.Name,
				"node_host":    PipelineStats.Host,
				"node_version": PipelineStats.Version,
				"plugin_name":  plugin.Name,
				"plugin_id":    plugin.ID,
				"plugin_type":  "input",
			}
			accumulator.AddFields("logstash_plugins", fields, tags)
		}

		// Filters Plugins
		for _, plugin := range PipelineStats.Pipeline.Plugins.Filters {
			//plugin := &plugin
			fields := map[string]interface{}{
				"duration_in_millis": plugin.Events.DurationInMillis,
				"in":                 plugin.Events.In,
				"out":                plugin.Events.Out,
			}
			tags := map[string]string{
				"node_id":      PipelineStats.ID,
				"node_name":    PipelineStats.Name,
				"node_host":    PipelineStats.Host,
				"node_version": PipelineStats.Version,
				"plugin_name":  plugin.Name,
				"plugin_id":    plugin.ID,
				"plugin_type":  "filter",
			}
			accumulator.AddFields("logstash_plugins", fields, tags)
		}

		// Output Plugins
		for _, plugin := range PipelineStats.Pipeline.Plugins.Outputs {
			//plugin := &plugin
			fields := map[string]interface{}{
				"duration_in_millis": plugin.Events.DurationInMillis,
				"in":                 plugin.Events.In,
				"out":                plugin.Events.Out,
			}
			tags := map[string]string{
				"node_id":      PipelineStats.ID,
				"node_name":    PipelineStats.Name,
				"node_host":    PipelineStats.Host,
				"node_version": PipelineStats.Version,
				"plugin_name":  plugin.Name,
				"plugin_id":    plugin.ID,
				"plugin_type":  "output",
			}
			accumulator.AddFields("logstash_plugins", fields, tags)
		}
	}

	return nil
}

func (logstash *Logstash) gatherPipelinesStats(url string, accumulator telegraf.Accumulator) error {
	PipelinesStats := &PipelinesStats{}

	err := logstash.gatherJsonData(url, PipelinesStats)
	if err != nil {
		return err
	}

	for pipelineName, pipeline := range PipelinesStats.Pipelines {
		stats := map[string]interface{}{
			"events": pipeline.Events,
		}

		tags := map[string]string{
			"node_id":      PipelinesStats.ID,
			"node_name":    PipelinesStats.Name,
			"node_host":    PipelinesStats.Host,
			"node_version": PipelinesStats.Version,
			"pipeline":     pipelineName,
		}

		// Events
		for part, stat := range stats {
			flattener := jsonparser.JSONFlattener{}
			// parse Json, ignoring strings and booleans
			err := flattener.FlattenJSON("", stat)
			if err != nil {
				return err
			}
			accumulator.AddFields("logstash_"+part, flattener.Fields, tags)
		}

		if logstash.CollectPluginsStats {
			// Input Plugins
			for _, plugin := range pipeline.Plugins.Inputs {
				//plugin := &plugin
				fields := map[string]interface{}{
					"queue_push_duration_in_millis": plugin.Events.QueuePushDurationInMillis,
					"duration_in_millis":            plugin.Events.DurationInMillis,
					"in":                            plugin.Events.In,
					"out":                           plugin.Events.Out,
				}
				tags := map[string]string{
					"node_id":      PipelinesStats.ID,
					"node_name":    PipelinesStats.Name,
					"node_host":    PipelinesStats.Host,
					"node_version": PipelinesStats.Version,
					"pipeline":     pipelineName,
					"plugin_name":  plugin.Name,
					"plugin_id":    plugin.ID,
					"plugin_type":  "input",
				}
				accumulator.AddFields("logstash_plugins", fields, tags)
			}

			// Filters Plugins
			for _, plugin := range pipeline.Plugins.Filters {
				//plugin := &plugin
				fields := map[string]interface{}{
					"duration_in_millis": plugin.Events.DurationInMillis,
					"in":                 plugin.Events.In,
					"out":                plugin.Events.Out,
				}
				tags := map[string]string{
					"node_id":      PipelinesStats.ID,
					"node_name":    PipelinesStats.Name,
					"node_host":    PipelinesStats.Host,
					"node_version": PipelinesStats.Version,
					"pipeline":     pipelineName,
					"plugin_name":  plugin.Name,
					"plugin_id":    plugin.ID,
					"plugin_type":  "filter",
				}
				accumulator.AddFields("logstash_plugins", fields, tags)
			}

			// Output Plugins
			for _, plugin := range pipeline.Plugins.Outputs {
				//plugin := &plugin
				fields := map[string]interface{}{
					"duration_in_millis": plugin.Events.DurationInMillis,
					"in":                 plugin.Events.In,
					"out":                plugin.Events.Out,
				}
				tags := map[string]string{
					"node_id":      PipelinesStats.ID,
					"node_name":    PipelinesStats.Name,
					"node_host":    PipelinesStats.Host,
					"node_version": PipelinesStats.Version,
					"pipeline":     pipelineName,
					"plugin_name":  plugin.Name,
					"plugin_id":    plugin.ID,
					"plugin_type":  "output",
				}
				accumulator.AddFields("logstash_plugins", fields, tags)
			}
		}
	}

	return nil
}

//Gather is main function to gather all metrics provided by this plugin
func (logstash *Logstash) Gather(accumulator telegraf.Accumulator) error {

	if logstash.client == nil {
		client, err := logstash.createHTTPClient()

		if err != nil {
			return err
		}
		logstash.client = client
	}

	if logstash.CollectJVMStats {
		jvmUrl, err := url.Parse(logstash.URL + jvmStats)
		if err != nil {
			return err
		}
		if err := logstash.gatherJVMStats(jvmUrl.String(), accumulator); err != nil {
			return err
		}
	}

	if logstash.CollectProcessStats {
		processUrl, err := url.Parse(logstash.URL + processStats)
		if err != nil {
			return err
		}
		if err := logstash.gatherProcessStats(processUrl.String(), accumulator); err != nil {
			return err
		}
	}

	if logstash.CollectPipelinesStats {
		if logstash.MultiPipeline {
			pipelinesUrl, err := url.Parse(logstash.URL + pipelinesStats)
			if err != nil {
				return err
			}
			if err := logstash.gatherPipelinesStats(pipelinesUrl.String(), accumulator); err != nil {
				return err
			}
		} else {
			pipelineUrl, err := url.Parse(logstash.URL + pipelineStats)
			if err != nil {
				return err
			}
			if err := logstash.gatherPipelineStats(pipelineUrl.String(), accumulator); err != nil {
				return err
			}
		}
	}

	return nil
}

func init() {
	inputs.Add("logstash", func() telegraf.Input {
		return NewLogstash()
	})
}
