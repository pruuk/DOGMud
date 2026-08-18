package usercommands

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/costs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// U5b-2: flee moved from refuse to partial charge. An exhausted character MUST
// still get to flee: go.go refuses all movement while in combat, so fleeing is
// the only player-initiated disengage. Refusing it at zero stamina would leave
// no alternative action that changes the character's situation.
func TestFleeCost_ShortAttemptCommitsAvailableStaminaAndCarriesNoSkill(t *testing.T) {
	u, room, cleanup := fleeFixture(t, 9250, 99842, 0)
	defer cleanup()
	u.Character.Stamina = 3
	engageFleeFixture(t, u)

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}

	if u.Character.Stamina != 0 {
		t.Errorf("stamina after short flee = %d, want 0", u.Character.Stamina)
	}
	if !u.Character.IsDisengaging() {
		t.Fatalf("short flee left state %v, want Disengaging", u.Character.CombatPhase.State())
	}
	includeSkill, admitted := TakeFleeAdmission(u)
	if !admitted || includeSkill {
		t.Error("short flee retained Skullduggery eligibility")
	}
	if _, admitted := TakeFleeAdmission(u); admitted {
		t.Error("a consumed flee admission was reusable")
	}
	if got := u.Character.GetSkillUseCount(string(costs.SpecFor(costs.ActionFlee).Skill)); got != 0 {
		t.Errorf("short flee progressed Skullduggery %d times, want 0", got)
	}

	shortageLines := 0
	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		if strings.Contains(msg, "instinct rather than technique") {
			shortageLines++
		}
	}
	if shortageLines != 1 {
		t.Errorf("shortage lines = %d, want exactly 1 per command", shortageLines)
	}
}

// Catches leaving a short admission behind when CombatPhase rejects the
// transition. A later legacy flee must not inherit a canceled attempt's state.
func TestFleeCost_TransitionVetoLeavesNoAdmission(t *testing.T) {
	u, room, cleanup := fleeFixture(t, 9256, 99848, 0)
	defer cleanup()
	engageFleeFixture(t, u)
	before := u.Character.Stamina
	u.Character.CombatPhase.RegisterPositionCheck(func() bool { return false })

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if u.Character.IsDisengaging() {
		t.Fatal("position-vetoed flee entered Disengaging")
	}
	if _, admitted := TakeFleeAdmission(u); admitted {
		t.Fatal("transition-vetoed short admission leaked into a later resolution")
	}
	if u.Character.Stamina != before {
		t.Errorf("position-vetoed flee spent stamina: got %d, want %d", u.Character.Stamina, before)
	}
	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		if strings.Contains(msg, "attempt to flee") ||
			strings.Contains(msg, "instinct rather than technique") {
			t.Errorf("position-vetoed flee published a nonexistent attempt: %q", msg)
		}
	}
}

// A target-death cascade can force the character to Idle after Flee's initial
// in-combat check but before the Disengaging transition is admitted. That
// rejected transition must be indistinguishable from the earlier queued-input
// race: no cost, no attempt banner, and one terminal explanation.
func TestFleeCost_TransitionRaceToIdleDoesNotPayOrClaimAttempt(t *testing.T) {
	u, room, cleanup := fleeFixture(t, 9259, 99852, 0)
	defer cleanup()
	engageFleeFixture(t, u)
	before := u.Character.Stamina
	u.Character.CombatPhase.RegisterPositionCheck(func() bool {
		u.Character.CombatPhase.ForceIdle(state.TransitionReason{
			Trigger: combatphase.TriggerTargetDied,
		})
		return true
	})

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}

	if u.Character.Stamina != before {
		t.Errorf("transition-raced flee spent stamina: got %d, want %d", u.Character.Stamina, before)
	}
	if u.Character.IsDisengaging() {
		t.Fatal("transition-raced flee entered Disengaging")
	}
	if _, admitted := TakeFleeAdmission(u); admitted {
		t.Fatal("transition-raced flee published an admission with no resolver")
	}

	var sawTerminal, sawAttempt bool
	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		sawTerminal = sawTerminal || strings.Contains(msg, "not in combat")
		sawAttempt = sawAttempt || strings.Contains(msg, "attempt to flee") ||
			strings.Contains(msg, "instinct rather than technique")
	}
	if !sawTerminal {
		t.Error("transition-raced flee gave no terminal explanation")
	}
	if sawAttempt {
		t.Error("transition-raced flee claimed an escape attempt began")
	}
}

