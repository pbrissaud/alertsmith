// Package engine wires the pipeline: parse each file into the IR, run every
// registered check, aggregate into a Report.
//
// The parse-failure matrix (ADR 0002) lands here: a file that does not parse is
// classified by content and becomes a Finding — helm-skipped (note, advisory)
// for a Helm template, yaml-malformed (error, enforceable) otherwise — never a
// silent drop, the worst failure for a coverage tool. Only a hard I/O error (an
// unreadable file) stays a FileError. Documents decoded before a mid-stream
// failure are still checked, and the output does not depend on file order.
package engine

import (
	"errors"

	"github.com/pbrissaud/alertsmith/internal/check"
	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/parse"
)

// Run parses and checks the given files, returning the aggregate Report plus the
// per-file I/O failures.
//
// Every file is processed. A parse failure is recovered as a classified
// parse.Failure and minted into a Finding (ADR 0002); only an unreadable file
// becomes a FileError. Either way the resources that did decode — including any
// before a mid-stream poison — are checked, so findings are never dropped. The
// caller decides the exit policy from the (possibly empty) failures slice.
func Run(files []string) (finding.Report, []FileError) {
	var report finding.Report
	var failures []FileError
	checks := check.Registered()

	for _, f := range files {
		// A file yields zero or more resources (ADR 0015); each is checked
		// independently. A parse failure may still return the resources decoded
		// before it (file-level granularity — ADR 0002), so classify the error
		// first, then always check whatever parsed.
		prs, err := parse.File(f)
		var fail *parse.Failure
		switch {
		case errors.As(err, &fail):
			// A classified parse failure is a Finding, not lost coverage.
			report.Add(check.ParseFailureFinding(fail.Helm, fail.Pos, fail.Cause.Error()))
		case err != nil:
			// A hard I/O error (unreadable file): coverage genuinely lost.
			failures = append(failures, FileError{File: f, Err: err})
		}
		for i := range prs {
			for _, c := range checks {
				report.Add(c.Check(&prs[i])...)
			}
		}
	}
	return report, failures
}

// FileError records that a single file could not be read. A parse failure is no
// longer a FileError — it is classified into a Finding (ADR 0002); only a hard
// I/O error reaches here. The run continues past it; the aggregate Report is
// returned alongside these failures so lost coverage is surfaced without
// discarding what did parse.
type FileError struct {
	File string
	Err  error
}

func (e FileError) Error() string { return e.Err.Error() }
func (e FileError) Unwrap() error { return e.Err }
