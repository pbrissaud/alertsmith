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
//	2  usage or a runtime error (e.g. a file that could not be read)
//
// A file that fails to *parse* no longer exits 2: it is classified into a
// helm-skipped or yaml-malformed Finding (ADR 0002), so it exits 1 like any
// other finding. Only a hard I/O error (unreadable file) is lost coverage → 2.
package main

import (
	"errors"
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
		// -h/--help is a successful request under ContinueOnError, not a usage
		// error: flag.ErrHelp means the help text was already printed.
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	files := fs.Args()
	if len(files) == 0 {
		fs.Usage()
		return 2
	}

	rep, failures := engine.Run(files)

	// Render first, always: an unreadable file costs coverage but must never
	// discard the findings we did compute for the files that parsed.
	report.Text(stdout, rep)
	for _, fe := range failures {
		fmt.Fprintln(stderr, "alertsmith:", fe.Error())
	}

	// Exit-code priority (lost coverage outranks findings):
	//   2  at least one file could not be read (some coverage was lost)
	//   1  the report has findings
	//   0  clean
	switch {
	case len(failures) > 0:
		return 2
	case !rep.Empty():
		return 1
	default:
		return 0
	}
}
