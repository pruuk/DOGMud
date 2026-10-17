package users

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var promptSlotsTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

func promptVisible(s string) string { return promptSlotsTagRE.ReplaceAllString(s, "") }

// shippedFightPromptTail is the target half of the shipped FightPrompt in
// _datafiles/config.yaml (TextFormats.FightPrompt), compiled the way the
// prompt renderer compiles it.
const shippedFightPromptTail = `{8}] {255}» {target}{8}[{targetpos}{8}|{targethealth}{8}]`

// TestPromptPositionDoesNotGlueToTheBar is the 2026-10-17 playtest line
// "CP:##########Prone]": {pos} printed its word straight after the bar.
func TestPromptPositionDoesNotGlueToTheBar(t *testing.T) {
	u := &UserRecord{
		UserId:    7002,
		Character: &characters.Character{Name: "proner", Position: position.NewMachine()},
	}
	if got := promptVisible(u.ProcessPromptString(`CP:{pos}]`)); got != `CP:]` {
		t.Errorf("a standing player shows no position and no gap, got %q", got)
	}
	r := state.TransitionReason{Trigger: "test_setup"}
	if err := u.Character.Position.TransitionToProne(position.ProneData{}, r); err != nil {
		t.Fatalf("prone: %v", err)
	}
	got := promptVisible(u.ProcessPromptString(`CP:{pos}]`))
	if !strings.HasPrefix(got, `CP: `) || strings.Contains(got, `CP:P`) {
		t.Errorf("a prone player's position must sit apart from the bar, got %q", got)
	}
}

// TestPromptTargetGroupHasNoEmptySlot is the 2026-10-17 playtest line
// "» Steppe Hare[|bruised]": a standing target's empty position slot left a
// bare separator, and an unseen target left "[|]".
func TestPromptTargetGroupHasNoEmptySlot(t *testing.T) {
	defer SetPromptSightCheck(nil)
	u := &UserRecord{
		UserId: 7003,
		Character: &characters.Character{
			Name:        "seer",
			CombatPhase: combatphase.NewMachine(),
		},
	}
	u.Character.SetAggro(0, 999998, characters.DefaultAttack)
	compiled := util.ConvertColorShortTags(shippedFightPromptTail)

	// Unseen target: both slots are empty, so the group goes.
	SetPromptSightCheck(func(*characters.Character) messaging.SightDecision { return messaging.SightNone })
	got := promptVisible(u.ProcessPromptString(compiled))
	if strings.Contains(got, "|") || strings.Contains(got, "[") {
		t.Errorf("an unseen target leaves an empty group: %q", got)
	}

	// The slot rules on their own: each empty side goes, a full pair stays.
	cases := map[string]string{
		`[<ansi fg="8">|</ansi>bruised]`: `[bruised]`,
		`[Prone<ansi fg="8">|</ansi>]`:   `[Prone]`,
		`[Prone|bruised]`:                `[Prone|bruised]`,
		`x[<ansi fg="8">|</ansi>]y`:      `xy`,
		`[* HP:## SP:##]`:                `[* HP:## SP:##]`,
	}
	for in, want := range cases {
		if got := promptVisible(collapseEmptyPromptSlots(in)); got != want {
			t.Errorf("collapseEmptyPromptSlots(%q) = %q, want %q", in, got, want)
		}
	}
}
