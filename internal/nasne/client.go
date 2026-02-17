package nasne

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultStatusPort   = 64210
	defaultRecordedPort = 64220
	defaultSchedulePort = 64220
)

// Client accesses nasne HTTP APIs and extracts a stable metrics snapshot.
type Client struct {
	scheme      string
	host        string
	statusPort  int
	httpClient  *http.Client
}

// Snapshot is a normalized view used by the exporter.
type Snapshot struct {
	Name                   string
	ProductName            string
	HardwareVersion        string
	SoftwareVersion        string
	HDDSizeBytes           float64
	HDDUsageBytes          float64
	DTCPIPClients          float64
	Recordings             float64
	RecordedTitles         float64
	ReservedTitles         float64
	ReservedConflictTitles float64
	ReservedNotFoundTitles float64
}

func NewClient(rawBaseURL string, timeout time.Duration) (*Client, error) {
	if rawBaseURL == "" {
		return nil, fmt.Errorf("base URL is required")
	}

	u, err := url.Parse(rawBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse base URL: %w", err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("base URL must include scheme and host")
	}

	statusPort := defaultStatusPort
	if p := u.Port(); p != "" {
		parsed, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("invalid port in base URL: %w", err)
		}
		statusPort = parsed
	}

	return &Client{
		scheme:     u.Scheme,
		host:       u.Hostname(),
		statusPort: statusPort,
		httpClient: &http.Client{Timeout: timeout},
	}, nil
}

