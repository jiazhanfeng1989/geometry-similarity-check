package similarity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"
)

// Format names a geometry encoding. Auto tries the structured formats first and falls back to
// polyline precision 5. Polyline5 and polyline6 cannot be told apart from the bytes alone — a
// polyline6 decoded as precision 5 is scaled by ten and makes the same roads look unrelated — so
// Auto never guesses polyline6. Pass FormatPolyline6 (or --target-format polyline6) when that is
// what you have.
type Format string

const (
	FormatAuto       Format = "auto"
	FormatPolyline5  Format = "polyline5"
	FormatPolyline   Format = "polyline" // alias of polyline5
	FormatPolyline6  Format = "polyline6"
	FormatPointArray Format = "pointarray"
	FormatGeoJSON    Format = "geojson"
	FormatWKT        Format = "wkt"
	FormatLatLon     Format = "latlon"
)

// Geometry is a decoded polyline plus the format that produced it, for the report header.
type Geometry struct {
	Points Points
	Format Format
}

// FileExists reports whether path is a readable regular file. The CLI uses this to decide
// whether an argument is a path or a literal geometry.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// LoadFile reads a geometry from disk.
func LoadFile(path string, format Format) (Geometry, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return Geometry{}, fmt.Errorf("reading %s: %w", path, err)
	}
	return Decode(string(buf), format)
}

// Load treats input as a file path when that file exists, otherwise as a geometry string.
func Load(input string, format Format) (Geometry, error) {
	if FileExists(input) {
		return LoadFile(input, format)
	}
	return Decode(input, format)
}

// Decode parses a geometry string in the given format. FormatAuto inspects the text.
func Decode(text string, format Format) (Geometry, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "\ufeff")
	if len(text) >= 2 && text[0] == '"' && text[len(text)-1] == '"' {
		text = text[1 : len(text)-1]
	}
	if text == "" {
		return Geometry{}, fmt.Errorf("empty geometry")
	}

	if format == "" || format == FormatAuto {
		format = detectFormat(text)
	}
	if format == FormatPolyline {
		format = FormatPolyline5
	}

	points, err := decodeAs(text, format)
	if err != nil {
		return Geometry{}, err
	}
	if len(points) < 2 {
		return Geometry{}, fmt.Errorf("%s decoded to %d points, need at least 2", format, len(points))
	}
	return Geometry{Points: points, Format: format}, nil
}

func detectFormat(text string) Format {
	trimmed := strings.TrimSpace(text)
	upper := strings.ToUpper(trimmed)

	switch {
	case strings.HasPrefix(upper, "LINESTRING"):
		return FormatWKT
	case looksLikeGeoJSON(trimmed):
		return FormatGeoJSON
	case looksLikePointArray(trimmed):
		return FormatPointArray
	case looksLikeLatLon(trimmed):
		return FormatLatLon
	default:
		return FormatPolyline5
	}
}

func looksLikeGeoJSON(text string) bool {
	if len(text) == 0 {
		return false
	}
	if text[0] != '{' && text[0] != '[' {
		return false
	}
	lower := strings.ToLower(text)
	return strings.Contains(lower, `"type"`) ||
		strings.Contains(lower, `"coordinates"`) ||
		strings.Contains(lower, `"lat"`) ||
		(text[0] == '[' && looksLikeCoordinateArray(text))
}

func looksLikeCoordinateArray(text string) bool {
	var raw any
	if json.Unmarshal([]byte(text), &raw) != nil {
		return false
	}
	return firstPair(raw) != nil
}

func looksLikePointArray(text string) bool {
	trimmed := strings.TrimSpace(text)
	trimmed = strings.TrimPrefix(trimmed, "{")
	trimmed = strings.TrimSuffix(trimmed, "}")
	if !strings.Contains(trimmed, ";") || !strings.Contains(trimmed, ",") {
		return false
	}
	first, _, _ := strings.Cut(trimmed, ";")
	lat, lon, found := strings.Cut(pairWithoutSpaces(first), ",")
	if !found {
		return false
	}
	_, latErr := strconv.ParseFloat(lat, 64)
	_, lonErr := strconv.ParseFloat(lon, 64)
	return latErr == nil && lonErr == nil
}

func pairWithoutSpaces(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, s)
}

func looksLikeLatLon(text string) bool {
	lines := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "lat,lon" || lower == "lat,lng" || lower == "latitude,longitude" {
			continue
		}
		if _, _, err := parseLatLonPair(line); err != nil {
			return false
		}
		lines++
	}
	return lines >= 2
}

