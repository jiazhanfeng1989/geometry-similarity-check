# geometry-similarity-check

Compare two Google encoded polylines and report how closely the **target** follows the **source**.

The score is a length-weighted corridor coverage plus a Hausdorff-style deviation veto — the same metric used to decide whether a routing engine followed a client-supplied reference route. This repository is a standalone, dependency-free extraction of that logic.

## Install

```bash
git clone https://github.com/zhfjia/geometry-similarity-check.git
cd geometry-similarity-check
go build -o gsc ./cmd/gsc
```

Go 1.22 or later. No CGo, no third-party modules.

## Usage

```bash
gsc -data <pair.json>
```

One JSON file: an array of pairs. Keys match the report roles (`source` is the reference, `target` is what is being checked):

```json
[
  {"source": "<google encoded polyline>", "target": "<google encoded polyline>"}
]
```

`encoding/json` unescapes the strings, so a polyline copied out of JSON (where `\` is written `\\`) decodes correctly. Example: [`testdata/example.json`](testdata/example.json).

Only **polyline5** and **polyline6** are supported. Precision is detected from the coordinates: decode as 5 first; if any point falls off the globe (lat outside ±90 or lon outside ±180), retry as 6. A polyline6 decoded as 5 is scaled by ten (49° becomes 490). If both look valid, polyline5 is used. The report header prints the detected format and both endpoints so a mix-up is obvious.

| Flag | Meaning |
|---|---|
| `-data` | JSON array of `{source, target}` |
| `-json` | Print the report as JSON |
| `-version` | Print version and exit |

```bash
./gsc -data testdata/example.json
./gsc -json -data testdata/example.json
```

Exit codes: `0` all pairs similar, `1` any pair not similar, `2` usage or decode error. Text output is tab-separated: first column is a stable key (same names as the JSON fields), remaining columns are values. Each pair starts with `pair	i/n`. `-json` prints an array of reports.

## Sample report

```
pair	1/2
verdict	similar
coverage	0.9301
coverage_threshold	0.90
max_deviation_m	118.9
max_deviation_veto_m	300
corridor_m	30
source	polyline5	89	3.84	49.056450,2.146330	49.046690,2.099650
target	polyline6	334	4.21	49.056429,2.146293	49.046676,2.099424
worst_target_from_source	118.9	319	4.08	49.046680,2.097978
worst_source_from_target	6.8	79	3.59	49.048550,2.101180
outside_stretch_count	1
outside_stretch	310	324	3.90	4.16	0.26	49.046381,2.099829	49.046960,2.099046

pair	2/2
verdict	not_similar
coverage	0.4190
coverage_threshold	0.90
max_deviation_m	2000.0
max_deviation_veto_m	300
corridor_m	30
source	polyline5	362	34.94	52.603940,4.687350	52.375560,4.769670
target	polyline6	1440	37.99	52.603942,4.687404	52.375401,4.769405
worst_target_from_source	2000.0	212	3.09	52.607341,4.716828
worst_source_from_target	1940.5	100	6.58	52.557310,4.689360
outside_stretch_count	2
outside_stretch	5	638	0.11	16.60	16.49	52.604771,4.687619	52.517660,4.717389
outside_stretch	788	947	20.64	26.00	5.35	52.485211,4.691489	52.440420,4.668640
```

Columns after the key:

| Key | Columns |
|---|---|
| `pair` | `i/n` (1-based index / pair count) |
| `source` / `target` | format, points, length_km, start, end |
| `worst_target_from_source` / `worst_source_from_target` | distance_m, point_index, along_km, coord |
| `outside_stretch` | from, to, from_km, to_km, length_km, from_coord, to_coord |

`worst_target_from_source` is target → source (a detour). `worst_source_from_target` is source → target (the target missed a stretch of source). Outside stretches turn a coverage number into somewhere to look on a map.

## Viewing the geometry

Paste the encoded `source` / `target` strings onto a map. Use the precision printed in the report (`polyline5` vs `polyline6`); the wrong one places the line far off the globe.

JSON writes a real `\` as `\\`. `gsc` unescapes that when it reads `-data`. If you copy the string out of the JSON file and paste it into a viewer, unescape it first (or use the checkbox below).

**[Valhalla Polyline Viewer](https://valhalla.github.io/demos/polyline/)** — both precisions; overlay source and target on one map.

1. Check **Polyline6** for a `polyline6` row; leave it unchecked for `polyline5`.
2. Check **Unescape `\`** if the string came from a JSON file.
3. Paste one encoded polyline per line, then decode.

**[Google Interactive Polyline Utility](https://developers.google.com/maps/documentation/utilities/polylineutility)** — polyline5 only.

1. Paste into **Encoded Polyline**.
2. Check **Unescape special characters** if the string came from a JSON file.
3. Click **Decode Polyline**.

Do not use the Google utility for `polyline6`.

## How similarity is decided

Two checks, in sequence:

1. **Coverage** — length-weighted share of the *target* that runs within `CorridorMeters` of the source. A length ratio, not a shape distance: one spur would make an identical line look unrelated under Fréchet or Hausdorff alone.
2. **Max deviation** — the worst vertex-to-segment gap between the two geometries, in both directions, saturating at `DeviationCeilingMeters`. A target can score 90% coverage while the remaining 10% takes a long detour; the veto rejects that regardless of coverage.

The pair is **similar** when `coverage >= CoverageThreshold` **and** `max deviation <= MaxDeviationMeters`. A threshold of zero switches the check off and reports every pair as similar.

`Score(target, source)` — coverage is how much of the **target** lies on the **source**. A target that is a subset of the source scores coverage 1.0; the reverse direction of the deviation is what judges a truncated line.

Distances to the polyline are true point-to-segment projections on a local metre plane, not vertex-to-vertex. Encoded polylines place vertices hundreds of metres apart on straight motorways; a point exactly on the line can be far from every vertex.

A uniform grid index keeps the worst case (two completely dissimilar long lines) roughly linear in the vertex count rather than quadratic.

## Tunable parameters (rebuild required)

Thresholds are **package-level variables** in [`similarity/params.go`](similarity/params.go), not CLI flags. Two runs of the same binary stay comparable; changing a number and rebuilding is the way to retune.

```go
var (
    CoverageThreshold      = 0.90  // 90% of the target must stay in the corridor
    CorridorMeters          = 30.0  // what counts as "the same road"
    MaxDeviationMeters      = 300.0 // veto on a long detour
    DeviationCeilingMeters  = 2000.0
    GridCellMeters          = 100.0  // index performance only
    MaxCellSpan       int64 = 8
    MaxReportedStretches    = 10
)
```

Each variable has a longer comment in that file: what it measures, what happens if you set it too tight or too loose, and which one to turn first.

```bash
# example: accept a nearby highway exit (coverage 0.70 instead of 0.90)
# edit similarity/params.go, then:
go build -o gsc ./cmd/gsc
```

## Library

```go
import "github.com/zhfjia/geometry-similarity-check/similarity"

pairs, err := similarity.LoadFile("example.json")
if err != nil {
    return err
}

for _, pair := range pairs {
    report, err := similarity.Analyze(pair.Source, pair.Target)
    if err != nil {
        return err
    }
    _ = report
}
```

`LoadFile` reads `[{"source": "...", "target": "..."}, ...]`. `Decode` parses a polyline string and detects polyline5 vs polyline6 the same way.