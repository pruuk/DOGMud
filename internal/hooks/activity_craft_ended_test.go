package hooks

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Messaging M6 row 7, R4: a craft that ends without finishing tells the room.

const (
	craftEndedRoomId     = 7360
	craftEndedCrafterId  = 7361
	craftEndedWatcherId  = 7362
	craftEndedRecipeId   = "craft-ended-test-recipe"
	craftEndedSkill      = "blacksmithing"
	craftEndedTagPattern = `<[^>]*>`
)

var craftEndedTag = regexp.MustCompile(craftEndedTagPattern)

func craftEndedPlain(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		out = append(out, strings.TrimSpace(craftEndedTag.ReplaceAllString(l, "")))
	}
	return out
}

// craftEndedScene stands a crafter and a watcher in one lit room. The crafter's
// Character comes from characters.New(), so OnCharacterCreated has wired the
// craft-ended hook exactly as in production.
func craftEndedScene(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		0: {SpeciesId: 0, Name: "Human", Size: species.Medium, Selectable: true},
	}))
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"city": {BiomeId: "city", Lamp: rooms.LampPtr(90)},
	}))
	// The deferred interrupt queue is package state: start and end empty.
	takeWorkInterrupts()
	t.Cleanup(func() { takeWorkInterrupts() })
	room := &rooms.Room{RoomId: craftEndedRoomId, Biome: "city"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{craftEndedRoomId: room}, map[string]*rooms.ZoneConfig{}))

	crafter := users.NewTestUser(craftEndedCrafterId, "kael", "Kael", 97361)
	crafter.Character = characters.New()
	crafter.Character.Name = "Kael"
	crafter.Character.RoomId = craftEndedRoomId
	crafter.Character.SetUserId(craftEndedCrafterId)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{
		craftEndedCrafterId: crafter,
		craftEndedWatcherId: users.NewTestUser(craftEndedWatcherId, "tob", "Tob", 97362),
	}))
	for _, id := range []int{craftEndedCrafterId, craftEndedWatcherId} {
		room.AddPlayer(id)
		users.GetByUserId(id).Character.RoomId = craftEndedRoomId
		events.DrainQueuedMessagesForTest(id)
	}

	t.Cleanup(crafting.SeedCraftingMessagesForTest(crafting.CraftingMessages{
		craftEndedSkill: {
			// One line per pool: the sender picks at random.
			StartObserver:   []string{"{actor} starts {item}."},
			SuccessObserver: []string{"{actor} finishes {item}."},
			FailureObserver: []string{"{actor} ruins it."},
			CancelObserver:  []string{"{actor} stops forging."},
			StartHeard:      []string{"Hammering starts."},
			FinishHeard:     []string{"The hammering stops."},
		},
	}))
	crafting.RegisterRecipeForTest(&crafting.RecipeSpec{RecipeId: craftEndedRecipeId, Skill: craftEndedSkill})
	t.Cleanup(func() { crafting.UnregisterRecipeForTest(craftEndedRecipeId) })
	return crafter
}

func endCraftWith(t *testing.T, crafter *users.UserRecord, trigger string) {
	t.Helper()
	c := crafter.Character
	err := c.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	if err := c.Activity.TransitionToFree(state.TransitionReason{Trigger: trigger}); err != nil {
		t.Fatalf("free: %v", err)
	}
}

