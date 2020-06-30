package metrics

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Sink is the destination where the metrics are reported to.
type Sink interface {

	// Report the snapshot of metrics to the monitoring system. No retry even if it failed.
	Report()
}

type falconConfig struct {
	falconAgentHost     string
	falconAgentPort     uint32
	falconAgentHTTPPath string

	clusterName           string
	port                  uint32
	metricsReportInterval time.Duration
}

type falconSink struct {
	cfg *falconConfig
}

type falconMetricData struct {
	endpoint    string  // local host
	metric      string  // metric name
	timestamp   int64   // the reporting time in unix seconds
	step        int32   // the reporting time interval in seconds
	value       float64 // metric value
	counterType string  // GAUGE or COUNTER
	tags        string
}

func (sink *falconSink) load() {
	sink.cfg = &falconConfig{
		falconAgentHost:       viper.GetString("falcon_agent.host"),
		falconAgentPort:       viper.GetUint32("falcon_agent.port"),
		falconAgentHTTPPath:   viper.GetString("falcon_agent.http_path"),
		clusterName:           viper.GetString("cluster_name"),
		port:                  viper.GetUint32("port"),
		metricsReportInterval: viper.GetDuration("metrics.report_interval"),
	}
}

func (sink *falconSink) Report() {
	metric := &falconMetricData{
		endpoint:    "",
		step:        int32(sink.cfg.metricsReportInterval.Seconds()),
		counterType: "GAUGE",
		tags:        fmt.Sprintf("service=pegasus,cluster=%s,job=collector,port=%d", sink.cfg.clusterName, sink.cfg.port),
	}
	buf := "["
	snapshots := getAllSnapshots()
	for _, sn := range snapshots {
		metric.metric = sn.name
		metric.value = sn.value
		metric.timestamp = time.Now().Unix()
		buf += sink.snapshotJSONString(metric) + ","
	}
	buf += "]"
	sink.postHTTPData(buf)
}

func (sink *falconSink) snapshotJSONString(m *falconMetricData) string {
	json, err := json.Marshal(m)
	if err != nil {
		log.Fatal("failed to marshall falcon metric to json: ", err)
	}
	return string(json)
}

func (sink *falconSink) postHTTPData(data string) {
	url := fmt.Sprintf("http://%s:%d/%s", sink.cfg.falconAgentHost, sink.cfg.falconAgentPort, sink.cfg.falconAgentHTTPPath)
	resp, err := http.Post(url, "application/x-www-form-urlencoded", bytes.NewReader([]byte(data)))
	if err == nil && resp.StatusCode != http.StatusOK {
		err = errors.New(resp.Status)
	}
	if err != nil {
		log.Errorf("failed to post metrics to falcon agent: %s", err)
		return
	}
}