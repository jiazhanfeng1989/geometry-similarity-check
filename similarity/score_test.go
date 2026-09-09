package similarity

import (
	"math"
	"testing"
)

func offsetEast(points Points, metres float64) Points {
	shifted := make(Points, 0, len(points))
	for _, p := range points {
		cosLat := math.Cos(p.Lat * math.Pi / 180.0)
		shifted = append(shifted, Point{Lat: p.Lat, Lon: p.Lon + metres/(metresPerDegreeLon*cosLat)})
	}
	return shifted
}

func straightLine(startLat, lon float64, spacingMeters float64, count int) Points {
	points := make(Points, 0, count)
	step := spacingMeters / metresPerDegreeLat
	for i := 0; i < count; i++ {
		points = append(points, Point{Lat: startLat + float64(i)*step, Lon: lon})
	}
	return points
}

func TestScoreIdenticalGeometry(t *testing.T) {
	leg := straightLine(37.0, -122.0, 100, 20)

	score, err := Score(leg, leg)
	if err != nil {
		t.Fatal(err)
	}
	if score.Coverage != 1.0 {
		t.Fatalf("coverage = %v, want 1", score.Coverage)
	}
	if score.MaxDeviation >= 1.0 {
		t.Fatalf("max deviation = %v, want < 1", score.MaxDeviation)
	}
	if !score.SimilarTo(0.9) {
		t.Fatal("identical geometry should be similar")
	}
}

func TestScoreDisjointGeometry(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 20)
	candidate := offsetEast(reference, 2000)

	score, err := Score(candidate, reference)
	if err != nil {
		t.Fatal(err)
	}
	if score.Coverage != 0 {
		t.Fatalf("coverage = %v, want 0", score.Coverage)
	}
	if score.SimilarTo(0.9) {
		t.Fatal("disjoint geometry should not be similar")
	}
}

func TestCoverageCorridorBoundary(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 20)

	inside, err := Score(offsetEast(reference, CorridorMeters*0.5), reference)
	if err != nil {
		t.Fatal(err)
	}
	if inside.Coverage != 1.0 {
		t.Fatalf("inside coverage = %v, want 1", inside.Coverage)
	}

	outside, err := Score(offsetEast(reference, CorridorMeters*2), reference)
	if err != nil {
		t.Fatal(err)
	}
	if outside.Coverage != 0 {
		t.Fatalf("outside coverage = %v, want 0", outside.Coverage)
	}
}

func TestScoreHighCoverageWithLongDetourIsVetoed(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 200)

	candidate := make(Points, len(reference))
	copy(candidate, reference)
	detour := offsetEast(reference[195:], 1000)
	copy(candidate[195:], detour)

	score, err := Score(candidate, reference)
	if err != nil {
		t.Fatal(err)
	}
	if score.Coverage <= 0.9 {
		t.Fatalf("coverage = %v, want > 0.9", score.Coverage)
	}
	if score.MaxDeviation <= MaxDeviationMeters {
		t.Fatalf("max deviation = %v, want > %v", score.MaxDeviation, MaxDeviationMeters)
	}
	if score.SimilarTo(0.9) {
		t.Fatal("the veto must reject it despite the coverage")
	}
}

func TestScoreCandidateCoveringOnlyPartOfTheReference(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 100)

	tests := []struct {
		name      string
		candidate Points
		similar   bool
	}{
		{name: "only the first fifth", candidate: reference[:20]},
		{name: "only the last fifth", candidate: reference[80:]},
		{name: "only a stretch in the middle", candidate: reference[40:60]},
		{name: "stops 200 m short of the end", candidate: reference[:98], similar: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			score, err := Score(test.candidate, reference)
			if err != nil {
				t.Fatal(err)
			}
			if score.Coverage != 1.0 {
				t.Fatalf("coverage = %v, want 1", score.Coverage)
			}
			if score.SimilarTo(0.9) != test.similar {
				t.Fatalf("similar = %v, want %v", score.SimilarTo(0.9), test.similar)
			}
			if test.similar {
				if math.Abs(score.MaxDeviation-200) > 1 {
					t.Fatalf("max deviation = %v, want ~200", score.MaxDeviation)
				}
				return
			}
			if score.MaxDeviation <= MaxDeviationMeters {
				t.Fatalf("max deviation = %v, want > %v", score.MaxDeviation, MaxDeviationMeters)
			}
		})
	}
}

func TestMaxDeviationIsLatitudeInvariant(t *testing.T) {
	const offsetMeters = 500.0

	equatorial := straightLine(1.0, 10.0, 100, 20)
	arctic := straightLine(70.0, 10.0, 100, 20)

	equatorialScore, err := Score(offsetEast(equatorial, offsetMeters), equatorial)
	if err != nil {
		t.Fatal(err)
	}
	arcticScore, err := Score(offsetEast(arctic, offsetMeters), arctic)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(equatorialScore.MaxDeviation-offsetMeters) > 20 {
		t.Fatalf("equatorial deviation = %v, want ~%v", equatorialScore.MaxDeviation, offsetMeters)
	}
	if math.Abs(arcticScore.MaxDeviation-offsetMeters) > 20 {
		t.Fatalf("arctic deviation = %v, want ~%v", arcticScore.MaxDeviation, offsetMeters)
	}
	if math.Abs(equatorialScore.MaxDeviation-arcticScore.MaxDeviation) > 20 {
		t.Fatalf("equatorial %v vs arctic %v", equatorialScore.MaxDeviation, arcticScore.MaxDeviation)
	}
}

