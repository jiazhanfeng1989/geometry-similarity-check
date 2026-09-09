package similarity

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strings"
)

// Report is the diagnostic dump produced for a source/target pair. It is what the CLI prints,
// matching the original "paste two geometries and see where they part" workflow.
type Report struct {
	Verdict               string      `json:"verdict"`
	Similar               bool        `json:"similar"`
	Coverage              float64     `json:"coverage"`
	CoverageThreshold     float64     `json:"coverage_threshold"`
	MaxDeviationM         float64     `json:"max_deviation_m"`
	MaxDeviationVetoM     float64     `json:"max_deviation_veto_m"`
	CorridorM             float64     `json:"corridor_m"`
	Source                LineInfo    `json:"source"`
	Target                LineInfo    `json:"target"`
	WorstTargetFromSource *WorstPoint `json:"worst_target_from_source"`
	WorstSourceFromTarget *WorstPoint `json:"worst_source_from_target"`
	OutsideStretches      []Stretch   `json:"outside_stretches"`
}

// LineInfo is the decoded identity of one input, printed so a precision-5/precision-6 mix-up
// is obvious from the endpoints.
type LineInfo struct {
	Role     string  `json:"role"`
	Format   Format  `json:"format"`
	Points   int     `json:"points"`
	LengthKM float64 `json:"length_km"`
	Start    string  `json:"start"`
	End      string  `json:"end"`
}

// WorstPoint names the single worst vertex in one direction of the Hausdorff measure.
type WorstPoint struct {
	Label      string  `json:"label"`
	DistanceM  float64 `json:"distance_m"`
	PointIndex int     `json:"point_index"`
	AlongKM    float64 `json:"along_km"`
	Coord      string  `json:"coord"`
}

// Stretch is a run of target vertices that fall outside the source corridor.
type Stretch struct {
	From      int     `json:"from"`
	To        int     `json:"to"`
	FromKM    float64 `json:"from_km"`
	ToKM      float64 `json:"to_km"`
	LengthKM  float64 `json:"length_km"`
	FromCoord string  `json:"from_coord"`
	ToCoord   string  `json:"to_coord"`
}

// Analyze scores target against source and locates the disagreement, if any.
func Analyze(source, target Geometry) (Report, error) {
	score, err := Score(target.Points, source.Points)
	if err != nil {
		return Report{}, err
	}

	similar := score.Similar()
	verdict := "not_similar"
	if similar {
		verdict = "similar"
	}

	sourceIndex := newSegmentIndex(source.Points)
	targetIndex := newSegmentIndex(target.Points)

	return Report{
		Verdict:               verdict,
		Similar:               similar,
		Coverage:              score.Coverage,
		CoverageThreshold:     CoverageThreshold,
		MaxDeviationM:         score.MaxDeviation,
		MaxDeviationVetoM:     MaxDeviationMeters,
		CorridorM:             CorridorMeters,
		Source:                lineInfo("source", source),
		Target:                lineInfo("target", target),
		WorstTargetFromSource: worstPoint("target straying from source", target.Points, sourceIndex),
		WorstSourceFromTarget: worstPoint("source that target skips", source.Points, targetIndex),
		OutsideStretches:      outsideStretches(target.Points, sourceIndex),
	}, nil
}

func lineInfo(role string, geom Geometry) LineInfo {
	points := geom.Points
	return LineInfo{
		Role:     role,
		Format:   geom.Format,
		Points:   len(points),
		LengthKM: Length(points) / 1000,
		Start:    points[0].String(),
		End:      points[len(points)-1].String(),
	}
}

func worstPoint(label string, points Points, other *segmentIndex) *WorstPoint {
	worst, at, along := -1, 0.0, 0.0
	var run float64
	for i, point := range points {
		if i > 0 {
			run += Haversine(points[i-1], point)
		}

		distance := other.minDistance(point, DeviationCeilingMeters)
		if math.IsInf(distance, 1) {
			distance = DeviationCeilingMeters
		}

		if distance > at {
			worst, at, along = i, distance, run
		}
	}

	if worst < 0 {
		return nil
	}
	return &WorstPoint{
		Label:      label,
		DistanceM:  at,
		PointIndex: worst,
		AlongKM:    along / 1000,
		Coord:      points[worst].String(),
	}
}

func outsideStretches(target Points, source *segmentIndex) []Stretch {
	inside := make([]bool, len(target))
	along := make([]float64, len(target))
	for i, point := range target {
		if i > 0 {
			along[i] = along[i-1] + Haversine(target[i-1], point)
		}
		inside[i] = source.minDistance(point, CorridorMeters) <= CorridorMeters
	}

	var stretches []Stretch
	for i, ok := range inside {
		if ok {
			continue
		}
		if len(stretches) > 0 && stretches[len(stretches)-1].To == i-1 {
			stretches[len(stretches)-1].To = i
			continue
		}
		stretches = append(stretches, Stretch{From: i, To: i})
	}

	for i := range stretches {
		s := &stretches[i]
		s.FromKM = along[s.From] / 1000
		s.ToKM = along[s.To] / 1000
		s.LengthKM = s.ToKM - s.FromKM
		s.FromCoord = target[s.From].String()
		s.ToCoord = target[s.To].String()
	}
	return stretches
}

// Write writes a tab-separated report. The first column is a stable key (same names as the
// JSON fields); remaining columns are values. One record per line, so `awk -F'\t'` can parse it.
func (r Report) Write(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "verdict\t%s\n", r.Verdict)
	fmt.Fprintf(&b, "coverage\t%.4f\n", r.Coverage)
	fmt.Fprintf(&b, "coverage_threshold\t%.2f\n", r.CoverageThreshold)
	fmt.Fprintf(&b, "max_deviation_m\t%.1f\n", r.MaxDeviationM)
	fmt.Fprintf(&b, "max_deviation_veto_m\t%.0f\n", r.MaxDeviationVetoM)
	fmt.Fprintf(&b, "corridor_m\t%.0f\n", r.CorridorM)
	writeLineInfo(&b, r.Source)
	writeLineInfo(&b, r.Target)
	if r.WorstTargetFromSource != nil {
		writeWorst(&b, "worst_target_from_source", r.WorstTargetFromSource)
	}
	if r.WorstSourceFromTarget != nil {
		writeWorst(&b, "worst_source_from_target", r.WorstSourceFromTarget)
	}
	fmt.Fprintf(&b, "outside_stretch_count\t%d\n", len(r.OutsideStretches))
	for i, s := range r.OutsideStretches {
		if i == MaxReportedStretches {
			fmt.Fprintf(&b, "outside_stretch_omitted\t%d\n", len(r.OutsideStretches)-MaxReportedStretches)
			break
		}
		fmt.Fprintf(&b, "outside_stretch\t%d\t%d\t%.2f\t%.2f\t%.2f\t%s\t%s\n",
			s.From, s.To, s.FromKM, s.ToKM, s.LengthKM, s.FromCoord, s.ToCoord)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeLineInfo(b *strings.Builder, info LineInfo) {
	fmt.Fprintf(b, "%s\t%s\t%d\t%.2f\t%s\t%s\n",
		info.Role, info.Format, info.Points, info.LengthKM, info.Start, info.End)
}

func writeWorst(b *strings.Builder, key string, w *WorstPoint) {
	fmt.Fprintf(b, "%s\t%.1f\t%d\t%.2f\t%s\n", key, w.DistanceM, w.PointIndex, w.AlongKM, w.Coord)
}

// JSON returns the report as indented JSON.
func (r Report) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
