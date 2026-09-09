# AGENTS.md

Guidance for coding agents working in this repository. Human-facing usage lives in `README.md`.

## What this is

A standalone Go library (`similarity`) plus CLI (`gsc`) that scores how closely a **target** Google encoded polyline follows a **source**. Coverage is a length-weighted corridor check; max deviation is a bidirectional Hausdorff veto. Extracted from an EV trip planner — no HTTP service, no trimming, no way-id matching.

Go 1.22+, standard library only. No CGo, no third-party modules.

## Layout

- `cmd/gsc/` — CLI. Keep `main.go` small: parse flags, `LoadFile`, `Analyze`, print.
- `similarity/` — decode, score, report, tunables.
- `testdata/example.json` — example pair (`source` + `target` polylines).
- Thresholds live in `similarity/params.go` as package-level variables, **not** CLI flags. Changing them requires a rebuild so two runs of the same binary stay comparable.

## Commands

```bash
go build -o gsc ./cmd/gsc
go test ./...
go test ./similarity -bench=.
./gsc -data testdata/example.json
```

Exit codes: `0` all similar, `1` any not similar, `2` usage or decode error.

## CLI contract (do not expand)

`gsc` uses the standard library `flag` package. No custom usage banner — `-h` is `PrintDefaults` only; how to run it belongs in `README.md`.

Required: `-data`. Optional: `-json` (report as JSON), `-version`.

Do **not** add:

- `-source` / `-target` as separate files
- positional arguments
- geometry strings on the command line
- stdin / `-` as a path
- `-source-format`, `-target-format`, `-format`, or `auto`
- GeoJSON, WKT, point-array, lat/lon CSV, or other encodings
- flags for `CoverageThreshold` / `CorridorMeters` / etc.

## Input

One JSON file — an array of pairs:

```json
[{"source": "...", "target": "..."}, {"source": "...", "target": "..."}]
```

JSON keys are `source` and `target` (same words as the report). `LoadFile` returns `[]Pair`. JSON unescaping is required: polylines copied from JSON contain `\\` for a real `\`; treating the file as a raw polyline string yields `truncated polyline`.

Only **polyline5** and **polyline6**. Detection in `Decode`: try precision 5; if any point is off the globe (lat outside ±90 or lon outside ±180), retry as 6. If both look valid, keep 5. A polyline6 decoded as 5 is scaled by ten (49° → 490). The report must keep the detected format and both endpoints.

## Scoring

`Score(target, source)` — coverage is how much of the **target** lies on the **source**. A target that is a subset of the source scores coverage 1.0. Max deviation is measured both ways so a truncated target is caught as well as a detour.

Default text is TSV: first column is a stable key matching the JSON field names. CLI prefixes each pair with `pair	i/n`. Scalar keys: `verdict`, `coverage`, `coverage_threshold`, `max_deviation_m`, `max_deviation_veto_m`, `corridor_m`. Multi-column rows after the key:

- `source` / `target` — format, points, length_km, start, end
- `worst_target_from_source` — target → source (detour / wrong road): distance_m, point_index, along_km, coord
- `worst_source_from_target` — source → target (target missed a stretch of source): same columns
- `outside_stretch_count` then zero or more `outside_stretch` rows — from, to, from_km, to_km, length_km, from_coord, to_coord

Do not invert the argument order of `Score` / `Analyze`. Distances are point-to-segment on a local metre plane, not vertex-to-vertex.

## Style

- Prefer small, boring stdlib code over helpers that reimplement `flag` or JSON.
- Put explanations of thresholds in comments on the variables in `params.go`, not in CLI help.
- Tests should write temp `[{"source":"...","target":"..."}]` files (or use `testdata/example.json`); do not pass encoded strings as `-data`.
- After changing CLI, decode, or the report, update `README.md` to match. Sample report output should come from a real `gsc` run against `testdata/`.
