package check

import (
	"github.com/prometheus/prometheus/promql/parser"

	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/ir"
)

// PromQLParse checks that every rule's `expr` parses as PromQL.
//
// It links prometheus/promql/parser directly and uses it purely as a validator
// on the reconstructed expr value — no shell-out to promtool (ADR 0009), and
// the parser never sits on the unmarshal path (ADR 0001).
type PromQLParse struct{}

// ID is the frozen check id (checks-v1 registry).
func (PromQLParse) ID() string { return "promql-parse" }

func (c PromQLParse) Check(pr *ir.PrometheusRule) []finding.Finding {
	// Default Options mirror standard Prometheus (experimental features off,
	// like promtool's default). The parser is safe to reuse across rules.
	p := parser.NewParser(parser.Options{})
	var out []finding.Finding
	for _, g := range pr.Groups {
		for _, r := range g.Rules {
			if !r.Expr.Present() {
				// A missing expr is a structural problem owned by rule-structure
				// (issue #6), not a parse error — nothing to parse here.
				continue
			}
			if _, err := p.ParseExpr(r.Expr.Value); err != nil {
				out = append(out, finding.Finding{
					Check:       c.ID(),
					Level:       finding.Error,
					Origin:      finding.Deterministic,
					Enforceable: true,
					Message:     "PromQL expression does not parse: " + err.Error(),
					Pos:         r.Expr.Pos,
				})
			}
		}
	}
	return out
}
