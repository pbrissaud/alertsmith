// Package engine wires the pipeline: parse each file into the IR, run every
// registered check, aggregate into a Report.
//
// The full parse-failure policy (Helm skip as advisory note, malformed as
// enforceable error, plus the mandatory anti-hang guard — ADR 0002) is a later
// slice. Until then every file is still processed: a file that fails to parse is
// recorded as a FileError and returned alongside the Report, never swallowed.
// Findings from the files that did parse are always kept — no silent drop (the
// worst failure for a coverage tool), and the output no longer depends on the
// order files are passed in.
package engine

import (
	"github.com/pbrissaud/alertsmith/internal/check"
	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/parse"
)

// Run parses and checks the given files, returning the aggregate Report plus the
// per-file parse failures.
//
// Every file is processed: a file that fails to parse is recorded as a
// FileError and the loop continues, so findings from the parseable files are
// always kept. The caller decides the exit policy from the (possibly empty)
// failures slice. Later slices replace this flat slice with the full
// parse-failure matrix (ADR 0002).
func Run(files []string) (finding.Report, []FileError) {
	var report finding.Report
	var failures []FileError
	checks := check.Registered()

	for _, f := range files {
		// A file yields zero or more resources (ADR 0015); each is checked
		// independently.
		prs, err := parse.File(f)
		if err != nil {
			failures = append(failures, FileError{File: f, Err: err})
			continue
		}
		for i := range prs {
			for _, c := range checks {
				report.Add(c.Check(&prs[i])...)
			}
		}
	}
	return report, failures
}

// FileError records that a single file could not be parsed (or read). The run
// continues past it; the aggregate Report is returned alongside these failures
// so lost coverage is surfaced without discarding what did parse.
type FileError struct {
	File string
	Err  error
}

func (e FileError) Error() string { return e.Err.Error() }
func (e FileError) Unwrap() error { return e.Err }
