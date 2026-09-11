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

// identityOpenTag matches exactly one opening tag of a player, mob or pet
// name. The same aliases Anonymize recognises.
var identityOpenTag = regexp.MustCompile(`^<ansi fg="(?:(?:username|mobname)(?:-[A-Za-z0-9_-]+)?|petname)">$`)

// identityCloseAfterName matches what may follow a name inside its identity
// tag: an optional duplicate index (" #2", see characters.FormattedName) and
// the closing tag.
var identityCloseAfterName = regexp.MustCompile(`^(?: #\d+)?</ansi>`)

// HideNames replaces each of names in text with what a reader who cannot make
// that party out perceives: "a figure" when they see shapes only, "something"
// when they see nothing. Clear sight returns text unchanged.
//
// A name matches as an EXACT substring, longest name first, only as a whole
// word ("Kesh" does not match inside "Keshara" or "Kesh_Two"), and never
// inside tag markup, so a name such as "green" cannot rewrite
// `<ansi fg="green">` and a later name cannot rewrite a replacement already
// made. When a match is the whole content of an identity tag, the tag goes with
// it, so the output never nests a combat-anon tag inside a name tag. The
// replacement is capitalized at the start of a sentence, looking through tags,
// so "⚡ SWEEP! Kesh dodges" reads "⚡ SWEEP! Something dodges".
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
		if insideTag(text, start) || !standsAsWord(text, start, end, name) {
			_, size := utf8.DecodeRuneInString(text[start:])
			from = start + size
			continue
		}
		cutStart, cutEnd := start, end
		if strings.HasSuffix(text[:start], `">`) {
			if open := strings.LastIndexByte(text[:start], '<'); open >= 0 && identityOpenTag.MatchString(text[open:start]) {
				if closing := identityCloseAfterName.FindStringIndex(text[end:]); closing != nil {
					cutStart, cutEnd = open, end+closing[1]
				}
			}
		}
		shown := word
		if atSentenceStart(text, cutStart) {
			shown = strings.ToUpper(word[:1]) + word[1:]
		}
		replacement := `<ansi fg="combat-anon">` + shown + `</ansi>`
		text = text[:cutStart] + replacement + text[cutEnd:]
		from = cutStart + len(replacement)
	}
	return text
}

// insideTag reports whether byte pos falls inside tag markup: the nearest '<'
// before it is later than the nearest '>'.
func insideTag(text string, pos int) bool {
	return strings.LastIndexByte(text[:pos], '<') > strings.LastIndexByte(text[:pos], '>')
}

// standsAsWord reports whether the match at [start, end) is a whole word: a
// name that begins or ends with a name character may not touch another one.
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

// isNameRune reports whether r can be part of a character name. Underscore is a
// legal name character, so it joins words here too.
func isNameRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' }

// atSentenceStart reports whether text placed at byte pos begins a sentence,
// scanning backwards through spaces and tags: the start of the text, the start
// of a line, or a sentence end followed by at least one space.
func atSentenceStart(text string, pos int) bool {
	sawSpace := false
	i := pos
	for i > 0 {
		c := text[i-1]
		switch {
		case c == ' ' || c == '\t':
			sawSpace = true
			i--
		case c == '\n' || c == '\r':
			return true
		case c == '>':
			open := strings.LastIndexByte(text[:i-1], '<')
			if open < 0 {
				return false
			}
			i = open
		default:
			return sawSpace && (c == '.' || c == '!' || c == '?')
		}
	}
	return true
}
