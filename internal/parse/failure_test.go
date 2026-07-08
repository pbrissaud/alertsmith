package parse

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// wantFailure asserts the parse stopped with a classified *Failure (not a clean
// parse, not some other error) and returns it.
func wantFailure(t *testing.T, err error) *Failure {
	t.Helper()
	var f *Failure
	if !errors.As(err, &f) {
		t.Fatalf("error = %v (%T), want a *parse.Failure", err, err)
	}
	return f
}

func TestBytes_helmSkippedControlFlow(t *testing.T) {
	// A Helm control-flow template (`{{- if }} … {{- end }}`) is not valid YAML
	// before rendering: parse fails AND the file contains `{{` → helm-skipped,
	// classified by content (ADR 0002). The failing line is the template line.
	src := []byte("groups:\n  - name: g\n{{- if .Values.enabled }}\n    rules: []\n{{- end }}\n")
	prs, err := Bytes("chart.yaml", src)
	f := wantFailure(t, err)
	if !f.Helm {
		t.Errorf("Helm = false, want true (file contains `{{`)")
	}
	if len(prs) != 0 {
		t.Errorf("resources = %d, want 0 (whole file is templated)", len(prs))
	}
	if f.Pos.File != "chart.yaml" || f.Pos.Line != 3 {
		t.Errorf("pos = %s:%d, want chart.yaml:3 (the `{{- if` line)", f.Pos.File, f.Pos.Line)
	}
}

func TestBytes_yamlMalformed(t *testing.T) {
	// Genuinely broken YAML with NO `{{` (an unterminated flow sequence) →
	// yaml-malformed, not a Helm skip (ADR 0002).
	src := []byte("groups: [unclosed, sequence\n")
	prs, err := Bytes("broken.yaml", src)
	f := wantFailure(t, err)
	if f.Helm {
		t.Errorf("Helm = true, want false (no `{{` in the file)")
	}
	if len(prs) != 0 {
		t.Errorf("resources = %d, want 0", len(prs))
	}
	if f.Pos.File != "broken.yaml" || f.Pos.Line < 1 {
		t.Errorf("pos = %s:%d, want a positioned finding in broken.yaml", f.Pos.File, f.Pos.Line)
	}
	if f.Cause == nil || !strings.Contains(f.Error(), "yaml:") {
		t.Errorf("Error() = %q, want it to carry the underlying yaml error", f.Error())
	}
}

func TestBytes_helmClassifiedByContentNotPath(t *testing.T) {
	// Detection is by CONTENT, never by path: a file that fails to parse and
	// carries a Helm chart marker (`.Chart`) anywhere is Helm-templated, even if
	// the break itself is a plain flow error.
	src := []byte("# rendered from {{ .Chart.Name }}\ngroups: [unclosed\n")
	_, err := Bytes("weird.yaml", src)
	f := wantFailure(t, err)
	if !f.Helm {
		t.Errorf("Helm = false, want true (the file carries a Helm chart marker)")
	}
}

func TestBytes_malformedNativeWithAnnotationTemplating(t *testing.T) {
	// The load-bearing distinction (review F1): native Prometheus annotations
	// legitimately use Go-template delimiters (`{{ $value }}`, `{{ $labels.x }}`),
	// so a genuinely-broken native rule file that contains them must NOT be
	// downgraded to an advisory helm-skipped note — it stays an enforceable
	// yaml-malformed error. Helm detection keys on Helm-specific markers, not a
	// bare `{{` scan.
	src := []byte("groups:\n  - name: g\n    rules:\n" +
		"      - alert: A\n        expr: up == 0\n" +
		"        annotations:\n          summary: \"{{ $value }} is high\n") // unterminated string, no Helm markers
	_, err := Bytes("native.yaml", src)
	f := wantFailure(t, err)
	if f.Helm {
		t.Errorf("Helm = true, want false: `{{ $value }}` is Prometheus annotation templating, not Helm — a real error here must stay enforceable")
	}
}

func TestBytes_partialThenPoisonKeepsCleanDocs(t *testing.T) {
	// File-level granularity (ADR 0002): a poisoning document loses the coverage
	// of the CLEAN documents that FOLLOW it, but the clean documents BEFORE it
	// were already decoded and are kept. Here doc1 (clean) survives, doc3 (after
	// the control-flow poison) is unreachable.
	src := []byte("groups:\n  - name: clean\n" +
		"---\n{{- if .X }}\nfoo: bar\n{{- end }}\n" +
		"---\ngroups:\n  - name: after\n")
	prs, err := Bytes("mixed.yaml", src)
	f := wantFailure(t, err)
	if !f.Helm {
		t.Errorf("Helm = false, want true")
	}
	if len(prs) != 1 {
		t.Fatalf("resources = %d, want 1 (the clean doc before the poison)", len(prs))
	}
	if got := prs[0].Groups[0].Name.Value; got != "clean" {
		t.Errorf("kept resource group = %q, want %q; the `after` doc must be lost, not silently substituted", got, "clean")
	}
}

// TestBytes_poisoningTerminates is the explicit non-progression guard test
// (ADR 0002): a poisoning stream makes yaml.v3 return the same error forever
// without advancing; a naive continue-on-error loop reached 5.4 GB before the
// OOM kill. Bytes MUST bail promptly. We run it off-goroutine and fail on any
// hang rather than relying on the package-wide test timeout.
func TestBytes_poisoningTerminates(t *testing.T) {
	poisons := map[string][]byte{
		"control-flow":       []byte("groups:\n  - name: g\n{{- if .V }}\n    rules: []\n{{- end }}\n"),
		"unclosed-flow":      []byte("groups: [unclosed, sequence\n"),
		"poison-after-clean": []byte("a: 1\n---\n{{- range .X }}\nb: 2\n{{- end }}\n---\nc: 3\n"),
	}
	for name, src := range poisons {
		t.Run(name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				_, err := Bytes("poison.yaml", src)
				done <- err
			}()
			select {
			case err := <-done:
				wantFailure(t, err) // terminated with a classified failure, not a hang
			case <-time.After(5 * time.Second):
				t.Fatal("Bytes did not terminate on a poisoning stream — anti-hang guard regressed")
			}
		})
	}
}

func TestErrorLine(t *testing.T) {
	// The yaml.v3 "line N" form is extracted; a message without a line degrades
	// to line 1 (top of file) so a file-level Finding still has a valid anchor.
	tests := []struct {
		in   error
		want int
	}{
		{errors.New("yaml: line 7: could not find expected ':'"), 7},
		{errors.New("yaml: line 1: did not find expected ',' or ']'"), 1},
		{errors.New("yaml: invalid map key: map[string]interface {}{...}"), 1}, // no line → fallback
		{errors.New("some other error"), 1},
	}
	for _, tt := range tests {
		if got := errorLine(tt.in); got != tt.want {
			t.Errorf("errorLine(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
