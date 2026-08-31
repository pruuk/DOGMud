package messaging

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)

func newChar(t *testing.T) *characters.Character {
	t.Helper()
	c := characters.New()
	c.Perception = perception.NewMachine()
	return c
}

func setBlinded(t *testing.T, c *characters.Character) {
	t.Helper()
	if err := c.Perception.TransitionTo(perception.Blinded,
		state.TransitionReason{Trigger: "test"}); err != nil {
		t.Fatalf("transition to Blinded failed: %v", err)
	}
}

func TestCanSeeClearlyLitRoomSighted(t *testing.T) {
	c := newChar(t)
	// Use nil room — the predicate short-circuits to "lit" on nil.
	// A zero-value &rooms.Room{} cannot be used here because
	// Room.GetVisibility() calls into the biome registry which isn't
	// loaded in unit-test context (panics on nil BiomeInfo). Real
	// lit-room behavior is exercised in end-to-end tests with engine
	// boot.
	if !CanSeeClearly(c, nil) {
		t.Fatal("Sighted observer in default (nil) room should see clearly")
	}
}

func TestCanSeeClearlyBlinded(t *testing.T) {
	c := newChar(t)
	setBlinded(t, c)
	// Same nil-room caveat as TestCanSeeClearlyLitRoomSighted.
	if CanSeeClearly(c, nil) {
		t.Fatal("Blinded observer must NOT see clearly even in a lit room")
	}
}

func TestCanSeeShapesInfraredInDark(t *testing.T) {
	c := newChar(t)
	// Note: GetVisibility() < 1 = dark. We can't easily fabricate a
	// dark Room here without engine coupling — this test uses the
	// nil-room path which short-circuits to lit. Real darkness
	// behavior is exercised in pipeline_test.go's end-to-end suite.
	if !CanSeeShapes(c, nil) {
		t.Fatal("Sighted observer must see shapes (nil room defaults to lit)")
	}
}

func TestCanSeeShapesBlindedNoInfrared(t *testing.T) {
	c := newChar(t)
	setBlinded(t, c)
	if CanSeeShapes(c, nil) {
		t.Fatal("Blinded observer must NOT see shapes, even with nil/lit room")
	}
	_ = buffs.InfraredVision // ensure the flag constant exists
}

func TestNilCharacterDefaultsToSeeing(t *testing.T) {
	if !CanSeeClearly(nil, nil) {
		t.Fatal("nil observer must default to CanSeeClearly (defensive)")
	}
	if !CanSeeShapes(nil, nil) {
		t.Fatal("nil observer must default to CanSeeShapes (defensive)")
	}
}

// setSleeping gives the character the Sleeping buff flag.
//
// Unlike blindness, sleep is not a Perception state -- it is a buff flag, so
// this seeds a minimal spec into the global registry and applies it. The
// registry is restored by the returned cleanup.
func setSleeping(t *testing.T, c *characters.Character) {
	t.Helper()
	const sleepBuffId = 9001
	restore := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		sleepBuffId: {
			BuffId: sleepBuffId,
			Name:   "Test Sleep",
			Flags:  []buffs.Flag{buffs.Sleeping},
		},
	})
	t.Cleanup(restore)
	if err := c.AddBuff(sleepBuffId, true); err != nil {
		t.Fatalf("applying the sleeping buff failed: %v", err)
	}
	if !c.HasBuffFlag(buffs.Sleeping) {
		t.Fatal("precondition: the character should now carry the Sleeping flag")
	}
}

// M0b Task 3. A sleeping character perceives nothing visual.
//
// The delivery pipeline had no concept of sleep at all -- grepping `Sleeping`
// under internal/messaging returned nothing -- so a sleeping player kept
// receiving every visual broadcast in the room: NPC dialogue, ambient flavour,
// arrivals and departures. Reported from play 2026-08-31.
//
// Audio is deliberately unaffected. Room.SendText bypasses this gate entirely,
// so a shout still reaches a sleeper and still wakes them; shout.go owns that.
func TestCanSeeClearly_SleeperPerceivesNothingVisual(t *testing.T) {
	c := newChar(t)
	setSleeping(t, c)
	// nil room short-circuits to LIT, so this proves sleep gates on its own
	// rather than riding on darkness.
	if CanSeeClearly(c, nil) {
		t.Error("a sleeping character must not see clearly, even in a lit room")
	}
	if CanSeeShapes(c, nil) {
		t.Error("a sleeping character must not see shapes either")
	}
}

// The direction a careless fix breaks: gating too broadly and blinding everyone.
// This passes BEFORE the change, which is what makes it a guard.
func TestCanSeeClearly_AwakeStillSees(t *testing.T) {
	c := newChar(t)
	if !CanSeeClearly(c, nil) {
		t.Error("an awake character in a lit room must still see clearly")
	}
	if !CanSeeShapes(c, nil) {
		t.Error("an awake character in a lit room must still see shapes")
	}
}
