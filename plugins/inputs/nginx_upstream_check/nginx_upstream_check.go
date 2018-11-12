package nginx_upstream_check

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/internal"
	"github.com/influxdata/telegraf/internal/tls"
	"github.com/influxdata/telegraf/plugins/inputs"
)

type NginxUpstreamCheck struct {
	Urls            []string
	ResponseTimeout internal.Duration

	tls.ClientConfig

	client *http.Client
}

type NginxUpstreamCheckData struct {
	Servers struct {
		Total      uint64                     `json:"total"`
		Generation uint64                     `json:"generation"`
		Server     []NginxUpstreamCheckServer `json:"server"`
	} `json:"servers"`
}

type NginxUpstreamCheckServer struct {
	Index    uint64 `json:"index"`
	Upstream string `json:"upstream"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Rise     uint64 `json:"rise"`
	Fall     uint64 `json:"fall"`
	Type     string `json:"type"`
	Port     uint16 `json:"port"`
}

var sampleConfig = `
  ## An array of nginx_upstream_check URLs
  ## They should be set to return a JSON formatted response
  urls = ["http://localhost/status?format=json"]

  ## HTTP response timeout (default: 5s)
  # response_timeout = "5s"

  ## Optional TLS Config
  # tls_ca = "/etc/telegraf/ca.pem"
  # tls_cert = "/etc/telegraf/cert.pem"
  # tls_key = "/etc/telegraf/key.pem"
  ## Use TLS but skip chain & host verification
  # insecure_skip_verify = false
`

var description = "Read nginx_upstream_check module status information (https://github.com/yaoweibin/nginx_upstream_check_module)"

func (check *NginxUpstreamCheck) SampleConfig() string {
	return sampleConfig
}

func (check *NginxUpstreamCheck) Description() string {
	return description
}

func (check *NginxUpstreamCheck) Gather(accumulator telegraf.Accumulator) error {
	var waitGroup sync.WaitGroup

	if check.client == nil {
		client, err := check.createHTTPClient()
		if err != nil {
			return err
		}
		check.client = client
	}

	for _, address := range check.Urls {
		address, err := url.Parse(address)
		if err != nil {
			accumulator.AddError(fmt.Errorf("Unable to parse address '%s': %s", address, err))
			continue
		}

		waitGroup.Add(1)
		go func(address *url.URL) {
			defer waitGroup.Done()
			accumulator.AddError(check.gatherURL(address, accumulator))
		}(address)
	}

	waitGroup.Wait()
	return nil
}

func (check *NginxUpstreamCheck) createHTTPClient() (*http.Client, error) {
	tlsConfig, err := check.ClientConfig.TLSConfig()
	if err != nil {
		return nil, err
	}

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: tlsConfig,
		},
		Timeout: check.ResponseTimeout.Duration,
	}

	return client, nil
}

func (check *NginxUpstreamCheck) gatherURL(address *url.URL, accumulator telegraf.Accumulator) error {
	response, err := check.client.Get(address.String())

	if err != nil {
		return fmt.Errorf("error making HTTP request to %s: %s", address.String(), err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s returned HTTP status %s", address.String(), response.Status)
	}
	contentType := strings.Split(response.Header.Get("Content-Type"), ";")[0]
	switch contentType {
	case "application/json":
		return gatherStatusURL(bufio.NewReader(response.Body), address, accumulator)
	default:
		return fmt.Errorf("%s returned unexpected content type %s", address.String(), contentType)
	}
}

func gatherStatusURL(reader *bufio.Reader, address *url.URL, accumulator telegraf.Accumulator) error {
	decoder := json.NewDecoder(reader)
	checkData := &NginxUpstreamCheckData{}
	err := decoder.Decode(checkData)
	if err != nil {
		return fmt.Errorf("Error while decoding JSON response")
	}

	for _, server := range checkData.Servers.Server {
		accumulator.AddFields("nginx_upstream_check", getFields(server), getTags(server, address))
	}

	return nil
}

func statusCode(status string) uint8 {
	switch status {
	case "up":
		return 1
	case "down":
		return 2
	default:
		return 0
	}
}

func getFields(server NginxUpstreamCheckServer) map[string]interface{} {
	return map[string]interface{}{
		"status":      server.Status,
		"status_code": statusCode(server.Status),
		"rise":        server.Rise,
		"fall":        server.Fall,
	}
}

func getTags(server NginxUpstreamCheckServer, address *url.URL) map[string]string {
	return map[string]string{
		"upstream": server.Upstream,
		"type":     server.Type,
		"name":     server.Name,
		"port":     fmt.Sprintf("%d", server.Port),
		"url":      address.String(),
	}
}

func init() {
	inputs.Add("nginx_upstream_check", func() telegraf.Input {
		return &NginxUpstreamCheck{}
	})
}
