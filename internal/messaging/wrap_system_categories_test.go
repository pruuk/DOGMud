package messaging

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// wrappedSystemCategories are the seven categories that once relied on each
// sender to fold a long line, and now wrap in the pipeline.
var wrappedSystemCategories = []Category{
	CategorySystem, CategoryWarning, CategoryError, CategoryRefusal,
	CategoryUsage, CategoryGroup, CategoryBroadcast,
}

// achievementExplainer is the line internal/hooks/Achievements_Poll.go sends
// on a player's first achievement, the line a player saw cut mid-word.
const achievementExplainer = `<ansi fg="white">(You just earned your first </ansi>` +
	`<ansi fg="yellow-bold">achievement</ansi><ansi fg="white">, a milestone you unlock ` +
	`simply by playing. Type </ansi><ansi fg="command">achievements</ansi>` +
	`<ansi fg="white"> anytime to see the ones you've earned and what else you can chase.)</ansi>`

func TestAchievementLineWrapsAtTheReadersWidthWithNoWordBroken(t *testing.T) {
	want := visibleWords(achievementExplainer)
	for _, width := range []int{30, 42, 80} {
		t.Run(fmt.Sprintf("width %d", width), func(t *testing.T) {
			got := RenderForRecipient(RenderInput{
				Category: CategorySystem, Text: achievementExplainer,
				Channel: ChannelAudio, LineWidth: width,
			})
			lines := strings.Split(got, "\n")
			require.Greater(t, len(lines), 1, "the explainer is over 80 columns and must fold: %q", got)
			for i, line := range lines {
				require.LessOrEqual(t, columnsOf(line), width, "line %d: %q", i+1, line)
			}
			require.Equal(t, want, visibleWords(got), "a word was broken or lost")
		})
	}
}

func TestSevenCategoriesWrapAtTheReadersWidth(t *testing.T) {
	const prose = "The warden studies your papers for a long while, then slides them " +
		"back across the scarred table and tells you plainly that nobody leaves " +
		"Thornwall tonight without the magistrate's seal upon every single page."
	want := visibleWords(prose)

	for _, cat := range wrappedSystemCategories {
		for _, width := range []int{30, 42, 80} {
			t.Run(fmt.Sprintf("%s at %d", cat, width), func(t *testing.T) {
				got := RenderForRecipient(RenderInput{
					Category: cat, Text: prose, Channel: ChannelAudio, LineWidth: width,
				})
				lines := strings.Split(got, "\n")
				require.Greater(t, len(lines), 1, "a %d column line must fold at %d: %q", len(prose), width, got)
				for i, line := range lines {
					require.LessOrEqual(t, columnsOf(line), width, "line %d: %q", i+1, line)
				}
				require.Equal(t, want, visibleWords(got), "a word was broken or lost")
			})
		}
	}
}

// exactly80 is one line of exactly 80 visible columns.
var exactly80 = "Row at the edge" + strings.Repeat(" ", 45) + strings.Repeat("-", 20)

func TestMultiLineTableOfShortLinesPassesUnchangedAt80(t *testing.T) {
	rows := []string{
		"Item              Qty   Value",
		"---------------   ---   -----",
		"  Healing draught     3     45g",
		"  Iron sword          1    120g",
		exactly80,
		" ╔══════════╗",
		" ║ Notice   ║",
		" ╚══════════╝",
	}
	require.Equal(t, 80, columnsOf(exactly80), "the fixture must sit on the width")

	for _, sep := range []struct{ name, text string }{{"LF", "\n"}, {"CRLF", "\r\n"}} {
		table := strings.Join(rows, sep.text)
		for _, cat := range wrappedSystemCategories {
			t.Run(fmt.Sprintf("%s %s", cat, sep.name), func(t *testing.T) {
				got := RenderForRecipient(RenderInput{
					Category: cat, Text: table, Channel: ChannelAudio, LineWidth: 80,
				})
				require.Equal(t, table, ansiTagPattern.ReplaceAllString(got, ""),
					"every line at or under the width must arrive byte for byte")
			})
		}
	}
}

func TestWrapAnsiLeavesALineThatFitsByteForByte(t *testing.T) {
	for _, in := range []string{
		"  two  leading, a   run,  and   trailing  ",
		"Item              Qty   Value",
		"tagged  <ansi fg=\"red\">  padding  </ansi>  kept",
		"",
		"one\n\n  three   after a blank",
		"crlf  row\r\n  next   row",
	} {
		require.Equal(t, in, WrapAnsi(in, 80))
	}
}

// A tag open across a hard break is closed and reopened around it, so every
// line stays balanced, and the padding inside still arrives untouched.
func TestWrapAnsiFittingLinesKeepTheirTagsBalanced(t *testing.T) {
	got := WrapAnsi(`<ansi fg="system">Item   Qty`+"\n"+`  Row    3</ansi>`, 80)
	require.Equal(t, `<ansi fg="system">Item   Qty</ansi>`+"\n"+`<ansi fg="system">  Row    3</ansi>`, got)
}

func TestWrapAnsiEmbeddedNewlineIsAHardBreak(t *testing.T) {
	require.Equal(t, "one\ntwo", WrapAnsi("one\ntwo", 80))
	got := WrapAnsi("a line long enough to fold at this width\nnext", 12)
	require.Equal(t, "next", strings.Split(got, "\n")[len(strings.Split(got, "\n"))-1],
		"the break survives a fold above it")
}

// numberedWords is count words of width columns each, numbered so the
// normalizer's duplicate-word stage has nothing to collapse.
func numberedWords(start, count int) []string {
	words := make([]string, 0, count)
	for i := 0; i < count; i++ {
		words = append(words, fmt.Sprintf("w%02d", start+i))
	}
	return words
}

// A sender's pre-wrap leaves CRLF in the text, and the wrapper used to count
// the carriage return as a column on the second pass and strand a word.
func TestNoCarriageReturnIsCountedAsAColumn(t *testing.T) {
	// Twenty three-column words and a one-column pad: exactly 80 columns.
	line80 := func(start int) string { return "x" + strings.Join(numberedWords(start, 20), " ") }
	require.Equal(t, 80, columnsOf(line80(0)))

	t.Run("a fitting line keeps its CRLF", func(t *testing.T) {
		in := line80(0) + "\r\n" + line80(20)
		got := RenderForRecipient(RenderInput{
			Category: CategoryBroadcast, Text: in, Channel: ChannelAudio, LineWidth: 80,
		})
		require.Equal(t, in, ansiTagPattern.ReplaceAllString(got, ""))
	})

	t.Run("a fold above a CRLF strands nothing", func(t *testing.T) {
		in := line80(0) + " " + line80(20) + "\r\ntail"
		lines := strings.Split(WrapAnsi(in, 80), "\n")
		require.Len(t, lines, 3, "the last word of the second row must stay on it: %q", lines)
		for _, line := range lines {
			require.LessOrEqual(t, columnsOf(strings.TrimRight(line, "\r")), 80, "%q", line)
		}
	})

	t.Run("a pre-wrapped text is stable under a second pass", func(t *testing.T) {
		once := WrapAnsi(line80(0)+" "+line80(20), 80)
		require.Equal(t, once, WrapAnsi(once, 80))
	})
}
