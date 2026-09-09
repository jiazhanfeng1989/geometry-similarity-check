package similarity

import (
	"math"
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

func TestDecodePointArray(t *testing.T) {
	geom, err := Decode("{37.0,-122.0;37.1,-122.1}", FormatPointArray)
	if err != nil {
		t.Fatal(err)
	}
	if len(geom.Points) != 2 {
		t.Fatalf("got %d points", len(geom.Points))
	}
	if geom.Points[0].Lat != 37.0 || geom.Points[0].Lon != -122.0 {
		t.Fatalf("first = %v", geom.Points[0])
	}
}

func TestDecodeWKT(t *testing.T) {
	geom, err := Decode("LINESTRING (-122.0 37.0, -122.1 37.1)", FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if geom.Format != FormatWKT {
		t.Fatalf("format = %s, want wkt", geom.Format)
	}
	if len(geom.Points) != 2 || geom.Points[0].Lat != 37.0 || geom.Points[0].Lon != -122.0 {
		t.Fatalf("points = %v", geom.Points)
	}
}

func TestDecodeGeoJSONLineString(t *testing.T) {
	text := `{"type":"LineString","coordinates":[[-122.0,37.0],[-122.1,37.1]]}`
	geom, err := Decode(text, FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if geom.Format != FormatGeoJSON {
		t.Fatalf("format = %s", geom.Format)
	}
	if len(geom.Points) != 2 || geom.Points[0].Lat != 37.0 || geom.Points[0].Lon != -122.0 {
		t.Fatalf("points = %v", geom.Points)
	}
}

func TestDecodeGeoJSONFeature(t *testing.T) {
	text := `{
		"type":"Feature",
		"geometry":{"type":"LineString","coordinates":[[-122.0,37.0],[-118.0,34.0]]},
		"properties":{}
	}`
	geom, err := Decode(text, FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if len(geom.Points) != 2 || geom.Points[1].Lat != 34.0 {
		t.Fatalf("points = %v", geom.Points)
	}
}

func TestDecodeLatLonCSV(t *testing.T) {
	text := "lat,lon\n37.0,-122.0\n37.1,-122.1\n"
	geom, err := Decode(text, FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if geom.Format != FormatLatLon {
		t.Fatalf("format = %s", geom.Format)
	}
	if len(geom.Points) != 2 {
		t.Fatalf("got %d points", len(geom.Points))
	}
}

func TestDecodeRejectsSinglePoint(t *testing.T) {
	_, err := Decode("37.0,-122.0\n", FormatLatLon)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDecodeQuotedPolyline(t *testing.T) {
	encoded, err := EncodePolyline(Points{{Lat: 37, Lon: -122}, {Lat: 38, Lon: -121}}, 5)
	if err != nil {
		t.Fatal(err)
	}
	geom, err := Decode(`"`+encoded+`"`, FormatPolyline5)
	if err != nil {
		t.Fatal(err)
	}
	if len(geom.Points) != 2 {
		t.Fatalf("got %d points", len(geom.Points))
	}
}

func TestDetectPolylineDoesNotGuessPrecision6(t *testing.T) {
	encoded, err := EncodePolyline(Points{{Lat: 37, Lon: -122}, {Lat: 38, Lon: -121}}, 6)
	if err != nil {
		t.Fatal(err)
	}
	detected := detectFormat(encoded)
	if detected != FormatPolyline5 {
		t.Fatalf("auto format = %s, want polyline5 (must not guess 6)", detected)
	}
}

func TestLoadFile(t *testing.T) {
	geom, err := LoadFile(testdataFile(t, "source.geojson"), FormatAuto)
	if err != nil {
		t.Fatal(err)
	}
	if geom.Format != FormatGeoJSON {
		t.Fatalf("format = %s", geom.Format)
	}
	if len(geom.Points) < 2 {
		t.Fatalf("got %d points", len(geom.Points))
	}
}

func TestUnknownFormat(t *testing.T) {
	_, err := Decode("LINESTRING (0 0, 1 1)", Format("nope"))
	if err == nil || !strings.Contains(err.Error(), "unknown format") {
		t.Fatalf("err = %v", err)
	}
}