func TestMaxDeviationWithDifferentVertexSpacing(t *testing.T) {
	dense := straightLine(37.0, -122.0, 50, 40)
	sparse := Points{dense[0], dense[len(dense)-1]}

	bowed := make(Points, len(dense))
	copy(bowed, dense)
	bowed[20] = offsetEast(Points{dense[20]}, 400)[0]

	score, err := Score(bowed, sparse)
	if err != nil {
		t.Fatal(err)
	}
	if score.MaxDeviation <= 300 {
		t.Fatalf("max deviation = %v, want > 300", score.MaxDeviation)
	}
}

func TestSnapOffsetDoesNotTripVeto(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 20)

	candidate := make(Points, len(reference))
	copy(candidate, reference)
	candidate[0] = offsetEast(Points{reference[0]}, 50)[0]

	score, err := Score(candidate, reference)
	if err != nil {
		t.Fatal(err)
	}
	if score.MaxDeviation >= MaxDeviationMeters {
		t.Fatalf("max deviation = %v, want < %v", score.MaxDeviation, MaxDeviationMeters)
	}
	if !score.SimilarTo(0.9) {
		t.Fatal("a 50 m snap offset must not trip the veto")
	}
}

func TestScoreRejectsDegenerateGeometry(t *testing.T) {
	leg := straightLine(37.0, -122.0, 100, 20)

	if _, err := Score(Points{leg[0]}, leg); err == nil {
		t.Fatal("expected error for a single-point target")
	}
	if _, err := Score(leg, Points{}); err == nil {
		t.Fatal("expected error for an empty source")
	}
}

func sharedHighwayWithExit(reference Points, metresPerStep float64) Points {
	candidate := make(Points, 0, len(reference))
	candidate = append(candidate, reference[:80]...)
	for i, point := range reference[80:] {
		candidate = append(candidate, offsetEast(Points{point}, float64(i+1)*metresPerStep)[0])
	}
	return candidate
}

func TestCoverageOnASharedHighwayWithANearbyExit(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 100)

	score, err := Score(sharedHighwayWithExit(reference, 10), reference)
	if err != nil {
		t.Fatal(err)
	}
	if score.MaxDeviation >= MaxDeviationMeters {
		t.Fatalf("max deviation = %v, want < %v", score.MaxDeviation, MaxDeviationMeters)
	}
	if score.Coverage <= 0.75 || score.Coverage >= 0.9 {
		t.Fatalf("coverage = %v, want (0.75, 0.9)", score.Coverage)
	}
	if score.SimilarTo(0.9) {
		t.Fatal("a leg leaving at a different exit is not the reference leg")
	}
	if !score.SimilarTo(0.7) {
		t.Fatal("the same leg passes under a threshold loose enough to allow it")
	}
}

func TestCoverageOnASharedHighwayWithADistantExit(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 100)

	score, err := Score(sharedHighwayWithExit(reference, 40), reference)
	if err != nil {
		t.Fatal(err)
	}
	if score.MaxDeviation <= MaxDeviationMeters {
		t.Fatalf("max deviation = %v, want > %v", score.MaxDeviation, MaxDeviationMeters)
	}
	if score.SimilarTo(0.1) {
		t.Fatal("no threshold can accept a distant exit")
	}
}

func TestCoverageIsDirectionAgnostic(t *testing.T) {
	reference := straightLine(37.0, -122.0, 100, 20)

	reversed := make(Points, 0, len(reference))
	for i := len(reference) - 1; i >= 0; i-- {
		reversed = append(reversed, reference[i])
	}

	score, err := Score(reversed, reference)
	if err != nil {
		t.Fatal(err)
	}
	if score.Coverage != 1.0 {
		t.Fatalf("coverage = %v, want 1", score.Coverage)
	}
	if score.MaxDeviation >= 1.0 {
		t.Fatalf("max deviation = %v, want < 1", score.MaxDeviation)
	}
}

func TestCoverageWithOversizedSegments(t *testing.T) {
	reference := straightLine(37.0, -122.0, 2000, 5)
	candidate := straightLine(37.0, -122.0, 100, 80)

	score, err := Score(candidate, reference)
	if err != nil {
		t.Fatal(err)
	}
	if score.Coverage <= 0.9 {
		t.Fatalf("coverage = %v, want > 0.9", score.Coverage)
	}
}

func TestAnalyzeUsedPair(t *testing.T) {
	source := Geometry{Points: straightLine(37.0, -122.0, 100, 20), Format: FormatGeoJSON}
	target := Geometry{Points: offsetEast(source.Points, 10), Format: FormatGeoJSON}

	report, err := Analyze(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Similar || report.Verdict != "used" {
		t.Fatalf("report = %+v, want used", report)
	}
	if len(report.OutsideStretches) != 0 {
		t.Fatalf("outside stretches = %d, want 0", len(report.OutsideStretches))
	}
}

func TestAnalyzeReportsOutsideStretches(t *testing.T) {
	source := Geometry{Points: straightLine(37.0, -122.0, 100, 20), Format: FormatPolyline5}
	target := Geometry{Points: offsetEast(source.Points, 2000), Format: FormatPolyline5}

	report, err := Analyze(source, target)
	if err != nil {
		t.Fatal(err)
	}
	if report.Similar || report.Verdict != "not_used" {
		t.Fatalf("report = %+v, want not_used", report)
	}
	if len(report.OutsideStretches) == 0 {
		t.Fatal("expected outside stretches")
	}
}
