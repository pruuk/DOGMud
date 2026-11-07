package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

func TestScheduleTick_NoScheduleReturnsEmptyPlan(t *testing.T) {
	mob := &mobs.Mob{ScheduleId: ""}

	plan := scheduleTickPlan(mob, 10)
	if plan.HasSchedule {
		t.Errorf("expected HasSchedule=false for empty schedule_id, got %+v", plan)
	}
}

func TestScheduleTick_UnknownScheduleReturnsEmptyPlan(t *testing.T) {
	mob := &mobs.Mob{ScheduleId: "definitely_not_real"}

	plan := scheduleTickPlan(mob, 10)
	if plan.HasSchedule {
		t.Errorf("expected HasSchedule=false for unknown schedule_id, got %+v", plan)
	}
}

func TestScheduleTick_WantsPathWhenAwayFromTarget(t *testing.T) {
	registerKerraScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "thornwall_smith"}
	mob.Character.RoomId = 1111 // not at any segment target

	plan := scheduleTickPlan(mob, 10 /* forge hour */)
	if !plan.HasSchedule {
		t.Fatalf("expected HasSchedule=true, got %+v", plan)
	}
	if !plan.WantsPath {
		t.Errorf("expected WantsPath=true when away from target, got %+v", plan)
	}
	if plan.TargetRoom != 5678 {
		t.Errorf("expected target=5678, got %d", plan.TargetRoom)
	}
}

func TestScheduleTick_NoPathWhenAtTarget(t *testing.T) {
	registerKerraScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "thornwall_smith"}
	mob.Character.RoomId = 5678 // at forge

	plan := scheduleTickPlan(mob, 10)
	if plan.WantsPath {
		t.Errorf("expected WantsPath=false when at target, got %+v", plan)
	}
}

func TestScheduleTick_SegmentTransitionDetected(t *testing.T) {
	registerKerraScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "thornwall_smith"}
	mob.Character.RoomId = 5678
	// Simulate last tick being the forge segment (Start=9). Now hour=19 is tavern.
	mob.Character.SetMiscData("schedule_last_seg_start", 9)

	plan := scheduleTickPlan(mob, 19 /* tavern hour */)
	if !plan.SegmentChanged {
		t.Errorf("expected SegmentChanged=true on transition, got %+v", plan)
	}
	if plan.NewSegmentStart != 18 {
		t.Errorf("expected NewSegmentStart=18, got %d", plan.NewSegmentStart)
	}
}

func TestScheduleTick_SegmentTransitionResetsFailCount(t *testing.T) {
	registerKerraScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "thornwall_smith"}
	mob.Character.RoomId = 1111 // not at any target

	// Pretend last tick was forge segment with 99 path failures (max retries
	// would be 20 in production config).
	mob.Character.SetMiscData("schedule_last_seg_start", 9)
	mob.Character.SetMiscData("schedule_path_fail_count", 99)

	// Now hour=19 (tavern) — segment transition. The 99-fail count from the
	// previous segment must NOT trigger WantsHomeFallback; the new segment
	// should get WantsPath against its target room (9012).
	plan := scheduleTickPlan(mob, 19)

	if !plan.SegmentChanged {
		t.Fatalf("expected SegmentChanged=true, got %+v", plan)
	}
	if plan.WantsHomeFallback {
		t.Errorf("expected WantsHomeFallback=false on segment transition with stale fail count, got %+v", plan)
	}
	if !plan.WantsPath {
		t.Errorf("expected WantsPath=true to new segment target, got %+v", plan)
	}
	if plan.TargetRoom != 9012 {
		t.Errorf("expected target=9012 (tavern), got %d", plan.TargetRoom)
	}
}

