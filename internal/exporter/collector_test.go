package exporter

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

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

func TestCollectorHealthyOnSuccess(t *testing.T) {
	c := NewCollector([]TargetFetcher{{Target: "http://192.168.11.1:64210", Fetcher: fakeFetcher{snapshot: nasne.Snapshot{Name: "nasne-a"}}}}, time.Second)
	r := prometheus.NewRegistry()
	r.MustRegister(c)

	_, err := r.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	if !c.Healthy() {
		t.Fatal("collector should be healthy after successful scrape")
	}
}

func TestCollectorHealthyOnPartialError(t *testing.T) {
	c := NewCollector([]TargetFetcher{
		{Target: "http://192.168.11.1:64210", Fetcher: fakeFetcher{snapshot: nasne.Snapshot{Name: "nasne-a"}}},
		{Target: "http://192.168.11.2:64210", Fetcher: fakeFetcher{err: errors.New("boom")}},
	}, time.Second)
	r := prometheus.NewRegistry()
	r.MustRegister(c)

	_, err := r.Gather()
	if err != nil {
		t.Fatalf("gather failed: %v", err)
	}

	if c.Healthy() {
		t.Fatal("collector should be unhealthy when at least one target fails")
	}
}
