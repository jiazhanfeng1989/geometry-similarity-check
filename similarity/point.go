package similarity

import (
	"fmt"
	"math"
)

// Point is a WGS84 geographic coordinate. Lat and Lon are in degrees.
type Point struct {
	Lat float64
	Lon float64
}

func (p Point) String() string {
	return fmt.Sprintf("%.6f,%.6f", p.Lat, p.Lon)
}

// Points is a polyline: an ordered sequence of vertices.
type Points []Point

// Length returns the haversine length of the polyline, in metres.
func Length(points Points) float64 {
	var total float64
	for i := 1; i < len(points); i++ {
		total += Haversine(points[i-1], points[i])
	}
	return total
}

// Haversine returns the great-circle distance between two points, in metres.
func Haversine(a, b Point) float64 {
	lat1 := a.Lat * math.Pi / 180
	lat2 := b.Lat * math.Pi / 180
	dLat := lat2 - lat1
	dLon := (b.Lon - a.Lon) * math.Pi / 180

	sinDLat := math.Sin(dLat / 2)
	sinDLon := math.Sin(dLon / 2)
	h := sinDLat*sinDLat + math.Cos(lat1)*math.Cos(lat2)*sinDLon*sinDLon
	if h > 1 {
		h = 1
	}
	return 2 * earthRadiusMeters * math.Asin(math.Sqrt(h))
}

// plane converts lat/lon to metres on a local equirectangular plane centred on the given
// reference parallel. Without it a degree of longitude, which shrinks with latitude, would make
// the same deviation score differently in Texas and in Norway. Over one leg the error is well
// under a metre.
type plane struct {
	lonScale float64
}

func newPlane(referenceLat float64) plane {
	cosLat := math.Cos(referenceLat * math.Pi / 180.0)
	if cosLat < minCosLat {
		cosLat = minCosLat
	}
	return plane{lonScale: metresPerDegreeLon * cosLat}
}

func (p plane) project(point Point) (float64, float64) {
	return point.Lon * p.lonScale, point.Lat * metresPerDegreeLat
}

func midLatitude(points Points) float64 {
	if len(points) == 0 {
		return 0
	}

	minLat, maxLat := points[0].Lat, points[0].Lat
	for _, p := range points[1:] {
		minLat = math.Min(minLat, p.Lat)
		maxLat = math.Max(maxLat, p.Lat)
	}
	return (minLat + maxLat) / 2
}

// distanceToSegment returns the perpendicular distance in metres from point to segment
// [start, end], clamped to the segment's endpoints.
//
// Matching against segments rather than vertices matters: encoded polylines place vertices
// hundreds of metres apart on straight motorway stretches, so a vehicle exactly on the line can
// be far from every vertex.
func distanceToSegment(point, start, end Point) float64 {
	pl := newPlane(point.Lat)
	px, py := pl.project(point)
	sx, sy := pl.project(start)
	ex, ey := pl.project(end)

	dx, dy := ex-sx, ey-sy
	lengthSquared := dx*dx + dy*dy
	if lengthSquared == 0 {
		return math.Hypot(px-sx, py-sy)
	}

	fraction := ((px-sx)*dx + (py-sy)*dy) / lengthSquared
	fraction = math.Max(0, math.Min(1, fraction))

	return math.Hypot(px-(sx+fraction*dx), py-(sy+fraction*dy))
}
