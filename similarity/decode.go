package similarity

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Format names a Google polyline encoding precision. Only polyline5 and polyline6 are supported.
type Format string

const (
	FormatPolyline5 Format = "polyline5"
	FormatPolyline6 Format = "polyline6"
)

// Geometry is a decoded polyline plus the format that produced it, for the report header.
type Geometry struct {
	Points Points
	Format Format
}

// Pair is a source/target comparison loaded from one JSON file.
type Pair struct {
	Source Geometry
	Target Geometry
}

type pairFile struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// LoadFile reads a JSON array of {"source": "...", "target": "..."} objects.
// JSON unescaping is what turns a copied \\ into the backslash the polyline encoding uses.
func LoadFile(path string) ([]Pair, error) {
	buf, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	raw := bytes.TrimSpace(buf)
	if len(raw) == 0 {
		return nil, fmt.Errorf("empty json")
	}

	var files []pairFile
	if err := json.Unmarshal(raw, &files); err != nil {
		return nil, fmt.Errorf("json: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no pairs")
	}

	pairs := make([]Pair, 0, len(files))
	for i, file := range files {
		pair, err := decodePair(file)
		if err != nil {
			return nil, fmt.Errorf("pair %d: %w", i+1, err)
		}
		pairs = append(pairs, pair)
	}
	return pairs, nil
}

func decodePair(file pairFile) (Pair, error) {
	if strings.TrimSpace(file.Source) == "" {
		return Pair{}, fmt.Errorf("missing source")
	}
	if strings.TrimSpace(file.Target) == "" {
		return Pair{}, fmt.Errorf("missing target")
	}
	source, err := Decode(file.Source)
	if err != nil {
		return Pair{}, fmt.Errorf("source: %w", err)
	}
	target, err := Decode(file.Target)
	if err != nil {
		return Pair{}, fmt.Errorf("target: %w", err)
	}
	return Pair{Source: source, Target: target}, nil
}

// Decode parses a Google encoded polyline. It supports polyline5 and polyline6 only, and
// picks precision by whether the decoded coordinates sit on the globe. A polyline6 decoded
// as 5 is scaled by ten (lat 490 instead of 49), so it falls outside ±90/±180 and is retried
// as 6. If both look valid, polyline5 wins.
func Decode(text string) (Geometry, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "\ufeff")
	if text == "" {
		return Geometry{}, fmt.Errorf("empty geometry")
	}

	pts5, err5 := DecodePolyline(text, 5)
	ok5 := err5 == nil && len(pts5) >= 2 && coordsOnGlobe(pts5)
	if ok5 {
		return Geometry{Points: pts5, Format: FormatPolyline5}, nil
	}

	pts6, err6 := DecodePolyline(text, 6)
	ok6 := err6 == nil && len(pts6) >= 2 && coordsOnGlobe(pts6)
	if ok6 {
		return Geometry{Points: pts6, Format: FormatPolyline6}, nil
	}

	if err5 != nil {
		return Geometry{}, err5
	}
	if err6 != nil {
		return Geometry{}, err6
	}
	n := len(pts5)
	if len(pts6) > n {
		n = len(pts6)
	}
	if n < 2 {
		return Geometry{}, fmt.Errorf("decoded to %d points, need at least 2", n)
	}
	return Geometry{}, fmt.Errorf("coordinates out of range as polyline5 and polyline6")
}

func coordsOnGlobe(pts Points) bool {
	for _, p := range pts {
		if p.Lat < -90 || p.Lat > 90 || p.Lon < -180 || p.Lon > 180 {
			return false
		}
	}
	return true
}