func (c *Client) FetchSnapshot(ctx context.Context) (Snapshot, error) {
	var (
		boxName  boxNameResp
		swVer    softwareVersionResp
		hwVer    hardwareVersionResp
		hddList  hddListResp
		dtcpList dtcpipClientListResp
		boxStat  boxStatusListResp
	)

	if err := c.getJSON(ctx, "status/boxNameGet", c.statusPort, nil, &boxName); err != nil {
		return Snapshot{}, err
	}
	if err := c.getJSON(ctx, "status/softwareVersionGet", c.statusPort, nil, &swVer); err != nil {
		return Snapshot{}, err
	}
	if err := c.getJSON(ctx, "status/hardwareVersionGet", c.statusPort, nil, &hwVer); err != nil {
		return Snapshot{}, err
	}
	if err := c.getJSON(ctx, "status/HDDListGet", c.statusPort, nil, &hddList); err != nil {
		return Snapshot{}, err
	}
	if err := c.getJSON(ctx, "status/dtcpipClientListGet", c.statusPort, nil, &dtcpList); err != nil {
		return Snapshot{}, err
	}
	if err := c.getJSON(ctx, "status/boxStatusListGet", c.statusPort, nil, &boxStat); err != nil {
		return Snapshot{}, err
	}

	recordedTitles, err := c.getRecordedTitles(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	reservedTitles, reservedConflict, reservedNotFound, err := c.getReservedStats(ctx)
	if err != nil {
		return Snapshot{}, err
	}

	hddTotal, hddUsed, err := c.getHDDUsage(ctx, hddList)
	if err != nil {
		return Snapshot{}, err
	}

	recordings := 0.0
	if boxStat.TuningStatus.Status == 1 {
		recordings = 1
	}

	return Snapshot{
		Name:                   boxName.Name,
		ProductName:            hwVer.ProductName,
		HardwareVersion:        strconv.Itoa(hwVer.HardwareVersion),
		SoftwareVersion:        swVer.SoftwareVersion,
		HDDSizeBytes:           hddTotal,
		HDDUsageBytes:          hddUsed,
		DTCPIPClients:          float64(dtcpList.Number),
		Recordings:             recordings,
		RecordedTitles:         recordedTitles,
		ReservedTitles:         reservedTitles,
		ReservedConflictTitles: reservedConflict,
		ReservedNotFoundTitles: reservedNotFound,
	}, nil
}

func (c *Client) getHDDUsage(ctx context.Context, list hddListResp) (total float64, used float64, err error) {
	for _, hdd := range list.HDD {
		q := url.Values{}
		q.Set("id", strconv.Itoa(hdd.ID))
		var info hddInfoResp
		if e := c.getJSON(ctx, "status/HDDInfoGet", c.statusPort, q, &info); e != nil {
			return 0, 0, e
		}
		total += info.HDD.TotalVolumeSize
		used += info.HDD.UsedVolumeSize
	}
	return total, used, nil
}

func (c *Client) getRecordedTitles(ctx context.Context) (float64, error) {
	var resp titleListResp
	q := commonListQuery()
	if err := c.getJSONWithFallback(ctx, "recorded/titleListGet", []int{defaultRecordedPort, c.statusPort}, q, &resp); err != nil {
		return 0, err
	}
	return float64(resp.TotalMatches), nil
}

func (c *Client) getReservedStats(ctx context.Context) (total float64, conflict float64, notFound float64, err error) {
	var resp reservedListResp
	q := commonListQuery()
	q.Set("withDescriptionLong", "0")
	q.Set("withUserData", "1")
	if err = c.getJSONWithFallback(ctx, "schedule/reservedListGet", []int{defaultSchedulePort, c.statusPort}, q, &resp); err != nil {
		return 0, 0, 0, err
	}

	total = float64(resp.TotalMatches)
	for _, item := range resp.Item {
		if item.ConflictID >= 1 {
			conflict++
		}
		if item.EventID == 65536 {
			notFound++
		}
	}
	return total, conflict, notFound, nil
}

func commonListQuery() url.Values {
	q := url.Values{}
	q.Set("searchCriteria", "0")
	q.Set("filter", "0")
	q.Set("startingIndex", "0")
	q.Set("requestedCount", "0")
	q.Set("sortCriteria", "0")
	return q
}

func (c *Client) getJSONWithFallback(ctx context.Context, endpoint string, ports []int, query url.Values, out any) error {
	var lastErr error
	seen := map[int]struct{}{}
	for _, p := range ports {
		if p <= 0 {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		if err := c.getJSON(ctx, endpoint, p, query, out); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no valid ports")
	}
	return lastErr
}

func (c *Client) getJSON(ctx context.Context, endpoint string, port int, query url.Values, out any) error {
	u := url.URL{Scheme: c.scheme, Host: fmt.Sprintf("%s:%d", c.host, port), Path: "/" + strings.TrimLeft(endpoint, "/")}
	if query != nil {
		u.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create request %q: %w", endpoint, err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request %q: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("request %q: status=%d body=%q", endpoint, resp.StatusCode, strings.TrimSpace(string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %q: %w", endpoint, err)
	}
	return nil
}

type boxNameResp struct {
	Name string `json:"name"`
}

type softwareVersionResp struct {
	SoftwareVersion string `json:"softwareVersion"`
}

type hardwareVersionResp struct {
	ProductName     string `json:"productName"`
	HardwareVersion int    `json:"hardwareVersion"`
}

type hddListResp struct {
	HDD []struct {
		ID int `json:"id"`
	} `json:"HDD"`
}

type hddInfoResp struct {
	HDD struct {
		TotalVolumeSize float64 `json:"totalVolumeSize"`
		UsedVolumeSize  float64 `json:"usedVolumeSize"`
	} `json:"HDD"`
}

type dtcpipClientListResp struct {
	Number int `json:"number"`
}

type boxStatusListResp struct {
	TuningStatus struct {
		Status int `json:"status"`
	} `json:"tuningStatus"`
}

type titleListResp struct {
	TotalMatches int `json:"totalMatches"`
}

type reservedListResp struct {
	TotalMatches int `json:"totalMatches"`
	Item         []struct {
		ConflictID int `json:"conflictId"`
		EventID    int `json:"eventId"`
	} `json:"item"`
}
