package parse

import (
	"testing"

	"github.com/pbrissaud/alertsmith/internal/ir"
)

// single asserts the file/bytes parsed to exactly one resource and returns it.
// A file holds 0..n resources (ADR 0015); the flat walking skeleton yields one.
func single(t *testing.T, prs []ir.PrometheusRule, err error) ir.PrometheusRule {
	t.Helper()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("resources = %d, want 1", len(prs))
	}
	return prs[0]
}

// count asserts the file/bytes parsed to exactly n resources and returns them,
// in document order. Used by the multi-document, CRD-list and mixed-kind cases.
func count(t *testing.T, prs []ir.PrometheusRule, err error, n int) []ir.PrometheusRule {
	t.Helper()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(prs) != n {
		t.Fatalf("resources = %d, want %d", len(prs), n)
	}
	return prs
}

func TestFile_flat(t *testing.T) {
	prs, err := File("testdata/flat.yaml")
	pr := single(t, prs, err)
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
	prs, err := Bytes("mem.yaml", src)
	pr := single(t, prs, err)
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
	prs, err := Bytes("mem.yaml", src)
	pr := single(t, prs, err)
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
	prs, err := Bytes("mem.yaml", src)
	pr := single(t, prs, err)
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
	prs, err := Bytes("mem.yaml", src)
	pr := single(t, prs, err)
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

func TestRule_invalidKind(t *testing.T) {
	// A rule block must carry exactly one of alert:/record:. Neither or both is
	// an Invalid rule — never silently minted as Alerting/Recording (ADR 0015).
	t.Run("neither, points at a present leaf", func(t *testing.T) {
		// alert:/record: both absent; only expr/for present (someone dropped the
		// `alert:` key). expr is on line 4.
		src := []byte("groups:\n  - name: g\n    rules:\n      - expr: up == 0\n        for: 5m\n")
		prs, err := Bytes("mem.yaml", src)
		pr := single(t, prs, err)
		r := pr.Groups[0].Rules[0]
		if r.Kind != ir.Invalid {
			t.Errorf("kind = %v, want Invalid", r.Kind)
		}
		if r.Name.Present() {
			t.Errorf("name should be absent, got %q", r.Name.Value)
		}
		if r.Pos.Line != 4 { // firstPos falls back to the first present leaf (expr@L4)
			t.Errorf("pos line = %d, want 4 (the first present leaf)", r.Pos.Line)
		}
	})
	t.Run("both, classified Invalid not Recording", func(t *testing.T) {
		src := []byte("groups:\n  - name: g\n    rules:\n      - alert: A\n        record: r\n        expr: up\n")
		prs, err := Bytes("mem.yaml", src)
		pr := single(t, prs, err)
		r := pr.Groups[0].Rules[0]
		if r.Kind != ir.Invalid {
			t.Errorf("kind = %v, want Invalid (a both-keys rule must not be silently Recording)", r.Kind)
		}
		if r.Pos.Line == 0 {
			t.Errorf("pos should point at a present node, got line 0")
		}
	})
}

func TestBytes_crd(t *testing.T) {
	// A prometheus-operator CRD must normalise to the SAME IR as the flat format,
	// and — the load-bearing invariant (ADR 0011) — every extracted leaf must
	// carry a non-zero position, so a Finding on a CRD rule stays clickable.
	src := []byte(`apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: example-rules
spec:
  groups:
    - name: example
      rules:
        - alert: HighLatency
          expr: rate(http_request_duration_seconds_sum[5m]) > 0.5
          for: 10m
          labels:
            severity: warning
          annotations:
            summary: high latency
`)
	prs, err := Bytes("crd.yaml", src)
	pr := single(t, prs, err)
	if pr.Format != ir.FormatCRD {
		t.Errorf("Format = %v, want FormatCRD", pr.Format)
	}
	if len(pr.Groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(pr.Groups))
	}
	g := pr.Groups[0]
	if g.Name.Value != "example" || g.Name.Pos.Line != 7 {
		t.Errorf("group name = %q@L%d, want example@L7", g.Name.Value, g.Name.Pos.Line)
	}
	if len(g.Rules) != 1 {
		t.Fatalf("rules = %d, want 1", len(g.Rules))
	}
	r := g.Rules[0]
	if r.Kind != ir.Alerting {
		t.Errorf("kind = %v, want Alerting (a valid CRD rule must not be misclassified)", r.Kind)
	}
	if r.Name.Value != "HighLatency" || r.Name.Pos.Line != 9 {
		t.Errorf("name = %q@L%d, want HighLatency@L9", r.Name.Value, r.Name.Pos.Line)
	}
	if r.Expr.Pos.Line != 10 || !r.Expr.Present() {
		t.Errorf("expr @L%d (present=%v), want L10/true", r.Expr.Pos.Line, r.Expr.Present())
	}
	if !r.For.Present() || r.For.Value != "10m" {
		t.Errorf("for = %q (present=%v), want 10m/true", r.For.Value, r.For.Present())
	}
	if sev := r.Labels["severity"]; sev.Value != "warning" || sev.Pos.Line != 13 {
		t.Errorf("labels.severity = %q@L%d, want warning@L13", sev.Value, sev.Pos.Line)
	}
	if sum := r.Annotations["summary"]; sum.Value != "high latency" || sum.Pos.Line != 15 {
		t.Errorf("annotations.summary = %q@L%d, want 'high latency'@L15", sum.Value, sum.Pos.Line)
	}
	// The invariant, stated directly: no present leaf of a valid CRD rule may sit
	// at line 0 (an unpositioned leaf is unrenderable as a check-run annotation).
	for _, f := range []ir.Field{g.Name, r.Name, r.Expr, r.For, r.Labels["severity"], r.Annotations["summary"]} {
		if !f.Present() {
			t.Errorf("leaf %q has no position (Pos.Line == 0)", f.Value)
		}
	}
}

