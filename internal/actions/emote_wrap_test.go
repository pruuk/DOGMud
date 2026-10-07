package actions

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// #430: a long mob emote came out as three lines with one word stranded on
// the middle one ("...buys what" / "others" / "will not"). FormatEmoteText
// hard-wraps at 80 with CRLF for the player emote category, which the
// pipeline leaves alone; a mob emote goes out as CategoryMobEmote, which the
// pipeline wraps again, and the second wrap counted the stray carriage
// return as a column. FormatMobEmoteText leaves the wrap to the pipeline.
func TestMobEmote_WrapsOnceAtTheReadersWidth(t *testing.T) {
	tags := regexp.MustCompile(`<[^>]*>`)
	for _, emote := range []string{
		"rolls a trinket across his knuckles and murmurs that he buys what others will not.",
		"fakes a wide yawn and stretches, hand passing briefly over a neighboring stool.",
		"turns to examine the ceiling with apparent fascination, one hand hanging loose at his side.",
	} {
		rendered := messaging.RenderForRecipient(messaging.RenderInput{
			Category:      messaging.CategoryMobEmote,
			Text:          FormatMobEmoteText("Sly Tam", emote),
			Channel:       messaging.ChannelVisual,
			SightDecision: messaging.SightFull,
			LineWidth:     80,
		})
		plain := tags.ReplaceAllString(rendered, "")
		if strings.Contains(plain, "\r") {
			t.Errorf("mob emote carries a carriage return: %q", plain)
		}
		lines := strings.Split(plain, "\n")
		if len(lines) != 2 {
			t.Errorf("mob emote rendered as %d lines, want 2: %q", len(lines), plain)
		}
		for _, line := range lines {
			if len([]rune(line)) > 80 {
				t.Errorf("mob emote line too wide: %q", line)
			}
		}
	}
}
