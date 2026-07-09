// Package parse turns a Prometheus rule file into the home IR.
//
// Parsing is a single pass into typed structs whose leaf fields are yaml.Node
// (the rulefmt pattern): this yields field-level positions without a generic
// tree walk (ADR 0001). The yaml.Node values are then *reconstructed* into
// ir.Field values; rulefmt/promql are handed those reconstructed values as
// validators and never sit on the unmarshal path.
//
// A file holds zero or more resources (ADR 0015): the documents of a `---`
// stream, each detected by content (apiVersion + kind, never by path) as a
// native flat rule file, a prometheus-operator CRD (a monitoring.coreos.com
// PrometheusRule or PrometheusRuleList) or an unrelated K8s kind that is
// ignored. The CRD path stays on the same yaml.Node-leaf structs — no
// monitoringv1 dependency — so a CRD leaf carries a position exactly like a flat
// one; reading `expr` as a node also absorbs the operator's intstr.IntOrString
// (`expr: 0` int vs `"up == 0"` string).
//
// Parse-failure policy (ADR 0002): a document that does not decode never fails
// silently. The stream stops at the first decode error — bailing there is the
// mandatory anti-hang guard, because a syntax error makes yaml.v3 return the
// same error forever without advancing (a poisoning stream). The failure is
// then classified by content into a [Failure] the caller turns into a Finding
// (Helm-templated skip vs genuinely malformed).
package parse

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/pbrissaud/alertsmith/internal/ir"
)

// mergeTag is the tag yaml.v3 resolves a `<<` merge key to. It is not exported
// by the library, so we match on the tag string it documents.
const mergeTag = "!!merge"

// doc is one decoded YAML document. Its rule leaves are yaml.Node (the rulefmt
// pattern) so the decoder records their line/column; apiVersion/kind only drive
// content detection and need no position. A given document fills at most one
// shape: a flat file has `groups:` at the root, a CRD has `spec.groups`, a list
// has `items`.
type doc struct {
	APIVersion string      `yaml:"apiVersion"`
	Kind       string      `yaml:"kind"`
	Groups     []groupNode `yaml:"groups"` // native flat: groups at the root
	Spec       spec        `yaml:"spec"`   // CRD PrometheusRule: spec.groups
	Items      []item      `yaml:"items"`  // PrometheusRuleList: items[].spec.groups
}

// spec is the CRD `spec:` we extract from; operator-only keys (interval, limit…)
// have no field and are dropped by the lenient (non-KnownFields) decode.
type spec struct {
	Groups []groupNode `yaml:"groups"`
}

// item is one PrometheusRuleList entry — a full PrometheusRule, of which only
// spec.groups is extracted, exactly like a standalone CRD.
type item struct {
	Spec spec `yaml:"spec"`
}

// groupNode/ruleNode mirror the rule-group shape shared by the flat root and the
// CRD spec.groups; every leaf is a yaml.Node so the decoder records its
// line/column for us.
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
// — ADR 0015): the `---` documents of the stream, each a native flat file, a
// CRD, or a member of a PrometheusRuleList; unrelated K8s kinds are ignored.
//
// The returned error is either a plain I/O error (the file could not be read, a
// hard failure) or a classified [Failure] from Bytes (a parse failure the caller
// turns into a Finding); errors.As tells them apart.
func File(path string) ([]ir.PrometheusRule, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Bytes(path, data)
}

