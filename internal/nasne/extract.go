package nasne

import "strings"

// ExtractSnapshot best-effort maps arbitrary nasne JSON payloads to stable metrics.
func ExtractSnapshot(payload map[string]any) Snapshot {
	flat := flatten(payload, "")

	return Snapshot{
		Name:                   firstString(flat, "name", "nasne_name", "status.name"),
		ProductName:            firstString(flat, "product_name", "productname", "model_name"),
		HardwareVersion:        firstString(flat, "hardware_version", "hw_version", "version.hardware"),
		SoftwareVersion:        firstString(flat, "software_version", "sw_version", "version.software", "firmware_version"),
		HDDSizeBytes:           firstNumber(flat, "hdd_size", "hdd_total_size", "storage_total_size", "hdd_size_bytes", "storage_total_bytes"),
		HDDUsageBytes:          firstNumber(flat, "hdd_using_size", "hdd_used_size", "storage_used_size", "hdd_usage_bytes", "storage_used_bytes"),
		DTCPIPClients:          firstNumber(flat, "dtcp_ip_client_count", "dtcpip_clients", "dtcp_clients"),
		Recordings:             firstNumber(flat, "recording_count", "recordings", "recording_titles"),
		RecordedTitles:         firstNumber(flat, "recorded_count", "recorded_titles", "recorded_title_count"),
		ReservedTitles:         firstNumber(flat, "reserved_count", "reserved_titles", "reserve_count"),
		ReservedConflictTitles: firstNumber(flat, "reserved_conflict_count", "conflict_count", "reserved_conflict_titles"),
		ReservedNotFoundTitles: firstNumber(flat, "reserved_not_found_count", "notfound_count", "reserved_notfound_titles"),
	}
}

func flatten(v any, prefix string) map[string]any {
	out := map[string]any{}
	switch t := v.(type) {
	case map[string]any:
		for k, vv := range t {
			key := normalize(k)
			next := key
			if prefix != "" {
				next = prefix + "." + key
			}
			out[next] = vv
			for nk, nv := range flatten(vv, next) {
				out[nk] = nv
			}
		}
	case []any:
		for _, vv := range t {
			for nk, nv := range flatten(vv, prefix) {
				out[nk] = nv
			}
		}
	}
	return out
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	replacer := strings.NewReplacer("-", "_", " ", "_", "/", "_")
	return replacer.Replace(s)
}

func firstString(flat map[string]any, keys ...string) string {
	for _, k := range keys {
		k = normalize(k)
		if v, ok := flat[k]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
		for fk, fv := range flat {
			if strings.HasSuffix(fk, "."+k) {
				if s, ok := fv.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

func firstNumber(flat map[string]any, keys ...string) float64 {
	for _, k := range keys {
		k = normalize(k)
		if n, ok := numberFrom(flat[k]); ok {
			return n
		}
		for fk, fv := range flat {
			if strings.HasSuffix(fk, "."+k) {
				if n, ok := numberFrom(fv); ok {
					return n
				}
			}
		}
	}
	return 0
}

func numberFrom(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint64:
		return float64(n), true
	}
	return 0, false
}
