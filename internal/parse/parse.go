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

// mergeTag is the tag yaml.v3 resolves a `<<` merge key to. It is not exported
// by the library, so we match on the tag string it documents.
const mergeTag = "!!merge"

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

// File reads and parses a Prometheus rule file into its resources.
//
// A file holds zero or more PrometheusRule resources (a resource is not a file
// — ADR 0015): one native flat document today, and with #2 several `---`
// documents, a CRD, or a mix. The walking skeleton parses a single flat
// document, so the slice has one element; #2 makes it additive.
func File(path string) ([]ir.PrometheusRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Bytes(path, data)
}

// Bytes parses rule content already in memory into its resources. The file name
// is only used to stamp provenance.
func Bytes(file string, data []byte) ([]ir.PrometheusRule, error) {
	var doc flatDoc
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", file, err)
	}

	pr := ir.PrometheusRule{File: file, Format: ir.FormatFlat}
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
	return []ir.PrometheusRule{pr}, nil
}

func rule(file string, r *ruleNode) ir.Rule {
	out := ir.Rule{
		Expr:        field(file, &r.Expr),
		For:         field(file, &r.For),
		Labels:      mapping(file, &r.Labels),
		Annotations: mapping(file, &r.Annotations),
	}
	// A valid rule carries exactly one of `alert:`/`record:`. Neither or both is
	// an Invalid rule — we do NOT guess a flavour (that would mislabel a broken
	// rule as alerting and trigger a cascade of phantom #7 findings). The
	// rule-structure check (#6) spells out which degenerate case it is.
	hasAlert, hasRecord := present(&r.Alert), present(&r.Record)
	switch {
	case hasRecord && !hasAlert:
		out.Kind = ir.Recording
		out.Name = field(file, &r.Record)
		out.Pos = pos(file, &r.Record)
	case hasAlert && !hasRecord:
		out.Kind = ir.Alerting
		out.Name = field(file, &r.Alert)
		out.Pos = pos(file, &r.Alert)
	default:
		out.Kind = ir.Invalid
		// Name follows whichever key exists (the both-case); empty when neither.
		if hasAlert {
			out.Name = field(file, &r.Alert)
		} else if hasRecord {
			out.Name = field(file, &r.Record)
		}
		// Point at any present leaf so a rule-level Finding is never stranded at
		// line 0 (except a wholly empty `- {}` block, which has nothing to point at).
		out.Pos = firstPos(file, &r.Alert, &r.Record, &r.Expr, &r.For, &r.Labels, &r.Annotations)
	}
	return out
}

// firstPos returns the position of the first present node, so a Finding on a
// rule with no name node can still point at something real.
func firstPos(file string, nodes ...*yaml.Node) ir.Position {
	for _, n := range nodes {
		if present(n) {
			return pos(file, n)
		}
	}
	return ir.Position{File: file}
}

// mapping reconstructs a labels/annotations block into value+position fields,
// folding in YAML merge keys (`<<`). Prometheus honours merge keys when loading
// rules, so we must too: otherwise the IR would carry a phantom label literally
// named "<<" and silently drop the merged-in keys (ADR 0001 — we validate what
// Prometheus actually sees).
func mapping(file string, n *yaml.Node) map[string]ir.Field {
	m := resolve(n)
	if m.Kind != yaml.MappingNode {
		return nil
	}
	out := make(map[string]ir.Field, len(m.Content)/2)
	// Explicit keys first so they win over anything merged in (YAML semantics).
	var merges []*yaml.Node
	for i := 0; i+1 < len(m.Content); i += 2 {
		key, val := m.Content[i], m.Content[i+1]
		if key.Tag == mergeTag {
			merges = append(merges, val)
			continue
		}
		out[key.Value] = field(file, val)
	}
	// Then fold in merged mappings. Earlier-listed sources win over later ones,
	// and none override an explicit key — so we only add keys not already set.
	for _, src := range merges {
		for _, ms := range mergeSources(src) {
			if ms.Kind != yaml.MappingNode {
				continue
			}
			for i := 0; i+1 < len(ms.Content); i += 2 {
				key, val := ms.Content[i], ms.Content[i+1]
				if key.Tag == mergeTag {
					continue // don't re-emit a nested "<<" as a label
				}
				if _, ok := out[key.Value]; ok {
					continue
				}
				// The merged key's only real source is the anchor, so field()
				// keeps its value (and position) from there; an aliased value is
				// itself resolved.
				out[key.Value] = field(file, val)
			}
		}
	}
	return out
}

// mergeSources expands a merge-key value into the mapping nodes it references.
// The value is either a single alias-to-mapping or a sequence of them (`<<`).
func mergeSources(n *yaml.Node) []*yaml.Node {
	n = resolve(n)
	if n.Kind != yaml.SequenceNode {
		return []*yaml.Node{n}
	}
	out := make([]*yaml.Node, 0, len(n.Content))
	for _, item := range n.Content {
		out = append(out, resolve(item))
	}
	return out
}

// resolve follows a YAML alias to the node it points at. yaml.v3 leaves aliases
// unresolved when a node is decoded into a yaml.Node field (the rulefmt
// pattern), so `expr: *base` would otherwise yield the anchor NAME instead of
// the referenced expression. Prometheus resolves aliases when loading rules, so
// we must too (ADR 0001). A non-alias node is returned unchanged.
func resolve(n *yaml.Node) *yaml.Node {
	if n.Kind == yaml.AliasNode && n.Alias != nil {
		return n.Alias
	}
	return n
}

// field reconstructs a scalar leaf into an ir.Field. An absent key decodes to a
// zero yaml.Node, which yields a zero (non-Present) Field.
func field(file string, n *yaml.Node) ir.Field {
	if !present(n) {
		return ir.Field{}
	}
	// Value comes from the resolved node (an alias points elsewhere); the
	// position stays on the ORIGINAL usage so a finding points at the rule's own
	// line, not the distant anchor definition (ADR 0001). For a plain scalar
	// original == resolved, so behaviour is unchanged.
	return ir.Field{Value: resolve(n).Value, Pos: pos(file, n)}
}

func pos(file string, n *yaml.Node) ir.Position {
	return ir.Position{File: file, Line: n.Line, Col: n.Column}
}

// present reports whether a yaml.Node was actually decoded from the source (an
// absent field stays at the zero Kind).
func present(n *yaml.Node) bool { return n.Kind != 0 }