// A flee command can be accepted from the socket while the player is fighting
// but reach the command handler after the same round kills and respawns them.
// The death cascade has already forced CombatPhase back to Idle by then. The
// stale command must not spend from the revived character or claim that an
// escape attempt began when no round resolver can ever produce its outcome.
func TestFleeCost_IdleRaceAfterDeathDoesNotPayOrClaimAttempt(t *testing.T) {
	u, room, cleanup := fleeFixture(t, 9258, 99851, 0)
	defer cleanup()
	before := u.Character.Stamina

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}

	if u.Character.Stamina != before {
		t.Errorf("idle stale flee spent stamina: got %d, want %d", u.Character.Stamina, before)
	}
	if u.Character.IsDisengaging() {
		t.Fatal("idle stale flee entered Disengaging")
	}
	if _, admitted := TakeFleeAdmission(u); admitted {
		t.Fatal("idle stale flee published an admission with no resolver")
	}

	var sawTerminal, sawAttempt bool
	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		sawTerminal = sawTerminal || strings.Contains(msg, "not in combat")
		sawAttempt = sawAttempt || strings.Contains(msg, "attempt to flee") ||
			strings.Contains(msg, "instinct rather than technique")
	}
	if !sawTerminal {
		t.Error("idle stale flee gave no terminal explanation")
	}
	if sawAttempt {
		t.Error("idle stale flee claimed an escape attempt began")
	}
}

// Catches returning through a command-level rejection before retracting an
// orphaned handoff. A rejected command cannot leave an earlier attempt's
// admission available to an asynchronous round resolver.
func TestFleeCost_RejectedCommandClearsOrphanedAdmission(t *testing.T) {
	const noFleeBuffID = 99849
	cleanupBuffs := buffs.SeedBuffsForTest(map[int]*buffs.BuffSpec{
		noFleeBuffID: {
			BuffId:        noFleeBuffID,
			Name:          "test no-flee",
			Description:   "rejects a flee command",
			RoundInterval: 1,
			TriggerCount:  1,
			Flags:         []buffs.Flag{buffs.NoFlee},
		},
	})
	defer cleanupBuffs()

	u, room, cleanup := fleeFixture(t, 9257, 99850, 0)
	defer cleanup()
	u.SetTempData(fleeIncludeSkillTempKey, fleeAdmission{includeSkill: false})
	if !u.Character.Buffs.AddBuff(noFleeBuffID, false) {
		t.Fatal("fixture could not apply the no-flee buff")
	}

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if _, admitted := TakeFleeAdmission(u); admitted {
		t.Fatal("rejected command left an orphaned flee admission reusable")
	}
}

// The transition is visible to the round driver before the command has
// finished committing its cost decision. A reentrant resolver must not consume
// that pending marker, while terminal cancellation still must be able to.
func TestTakeFleeAdmission_PendingHandoffWaitsForCommandOrCancellation(t *testing.T) {
	u, _, cleanup := fleeFixture(t, 9260, 99853, 0)
	defer cleanup()
	u.SetTempData(fleeIncludeSkillTempKey, fleeAdmission{})

	if _, admitted := TakeFleeAdmission(u); admitted {
		t.Fatal("round resolver consumed a pending flee handoff")
	}
	if !CancelFleeAdmission(u) {
		t.Fatal("terminal cancellation could not retract a pending flee handoff")
	}
	if CancelFleeAdmission(u) {
		t.Fatal("pending flee handoff was canceled more than once")
	}
}

// Catches treating a positive fractional quote as short merely because the
// pool is empty. With literal 1 x 1.10 x 0.5 = 0.55, the first attempt owes no
// whole point and is eligible; the second carries to 1.10, owes one, and is
// exactly partially paid at zero Stamina.
func TestFleeCost_ZeroStaminaFractionalCarryBecomesShortOnlyWhenWholeDue(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.FleeStaminaCost = 1
	cfg.Balance.FlightFleeStaminaMult = 0.5
	configs.SetConfigForTest(t, cfg)
	cleanupMutations := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"winged-flight": {
			MutationId: "winged-flight", Name: "Winged Flight", Rarity: 8,
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "flying"}},
		},
	})
	defer cleanupMutations()

	u, room, cleanup := fleeFixture(t, 9257, 99849, 0)
	defer cleanup()
	u.Character.Mutations = map[string]int{"winged-flight": 1}
	u.Character.Stamina = 0
	engageFleeFixture(t, u)

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("first Flee returned %v", err)
	}
	includeSkill, admitted := TakeFleeAdmission(u)
	if !admitted || !includeSkill {
		t.Fatal("0.55 quote at zero Stamina was marked short before a whole point was due")
	}
	u.Character.CombatPhase.ResolveFlee(false)
	events.DrainQueuedMessagesForTest(u.UserId)

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("second Flee returned %v", err)
	}
	includeSkill, admitted = TakeFleeAdmission(u)
	if !admitted || includeSkill {
		t.Fatal("carried 1.10 quote at zero Stamina was not reported partially paid")
	}
	if u.Character.Stamina != 0 {
		t.Fatalf("partial fractional commit changed zero Stamina to %d", u.Character.Stamina)
	}
	shortageLines := 0
	for _, msg := range events.DrainQueuedMessagesForTest(u.UserId) {
		if strings.Contains(msg, "instinct rather than technique") {
			shortageLines++
		}
	}
	if shortageLines != 1 {
		t.Fatalf("second fractional attempt shortage lines = %d, want 1", shortageLines)
	}
}

