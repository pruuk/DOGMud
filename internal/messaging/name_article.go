package messaging

import (
	"regexp"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// identityNameSpan matches a player or mob identity open tag and the name text
// it opens, up to the next tag. Pet names are left out: a player picks those.
var identityNameSpan = regexp.MustCompile(`<ansi fg="(?:username|mobname)(?:-[A-Za-z0-9_-]+)?">([^<]+)`)

// LowerMidSentenceNameArticles lowercases the article a creature's own name
// opens with ("A Culvert Slime", "An Ox", "The Pass-Apex") wherever that name
// stands mid-sentence, so "You barrel into A Culvert Slime" reads "You barrel
// into a Culvert Slime" and "Also here: a Culvert Slime" (#572, owner ruling
// 2026-10-19). A name that begins a sentence keeps its capital, by the same
// test HideNames uses (atSentenceStart): the start of the text or a line, or a
// sentence end and a space. Mob data keeps the capital; this is the one place
// every tagged name passes, which is why the fix lives here and not at each of
// the hundreds of sites that format a name. The word rule is
// items.MidSentenceName, the one items already use.
//
// Only a name inside an identity tag changes: untagged prose is authored and
// owns its casing. Every category runs it, the hand-formatted ones too,
// because a name's article is not a style choice the category made.
func LowerMidSentenceNameArticles(text string) string {
	locs := identityNameSpan.FindAllStringSubmatchIndex(text, -1)
	if locs == nil {
		return text
	}
	// Rewrites keep every length (only letter case changes), so the indexes
	// found above stay valid while the loop edits.
	b := []byte(text)
	for _, m := range locs {
		nameStart, nameEnd := m[2], m[3]
		name := text[nameStart:nameEnd]
		lowered := items.MidSentenceName(name)
		if lowered == name || atSentenceStart(text, m[0]) {
			continue
		}
		copy(b[nameStart:nameEnd], lowered)
	}
	return string(b)
}
