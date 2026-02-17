package exporter

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ryomaholiday/nasne_exporter/internal/nasne"
)

type Fetcher interface {
	FetchSnapshot(ctx context.Context) (nasne.Snapshot, error)
}

type TargetFetcher struct {
	Target  string
	Fetcher Fetcher
}

type collectResult struct {
	target   string
	snapshot nasne.Snapshot
	err      error
	duration float64
}

type Collector struct {
	targets []TargetFetcher
	timeout time.Duration

	mu               sync.RWMutex
	lastScrapeErrors map[string]error
	hasScrapedOnce   bool

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

func NewCollector(targets []TargetFetcher, timeout time.Duration) *Collector {
	return &Collector{
		targets:           targets,
		timeout:           timeout,
		lastScrapeErrors:  map[string]error{},
		collectDuration:   prometheus.NewDesc("nasne_collect_duration_seconds", "Time spent collecting metrics from nasne.", []string{"target"}, nil),
		up:                prometheus.NewDesc("nasne_up", "Whether the last scrape from nasne succeeded.", []string{"target"}, nil),
		info:              prometheus.NewDesc("nasne_info", "nasne device information.", []string{"target", "name", "product_name", "hardware_version", "software_version"}, nil),
		hddSizeBytes:      prometheus.NewDesc("nasne_hdd_size_bytes", "Total HDD size in bytes.", []string{"target"}, nil),
		hddUsageBytes:     prometheus.NewDesc("nasne_hdd_usage_bytes", "Used HDD size in bytes.", []string{"target"}, nil),
		dtcpipClients:     prometheus.NewDesc("nasne_dtcpip_clients", "Connected DTCP-IP clients.", []string{"target"}, nil),
		recordings:        prometheus.NewDesc("nasne_recordings", "Number of current recordings.", []string{"target"}, nil),
		recordedTitles:    prometheus.NewDesc("nasne_recorded_titles", "Number of recorded titles.", []string{"target"}, nil),
		reservedTitles:    prometheus.NewDesc("nasne_reserved_titles", "Number of reserved titles.", []string{"target"}, nil),
		reservedConflict:  prometheus.NewDesc("nasne_reserved_conflict_titles", "Number of conflicting reserved titles.", []string{"target"}, nil),
		reservedNotFound:  prometheus.NewDesc("nasne_reserved_notfound_titles", "Number of not-found reserved titles.", []string{"target"}, nil),
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
	results := make(chan collectResult, len(c.targets))
	var wg sync.WaitGroup

	for _, t := range c.targets {
		target := t
		wg.Add(1)
		go func() {
			defer wg.Done()
			start := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
			snapshot, err := target.Fetcher.FetchSnapshot(ctx)
			cancel()
			results <- collectResult{
				target:   target.Target,
				snapshot: snapshot,
				err:      err,
				duration: time.Since(start).Seconds(),
			}
		}()
	}

	wg.Wait()
	close(results)

	errors := map[string]error{}
	for r := range results {
		ch <- prometheus.MustNewConstMetric(c.collectDuration, prometheus.GaugeValue, r.duration, r.target)

		if r.err != nil {
			errors[r.target] = r.err
			log.Printf("warn: scrape failed for target=%s err=%v", r.target, r.err)
			ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0, r.target)
			continue
		}

		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1, r.target)
		ch <- prometheus.MustNewConstMetric(c.info, prometheus.GaugeValue, 1, r.target, r.snapshot.Name, r.snapshot.ProductName, r.snapshot.HardwareVersion, r.snapshot.SoftwareVersion)
		ch <- prometheus.MustNewConstMetric(c.hddSizeBytes, prometheus.GaugeValue, r.snapshot.HDDSizeBytes, r.target)
		ch <- prometheus.MustNewConstMetric(c.hddUsageBytes, prometheus.GaugeValue, r.snapshot.HDDUsageBytes, r.target)
		ch <- prometheus.MustNewConstMetric(c.dtcpipClients, prometheus.GaugeValue, r.snapshot.DTCPIPClients, r.target)
		ch <- prometheus.MustNewConstMetric(c.recordings, prometheus.GaugeValue, r.snapshot.Recordings, r.target)
		ch <- prometheus.MustNewConstMetric(c.recordedTitles, prometheus.GaugeValue, r.snapshot.RecordedTitles, r.target)
		ch <- prometheus.MustNewConstMetric(c.reservedTitles, prometheus.GaugeValue, r.snapshot.ReservedTitles, r.target)
		ch <- prometheus.MustNewConstMetric(c.reservedConflict, prometheus.GaugeValue, r.snapshot.ReservedConflictTitles, r.target)
		ch <- prometheus.MustNewConstMetric(c.reservedNotFound, prometheus.GaugeValue, r.snapshot.ReservedNotFoundTitles, r.target)
	}

	c.mu.Lock()
	c.lastScrapeErrors = errors
	c.hasScrapedOnce = true
	c.mu.Unlock()
}

func (c *Collector) Healthy() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.targets) > 0 && c.hasScrapedOnce && len(c.lastScrapeErrors) == 0
}
