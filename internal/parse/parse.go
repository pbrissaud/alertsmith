// Package parse turns a native flat Prometheus rule file into the home IR.
//
// Parsing is a single pass into typed structs whose leaf fields are yaml.Node
// (the rulefmt pattern): this yields field-level positions without a generic
// tree walk (ADR 0001). The yaml.Node values are then *reconstructed* into
// ir.Field values; rulefmt/promql are handed those reconstructed values as
// validators and never sit on the unmarshal path.
//
// The walking skeleton handles the native flat format only, single document.
// The CRD envelope, multi-document YAML and the parse-failure policy (Helm skip
// / malformed / anti-hang guard) arrive in later slices (ADR 0002).
package parse

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/pbrissaud/alertsmith/internal/ir"
)

// wire structs mirror the native flat format; every leaf is a yaml.Node so the
// decoder records its line/column for us.
type flatDoc struct {
	Groups []groupNode `yaml:"groups"`
}

type groupNode struct {
	Name  yaml.Node  `yaml:"name"`
	Rules []ruleNode `yaml:"rules"`
}

type ruleNode struct {
	Alert       yaml.Node `yaml:"alert"`
	Record      yaml.Node `yaml:"record"`
	Expr        yaml.Node `yaml:"expr"`
	For         yaml.Node `yaml:"for"`
	Labels      yaml.Node `yaml:"labels"`
	Annotations yaml.Node `yaml:"annotations"`
}

// File reads and parses a native flat Prometheus rule file.
func File(path string) (*ir.PrometheusRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Bytes(path, data)
}

// Bytes parses native flat rule content already in memory. The file name is
// only used to stamp provenance.
func Bytes(file string, data []byte) (*ir.PrometheusRule, error) {
	var doc flatDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}

	pr := &ir.PrometheusRule{File: file, Format: ir.FormatFlat}
	for _, g := range doc.Groups {
		group := ir.Group{
			Name: field(file, &g.Name),
			Pos:  pos(file, &g.Name),
		}
		for _, r := range g.Rules {
			group.Rules = append(group.Rules, rule(file, &r))
		}
		pr.Groups = append(pr.Groups, group)
	}
	return pr, nil
}

func rule(file string, r *ruleNode) ir.Rule {
	out := ir.Rule{
		Expr:        field(file, &r.Expr),
		For:         field(file, &r.For),
		Labels:      mapping(file, &r.Labels),
		Annotations: mapping(file, &r.Annotations),
	}
	// A rule is recording iff it carries `record:`, otherwise alerting; the
	// name and the rule's own position follow whichever key is present.
	switch {
	case present(&r.Record):
		out.Kind = ir.Recording
		out.Name = field(file, &r.Record)
		out.Pos = pos(file, &r.Record)
	default:
		out.Kind = ir.Alerting
		out.Name = field(file, &r.Alert)
		out.Pos = pos(file, &r.Alert)
	}
	return out
}

// mapping reconstructs a labels/annotations block into value+position fields.
func mapping(file string, n *yaml.Node) map[string]ir.Field {
	if n.Kind != yaml.MappingNode {
		return nil
	}
	out := make(map[string]ir.Field, len(n.Content)/2)
	for i := 0; i+1 < len(n.Content); i += 2 {
		key := n.Content[i]
		val := n.Content[i+1]
		out[key.Value] = field(file, val)
	}
	return out
}

// field reconstructs a scalar leaf into an ir.Field. An absent key decodes to a
// zero yaml.Node, which yields a zero (non-Present) Field.
func field(file string, n *yaml.Node) ir.Field {
	if !present(n) {
		return ir.Field{}
	}
	return ir.Field{Value: n.Value, Pos: pos(file, n)}
}

func pos(file string, n *yaml.Node) ir.Position {
	return ir.Position{File: file, Line: n.Line, Col: n.Column}
}

// present reports whether a yaml.Node was actually decoded from the source (an
// absent field stays at the zero Kind).
func present(n *yaml.Node) bool { return n.Kind != 0 }
