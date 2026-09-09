package similarity

// Tunable parameters. They are package-level variables so they sit in one place with the
// comments that explain what they do. The CLI does not expose flags for them: change a value
// here and recompile (`go build ./cmd/gsc`). Runtime flags would make two runs of the same
// binary incomparable, which is the opposite of what a diagnostic tool wants.
//
// The numbers below are the defaults used by the EV trip-planner reference-route matcher this
// library was extracted from. They are engineering guesses, not derived constants.
//
// What to turn, in order:
//
//  1. CoverageThreshold — "how much of the target may leave the source corridor". This is the
//     primary knob. 0.90 is strict (a nearby highway exit fails); 0.70 is loose. Zero switches
//     the check off and reports every pair as similar.
//  2. CorridorMeters — "what counts as the same road". Too tight and a frontage road or ramp
//     reads as a different road; too loose and the far carriageway of a divided highway reads
//     as identical.
//  3. MaxDeviationMeters — veto on a long detour. A target can score high coverage while the
//     remaining stretch takes a long way around; the veto rejects that regardless of coverage.
//  4. DeviationCeilingMeters — where measuring the gap exactly stops being worth the work.
//     Anything this far off was vetoed long ago. Lowering it toward MaxDeviationMeters is the
//     first move if scoring ever shows up in latency, at the price of losing the exact figure
//     in the report.
//  5. GridCellMeters / MaxCellSpan — performance only. They must not change a verdict; if they
//     do, that is a bug in the index.

var (
	// CoverageThreshold is the share of the target's length that must run inside CorridorMeters
	// of the source, in [0, 1]. 0.90 means 90%. Zero disables the check: every pair is then
	// reported as similar, and the scoring work can still be run for the numbers.
	CoverageThreshold = 0.90

	// CorridorMeters is how far the target may stray from the source and still count as covered.
	CorridorMeters = 30.0

	// MaxDeviationMeters vetoes a pair whose worst gap exceeds it, however good its coverage.
	MaxDeviationMeters = 300.0

	// DeviationCeilingMeters is where measuring the deviation exactly stops being worth the
	// work. Anything this far off was vetoed long ago, and the bound is what keeps two unrelated
	// geometries cheap to score.
	DeviationCeilingMeters = 2000.0

	// GridCellMeters sizes the segment-index cells, and must be at least CorridorMeters so a
	// corridor query only looks at the nine cells around the point. Corridor-sized cells would
	// make one sparse motorway segment span hundreds of cells; very large cells put too many
	// segments in each.
	GridCellMeters = 100.0

	// MaxCellSpan caps how many cells along one axis a segment may be registered in. Longer
	// segments (sparse motorway geometry, ferry links) go in the oversized list, which every
	// query scans.
	MaxCellSpan int64 = 8

	// MaxReportedStretches keeps two unrelated routes from printing a line per vertex.
	MaxReportedStretches = 10
)

const (
	metresPerDegreeLat = 110574.0
	metresPerDegreeLon = 111320.0

	// minCosLat keeps the longitude scale finite near the poles.
	minCosLat = 0.0001

	// earthRadiusMeters is the mean Earth radius used for path length (coverage weighting and
	// "km along" in the report). Point-to-segment distances use the local metre plane instead,
	// so this value does not affect MaxDeviation.
	earthRadiusMeters = 6371000.0
)
