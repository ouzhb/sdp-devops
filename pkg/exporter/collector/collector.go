package collector

import (
	mapset "github.com/deckarep/golang-set"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sirupsen/logrus"
	"os"
	"sdp-devops/pkg/exporter/config"
	"strings"
	"sync"
	"time"
)

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"监控数据采集时间。",
		[]string{"collector", "node"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"监控数据是否正常采集。",
		[]string{"collector", "node"},
		nil,
	)

	factories = make(map[string]func() (Collector, error))
	excluding = mapset.NewSet()
	including = mapset.NewSet()
)

const namespace = "sdp"

// SDPCollector 最上层的采集器
type SDPCollector struct {
	Collectors map[string]Collector
}

// 实现 prometheus.Collector 的 Describe 接口
func (n SDPCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- scrapeDurationDesc
	ch <- scrapeSuccessDesc
}

// 实现 prometheus.Collector 的 Describe 接口 Collect
func (n SDPCollector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(n.Collectors))
	for name, c := range n.Collectors {
		go func(name string, c Collector) {
			execute(name, c, ch)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

// 上层采集器调用子异步调用子采集器Update接口，更新采集的数据
func execute(name string, c Collector, ch chan<- prometheus.Metric) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64
	nodename, _ := os.Hostname()
	if err != nil {
		logrus.Errorf("%s,数据采集异常", name)
		success = 0
	} else {
		logrus.Infof("%s,数据采集正常，持续时间：%f", name, duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name, nodename)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name, nodename)
}

// Collector 所有指标采集类实现Collector接口用于更新指标
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) error
}

func registerCollector(collector string, factory func() (Collector, error)) {
	factories[collector] = factory
}

func disabled(collector string) bool {
	isDisabled := excluding.Contains(collector)

	if !isDisabled && strings.EqualFold(config.IncludingCol, "") {
		// 没进黑名单，且没有配置白名单
		return false
	}
	return !including.Contains(collector)

}

// 创建SDPCollector
func NewNodeCollector() (*SDPCollector, error) {

	for _, s := range strings.Split(config.ExcludingCol, ",") {
		excluding.Add(s)
	}
	for _, s := range strings.Split(config.IncludingCol, ",") {
		including.Add(s)
	}
	logrus.Infof("采集器白名单：%s", including.String())
	logrus.Infof("采集器黑名单：%s", excluding.String())

	collectors := make(map[string]Collector)
	for key, f := range factories {
		if disabled(key) {
			logrus.Infof("禁用采集器：%s", key)
			continue
		} else {
			logrus.Infof("启用采集器：%s", key)
			collector, err := f()
			if err != nil {
				return nil, err
			}
			collectors[key] = collector
		}

	}
	return &SDPCollector{Collectors: collectors}, nil
}
