package parse

import (
	"testing"

	"github.com/pbrissaud/alertsmith/internal/ir"
)

func TestFile_flat(t *testing.T) {
	pr, err := File("testdata/flat.yaml")
	if err != nil {
		t.Fatalf("File: %v", err)
	}
	if pr.Format != ir.FormatFlat {
		t.Errorf("Format = %v, want FormatFlat", pr.Format)
	}
	if got := len(pr.Groups); got != 1 {
		t.Fatalf("groups = %d, want 1", got)
	}
	g := pr.Groups[0]
	if g.Name.Value != "example" {
		t.Errorf("group name = %q, want %q", g.Name.Value, "example")
	}
	if g.Name.Pos.Line != 2 {
		t.Errorf("group name line = %d, want 2", g.Name.Pos.Line)
	}
	if len(g.Rules) != 2 {
		t.Fatalf("rules = %d, want 2", len(g.Rules))
	}

	// Alerting rule with the full shape.
	alert := g.Rules[0]
	if alert.Kind != ir.Alerting {
		t.Errorf("rule[0] kind = %v, want Alerting", alert.Kind)
	}
	if alert.Name.Value != "HighLatency" || alert.Name.Pos.Line != 4 {
		t.Errorf("rule[0] name = %q@L%d, want HighLatency@L4", alert.Name.Value, alert.Name.Pos.Line)
	}
	if alert.Expr.Pos.Line != 5 {
		t.Errorf("rule[0] expr line = %d, want 5", alert.Expr.Pos.Line)
	}
	if !alert.For.Present() || alert.For.Value != "10m" {
		t.Errorf("rule[0] for = %q (present=%v), want 10m/true", alert.For.Value, alert.For.Present())
	}
	if got := alert.Labels["severity"]; got.Value != "warning" || got.Pos.Line != 8 {
		t.Errorf("rule[0] labels.severity = %q@L%d, want warning@L8", got.Value, got.Pos.Line)
	}
	if got := alert.Annotations["summary"]; got.Value != "high latency" || got.Pos.Line != 10 {
		t.Errorf("rule[0] annotations.summary = %q@L%d, want 'high latency'@L10", got.Value, got.Pos.Line)
	}

	// Recording rule: no `for:`, colon-name.
	rec := g.Rules[1]
	if rec.Kind != ir.Recording {
		t.Errorf("rule[1] kind = %v, want Recording", rec.Kind)
	}
	if rec.Name.Value != "job:http_requests:rate5m" || rec.Name.Pos.Line != 11 {
		t.Errorf("rule[1] name = %q@L%d, want job:http_requests:rate5m@L11", rec.Name.Value, rec.Name.Pos.Line)
	}
	if rec.For.Present() {
		t.Errorf("rule[1] for should be absent, got %q", rec.For.Value)
	}
}

func TestBytes_exprAsInt(t *testing.T) {
	// `expr: 0` (int in native flat) must be read as a string leaf without any
	// typed dependency (ADR 0001).
	src := []byte("groups:\n  - name: g\n    rules:\n      - record: up:count\n        expr: 0\n")
	pr, err := Bytes("mem.yaml", src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	got := pr.Groups[0].Rules[0].Expr
	if got.Value != "0" {
		t.Errorf("expr value = %q, want \"0\"", got.Value)
	}
}

func TestBytes_aliasedExpr(t *testing.T) {
	// An anchor holding a BROKEN PromQL expr, referenced via *alias. yaml.v3
	// leaves the alias unresolved when decoding into a yaml.Node, so without
	// resolution the leaf would carry the anchor NAME ("base") — which parses
	// fine as a bare selector and hides the bug. The parser must resolve it so
	// the leaf holds the real (broken) expression text for promql-parse to catch.
	src := []byte("" +
		"anchors:\n" +
		"  - &base rate(http_requests_total[5m)\n" + // missing ']' -> broken PromQL
		"groups:\n" +
		"  - name: g\n" +
		"    rules:\n" +
		"      - alert: A\n" +
		"        expr: *base\n")
	pr, err := Bytes("mem.yaml", src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	expr := pr.Groups[0].Rules[0].Expr
	if expr.Value != "rate(http_requests_total[5m)" {
		t.Errorf("expr value = %q, want the resolved (broken) expression, not the anchor name", expr.Value)
	}
	if !expr.Present() || expr.Pos.Line == 0 {
		t.Errorf("expr should be positioned at its usage site, got line %d", expr.Pos.Line)
	}
}

func TestBytes_labelMergeKey(t *testing.T) {
	// `<<: *common` must be folded in (Prometheus honours YAML merge keys), with
	// explicit keys winning over merged ones — not surfaced as a phantom "<<"
	// label with the merged-in keys lost (ADR 0001).
	src := []byte("" +
		"anchors:\n" +
		"  common: &common\n" +
		"    team: sre\n" +
		"    severity: ticket\n" +
		"groups:\n" +
		"  - name: g\n" +
		"    rules:\n" +
		"      - alert: A\n" +
		"        expr: up == 0\n" +
		"        labels:\n" +
		"          <<: *common\n" +
		"          severity: page\n")
	pr, err := Bytes("mem.yaml", src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	labels := pr.Groups[0].Rules[0].Labels
	if _, ok := labels["<<"]; ok {
		t.Errorf("labels has a phantom \"<<\" key: %+v", labels)
	}
	if team := labels["team"]; team.Value != "sre" {
		t.Errorf("labels.team = %q, want merged-in \"sre\"", team.Value)
	}
	if sev := labels["severity"]; sev.Value != "page" {
		t.Errorf("labels.severity = %q, want explicit \"page\" to win over merged \"ticket\"", sev.Value)
	}
	if labels["team"].Pos.Line == 0 || labels["severity"].Pos.Line == 0 {
		t.Errorf("positions must be non-zero: team@L%d severity@L%d",
			labels["team"].Pos.Line, labels["severity"].Pos.Line)
	}
}

func TestBytes_labelMergeSequence(t *testing.T) {
	// A sequence merge (`<<: [*a, *b]`): earlier-listed source wins over later.
	src := []byte("" +
		"anchors:\n" +
		"  a: &a\n" +
		"    team: sre\n" +
		"  b: &b\n" +
		"    team: infra\n" +
		"    tier: gold\n" +
		"groups:\n" +
		"  - name: g\n" +
		"    rules:\n" +
		"      - alert: A\n" +
		"        expr: up == 0\n" +
		"        labels:\n" +
		"          <<: [*a, *b]\n")
	pr, err := Bytes("mem.yaml", src)
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	labels := pr.Groups[0].Rules[0].Labels
	if _, ok := labels["<<"]; ok {
		t.Errorf("labels has a phantom \"<<\" key: %+v", labels)
	}
	if team := labels["team"]; team.Value != "sre" {
		t.Errorf("labels.team = %q, want \"sre\" (first source wins)", team.Value)
	}
	if tier := labels["tier"]; tier.Value != "gold" {
		t.Errorf("labels.tier = %q, want merged-in \"gold\"", tier.Value)
	}
}
