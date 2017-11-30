package burrow

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/fortytw2/leaktest"
	"github.com/influxdata/telegraf/testutil"
	"github.com/stretchr/testify/require"
)

// remap uri to json file, eg: /v2/kafka -> ./testdata/v2_kafka.json
func getResponseJSON(requestURI string) ([]byte, int) {
	uri := strings.TrimLeft(requestURI, "/")
	mappedFile := strings.Replace(uri, "/", "_", -1)
	jsonFile := fmt.Sprintf("./testdata/%s.json", mappedFile)

	code := 200
	_, err := os.Stat(jsonFile)
	if err != nil {
		code = 404
		jsonFile = "./testdata/error.json"
	}

	// respond with file
	b, _ := ioutil.ReadFile(jsonFile)
	return b, code
}

// return mocked HTTP server
func getHTTPServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, code := getResponseJSON(r.RequestURI)
		w.WriteHeader(code)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

// return mocked HTTP server with basic auth
func getHTTPServerBasicAuth() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

		username, password, authOK := r.BasicAuth()
		if authOK == false {
			http.Error(w, "Not authorized", 401)
			return
		}

		if username != "test" && password != "test" {
			http.Error(w, "Not authorized", 401)
			return
		}

		// ok, continue
		body, code := getResponseJSON(r.RequestURI)
		w.WriteHeader(code)
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

