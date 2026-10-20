package menu

import (
	"strings"
	"testing"
)

// The dashes are built from code points so no literal dash character lives in
// this source file.
var (
	emDash = string(rune(0x2014))
	enDash = string(rune(0x2013))
)

func hasProblem(probs []string, sub string) bool {
	for _, p := range probs {
		if strings.Contains(p, sub) {
			return true
		}
	}
	return false
}

func TestLintLabels_AcceptsTheTalkMenu(t *testing.T) {
	if probs := LintLabels(talkMenu()); len(probs) != 0 {
		t.Fatalf("clean menu flagged: %v", probs)
	}
}

func TestLintLabels_FlagsEachRule(t *testing.T) {
	cases := []struct {
		name string
		e    Entry
		want string
	}{
		{"empty label", Entry{Id: "a", Label: ""}, "empty label"},
		{"label too long", Entry{Id: "a", Label: strings.Repeat("x", 51)}, "label over"},
		{"detail too long", Entry{Id: "a", Label: "ok", Detail: strings.Repeat("x", 71)}, "detail over"},
		{"em dash", Entry{Id: "a", Label: "Wait " + emDash + " what?"}, "em or en dash"},
		{"en dash", Entry{Id: "a", Label: "One " + enDash + " two"}, "em or en dash"},
		{"Ask prefix", Entry{Id: "a", Label: "Ask: Where is it?"}, "prefix"},
		{"Task prefix", Entry{Id: "a", Label: "task: The Feud"}, "prefix"},
	}
	for _, c := range cases {
		probs := LintLabels(Menu{Entries: []Entry{c.e}})
		if !hasProblem(probs, c.want) {
			t.Errorf("%s: want a problem containing %q, got %v", c.name, c.want, probs)
		}
	}

	dup := Menu{Entries: []Entry{{Id: "a", Label: "Where?"}, {Id: "b", Label: "where?"}}}
	if probs := LintLabels(dup); !hasProblem(probs, "duplicate label") {
		t.Errorf("duplicate labels: %v", probs)
	}
	dupID := Menu{Entries: []Entry{{Id: "a", Label: "One"}, {Id: "a", Label: "Two"}}}
	if probs := LintLabels(dupID); !hasProblem(probs, "duplicate id") {
		t.Errorf("duplicate ids: %v", probs)
	}
	badDefault := Menu{Entries: []Entry{{Id: "a", Label: "One"}}, Default: "zzz"}
	if probs := LintLabels(badDefault); !hasProblem(probs, "default") {
		t.Errorf("dangling default: %v", probs)
	}
}

func TestLintLabels_Boundaries(t *testing.T) {
	ok := Menu{Entries: []Entry{{Id: "a", Label: strings.Repeat("x", 50), Detail: strings.Repeat("y", 70)}}}
	if probs := LintLabels(ok); len(probs) != 0 {
		t.Errorf("50-char label and 70-char detail flagged: %v", probs)
	}
	if probs := LintLabels(Menu{Entries: []Entry{{Id: "a", Label: strings.Repeat("x", 51)}}}); !hasProblem(probs, "label over") {
		t.Errorf("51-char label not flagged: %v", probs)
	}
	if probs := LintLabels(Menu{Entries: []Entry{{Id: "a", Label: "ok", Detail: strings.Repeat("y", 71)}}}); !hasProblem(probs, "detail over") {
		t.Errorf("71-char detail not flagged: %v", probs)
	}
}

func TestLintLabels_LeaveAliasMustNotShadowAnEntry(t *testing.T) {
	ok := Menu{Entries: []Entry{{Id: "a", Label: "One"}}, Leave: "Done", LeaveAliases: []string{"0"}}
	if probs := LintLabels(ok); len(probs) != 0 {
		t.Errorf("clean leave alias flagged: %v", probs)
	}
	clash := Menu{Entries: []Entry{{Id: "x", Label: "One", Aliases: []string{"q"}}, {Id: "quit", Label: "Two"}}, Leave: "Done", LeaveAliases: []string{"Q", "quit"}}
	probs := LintLabels(clash)
	if !hasProblem(probs, `leave alias "Q"`) || !hasProblem(probs, `leave alias "quit"`) {
		t.Errorf("leave aliases shadowing an entry: %v", probs)
	}
}

func TestLintLabels_ChecksTheLeaveLabel(t *testing.T) {
	long := Menu{Entries: []Entry{{Id: "a", Label: "One"}}, Leave: strings.Repeat("x", 51)}
	if probs := LintLabels(long); !hasProblem(probs, "label over") {
		t.Errorf("long leave label: %v", probs)
	}
	dash := Menu{Entries: []Entry{{Id: "a", Label: "One"}}, Leave: "Go " + emDash + " now"}
	if probs := LintLabels(dash); !hasProblem(probs, "em or en dash") {
		t.Errorf("dash in leave label: %v", probs)
	}
	dup := Menu{Entries: []Entry{{Id: "a", Label: "Leave"}}, Leave: "leave"}
	if probs := LintLabels(dup); !hasProblem(probs, "duplicate label") {
		t.Errorf("entry duplicating the leave label: %v", probs)
	}
}
