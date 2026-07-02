// Command alertsmith lints Prometheus alerting and recording rules.
//
// Walking skeleton: it lints native flat PrometheusRule files passed as
// arguments and prints findings as text. The CRD format, .alertsmith.yaml
// config, the block_on verdict and the GitHub Action wrap are later slices.
//
// Exit codes:
//
//	0  no findings
//	1  findings were produced
//	2  usage or a runtime error (e.g. a file that could not be parsed)
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/pbrissaud/alertsmith/internal/engine"
	"github.com/pbrissaud/alertsmith/internal/report"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("alertsmith", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: alertsmith <file.yaml> [file.yaml ...]")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	files := fs.Args()
	if len(files) == 0 {
		fs.Usage()
		return 2
	}

	rep, err := engine.Run(files)
	if err != nil {
		fmt.Fprintln(stderr, "alertsmith:", err)
		return 2
	}

	report.Text(stdout, rep)
	if rep.Empty() {
		return 0
	}
	return 1
}