// registerKerraScheduleForTest injects the Kerra fixture and registers cleanup.
func registerKerraScheduleForTest(t *testing.T) {
	t.Helper()
	mobs.RegisterScheduleForTest(&mobs.Schedule{
		Id: "thornwall_smith",
		Segments: []mobs.ScheduleSegment{
			{Start: 6, End: 9, TargetRoom: 1234, IdleCommands: []string{"wake"}},
			{Start: 9, End: 18, TargetRoom: 5678, Activity: "craft", IdleCommands: []string{"hammer"}},
			{Start: 18, End: 22, TargetRoom: 9012, IdleCommands: []string{"sip"}},
			{Start: 22, End: 6, TargetRoom: 1234, IdleCommands: []string{"sleep"}},
		},
	})
	t.Cleanup(func() { mobs.UnregisterScheduleForTest("thornwall_smith") })
}

func TestScheduleTick_WantsSleep_AtTargetDuringSleepSegment(t *testing.T) {
	registerSleepyScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.Character.RoomId = 1234

	plan := scheduleTickPlan(mob, 23)
	if !plan.HasSchedule {
		t.Fatalf("expected HasSchedule=true")
	}
	if !plan.WantsSleep {
		t.Errorf("expected WantsSleep=true at target during sleep segment, got %+v", plan)
	}
}

// Owner 2026-11-07: a shopkeeper does not doze off with a customer (a player
// who is not hidden) in the shop; an empty shop, or a mob with no shop, still
// sleeps on schedule.
func TestScheduleTick_ShopkeeperStaysUpForACustomer(t *testing.T) {
	registerSleepyScheduleForTest(t)
	shop := &rooms.Room{RoomId: 1234, Zone: "SleepyZone"}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{1234: shop},
		map[string]*rooms.ZoneConfig{"SleepyZone": {Name: "SleepyZone", RoomId: 1234}}))

	keeper := &mobs.Mob{ScheduleId: "sleepy_test", HomeRoomId: 1234}
	keeper.Character.RoomId = 1234
	keeper.Character.Shop = characters.Shop{{ItemId: 1, Quantity: 1}}
	plain := &mobs.Mob{ScheduleId: "sleepy_test"}
	plain.Character.RoomId = 1234
	// A keeper whose shop is elsewhere (Kerra in her bedroom) is no
	// shopkeeper here (review 2026-11-07).
	away := &mobs.Mob{ScheduleId: "sleepy_test", HomeRoomId: 999}
	away.Character.RoomId = 1234
	away.Character.Shop = characters.Shop{{ItemId: 1, Quantity: 1}}

	if !scheduleTickPlan(keeper, 23).WantsSleep {
		t.Fatal("an empty shop: the keeper should sleep on schedule")
	}

	customer := users.NewTestUser(9861, "customer", "Customer", 99861)
	customer.Character.RoomId = 1234
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{9861: customer}))
	shop.AddPlayer(9861)

	if scheduleTickPlan(keeper, 23).WantsSleep {
		t.Error("a customer in the shop: the keeper should stay up")
	}
	if !scheduleTickPlan(plain, 23).WantsSleep {
		t.Error("a mob with no shop sleeps on schedule even with a player present")
	}
	if !scheduleTickPlan(away, 23).WantsSleep {
		t.Error("a keeper asleep away from their shop sleeps on schedule even with a player present")
	}
}

func TestScheduleTick_WantsSleep_FalseWhenAwayFromTarget(t *testing.T) {
	registerSleepyScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.Character.RoomId = 5678

	plan := scheduleTickPlan(mob, 23)
	if plan.WantsSleep {
		t.Errorf("expected WantsSleep=false away from target, got %+v", plan)
	}
}

// pinScheduleRoundForTest sets the global round counter and restores the
// value it found on cleanup. Restoring a fixed 0 instead left the counter
// below RoundCountMinimum for every later test, and a fresh character's
// AssistCommandedRound is also 0, so TryClaimAssistCommand refused the
// companion owner's assist in party_autoassist_sight_test.go (#478).
func pinScheduleRoundForTest(t *testing.T, round uint64) {
	t.Helper()
	prev := util.GetRoundCount()
	util.SetRoundCountForTest(round)
	t.Cleanup(func() { util.SetRoundCountForTest(prev) })
}

