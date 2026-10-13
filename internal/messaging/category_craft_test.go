package messaging

import (
	"strings"
	"testing"
)

// craftLongLines are shipped recipe lines (#515): the healing salve's
// success_actor, the issue's example at 88 columns, and the longest
// success_actor in _datafiles/world/dogmud/recipes (Staff of the Hollow
// Choir). The send site wraps each in a green or red tag, as here.
var craftLongLines = []string{
	`<ansi fg="green">You grind the root to paste, mix it with water, and seal it in the bottle. A healing salve!</ansi>`,
	`<ansi fg="green">You crown the conductor-core with the ring of focus-gems and hollow the stave through to seat the voice-box, then bind the chorus-shard to the whole with chrysalis setting. The staff hums a chord just below hearing. Veyra's masterwork is finished -- the Staff of the Hollow Choir is bound.</ansi>`,
}

// visibleWords strips ansi tags and splits on whitespace, so a wrapped
// render can be compared word for word with its source.
func visibleWords(s string) []string {
	return strings.Fields(ansiTagPattern.ReplaceAllString(s, ""))
}

// TestCraftLinesWrapAt80 is #515's fix: the crafter's own craft lines ride
// CategoryCraft, which the pipeline folds at the reader's width. Every
// output line fits in 80 visible columns and no word is lost or changed.
func TestCraftLinesWrapAt80(t *testing.T) {
	for _, src := range craftLongLines {
		got := RenderForRecipient(RenderInput{
			Category:  CategoryCraft,
			Text:      src,
			Channel:   ChannelAudio,
			LineWidth: 80,
		})
		lines := splitLines(got)
		if len(lines) < 2 {
			t.Fatalf("a %d-column craft line must fold at 80, got one line:\n%q", displayWidth(src), got)
		}
		for i, line := range lines {
			if w := displayWidth(line); w > 80 {
				t.Errorf("line %d is %d visible columns, over 80: %q", i+1, w, line)
			}
		}
		if want, have := strings.Join(visibleWords(src), " "), strings.Join(visibleWords(got), " "); want != have {
			t.Errorf("wrap changed the text:\nwant %q\ngot  %q", want, have)
		}
	}
}

// TestCraftLinesWouldNotWrapOnSystem proves the probe above can fail: the
// same line on CategorySystem, where these lines rode before #515, comes
// out as one line wider than 80.
func TestCraftLinesWouldNotWrapOnSystem(t *testing.T) {
	got := RenderForRecipient(RenderInput{
		Category:  CategorySystem,
		Text:      craftLongLines[0],
		Channel:   ChannelAudio,
		LineWidth: 80,
	})
	if lines := splitLines(got); len(lines) != 1 || displayWidth(lines[0]) <= 80 {
		t.Fatalf("expected CategorySystem to leave the 88-column line whole, got:\n%q", got)
	}
}

// TestCategoryCraftKeepsSystemTreatment pins that moving the craft lines off
// CategorySystem changed only the wrap: the same normalisation stages run
// and no verbosity tier drops them.
func TestCategoryCraftKeepsSystemTreatment(t *testing.T) {
	if CategoryCraft.String() != "craft" {
		t.Fatalf("CategoryCraft.String() = %q, want craft", CategoryCraft.String())
	}
	if skipStages(CategoryCraft) != skipStages(CategorySystem) {
		t.Error("CategoryCraft must normalise exactly as CategorySystem")
	}
	for _, v := range []Verbosity{VerbosityFull, VerbosityMedium, VerbosityLight} {
		if v.Suppresses(CategoryCraft) {
			t.Errorf("verbosity %v suppresses CategoryCraft; no tier may", v)
		}
	}
}