// fleeFixture builds a player loaded to the given fraction of carry capacity,
// seeded into the user manager so Flee's SendText has somewhere to go.
//
// Capacity is pinned to a round 100 and overridden rather than derived from
// Strength, so the load ratio does not move when the stat defaults do. itemId
// must be unique per call: the item spec registry is package-global and shared
// across the whole test binary.
func fleeFixture(t *testing.T, userId, itemId int, fraction float64) (*users.UserRecord, *rooms.Room, func()) {
	t.Helper()

	u := users.NewTestUser(userId, "fleer", "Fleer", uint64(userId))
	u.Character = characters.New()
	u.Character.Name = "Fleer"
	u.Character.SetUserId(userId)
	u.Character.Validate()

	const capacity = 100.0
	if fraction > 0 {
		items.RegisterTestItemSpec(&items.ItemSpec{
			ItemId: itemId,
			Name:   "test lead pig",
			Weight: capacity * fraction,
		})
		u.Character.Items = append(u.Character.Items, items.Item{ItemId: itemId})
	}
	characters.ApplyMobOverrides(u.Character, 0, 0, capacity)

	// Big enough that even the dearest flee this table prices is affordable,
	// so the reading is the COST and not the size of the pool.
	u.Character.StaminaMax.Base = 500
	u.Character.StaminaMax.Recalculate()
	u.Character.Stamina = 500

	if got := u.Character.CarryCapacity(); got != capacity {
		t.Fatalf("fixture capacity is %.1f, want %.1f; the override did not take", got, capacity)
	}

	cleanUsers := users.SeedUsersForTest(map[int]*users.UserRecord{userId: u})
	room := &rooms.Room{RoomId: 999903}
	events.DrainQueuedMessagesForTest(userId)

	return u, room, func() {
		events.DrainQueuedMessagesForTest(userId)
		cleanUsers()
	}
}

func engageFleeFixture(t *testing.T, u *users.UserRecord) {
	t.Helper()
	if err := u.Character.CombatPhase.TransitionToEngaging(
		combatphase.EngagingData{Target: state.ActorRef{MobInstanceId: 4242}},
		state.TransitionReason{Trigger: combatphase.TriggerAttackCommand},
	); err != nil {
		t.Fatalf("fixture could not enter combat: %v", err)
	}
	u.Character.CombatPhase.OnRoundTick()
}

// Catches applying flight after quoting (where it cannot affect the immutable
// amount), omitting it, or returning to the old manual float debit.
func TestFleeCost_FlyingModifiesTheQuotedBaseBeforeCommit(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.FleeStaminaCost = 10
	cfg.Balance.FlightFleeStaminaMult = 0.5
	configs.SetConfigForTest(t, cfg)
	cleanupMutations := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"winged-flight": {
			MutationId: "winged-flight", Name: "Winged Flight", Rarity: 8,
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "flying"}},
		},
	})
	defer cleanupMutations()

	u, room, cleanup := fleeFixture(t, 9255, 99847, 0)
	defer cleanup()
	u.Character.Mutations = map[string]int{"winged-flight": 1}
	engageFleeFixture(t, u)
	before := u.Character.Stamina

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}
	if spent := before - u.Character.Stamina; spent != 5 {
		t.Errorf("flying flee charged %d, want literal 5 from base 10 x modifier 0.5", spent)
	}
}

