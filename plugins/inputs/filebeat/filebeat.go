package filebeat

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/internal/tls"
	"github.com/influxdata/telegraf/plugins/inputs"

	jsonparser "github.com/influxdata/telegraf/plugins/parsers/json"
)

const sampleConfig = `
[[inputs.filebeat]]
  ## An URL from which to read Filebeat-formatted JSON
  ## Default is "http://127.0.0.1:5066".
  url = "http://127.0.0.1:5066"

  ## Enable collection of the generic Beat stats
  collect_beat_stats = true

  ## Enable the collection if Libbeat stats
  collect_libbeat_stats = true

  ## Enable the collection of OS level stats
  collect_system_stats = false

  ## Enable the collection of Filebeat stats
  collect_filebeat_stats = true

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

const SuffixInfo = "/"
const SuffixStats = "/stats"

type FileBeatInfo struct {
	Beat     string `json:"beat"`
	Hostname string `json:"hostname"`
	Name     string `json:"name"`
	UUID     string `json:"uuid"`
	Version  string `json:"version"`
}

type FileBeatStats struct {
	Beat     map[string]interface{} `json:"beat"`
	Filebeat interface{}            `json:"filebeat"`
	Libbeat  interface{}            `json:"libbeat"`
	System   interface{}            `json:"system"`
}

type Filebeat struct {
	URL string `toml:"url"`

	CollectBeatStats     bool `toml:"collect_beat_stats"`
	CollectLibbeatStats  bool `toml:"collect_libbeat_stats"`
	CollectSystemStats   bool `toml:"collect_system_stats"`
	CollectFilebeatStats bool `toml:"collect_filebeat_stats"`

	Username string            `toml:"username"`
	Password string            `toml:"password"`
	Timeout  internal.Duration `toml:"timeout"`

	tls.ClientConfig
	client *http.Client
}

func NewFilebeat() *Filebeat {
	return &Filebeat{
		URL:                  "http://127.0.0.1:5066",
		CollectBeatStats:     true,
		CollectLibbeatStats:  true,
		CollectSystemStats:   true,
		CollectFilebeatStats: true,
		Timeout:              internal.Duration{Duration: time.Second * 5},
	}
}

func (filebeat *Filebeat) Description() string { return "Read metrics exposed by Filebeat" }

func (filebeat *Filebeat) SampleConfig() string { return sampleConfig }

func (filebeat *Filebeat) createHTTPClient() (*http.Client, error) {
	tlsCfg, err := filebeat.ClientConfig.TLSConfig()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsCfg,
		},
		Timeout: filebeat.Timeout.Duration,
	}

	return client, nil
}

func (filebeat *Filebeat) gatherJsonData(url string, v interface{}) (err error) {

	request, err := http.NewRequest("GET", url, nil)

	if (filebeat.Username != "") || (filebeat.Password != "") {
		request.SetBasicAuth(filebeat.Username, filebeat.Password)
	}

	response, err := filebeat.client.Do(request)
	if err != nil {
		return err
	}

	defer response.Body.Close()

	if err = json.NewDecoder(response.Body).Decode(v); err != nil {
		return err
	}

	return nil
}

func (filebeat *Filebeat) gatherInfoTags(url string) (map[string]string, error) {
	fileBeatInfo := &FileBeatInfo{}

	err := filebeat.gatherJsonData(url, fileBeatInfo)
	if err != nil {
		return nil, err
	}

	tags := map[string]string{
		"beat_id":      fileBeatInfo.UUID,
		"beat_name":    fileBeatInfo.Name,
		"beat_host":    fileBeatInfo.Hostname,
		"beat_version": fileBeatInfo.Version,
	}

	return tags, nil
}

func (filebeat *Filebeat) gatherStats(accumulator telegraf.Accumulator) error {
	fileBeatStats := &FileBeatStats{}

	infoUrl, err := url.Parse(filebeat.URL + SuffixInfo)
	if err != nil {
		return err
	}
	statsUrl, err := url.Parse(filebeat.URL + SuffixStats)
	if err != nil {
		return err
	}

	tags, err := filebeat.gatherInfoTags(infoUrl.String())
	if err != nil {
		return err
	}

	err = filebeat.gatherJsonData(statsUrl.String(), fileBeatStats)
	if err != nil {
		return err
	}

	if filebeat.CollectBeatStats {
		flattenerBeat := jsonparser.JSONFlattener{}
		err := flattenerBeat.FlattenJSON("", fileBeatStats.Beat)
		if err != nil {
			return err
		}
		accumulator.AddFields("filebeat_beat", flattenerBeat.Fields, tags)
	}

	if filebeat.CollectFilebeatStats {
		flattenerFilebeat := jsonparser.JSONFlattener{}
		err := flattenerFilebeat.FlattenJSON("", fileBeatStats.Filebeat)
		if err != nil {
			return err
		}
		accumulator.AddFields("filebeat", flattenerFilebeat.Fields, tags)
	}

	if filebeat.CollectLibbeatStats {
		flattenerLibbeat := jsonparser.JSONFlattener{}
		err := flattenerLibbeat.FlattenJSON("", fileBeatStats.Libbeat)
		if err != nil {
			return err
		}
		accumulator.AddFields("filebeat_libbeat", flattenerLibbeat.Fields, tags)
	}

	if filebeat.CollectSystemStats {
		flattenerSystem := jsonparser.JSONFlattener{}
		err := flattenerSystem.FlattenJSON("", fileBeatStats.System)
		if err != nil {
			return err
		}
		accumulator.AddFields("filebeat_system", flattenerSystem.Fields, tags)
	}

	return nil
}

func (filebeat *Filebeat) Gather(accumulator telegraf.Accumulator) error {
	if filebeat.client == nil {
		client, err := filebeat.createHTTPClient()

		if err != nil {
			return err
		}
		filebeat.client = client
	}

	err := filebeat.gatherStats(accumulator)
	if err != nil {
		return err
	}

	return nil
}

func init() {
	inputs.Add("filebeat", func() telegraf.Input {
		return NewFilebeat()
	})
}