func TestBytes_crdExprAsInt(t *testing.T) {
	// `expr: 0` in a CRD is the operator's intstr.IntOrString int case; reading it
	// as a yaml.Node leaf yields "0" with a position, no monitoringv1 dependency.
	src := []byte(`apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
spec:
  groups:
    - name: g
      rules:
        - record: up:count
          expr: 0
`)
	prs, err := Bytes("crd.yaml", src)
	pr := single(t, prs, err)
	got := pr.Groups[0].Rules[0].Expr
	if got.Value != "0" {
		t.Errorf("expr value = %q, want \"0\"", got.Value)
	}
	if !got.Present() {
		t.Errorf("expr must carry a position even as an int leaf")
	}
}

func TestBytes_streamLinesAbsolute(t *testing.T) {
	// Positions must be absolute in the file, not reset per `---` document, so a
	// Finding on the second document still points at the right file:line.
	src := []byte(`groups:
  - name: first
    rules:
      - alert: A
        expr: up == 0
---
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
spec:
  groups:
    - name: second
      rules:
        - alert: B
          expr: down == 1
`)
	prs, err := Bytes("stream.yaml", src)
	got := count(t, prs, err, 2)
	if got[0].Format != ir.FormatFlat || got[1].Format != ir.FormatCRD {
		t.Fatalf("formats = [%v %v], want [FormatFlat FormatCRD]", got[0].Format, got[1].Format)
	}
	g := got[1].Groups[0]
	if g.Name.Value != "second" || g.Name.Pos.Line != 11 {
		t.Errorf("2nd doc group = %q@L%d, want second@L11 (absolute, not reset per doc)", g.Name.Value, g.Name.Pos.Line)
	}
	if expr := g.Rules[0].Expr; expr.Pos.Line != 14 {
		t.Errorf("2nd doc expr @L%d, want L14 (absolute line in the file)", expr.Pos.Line)
	}
}

func TestBytes_multiDocMix(t *testing.T) {
	// A file mixing a CRD, an unrelated K8s kind and a flat doc: the non-
	// PrometheusRule is ignored, the rules from the other two are extracted.
	src := []byte(`apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
spec:
  groups:
    - name: crd-group
      rules:
        - alert: A
          expr: up == 0
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: not-a-rule
data:
  foo: bar
---
groups:
  - name: flat-group
    rules:
      - alert: B
        expr: down == 1
`)
	prs, err := Bytes("mix.yaml", src)
	got := count(t, prs, err, 2)
	if got[0].Format != ir.FormatCRD || got[1].Format != ir.FormatFlat {
		t.Fatalf("formats = [%v %v], want [FormatCRD FormatFlat] (ConfigMap dropped)", got[0].Format, got[1].Format)
	}
	if got[0].Groups[0].Name.Value != "crd-group" {
		t.Errorf("resource[0] group = %q, want crd-group", got[0].Groups[0].Name.Value)
	}
	if got[1].Groups[0].Name.Value != "flat-group" {
		t.Errorf("resource[1] group = %q, want flat-group", got[1].Groups[0].Name.Value)
	}
}

func TestBytes_prometheusRuleList(t *testing.T) {
	// A PrometheusRuleList must unfold into one resource per item — not doing so
	// silently misses every rule in a List-shaped file (ADR 0015).
	src := []byte(`apiVersion: monitoring.coreos.com/v1
kind: PrometheusRuleList
items:
  - apiVersion: monitoring.coreos.com/v1
    kind: PrometheusRule
    metadata:
      name: one
    spec:
      groups:
        - name: g1
          rules:
            - alert: A
              expr: up == 0
  - apiVersion: monitoring.coreos.com/v1
    kind: PrometheusRule
    metadata:
      name: two
    spec:
      groups:
        - name: g2
          rules:
            - record: r
              expr: sum(up)
`)
	prs, err := Bytes("list.yaml", src)
	got := count(t, prs, err, 2)
	for i, pr := range got {
		if pr.Format != ir.FormatCRD {
			t.Errorf("item[%d] Format = %v, want FormatCRD", i, pr.Format)
		}
	}
	if got[0].Groups[0].Name.Value != "g1" || got[1].Groups[0].Name.Value != "g2" {
		t.Errorf("group names = [%q %q], want [g1 g2]", got[0].Groups[0].Name.Value, got[1].Groups[0].Name.Value)
	}
	// The position invariant must survive the unfold: an item's leaves stay positioned.
	if r := got[0].Groups[0].Rules[0]; !r.Name.Present() || !r.Expr.Present() {
		t.Errorf("item[0] rule leaves lost position: name.present=%v expr.present=%v", r.Name.Present(), r.Expr.Present())
	}
}

