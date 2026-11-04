package messaging

import (
	"regexp"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// itemNameAfterWord matches "the", "your" or "their" directly before an item's
// open tag, and the name text the tag opens, up to the next tag.
var itemNameAfterWord = regexp.MustCompile(`\b([Tt]he|[Yy]our|[Tt]heir) (<ansi fg="item(?:name)?(?:-[A-Za-z0-9_-]+)?">)([^<]+)`)

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
		if first, _, ok := strings.Cut(name, ` `); ok && strings.ToLower(word) == `the` && strings.HasSuffix(first, `'s`) {
			return tag + name
		}
		return match
	})
}
