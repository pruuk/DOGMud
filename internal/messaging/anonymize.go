package messaging

import "regexp"

// nameTagPattern matches player, mob and pet identity tags, including the
// suffixed forms FormattedName.String renders for display roles and duplicate
// indices (`mobname-dup2`, `username-aggro`, `username-dead`), and the
// optional adjective span FormattedName.String prints right after the tag.
//
// Player tags need the suffix as much as mob tags do. Until 2026-09-11 only
// `mobname` accepted one, and until slice B (2026-09-12) GetCharacterName(true)
// rendered `username-aggro` for any character not fighting a player, so the
// suffixed form was the common one in room text: a player's name reached
// infrared-only observers in full. A foe fighting the viewer still renders it,
// and `username-dead` never depended on the viewer, so the suffix branch stays.
//
// The adjective span has to go here too: rooms.go's sendTextVisualJudgedBy
// deliberately runs `HideNames(Anonymize(txt), ...)`, Anonymize first, so by
// the time HideNames would try to remove the span there is no tag left to
// anchor on and an infrared observer reads "a figure (dead)".
var nameTagPattern = regexp.MustCompile(
	`<ansi fg="((?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">[^<]+</ansi>(?:` + adjectiveSpanBody + `)?`,
)

// Anonymize strips player/mob/pet name ANSI tags and replaces them
// with a `combat-anon`-colored "a figure" placeholder. Used by the
// pipeline for infrared-only observers in dark rooms.
//
// v1 limitation: bare-name occurrences (names embedded in prose
// without an ANSI tag) leak through. The 228-site audit gets most
// names properly tagged; remaining leaks are tracked as followups.
func Anonymize(text string) string {
	if text == "" {
		return text
	}
	return nameTagPattern.ReplaceAllString(text,
		`<ansi fg="combat-anon">a figure</ansi>`)
}
