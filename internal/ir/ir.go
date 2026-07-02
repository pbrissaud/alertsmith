// Package ir defines AlertSmith's home intermediate representation (IR).
//
// Both input formats (native flat Prometheus files and the prometheus-operator
// CRD) are normalised into this IR; the whole engine downstream — deterministic
// checks and, later, the LLM layer — runs *exclusively* on the IR and never
// re-reads the source format (ADR 0001).
//
// Provenance (file + line) is carried at the *leaf*, not at the root: a
// multi-document YAML file would otherwise break the model (ADR 0001). Every
// flaggable scalar is a [Field], which pairs the reconstructed value with the
// [Position] it came from.
package ir

// Position is the provenance of a single leaf: the file it came from and the
// 1-based line/column reported by the YAML node. It is what turns a Finding
// into a clickable file:line and, later, a GitHub check-run annotation (0011).
type Position struct {
	File string
	Line int
	Col  int
}

// Field is a scalar leaf of the IR together with its provenance.
//
// The value is *reconstructed* from the parsed YAML node (its string form), so
// validators such as promql/parser receive plain values and never sit on the
// unmarshal path (ADR 0001). Reading the value as a string also absorbs the
// native `expr: 0` (int) vs `expr: "up == 0"` (string) case without any typed
// dependency.
type Field struct {
	Value string
	Pos   Position
}

// Present reports whether the field was actually set in the source. A zero
// Field (no position) means the key was absent — the distinction most
// "missing-*" checks rely on.
//
// INVARIANT: presence is deliberately welded to provenance. Every present leaf
// originates from a positioned yaml.Node (ADR 0001 mandates yaml.Node leaves for
// both the native and CRD formats — no decode into operator Go types that would
// drop the line), so a present Field always carries a non-zero Pos.Line. This is
// a feature, not a coincidence: a value we cannot point at is unrenderable as a
// check-run annotation (ADR 0011). If a leaf ever has a value but no position,
// that is a parser bug this coupling surfaces early — not a case to paper over.
// Only introduce a separate `set bool` if V1 ever fabricates a value with no
// source line (e.g. a defaulted annotation); no such path exists today.
func (f Field) Present() bool { return f.Pos.Line != 0 }

// Kind distinguishes the two rule flavours Prometheus supports, plus the
// degenerate case. A rule block must carry exactly one of `alert:`/`record:`.
type Kind int

const (
	// Invalid is a rule block that carries neither or both of `alert:`/`record:`
	// — not a valid single-purpose rule. It is the zero value on purpose (a
	// fail-safe: an unclassified rule is never silently treated as an alerting
	// rule). rulefmt rejects such blocks; the rule-structure check (#6) owns the
	// precise Finding, and alerting-only checks (#7) skip Invalid rules to avoid
	// a cascade of phantom findings on an already-broken rule.
	Invalid Kind = iota
	// Alerting is a rule that fires an alert when its PromQL is true (`alert:`).
	Alerting
	// Recording is a rule that precomputes a derived metric (`record:`).
	Recording
)

func (k Kind) String() string {
	switch k {
	case Alerting:
		return "alerting"
	case Recording:
		return "recording"
	default:
		return "invalid"
	}
}

// Rule is a single alerting, recording or invalid rule, normalised.
type Rule struct {
	Kind Kind
	// Name is the alert name (`alert:`) or recorded metric name (`record:`).
	// Empty for an Invalid rule that carries neither key.
	Name Field
	Expr Field
	// For is optional; check Present() before use.
	For         Field
	Labels      map[string]Field
	Annotations map[string]Field
	// Pos points at the rule itself (its name node), used when a Finding is
	// about the rule as a whole rather than one of its leaves.
	Pos Position
}

// Group is a named set of rules evaluated together (`groups[].name`).
type Group struct {
	Name  Field
	Rules []Rule
	Pos   Position
}

// Format records which envelope a PrometheusRule was decoded from. Flat is the
// only format handled in the walking skeleton; CRD arrives in a later slice.
type Format int

const (
	FormatFlat Format = iota
	FormatCRD
)

// PrometheusRule is one normalised resource: the groups extracted from a single
// source document, tagged with the file and format they came from.
type PrometheusRule struct {
	Groups []Group
	File   string
	Format Format
}
