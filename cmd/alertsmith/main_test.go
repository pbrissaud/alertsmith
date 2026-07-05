package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	return p
}

func TestRun_exitCodes(t *testing.T) {
	valid := write(t, "ok.yaml",
		"groups:\n  - name: g\n    rules:\n      - alert: Ok\n        expr: up == 0\n")
	bad := write(t, "bad.yaml",
		"groups:\n  - name: g\n    rules:\n      - alert: Broken\n        expr: this is (not valid\n")

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantOut  string
	}{
		// A file that parses cleanly with no findings is the only exit-0 path;
		// a parse/read failure (missing file) still counts as lost coverage → 2.
		{"valid file exits 0", []string{valid}, 0, "no findings"},
		{"findings exit 1", []string{bad}, 1, "promql-parse"},
		{"unreadable file exits 2", []string{filepath.Join(t.TempDir(), "nope.yaml")}, 2, ""},
		{"no args exits 2", nil, 2, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := run(tt.args, &out, &errb)
			if code != tt.wantCode {
				t.Errorf("exit = %d, want %d (stderr: %s)", code, tt.wantCode, errb.String())
			}
			if tt.wantOut != "" && !strings.Contains(out.String(), tt.wantOut) {
				t.Errorf("stdout = %q, want it to contain %q", out.String(), tt.wantOut)
			}
		})
	}
}

// A parse failure must not swallow the findings computed for the files that did
// parse. Under ADR 0002 a malformed file (no `{{`) is now a yaml-malformed
// Finding, not lost coverage: both files' findings land in stdout and the run
// exits 1 (findings), never 2 (which is reserved for unreadable files).
func TestRun_multiFileKeepsFindingsDespiteParseError(t *testing.T) {
	// First file parses fine and yields a real promql-parse finding; the second
	// is malformed YAML (an unterminated flow sequence, no `{{`).
	broken := write(t, "broken.yaml",
		"groups:\n  - name: g\n    rules:\n      - alert: Broken\n        expr: this is (not valid\n")
	malformed := write(t, "malformed.yaml", "groups: [unclosed\n")

	var out, errb bytes.Buffer
	code := run([]string{broken, malformed}, &out, &errb)

	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "promql-parse") {
		t.Errorf("stdout = %q, want it to keep the first file's promql-parse finding", out.String())
	}
	if !strings.Contains(out.String(), "yaml-malformed") {
		t.Errorf("stdout = %q, want the malformed file surfaced as a yaml-malformed finding", out.String())
	}
	if errb.String() != "" {
		t.Errorf("stderr = %q, want empty: a parse failure is a finding, not a FileError", errb.String())
	}
}

// A Helm-templated file (control-flow `{{- if }}`) is skipped as an advisory
// helm-skipped note. It is a finding to show (exit 1) but never lost coverage
// (exit 2): the file was recognised, not dropped (ADR 0002).
func TestRun_helmTemplatedSkipped(t *testing.T) {
	helm := write(t, "chart.yaml",
		"groups:\n  - name: g\n{{- if .Values.enabled }}\n    rules: []\n{{- end }}\n")

	var out, errb bytes.Buffer
	code := run([]string{helm}, &out, &errb)

	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr: %s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "helm-skipped") {
		t.Errorf("stdout = %q, want a helm-skipped finding", out.String())
	}
	if !strings.Contains(out.String(), "note") {
		t.Errorf("stdout = %q, want the helm-skipped finding at level note", out.String())
	}
	if errb.String() != "" {
		t.Errorf("stderr = %q, want empty: a skipped Helm file is a finding, not a FileError", errb.String())
	}
}

// -h/--help is a successful request and must exit 0, not 2 (Bug 2), so CI does
// not read a help invocation as a failure.
func TestRun_helpExitsZero(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"--help"}, &out, &errb); code != 0 {
		t.Errorf("exit = %d, want 0 for --help (stderr: %s)", code, errb.String())
	}
}
