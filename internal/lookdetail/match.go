package lookdetail

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/GoMudEngine/GoMud/internal/baubles"
)

// Bounds on what may be looked at: a word or a short phrase, as a player
// would type the name of something in the room's description.
const (
	minPhraseRunes = 3
	maxPhraseRunes = 40
	maxPhraseWords = 4
	maxExcerpt     = 300
)

// stopwords are never worth a closer look: "look at it", "look at here".
var stopwords = map[string]bool{
	`and`: true, `are`: true, `but`: true, `for`: true, `from`: true, `her`: true, `here`: true,
	`him`: true, `his`: true, `its`: true, `not`: true, `one`: true, `out`: true, `that`: true,
	`the`: true, `them`: true, `then`: true, `there`: true, `they`: true, `this`: true, `with`: true,
	`you`: true, `your`: true, `room`: true, `around`: true, `everything`: true, `nothing`: true,
	`something`: true, `anything`: true, `all`: true, `was`: true, `has`: true, `have`: true,
}

// Phrase normalises what the player asked to look at, or reports that it is
// not something a closer look can be about: lower case, single spaces, a
// leading article dropped; 3 to 40 characters, at most 4 words, letters,
// spaces, apostrophes and hyphens only, and not a bare stopword.
func Phrase(lookAt string) (string, bool) {
	p := strings.Join(strings.Fields(strings.ToLower(lookAt)), ` `)
	for _, article := range []string{`the `, `a `, `an `, `some `, `my `} {
		p = strings.TrimPrefix(p, article)
	}
	n := len([]rune(p))
	if n < minPhraseRunes || n > maxPhraseRunes || len(strings.Fields(p)) > maxPhraseWords || stopwords[p] {
		return ``, false
	}
	for _, c := range p {
		if !unicode.IsLetter(c) && c != ' ' && c != '\'' && c != '-' {
			return ``, false
		}
	}
	return p, true
}

// forms are the spellings of phrase a description may use: as typed, and
// the plain plural or singular of its last word.
func forms(phrase string) []string {
	out := []string{phrase}
	add := func(s string) {
		if len([]rune(s)) >= minPhraseRunes {
			out = append(out, s)
		}
	}
	switch {
	case strings.HasSuffix(phrase, `ies`):
		add(strings.TrimSuffix(phrase, `ies`) + `y`)
	case strings.HasSuffix(phrase, `es`):
		add(strings.TrimSuffix(phrase, `es`))
		add(strings.TrimSuffix(phrase, `s`))
	case strings.HasSuffix(phrase, `s`):
		add(strings.TrimSuffix(phrase, `s`))
	case strings.HasSuffix(phrase, `y`):
		add(strings.TrimSuffix(phrase, `y`) + `ies`)
		add(phrase + `s`)
	default:
		add(phrase + `s`)
		add(phrase + `es`)
	}
	return out
}

var sentenceEnd = regexp.MustCompile(`[.!?]+(\s|$)`)

// Find reports whether phrase names something in description: one of its
// forms, as whole words, ignoring case and markup. shown is the words as the
// description has them (lower case), excerpt the sentence they are in.
func Find(description string, phrase string) (shown string, excerpt string, ok bool) {
	text := baubles.PlainText(description)
	for _, f := range forms(phrase) {
		words := strings.Fields(f)
		for i := range words {
			words[i] = regexp.QuoteMeta(words[i])
		}
		re := regexp.MustCompile(`(?i)\b` + strings.Join(words, `\s+`) + `\b`)
		loc := re.FindStringIndex(text)
		if loc == nil {
			continue
		}
		shown = strings.ToLower(strings.Join(strings.Fields(text[loc[0]:loc[1]]), ` `))
		return shown, sentenceAround(text, loc[0], loc[1]), true
	}
	return ``, ``, false
}

// sentenceAround is the sentence of text holding [start, end).
func sentenceAround(text string, start int, end int) string {
	from := 0
	for _, m := range sentenceEnd.FindAllStringIndex(text[:start], -1) {
		from = m[1]
	}
	to := len(text)
	if m := sentenceEnd.FindStringIndex(text[end:]); m != nil {
		to = end + m[1]
	}
	s := strings.TrimSpace(text[from:to])
	if r := []rune(s); len(r) > maxExcerpt {
		s = string(r[:maxExcerpt]) + `...`
	}
	return s
}