// Bytes parses rule content already in memory into its resources. The file name
// is only used to stamp provenance.
//
// It streams the `---` documents in order; each is classified by content and
// expanded into zero (an ignored kind or an empty document), one (a flat file or
// a CRD PrometheusRule) or many (a PrometheusRuleList) resources. On a decode
// error it returns the resources decoded so far together with a classified
// [Failure] in the error slot (nil error on a clean parse) — never a silent
// drop (ADR 0002).
func Bytes(file string, data []byte) ([]ir.PrometheusRule, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	var out []ir.PrometheusRule
	for {
		var d doc
		err := dec.Decode(&d)
		if err == io.EOF {
			break
		}
		if err != nil {
			// MANDATORY anti-hang guard (ADR 0002). A *syntax* error poisons the
			// rest of a `---` stream: yaml.v3 (v3.0.1, pinned) returns the SAME
			// error on every further Decode without advancing and never reaches
			// EOF — a naive continue-on-error loop spun to 5.4 GB before the OOM
			// kill (spike). The decoder exposes no resumable offset to skip the
			// bad document, so bailing on this first non-progressing error IS the
			// guard; it is a non-blocking condition of the Action, not an option.
			//
			// A *type* error (a `*yaml.TypeError`, e.g. a scalar where a sequence
			// is expected) would actually advance the decoder, so subsequent docs
			// are technically recoverable — but V1 stops uniformly for file-level
			// granularity (recovering them is the deferred "dégradé" of ADR 0002).
			// Documents decoded before this point are kept, the ones after are
			// unreachable; the failure is classified by content and surfaced as a
			// Finding by the caller.
			return out, newFailure(file, data, err)
		}
		if d.empty() {
			continue // a null / `---`-padding document carries no resource
		}
		switch classify(d) {
		case docFlat:
			out = append(out, resource(file, ir.FormatFlat, d.Groups))
		case docCRD:
			out = append(out, resource(file, ir.FormatCRD, d.Spec.Groups))
		case docList:
			// A list is one resource per item, each a CRD PrometheusRule.
			for i := range d.Items {
				out = append(out, resource(file, ir.FormatCRD, d.Items[i].Spec.Groups))
			}
		case docSkip:
			// an unrelated K8s kind (ConfigMap, Deployment…): not a PrometheusRule
		}
	}
	return out, nil
}

// Failure is a classified parse failure: the document stream stopped early and
// the caller MUST surface it as a Finding — never a silent drop, the worst
// failure for a coverage tool (ADR 0002). It implements error so a parse
// function can return it in the error slot; the engine recovers it with
// errors.As and mints the matching Finding, distinguishing it from a plain I/O
// error (an unreadable file, which stays a hard failure).
type Failure struct {
	// Helm is the content-detection verdict: the file contains `{{`/`}}`, so it
	// is a Helm template to skip as an advisory note rather than YAML that is
	// genuinely malformed (an enforceable error). Detection is by content, never
	// by path — a `templates/` dir or a neighbouring Chart.yaml are not reliable
	// (ADR 0002).
	Helm  bool
	Pos   ir.Position // file + best-effort line of the failing document
	Cause error       // the underlying yaml decode error
}

func (f *Failure) Error() string { return f.Cause.Error() }
func (f *Failure) Unwrap() error { return f.Cause }

// newFailure classifies a decode error. The Helm verdict is content detection
// over the WHOLE file (ADR 0002), keyed on Helm-specific markers rather than a
// bare `{{` scan — see helmMarkers for why a bare scan is wrong.
func newFailure(file string, data []byte, cause error) *Failure {
	return &Failure{
		Helm:  looksHelmTemplated(data),
		Pos:   ir.Position{File: file, Line: errorLine(cause), Col: 1},
		Cause: cause,
	}
}

// helmMarkers are byte sequences characteristic of Helm/Go-template *control*
// and *chart context* — the templating that actually breaks YAML before
// rendering (the ADR 0002 spike: `{{- if }}`/`{{- end }}` control-flow poisons
// the parse). A bare `{{`/`}}` scan is wrong: native Prometheus rules routinely
// template their annotations with the SAME delimiters (`{{ $value }}`,
// `{{ $labels.instance }}`), so a genuinely-malformed native file would be
// misclassified as an advisory Helm skip and slip past the enforceable
// yaml-malformed gate. These markers key on the whitespace-trim tokens and the
// `.`-rooted chart objects that Prometheus's `$`-variable templating never uses.
//
// The discriminator is a heuristic, not a proof (ADR 0002 already notes content
// detection is unreliable): a Prometheus annotation that itself uses `{{-` trim
// markers is a rare residual false-Helm. That is the safe side — helm-skipped is
// advisory and can never block, so it cannot turn a broken file into a blocked
// merge, whereas the far more common `{{ $value }}` false positive of the old
// bare scan silently downgraded real errors.
var helmMarkers = [][]byte{
	[]byte("{{-"), []byte("-}}"), // whitespace-trim markers: Go-template control
	[]byte(".Values"), []byte(".Release"), []byte(".Chart"), // Helm chart objects
	[]byte(".Capabilities"), []byte(".Files"),
}

