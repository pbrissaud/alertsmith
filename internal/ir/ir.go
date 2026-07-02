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
func (f Field) Present() bool { return f.Pos.Line != 0 }

// Kind distinguishes the two rule flavours Prometheus supports.
type Kind int

const (
	// Alerting is a rule that fires an alert when its PromQL is true (`alert:`).
	Alerting Kind = iota
	// Recording is a rule that precomputes a derived metric (`record:`).
	Recording
)

// Rule is a single alerting or recording rule, normalised.
type Rule struct {
	Kind Kind
	// Name is the alert name (`alert:`) or recorded metric name (`record:`).
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
