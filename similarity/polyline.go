package similarity

import (
	"fmt"
	"math"
	"strings"
)

// DecodePolyline decodes a Google encoded polyline at the given decimal precision
// (5 for the classic encoding, 6 for polyline6).
func DecodePolyline(encoded string, precision int) (Points, error) {
	if precision < 1 || precision > 8 {
		return nil, fmt.Errorf("polyline precision must be 1-8, got %d", precision)
	}
	if encoded == "" {
		return nil, fmt.Errorf("empty polyline")
	}

	scale := 1.0
	for range precision {
		scale *= 10
	}

	var lat, lon int
	var points Points
	i := 0
	for i < len(encoded) {
		dlat, next, err := decodePolylineDelta(encoded, i)
		if err != nil {
			return nil, err
		}
		i = next

		dlon, next, err := decodePolylineDelta(encoded, i)
		if err != nil {
			return nil, err
		}
		i = next

		lat += dlat
		lon += dlon
		points = append(points, Point{
			Lat: float64(lat) / scale,
			Lon: float64(lon) / scale,
		})
	}
	return points, nil
}

func decodePolylineDelta(s string, i int) (int, int, error) {
	var result int
	var shift uint
	for {
		if i >= len(s) {
			return 0, i, fmt.Errorf("truncated polyline")
		}
		b := int(s[i]) - 63
		i++
		result |= (b & 0x1f) << shift
		shift += 5
		if b < 0x20 {
			break
		}
	}
	if result&1 != 0 {
		return ^(result >> 1), i, nil
	}
	return result >> 1, i, nil
}

// EncodePolyline encodes points as a Google polyline at the given decimal precision.
func EncodePolyline(points Points, precision int) (string, error) {
	if precision < 1 || precision > 8 {
		return "", fmt.Errorf("polyline precision must be 1-8, got %d", precision)
	}

	scale := 1.0
	for range precision {
		scale *= 10
	}

	var b strings.Builder
	var prevLat, prevLon int
	for _, p := range points {
		lat := int(math.Round(p.Lat * scale))
		lon := int(math.Round(p.Lon * scale))
		encodePolylineDelta(&b, lat-prevLat)
		encodePolylineDelta(&b, lon-prevLon)
		prevLat, prevLon = lat, lon
	}
	return b.String(), nil
}

func encodePolylineDelta(b *strings.Builder, value int) {
	v := value << 1
	if value < 0 {
		v = ^v
	}
	for v >= 0x20 {
		b.WriteByte(byte((0x20 | (v & 0x1f)) + 63))
		v >>= 5
	}
	b.WriteByte(byte(v + 63))
}
