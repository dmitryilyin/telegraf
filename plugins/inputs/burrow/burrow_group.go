package burrow

import (
	"fmt"
	"net/url"
	"strconv"
	"sync"
)

// fetch consumer groups: /v2/kafka/(cluster)/consumer
func gatherGroupStats(api apiClient, clusterList []string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Does configuration permit publishing any kind of consumer statistic?
	if api.disableGroupSummary == true && api.disableGroupTopics == true {
		return
	}

	producerChan := make(chan string, len(clusterList))
	doneChan := make(chan bool, len(clusterList))

	for i := 0; i < api.workerCount; i++ {
		go withAPICall(api, producerChan, doneChan, fetchGroup)
	}

	for _, cluster := range clusterList {
		escaped := url.PathEscape(cluster)
		producerChan <- fmt.Sprintf("%s/%s/consumer", api.apiPrefix, escaped)
	}

	for i := len(clusterList); i > 0; i-- {
		<-doneChan
	}

	close(producerChan)
}

// fetch consumer status: /v2/kafka/(cluster)/consumer/(group)/status
func fetchGroup(api apiClient, res apiResponse, uri string) {
	groupList := whitelistSlice(res.Groups, api.limitGroups)

	producerChan := make(chan string, len(groupList))
	doneChan := make(chan bool, len(groupList))

	for i := 0; i < api.workerCount; i++ {
		go withAPICall(api, producerChan, doneChan, func(api apiClient, res apiResponse, uri string) {
			publishGroupSummary(api, res, uri)
			publishGroupTopic(api, res, uri)
		})
	}

	for _, group := range groupList {
		escaped := url.PathEscape(group)
		producerChan <- fmt.Sprintf("%s/%s/lag", uri, escaped)
	}

	for i := len(groupList); i > 0; i-- {
		<-doneChan
	}

	close(producerChan)
}

// publish consumer status
func publishGroupSummary(api apiClient, res apiResponse, uri string) {
	if api.disableGroupSummary == true {
		return
	}

	statusCode := remapStatus(res.Status.Status)
	tags := map[string]string{
		"cluster": res.Status.Cluster,
		"group":   res.Status.Group,
	}

	fields := map[string]interface{}{
		"status":      res.Status.Status,
		"status_code": statusCode,
	}

	// if maxlag is present, publish maxlag stats
	if res.Status.Maxlag != nil {
		fields["maxlag.topic"] = res.Status.Maxlag.Topic
		fields["maxlag.parittion"] = res.Status.Maxlag.Partition
		fields["maxlag.status"] = res.Status.Maxlag.Status
		fields["maxlag.status_code"] = remapStatus(res.Status.Maxlag.Status)
		// start
		fields["maxlag.start.offset"] = res.Status.Maxlag.Start.Offset
		fields["maxlag.start.timestamp"] = res.Status.Maxlag.Start.Timestamp
		fields["maxlag.start.lag"] = res.Status.Maxlag.Start.Lag
		// end
		fields["maxlag.end.offset"] = res.Status.Maxlag.End.Offset
		fields["maxlag.end.timestamp"] = res.Status.Maxlag.End.Timestamp
		fields["maxlag.end.lag"] = res.Status.Maxlag.End.Lag
	}

	api.acc.AddFields("burrow_group_summary", fields, tags)
}

func publishGroupTopic(api apiClient, res apiResponse, uri string) {
	if api.disableGroupTopics == true {
		return
	}

	for _, partition := range res.Status.Partitions {
		statusCode := remapStatus(partition.Status)

		tags := map[string]string{
			"cluster":   res.Request.Cluster,
			"group":     res.Request.Group,
			"topic":     partition.Topic,
			"partition": strconv.FormatInt(int64(partition.Partition), 10),
		}

		api.acc.AddFields(
			"burrow_group_topic",
			map[string]interface{}{
				"start.offset":    partition.Start.Offset,
				"start.lag":       partition.Start.Lag,
				"start.timestamp": partition.Start.Timestamp,
				"end.offset":      partition.End.Offset,
				"end.lag":         partition.End.Lag,
				"end.timestamp":   partition.End.Timestamp,
				"status":          partition.Status,
				"status_code":     statusCode,
			},
			tags,
		)
	}
}
