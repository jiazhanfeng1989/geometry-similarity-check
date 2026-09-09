# geometry-similarity-check

Compare two route geometries and report how closely the **target** follows the **source**.

The score is a length-weighted corridor coverage plus a Hausdorff-style deviation veto — the same metric used to decide whether a routing engine followed a client-supplied reference route. This repository is a standalone, dependency-free extraction of that logic: two inputs in, a diagnostic report out.

中文说明见下方 [中文](#中文)。

## Install

```bash
git clone https://github.com/zhfjia/geometry-similarity-check.git
cd geometry-similarity-check
go build -o gsc ./cmd/gsc
```

Go 1.22 or later. No CGo, no third-party modules.

## Usage

```bash
gsc [flags] <source> <target>
```

Each argument is a **file path**, a **geometry string**, or `-` for stdin (only one side). If the argument names an existing file, the file is read; otherwise it is treated as the geometry itself.

```bash
# two GeoJSON files (included as examples)
./gsc testdata/source.geojson testdata/target-close.geojson

# a polyline5 reference against a polyline6 response
./gsc --source-format polyline5 --target-format polyline6 source.poly target.poly6

# mixed: file + literal point-array
./gsc testdata/source.geojson '{37.0,-122.0;37.009043,-122.0}'

# machine-readable
./gsc --json testdata/source.geojson testdata/target-far.geojson
```

Exit codes: `0` similar (`used`), `1` not similar (`not_used`), `2` usage or decode error.

### Input formats

| Format | `-source-format` / `-target-format` | Coordinate order | Notes |
|---|---|---|---|
| Auto (default) | `auto` | — | Tries WKT, GeoJSON, point-array, lat/lon CSV; otherwise polyline **precision 5** |
| Google polyline 5 | `polyline5` / `polyline` | — | Default for encoded strings |
| Google polyline 6 | `polyline6` | — | **Never auto-detected.** A polyline6 decoded as 5 is scaled by ten and the same roads look unrelated |
| Point array | `pointarray` | lat, lon | `{lat1,lon1;lat2,lon2}` |
| GeoJSON | `geojson` | lon, lat | LineString, MultiLineString, Feature, FeatureCollection |
| WKT | `wkt` | lon, lat | `LINESTRING (lon lat, ...)` |
| CSV | `latlon` | lat, lon | One `lat,lon` per line; a header row is optional |

Polyline strings with backslashes are safer in a file than on the shell command line.

## Sample report

```
source     geojson    2 points, 1.01 km, 37.000000,-122.000000 to 37.009043,-122.000000
target     geojson    2 points, 1.01 km, 37.000000,-121.999887 to 37.009043,-121.999887
coverage 1.0000 (threshold 0.90), max deviation 10.0 m (veto above 300 m) -> used
worst target straying from source            10.0 m at point 0, 0.00 km along, 37.000000,-121.999887
worst source that target skips               10.0 m at point 0, 0.00 km along, 37.000000,-122.000000
every one of the 2 target points lies within the 30 m corridor, so the two describe the same roads
```

The header prints both endpoints so a precision-5 / precision-6 mix-up is obvious before you trust the score. The two "worst" lines are the two directions of the Hausdorff measure: the first catches a detour, the second a route that stops short or only shares a tail. Outside stretches turn a coverage number into somewhere to look on a map.

## How similarity is decided

Two checks, in sequence:

1. **Coverage** — length-weighted share of the *target* that runs within `CorridorMeters` of the source. A length ratio, not a shape distance: one spur would make an identical line look unrelated under Fréchet or Hausdorff alone, and the threshold is a percentage of length.
2. **Max deviation** — the worst vertex-to-segment gap between the two geometries, in both directions, saturating at `DeviationCeilingMeters`. A target can score 90% coverage while the remaining 10% takes a long detour; the veto rejects that regardless of coverage.

The pair is **used** when `coverage >= CoverageThreshold` **and** `max deviation <= MaxDeviationMeters`. A threshold of zero switches the check off and reports every pair as similar.

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

source, _ := similarity.Decode(srcText, similarity.FormatAuto)
target, _ := similarity.Decode(tgtText, similarity.FormatPolyline6)

report, err := similarity.Analyze(source, target)
score, err := similarity.Score(target.Points, source.Points)
```

`Score(target, source)` — coverage is how much of the **target** lies on the **source**. A target that is a subset of the source scores coverage 1.0; the reverse direction of the deviation is what judges a truncated line.

## Origin

Extracted from an EV trip planner's reference-route matcher, where a client-supplied Google route is compared against the route a downstream direction service returned. The algorithm is general: any two polylines will do.

This tool does not include trimming, way-id matching, or the surrounding HTTP service. Those stay in the original product.

---

## 中文

比较两条路线几何，判断 **target** 是否足够贴近 **source**。

算法是「走廊覆盖率 + 最大偏离一票否决」：覆盖率是 target 有多少长度落在 source 两侧 `CorridorMeters` 走廊内；最大偏离是双向 Hausdorff（点到线段，不是点到顶点）。两条同时过线才算 `used`。

### 输入

两个 geometry：文件或直接把字符串当参数。支持 Google polyline 5/6、`{lat,lon;...}`、GeoJSON LineString、WKT LINESTRING、逐行 `lat,lon`。`auto` 能识别结构化格式，**不会**猜测 polyline6 —— 编解码精度弄反会把坐标缩放十倍，看起来像完全不同的路，请显式传 `--target-format polyline6`。

### 输出

和原工程 `reportPair` 同一类诊断：两端点（用来核对精度）、覆盖率、最大偏离、两个方向上最差点、以及 target 离开走廊的若干段。

### 阈值

全部在 `similarity/params.go` 里用全局变量写死，并附了完整注释。改阈值需要重新编译，没有运行时 flag。

```bash
go build -o gsc ./cmd/gsc
./gsc testdata/source.geojson testdata/target-close.geojson
```
