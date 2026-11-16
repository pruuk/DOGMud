package baubles

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// Text validation for a model reply. The strict JSON schema fixes the SHAPE
// of the answer; everything the schema cannot say is checked here, and any
// failure sends the find down the fallback path (the corpus, or a generic
// trinket) instead of into the world.

const (
	maxNameLen        = 40
	minNameWords      = 1
	maxNameWords      = 6
	maxNameSimpleLen  = 20
	minDescriptionLen = 20
	maxDescriptionLen = 400
	maxMaterialLen    = 30
)

var (
	tagRE        = regexp.MustCompile(`<[^>]*>`)
	ansiRE       = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	nameSimpleRE = regexp.MustCompile(`^[a-z]{2,20}$`)
	// linkRE is text that reads as a link: a scheme, a www., or a
	// domain-shaped word (evil.com, shop.co/x). Matched on lower-cased text.
	linkRE = regexp.MustCompile(`://|www\.|[a-z0-9-]+\.[a-z]{2,6}(/|\b)`)
)

// reservedNouns are keywords the game's real items answer to. A bauble whose
// one-word keyword is one of these would hijack `get key`, `drink potion`,
// `wield sword` and the like, so the keyword is replaced with "trinket"
// (the name itself may keep the word: "Bent Iron Key" is fine to read).
var reservedNouns = map[string]bool{
	`key`: true, `sword`: true, `dagger`: true, `knife`: true, `axe`: true, `mace`: true,
	`bow`: true, `arrow`: true, `bolt`: true, `spear`: true, `staff`: true, `shield`: true,
	`potion`: true, `elixir`: true, `scroll`: true, `book`: true, `map`: true, `note`: true,
	`letter`: true, `token`: true, `coin`: true, `gold`: true, `ring`: true, `amulet`: true,
	`bread`: true, `food`: true, `water`: true, `ale`: true, `wine`: true, `rope`: true,
	`torch`: true, `lantern`: true, `lockpick`: true, `pick`: true, `corpse`: true,
	`bag`: true, `backpack`: true, `chest`: true, `crate`: true, `door`: true,
	`herb`: true, `ore`: true, `gem`: true, `bauble`: true,
	// Natural finds (prompt version 3) must not take the keywords of real
	// materials, quest items and props: the Reckoning Bone (quest 58), the
	// weighted and river stones, and crafting materials (a black pearl, a
	// shrimp shell, a leviathan tooth, a geode, amber, claws and fangs).
	`bone`: true, `stone`: true, `shell`: true, `tooth`: true, `geode`: true,
	`amber`: true, `pearl`: true, `claw`: true, `fang`: true, `crystal`: true,
}

// ErrUnusableReply wraps every reason a reply cannot be used.
var ErrUnusableReply = errors.New(`unusable bauble reply`)

// typographyFold turns the typography a model reaches for into ASCII
// (owner ruling 15): curly quotes to ' and ", en and em dashes to -, the
// ellipsis to ... (NFKC already does the ellipsis; it is listed so the rule
// reads whole). It also drops < and > (markup leftovers) and turns a
// backtick into '.
var typographyFold = strings.NewReplacer(
	`<`, ``, `>`, ``, "`", `'`,
	"\U00002018", `'`, "\U00002019", `'`, "\U0000201C", `"`, "\U0000201D", `"`,
	"\U00002013", `-`, "\U00002014", `-`, "\U00002026", `...`,
)

// cleanLine folds compatibility forms (NFKC, so fullwidth markup and
// letters become plain ones before anything else looks), strips markup,
// turns control characters and every Unicode space into a plain space,
// drops format, private-use, surrogate and combining characters, the line
// and paragraph separators and the Hangul fillers (which render as
// nothing), folds curly quotes, dashes and the ellipsis to ASCII
// (typographyFold), and collapses whitespace.
func cleanLine(s string) string {
	s = norm.NFKC.String(s)
	s = ansiRE.ReplaceAllString(s, ``)
	s = tagRE.ReplaceAllString(s, ``)
	s = strings.Map(cleanRune, s)
	s = typographyFold.Replace(s)
	return strings.Join(strings.Fields(s), ` `)
}

