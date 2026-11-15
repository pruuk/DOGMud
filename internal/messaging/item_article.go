package messaging

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// itemNameAfterWord matches "the", "this", "your", "their", "his", "her" or
// "its" directly before an
// item's open tag, and the name text the tag opens, up to the next tag.
var itemNameAfterWord = regexp.MustCompile(`\b([Tt]he|[Tt]his|[Yy]our|[Tt]heir|[Hh]is|[Hh]er|[Ii]ts) (<ansi fg="item(?:name)?(?:-[A-Za-z0-9_-]+)?">)([^<]+)`)

// DropDoubledItemArticles fixes a line that puts its own word before an item's
// name when the name cannot take it: "You show the A Folded Page" reads "You
// show the Folded Page", and "You look at the Soren's Iron Pin" reads "You look
// at Soren's Iron Pin". Item data keeps its names; about 75 sites format "the
// <item>", and this is the one place every tagged name passes, as with
// LowerMidSentenceNameArticles. Only a name inside an item tag changes.
func DropDoubledItemArticles(text string) string {
	if !strings.Contains(text, `<ansi fg="item`) {
		return text
	}
	return itemNameAfterWord.ReplaceAllStringFunc(text, func(match string) string {
		m := itemNameAfterWord.FindStringSubmatch(match)
		word, tag, name := m[1], m[2], m[3]
		if bare := items.NameWithoutArticle(name); bare != name {
			return word + ` ` + tag + bare
		}
		// A person's name takes neither "the" nor a possessive determiner:
		// "the Soren's Iron Pin", "gives you their Soren's Iron Pin". A trade
		// possessive ("the Warden's Lash") is not a proper name and keeps
		// its word. The flag decides, not the apostrophe (#799).
		switch strings.ToLower(word) {
		case `the`, `your`, `their`, `his`, `her`, `its`:
			if items.IsProperName(name) {
				return tag + name
			}
		}
		return match
	})
}
