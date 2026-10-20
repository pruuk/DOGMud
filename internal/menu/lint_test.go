package menu

import (
	"strings"
	"testing"
)

func TestLintLabels_AcceptsTheTalkMenu(t *testing.T) {
	if probs := LintLabels(talkMenu()); len(probs) != 0 {
		t.Fatalf("clean menu flagged: %v", probs)
	}
}

func TestLintLabels_FlagsEachRule(t *testing.T) {
	cases := map[string]Entry{
		"empty label":     {Id: "a", Label: ""},
		"label too long":  {Id: "a", Label: strings.Repeat("x", 51)},
		"detail too long": {Id: "a", Label: "ok", Detail: strings.Repeat("x", 73)},
		"em dash":         {Id: "a", Label: "Wait — what?"},
		"en dash":         {Id: "a", Label: "One – two"},
		"Ask prefix":      {Id: "a", Label: "Ask: Where is it?"},
		"Task prefix":     {Id: "a", Label: "task: The Feud"},
	}
	for name, e := range cases {
		if probs := LintLabels(Menu{Entries: []Entry{e}}); len(probs) == 0 {
			t.Errorf("%s: not flagged", name)
		}
	}
	dup := Menu{Entries: []Entry{{Id: "a", Label: "Where?"}, {Id: "b", Label: "where?"}}}
	if probs := LintLabels(dup); len(probs) == 0 {
		t.Errorf("duplicate labels not flagged")
	}
}
