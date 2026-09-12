package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// TestApplyPurgeEffects pins the three things a purging draught is supposed to
// do and, before this change, did none of. Buff 70 -- the only thing the item
// declared -- carries a flavour line and no statmods, and there is no buff
// scripting layer, so the draught was inert: it charged toxicity and delivered
// nothing. Buff 76, the weakness it was designed to leave behind, was authored
// in full and referenced by nothing at all.
func TestApplyPurgeEffects(t *testing.T) {
	cleanup := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		61: {BuffId: 61, Name: "Ironhide Brew", TriggerCount: 400, RoundInterval: 1},
		76: {BuffId: 76, Name: "Purging Weakness", TriggerCount: 50, RoundInterval: 1},
	})
	defer cleanup()

	c := characters.New()
	c.Stats.Vitality.Base = 300
	c.Stats.Vitality.Recalculate()
	u := &users.UserRecord{UserId: 7104, Character: c}

	if err := c.AddBuffScaled(61, 1.0); err != nil {
		t.Fatalf("setup: AddBuffScaled(61) = %v", err)
	}
	c.Toxicity = 40
	events.DrainQueuedBuffsForTest(u.UserId) // start from a clean queue

	if !c.HasBuff(61) {
		t.Fatalf("setup: expected the potion buff to be present before the purge")
	}

	applyPurgeEffects(u)

	// RemoveBuff only marks TriggersLeft as expired; the map entry HasBuff
	// checks isn't evicted until the next round's Prune() sweep (see
	// internal/buffs/buffs.go RemoveBuff/Prune, and the same pattern pinned by
	// internal/hooks/pinnacle_ambient_smart_test.go). Prune here to observe the
	// post-sweep state a real drinker would see a moment later.
	c.Buffs.Prune()

	if c.HasBuff(61) {
		t.Errorf("potion buff 61 survived the purge; it must be stripped")
	}
	if c.Toxicity != 0 {
		t.Errorf("Toxicity = %v after the purge, want 0", c.Toxicity)
	}

	// The weakness is QUEUED, not applied in place. Adding it through
	// Character.AddBuffScaled applied it silently: the drinker took a
	// fifty-round stat penalty and read nothing about it. Buff_ApplyBuffs is
	// what narrates the start, and only the event reaches it.
	queued := events.DrainQueuedBuffsForTest(u.UserId)
	var weakness *events.Buff
	for i := range queued {
		if queued[i].BuffId == 76 {
			weakness = &queued[i]
		}
	}
	if weakness == nil {
		t.Fatalf("purging weakness (buff 76) was not queued; the purge must cost something, and it must say so. queued: %+v", queued)
	}
	if weakness.DurationMult != 1.0 {
		t.Errorf("queued buff 76 DurationMult = %v, want 1.0 (the authored duration)", weakness.DurationMult)
	}
}

// TestDetoxItemsBypassTheToxicityGate pins that BOTH detox items skip the
// "would this exceed your tolerance?" pre-check. Gating a detox behind low
// toxicity makes the cure unavailable to exactly the players who need it, and
// at the current tuning the Purging Draught (34) sits above a fresh
// character's whole tolerance (33.3), so without this it could be bought and
// never drunk.
func TestDetoxItemsBypassTheToxicityGate(t *testing.T) {
	for _, tc := range []struct {
		name   string
		itemId int
		want   bool
	}{
		{"Ysolde's Purge", ysoldesPurgeItemId, true},
		{"Purging Draught", purgingDraughtItemId, true},
		{"an ordinary potion", 30036, false},
	} {
		if got := bypassesToxicityGate(tc.itemId); got != tc.want {
			t.Errorf("%s: bypassesToxicityGate(%d) = %v, want %v",
				tc.name, tc.itemId, got, tc.want)
		}
	}
}
