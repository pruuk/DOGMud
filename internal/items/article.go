package items

import "strings"

// IndefiniteArticle returns the article a line puts before one item's name:
// "some" for a name that reads as plural ("Iron Lockpicks", "Leather Boots"),
// otherwise "an" before a vowel and "a" before anything else, the letter rule
// the messaging normalizer applies. A sale line once read "You sell an Iron
// Lockpicks".
//
// A name reads as plural when its last word ends in "s" but not "ss", "us"
// or "is" (Cutlass, Abacus and Aegis stay singular). Item data has no plural
// field; every shipped name this rule calls plural is one (2026-10-17).
func IndefiniteArticle(name string) string {
	words := strings.Fields(name)
	if len(words) == 0 {
		return `a`
	}
	last := strings.ToLower(words[len(words)-1])
	if strings.HasSuffix(last, `s`) && !strings.HasSuffix(last, `ss`) &&
		!strings.HasSuffix(last, `us`) && !strings.HasSuffix(last, `is`) {
		return `some`
	}
	switch strings.ToLower(words[0])[0] {
	case 'a', 'e', 'i', 'o', 'u':
		return `an`
	}
	return `a`
}
