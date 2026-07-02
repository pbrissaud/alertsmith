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
