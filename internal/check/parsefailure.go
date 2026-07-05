package check

import (
	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/ir"
)

// Parse-failure findings own the two check ids of the ADR 0002 matrix. They are
// not Check-interface implementations (they run on a parse failure, not on a
// parsed resource, so they never appear in Registered()), but the (id, Level,
// enforceable) tuples are checks-v1 registry contracts and live here with the
// other checks. The engine calls ParseFailureFinding when parse hands it a
// classified failure instead of resources.
const (
	idHelmSkipped   = "helm-skipped"
	idYAMLMalformed = "yaml-malformed"
)

// ParseFailureFinding turns a classified parse failure into its Finding, so a
// file that does not parse becomes output rather than a silent drop — the worst
// failure for a coverage tool (ADR 0002).
//
// helm=true → helm-skipped: Level note, advisory. It is enforceable=false by
// nature and can therefore NEVER block, whatever block_on (ADR 0002/0006) — a
// Helm-templated file is skipped, not judged.
//
// helm=false → yaml-malformed: Level error, enforceable. Genuinely broken YAML
// is a real defect that may block.
func ParseFailureFinding(helm bool, pos ir.Position, cause string) finding.Finding {
	if helm {
		return finding.Finding{
			Check:       idHelmSkipped,
			Level:       finding.Note,
			Origin:      finding.Deterministic,
			Enforceable: false,
			Message:     "skipped: Helm-templated, not valid YAML before rendering (" + cause + ")",
			Pos:         pos,
		}
	}
	return finding.Finding{
		Check:       idYAMLMalformed,
		Level:       finding.Error,
		Origin:      finding.Deterministic,
		Enforceable: true,
		Message:     "YAML does not parse: " + cause,
		Pos:         pos,
	}
}