// test burrow_topic measurement
func TestTopicOffsetMeasurement(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServer()
	defer s.Close()

	plugin := &burrow{
		Servers:             []string{s.URL},
		DisableGroupSummary: true,
		DisableGroupTopics:  true,
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	fields := []map[string]interface{}{
		// topicA
		{"offset": int64(1000000)},
		{"offset": int64(1000001)},
		{"offset": int64(1000002)},
		// topicB
		{"offset": int64(2000000)},
		// topicC
		{"offset": int64(3000000)},
	}
	tags := []map[string]string{
		// topicA
		{"cluster": "clustername1", "topic": "topicA", "partition": "0"},
		{"cluster": "clustername1", "topic": "topicA", "partition": "1"},
		{"cluster": "clustername1", "topic": "topicA", "partition": "2"},
		// topicB
		{"cluster": "clustername1", "topic": "topicB", "partition": "0"},
		// topicC
		{"cluster": "clustername2", "topic": "topicC", "partition": "0"},
	}

	require.Exactly(t, 5, len(acc.Metrics)) // (5 burrow_topic)
	require.Empty(t, acc.Errors)
	require.Equal(t, true, acc.HasMeasurement("burrow_topic_offset"))

	for i := 0; i < len(fields); i++ {
		acc.AssertContainsTaggedFields(t, "burrow_topic_offset", fields[i], tags[i])
	}
}

// test burrow_consumer measurement
func TestGroupTopicMeasurement(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServer()
	defer s.Close()

	plugin := &burrow{
		Servers:             []string{s.URL},
		DisableTopics:       true,
		DisableGroupSummary: true,
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	fields := []map[string]interface{}{
		// group1
		{
			"start.offset":    int64(823889),
			"start.lag":       int64(20),
			"start.timestamp": int64(1432423256000),
			"end.offset":      int64(824743),
			"end.lag":         int64(25),
			"end.timestamp":   int64(1432423796001),
			"status":          "WARN",
			"status_code":     3,
		},
		// group2
		{
			"start.offset":    int64(823890),
			"start.lag":       int64(20),
			"start.timestamp": int64(1432423256002),
			"end.offset":      int64(824745),
			"end.lag":         int64(26),
			"end.timestamp":   int64(1432423796003),
			"status":          "OK",
			"status_code":     1,
		},
		// group3
		{
			"start.offset":    int64(523889),
			"start.lag":       int64(11),
			"start.timestamp": int64(1432423256000),
			"end.offset":      int64(524743),
			"end.lag":         int64(26),
			"end.timestamp":   int64(1432423796000),
			"status":          "ERR",
			"status_code":     4,
		},
	}
	tags := []map[string]string{
		{"cluster": "clustername1", "group": "group1", "topic": "topicA", "partition": "0"},
		{"cluster": "clustername1", "group": "group2", "topic": "topicB", "partition": "0"},
		{"cluster": "clustername2", "group": "group3", "topic": "topicC", "partition": "0"},
	}

	require.Exactly(t, 3, len(acc.Metrics))
	require.Empty(t, acc.Errors)
	require.Equal(t, true, acc.HasMeasurement("burrow_group_topic"))

	for i := 0; i < len(fields); i++ {
		acc.AssertContainsTaggedFields(t, "burrow_group_topic", fields[i], tags[i])
	}
}

func TestGroupSummaryMeasurement(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServer()
	defer s.Close()

	plugin := &burrow{
		Servers:            []string{s.URL},
		DisableTopics:      true,
		DisableGroupTopics: true,
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	fields := []map[string]interface{}{
		{
			"status":      "WARN",
			"status_code": 3,
			//
			"maxlag.topic":       "topicA",
			"maxlag.parittion":   int32(0),
			"maxlag.status":      "WARN",
			"maxlag.status_code": 3,
			// start
			"maxlag.start.offset":    int64(823889),
			"maxlag.start.timestamp": int64(1432423256000),
			"maxlag.start.lag":       int64(20),
			// end
			"maxlag.end.offset":    int64(824743),
			"maxlag.end.timestamp": int64(1432423796001),
			"maxlag.end.lag":       int64(25),
		},
		{
			"status":      "OK",
			"status_code": 1,
			//
			"maxlag.topic":       "topicB",
			"maxlag.parittion":   int32(0),
			"maxlag.status":      "WARN",
			"maxlag.status_code": 3,
			// start
			"maxlag.start.offset":    int64(823889),
			"maxlag.start.timestamp": int64(1432423256002),
			"maxlag.start.lag":       int64(20),
			// end
			"maxlag.end.offset":    int64(824743),
			"maxlag.end.timestamp": int64(1432423796003),
			"maxlag.end.lag":       int64(26),
		},
		{
			"status":      "WARN",
			"status_code": 3,
		},
	}

	tags := []map[string]string{
		{"cluster": "clustername1", "group": "group1"},
		{"cluster": "clustername1", "group": "group2"},
		{"cluster": "clustername2", "group": "group3"},
	}

	require.Exactly(t, 3, len(acc.Metrics))
	require.Empty(t, acc.Errors)
	require.Equal(t, true, acc.HasMeasurement("burrow_group_summary"))

	for i := 0; i < len(fields); i++ {
		acc.AssertContainsTaggedFields(t, "burrow_group_summary", fields[i], tags[i])
	}
}

// collect from multiple servers
func TestMultipleServers(t *testing.T) {
	defer leaktest.Check(t)()

	s1 := getHTTPServer()
	defer s1.Close()

	s2 := getHTTPServer()
	defer s2.Close()

	plugin := &burrow{
		Servers: []string{s1.URL, s2.URL},
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	// (5 burrow_topic, 3 burrow_group_topic, 3 burrow_group_summary) * 2
	require.Exactly(t, 22, len(acc.Metrics))
	require.Empty(t, acc.Errors)
}

// collect multiple times
func TestMultipleRuns(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServer()
	defer s.Close()

	plugin := &burrow{
		Servers: []string{s.URL},
	}

	for i := 0; i < 4; i++ {
		acc := &testutil.Accumulator{}
		plugin.Gather(acc)

		// 5 burrow_topic, 3 burrow_group_topic, 3 burrow_group_summary
		require.Exactly(t, 11, len(acc.Metrics))
		require.Empty(t, acc.Errors)
	}
}

// collect from http basic auth server (plugin wide config)
func TestBasicAuthConfig(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServerBasicAuth()
	defer s.Close()

	plugin := &burrow{
		Servers:  []string{s.URL},
		Username: "test",
		Password: "test",
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	// 5 burrow_topic, 3 burrow_group_topic, 3 burrow_group_summary
	require.Exactly(t, 11, len(acc.Metrics))
	require.Empty(t, acc.Errors)
}

// collect from http basic auth server (endpoint config)
func TestBasicAuthEndpoint(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServerBasicAuth()
	defer s.Close()

	u, err := url.Parse(s.URL)
	require.NoError(t, err)

	serverURL := url.URL{
		Scheme: "http",
		User:   url.UserPassword("test", "test"),
		Host:   u.Host,
	}

	plugin := &burrow{
		Servers:  []string{serverURL.String()},
		Username: "invalid_username",
		Password: "invalid_password",
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	// 5 burrow_topic, 3 burrow_group_topic, 3 burrow_group_summary
	require.Exactly(t, 11, len(acc.Metrics))
	require.Empty(t, acc.Errors)
}

// collect from whitelisted clusters
func TestLimitClusters(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServer()
	defer s.Close()

	plugin := &burrow{
		Servers:  []string{s.URL},
		Clusters: []string{"non_existent_cluster"}, // disable clusters
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	// no match by cluster
	require.Exactly(t, 0, len(acc.Metrics))
	require.Empty(t, acc.Errors)
}

// collect from whitelisted groups
func TestLimitGroups(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServer()
	defer s.Close()

	plugin := &burrow{
		Servers:       []string{s.URL},
		Groups:        []string{"group2"},
		DisableTopics: true,
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	// 1 burrow_group_topic, 1 burrow_group_summary
	require.Exactly(t, 2, len(acc.Metrics))
	require.Empty(t, acc.Errors)
}

// collect from whitelisted topics
func TestLimitTopics(t *testing.T) {
	defer leaktest.Check(t)()

	s := getHTTPServer()
	defer s.Close()

	plugin := &burrow{
		Servers:             []string{s.URL},
		DisableGroupTopics:  true,
		DisableGroupSummary: true,
		Topics:              []string{"topicB"},
	}

	acc := &testutil.Accumulator{}
	plugin.Gather(acc)

	// 1 burrow_topic
	require.Exactly(t, 1, len(acc.Metrics))
	require.Empty(t, acc.Errors)
}