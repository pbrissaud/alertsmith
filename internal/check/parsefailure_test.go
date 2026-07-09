package check

import (
	"strings"
	"testing"

	"github.com/pbrissaud/alertsmith/internal/finding"
	"github.com/pbrissaud/alertsmith/internal/ir"
)

func TestParseFailureFinding(t *testing.T) {
	pos := ir.Position{File: "f.yaml", Line: 3, Col: 1}

	t.Run("helm skip is an advisory note", func(t *testing.T) {
		// The frozen contract (checks-v1 / ADR 0002): a Helm skip can NEVER block,
		// whatever block_on — it must be advisory, at level note.
		f := ParseFailureFinding(true, pos, "yaml: line 3: boom")
		if f.Check != "helm-skipped" {
			t.Errorf("check = %q, want helm-skipped (frozen id)", f.Check)
		}
		if f.Level != finding.Note {
			t.Errorf("level = %v, want note", f.Level)
		}
		if f.Enforceable {
			t.Errorf("Enforceable = true, want false: a Helm skip must never block")
		}
		if f.Origin != finding.Deterministic {
			t.Errorf("origin = %v, want deterministic", f.Origin)
		}
		if f.Pos != pos {
			t.Errorf("pos = %+v, want %+v", f.Pos, pos)
		}
	})

	t.Run("malformed YAML is an enforceable error", func(t *testing.T) {
		f := ParseFailureFinding(false, pos, "yaml: line 3: boom")
		if f.Check != "yaml-malformed" {
			t.Errorf("check = %q, want yaml-malformed (frozen id)", f.Check)
		}
		if f.Level != finding.Error {
			t.Errorf("level = %v, want error", f.Level)
		}
		if !f.Enforceable {
			t.Errorf("Enforceable = false, want true: malformed YAML may block")
		}
		// Symmetric with the helm branch: the enforceable, blocking finding must
		// also carry a deterministic origin and a clickable position (ADR 0011).
		if f.Origin != finding.Deterministic {
			t.Errorf("origin = %v, want deterministic", f.Origin)
		}
		if f.Pos != pos {
			t.Errorf("pos = %+v, want %+v", f.Pos, pos)
		}
	})
}

func TestParseFailureFinding_cleansCause(t *testing.T) {
	// A yaml *TypeError is multi-line and names an unexported Go type. The finding
	// Message must stay a single line (report.Text is one line per finding) and
	// not leak internals, or a broken document corrupts the machine-readable
	// stdout stream and exposes `parse.groupNode` (review F5/F7).
	cause := "yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `oops` into []parse.groupNode"
	f := ParseFailureFinding(false, ir.Position{File: "f.yaml", Line: 1, Col: 1}, cause)
	if strings.Contains(f.Message, "\n") {
		t.Errorf("message is multi-line, want a single line: %q", f.Message)
	}
	if strings.Contains(f.Message, "parse.groupNode") {
		t.Errorf("message leaks the internal type name: %q", f.Message)
	}
}
