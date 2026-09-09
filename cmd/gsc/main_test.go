package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRunCloseGeometries(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{testdata(t, "source.geojson"), testdata(t, "target-close.geojson")}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "-> used") {
		t.Fatalf("output:\n%s", out)
	}
	if !strings.Contains(out, "source") || !strings.Contains(out, "target") {
		t.Fatalf("missing roles:\n%s", out)
	}
}

func TestRunFarGeometries(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{testdata(t, "source.geojson"), testdata(t, "target-far.geojson")}, stdout, stderr)
	if code != 1 {
		t.Fatalf("exit %d, want 1 (not similar), stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "-> not_used") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunJSON(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{"-json", testdata(t, "source.geojson"), testdata(t, "target-close.geojson")}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"verdict": "used"`) {
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

func TestRunFlagsAfterOperands(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	code := run([]string{
		testdata(t, "source.polyline5"),
		testdata(t, "target-close.polyline6"),
		"-target-format", "polyline6",
	}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "polyline6") {
		t.Fatalf("output:\n%s", stdout.String())
	}
}

func TestRunLiteralPointArray(t *testing.T) {
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	source := "{37.0,-122.0;37.009043,-122.0}"
	target := "{37.0,-121.999887;37.009043,-121.999887}"
	code := run([]string{source, target}, stdout, stderr)
	if code != 0 {
		t.Fatalf("exit %d, stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
}
