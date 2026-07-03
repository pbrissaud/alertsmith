// Package check holds the deterministic checks and the seam they plug into.
//
// The walking skeleton defines only the minimal Check interface and a flat
// slice of registered checks. The full registry — per-check default Level,
// enforceable/advisory, on/off, driving the .alertsmith.yaml keys and
// Suppression validation — is a later slice (checks-v1 registry, issue #6).
//
// Every check here is a *mechanism*: OSS/free forever, consuming no paid
// Ruleset in V1 (ADR 0012/0013).
package check

import (
	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/ir"
)

// Check evaluates one normalised PrometheusRule and returns any findings.
//
// Checks with repo-wide scope (recording-rule resolution, issue #8) will take a
// broader context; that richer signature is introduced when it is first needed
// rather than guessed at here.
type Check interface {
	ID() string
	Check(pr *ir.PrometheusRule) []finding.Finding
}

// Registered returns the checks the engine runs, in order. The skeleton wires a
// single check to prove the pipeline end to end; issue #6 turns this into the
// real registry.
func Registered() []Check {
	return []Check{
		PromQLParse{},
	}
}
