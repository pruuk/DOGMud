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
// The adjective span has to go here too: rooms.go's deliverVisual
// deliberately runs `HideNames(Anonymize(txt), ...)`, Anonymize first, so by
// the time HideNames would try to remove the span there is no tag left to
// anchor on and an infrared observer reads "a figure (dead)".
//
// The `poss` group (#246) captures a possessive written INSIDE the tag, the
// shape about 275 combat template lines use (the YAML puts the doubled
// quote of {actor}'s before the closing tag). The body
// is lazy so the optional group gets the 's rather than the body swallowing
// it; Anonymize re-emits it after the figure word.
var nameTagPattern = regexp.MustCompile(
	`<ansi fg="((?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">[^<]+?(?P<poss>'s|’s)?</ansi>(?:` + adjectiveSpanBody + `)?`,
)

var nameTagPossIdx = nameTagPattern.SubexpIndex("poss")

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
	// Capitalise at a sentence start, the same rule HideNames applies.
	//
	// 🪤 This used to substitute a flat lowercase "a figure" everywhere, and
	// the difference shows in play because Anonymize runs BEFORE HideNames in
	// the room pipeline (see rooms.deliverVisual, where the ordering
	// is deliberate so whole name tags are matched first). By the time
	// HideNames runs the name is already gone, so its capitalisation never
	// got a chance. Read in play on 2026-09-21, among correctly capitalised
	// siblings: "a figure moves with increasing swiftness.", "a figure
	// clambers to their feet in a rushed panic." and, after a banner,
	// "*** a figure lands a DEVASTATING SNAP on you! ***".
	out := make([]byte, 0, len(text))
	last := 0
	for _, loc := range nameTagPattern.FindAllStringSubmatchIndex(text, -1) {
		out = append(out, text[last:loc[0]]...)
		word := "a figure"
		if atSentenceStart(text, loc[0]) {
			word = "A figure"
		}
		out = append(out, `<ansi fg="combat-anon">`+word+`</ansi>`...)
		if ps := loc[2*nameTagPossIdx]; ps >= 0 {
			out = append(out, text[ps:loc[2*nameTagPossIdx+1]]...)
		}
		last = loc[1]
	}
	if last == 0 {
		return text
	}
	return string(append(out, text[last:]...))
}
