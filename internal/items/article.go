package items

import "strings"

// ArticlePrefix returns what a line puts before one item's name, trailing
// space included, so a caller writes `"You find %s%s"` with no space of its
// own: "some " for a name that reads as plural ("Iron Lockpicks", "Leather
// Boots"), otherwise "an " before a vowel and "a " before anything else, the
// letter rule the messaging normalizer applies. A sale line once read "You
// sell an Iron Lockpicks".
//
// A name that already starts with an article ("A Sheaf of Gleaned Grain",
// "An", "The", "Some") gets nothing: a forage line once read "You find an A
// Sheaf of Gleaned Grain." (2026-10-17).
//
// A name reads as plural when its last word ends in "s" but not "ss", "us"
// or "is" (Cutlass, Abacus and Aegis stay singular). Item data has no plural
// field; every shipped name this rule calls plural is one (2026-10-17).
func ArticlePrefix(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return `a `
	}
	if len(words) > 1 {
		switch strings.ToLower(words[0]) {
		case `a`, `an`, `the`, `some`:
			return ``
		}
	}
	last := strings.ToLower(words[len(words)-1])
	if strings.HasSuffix(last, `s`) && !strings.HasSuffix(last, `ss`) &&
		!strings.HasSuffix(last, `us`) && !strings.HasSuffix(last, `is`) {
		return `some `
	}
	switch strings.ToLower(words[0])[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return `an `
	}
	return `a `
}

// MidSentenceName returns name as it reads after ArticlePrefix: a name that
// opens with its own article ("A Sheaf of Gleaned Grain") has that article
// lowercased, so the line reads "You find a Sheaf of Gleaned Grain." rather
// than "You find A Sheaf..." (2026-10-17 newbie-duo playtest, #620). Only
// the article words ArticlePrefix recognises change; every other capital may
// be part of a proper name and stays. Callers pair it with ArticlePrefix:
// `"You find %s%s", ArticlePrefix(n), MidSentenceName(n)`.
func MidSentenceName(name string) string {
	if first, rest, ok := leadingArticle(name); ok {
		return strings.ToLower(first) + ` ` + rest
	}
	return name
}

// NameWithoutArticle returns name with its own leading article dropped, for a
// compound noun that brings its own article or count: a corpse reads "the
// Culvert Slime corpse" and "4 Culvert Slime corpses", never "the A Culvert
// Slime corpse" (#572). The article words are MidSentenceName's.
func NameWithoutArticle(name string) string {
	if _, rest, ok := leadingArticle(name); ok {
		return rest
	}
	return name
}

// leadingArticle splits off the article a name opens with, if any. A name
// that is nothing but the word ("A", "The") has no article to split.
func leadingArticle(name string) (first, rest string, ok bool) {
	first, rest, ok = strings.Cut(name, ` `)
	if !ok || strings.TrimSpace(rest) == `` {
		return ``, ``, false
	}
	switch strings.ToLower(first) {
	case `a`, `an`, `the`, `some`:
		return first, rest, true
	}
	return ``, ``, false
}
