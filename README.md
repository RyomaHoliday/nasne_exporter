# nasne_exporter

Prometheus exporter for Sony nasne.

This project uses the official Prometheus Go client library (`prometheus/client_golang`) and is designed to run both directly and as a container image.

## Features

- `GET /metrics` endpoint (Prometheus format)
- `GET /healthz` endpoint
- Configurable nasne base URL (single / multiple)
- Multi-stage Docker build
- GitHub Actions release workflow for GHCR (`v*` tags)

## Metrics

Exporter metrics include (all gauges now include `target` label for multi-device support):

- `nasne_up`
- `nasne_collect_duration_seconds`
- `nasne_info{name,product_name,hardware_version,software_version}`
- `nasne_hdd_size_bytes`
- `nasne_hdd_usage_bytes`
- `nasne_dtcpip_clients`
- `nasne_recordings`
- `nasne_recorded_titles`
- `nasne_reserved_titles`
- `nasne_reserved_conflict_titles`
- `nasne_reserved_notfound_titles`

## Configuration

Flags (with env var fallback):

- `--nasne-url` (`NASNE_URL`) **required** (comma-separated for multiple nasne)
- `--listen-address` (`LISTEN_ADDRESS`, default `:9900`)
- `--metrics-path` (`METRICS_PATH`, default `/metrics`)
- `--health-path` (`HEALTH_PATH`, default `/healthz`)
- (エンドポイントはnasne API標準パスを使用: `status/*`, `recorded/*`, `schedule/*`)
- `--http-timeout` (`HTTP_TIMEOUT`, default `5s`)
- `--scrape-timeout` (`SCRAPE_TIMEOUT`, default `10s`)

## Run locally

```bash
go mod tidy
go run ./cmd/nasne_exporter \
  --nasne-url=http://192.168.11.1:64210,http://192.168.11.2:64210
```

## Build binary

```bash
go build ./cmd/nasne_exporter
```

## Docker

```bash
docker build -t nasne_exporter:local .
docker run --rm -p 9900:9900 \
  -e NASNE_URL=http://192.168.11.1:64210,http://192.168.11.2:64210 \
  nasne_exporter:local
```

## Prometheus scrape example

```yaml
scrape_configs:
  - job_name: nasne
    scrape_interval: 30s
    static_configs:
      - targets: ["nasne-exporter:9900"]
```

## Release (GitHub + GHCR)

1. Push this repository to GitHub.
2. Create a tag like `v0.1.0` and push it.
3. GitHub Actions workflow (`.github/workflows/release.yml`) will build and publish to GHCR:
   - `ghcr.io/<owner>/nasne_exporter:v0.1.0`
   - `ghcr.io/<owner>/nasne_exporter:latest`

## Notes / caveats

nasne firmware and API payloads can differ by model/version. This exporter uses known nasne API endpoints (`status/*`, `recorded/*`, `schedule/*`) and should work with typical nasne setups.

## Acknowledgements

Special thanks to [hatotaka/nasne_exporter](https://github.com/hatotaka/nasne_exporter) for pioneering the nasne Prometheus exporter ecosystem and for serving as a helpful reference while designing this implementation.
