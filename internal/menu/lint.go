package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxLabel  = 50
	maxDetail = 70
)

// dashes holds the em and en dash. Built from code points so the source
// carries no literal dash character.
var dashes = string(rune(0x2013)) + string(rune(0x2014))

// LintLabels checks the menu language conventions a machine can check:
// length, no em or en dashes, no "Ask:" / "Task:" prefix, no duplicate
// labels or ids, and a Default that names a real entry. The Leave label is
// linted like any other. Phase 1 wires it into the dialogue validator.
func LintLabels(m Menu) []string {
	var probs []string
	seen := map[string]bool{}
	seenID := map[string]bool{}
	for i, e := range m.all() {
		id := e.Id
		if m.Leave != "" && i == len(m.Entries) {
			id = "leave"
		}
		where := fmt.Sprintf("entry %d (%s)", i+1, id)
		label := strings.TrimSpace(e.Label)
		if label == "" {
			probs = append(probs, where+": empty label")
		}
		if utf8.RuneCountInString(e.Label) > maxLabel {
			probs = append(probs, fmt.Sprintf("%s: label over %d characters", where, maxLabel))
		}
		if utf8.RuneCountInString(e.Detail) > maxDetail {
			probs = append(probs, fmt.Sprintf("%s: detail over %d characters", where, maxDetail))
		}
		if strings.ContainsAny(e.Label+e.Detail, dashes) {
			probs = append(probs, where+": em or en dash")
		}
		low := strings.ToLower(label)
		if strings.HasPrefix(low, "ask:") || strings.HasPrefix(low, "task:") {
			probs = append(probs, where+`: "Ask:" or "Task:" prefix`)
		}
		if label != "" && seen[low] {
			probs = append(probs, where+": duplicate label")
		}
		seen[low] = true
		if e.Id != "" {
			if seenID[e.Id] {
				probs = append(probs, where+": duplicate id")
			}
			seenID[e.Id] = true
		}
	}
	// A leave alias is checked first, so one that matches an entry's id or
	// alias would hide that entry.
	for _, a := range m.LeaveAliases {
		for _, e := range m.Entries {
			hit := strings.EqualFold(a, e.Id)
			for _, ea := range e.Aliases {
				hit = hit || strings.EqualFold(a, ea)
			}
			if hit {
				probs = append(probs, fmt.Sprintf("leave alias %q shadows entry %q", a, e.Id))
			}
		}
	}
	if m.Default != "" {
		found := false
		for _, e := range m.Entries {
			if e.Id == m.Default {
				found = true
				break
			}
		}
		if !found {
			probs = append(probs, fmt.Sprintf("default %q names no entry", m.Default))
		}
	}
	return probs
}
