// Package banner formats the SKILL ADVANCEMENT / STATISTIC INCREASED
// banner used by the progression system. Lives as a leaf package
// (no imports of characters, users, or messaging) so that
// internal/characters/ can import it without creating the
// messaging→characters cycle.
//
// characters/progression_notify.go (notifyProgressionBanner) hands the
// callback registered in main.go a render function that calls Format at the
// reader's width; the callback delivers it through the messaging pipeline
// on CategorySkillProgress.
//
// This comment used to describe an internal/messaging/progression.go
// re-exporting a SendProgression helper for callers that already hold
// a UserSender. No such file, function or interface exists in
// internal/messaging, and internal/messaging/context.md carried the
// same phantoms until 2026-09-08. Nothing needs building here; the
// reference was simply wrong.
package banner

import (
	"strings"
	"unicode/utf8"
)

// Kind discriminates skill vs stat advancement banners.
type Kind int

const (
	Skill Kind = iota
	Stat
)

// TierChange marks a tier crossing (e.g., apprentice → journeyman).
// nil = no tier change; banner omits the transition line.
type TierChange struct {
	From string
	To   string
}

// RuleWidth is the widest the banner's rule is drawn: a desktop has always
// seen 64 glyphs of `━`.
const RuleWidth = 64

// Format returns the banner string (no trailing newline). Three lines
// when tier is nil (rule / title / name / rule); four when a tier
// change is supplied.
//
// width is the reader's screen width. The rule is min(width, RuleWidth)
// glyphs and every line is centred against it, so a 30-column phone gets a
// 30-glyph rule and a desktop the 64 it always had. A width of 0 or less
// (none known) draws at RuleWidth. The banner is sent on a per-round path,
// so it is laid out for the width here and never measured against a form
// (spec ruling 12, plan 3b-3).
func Format(kind Kind, name string, tier *TierChange, width int) string {
	if width <= 0 || width > RuleWidth {
		width = RuleWidth
	}
	rule := strings.Repeat("━", width)
	center := func(s string) string { return centerIn(s, width) }
	var title string
	switch kind {
	case Skill:
		title = "SKILL ADVANCEMENT"
	case Stat:
		title = "STATISTIC INCREASED"
	default:
		title = "PROGRESSION"
	}

	lines := []string{
		rule,
		center(title),
		center(name),
	}
	if tier != nil {
		lines = append(lines, center(tier.From+" → "+tier.To))
	}
	lines = append(lines, rule)
	return strings.Join(lines, "\n")
}

// centerIn pads s with leading spaces to roughly center it inside a rule
// width glyphs wide.
//
// Uses rune count (not byte count) for s because the rule is composed of
// `━` (3-byte UTF-8 chars) and the text may hold `→`: len() would push the
// centred text far to the right of the rule.
func centerIn(s string, width int) string {
	sWidth := utf8.RuneCountInString(s)
	if sWidth >= width {
		return s
	}
	pad := (width - sWidth) / 2
	return strings.Repeat(" ", pad) + s
}