func TestCraftEnded_RoomHearsWhenTheCraftStopsShort(t *testing.T) {
	for _, trigger := range []string{
		activity.TriggerCraftCancel,
		activity.TriggerMovementInterrupt,
		activity.TriggerDamageInterrupt,
	} {
		t.Run(trigger, func(t *testing.T) {
			crafter := craftEndedScene(t)
			endCraftWith(t, crafter, trigger)
			if trigger == activity.TriggerDamageInterrupt {
				// Damage narrates at the round-end flush (owner ruling
				// 2026-10-13); every other exit narrates at once.
				flushWorkInterrupts()
			}

			got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
			if len(got) != 1 || got[0] != "Kael stops forging." {
				t.Fatalf("watcher read %q, want one line %q", got, "Kael stops forging.")
			}
			own := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId))
			if trigger == activity.TriggerDamageInterrupt {
				// Hurt: the crafter is told once, in the second person (#514).
				if len(own) != 1 || own[0] != "The pain interrupts your crafting." {
					t.Fatalf("crafter read %q, want one line %q", own, "The pain interrupts your crafting.")
				}
			} else if len(own) != 0 {
				t.Fatalf("crafter was sent a room line: %q", own)
			}
		})
	}
}

func TestCraftEnded_CompletionAndDeathSendNoCancelLine(t *testing.T) {
	for _, trigger := range []string{activity.TriggerCraftComplete, activity.TriggerDeath} {
		t.Run(trigger, func(t *testing.T) {
			crafter := craftEndedScene(t)
			endCraftWith(t, crafter, trigger)

			if got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId)); len(got) != 0 {
				t.Fatalf("watcher read %q, want nothing", got)
			}
		})
	}
}

const craftEndedMobId = 98361

// A mob crafter reaches the same hook: the lit watcher reads the mob's
// cancel line, named by what the room can see of it.
func TestCraftEnded_MobCrafterStopsAndTheRoomReadsIt(t *testing.T) {
	craftEndedScene(t)
	room := rooms.LoadRoom(craftEndedRoomId)

	m := &mobs.Mob{InstanceId: craftEndedMobId}
	m.Character.Name = "Smith"
	m.Character.MobInstanceId = craftEndedMobId
	m.Character.RoomId = craftEndedRoomId
	m.Character.Validate() // fires OnCharacterCreated, wiring the hook
	// A live mob: the hook skips the dying.
	m.Character.Health = 10
	mobs.SetInstanceForTest(craftEndedMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(craftEndedMobId, nil) })
	room.AddMob(craftEndedMobId)

	err := m.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	if err := m.Character.Activity.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerCraftCancel}); err != nil {
		t.Fatalf("free: %v", err)
	}

	got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
	if len(got) != 1 || got[0] != "Smith stops forging." {
		t.Fatalf("watcher read %q, want one line %q", got, "Smith stops forging.")
	}
}

// A player crafter who is hit is told the work stopped, as one who walks
// away is; the watcher reads the cancel line. A mob is told nothing.
func TestApplyHarm_PlayerCrafterIsTold(t *testing.T) {
	crafter := craftEndedScene(t)
	err := crafter.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	crafter.Character.Health = 20
	crafter.Character.ApplyHarm(characters.PoolHealth, 1, state.ActorRef{})
	flushWorkInterrupts()

	own := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId))
	if len(own) != 1 || own[0] != "The pain interrupts your crafting." {
		t.Fatalf("crafter read %q, want one line %q", own, "The pain interrupts your crafting.")
	}
	got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
	if len(got) != 1 || got[0] != "Kael stops forging." {
		t.Fatalf("watcher read %q, want one line %q", got, "Kael stops forging.")
	}
	// Already Free: a second hit says nothing.
	crafter.Character.ApplyHarm(characters.PoolHealth, 1, state.ActorRef{})
	flushWorkInterrupts()
	if own := events.DrainQueuedMessagesForTest(craftEndedCrafterId); len(own) != 0 {
		t.Fatalf("a hit on an idle crafter sent %q", own)
	}
}

