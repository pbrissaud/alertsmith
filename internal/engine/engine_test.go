package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pbrissaud/alertsmith/internal/finding"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

// checkOf returns the single finding with the given check id, failing if it is
// absent or duplicated — the ADR 0002 matrix mints exactly one per failed file.
func checkOf(t *testing.T, rep finding.Report, id string) finding.Finding {
	t.Helper()
	var got []finding.Finding
	for _, f := range rep.Findings {
		if f.Check == id {
			got = append(got, f)
		}
	}
	if len(got) != 1 {
		t.Fatalf("findings with check %q = %d, want 1 (%+v)", id, len(got), rep.Findings)
	}
	return got[0]
}

func TestRun_helmSkippedIsAdvisoryNote(t *testing.T) {
	// A Helm-templated file → a single helm-skipped Finding: note, advisory
	// (never blocks whatever block_on), and NOT a lost-coverage FileError.
	f := write(t, "chart.yaml", "groups:\n  - name: g\n{{- if .V }}\n    rules: []\n{{- end }}\n")

	rep, failures := Run([]string{f})
	if len(failures) != 0 {
		t.Errorf("failures = %v, want none: a skipped Helm file is a finding, not lost coverage", failures)
	}
	fin := checkOf(t, rep, "helm-skipped")
	if fin.Level != finding.Note {
		t.Errorf("level = %v, want note", fin.Level)
	}
	if fin.Enforceable {
		t.Errorf("helm-skipped must be advisory (Enforceable=false) so it can never block")
	}
	if fin.Pos.Line == 0 {
		t.Errorf("finding has no position — unrenderable as an annotation (ADR 0011)")
	}
}

func TestRun_yamlMalformedIsEnforceableError(t *testing.T) {
	// Genuinely broken YAML (no `{{`) → a yaml-malformed Finding: error,
	// enforceable (may block), again not a FileError.
	f := write(t, "broken.yaml", "groups: [unclosed, sequence\n")

	rep, failures := Run([]string{f})
	if len(failures) != 0 {
		t.Errorf("failures = %v, want none: malformed YAML is a finding, not a FileError", failures)
	}
	fin := checkOf(t, rep, "yaml-malformed")
	if fin.Level != finding.Error {
		t.Errorf("level = %v, want error", fin.Level)
	}
	if !fin.Enforceable {
		t.Errorf("yaml-malformed must be enforceable (it is a real defect that may block)")
	}
}

func TestRun_unreadableFileIsLostCoverage(t *testing.T) {
	// A file that cannot be READ is a hard I/O error → FileError (exit-2 path),
	// distinct from a parse failure. It must not be misclassified as a finding.
	missing := filepath.Join(t.TempDir(), "nope.yaml")

	rep, failures := Run([]string{missing})
	if len(failures) != 1 || failures[0].File != missing {
		t.Fatalf("failures = %v, want one FileError for %q", failures, missing)
	}
	if !rep.Empty() {
		t.Errorf("report = %+v, want empty: an unreadable file yields no findings", rep.Findings)
	}
}

func TestRun_cleanDocsBeforePoisonStillChecked(t *testing.T) {
	// File-level granularity keeps the documents decoded before a poison: a first
	// doc with a broken PromQL expr (valid YAML) is still checked and yields a
	// promql-parse finding, alongside the file's helm-skipped skip.
	f := write(t, "mixed.yaml",
		"groups:\n  - name: g\n    rules:\n      - alert: A\n        expr: this is (not valid\n"+
			"---\n{{- if .X }}\nfoo: bar\n{{- end }}\n")

	rep, failures := Run([]string{f})
	if len(failures) != 0 {
		t.Errorf("failures = %v, want none", failures)
	}
	checkOf(t, rep, "promql-parse") // the clean doc before the poison was checked
	checkOf(t, rep, "helm-skipped") // the poison was surfaced, not dropped
}

func TestRun_noSilentDrop(t *testing.T) {
	// The promise #1 invariant (ADR 0002): every file that fails to parse yields
	// a finding — never a silent drop, whatever the failure shape.
	cases := map[string]string{
		"helm":      "groups:\n  - name: g\n{{- if .V }}\n{{- end }}\n",
		"malformed": "groups: [unclosed\n",
	}
	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			f := write(t, name+".yaml", content)
			rep, failures := Run([]string{f})
			if rep.Empty() && len(failures) == 0 {
				t.Fatal("a failed parse produced neither a finding nor a FileError — silent drop")
			}
			if !rep.Empty() && len(failures) != 0 {
				t.Errorf("a parse failure should be a finding, not also a FileError: %v", failures)
			}
		})
	}
}