func TestScheduleTick_WantsSleep_FalseDuringGraceCooldown(t *testing.T) {
	registerSleepyScheduleForTest(t)
	pinScheduleRoundForTest(t, 1000)

	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.Character.RoomId = 1234
	mob.Character.SetMiscData("schedule_wake_round", 980)

	plan := scheduleTickPlan(mob, 23)
	if plan.WantsSleep {
		t.Errorf("expected WantsSleep=false during grace cooldown, got %+v", plan)
	}
}

func TestScheduleTick_WantsSleep_TrueAfterGraceCooldown(t *testing.T) {
	registerSleepyScheduleForTest(t)
	pinScheduleRoundForTest(t, 1100)

	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.Character.RoomId = 1234
	mob.Character.SetMiscData("schedule_wake_round", 1000)

	plan := scheduleTickPlan(mob, 23)
	if !plan.WantsSleep {
		t.Errorf("expected WantsSleep=true after grace cooldown, got %+v", plan)
	}
}

func TestScheduleTick_WantsWake_OnExitFromSleepSegment(t *testing.T) {
	registerSleepyScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.Character.RoomId = 1234
	mob.Character.SetMiscData("schedule_last_seg_start", 22)

	plan := scheduleTickPlan(mob, 6)
	if !plan.WantsWake {
		t.Errorf("expected WantsWake=true on exit from sleep segment, got %+v", plan)
	}
}

func registerSleepyScheduleForTest(t *testing.T) {
	t.Helper()
	mobs.RegisterScheduleForTest(&mobs.Schedule{
		Id: "sleepy_test",
		Segments: []mobs.ScheduleSegment{
			{Start: 6, End: 22, TargetRoom: 1234, Activity: "",
				IdleCommands: []string{"emote wakes."}},
			{Start: 22, End: 6, TargetRoom: 1234, Activity: "sleeping",
				IdleCommands: []string{"emote snores."}},
		},
	})
	t.Cleanup(func() { mobs.UnregisterScheduleForTest("sleepy_test") })
}

func TestApplySchedulePlan_StampsActivePatrolId_OnPatrolSegment(t *testing.T) {
	mobs.RegisterScheduleForTest(&mobs.Schedule{
		Id: "guard_sched",
		Segments: []mobs.ScheduleSegment{
			{Start: 0, End: 12, Activity: "patrol", PatrolId: "guard_patrol",
				IdleCommands: []string{"watches."}},
			{Start: 12, End: 24, TargetRoom: 9999, Activity: "",
				IdleCommands: []string{"sleeps."}},
		},
	})
	defer mobs.UnregisterScheduleForTest("guard_sched")

	mob := &mobs.Mob{ScheduleId: "guard_sched"}
	mob.Character.RoomId = 1000

	plan := scheduleTickPlan(mob, 8) // patrol segment

	applySchedulePlan(mob, plan)

	got := mob.Character.GetMiscData("active_patrol_id")
	if got == nil || got.(string) != "guard_patrol" {
		t.Errorf("expected active_patrol_id=guard_patrol, got %v", got)
	}
}

// TestScheduleTickPlan_PatrolSegmentDoesNotSetWantsPath is a regression test
// for the runtime mis-pathing bug (chunk 3.4): scheduleTickPlan must not set
// WantsPath for patrol segments. Before the fix, TargetRoom==0 for patrol
// segments caused WantsPath=true every tick, and the applier queued
// `pathto 0`, sending the guard toward the start-room alias instead of
// patrolling.
func TestScheduleTickPlan_PatrolSegmentDoesNotSetWantsPath(t *testing.T) {
	mobs.RegisterScheduleForTest(&mobs.Schedule{
		Id: "patrol_only_sched",
		Segments: []mobs.ScheduleSegment{
			{Start: 0, End: 24, Activity: "patrol", PatrolId: "some_patrol",
				IdleCommands: []string{"x"}},
		},
	})
	defer mobs.UnregisterScheduleForTest("patrol_only_sched")

	mob := &mobs.Mob{ScheduleId: "patrol_only_sched"}
	mob.Character.RoomId = 9999 // not room 0 — would set WantsPath=true if patrol-skip is missing

	plan := scheduleTickPlan(mob, 8)

	if plan.WantsPath {
		t.Errorf("expected WantsPath=false for patrol segment (patrol executor handles movement), got %+v", plan)
	}
	if plan.WantsHomeFallback {
		t.Errorf("expected WantsHomeFallback=false for patrol segment, got %+v", plan)
	}
	if plan.TargetRoom != 0 {
		t.Errorf("expected TargetRoom=0 for patrol segment (no target_room), got %d", plan.TargetRoom)
	}
}