func decodeAs(text string, format Format) (Points, error) {
	switch format {
	case FormatPolyline5:
		return DecodePolyline(text, 5)
	case FormatPolyline6:
		return DecodePolyline(text, 6)
	case FormatPointArray:
		return decodePointArray(text)
	case FormatGeoJSON:
		return decodeGeoJSON(text)
	case FormatWKT:
		return decodeWKT(text)
	case FormatLatLon:
		return decodeLatLon(text)
	default:
		return nil, fmt.Errorf("unknown format %q (want auto, polyline5, polyline6, pointarray, geojson, wkt, latlon)", format)
	}
}

// decodePointArray parses the `{lat1,lon1;lat2,lon2}` geometry format used by some routing APIs.
func decodePointArray(geometry string) (Points, error) {
	trimmed := strings.TrimSpace(geometry)
	trimmed = strings.TrimPrefix(trimmed, "{")
	trimmed = strings.TrimSuffix(trimmed, "}")
	if trimmed == "" {
		return nil, nil
	}

	pairs := strings.Split(trimmed, ";")
	points := make(Points, 0, len(pairs))
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}

		lat, lon, found := strings.Cut(pair, ",")
		if !found {
			return nil, fmt.Errorf("invalid point array entry %q", pair)
		}

		point, err := parsePoint(strings.TrimSpace(lat), strings.TrimSpace(lon))
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}

func parsePoint(lat, lon string) (Point, error) {
	parsedLat, err := strconv.ParseFloat(lat, 64)
	if err != nil {
		return Point{}, fmt.Errorf("invalid latitude %q: %w", lat, err)
	}
	parsedLon, err := strconv.ParseFloat(lon, 64)
	if err != nil {
		return Point{}, fmt.Errorf("invalid longitude %q: %w", lon, err)
	}
	return Point{Lat: parsedLat, Lon: parsedLon}, nil
}

func decodeLatLon(text string) (Points, error) {
	var points Points
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if lower == "lat,lon" || lower == "lat,lng" || lower == "latitude,longitude" {
			continue
		}
		point, _, err := parseLatLonPair(line)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}

func parseLatLonPair(line string) (Point, string, error) {
	line = strings.ReplaceAll(line, "\t", ",")
	fields := strings.FieldsFunc(line, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	if len(fields) < 2 {
		return Point{}, "", fmt.Errorf("need lat,lon on line %q", line)
	}
	point, err := parsePoint(fields[0], fields[1])
	return point, "", err
}

func decodeWKT(text string) (Points, error) {
	trimmed := strings.TrimSpace(text)
	upper := strings.ToUpper(trimmed)
	if !strings.HasPrefix(upper, "LINESTRING") {
		return nil, fmt.Errorf("WKT must start with LINESTRING, got %q", firstToken(trimmed))
	}

	start := strings.Index(trimmed, "(")
	end := strings.LastIndex(trimmed, ")")
	if start < 0 || end <= start {
		return nil, fmt.Errorf("invalid LINESTRING: missing parentheses")
	}

	body := trimmed[start+1 : end]
	body = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(body), "("))
	body = strings.TrimSuffix(strings.TrimSpace(body), ")")

	var points Points
	for _, part := range strings.Split(body, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		fields := strings.Fields(part)
		if len(fields) < 2 {
			return nil, fmt.Errorf("invalid WKT vertex %q", part)
		}
		lon, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid WKT longitude %q: %w", fields[0], err)
		}
		lat, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			return nil, fmt.Errorf("invalid WKT latitude %q: %w", fields[1], err)
		}
		points = append(points, Point{Lat: lat, Lon: lon})
	}
	return points, nil
}

func firstToken(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexFunc(s, unicode.IsSpace); i >= 0 {
		return s[:i]
	}
	if len(s) > 32 {
		return s[:32]
	}
	return s
}

func decodeGeoJSON(text string) (Points, error) {
	var raw any
	dec := json.NewDecoder(bytes.NewReader([]byte(text)))
	dec.UseNumber()
	if err := dec.Decode(&raw); err != nil {
		return nil, fmt.Errorf("geojson: %w", err)
	}

	if points := pointsFromObject(raw); len(points) >= 2 {
		return points, nil
	}

	if pair := firstPair(raw); pair != nil {
		return collectPairs(raw)
	}

	return nil, fmt.Errorf("geojson: no LineString coordinates found")
}