func TestBytes_emptyDocsSkipped(t *testing.T) {
	// Leading/trailing `---`, an empty gap and a comment-only document are padding
	// (common in Helm output); none must become a phantom empty resource.
	src := []byte(`---
# just a comment, nothing to parse
---
groups:
  - name: g
    rules:
      - alert: A
        expr: up == 0
---
---
`)
	prs, err := Bytes("pad.yaml", src)
	pr := single(t, prs, err)
	if pr.Groups[0].Name.Value != "g" {
		t.Errorf("group = %q, want g", pr.Groups[0].Name.Value)
	}
}

func TestBytes_otherKindOnly(t *testing.T) {
	// A file with no PrometheusRule at all yields zero resources and no error —
	// it is simply out of scope, not a parse failure.
	src := []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
spec:
  replicas: 3
`)
	prs, err := Bytes("dep.yaml", src)
	count(t, prs, err, 0)
}

func TestBytes_apiVersionDetection(t *testing.T) {
	// Detection guards the API group, not a pinned version. A monitoring.coreos.com
	// PrometheusRule of any version (or with a missing apiVersion) is extracted; a
	// foreign group borrowing the kind name is skipped.
	crd := func(apiVersion string) []byte {
		head := ""
		if apiVersion != "" {
			head = "apiVersion: " + apiVersion + "\n"
		}
		return []byte(head + `kind: PrometheusRule
spec:
  groups:
    - name: g
      rules:
        - alert: A
          expr: up == 0
`)
	}
	t.Run("v1beta1 still extracted (version not pinned)", func(t *testing.T) {
		prs, err := Bytes("c.yaml", crd("monitoring.coreos.com/v1beta1"))
		pr := single(t, prs, err)
		if pr.Format != ir.FormatCRD {
			t.Errorf("Format = %v, want FormatCRD", pr.Format)
		}
	})
	t.Run("apiVersion absent still extracted", func(t *testing.T) {
		prs, err := Bytes("c.yaml", crd(""))
		pr := single(t, prs, err)
		if pr.Format != ir.FormatCRD {
			t.Errorf("Format = %v, want FormatCRD", pr.Format)
		}
	})
	t.Run("foreign group skipped", func(t *testing.T) {
		prs, err := Bytes("c.yaml", crd("example.com/v1"))
		count(t, prs, err, 0)
	})
}

func TestBytes_crdLabelMergeKey(t *testing.T) {
	// The flat path's merge-key folding must work identically inside the CRD
	// envelope: `<<: *common` folded in, the explicit key wins, positions non-zero.
	src := []byte(`apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
spec:
  groups:
    - name: g
      rules:
        - alert: A
          expr: up == 0
          labels: &common
            team: sre
            severity: ticket
        - alert: B
          expr: up == 1
          labels:
            <<: *common
            severity: page
`)
	prs, err := Bytes("crd.yaml", src)
	pr := single(t, prs, err)
	labels := pr.Groups[0].Rules[1].Labels
	if _, ok := labels["<<"]; ok {
		t.Errorf("phantom \"<<\" label: %+v", labels)
	}
	if labels["team"].Value != "sre" {
		t.Errorf("labels.team = %q, want merged-in \"sre\"", labels["team"].Value)
	}
	if labels["severity"].Value != "page" {
		t.Errorf("labels.severity = %q, want explicit \"page\" over merged \"ticket\"", labels["severity"].Value)
	}
	if labels["team"].Pos.Line == 0 || labels["severity"].Pos.Line == 0 {
		t.Errorf("label positions must be non-zero: team@L%d severity@L%d",
			labels["team"].Pos.Line, labels["severity"].Pos.Line)
	}
}

func TestBytes_twoFlatDocs(t *testing.T) {
	// Two native flat documents in one file are two resources (ADR 0015).
	src := []byte(`groups:
  - name: g1
    rules:
      - alert: A
        expr: up == 0
---
groups:
  - name: g2
    rules:
      - alert: B
        expr: down == 1
`)
	prs, err := Bytes("two.yaml", src)
	got := count(t, prs, err, 2)
	if got[0].Format != ir.FormatFlat || got[1].Format != ir.FormatFlat {
		t.Errorf("formats = [%v %v], want both FormatFlat", got[0].Format, got[1].Format)
	}
	if got[0].Groups[0].Name.Value != "g1" || got[1].Groups[0].Name.Value != "g2" {
		t.Errorf("groups = [%q %q], want [g1 g2]", got[0].Groups[0].Name.Value, got[1].Groups[0].Name.Value)
	}
}
