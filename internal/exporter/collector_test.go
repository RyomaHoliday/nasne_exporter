package exporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/ryomaholiday/nasne_exporter/internal/nasne"
)

type fakeFetcher struct {
	snapshot nasne.Snapshot
	err      error
}

func (f fakeFetcher) FetchSnapshot(_ context.Context) (nasne.Snapshot, error) {
	if f.err != nil {
		return nasne.Snapshot{}, f.err
	}
	return f.snapshot, nil
}

func TestCollectorUpMetricOnSuccess(t *testing.T) {
	c := NewCollector(fakeFetcher{snapshot: nasne.Snapshot{Name: "nasne"}}, time.Second)
	r := prometheus.NewRegistry()
	r.MustRegister(c)

	if got := testutil.ToFloat64(prometheus.NewGaugeFunc(prometheus.GaugeOpts{}, func() float64 {
		mfs, _ := r.Gather()
		for _, mf := range mfs {
			if mf.GetName() == "nasne_up" && len(mf.Metric) > 0 {
				return mf.Metric[0].GetGauge().GetValue()
			}
		}
		return -1
	})); got != 1 {
		t.Fatalf("expected nasne_up=1, got %v", got)
	}
}

func TestCollectorHealthyOnError(t *testing.T) {
	c := NewCollector(fakeFetcher{err: errors.New("boom")}, time.Second)
	r := prometheus.NewRegistry()
	r.MustRegister(c)
	_, _ = r.Gather()

	if c.Healthy() {
		t.Fatal("collector should be unhealthy after scrape error")
	}
}