func pointsFromObject(raw any) Points {
	obj, ok := raw.(map[string]any)
	if !ok {
		return nil
	}

	typ, _ := obj["type"].(string)
	switch strings.ToLower(typ) {
	case "linestring":
		return pairsToPoints(obj["coordinates"])
	case "multilinestring":
		return flattenLineStrings(obj["coordinates"])
	case "feature":
		return pointsFromObject(obj["geometry"])
	case "featurecollection":
		features, _ := obj["features"].([]any)
		for _, feature := range features {
			if points := pointsFromObject(feature); len(points) >= 2 {
				return points
			}
		}
	case "geometrycollection":
		geoms, _ := obj["geometries"].([]any)
		for _, geom := range geoms {
			if points := pointsFromObject(geom); len(points) >= 2 {
				return points
			}
		}
	}

	if coords, ok := obj["coordinates"]; ok {
		if points := pairsToPoints(coords); len(points) >= 2 {
			return points
		}
		if points := flattenLineStrings(coords); len(points) >= 2 {
			return points
		}
	}

	if geom, ok := obj["geometry"]; ok {
		return pointsFromObject(geom)
	}

	if lat, lon, ok := latLonFromMap(obj); ok {
		return Points{{Lat: lat, Lon: lon}}
	}
	return nil
}

func flattenLineStrings(raw any) Points {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	var points Points
	for _, line := range arr {
		points = append(points, pairsToPoints(line)...)
	}
	return points
}

func collectPairs(raw any) (Points, error) {
	if pair := asLonLat(raw); pair != nil {
		return Points{*pair}, nil
	}

	arr, ok := raw.([]any)
	if !ok {
		if obj, ok := raw.(map[string]any); ok {
			if lat, lon, ok := latLonFromMap(obj); ok {
				return Points{{Lat: lat, Lon: lon}}, nil
			}
			if nested, ok := obj["coordinates"]; ok {
				return collectPairs(nested)
			}
		}
		return nil, fmt.Errorf("geojson: unsupported coordinate value")
	}

	if len(arr) >= 2 {
		if _, isNum := arr[0].(json.Number); isNum {
			return pairsToPoints(arr), nil
		}
		if _, isFloat := arr[0].(float64); isFloat {
			return pairsToPoints(arr), nil
		}
	}

	var points Points
	for _, item := range arr {
		part, err := collectPairs(item)
		if err != nil {
			return nil, err
		}
		points = append(points, part...)
	}
	return points, nil
}

func pairsToPoints(raw any) Points {
	arr, ok := raw.([]any)
	if !ok {
		return nil
	}
	if len(arr) >= 2 {
		if p := asLonLat(arr); p != nil {
			return nil
		}
	}
	var points Points
	for _, item := range arr {
		if p := asLonLat(item); p != nil {
			points = append(points, *p)
			continue
		}
		if obj, ok := item.(map[string]any); ok {
			if lat, lon, ok := latLonFromMap(obj); ok {
				points = append(points, Point{Lat: lat, Lon: lon})
			}
		}
	}
	return points
}

func asLonLat(raw any) *Point {
	arr, ok := raw.([]any)
	if !ok || len(arr) < 2 {
		return nil
	}
	lon, okLon := jsonFloat(arr[0])
	lat, okLat := jsonFloat(arr[1])
	if !okLon || !okLat {
		return nil
	}
	return &Point{Lat: lat, Lon: lon}
}

func firstPair(raw any) *Point {
	if p := asLonLat(raw); p != nil {
		return p
	}
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if p := firstPair(item); p != nil {
				return p
			}
		}
	case map[string]any:
		if coords, ok := v["coordinates"]; ok {
			return firstPair(coords)
		}
		if lat, lon, ok := latLonFromMap(v); ok {
			return &Point{Lat: lat, Lon: lon}
		}
	}
	return nil
}

func latLonFromMap(obj map[string]any) (float64, float64, bool) {
	lat, okLat := jsonFloat(firstKey(obj, "lat", "latitude"))
	lon, okLon := jsonFloat(firstKey(obj, "lon", "lng", "longitude"))
	return lat, lon, okLat && okLon
}

func firstKey(obj map[string]any, keys ...string) any {
	for _, key := range keys {
		if v, ok := obj[key]; ok {
			return v
		}
	}
	return nil
}

func jsonFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	default:
		return 0, false
	}
}
