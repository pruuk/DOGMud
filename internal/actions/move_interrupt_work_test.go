package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
)

// #512, owner ruling 2026-10-13: a move stops crafting and salvaging for a
// player and a mob alike, through one helper. Casting is never interrupted
// by movement.

func busyCharacter(t *testing.T, work activity.State) *characters.Character {
	t.Helper()
	c := characters.New()
	var err error
	switch work {
	case activity.Crafting:
		err = c.Activity.TransitionToCrafting(
			activity.CraftingData{RecipeId: "interrupt-test", RoundsTotal: 5},
			state.TransitionReason{Trigger: activity.TriggerCraftBegin})
	case activity.Salvaging:
		err = c.Activity.TransitionToSalvaging(
			activity.SalvagingData{ItemUuid: "interrupt-test", RoundsTotal: 5},
			state.TransitionReason{Trigger: activity.TriggerSalvageBegin})
	case activity.Casting:
		err = c.Activity.TransitionToCasting(
			activity.CastingData{SpellId: "sparks", FoldsNeeded: 3},
			state.TransitionReason{Trigger: activity.TriggerCastBegin})
	}
	if err != nil {
		t.Fatalf("fixture: begin %v: %v", work, err)
	}
	return c
}

func TestInterruptWorkOnMove_StopsCraftingAndSalvaging(t *testing.T) {
	for _, work := range []activity.State{activity.Crafting, activity.Salvaging} {
		c := busyCharacter(t, work)
		var trigger string
		if work == activity.Crafting {
			c.Activity.OnCraftEnded("test", func(_ activity.CraftingData, r state.TransitionReason) { trigger = r.Trigger })
		} else {
			c.Activity.OnSalvageEnded("test", func(_ activity.SalvagingData, r state.TransitionReason) { trigger = r.Trigger })
		}

		if got := InterruptWorkOnMove(c); got != work {
			t.Errorf("%v: InterruptWorkOnMove = %v, want %v", work, got, work)
		}
		if !c.Activity.IsFree() {
			t.Errorf("%v: Activity = %v after a move, want Free", work, c.Activity.State())
		}
		if trigger != activity.TriggerMovementInterrupt {
			t.Errorf("%v: ended with trigger %q, want %q", work, trigger, activity.TriggerMovementInterrupt)
		}
	}
}

func TestInterruptWorkOnMove_LeavesCastingAlone(t *testing.T) {
	c := busyCharacter(t, activity.Casting)
	if got := InterruptWorkOnMove(c); got != activity.Free {
		t.Errorf("InterruptWorkOnMove = %v for a caster, want Free (nothing stopped)", got)
	}
	if !c.Activity.IsCasting() {
		t.Errorf("Activity = %v after a move, want Casting (movement never interrupts a cast)", c.Activity.State())
	}
}

func TestInterruptWorkOnMove_IdleAndNilAreNoOps(t *testing.T) {
	if got := InterruptWorkOnMove(nil); got != activity.Free {
		t.Errorf("nil character: got %v, want Free", got)
	}
	c := characters.New()
	if got := InterruptWorkOnMove(c); got != activity.Free {
		t.Errorf("idle character: got %v, want Free", got)
	}
	c.Activity = nil
	if got := InterruptWorkOnMove(c); got != activity.Free {
		t.Errorf("nil Activity: got %v, want Free", got)
	}
}

// RelocateMob is where a mob's walk and its flee both end, so it is where a
// mob's move stops its work. A casting mob walks on still casting.
func TestRelocateMob_StopsWorkButNotACast(t *testing.T) {
	for _, work := range []activity.State{activity.Crafting, activity.Salvaging, activity.Casting} {
		t.Run(work.String(), func(t *testing.T) {
			t.Cleanup(func() { events.DrainAllQueuedEventsForTest() })
			const from, to, instId = 99511, 99512, 98511
			t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{
				from: {RoomId: from, Zone: "test", Exits: map[string]exit.RoomExit{"north": {RoomId: to}}},
				to:   {RoomId: to, Zone: "test", Exits: map[string]exit.RoomExit{"south": {RoomId: from}}},
			}, map[string]*rooms.ZoneConfig{}))

			m := &mobs.Mob{InstanceId: instId, Character: *busyCharacter(t, work)}
			m.Character.Name = "Smith"
			m.Character.RoomId = from
			mobs.SetInstanceForTest(instId, m)
			t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })
			fromRoom, toRoom := rooms.LoadRoom(from), rooms.LoadRoom(to)
			fromRoom.AddMob(instId)

			RelocateMob(m, fromRoom, "north", toRoom, false)

			if m.Character.RoomId != to {
				t.Fatalf("fixture: mob RoomId = %d, want %d", m.Character.RoomId, to)
			}
			want := activity.Free
			if work == activity.Casting {
				want = activity.Casting
			}
			if got := m.Character.Activity.State(); got != want {
				t.Errorf("Activity = %v after the move, want %v", got, want)
			}
		})
	}
}
