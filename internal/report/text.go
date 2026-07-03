// Package report renders a finding.Report for humans.
//
// The walking skeleton ships a plain text renderer for the CLI (dogfood-first,
// build order #1). The GitHub check-run annotation renderer is a later slice
// (ADR 0011).
package report

import (
	"fmt"
	"io"
	"sort"

	"github.com/pbrissaud/alertsmith/internal/finding"
)

// Text writes each finding as a clickable `file:line:col  level  check  message`
// line, sorted by position, followed by a one-line summary.
func Text(w io.Writer, r finding.Report) {
	findings := make([]finding.Finding, len(r.Findings))
	copy(findings, r.Findings)
	sort.SliceStable(findings, func(i, j int) bool {
		a, b := findings[i].Pos, findings[j].Pos
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Col < b.Col
	})

	for _, f := range findings {
		fmt.Fprintf(w, "%s:%d:%d  %s  %s  %s\n",
			f.Pos.File, f.Pos.Line, f.Pos.Col, f.Level, f.Check, f.Message)
	}

	if r.Empty() {
		fmt.Fprintln(w, "no findings")
		return
	}
	plural := "s"
	if len(findings) == 1 {
		plural = ""
	}
	fmt.Fprintf(w, "%d finding%s\n", len(findings), plural)
}
