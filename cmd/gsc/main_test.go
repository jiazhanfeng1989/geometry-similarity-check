package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zhfjia/geometry-similarity-check/similarity"
)

func poly5(t *testing.T, pts similarity.Points) string {
	t.Helper()
	encoded, err := similarity.EncodePolyline(pts, 5)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func writePairJSON(t *testing.T, source, target string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pair.json")
	buf, err := json.Marshal([]struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}{{source, target}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func closePair(t *testing.T) string {
	t.Helper()
	source := similarity.Points{
		{Lat: 37.0, Lon: -122.0},
		{Lat: 37.009043, Lon: -122.0},
	}
	target := similarity.Points{
		{Lat: 37.0, Lon: -121.999887},
		{Lat: 37.009043, Lon: -121.999887},
	}
	return writePairJSON(t, poly5(t, source), poly5(t, target))
}

func farPair(t *testing.T) string {
	t.Helper()
	source := similarity.Points{
		{Lat: 37.0, Lon: -122.0},
		{Lat: 37.009043, Lon: -122.0},
	}
	target := similarity.Points{
		{Lat: 37.0, Lon: -121.98},
		{Lat: 37.009043, Lon: -121.98},
	}
	return writePairJSON(t, poly5(t, source), poly5(t, target))
}

func TestRunCloseGeometries(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{"-data", closePair(t)}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "verdict\tsimilar") {
		t.Fatalf("output:\n%s", out)
	}
	if !strings.Contains(out, "source") || !strings.Contains(out, "target") {
		t.Fatalf("missing roles:\n%s", out)
	}
}

func TestRunFarGeometries(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{"-data", farPair(t)}, stdout, stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (not similar), stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "verdict\tnot_similar") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunJSON(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{"-json", "-data", closePair(t)}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"verdict": "similar"`) {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunVersion(t *testing.T) {
	stdout := new(bytes.Buffer)
	code := run([]string{"-version"}, stdout, new(bytes.Buffer))
	if code != 0 || !strings.Contains(stdout.String(), version) {
		t.Fatalf("exit %d output %q", code, stdout.String())
	}
}

func TestRunMissingArgs(t *testing.T) {
	code := run(nil, new(bytes.Buffer), new(bytes.Buffer))
	if code != 2 {
		t.Fatalf("exit %d, want 2", code)
	}
}

func TestRunFlagOrderDoesNotMatter(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{"-data", closePair(t), "-json"}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), `"verdict": "similar"`) {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunPolyline6(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	pts := similarity.Points{
		{Lat: 37.0, Lon: -122.0},
		{Lat: 37.009043, Lon: -122.0},
	}
	encoded, err := similarity.EncodePolyline(pts, 6)
	if err != nil {
		t.Fatal(err)
	}
	code := run([]string{"-data", writePairJSON(t, encoded, encoded)}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "polyline6") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunMultiplePairs(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	closeSrc := similarity.Points{
		{Lat: 37.0, Lon: -122.0},
		{Lat: 37.009043, Lon: -122.0},
	}
	closeTgt := similarity.Points{
		{Lat: 37.0, Lon: -121.999887},
		{Lat: 37.009043, Lon: -121.999887},
	}
	farTgt := similarity.Points{
		{Lat: 37.0, Lon: -121.98},
		{Lat: 37.009043, Lon: -121.98},
	}
	path := filepath.Join(t.TempDir(), "pairs.json")
	buf, err := json.Marshal([]struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}{
		{poly5(t, closeSrc), poly5(t, closeTgt)},
		{poly5(t, closeSrc), poly5(t, farTgt)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	code := run([]string{"-data", path}, stdout, stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "pair\t1/2") || !strings.Contains(out, "pair\t2/2") {
		t.Fatalf("output:\n%s", out)
	}
	if !strings.Contains(out, "verdict\tsimilar") || !strings.Contains(out, "verdict\tnot_similar") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestRunMultiplePairsJSON(t *testing.T) {
	stdout := new(bytes.Buffer)
	closeSrc := similarity.Points{
		{Lat: 37.0, Lon: -122.0},
		{Lat: 37.009043, Lon: -122.0},
	}
	closeTgt := similarity.Points{
		{Lat: 37.0, Lon: -121.999887},
		{Lat: 37.009043, Lon: -121.999887},
	}
	path := filepath.Join(t.TempDir(), "pairs.json")
	buf, err := json.Marshal([]struct {
		Source string `json:"source"`
		Target string `json:"target"`
	}{
		{poly5(t, closeSrc), poly5(t, closeTgt)},
		{poly5(t, closeSrc), poly5(t, closeTgt)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	code := run([]string{"-json", "-data", path}, stdout, new(bytes.Buffer))
	if code != 0 {
		t.Fatalf("exit %d, stdout=%s", code, stdout.String())
	}
	var reports []map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &reports); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout.String())
	}
	if len(reports) != 2 {
		t.Fatalf("got %d reports", len(reports))
	}
}

func TestRunRejectsLiteralString(t *testing.T) {
	stderr := new(bytes.Buffer)
	code := run([]string{"-data", "_p~iF~ps|U_ulLnnqC_mqNvxq`@"}, new(bytes.Buffer), stderr)
	if code != 2 {
		t.Fatalf("exit %d, want 2, stderr=%s", code, stderr.String())
	}
}
