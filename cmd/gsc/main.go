package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

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
		data        = fs.String("data", "", "JSON file: array of {source, target}")
		jsonOut     = fs.Bool("json", false, "print the report as JSON")
		showVersion = fs.Bool("version", false, "print version and exit")
	)

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

	if *data == "" || fs.NArg() != 0 {
		fs.Usage()
		return 2
	}

	pairs, err := similarity.LoadFile(*data)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}

	reports := make([]similarity.Report, 0, len(pairs))
	for i, pair := range pairs {
		report, err := similarity.Analyze(pair.Source, pair.Target)
		if err != nil {
			fmt.Fprintf(stderr, "pair %d: %v\n", i+1, err)
			return 2
		}
		reports = append(reports, report)
	}

	if *jsonOut {
		payload, err := json.MarshalIndent(reports, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "json: %v\n", err)
			return 2
		}
		fmt.Fprintln(stdout, string(payload))
	} else {
		for i, report := range reports {
			if i > 0 {
				fmt.Fprintln(stdout)
			}
			fmt.Fprintf(stdout, "pair\t%d/%d\n", i+1, len(reports))
			if err := report.Write(stdout); err != nil {
				fmt.Fprintf(stderr, "write: %v\n", err)
				return 2
			}
		}
	}

	for _, report := range reports {
		if !report.Similar {
			return 1
		}
	}
	return 0
}