func TestApplySchedulePlan_StampsEmptyOnNonPatrolSegment(t *testing.T) {
	drainEventQueueOnCleanup(t)
	mobs.RegisterScheduleForTest(&mobs.Schedule{
		Id: "non_patrol_sched",
		Segments: []mobs.ScheduleSegment{
			{Start: 0, End: 24, TargetRoom: 9999, Activity: "",
				IdleCommands: []string{"x"}},
		},
	})
	defer mobs.UnregisterScheduleForTest("non_patrol_sched")

	mob := &mobs.Mob{ScheduleId: "non_patrol_sched"}
	mob.Character.RoomId = 1000

	plan := scheduleTickPlan(mob, 8)

	applySchedulePlan(mob, plan)

	// Stamp should be empty string (or unset == empty when read). Patrol
	// executor reads-and-clears so empty/unset both mean "no patrol".
	got := mob.Character.GetMiscData("active_patrol_id")
	if got != nil {
		if s, ok := got.(string); ok && s != "" {
			t.Errorf("expected empty active_patrol_id on non-patrol segment, got %q", s)
		}
	}
}

// TestScheduleTick_SuppressSleepIdle_WhenWokenMidSleepWindow covers the
// 2026-07-30 report: a blacksmith roused by a shout kept emoting "turns over
// with a soft snore" while standing at her counter selling swords.
//
// The idle pool is only swapped on a segment CHANGE, so once inside a sleeping
// segment the mob keeps drawing sleep flavour for the whole window — including
// the stretch a player has woken them for (ScheduleWakeGraceRounds holds them
// up). Genuinely sleeping mobs never reach the idle emitter, so this is only
// ever visible while awake.
func TestScheduleTick_SuppressSleepIdle_WhenWokenMidSleepWindow(t *testing.T) {
	registerSleepyScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.Character.RoomId = 1234
	mob.Character.Conditions = conditions.New() // awake: no Sleeping flag

	plan := scheduleTickPlan(mob, 23 /* inside the sleep window */)
	if !plan.HasSchedule {
		t.Fatalf("expected HasSchedule=true")
	}
	if !plan.SuppressSleepIdle {
		t.Errorf("awake inside a sleep window must suppress the sleep idle pool, got %+v", plan)
	}
}

// The mirror case: outside a sleep window the pool must be left alone, or a mob
// would fall silent for most of the day.
func TestScheduleTick_SuppressSleepIdle_NotSetOutsideSleepWindow(t *testing.T) {
	registerSleepyScheduleForTest(t)

	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.Character.RoomId = 1234
	mob.Character.Conditions = conditions.New()

	plan := scheduleTickPlan(mob, 10 /* awake window */)
	if plan.SuppressSleepIdle {
		t.Errorf("awake window must keep its idle pool, got %+v", plan)
	}
}

// applySchedulePlan must actually clear the pool, not merely flag it.
func TestApplySchedulePlan_ClearsIdleCommandsWhenWokenMidSleepWindow(t *testing.T) {
	mob := &mobs.Mob{ScheduleId: "sleepy_test"}
	mob.IdleCommands = []string{"emote snores."}

	applySchedulePlan(mob, schedulePlan{HasSchedule: true, SuppressSleepIdle: true})

	if len(mob.IdleCommands) != 0 {
		t.Errorf("expected the sleep idle pool to be cleared, still have %v", mob.IdleCommands)
	}
}
