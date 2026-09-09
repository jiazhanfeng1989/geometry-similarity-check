package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/zhfjia/geometry-similarity-check/similarity"
)

const version = "0.1.0"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("gsc", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		sourceFormat = fs.String("source-format", "auto", "source encoding: auto, polyline5, polyline6, pointarray, geojson, wkt, latlon")
		targetFormat = fs.String("target-format", "auto", "target encoding (same values as -source-format)")
		bothFormat   = fs.String("format", "", "set source and target format together")
		jsonOut      = fs.Bool("json", false, "print the report as JSON")
		showVersion  = fs.Bool("version", false, "print version and exit")
	)

	fs.Usage = func() {
		fmt.Fprintf(stderr, `gsc %s — compare how closely a target geometry follows a source geometry.

Usage:
  gsc [flags] <source> <target>

Each argument is a file path, a geometry string, or "-" for stdin (only one side).
If the argument names an existing file, the file is read; otherwise it is the geometry.

Flags:
`, version)
		fs.PrintDefaults()
		fmt.Fprintf(stderr, `
Verdict: "used" when coverage >= CoverageThreshold and max deviation <= MaxDeviationMeters.
Those thresholds live in similarity/params.go and require a rebuild to change.

Examples:
  gsc testdata/source.geojson testdata/target-close.geojson
  gsc --target-format polyline6 source.poly target.poly6
  gsc --json '{lat,lon;...}' '{lat,lon;...}'
`)
	}

	args = hoistFlags(args)
	if err := fs.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}

	if *showVersion {
		fmt.Fprintln(stdout, version)
		return 0
	}

	if fs.NArg() != 2 {
		fs.Usage()
		return 2
	}
	if fs.Arg(0) == "-" && fs.Arg(1) == "-" {
		fmt.Fprintln(stderr, "only one of source or target can be stdin")
		return 2
	}

	srcFmt := similarity.Format(*sourceFormat)
	tgtFmt := similarity.Format(*targetFormat)
	if *bothFormat != "" {
		srcFmt = similarity.Format(*bothFormat)
		tgtFmt = similarity.Format(*bothFormat)
	}

	source, err := loadArg(fs.Arg(0), srcFmt)
	if err != nil {
		fmt.Fprintf(stderr, "source: %v\n", err)
		return 2
	}
	target, err := loadArg(fs.Arg(1), tgtFmt)
	if err != nil {
		fmt.Fprintf(stderr, "target: %v\n", err)
		return 2
	}

	report, err := similarity.Analyze(source, target)
	if err != nil {
		fmt.Fprintf(stderr, "score: %v\n", err)
		return 2
	}

	if *jsonOut {
		payload, err := report.JSON()
		if err != nil {
			fmt.Fprintf(stderr, "json: %v\n", err)
			return 2
		}
		fmt.Fprintln(stdout, string(payload))
	} else if err := report.Write(stdout); err != nil {
		fmt.Fprintf(stderr, "write: %v\n", err)
		return 2
	}

	if report.Similar {
		return 0
	}
	return 1
}

// hoistFlags moves flags in front of positional arguments so `gsc a.poly b.poly -json`
// works with the standard library flag parser, which otherwise stops at the first operand.
func hoistFlags(args []string) []string {
	var flags, pos []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			pos = append(pos, args[i+1:]...)
			break
		}
		if arg == "-" || !strings.HasPrefix(arg, "-") {
			pos = append(pos, arg)
			continue
		}

		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if strings.Contains(name, "=") {
			continue
		}
		switch name {
		case "json", "version", "h", "help":
			continue
		}
		if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, pos...)
}

func loadArg(input string, format similarity.Format) (similarity.Geometry, error) {
	if input == "-" {
		buf, err := io.ReadAll(os.Stdin)
		if err != nil {
			return similarity.Geometry{}, fmt.Errorf("reading stdin: %w", err)
		}
		return similarity.Decode(string(buf), format)
	}
	return similarity.Load(input, format)
}
