// Package engine wires the pipeline: parse each file into the IR, run every
// registered check, aggregate into a Report.
//
// The parse-failure policy (Helm skip as advisory note, malformed as
// enforceable error, plus the mandatory anti-hang guard — ADR 0002) is a later
// slice. Until then a parse error is surfaced to the caller rather than being
// swallowed: never a silent drop (the worst failure for a coverage tool).
package engine

import (
	"github.com/pbrissaud/alertsmith/internal/check"
	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/parse"
)

// Run parses and checks the given files, returning the aggregate Report.
//
// A ParseError is returned for the first file that fails to parse; callers get
// a non-silent failure. Later slices replace this with the full parse-failure
// matrix (ADR 0002).
func Run(files []string) (finding.Report, error) {
	var report finding.Report
	checks := check.Registered()

	for _, f := range files {
		pr, err := parse.File(f)
		if err != nil {
			return report, &ParseError{File: f, Err: err}
		}
		for _, c := range checks {
			report.Add(c.Check(pr)...)
		}
	}
	return report, nil
}

// ParseError signals that a file could not be parsed. It is a distinct type so
// the parse-failure policy slice can replace this branch without touching
// callers.
type ParseError struct {
	File string
	Err  error
}

func (e *ParseError) Error() string { return e.Err.Error() }
func (e *ParseError) Unwrap() error { return e.Err }
