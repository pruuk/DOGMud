package messaging

import (
	"strings"
	"testing"
)

// TestPreformattedCategoriesPassThroughUnwrapped pins the CONSEQUENCE
// of shouldWrap's allowlist (pipeline.go), not its policy. Task 4's
// tests (pipeline_test.go's TestShouldWrapMatchesPinnedAllowlist and
// friends) already pin which categories the switch admits. This test
// exists because an allowlist can be correct in isolation and still
// let a real payload shape through mangled if nobody ever ran one
// through it.
//
// Five shapes are run through RenderForRecipient. Three categories are
// excluded from the wrap stage and run at LineWidth: 20, far narrower
// than any fixture's content, so the stage would have every reason to
// fire if it were mistakenly admitted. System and Broadcast are admitted,
// so they run at 80, wider than the fixture, where the stage must leave
// a line that fits alone:
//
//  1. CategorySystem    - a column table (inventory-style listing).
//  2. CategoryBroadcast - a box-drawn banner, using the box-drawing
//     characters as in usercommands/motd.go's frame.
//  3. CategoryRoomDescription - a side-by-side block: prose on the
//     left, a minimap column on the right, separated by a vertical bar.
//  4. CategorySplash - small ASCII art.
//  5. CategorySkillProgress - a progress banner.
//
// These are SHAPES, not live renders: internal/messaging sits below
// internal/templates and internal/usercommands in the import graph
// and cannot call the real template/command code that produces these
// screens. Task 7 closes that gap with a root-package test that
// renders real production templates; this test is not sufficient by
// itself, only a guard against the wrap stage specifically.
//
// Why not byte equality (if got != fixture.text): it would fail on
// all five fixtures here, for two reasons that are legitimate pipeline
// behavior and out of scope for a wrap guard:
//
//   - applyCategoryColor (pipeline.go) wraps every category except
//     CategoryDefault in `<ansi fg="...">...</ansi>`. All five
//     categories below are non-default, so all five gain a tag.
//   - skipStages (normalize.go) exempts CategoryRoomDescription,
//     CategorySplash and CategorySkillProgress from normalization, but
//     NOT CategorySystem or CategoryBroadcast. Those two additionally
//     get a sentence-ending period appended to their last line by
//     appendEndPunct.
//
// Neither of those is the wrap stage's doing, and fixing them (for
// example, exempting CategorySystem from normalization) is explicitly
// out of scope here: it would disable capitalization across roughly
// 1975 refusal sites to protect a few dozen tables, and is filed as
// its own follow-up.
//
// So this test asserts what the wrap guard is actually for. Wrapping
// damages pre-formatted text in exactly two ways:
//
//  1. It folds long lines, changing the line count.
//  2. It collapses runs of padding spaces, destroying column alignment
//     WITHOUT changing the line count (WrapAnsi rejoins words with a
//     single space regardless of how many spaces separated them in the
//     source). A real table once went from 494 bytes to 436 at width
//     55 with its newline count unchanged, so a line-count check alone
//     would have passed while the table was ruined.
//
// Both are asserted for every fixture:
//
//   - the rendered output has the same number of lines as the fixture
//     text, and
//   - every line of the fixture text still appears verbatim as a
//     substring of the rendered output, which proves internal padding
//     runs survived (a folded or space-collapsed line could not
//     contain the original line as a substring).
//
// Channel is ChannelAudio throughout: this test is about wrap, not
// about the sight gate, and audio ignores SightDecision entirely.
func TestPreformattedCategoriesPassThroughUnwrapped(t *testing.T) {
	fixtures := []struct {
		name     string
		category Category
		width    int
		text     string
	}{
		{
			name:     "system column table",
			category: CategorySystem,
			width:    80,
			text: "Item              Qty   Value\n" +
				"---------------   ---   -----\n" +
				"healing draught     3     45g\n" +
				"iron sword          1    120g",
		},
		{
			name:     "broadcast box banner",
			category: CategoryBroadcast,
			width:    80,
			text: " ╔════════════════════════════╗\n" +
				" ║ Server Notice: reboot at 3am ║\n" +
				" ╚════════════════════════════╝",
		},
		{
			name:     "room description side-by-side minimap",
			category: CategoryRoomDescription,
			width:    20,
			text: "You stand in a wide stone hall.   | . . # . .\n" +
				"Torches line the walls, guttering. | . @ . # .\n" +
				"A cold draft comes from the north. | . . . . .",
		},
		{
			name:     "splash ascii art",
			category: CategorySplash,
			width:    20,
			text: "   /\\_/\\\n" +
				"  ( o.o )\n" +
				"   > ^ <",
		},
		{
			name:     "skill progress banner",
			category: CategorySkillProgress,
			width:    20,
			text: "[==========          ] Swordsmanship\n" +
				"[====================] Unarmed Combat",
		},
	}

	for _, f := range fixtures {
		t.Run(f.name, func(t *testing.T) {
			got := RenderForRecipient(RenderInput{
				Category:  f.category,
				Text:      f.text,
				Channel:   ChannelAudio,
				LineWidth: f.width,
			})

			wantLines := strings.Split(f.text, "\n")
			gotLines := strings.Split(got, "\n")
			if len(gotLines) != len(wantLines) {
				t.Fatalf("%s: line count changed: fixture had %d lines, output has %d\nfixture:\n%s\noutput:\n%s",
					f.name, len(wantLines), len(gotLines), f.text, got)
			}

			for _, line := range wantLines {
				if !strings.Contains(got, line) {
					t.Fatalf("%s: fixture line not found verbatim in output (padding likely collapsed): %q\noutput:\n%s",
						f.name, line, got)
				}
			}
		})
	}
}
