package messaging

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/species"
)

// WeaponWord is what a reader below full sight reads in place of a weapon's
// name in a combat line (owner ruling R8, 2026-10-08): "something slashes you
// with their weapon", "the fumbled weapon".
const WeaponWord = "weapon"

// itemTagOpen matches the opening tag of an item name in narration. Combat
// templates wrap {itemname}, {weapon} and {attack} in fg="item"; the shoot
// verb wraps its weapon in fg="itemname" (usercommands/shoot.go).
var itemTagOpen = regexp.MustCompile(`<ansi fg="item(?:name)?">`)

// anyAnsiTag matches any opening or closing ansi tag, for reading a tag body
// as plain text.
var anyAnsiTag = regexp.MustCompile(`<ansi[^>]*>|</ansi>`)

// HideWeapons hides the weapons in a combat line from a reader at d. Below
// SightFull every item-tagged name becomes WeaponWord, except:
//
//   - a natural weapon: "fists" or any species' UnarmedName. Templates tag
//     {itemname} whether it holds an item or a body part, and a body part is
//     not a weapon a reader learns anything from (spec F2: unarmed names stay);
//   - a name in keep, which a participant passes for their own gear: the
//     reader knows what is in their own hand, as `look` by touch does (#218).
//
// A preceding "an" becomes "a", because the pipeline's a/an stage runs
// before the sight stage and has already agreed the article with the real
// name ("an Iron Longsword").
//
// Clear sight returns text unchanged. Untagged item names are not touched;
// spec F2 tags the combat templates that left a weapon token bare.
func HideWeapons(text string, d SightDecision, keep []string) string {
	if d == SightFull || text == "" || !strings.Contains(text, `<ansi fg="item`) {
		return text
	}
	var b strings.Builder
	last := 0
	for _, loc := range itemTagOpen.FindAllStringIndex(text, -1) {
		if loc[0] < last {
			continue // nested inside a tag already replaced
		}
		end := closingAnsiEnd(text, loc[1])
		if end < 0 {
			continue
		}
		plain := strings.TrimSpace(anyAnsiTag.ReplaceAllString(text[loc[1]:end], ""))
		if plain == "" || isNaturalWeapon(plain) || namedIn(plain, keep) {
			continue
		}
		word := WeaponWord
		if atSentenceStart(text, loc[0]) {
			word = strings.ToUpper(word[:1]) + word[1:]
		}
		b.WriteString(articleBeforeConsonant(text[last:loc[0]]))
		b.WriteString(`<ansi fg="combat-anon">` + word + `</ansi>`)
		last = end
	}
	if last == 0 {
		return text
	}
	b.WriteString(text[last:])
	return b.String()
}

// closingAnsiEnd returns the index just past the </ansi> that closes a tag
// whose opening ends at from, counting nested ansi tags (a display name can
// carry a quest star or an adjective span inside its item tag). It returns -1
// for an unclosed tag.
func closingAnsiEnd(text string, from int) int {
	depth := 1
	for i := from; i < len(text); {
		switch {
		case strings.HasPrefix(text[i:], "</ansi>"):
			depth--
			i += len("</ansi>")
			if depth == 0 {
				return i
			}
		case strings.HasPrefix(text[i:], "<ansi"):
			depth++
			i += len("<ansi")
		default:
			i++
		}
	}
	return -1
}

// articleBeforeConsonant rewrites a trailing article "an " or "An " to "a "
// or "A ", for the consonant of WeaponWord.
func articleBeforeConsonant(prefix string) string {
	for _, art := range []string{"an ", "An "} {
		if !strings.HasSuffix(prefix, art) {
			continue
		}
		start := len(prefix) - len(art)
		if start > 0 {
			c := prefix[start-1]
			if c != ' ' && c != '>' && c != '\n' {
				return prefix // "Elan " is not an article
			}
		}
		return prefix[:start] + art[:1] + " "
	}
	return prefix
}

// isNaturalWeapon reports whether plain is a body part rather than an item:
// the "fists" default combat falls back to, or a species' UnarmedName.
func isNaturalWeapon(plain string) bool {
	if strings.EqualFold(plain, "fists") {
		return true
	}
	for _, s := range species.GetAllSpecies() {
		if s.UnarmedName != "" && strings.EqualFold(plain, s.UnarmedName) {
			return true
		}
	}
	return false
}

// namedIn reports whether plain matches one of names, ignoring case and tags.
func namedIn(plain string, names []string) bool {
	for _, n := range names {
		if n != "" && strings.EqualFold(plain, strings.TrimSpace(anyAnsiTag.ReplaceAllString(n, ""))) {
			return true
		}
	}
	return false
}

// isCombatNarration reports whether cat narrates a fight: swings, defences,
// grapples and the special-move verbs. The pipeline hides weapons from a
// shapes-only reader of these categories only (spec F2: combat paths).
func isCombatNarration(cat Category) bool {
	switch cat {
	case CategoryHitMelee, CategoryHitBlunt, CategoryHitNaturalSharp,
		CategoryHitRanged, CategoryHitCaster, CategoryHitUnarmed,
		CategoryDodge, CategoryParry, CategoryBlock,
		CategoryGrappleFlow, CategoryGrappleHigh, CategorySubmission,
		CategorySurpriseAttack, CategoryKick, CategoryTrip, CategoryBash:
		return true
	}
	return false
}
