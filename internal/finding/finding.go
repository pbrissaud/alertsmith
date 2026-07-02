// Package finding defines the atomic output of the engine.
//
// A Finding is one problem detected on one rule, carrying its provenance, its
// origin (deterministic vs LLM) and its enforceability. Origin and
// enforceability are two *distinct* axes: LLM findings are always advisory, but
// enforceability is not derivable from origin — some deterministic findings are
// advisory by nature (Helm skip, incomplete-index missing-ref, …) (ADR 0006).
package finding

import "github.com/pbrissaud/alertsmith/internal/ir"

// Level is the severity of a Finding — a property of the engine, distinct from
// the alert's own `severity` label (which is the client's data). Values are
// ordered note < warning < error so a later `block_on` threshold can be a
// single `Level >= threshold` comparison (ADR 0004).
type Level int

const (
	Note Level = iota
	Warning
	Error
)

func (l Level) String() string {
	switch l {
	case Note:
		return "note"
	case Warning:
		return "warning"
	case Error:
		return "error"
	default:
		return "unknown"
	}
}

// Origin records whether a Finding came from a deterministic check or the LLM
// layer. LLM findings are always advisory (ADR 0006). The LLM layer is out of
// V1; the axis exists now so the verdict logic (block_on) is written once.
type Origin int

const (
	Deterministic Origin = iota
	LLM
)

func (o Origin) String() string {
	if o == LLM {
		return "llm"
	}
	return "deterministic"
}

// Finding is the atomic unit of engine output.
//
// Enforceable is carried explicitly: `Blocking = Enforceable && Level >=
// block_on` is computed at verdict time (ADR 0006), it is not stored here.
type Finding struct {
	Check       string // the check id that produced it (frozen contract, checks-v1)
	Level       Level
	Origin      Origin
	Enforceable bool
	Message     string
	Pos         ir.Position
}

// Report is the aggregate of every Finding produced by a run.
type Report struct {
	Findings []Finding
}

// Add appends findings to the report.
func (r *Report) Add(fs ...Finding) { r.Findings = append(r.Findings, fs...) }

// Empty reports whether the run produced no findings.
func (r *Report) Empty() bool { return len(r.Findings) == 0 }
