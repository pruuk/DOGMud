package messaging

import (
	"strings"
	"testing"
)

// RenderForRecipient is the per-recipient pipeline entry point used
// internally by the Room/UserRecord Send helpers. Returns the final
// text to deliver. An empty return string means "don't deliver to
// this recipient" (used by the sight gate).
func TestRenderForRecipientStubReturnsTextUnchanged(t *testing.T) {
	got := RenderForRecipient(RenderInput{
		Category:  CategoryDefault,
		Text:      "Hello, world.",
		Channel:   ChannelAudio,
		LineWidth: 80,
	})
	if got != "Hello, world." {
		t.Fatalf("stub pipeline mutated text: got %q", got)
	}
}

func TestChannelConstants(t *testing.T) {
	if ChannelAudio == ChannelVisual {
		t.Fatal("ChannelAudio and ChannelVisual must differ")
	}
}

func TestApplyCategoryColorWrapsTagForKnownCategory(t *testing.T) {
	got := applyCategoryColor(CategoryHitMelee, "strikes deeply")
	want := `<ansi fg="hit-melee">strikes deeply</ansi>`
	if got != want {
		t.Fatalf("color wrap: got %q want %q", got, want)
	}
}

func TestApplyCategoryColorDefaultPassesThrough(t *testing.T) {
	got := applyCategoryColor(CategoryDefault, "plain text")
	if got != "plain text" {
		t.Fatalf("CategoryDefault must pass text through unchanged, got %q", got)
	}
}

func TestApplyCategoryColorEmptyTextPassesThrough(t *testing.T) {
	got := applyCategoryColor(CategoryHitMelee, "")
	if got != "" {
		t.Fatalf("empty text must pass through unchanged, got %q", got)
	}
}

// wrapAllowlist pins the exact set of categories shouldWrap admits. The
// duplication against shouldWrap's switch is deliberate: the point of
// this map is that changing shouldWrap without changing this list fails
// the build, so wrap policy can only change by a deliberate edit in both
// places at once.
var wrapAllowlist = map[Category]bool{
	// Combat — hits.
	CategoryHitMelee:        true,
	CategoryHitBlunt:        true,
	CategoryHitNaturalSharp: true,
	CategoryHitRanged:       true,
	CategoryHitCaster:       true,
	CategoryHitUnarmed:      true,

	// Combat — defense.
	CategoryDodge: true,
	CategoryParry: true,
	CategoryBlock: true,

	// Combat — grapple.
	CategoryGrappleFlow: true,

	// Combat — outcome.
	CategorySubmission:         true,
	CategoryDeath:              true,
	CategoryCombatSummary:      true,
	CategoryCombatBlindWarning: true,

	// Combat — special moves.
	CategorySurpriseAttack: true,
	CategoryKick:           true,
	CategoryTrip:           true,
	CategoryBash:           true,
	CategoryRally:          true,
	CategoryWarcry:         true,
	CategoryTauntSuccess:   true,
	CategoryTauntResist:    true,
	CategoryTauntFailure:   true,

	// Spells.
	CategorySpellFold:          true,
	CategorySpellDisruption:    true,
	CategorySpellElemental:     true,
	CategorySpellEnhancement:   true,
	CategorySpellMental:        true,
	CategorySpellVital:         true,
	CategorySpellManifestation: true,

	// NPC and ambient prose.
	CategoryNPCDialogue:  true,
	CategoryDialogueHint: true,
	CategoryMobIdle:      true,
	CategoryMobEmote:     true,
	CategoryRoomEntry:    true,
	CategoryRoomExit:     true,
	CategoryWeather:      true,
	CategoryTimeOfDay:    true,
	CategoryLight:        true,

	// Other narration plus tips.
	CategoryLoot:            true,
	CategoryEquipment:       true,
	CategoryConditionApply:  true,
	CategoryConditionExpire: true,
	CategoryMutation:        true,
	CategoryTip:             true,
	CategoryLogout:          true,
}

// TestShouldWrapMatchesPinnedAllowlist pins shouldWrap's exact
// membership against wrapAllowlist. A mismatch means one of the two
// changed without the other; update both deliberately. It also asserts
// the admitted count is exactly 46, so the pin itself cannot be edited
// without updating its own count.
func TestShouldWrapMatchesPinnedAllowlist(t *testing.T) {
	admitted := 0
	for c := CategoryDefault; c < categoryMax; c++ {
		got := shouldWrap(c)
		want := wrapAllowlist[c]
		if got != want {
			t.Errorf("shouldWrap(%q) = %v, wrapAllowlist says %v — update both shouldWrap and wrapAllowlist deliberately", c, got, want)
		}
		if got {
			admitted++
		}
	}
	if admitted != 46 {
		t.Errorf("expected exactly 46 categories admitted to wrap, got %d", admitted)
	}
}

// TestMixedAndPreformattedCategoriesNeverWrap states the never-wrap rule
// independently of wrapAllowlist's membership pin, so a careless edit to
// the list cannot quietly admit one of these.
func TestMixedAndPreformattedCategoriesNeverWrap(t *testing.T) {
	neverWrap := map[Category]string{
		CategorySystem:          "mixed bucket: refusals alongside the score sheet, inventory, who, help, admin DynamicList tables and the ASCII map",
		CategoryBroadcast:       "mixed bucket: free channel chat alongside the hand-drawn MOTD box",
		CategoryRoomDescription: "renders a side-by-side block, prose left and minimap right",
		CategorySplash:          "rendered ASCII art",
		CategorySkillProgress:   "a banner with its own formatting",
		CategorySpeech:          "already wrapped at a hardcoded 80 by say.go, ignoring LineWidth",
		CategoryWhisper:         "already wrapped at a hardcoded 80 by whisper.go, ignoring LineWidth",
		CategoryShout:           "already wrapped at a hardcoded 80 by shout.go, ignoring LineWidth",
		CategoryEmote:           "already wrapped at a hardcoded 80 by reply.go, ignoring LineWidth",
		CategoryDefault:         "the fail-safe default: an unclassified category keeps today's behavior",
	}

	for cat, reason := range neverWrap {
		if shouldWrap(cat) {
			t.Errorf("shouldWrap(%q) = true, want false: %s", cat, reason)
		}
		if wrapAllowlist[cat] {
			t.Errorf("wrapAllowlist[%q] = true, want false: %s", cat, reason)
		}
	}
}

