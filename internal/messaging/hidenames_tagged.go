package messaging

import (
	"regexp"
	"strings"
)

// identityTagPattern matches one whole identity tag and captures its content:
// a player, mob or pet name as characters.FormattedName.String renders it.
var identityTagPattern = regexp.MustCompile(`<ansi fg="(?:(?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">([^<]*)</ansi>`)

// dupIndexSuffix matches the duplicate index FormattedName.String appends to a
// mob's name in a room holding more than one of them.
var dupIndexSuffix = regexp.MustCompile(` #\d+$`)

// adjectiveSpan matches the adjective list FormattedName.String prints right
// after an identity tag: a space, then a black-bold span holding a
// parenthesised list such as "(dead)" or "(♥friend|hidden)". It goes with the
// name it describes; "something (dead)" would tell a blind reader what they
// could not see.
//
// CompileAdjectiveSwaps (internal/characters/formattedname.go) runs each
// adjective through colorpatterns.ApplyColorPattern, which wraps every rune
// in its own colour tag, so the body of the parentheses is nested markup, not
// plain text: "(<ansi fg=\"52\">d</ansi><ansi fg=\"88\">e</ansi>...)". The
// pattern admits one level of such single-purpose nested colour tags and
// nothing else. No other producer emits a black-bold parenthesised span
// directly after an identity tag: items.go:501 follows an item tag,
// broadcast.go:16 precedes the mob tag, and search.go builds its span inside
// the name tag.
var adjectiveSpan = regexp.MustCompile(`^ <ansi fg="black-bold">\((?:[^<]|<ansi fg="[^"]*">[^<]*</ansi>)*\)</ansi>`)

// hideTaggedName replaces a whole identity tag whose content is this name,
// ignoring case and any duplicate index, and one adjective span directly
// after it.
//
// WHY CASE-INSENSITIVE HERE AND NOWHERE ELSE. An identity tag's content is
// always a creature's name, so matching loosely inside one cannot hide an
// ordinary word. Outside a tag it can: mob names include "guard", and prose
// says "you guard against". The forms genuinely differ: authored mob names are
// lowercase ("skeleton"), FormattedName.String title-cases them for display
// and may append " #2", and the channel defence triad prints that display form
// while the verb's own lines print the raw name. Without this, a blind
// attacker read the mob's name in the defence line.
func hideTaggedName(text, name, word string) string {
	matches := identityTagPattern.FindAllStringSubmatchIndex(text, -1)
	if matches == nil {
		return text
	}
	var b strings.Builder
	last := 0
	for _, m := range matches {
		inner := text[m[2]:m[3]]
		if !strings.EqualFold(dupIndexSuffix.ReplaceAllString(inner, ""), name) {
			continue
		}
		shown := word
		if atSentenceStart(text, m[0]) {
			shown = strings.ToUpper(word[:1]) + word[1:]
		}
		b.WriteString(text[last:m[0]])
		b.WriteString(`<ansi fg="combat-anon">`)
		b.WriteString(shown)
		b.WriteString(`</ansi>`)
		last = m[1]
		if adj := adjectiveSpan.FindStringIndex(text[last:]); adj != nil {
			last += adj[1]
		}
	}
	if last == 0 {
		return text
	}
	b.WriteString(text[last:])
	return b.String()
}
