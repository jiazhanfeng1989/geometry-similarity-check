package similarity

import (
	"fmt"
	"math"
)

// Similarity describes how closely a target polyline follows a source polyline.
type Similarity struct {
	// Coverage is the share of the target's length that runs inside the source's corridor, in
	// [0, 1].
	Coverage float64

	// MaxDeviation is the worst gap in metres between the two geometries, saturating at
	// DeviationCeilingMeters.
	MaxDeviation float64
}

// Similar reports whether the target follows the source closely enough, using CoverageThreshold
// and MaxDeviationMeters. A threshold of zero switches the check off and accepts every pair.
func (s Similarity) Similar() bool {
	return s.SimilarTo(CoverageThreshold)
}

// SimilarTo is Similar with an explicit coverage threshold. Zero switches the check off.
func (s Similarity) SimilarTo(coverageThreshold float64) bool {
	if coverageThreshold <= 0 {
		return true
	}
	return s.Coverage >= coverageThreshold && s.MaxDeviation <= MaxDeviationMeters
}

// Score measures target against source.
//
// Coverage is a length ratio, not a shape distance, because the configured threshold is a
// percentage of length and a distance is worst-case: one spur would make an identical line look
// unrelated. MaxDeviation is the Hausdorff distance between the two geometries, taken in both
// directions so that a target skipping a stretch the source covers is caught as well as one
// straying from it.
func Score(target, source Points) (Similarity, error) {
	if len(target) < 2 || len(source) < 2 {
		return Similarity{}, fmt.Errorf("both geometries need at least 2 points, got %d and %d", len(target), len(source))
	}

	targetIndex := newSegmentIndex(target)
	sourceIndex := newSegmentIndex(source)

	return Similarity{
		Coverage:     coverage(target, sourceIndex),
		MaxDeviation: maxDeviation(target, targetIndex, source, sourceIndex),
	}, nil
}

// coverage is the length-weighted share of the target that runs within CorridorMeters of the
// source.
func coverage(target Points, source *segmentIndex) float64 {
	inside := make([]bool, len(target))
	for i := range target {
		inside[i] = source.minDistance(target[i], CorridorMeters) <= CorridorMeters
	}

	var covered, total float64
	for i := 1; i < len(target); i++ {
		length := Haversine(target[i-1], target[i])
		total += length
		if inside[i-1] && inside[i] {
			covered += length
		}
	}

	if total == 0 {
		return 0
	}
	return covered / total
}

func maxDeviation(target Points, targetIndex *segmentIndex, source Points, sourceIndex *segmentIndex) float64 {
	return math.Max(
		directedMaxDeviation(target, sourceIndex),
		directedMaxDeviation(source, targetIndex),
	)
}

func directedMaxDeviation(points Points, other *segmentIndex) float64 {
	var worst float64
	for _, point := range points {
		distance := other.minDistance(point, DeviationCeilingMeters)
		if math.IsInf(distance, 1) {
			// Nothing within the ceiling, and the caller only compares against
			// MaxDeviationMeters, so there is no point measuring how much further it is.
			return DeviationCeilingMeters
		}

		worst = math.Max(worst, distance)
	}
	return worst
}

// cellKey identifies a cell of the segment index grid.
type cellKey struct {
	lat int64
	lon int64
}

// segmentIndex is a uniform grid over a polyline's segments, so a point-to-polyline distance
// costs roughly constant time. A line can carry tens of thousands of vertices, and the worst case
// for a scan, two completely dissimilar lines, is exactly the case we must score.
type segmentIndex struct {
	points    Points
	cells     map[cellKey][]int32
	oversized []int32
	latSize   float64
	lonSize   float64
}

func newSegmentIndex(points Points) *segmentIndex {
	cosLat := math.Cos(midLatitude(points) * math.Pi / 180.0)
	if cosLat < minCosLat {
		cosLat = minCosLat
	}

	index := &segmentIndex{
		points:  points,
		cells:   make(map[cellKey][]int32),
		latSize: GridCellMeters / metresPerDegreeLat,
		lonSize: GridCellMeters / (metresPerDegreeLon * cosLat),
	}

	for segment := 0; segment+1 < len(points); segment++ {
		index.insert(int32(segment))
	}
	return index
}

func (s *segmentIndex) insert(segment int32) {
	start, end := s.points[segment], s.points[segment+1]

	minLatCell, maxLatCell := sortedCells(start.Lat, end.Lat, s.latSize)
	minLonCell, maxLonCell := sortedCells(start.Lon, end.Lon, s.lonSize)

	if maxLatCell-minLatCell >= MaxCellSpan || maxLonCell-minLonCell >= MaxCellSpan {
		s.oversized = append(s.oversized, segment)
		return
	}

	for lat := minLatCell; lat <= maxLatCell; lat++ {
		for lon := minLonCell; lon <= maxLonCell; lon++ {
			key := cellKey{lat: lat, lon: lon}
			s.cells[key] = append(s.cells[key], segment)
		}
	}
}

// minDistance returns the distance in metres from the point to the nearest segment, or +Inf when
// nothing lies within searchLimit. The limit keeps geometries that are nowhere near each other
// cheap to score; +Inf rather than searchLimit keeps "just at the limit" distinguishable from
// "further than we looked".
func (s *segmentIndex) minDistance(point Point, searchLimit float64) float64 {
	best := s.nearestIn(point, s.oversized, math.Inf(1))

	latCell := cellIndex(point.Lat, s.latSize)
	lonCell := cellIndex(point.Lon, s.lonSize)

	maxRing := int64(math.Ceil(searchLimit/GridCellMeters)) + 1
	for ring := int64(0); ring <= maxRing; ring++ {
		// Every point of a cell in this ring is at least (ring-1) cells away, so once the best
		// distance is within that bound no further ring can improve on it.
		if best <= float64(ring-1)*GridCellMeters {
			break
		}

		best = s.nearestInRing(point, latCell, lonCell, ring, best)
	}

	if best > searchLimit {
		return math.Inf(1)
	}
	return best
}

func (s *segmentIndex) nearestInRing(point Point, latCell, lonCell, ring int64, best float64) float64 {
	for lat := latCell - ring; lat <= latCell+ring; lat++ {
		for lon := lonCell - ring; lon <= lonCell+ring; lon++ {
			if max(abs64(lat-latCell), abs64(lon-lonCell)) != ring {
				continue
			}

			best = math.Min(best, s.nearestIn(point, s.cells[cellKey{lat: lat, lon: lon}], best))
		}
	}
	return best
}

func (s *segmentIndex) nearestIn(point Point, segments []int32, best float64) float64 {
	for _, segment := range segments {
		best = math.Min(best, distanceToSegment(point, s.points[segment], s.points[segment+1]))
	}
	return best
}

func sortedCells(a, b, size float64) (int64, int64) {
	first, second := cellIndex(a, size), cellIndex(b, size)
	if first > second {
		return second, first
	}
	return first, second
}

func cellIndex(value, size float64) int64 {
	return int64(math.Floor(value / size))
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