// TestRoomDescriptionSkipsWrap is the regression test for the
// look-command minimap layout bug — descriptions/room templates
// render a side-by-side block (prose on the left, minimap column on
// the right). The pipeline's wrap stage would otherwise wrap each
// line at LineWidth and shatter the side-by-side layout.
func TestRoomDescriptionSkipsWrap(t *testing.T) {
	// Line containing what looks like a side-by-side row: text + many
	// spaces + map column. Total length 90 > LineWidth 40, but the
	// pipeline must NOT wrap this — the template owns the column.
	row := "the cobblestone road winds west          " + "║·····║"
	got := RenderForRecipient(RenderInput{
		Category:  CategoryRoomDescription,
		Text:      row,
		Channel:   ChannelAudio,
		LineWidth: 40,
	})
	// The line should still be one line (no \n introduced by wrap).
	// Color stage may add an ANSI tag wrapper, but no newlines.
	for _, ch := range got {
		if ch == '\n' {
			t.Fatalf("CategoryRoomDescription must not wrap (side-by-side layout would break); got %q", got)
		}
	}
}

// TestHitMeleeWrapsAtRecipientLineWidth supersedes the old
// TestHitMeleeDoesNotWrap. Task 4 admits combat narrative to the wrap
// allowlist, so the "combat never wraps" assumption that test pinned is
// now false; this proves the new behavior instead, the same shape as
// TestTipWrapsAtRecipientLineWidth.
func TestHitMeleeWrapsAtRecipientLineWidth(t *testing.T) {
	long := "the rusty longsword bites through cloth and into flesh causing a serious wound to the defender's left shoulder"
	got := RenderForRecipient(RenderInput{
		Category:  CategoryHitMelee,
		Text:      long,
		Channel:   ChannelAudio,
		LineWidth: 40,
	})

	lines := splitLines(got)
	if len(lines) < 2 {
		t.Fatalf("expected CategoryHitMelee to fold at width 40, got one line:\n%q", got)
	}
	for i, line := range lines {
		if w := displayWidth(line); w > 40 {
			t.Errorf("line %d is %d visible columns, over the 40 requested: %q", i+1, w, line)
		}
	}
}

// TestTipWrapsAtRecipientLineWidth is the motivating case: 64 of the 74
// shipped tips exceed 80 characters, longest 211, and a bare telnet client
// breaks them mid-word.
func TestTipWrapsAtRecipientLineWidth(t *testing.T) {
	long := "Healing up slow? Some rooms are sanctuaries, temples, certain camps, " +
		"the Sanctum Basin tutorial, and regenerate health, stamina, and conviction " +
		"much faster than ordinary rooms. Look for a peaceful description."

	got := RenderForRecipient(RenderInput{
		Category:  CategoryTip,
		Text:      long,
		Channel:   ChannelAudio,
		LineWidth: 60,
	})

	lines := splitLines(got)
	if len(lines) < 2 {
		t.Fatalf("a 211-character tip must fold at width 60, got one line:\n%q", got)
	}
	for i, line := range lines {
		if w := displayWidth(line); w > 60 {
			t.Errorf("line %d is %d visible columns, over the 60 requested: %q", i+1, w, line)
		}
	}
}

// TestSystemNeverWraps guards the mixed bucket. CategorySystem carries the
// score sheet, inventory, who, help, the admin DynamicList tables and the
// ASCII map alongside one-line refusals, so folding it would shatter every
// one of those layouts.
//
// This does not assert byte-exact passthrough. The pipeline legitimately
// adds a category colour tag (stage 5) and, today, a normalization period
// on the table's last line, because skipStages does not exempt
// CategorySystem (stage 2). Neither is the wrap stage's doing, and
// neither is this test's business. What it guards is the wrap stage
// specifically, which can damage a table two ways: folding a long line
// changes the line count, and, even without folding, wrap can collapse
// runs of padding spaces and destroy column alignment while leaving the
// line count unchanged (a real DynamicList table went from 494 bytes to
// 436 at width 55 with its newline count untouched). So this checks
// both: the output has the same number of lines as the input, and each
// input line's padding survives intact as an exact substring of the
// output.
func TestSystemNeverWraps(t *testing.T) {
	table := "Item              Qty   Value\n" +
		"---------------   ---   -----\n" +
		"healing draught     3     45g"

	got := RenderForRecipient(RenderInput{
		Category:  CategorySystem,
		Text:      table,
		Channel:   ChannelAudio,
		LineWidth: 20,
	})

	inputLines := splitLines(table)
	gotLines := splitLines(got)
	if len(gotLines) != len(inputLines) {
		t.Fatalf("CategorySystem must not fold lines: input had %d, output has %d:\n%q", len(inputLines), len(gotLines), got)
	}
	for _, line := range inputLines {
		if !strings.Contains(got, line) {
			t.Errorf("CategorySystem must preserve column padding: expected output to contain %q, got:\n%q", line, got)
		}
	}
}
