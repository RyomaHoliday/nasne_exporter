package exporter

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ryomaholiday/nasne_exporter/internal/nasne"
)

type Fetcher interface {
	FetchSnapshot(ctx context.Context) (nasne.Snapshot, error)
}

type Collector struct {
	client  Fetcher
	timeout time.Duration

	mu              sync.RWMutex
	lastScrapeError error

	collectDuration  *prometheus.Desc
	up               *prometheus.Desc
	info             *prometheus.Desc
	hddSizeBytes     *prometheus.Desc
	hddUsageBytes    *prometheus.Desc
	dtcpipClients    *prometheus.Desc
	recordings       *prometheus.Desc
	recordedTitles   *prometheus.Desc
	reservedTitles   *prometheus.Desc
	reservedConflict *prometheus.Desc
	reservedNotFound *prometheus.Desc
}

func NewCollector(client Fetcher, timeout time.Duration) *Collector {
	return &Collector{
		client:           client,
		timeout:          timeout,
		collectDuration:  prometheus.NewDesc("nasne_collect_duration_seconds", "Time spent collecting metrics from nasne.", nil, nil),
		up:               prometheus.NewDesc("nasne_up", "Whether the last scrape from nasne succeeded.", nil, nil),
		info:             prometheus.NewDesc("nasne_info", "nasne device information.", []string{"name", "product_name", "hardware_version", "software_version"}, nil),
		hddSizeBytes:     prometheus.NewDesc("nasne_hdd_size_bytes", "Total HDD size in bytes.", nil, nil),
		hddUsageBytes:    prometheus.NewDesc("nasne_hdd_usage_bytes", "Used HDD size in bytes.", nil, nil),
		dtcpipClients:    prometheus.NewDesc("nasne_dtcpip_clients", "Connected DTCP-IP clients.", nil, nil),
		recordings:       prometheus.NewDesc("nasne_recordings", "Number of current recordings.", nil, nil),
		recordedTitles:   prometheus.NewDesc("nasne_recorded_titles", "Number of recorded titles.", nil, nil),
		reservedTitles:   prometheus.NewDesc("nasne_reserved_titles", "Number of reserved titles.", nil, nil),
		reservedConflict: prometheus.NewDesc("nasne_reserved_conflict_titles", "Number of conflicting reserved titles.", nil, nil),
		reservedNotFound: prometheus.NewDesc("nasne_reserved_notfound_titles", "Number of not-found reserved titles.", nil, nil),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.collectDuration
	ch <- c.up
	ch <- c.info
	ch <- c.hddSizeBytes
	ch <- c.hddUsageBytes
	ch <- c.dtcpipClients
	ch <- c.recordings
	ch <- c.recordedTitles
	ch <- c.reservedTitles
	ch <- c.reservedConflict
	ch <- c.reservedNotFound
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	snapshot, err := c.client.FetchSnapshot(ctx)
	duration := time.Since(start).Seconds()
	ch <- prometheus.MustNewConstMetric(c.collectDuration, prometheus.GaugeValue, duration)

	if err != nil {
		c.mu.Lock()
		c.lastScrapeError = err
		c.mu.Unlock()
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
		return
	}

	c.mu.Lock()
	c.lastScrapeError = nil
	c.mu.Unlock()

	ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)
	ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1, snapshot.Name, snapshot.ProductName, snapshot.HardwareVersion, snapshot.SoftwareVersion)
	ch <- prometheus.MustNewConstMetric(c.hddSizeBytes, prometheus.GaugeValue, snapshot.HDDSizeBytes)
	ch <- prometheus.MustNewConstMetric(c.hddUsageBytes, prometheus.GaugeValue, snapshot.HDDUsageBytes)
	ch <- prometheus.MustNewConstMetric(c.dtcpipClients, prometheus.GaugeValue, snapshot.DTCPIPClients)
	ch <- prometheus.MustNewConstMetric(c.recordings, prometheus.GaugeValue, snapshot.Recordings)
	ch <- prometheus.MustNewConstMetric(c.recordedTitles, prometheus.GaugeValue, snapshot.RecordedTitles)
	ch <- prometheus.MustNewConstMetric(c.reservedTitles, prometheus.GaugeValue, snapshot.ReservedTitles)
	ch <- prometheus.MustNewConstMetric(c.reservedConflict, prometheus.GaugeValue, snapshot.ReservedConflictTitles)
	ch <- prometheus.MustNewConstMetric(c.reservedNotFound, prometheus.GaugeValue, snapshot.ReservedNotFoundTitles)
}

func (c *Collector) Healthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastScrapeError == nil
}
