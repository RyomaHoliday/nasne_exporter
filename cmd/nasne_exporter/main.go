package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ryomaholiday/nasne_exporter/internal/exporter"
	"github.com/ryomaholiday/nasne_exporter/internal/nasne"
)

func main() {
	var (
		nasneURL      = flag.String("nasne-url", envOrDefault("NASNE_URL", ""), "nasne base URL (e.g. http://192.168.1.10:64210)")
		listenAddress = flag.String("listen-address", envOrDefault("LISTEN_ADDRESS", ":9900"), "address to listen on")
		metricsPath   = flag.String("metrics-path", envOrDefault("METRICS_PATH", "/metrics"), "metrics HTTP path")
		healthPath    = flag.String("health-path", envOrDefault("HEALTH_PATH", "/healthz"), "health check path")
		endpointCSV   = flag.String("nasne-endpoints", envOrDefault("NASNE_ENDPOINTS", "/status,/storage,/schedule"), "comma-separated nasne API paths")
		httpTimeout   = flag.Duration("http-timeout", envDuration("HTTP_TIMEOUT", 5*time.Second), "timeout per HTTP request to nasne")
		scrapeTimeout = flag.Duration("scrape-timeout", envDuration("SCRAPE_TIMEOUT", 10*time.Second), "timeout for total scrape")
	)
	flag.Parse()

	if *nasneURL == "" {
		log.Fatal("nasne-url (or NASNE_URL) is required")
	}

	client, err := nasne.NewClient(*nasneURL, splitCSV(*endpointCSV), *httpTimeout)
	if err != nil {
		log.Fatalf("create nasne client: %v", err)
	}

	collector := exporter.NewCollector(client, *scrapeTimeout)
	registry := prometheus.NewRegistry()
	registry.MustRegister(collector)

	mux := http.NewServeMux()
	mux.Handle(*metricsPath, promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	mux.HandleFunc(*healthPath, func(w http.ResponseWriter, _ *http.Request) {
		if !collector.Healthy() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("unhealthy\n"))
			return
		}
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("nasne_exporter\n"))
	})

	server := &http.Server{
		Addr:              *listenAddress,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("starting nasne_exporter on %s", *listenAddress)
	log.Printf("metrics endpoint: %s", *metricsPath)
	log.Printf("health endpoint: %s", *healthPath)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("http server failed: %v", err)
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func envOrDefault(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}

func envDuration(k string, d time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return d
	}
	parsed, err := time.ParseDuration(v)
	if err != nil {
		log.Printf("invalid duration in %s=%q: %v (using default %s)", k, v, err, d)
		return d
	}
	return parsed
}
