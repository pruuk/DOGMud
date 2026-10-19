package messaging

import (
	"strings"
	"testing"
)

// #614: every deliverer ends a message with its own "\n", so a text that
// already ended in a line break (a template's last line) printed a stray
// blank line. The pipeline trims trailing breaks; inner breaks stay.
func TestRenderForRecipient_TrimsTrailingLineBreaks(t *testing.T) {
	for _, cat := range []Category{CategorySystem, CategoryScreen} {
		for _, in := range []string{"row one\r\nrow two\r\n", "row one\r\nrow two\r\n\r\n", "row one\nrow two\n\n\n"} {
			out := RenderForRecipient(RenderInput{Category: cat, Text: in, Channel: ChannelAudio, LineWidth: 80})
			bare := strings.ReplaceAll(stripTagsForTest(out), "\r", "")
			if strings.HasSuffix(bare, "\n") {
				t.Errorf("%s: %q rendered with a trailing break: %q", cat, in, out)
			}
			// System's normalizer capitalizes the first letter.
			if !strings.Contains(strings.ToLower(bare), "row one\nrow two") {
				t.Errorf("%s: %q lost its inner break: %q", cat, in, out)
			}
		}
		if out := RenderForRecipient(RenderInput{Category: cat, Text: "\r\n\n", Channel: ChannelAudio, LineWidth: 80}); out != "" {
			t.Errorf("%s: a text of only line breaks must not be delivered, got %q", cat, out)
		}
	}
}

func stripTagsForTest(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}