// The mob half: a crafting mob that is hit is told nothing (it has no
// connection to tell), and the lit watcher reads the stop line exactly once.
func TestApplyHarm_MobCrafterSendsNoCrafterLine(t *testing.T) {
	craftEndedScene(t)
	room := rooms.LoadRoom(craftEndedRoomId)

	m := &mobs.Mob{InstanceId: craftEndedMobId}
	m.Character.Name = "Smith"
	m.Character.MobInstanceId = craftEndedMobId
	m.Character.RoomId = craftEndedRoomId
	m.Character.Validate() // fires OnCharacterCreated, wiring the hook
	m.Character.Health = 10
	mobs.SetInstanceForTest(craftEndedMobId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(craftEndedMobId, nil) })
	room.AddMob(craftEndedMobId)

	err := m.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	m.Character.ApplyHarm(characters.PoolHealth, 1, state.ActorRef{})
	flushWorkInterrupts()

	got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
	if len(got) != 1 || got[0] != "Smith stops forging." {
		t.Fatalf("watcher read %q, want one line %q", got, "Smith stops forging.")
	}
	// The scene's player Kael stands in the room too: he reads the stop line
	// and nothing else, in particular no "your crafting" line.
	other := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId))
	if len(other) != 1 || other[0] != "Smith stops forging." {
		t.Fatalf("bystander read %q, want only %q", other, "Smith stops forging.")
	}
}

// A hit on a salvaging player is told as walking away is; nobody else hears.
func TestApplyHarm_PlayerSalvagerIsTold(t *testing.T) {
	crafter := craftEndedScene(t)
	err := crafter.Character.Activity.TransitionToSalvaging(
		activity.SalvagingData{},
		state.TransitionReason{Trigger: activity.TriggerSalvageBegin},
	)
	if err != nil {
		t.Fatalf("begin salvage: %v", err)
	}
	crafter.Character.Health = 20
	crafter.Character.ApplyHarm(characters.PoolHealth, 1, state.ActorRef{})
	if st := crafter.Character.Activity.State(); st != activity.Free {
		t.Fatalf("activity is %v after a hit, want Free at once", st)
	}
	// Deferred: nothing until the round-end flush.
	if own := events.DrainQueuedMessagesForTest(craftEndedCrafterId); len(own) != 0 {
		t.Fatalf("salvager read %q before the flush, want nothing yet", own)
	}
	flushWorkInterrupts()

	own := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId))
	if len(own) != 1 || own[0] != "The pain interrupts your salvaging." {
		t.Fatalf("salvager read %q, want one line %q", own, "The pain interrupts your salvaging.")
	}
	if got := events.DrainQueuedMessagesForTest(craftEndedWatcherId); len(got) != 0 {
		t.Fatalf("watcher heard %q, want nothing", got)
	}
}

// A killing blow through the real seam (ApplyHarm, health 1, harm 5) frees the
// craft but tells nobody anything: no room stop line, no crafter line. The
// death narration covers it.
func TestApplyHarm_KillingBlowOnCrafterSendsNoLines(t *testing.T) {
	crafter := craftEndedScene(t)
	crafter.Character.Health = 1
	err := crafter.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	crafter.Character.ApplyHarm(characters.PoolHealth, 5, state.ActorRef{})
	flushWorkInterrupts()

	if st := crafter.Character.Activity.State(); st != activity.Free {
		t.Fatalf("activity is %v after a killing blow, want Free", st)
	}
	if got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId)); len(got) != 0 {
		t.Fatalf("watcher read %q on a killing blow, want nothing", got)
	}
	if own := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId)); len(own) != 0 {
		t.Fatalf("crafter read %q on a killing blow, want nothing", own)
	}
}

// Owner ruling 2026-10-13: a killing blow that interrupts the craft sends the
// room no stop line. Lethal damage leaves the crafter below one health but
// still Alive (CharacterDied is only queued), so the hook must read health.
func TestCraftEnded_KillingBlowSendsNoStopLine(t *testing.T) {
	crafter := craftEndedScene(t)
	crafter.Character.Health = -3
	endCraftWith(t, crafter, activity.TriggerDamageInterrupt)
	flushWorkInterrupts()
	if got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId)); len(got) != 0 {
		t.Fatalf("watcher read %q on a killing blow, want nothing", got)
	}

	crafter = craftEndedScene(t)
	crafter.Character.Health = 1
	endCraftWith(t, crafter, activity.TriggerDamageInterrupt)
	flushWorkInterrupts()
	got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
	if len(got) != 1 || got[0] != "Kael stops forging." {
		t.Fatalf("watcher read %q on a survived blow, want one line %q", got, "Kael stops forging.")
	}
}

