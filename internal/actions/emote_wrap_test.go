package actions

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

var emoteWrapTags = regexp.MustCompile(`<[^>]*>`)

// The long Sly Tam idle emotes that showed the #430 defect.
var longMobEmotes = []string{
	"rolls a trinket across his knuckles and murmurs that he buys what others will not.",
	"fakes a wide yawn and stretches, hand passing briefly over a neighboring stool.",
	"turns to examine the ceiling with apparent fascination, one hand hanging loose at his side.",
}

// renderMobEmoteLine sends text through the pipeline as a mob emote at the
// reader's width and returns what the reader sees, ANSI tags stripped.
func renderMobEmoteLine(text string, width int) string {
	rendered := messaging.RenderForRecipient(messaging.RenderInput{
		Category:      messaging.CategoryMobEmote,
		Text:          text,
		Channel:       messaging.ChannelVisual,
		SightDecision: messaging.SightFull,
		LineWidth:     width,
	})
	return emoteWrapTags.ReplaceAllString(rendered, "")
}

// greedyLineCount is how many lines a single word-wrap of plain at width
// takes: the count a reader should see when the line is wrapped once.
func greedyLineCount(plain string, width int) int {
	lines, col := 1, 0
	for _, w := range strings.Fields(plain) {
		n := len([]rune(w))
		switch {
		case col == 0:
			col = n
		case col+1+n <= width:
			col += 1 + n
		default:
			lines++
			col = n
		}
	}
	return lines
}

// emoteWrapProblems lists what is wrong with a rendered mob emote at width:
// a stray carriage return, a line over width, or more lines than one wrap
// would make (the stranded-word symptom of wrapping twice).
func emoteWrapProblems(plain string, width int) []string {
	var out []string
	if strings.Contains(plain, "\r") {
		out = append(out, "carries a carriage return")
	}
	lines := strings.Split(strings.ReplaceAll(plain, "\r", ""), "\n")
	for _, line := range lines {
		if len([]rune(line)) > width {
			out = append(out, fmt.Sprintf("line wider than %d: %q", width, line))
		}
	}
	if want := greedyLineCount(plain, width); len(lines) != want {
		out = append(out, fmt.Sprintf("%d lines, one wrap makes %d", len(lines), want))
	}
	return out
}

// #430: a long mob emote came out as three lines with one word stranded on
// the middle one ("...buys what" / "others" / "will not"). FormatEmoteText
// hard-wraps at 80 with CRLF for the player emote category, which the
// pipeline leaves alone; a mob emote goes out as CategoryMobEmote, which the
// pipeline wraps again, and the second wrap counted the stray carriage
// return as a column. FormatMobEmoteText leaves the wrap to the pipeline, so
// the line is wrapped once at whatever width the reader has.
func TestMobEmote_WrapsOnceAtTheReadersWidth(t *testing.T) {
	for _, width := range []int{80, 60} {
		for _, emote := range longMobEmotes {
			plain := renderMobEmoteLine(FormatMobEmoteText("Sly Tam", emote), width)
			for _, p := range emoteWrapProblems(plain, width) {
				t.Errorf("width %d: mob emote %s: %q", width, p, plain)
			}
		}
	}
}

// The old path, FormatEmoteText's own 80-column wrap followed by the
// pipeline's, fails the same check at both widths: at 80 it strands a word
// on a middle line, at 60 it makes three lines where one wrap makes two.
// This is what proves the check above can fail; if FormatMobEmoteText ever
// wraps again, the test above sees this output.
func TestMobEmote_DoubleWrapPathIsCaught(t *testing.T) {
	for _, width := range []int{80, 60} {
		for _, emote := range longMobEmotes {
			plain := renderMobEmoteLine(FormatEmoteText("Sly Tam", emote, "mobname"), width)
			if len(emoteWrapProblems(plain, width)) == 0 {
				t.Errorf("width %d: double-wrapped emote passed the check: %q", width, plain)
			}
		}
	}
	stranded := renderMobEmoteLine(FormatEmoteText("Sly Tam", longMobEmotes[0], "mobname"), 80)
	if lines := strings.Split(strings.ReplaceAll(stranded, "\r", ""), "\n"); len(lines) != 3 || lines[1] != "others" {
		t.Errorf("double wrap at 80 should strand %q on the middle line, got %q", "others", stranded)
	}
}
