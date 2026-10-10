package usercommands

import (
	"testing"
	"unicode/utf8"

	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
)

// #253 follow-up: a padded admin column cut its text with "…", one rune, so
// the column held in UTF-8 but an ASCII client, which reads "...", saw the
// row pushed two columns wide. The cut now ends in "..." itself, three runes
// short, and holds in both charsets.
func TestAdminColumnTruncation_EndsInThreeDotsAndHoldsTheWidth(t *testing.T) {
	for name, cut := range map[string]func(string, int) string{
		"bountyTruncate": bountyTruncate,
		"factTruncate":   factTruncate,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "short", cut("short", 10), "text that fits is untouched")
			assert.Equal(t, "exactlyten", cut("exactlyten", 10), "text at the width is untouched")

			got := cut("abcdefghijklmnop", 10)
			assert.Equal(t, "abcdefg...", got)
			assert.Equal(t, 10, utf8.RuneCountInString(got))
			assert.Equal(t, got, util.ConvertToAscii(got), "the cut is plain ASCII")

			// Cut by rune, never inside one.
			got = cut("ÄÖÜäöüßÄÖÜäöüß", 10)
			assert.True(t, utf8.ValidString(got))
			assert.Equal(t, 10, utf8.RuneCountInString(got))

			assert.Equal(t, "abc", cut("abcdefgh", 3), "a width of three or less has no room for dots")
		})
	}
}

// The faction and opinion admin cuts end in "~" and used to cut by byte, which
// could split a multi-byte character and emit invalid UTF-8.
func TestAdminColumnTruncation_FactionAndOpinionCutByRune(t *testing.T) {
	for name, cut := range map[string]func(string, int) string{
		"factionTruncate": factionTruncate,
		"opinionTruncate": opinionTruncate,
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "short", cut("short", 10))
			assert.Equal(t, "exactlyten", cut("exactlyten", 10))
			assert.Equal(t, "abcdefghi~", cut("abcdefghijklmnop", 10))

			// Eight runes fit by rune count though they are sixteen bytes.
			assert.Equal(t, "ÄÖÜäöüßÄ", cut("ÄÖÜäöüßÄ", 8), "fits by rune, not by byte")

			got := cut("ÄÖÜäöüßÄÖÜäöüß", 10)
			assert.True(t, utf8.ValidString(got))
			assert.Equal(t, 10, utf8.RuneCountInString(got))
			assert.Equal(t, "ÄÖÜäöüßÄÖ~", got)

			assert.Equal(t, "", cut("abc", 0))
		})
	}
}
