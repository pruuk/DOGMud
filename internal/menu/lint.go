package menu

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	maxLabel  = 50
	maxDetail = 72
)

// LintLabels checks the menu language conventions a machine can check:
// length, no em or en dashes, no "Ask:" / "Task:" prefix, no duplicate
// labels. Phase 1 wires it into the dialogue validator.
func LintLabels(m Menu) []string {
	var probs []string
	seen := map[string]bool{}
	for i, e := range m.Entries {
		where := fmt.Sprintf("entry %d (%s)", i+1, e.Id)
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
		if strings.ContainsAny(e.Label+e.Detail, "–—") {
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
	}
	return probs
}