func looksHelmTemplated(data []byte) bool {
	for _, m := range helmMarkers {
		if bytes.Contains(data, m) {
			return true
		}
	}
	return false
}

// yamlLineRe matches the 1-based line yaml.v3 embeds in a decode error
// ("yaml: line N: …"). yaml.v3 is pinned in go.mod, so the format is stable.
var yamlLineRe = regexp.MustCompile(`line (\d+)`)

// errorLine best-effort extracts that line, falling back to line 1 (top of
// file) so a file-level Finding always carries a renderable anchor (ADR 0011):
// some syntax errors (e.g. "invalid map key") carry no line, and an unrecognised
// format degrades to line 1 rather than a wrong line.
func errorLine(err error) int {
	if m := yamlLineRe.FindStringSubmatch(err.Error()); m != nil {
		if n, e := strconv.Atoi(m[1]); e == nil && n > 0 {
			return n
		}
	}
	return 1
}

// docType is how a decoded document maps to resources.
type docType int

const (
	docSkip docType = iota // an unrelated K8s kind: no resource
	docFlat                // native flat file: groups at the root
	docCRD                 // a prometheus-operator PrometheusRule: spec.groups
	docList                // a PrometheusRuleList: one resource per item
)

const (
	kindRule     = "PrometheusRule"
	kindRuleList = "PrometheusRuleList"
	// apiGroupCoreOS is the prometheus-operator API group prefix. Detection
	// matches the group, not a pinned version: a v1beta1/v1alpha1 PrometheusRule
	// must still be covered — silently skipping it is the worst failure for a
	// coverage tool (ADR 0015).
	apiGroupCoreOS = "monitoring.coreos.com/"
)

// classify decides what a document expands into, from its apiVersion + kind
// (content detection — ADR 0001, never by path). No kind is a native flat file;
// the operator kinds are honoured only under the monitoring.coreos.com group (or
// a missing apiVersion); any other kind is an unrelated K8s object we ignore.
func classify(d doc) docType {
	switch d.Kind {
	case "":
		return docFlat
	case kindRule:
		if operatorGroup(d.APIVersion) {
			return docCRD
		}
		return docSkip
	case kindRuleList:
		if operatorGroup(d.APIVersion) {
			return docList
		}
		return docSkip
	default:
		return docSkip
	}
}

// operatorGroup reports whether an apiVersion belongs to the prometheus-operator
// group. An absent apiVersion is accepted (a kind: PrometheusRule that forgot
// the field is still one); a foreign group borrowing the kind name is not.
func operatorGroup(apiVersion string) bool {
	return apiVersion == "" || strings.HasPrefix(apiVersion, apiGroupCoreOS)
}

// empty reports whether a decoded document carries nothing to extract — a null
// document from `---` padding or a Helm template that rendered to nothing. It is
// tested before classify because a null document and a flat one both have an
// empty Kind; a stray apiVersion-only fragment with no groups is empty too, so
// it is skipped rather than emitted as an empty flat resource.
func (d doc) empty() bool {
	return d.Kind == "" && len(d.Groups) == 0 && len(d.Spec.Groups) == 0 && len(d.Items) == 0
}

// resource builds one normalised PrometheusRule from a document's rule groups —
// the same walk for the flat root, a CRD spec.groups and a list item, so the IR
// downstream is identical regardless of the envelope.
func resource(file string, format ir.Format, groups []groupNode) ir.PrometheusRule {
	pr := ir.PrometheusRule{File: file, Format: format}
	for i := range groups {
		g := &groups[i]
		group := ir.Group{
			Name: field(file, &g.Name),
			Pos:  pos(file, &g.Name),
		}
		for j := range g.Rules {
			group.Rules = append(group.Rules, rule(file, &g.Rules[j]))
		}
		pr.Groups = append(pr.Groups, group)
	}
	return pr
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
