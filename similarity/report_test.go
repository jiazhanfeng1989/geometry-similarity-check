package similarity

import (
	"strings"
	"testing"
)

func TestReportWriteTSV(t *testing.T) {
	source := Geometry{Points: straightLine(37.0, -122.0, 100, 20), Format: FormatPolyline5}
	target := Geometry{Points: offsetEast(source.Points, 2000), Format: FormatPolyline5}

	report, err := Analyze(source, target)
	if err != nil {
		t.Fatal(err)
	}

	var buf strings.Builder
	if err := report.Write(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	if len(lines) == 0 {
		t.Fatal("empty report")
	}

	got := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.Contains(line, " ") && !strings.Contains(line, "\t") {
			t.Fatalf("line is not tab-separated: %q", line)
		}
		key, _, ok := strings.Cut(line, "\t")
		if !ok {
			t.Fatalf("missing tab: %q", line)
		}
		got = append(got, key)
	}

	wantPrefix := []string{
		"verdict",
		"coverage",
		"coverage_threshold",
		"max_deviation_m",
		"max_deviation_veto_m",
		"corridor_m",
		"source",
		"target",
		"worst_target_from_source",
		"worst_source_from_target",
		"outside_stretch_count",
	}
	if len(got) < len(wantPrefix) {
		t.Fatalf("keys = %v", got)
	}
	for i, key := range wantPrefix {
		if got[i] != key {
			t.Fatalf("key %d = %q, want %q\n%s", i, got[i], key, out)
		}
	}
	if !strings.Contains(out, "outside_stretch\t") {
		t.Fatalf("missing outside_stretch:\n%s", out)
	}
}
