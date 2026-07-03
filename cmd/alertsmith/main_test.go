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

// A malformed file must not swallow the findings computed for the files that
// did parse, and a parse failure must still be reported and exit 2 (Bug 1).
func TestRun_multiFileKeepsFindingsDespiteParseError(t *testing.T) {
	// First file parses fine and yields a real promql-parse finding; the second
	// is malformed YAML (an unterminated flow sequence). Under the old
	// abort-on-first-error behaviour the first file's finding was discarded.
	broken := write(t, "broken.yaml",
		"groups:\n  - name: g\n    rules:\n      - alert: Broken\n        expr: this is (not valid\n")
	malformed := write(t, "malformed.yaml", "groups: [unclosed\n")

	var out, errb bytes.Buffer
	code := run([]string{broken, malformed}, &out, &errb)

	if code != 2 {
		t.Fatalf("exit = %d, want 2 (stderr: %s)", code, errb.String())
	}
	if !strings.Contains(out.String(), "promql-parse") {
		t.Errorf("stdout = %q, want it to keep the first file's promql-parse finding", out.String())
	}
	if !strings.Contains(errb.String(), "malformed.yaml") {
		t.Errorf("stderr = %q, want it to mention the malformed file", errb.String())
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