// THE ASSERTION THAT BITES IF FLEE STOPS TAKING THE ENCUMBRANCE TERM. Before
// U7 flee was a flat int with no load term at all, so breaking away with a third
// more than your capacity on your back cost exactly what breaking away
// empty-handed cost, which quietly undid the premise that what you carry is a
// decision.
//
// Drives the real Flee command rather than re-deriving its arithmetic, so a
// regression that leaves the registry entry intact but stops routing the command
// through costs.Calc still fails here.
func TestFleeCost_TakesTheEncumbranceTerm(t *testing.T) {
	bal := configs.GetBalanceConfig()
	spec := costs.SpecFor(costs.ActionFlee)

	if !spec.Physical {
		t.Fatal("the flee registry entry is not Physical; encumbrance cannot reach it")
	}

	cases := []struct {
		name     string
		userId   int
		itemId   int
		fraction float64
	}{
		{"empty-handed", 9251, 99843, 0},
		{"crushed", 9252, 99844, 1.32},
	}

	spent := make(map[string]int, len(cases))

	for _, tc := range cases {
		u, room, cleanup := fleeFixture(t, tc.userId, tc.itemId, tc.fraction)
		engageFleeFixture(t, u)

		before := u.Character.Stamina
		if _, err := Flee("", u, room, 0); err != nil {
			cleanup()
			t.Fatalf("%s: Flee returned %v", tc.name, err)
		}
		spent[tc.name] = before - u.Character.Stamina

		// Composed from the same registry entry the command uses, so the
		// message names the knob that moved rather than only the mismatch.
		want := int(math.Floor(costs.Calc(costs.Input{
			Base:      float64(bal.FleeStaminaCost),
			Carried:   u.Character.GetCarriedWeight(),
			Capacity:  u.Character.CarryCapacity(),
			Physical:  spec.Physical,
			SkillRank: u.Character.GetSkillLevel(spec.Skill),
			HasSkill:  spec.SkillSource != costs.SkillNone,
		})))
		if spent[tc.name] != want {
			t.Errorf("%s: flee charged %d stamina, want %d; the command is not "+
				"pricing through costs.Calc", tc.name, spent[tc.name], want)
		}
		cleanup()
	}

	if spent["crushed"] <= spent["empty-handed"] {
		t.Errorf("fleeing crushed costs %d and fleeing empty-handed costs %d; load "+
			"must make escape dearer, and this is exactly the flat charge U7 "+
			"replaced", spent["crushed"], spent["empty-handed"])
	}
}

// Flee NEVER refuses for lack of stamina (U5b-2). go.go refuses all movement
// while in combat, so fleeing is the only player-initiated disengage; an
// exhausted character who could not flee would have no action left that changes
// their situation. Pricing it through costs.Calc must not have changed that.
func TestFleeCost_ExhaustedCharacterStillGetsToAttempt(t *testing.T) {
	u, room, cleanup := fleeFixture(t, 9253, 99845, 1.32)
	defer cleanup()

	u.Character.Stamina = 0
	engageFleeFixture(t, u)

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("Flee returned %v", err)
	}

	if u.Character.Stamina != 0 {
		t.Errorf("stamina is %d after fleeing on an empty pool, want 0", u.Character.Stamina)
	}

	var attempted bool
	for _, msg := range events.DrainQueuedMessagesForTest(9253) {
		if strings.Contains(msg, "attempt to flee") {
			attempted = true
		}
	}
	if !attempted {
		t.Error("an exhausted character was not told they attempt to flee; flee " +
			"must never refuse for cost")
	}
}

// A second flee while the first is still resolving used to print nothing at all.
// A playtester typed flee twice at zero stamina, saw one line and then silence,
// and died reading the silence as the command being swallowed.
func TestFleeCost_ASecondFleeSaysSomething(t *testing.T) {
	u, room, cleanup := fleeFixture(t, 9254, 99846, 0)
	defer cleanup()

	// Disengaging is only reachable from Engaging/Engaged, so put the fixture
	// in a fight first. Without this the second-flee branch is unreachable and
	// the test would pass on nothing.
	engageFleeFixture(t, u)

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("first Flee returned %v", err)
	}
	if !u.Character.IsDisengaging() {
		t.Fatalf("first flee left the character in %v, not Disengaging; the "+
			"second-flee path would not be exercised", u.Character.CombatPhase.State())
	}
	events.DrainQueuedMessagesForTest(9254)

	if _, err := Flee("", u, room, 0); err != nil {
		t.Fatalf("second Flee returned %v", err)
	}

	if got := events.DrainQueuedMessagesForTest(9254); len(got) == 0 {
		t.Error("a second flee printed nothing at all; the player reads silence as " +
			"the command being swallowed")
	}
}
