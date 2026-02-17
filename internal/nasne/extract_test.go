package nasne

import "testing"

func TestExtractSnapshot(t *testing.T) {
	payload := map[string]any{
		"status": map[string]any{
			"name":                 "living-room-nasne",
			"product_name":         "nasne",
			"software_version":     "4.0",
			"hardware_version":     "1.1",
			"dtcp_ip_client_count": float64(2),
		},
		"storage": map[string]any{
			"hdd_size":       float64(1024),
			"hdd_using_size": float64(512),
		},
		"schedule": map[string]any{
			"recorded_count":           float64(42),
			"recording_count":          float64(1),
			"reserved_count":           float64(7),
			"reserved_conflict_count":  float64(3),
			"reserved_not_found_count": float64(4),
		},
	}

	s := ExtractSnapshot(payload)
	if s.Name != "living-room-nasne" {
		t.Fatalf("unexpected name: %q", s.Name)
	}
	if s.HDDSizeBytes != 1024 || s.HDDUsageBytes != 512 {
		t.Fatalf("unexpected storage: size=%v usage=%v", s.HDDSizeBytes, s.HDDUsageBytes)
	}
	if s.RecordedTitles != 42 || s.Recordings != 1 || s.ReservedTitles != 7 {
		t.Fatalf("unexpected recording counters: %+v", s)
	}
	if s.ReservedConflictTitles != 3 || s.ReservedNotFoundTitles != 4 {
		t.Fatalf("unexpected reservation counters: %+v", s)
	}
}
