package messaging

import (
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// NoName marks a side of an Audience with nobody on it: a salvaged corpse, a
// thrown item landing in a crowd. It is the empty string, spelled out so the
// root Audience guard can tell a considered absence from a forgotten name, the
// same idea as NoLine.
const NoName = ""

// identityOpenTag matches a player, mob or pet name's opening tag at the very
// end of the text before a name. The same aliases Anonymize recognises.
var identityOpenTag = regexp.MustCompile(`<ansi fg="(?:(?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">$`)

// identityCloseAfterName matches what may follow a name inside its identity
// tag: an optional duplicate index (" #2", see characters.FormattedName) and
// the closing tag.
var identityCloseAfterName = regexp.MustCompile(`^(?: #\d+)?</ansi>`)

// HideNames replaces each of names in text with what a reader who cannot make
// that party out perceives: "a figure" when they see shapes only, "something"
// when they see nothing. Clear sight returns text unchanged.
//
// A name matches as an EXACT substring, longest name first, and only as a whole
// word: "Kesh" does not match inside "Keshara". When a match is the whole
// content of an identity tag, the tag goes with it, so the output never nests
// a combat-anon tag inside a name tag. The replacement is capitalized at the
// start of a sentence, looking through ANSI tags, so "⚡ SWEEP! Kesh dodges"
// reads "⚡ SWEEP! Something dodges".
func HideNames(text string, names []string, d SightDecision) string {
	if d == SightFull || text == "" {
		return text
	}
	word := "something"
	if d == SightShapes {
		word = "a figure"
	}
	ordered := make([]string, 0, len(names))
	for _, n := range names {
		if n != NoName {
			ordered = append(ordered, n)
		}
	}
	sort.SliceStable(ordered, func(i, j int) bool { return len(ordered[i]) > len(ordered[j]) })
	for _, name := range ordered {
		text = hideOneName(text, name, word)
	}
	return text
}

func hideOneName(text, name, word string) string {
	from := 0
	for from <= len(text) {
		idx := strings.Index(text[from:], name)
		if idx < 0 {
			return text
		}
		start := from + idx
		end := start + len(name)
		if !standsAsWord(text, start, end, name) {
			_, size := utf8.DecodeRuneInString(text[start:])
			from = start + size
			continue
		}
		cutStart, cutEnd := start, end
		if open := identityOpenTag.FindStringIndex(text[:start]); open != nil {
			if closing := identityCloseAfterName.FindStringIndex(text[end:]); closing != nil {
				cutStart, cutEnd = open[0], end+closing[1]
			}
		}
		shown := word
		if atSentenceStart(text[:cutStart]) {
			shown = strings.ToUpper(word[:1]) + word[1:]
		}
		replacement := `<ansi fg="combat-anon">` + shown + `</ansi>`
		text = text[:cutStart] + replacement + text[cutEnd:]
		from = cutStart + len(replacement)
	}
	return text
}

// standsAsWord reports whether the match at [start, end) is a whole word: a
// name that begins or ends with a letter or digit may not touch another one.
func standsAsWord(text string, start, end int, name string) bool {
	first, _ := utf8.DecodeRuneInString(name)
	if isNameRune(first) && start > 0 {
		prev, _ := utf8.DecodeLastRuneInString(text[:start])
		if isNameRune(prev) {
			return false
		}
	}
	last, _ := utf8.DecodeLastRuneInString(name)
	if isNameRune(last) && end < len(text) {
		next, _ := utf8.DecodeRuneInString(text[end:])
		if isNameRune(next) {
			return false
		}
	}
	return true
}

func isNameRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// atSentenceStart reports whether text placed after prefix begins a sentence:
// only tags and spaces before it, or a sentence end followed by a space.
func atSentenceStart(prefix string) bool {
	plain := ansiTagPattern.ReplaceAllString(prefix, "")
	trimmed := strings.TrimRight(plain, " ")
	if trimmed == "" {
		return true
	}
	if len(trimmed) == len(plain) {
		return false
	}
	switch trimmed[len(trimmed)-1] {
	case '.', '!', '?':
		return true
	}
	return false
}
