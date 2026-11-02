package main

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// Quests 99 and 101 teach and count a theft, so only a theft moves them. These
// walk the real commands on the shipped quests: the player types drop, get,
// steal and plant through the dispatcher, the dummy dies through its real death
// pipeline, and the gain reaches the trigger through the event queue. Nothing
// raises a quest event by hand.

const (
	purseJetty = 5218
	purseItem  = 60
	purseDummy = 9127
)

// newPurseStudent stands a rank 1 student on the Reed Jetty beside the dummy.
// High Dexterity makes a theft likely, not certain: ContestFloor still lets the
// dummy feel one in eight, so stealLands retries.
func newPurseStudent(t *testing.T) *spineWalk {
	t.Helper()
	w := newSpineWalk(t)
	_ = w.user.Character.GetSkillLevel(skills.Skullduggery) // allocates Skills
	w.user.Character.Skills[string(skills.Skullduggery)] = 1
	w.user.Character.Stats.Dexterity.Base = 3000
	w.user.Character.Stats.Dexterity.Recalculate()
	// A test user has no Strength, so its pack refuses the second purse.
	w.user.Character.Stats.Strength.Base = 100
	w.user.Character.Stats.Strength.Recalculate()
	w.walkTo(purseJetty)
	w.dummy()
	return w
}

// atHidden walks quest 99 to its theft step, one token at a time, as play does.
func (w *spineWalk) atHidden() {
	w.t.Helper()
	for _, tok := range []string{"99-start", "99-lesson", "99-hidden"} {
		w.user.Character.GiveQuestToken(tok)
	}
	w.step(99, "hidden", "the setup")
}

// dummy is the Pocket Dummy on the jetty, spawned as the room's list spawns it.
func (w *spineWalk) dummy() *mobs.Mob {
	w.t.Helper()
	_, m := w.npc(purseDummy)
	return m
}

func (w *spineWalk) pursesHeld() int {
	n := 0
	for _, itm := range w.user.Character.Items {
		if itm.ItemId == purseItem {
			n++
		}
	}
	return n
}

// givePurse puts a purse in the pack with no event, as if it were always there.
// The drop that follows is the first thing the quest could hear.
func (w *spineWalk) givePurse() {
	w.t.Helper()
	if !w.user.Character.StoreItem(items.New(purseItem)) {
		w.t.Fatal("the pack refused a purse")
	}
}

// calm ends what a caught theft started and spends the wait: the cooldown is
// the only thing that keeps a student from trying again, and the test has no
// real seconds to spend.
func (w *spineWalk) calm() {
	w.t.Helper()
	w.user.Character.EndAggro()
	w.dummy().Character.EndAggro()
	w.user.Character.Cooldowns = nil
}

// stealLands types steal dummy until a purse lands in the pack, refilling the
// pocket as the dummy's idle tick does. The pickpocket's pause is cut short the
// way a copyover cuts it, so the reveal runs now, under the real code.
func (w *spineWalk) stealLands() {
	w.t.Helper()
	before := w.pursesHeld()
	for try := 0; try < 60; try++ {
		w.calm()
		m := w.dummy()
		if _, has := m.Character.FindInBackpack("purse"); !has {
			if !m.Character.StoreItem(items.New(purseItem)) {
				w.t.Fatal("the dummy refused a purse")
			}
		}
		w.typed("steal", "dummy")
		actions.FlushPocketAttempts()
		w.pump()
		if w.pursesHeld() > before {
			w.calm()
			return
		}
	}
	w.t.Fatalf("no theft landed in 60 tries, at %v", w.user.Character.QuestProgress)
}

// dropAndGet is the reported route: the same purse, down and up again.
func (w *spineWalk) dropAndGet() {
	w.t.Helper()
	held := w.pursesHeld()
	w.typed("drop", "purse")
	if got := w.pursesHeld(); got != held-1 {
		w.t.Fatalf("drop left %d purses, want %d", got, held-1)
	}
	w.typed("get", "purse")
	if got := w.pursesHeld(); got != held {
		w.t.Fatalf("get left %d purses, want %d", got, held)
	}
}

// A picked-up purse is not a lift, and it does not stand in the way of one: the
// same student still steals another and moves on, and can plant either.
func TestSmoke_Quest99_OnlyATheftIsALift(t *testing.T) {
	w := newPurseStudent(t)
	w.atHidden()
	w.givePurse()

	w.dropAndGet()
	w.step(99, "hidden", "drop and get")

	// Plant works on a picked-up purse and teaches nothing yet: the plant step
	// is gated on the lift.
	w.typed("plant", "purse on dummy")
	actions.FlushPocketAttempts()
	w.pump()
	w.step(99, "hidden", "a plant before any lift")
	if w.pursesHeld() == 0 {
		w.givePurse()
	}

	w.stealLands()
	w.step(99, "lifted", "a theft that landed")

	w.calm()
	w.typed("plant", "purse on dummy")
	w.pump()
	w.step(99, "planted", "the plant")
}

// Knocking the dummy down and looting the purse from its corpse is not a lift.
// A fresh dummy is set out and its purse can still be stolen.
func TestSmoke_Quest99_LootingTheDummyIsNotALift(t *testing.T) {
	w := newPurseStudent(t)
	w.atHidden()

	m := w.dummy()
	if _, has := m.Character.FindInBackpack("purse"); !has {
		// A world reload can hand back the dummy an earlier walk emptied.
		if !m.Character.StoreItem(items.New(purseItem)) {
			t.Fatal("the dummy refused a purse")
		}
	}
	m.Character.Die(state.ActorRef{UserId: w.user.UserId}, life.TriggerHealthZero)
	w.pump()

	w.typed("get", "purse from pocket dummy corpse")
	if got := w.pursesHeld(); got != 1 {
		t.Fatalf("looting the corpse left %d purses, want 1", got)
	}
	w.step(99, "hidden", "looting the corpse")

	w.dropAndGet()
	w.step(99, "hidden", "dropping and getting the looted purse")

	w.stealLands()
	w.step(99, "lifted", "a theft from the fresh dummy")
}

// The reported exploit: three drops and gets complete Quick Fingers. Three
// thefts do, and a pickup between them neither counts nor resets the count.
func TestSmoke_Quest101_CountsThefts(t *testing.T) {
	w := newPurseStudent(t)
	w.user.Character.GiveQuestToken("101-start")
	w.givePurse()

	for i := 0; i < 3; i++ {
		w.dropAndGet()
	}
	w.step(101, "start", "three drops and gets")

	w.stealLands()
	w.dropAndGet()
	w.stealLands()
	w.step(101, "start", "two thefts and a pickup")
	w.stealLands()
	w.step(101, "lifted", "the third theft")
}