const craftEndedHitLine = "The ogre clubs you."

// Owner ruling 2026-10-13, option A: the craft ends the instant damage lands,
// but its lines wait for the round-end flush, so the crafter reads the hit
// first and the pain second. Before the flush neither the crafter nor the
// room has been told anything.
func TestApplyHarm_InterruptLinesFollowTheHitLine(t *testing.T) {
	crafter := craftEndedScene(t)
	// Start from an empty queue: the drain helpers read the heap's backing
	// array, which pushes alone keep in insertion order.
	events.DrainAllQueuedEventsForTest()
	err := crafter.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	crafter.Character.Health = 20
	// Damage lands first, as in rollCombatAttack: the hit line is sent after.
	crafter.Character.ApplyHarm(characters.PoolHealth, 1, state.ActorRef{})

	if st := crafter.Character.Activity.State(); st != activity.Free {
		t.Fatalf("activity is %v after a hit, want Free at once", st)
	}
	if own := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId)); len(own) != 0 {
		t.Fatalf("crafter read %q before the flush, want nothing yet", own)
	}
	if got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId)); len(got) != 0 {
		t.Fatalf("watcher read %q before the flush, want nothing yet", got)
	}

	crafter.SendText(messaging.CategoryHitBlunt, craftEndedHitLine)
	flushWorkInterrupts()

	own := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId))
	want := []string{craftEndedHitLine, "The pain interrupts your crafting."}
	if len(own) != 2 || own[0] != want[0] || own[1] != want[1] {
		t.Fatalf("crafter read %q, want %q in that order", own, want)
	}
	got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
	if len(got) != 1 || got[0] != "Kael stops forging." {
		t.Fatalf("watcher read %q, want one line %q", got, "Kael stops forging.")
	}
}

// A non-damage exit is not deferred: a cancel narrates at once and queues
// nothing for the flush.
func TestCraftEnded_CancelNarratesWithoutAFlush(t *testing.T) {
	crafter := craftEndedScene(t)
	endCraftWith(t, crafter, activity.TriggerCraftCancel)

	got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId))
	if len(got) != 1 || got[0] != "Kael stops forging." {
		t.Fatalf("watcher read %q with no flush, want one line %q", got, "Kael stops forging.")
	}
	if pending := takeWorkInterrupts(); len(pending) != 0 {
		t.Fatalf("a cancel queued %d deferred interrupts, want none", len(pending))
	}
}

// A crafter hurt and then killed later in the same round is told nothing at
// the flush: the death narration covers it, as for a single killing blow.
func TestFlushWorkInterrupts_DropsACrafterKilledLaterInTheRound(t *testing.T) {
	crafter := craftEndedScene(t)
	err := crafter.Character.Activity.TransitionToCrafting(
		activity.CraftingData{RecipeId: craftEndedRecipeId, RoundsTotal: 5},
		state.TransitionReason{Trigger: activity.TriggerCraftBegin},
	)
	if err != nil {
		t.Fatalf("begin craft: %v", err)
	}
	crafter.Character.Health = 20
	crafter.Character.ApplyHarm(characters.PoolHealth, 1, state.ActorRef{})
	crafter.Character.ApplyHarm(characters.PoolHealth, 50, state.ActorRef{})
	flushWorkInterrupts()

	if own := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedCrafterId)); len(own) != 0 {
		t.Fatalf("crafter read %q after dying in the round, want nothing", own)
	}
	if got := craftEndedPlain(events.DrainQueuedMessagesForTest(craftEndedWatcherId)); len(got) != 0 {
		t.Fatalf("watcher read %q after the crafter died in the round, want nothing", got)
	}
}