// cleanRune is cleanLine's per-character rule: -1 drops it.
func cleanRune(r rune) rune {
	switch {
	case r == '\U00002028', r == '\U00002029', r == '\U0000115F', r == '\U00001160', r == '\U00003164', r == '\U0000FFA0':
		return -1
	case unicode.IsControl(r), unicode.Is(unicode.Zs, r):
		return ' '
	case unicode.In(r, unicode.Cf, unicode.Co, unicode.Cs, unicode.Mn):
		return -1
	}
	return r
}

// quoteShort quotes at most 60 runes of s for an error that may be logged.
func quoteShort(s string) string {
	const most = 60
	if utf8.RuneCountInString(s) > most {
		s = string([]rune(s)[:most]) + `...`
	}
	return strconv.Quote(s)
}

// CleanReply validates and tidies the text of a model reply. It returns the
// cleaned reply, or an error wrapping ErrUnusableReply. Numbers are not
// touched here; ApplyLimits clamps those.
func CleanReply(r Reply) (Reply, error) {
	bad := func(format string, a ...any) (Reply, error) {
		return Reply{}, fmt.Errorf(`%w: `+format, append([]any{ErrUnusableReply}, a...)...)
	}

	r.Name = cleanLine(r.Name)
	r.NameSimple = strings.ToLower(cleanLine(r.NameSimple))
	r.Description = cleanLine(r.Description)
	r.Material = strings.ToLower(cleanLine(r.Material))

	words := strings.Fields(r.Name)
	if r.Name == `` || utf8.RuneCountInString(r.Name) > maxNameLen || len(words) < minNameWords || len(words) > maxNameWords {
		return bad(`name %s`, quoteShort(r.Name))
	}
	if strings.IndexFunc(r.Name, unicode.IsDigit) >= 0 {
		return bad(`name has digits: %s`, quoteShort(r.Name))
	}
	if authoredName(r.Name) {
		return bad(`name is a real item's: %s`, quoteShort(r.Name))
	}
	if n := utf8.RuneCountInString(r.Description); n < minDescriptionLen || n > maxDescriptionLen {
		return bad(`description length %d`, n)
	}
	if utf8.RuneCountInString(r.Material) > maxMaterialLen {
		r.Material = ``
	}
	for _, f := range [...]struct{ field, text string }{{`name`, r.Name}, {`description`, r.Description}, {`material`, r.Material}} {
		if linkRE.MatchString(strings.ToLower(f.text)) {
			return bad(`%s reads as a link: %s`, f.field, quoteShort(f.text))
		}
	}

	// The keyword must be one plain lowercase word that no real item
	// answers to (reservedNouns, and every loaded item's own keyword and
	// head noun: items.AuthoredKeyword). Otherwise the name's own last word;
	// then its other words, last to first, of four letters or more ("silver"
	// for a Tarnished Silver Locket, when a real locket exists); then
	// "trinket". Players can still use any word of the name.
	if !usableKeyword(r.NameSimple) {
		r.NameSimple = genericNameSimple
		for i, word := range slices.Backward(words) {
			w := strings.ToLower(strings.TrimSuffix(strings.TrimSuffix(word, `'s`), `'`))
			if (i == len(words)-1 || len(w) >= 4) && usableKeyword(w) {
				r.NameSimple = w
				break
			}
		}
	}
	return r, nil
}

// usableKeyword is a keyword a bauble may take.
func usableKeyword(w string) bool {
	return nameSimpleRE.MatchString(w) && len(w) <= maxNameSimpleLen && !reservedNouns[w] && !authoredKeyword(w)
}

// authoredKeyword is items.AuthoredKeyword. A variable for tests.
var authoredKeyword = items.AuthoredKeyword

// authoredName is items.AuthoredName. A variable for tests.
var authoredName = items.AuthoredName

// PlainText is cleanLine: room text sent in a prompt gets the same NFKC,
// typography fold and invisible/format character drops as model output,
// with markup, colour codes and control characters removed and whitespace
// collapsed.
func PlainText(s string) string {
	return cleanLine(s)
}
