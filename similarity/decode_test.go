package similarity

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func testdataFile(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "testdata", name)
}

func writePairJSON(t *testing.T, source, target string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "pair.json")
	buf, err := json.Marshal([]pairFile{{Source: source, Target: target}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDecodePolylineGoogleSample(t *testing.T) {
	// https://developers.google.com/maps/documentation/utilities/polylinealgorithm
	points, err := DecodePolyline("_p~iF~ps|U_ulLnnqC_mqNvxq`@", 5)
	if err != nil {
		t.Fatal(err)
	}
	want := Points{
		{Lat: 38.5, Lon: -120.2},
		{Lat: 40.7, Lon: -120.95},
		{Lat: 43.252, Lon: -126.453},
	}
	if len(points) != len(want) {
		t.Fatalf("got %d points, want %d", len(points), len(want))
	}
	for i := range want {
		if math.Abs(points[i].Lat-want[i].Lat) > 1e-5 || math.Abs(points[i].Lon-want[i].Lon) > 1e-5 {
			t.Fatalf("point %d = %v, want %v", i, points[i], want[i])
		}
	}
}

func TestPolylineRoundTrip(t *testing.T) {
	original := Points{
		{Lat: 37.7749, Lon: -122.4194},
		{Lat: 34.0522, Lon: -118.2437},
		{Lat: 40.7128, Lon: -74.0060},
	}
	for _, precision := range []int{5, 6} {
		encoded, err := EncodePolyline(original, precision)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := DecodePolyline(encoded, precision)
		if err != nil {
			t.Fatal(err)
		}
		if len(decoded) != len(original) {
			t.Fatalf("precision %d: got %d points", precision, len(decoded))
		}
		tol := math.Pow10(-precision)
		for i := range original {
			if math.Abs(decoded[i].Lat-original[i].Lat) > tol || math.Abs(decoded[i].Lon-original[i].Lon) > tol {
				t.Fatalf("precision %d point %d = %v, want %v", precision, i, decoded[i], original[i])
			}
		}
	}
}

func TestDecodeDetectsPolyline5And6(t *testing.T) {
	original := Points{{Lat: 37, Lon: -122}, {Lat: 38, Lon: -121}}
	for _, want := range []Format{FormatPolyline5, FormatPolyline6} {
		precision := 5
		if want == FormatPolyline6 {
			precision = 6
		}
		encoded, err := EncodePolyline(original, precision)
		if err != nil {
			t.Fatal(err)
		}
		geom, err := Decode(encoded)
		if err != nil {
			t.Fatal(err)
		}
		if geom.Format != want {
			t.Fatalf("detected %s, want %s", geom.Format, want)
		}
		if len(geom.Points) != 2 {
			t.Fatalf("%s: got %d points", want, len(geom.Points))
		}
		tol := math.Pow10(-precision)
		for i := range original {
			if math.Abs(geom.Points[i].Lat-original[i].Lat) > tol || math.Abs(geom.Points[i].Lon-original[i].Lon) > tol {
				t.Fatalf("%s point %d = %v, want %v", want, i, geom.Points[i], original[i])
			}
		}
	}
}

func TestDecodeRejectsSinglePoint(t *testing.T) {
	encoded, err := EncodePolyline(Points{{Lat: 37, Lon: -122}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	_, err = Decode(encoded)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestLoadFile(t *testing.T) {
	encoded, err := EncodePolyline(Points{{Lat: 37, Lon: -122}, {Lat: 38, Lon: -121}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	pairs, err := LoadFile(writePairJSON(t, encoded, encoded))
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 {
		t.Fatalf("got %d pairs", len(pairs))
	}
	pair := pairs[0]
	if pair.Source.Format != FormatPolyline5 || pair.Target.Format != FormatPolyline5 {
		t.Fatalf("formats = %s / %s", pair.Source.Format, pair.Target.Format)
	}
	if len(pair.Source.Points) < 2 || len(pair.Target.Points) < 2 {
		t.Fatalf("got %d / %d points", len(pair.Source.Points), len(pair.Target.Points))
	}
}

func TestLoadFileTestdata(t *testing.T) {
	pairs, err := LoadFile(testdataFile(t, "example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 {
		t.Fatalf("got %d pairs", len(pairs))
	}
	pair := pairs[0]
	if pair.Source.Format != FormatPolyline5 {
		t.Fatalf("source format = %s, want polyline5", pair.Source.Format)
	}
	if pair.Target.Format != FormatPolyline6 {
		t.Fatalf("target format = %s, want polyline6", pair.Target.Format)
	}
	if len(pair.Source.Points) < 2 || len(pair.Target.Points) < 2 {
		t.Fatalf("got %d / %d points", len(pair.Source.Points), len(pair.Target.Points))
	}
}

func TestLoadFileRejectsRawPolyline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "raw.txt")
	if err := os.WriteFile(path, []byte("_p~iF~ps|U_ulLnnqC_mqNvxq`@"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "json") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileArray(t *testing.T) {
	encoded, err := EncodePolyline(Points{{Lat: 37, Lon: -122}, {Lat: 38, Lon: -121}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "pairs.json")
	buf, err := json.Marshal([]pairFile{
		{Source: encoded, Target: encoded},
		{Source: encoded, Target: encoded},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf, 0o644); err != nil {
		t.Fatal(err)
	}
	pairs, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 2 {
		t.Fatalf("got %d pairs", len(pairs))
	}
}

func TestLoadFileEmptyArray(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte(`[]`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "no pairs") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadFileMissingFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(path, []byte(`[{}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadFile(path)
	if err == nil || !strings.Contains(err.Error(), "missing source") {
		t.Fatalf("err = %v", err)
	}
}
