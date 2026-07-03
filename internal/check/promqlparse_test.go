package check

import (
	"strings"
	"testing"

	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/parse"
)

func run(t *testing.T, src string) []finding.Finding {
	t.Helper()
	prs, err := parse.Bytes("mem.yaml", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var out []finding.Finding
	for i := range prs {
		out = append(out, PromQLParse{}.Check(&prs[i])...)
	}
	return out
}

func TestPromQLParse_flagsBadExpr(t *testing.T) {
	// expr on line 5.
	src := "groups:\n  - name: g\n    rules:\n      - alert: Broken\n        expr: this is (not valid promql\n"
	got := run(t, src)
	if len(got) != 1 {
		t.Fatalf("findings = %d, want 1", len(got))
	}
	f := got[0]
	if f.Check != "promql-parse" {
		t.Errorf("check = %q, want promql-parse", f.Check)
	}
	if f.Level != finding.Error {
		t.Errorf("level = %v, want error", f.Level)
	}
	if !f.Enforceable || f.Origin != finding.Deterministic {
		t.Errorf("enforceable/origin = %v/%v, want true/deterministic", f.Enforceable, f.Origin)
	}
	if f.Pos.Line != 5 {
		t.Errorf("line = %d, want 5 (the expr leaf)", f.Pos.Line)
	}
	if !strings.Contains(f.Message, "does not parse") {
		t.Errorf("message = %q, want it to mention a parse failure", f.Message)
	}
}

func TestPromQLParse_acceptsValidExpr(t *testing.T) {
	src := "groups:\n  - name: g\n    rules:\n      - alert: Ok\n        expr: up == 0\n"
	if got := run(t, src); len(got) != 0 {
		t.Fatalf("findings = %d, want 0: %+v", len(got), got)
	}
}

func TestPromQLParse_ignoresMissingExpr(t *testing.T) {
	// No expr at all: nothing to parse here (rule-structure owns this, #6).
	src := "groups:\n  - name: g\n    rules:\n      - alert: NoExpr\n"
	if got := run(t, src); len(got) != 0 {
		t.Fatalf("findings = %d, want 0", len(got))
	}
}
