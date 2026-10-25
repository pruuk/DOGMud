package questloop

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Backlog is the committed list of known findings, quests/loop-backlog.txt
// (spec 5.3). A listed finding is a warning; an unlisted one refuses; a
// listed line with no finding is stale and fails CI, so the list only
// shrinks. Phase 7 deletes lines as each zone pass clears them.
type Backlog map[string]bool

const backlogHeader = `# Quest loop backlog (#375 phase 2, spec 5.3). One finding key per line,
# "rule questId where", for a loop-rule failure in shipped content that the
# phase 7 zone passes have not fixed yet. A finding listed here is a boot
# warning; any finding not listed refuses boot, CI and the quest editor's
# save. A line with no matching finding fails CI: delete it when its finding
# is fixed. Never add a line by hand to make new content pass.
`

// ParseBacklog reads the backlog text; comment and blank lines are skipped.
func ParseBacklog(data []byte) (Backlog, error) {
	b := Backlog{}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	for n, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if b[line] {
			return b, fmt.Errorf("loop-backlog.txt line %d: repeated line %q", n+1, line)
		}
		b[line] = true
	}
	return b, nil
}

// FormatBacklog writes the findings' keys as backlog text, sorted, once each.
func FormatBacklog(fs []Finding) []byte {
	seen := map[string]bool{}
	var ks []string
	for _, f := range fs {
		if k := f.Key(); !seen[k] {
			seen[k] = true
			ks = append(ks, k)
		}
	}
	sort.Strings(ks)
	return []byte(backlogHeader + strings.Join(ks, "\n") + "\n")
}

// Ratchet splits findings into fresh (not in the backlog: they refuse) and
// known (listed: warnings), and returns the backlog lines no finding matched.
func Ratchet(fs []Finding, b Backlog) (fresh, known []Finding, stale []string) {
	matched := map[string]bool{}
	for _, f := range fs {
		k := f.Key()
		if b[k] {
			known = append(known, f)
			matched[k] = true
		} else {
			fresh = append(fresh, f)
		}
	}
	for k := range b {
		if !matched[k] {
			stale = append(stale, k)
		}
	}
	sort.Strings(stale)
	return fresh, known, stale
}
