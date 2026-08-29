package actions

import (
	"github.com/GoMudEngine/GoMud/internal/state"
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// U10d: RangedUnengagedDamageMultiplier.
//
// The bow's flat damage_multiplier came down onto the melee band in the
// previous commit. The inflation it removed was paying for something real -- a
// shot is ONE attack where melee swings up to four times, and the reload burns
// the shared special-move timer -- so the compensation moves from flat to
// SITUATIONAL: a shot carries the multiplier only while nothing in the
// SHOOTER's room has the shooter as its aggro target.
//
// Nine cases below. Two of them (5 and 6) are controls that would pass with
// the feature absent; that is stated on each. The rest would not:
//
//	1. unengaged carries the multiplier                       -- fails at 1.0x
//	2. it drops mid-fight the moment something turns on you   -- kills "always true"
//	3. cross-room reads the SHOOTER's room, both directions   -- kills "scan targetRoom"
//	4. it COMPOUNDS with the surprise opener (product, not max)
//	5. melee is untouched                                     (control)
//	6. the knob at 1.0 is a true no-op                        (control)
//	7. your OWN companion aggroed on you is not "someone is hitting me"
//	8. the TARGET reciprocating ends your own bonus (the headline claim)
//	9. a mob shooter cannot borrow a player's charm by id collision
// ---------------------------------------------------------------------------

const (
	// The shipped value is 2.75; pinned here so a config edit cannot move what
	// these tests are measuring.
	unengagedKnob = 2.75

	// The shooter needs a real user id: shooterIsUnengaged matches an
	// attacker's Aggro against GetUserId()/MobInstanceId, and a zero-identity
	// fixture can never be matched by anything.
	unengagedShooterUserId = 7

	// Instance ids for the two "watchers" -- bystander mobs that are in combat
	// throughout, so the only thing that ever changes between phases is WHO
	// they are aggroed on.
	unengagedWatcherHere  = 501 // lives in the shooter's room
	unengagedWatcherThere = 502 // lives in the target's room

	// Aggro pointed at a third party. Non-nil and non-zero so the watcher is
	// still returned by GetMobs(FindFighting) -- the room population must be
	// identical in every phase.
	unengagedOtherVictim = 999

	unengagedSamples   = 80
	unengagedTolerance = 0.10

	// A MOB shooter's instance id, chosen to collide with a plausible PLAYER
	// user id -- that collision is the whole point of case 9.
	unengagedMobShooterInstanceId = 3
)

// unengagedWatcher describes a bystander mob to place in the fixture.
// charmedBy, when non-zero, makes it the companion of that PLAYER user id.
type unengagedWatcher struct {
	instanceId int
	roomId     int
	aggro      state.ActorRef
	charmedBy  int
}

// aggroOnShooter and aggroElsewhere are the two states a watcher can be in.
// U12c-2: these are target REFERENCES now, applied through SetAggro, rather
// than Aggro structs assigned into the field.
func aggroOnShooter() state.ActorRef {
	return state.ActorRef{UserId: unengagedShooterUserId}
}

func aggroElsewhere() state.ActorRef {
	return state.ActorRef{MobInstanceId: unengagedOtherVictim}
}

// seedUnengagedFire seeds room 1 (the shooter's room) with a north exit to
// room 2, puts the skeleton target in defenderRoomId, and places each watcher.
// Modelled on seedFireMobInRoom; it needs a second mob template and arbitrary
// bystanders, which that helper cannot express.
func seedUnengagedFire(t *testing.T, defenderRoomId int, watchers ...unengagedWatcher) func() {
	t.Helper()

	mobSpecs := map[int]*mobs.Mob{
		1: {MobId: 1, Zone: "TestZone", Character: characters.Character{Name: "Skeleton"}},
		2: {MobId: 2, Zone: "TestZone", Character: characters.Character{Name: "Watcher"}},
	}

	var defChar characters.Character
	defChar.Name = "Skeleton"
	defChar.RoomId = defenderRoomId
	defChar.Health = surpriseMobHealth
	defChar.Buffs = buffs.New()
	defChar.Cooldowns = map[string]int{}
	defChar.Stats.Dexterity.ValueAdj = 1

	mobInstances := map[int]*mobs.Mob{
		500: {MobId: 1, InstanceId: 500, HomeRoomId: defenderRoomId, Character: defChar},
	}
	for _, w := range watchers {
		var wc characters.Character
		wc.Name = "Watcher"
		wc.RoomId = w.roomId
		wc.Health = 1000
		wc.Buffs = buffs.New()
		wc.Cooldowns = map[string]int{}
		wc.MobInstanceId = w.instanceId
		// Charmed set directly, and the aggro after it: Character.Charm()
		// clears aggro that points at the charmer, so calling it here would
		// erase the very state this reproduces. The live holes (steal, plant,
		// the behaviour-tree mob_idle attack fallback) all set the aggro long
		// after the charm, and so arrive at exactly this shape.
		if w.charmedBy > 0 {
			wc.Charmed = characters.NewCharm(w.charmedBy, characters.CharmPermanent, "")
		}
		wc.SetAggro(w.aggro.UserId, w.aggro.MobInstanceId, characters.DefaultAttack)
		mobInstances[w.instanceId] = &mobs.Mob{
			MobId: 2, InstanceId: w.instanceId, HomeRoomId: w.roomId, Character: wc,
		}
	}
	cleanupMobs := mobs.SeedMobsForTest(mobSpecs, mobInstances)

	room1 := &rooms.Room{
		RoomId: 1, Zone: "TestZone", Title: "Room One", Biome: "city",
		Exits: map[string]exit.RoomExit{"north": {RoomId: 2}},
	}
	room2 := &rooms.Room{RoomId: 2, Zone: "TestZone", Title: "Room Two", Biome: "city"}
	cleanupRooms := rooms.SeedRoomsForTest(
		map[int]*rooms.Room{1: room1, 2: room2},
		map[string]*rooms.ZoneConfig{
			"TestZone": {Name: "TestZone", RoomId: 1, RoomIds: map[int]struct{}{1: {}, 2: {}}},
		},
	)

	byId := map[int]*rooms.Room{1: room1, 2: room2}
	counts := map[int]int{}
	byId[defenderRoomId].AddMob(500)
	counts[defenderRoomId]++
	for _, w := range watchers {
		byId[w.roomId].AddMob(w.instanceId)
		counts[w.roomId]++
	}
	for roomId, ct := range counts {
		rooms.MarkRoomOccupancy(roomId, 0, ct)
	}

	return func() {
		cleanupRooms()
		cleanupMobs()
	}
}

// setUnengagedKnob overrides RangedUnengagedDamageMultiplier on top of
// whatever pinRangedSurpriseBalance already pinned. SetConfigForTest stacks
// its restores, so calling this twice inside one test is safe.
func setUnengagedKnob(t *testing.T, v float64) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.RangedUnengagedDamageMultiplier = configs.ConfigFloat(v)
	configs.SetConfigForTest(t, cfg)
}

// newUnengagedShooter is newSurpriseShooter with an identity, so an attacker's
// Aggro has something to point at.
func newUnengagedShooter(hidden bool) *characters.Character {
	char := newSurpriseShooter(hidden)
	char.SetUserId(unengagedShooterUserId)
	return char
}

// newUnengagedMobShooter builds a MOB archer: no user id, a mob instance id.
func newUnengagedMobShooter() *characters.Character {
	char := newSurpriseShooter(false)
	char.MobInstanceId = unengagedMobShooterInstanceId
	return char
}

// setWatcherAggro re-points a seeded watcher without disturbing anything else.
func setWatcherAggro(t *testing.T, instanceId int, aggro state.ActorRef) {
	t.Helper()
	w := mobs.GetInstance(instanceId)
	require.NotNil(t, w, "watcher %d must be seeded", instanceId)
	w.Character.SetAggro(aggro.UserId, aggro.MobInstanceId, characters.DefaultAttack)
}

// sampleShotsFromOneShooter fires n shots from the SAME shooter object,
// restoring between shots only what a shot spends -- the weapon's load, the
// shooter's stamina (it feeds the resource-depletion damage multiplier), and
// the target's health. Nothing about engagement is reset, which is the point:
// the drop in case 2 happens inside one continuous fight.
func sampleShotsFromOneShooter(t *testing.T, char *characters.Character, rest string, n int) (float64, FireResult) {
	t.Helper()
	target := mobs.GetInstance(500)
	require.NotNil(t, target)
	actor := newStubActor(char, rooms.LoadRoom(1))

	total := 0
	var last FireResult
	for i := 0; i < n; i++ {
		target.Character.Health = surpriseMobHealth
		char.Stamina = char.StaminaMax.Value
		char.Equipment.Weapon.Loaded = true
		res := ExecuteFire(actor, rest)
		require.True(t, res.Executed, "sample %d: the shot must execute", i)
		require.True(t, res.MoveResult.Hit, "sample %d: the deterministic win must land", i)
		total += res.MoveResult.Damage
		last = res
	}
	return float64(total) / float64(n), last
}

// sampleFreshShooterDamage builds a NEW shooter per shot, which the stealth
// opener needs (it claims the one-shot special-move cooldown).
func sampleFreshShooterDamage(t *testing.T, hidden bool, n int) float64 {
	t.Helper()
	target := mobs.GetInstance(500)
	require.NotNil(t, target)

	total := 0
	for i := 0; i < n; i++ {
		target.Character.Health = surpriseMobHealth
		char := newUnengagedShooter(hidden)
		res := ExecuteFire(newStubActor(char, rooms.LoadRoom(1)), "skeleton")
		require.True(t, res.Executed, "sample %d: the shot must execute", i)
		require.True(t, res.MoveResult.Hit, "sample %d: the deterministic win must land", i)
		require.Equal(t, hidden, res.MoveResult.Crit,
			"sample %d: a stealth shot must crit and an ordinary one must not", i)
		total += res.MoveResult.Damage
	}
	return float64(total) / float64(n)
}

// ---------------------------------------------------------------------------
// 1. Nothing is targeting the shooter -> the multiplier applies
// ---------------------------------------------------------------------------

// Would this pass with the feature absent? NO. With no consumer for the knob
// both means are the same shot and the ratio is 1.0, not 2.75.
func TestFireUnengaged_ClearRoomCarriesTheMultiplier(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	cleanup := seedUnengagedFire(t, 1)
	defer cleanup()

	char := newUnengagedShooter(false)

	setUnengagedKnob(t, 1.0)
	baseline, _ := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)
	require.Greater(t, baseline, 0.0, "fixture sanity: the shot must do real damage")

	setUnengagedKnob(t, unengagedKnob)
	boosted, last := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)

	assert.InEpsilon(t, unengagedKnob, boosted/baseline, unengagedTolerance,
		"an unengaged shot's sampled mean (%.1f) over the same shot at 1.0x (%.1f) "+
			"must sit at RangedUnengagedDamageMultiplier", boosted, baseline)
	assert.False(t, last.AimedWhileEngaged,
		"nothing is targeting the shooter, so the shot was not taken while engaged")
}

// ---------------------------------------------------------------------------
// 2. Something turns on the shooter mid-fight -> the multiplier drops
// ---------------------------------------------------------------------------

// The whole point of the rule: you cannot aim while someone is hitting you.
// SAME shooter, SAME fight, no re-equip -- the watcher is in the room and in
// combat throughout, and the only thing that changes between the two samples
// is who it is aggroed on.
//
// Would this pass with the feature absent? NO -- and it is also the mutation
// guard: make shooterIsUnengaged return true unconditionally and both means
// collapse onto each other.
func TestFireUnengaged_BonusDropsWhenSomethingTargetsTheShooter(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	cleanup := seedUnengagedFire(t, 1,
		unengagedWatcher{unengagedWatcherHere, 1, aggroElsewhere(), 0})
	defer cleanup()
	setUnengagedKnob(t, unengagedKnob)

	char := newUnengagedShooter(false)

	clear, clearRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)
	require.Greater(t, clear, 0.0, "fixture sanity: the shot must do real damage")
	assert.False(t, clearRes.AimedWhileEngaged,
		"the watcher is fighting someone else, so the shooter is unengaged")

	// Same fight. The watcher turns on the shooter.
	setWatcherAggro(t, unengagedWatcherHere, aggroOnShooter())

	engaged, engagedRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)

	assert.InEpsilon(t, unengagedKnob, clear/engaged, unengagedTolerance,
		"the unengaged mean (%.1f) must sit a full RangedUnengagedDamageMultiplier "+
			"above the engaged mean (%.1f)", clear, engaged)
	assert.True(t, engagedRes.AimedWhileEngaged,
		"the shot was taken while something in the room was on the shooter")
}

// ---------------------------------------------------------------------------
// 3. Cross-room: the SHOOTER's room decides, in both directions
// ---------------------------------------------------------------------------

// Three phases, because a cross-room test written with no attackers present
// passes under EITHER reading of which room is scanned -- and passes with the
// feature absent entirely.
//
//	A: both rooms clear                        -> bonus
//	B: attacker in the SHOOTER's room          -> NO bonus
//	C: attacker in the TARGET's room only      -> bonus KEPT
//
// B kills "scan targetRoom" (that mutant sees a clear room 2 and pays the
// bonus); C kills it from the other side (that mutant sees the attacker in
// room 2 and withholds it). A sniper who is himself in melee is engaged: that
// is the rule, not an edge case.
func TestFireUnengaged_CrossRoomReadsTheShootersRoom(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	cleanup := seedUnengagedFire(t, 2,
		unengagedWatcher{unengagedWatcherHere, 1, aggroElsewhere(), 0},
		unengagedWatcher{unengagedWatcherThere, 2, aggroElsewhere(), 0})
	defer cleanup()
	setUnengagedKnob(t, unengagedKnob)

	char := newUnengagedShooter(false)

	// A: nothing is on the shooter anywhere.
	clear, clearRes := sampleShotsFromOneShooter(t, char, "skeleton north", unengagedSamples)
	require.True(t, clearRes.CrossRoom, "fixture sanity: this must be the cross-room path")
	require.Greater(t, clear, 0.0, "fixture sanity: the shot must do real damage")
	assert.False(t, clearRes.AimedWhileEngaged)

	// B: the sniper is himself in melee.
	setWatcherAggro(t, unengagedWatcherHere, aggroOnShooter())
	engaged, engagedRes := sampleShotsFromOneShooter(t, char, "skeleton north", unengagedSamples)
	assert.InEpsilon(t, unengagedKnob, clear/engaged, unengagedTolerance,
		"a sniper being attacked in his OWN room (%.1f) must lose the bonus the "+
			"clear-room sniper keeps (%.1f)", engaged, clear)
	assert.True(t, engagedRes.AimedWhileEngaged)

	// C: the shooter's room is clear again; the attacker is in the TARGET's
	// room instead. That is somebody else's problem, not the sniper's.
	setWatcherAggro(t, unengagedWatcherHere, aggroElsewhere())
	setWatcherAggro(t, unengagedWatcherThere, aggroOnShooter())
	remote, remoteRes := sampleShotsFromOneShooter(t, char, "skeleton north", unengagedSamples)
	assert.InEpsilon(t, clear, remote, unengagedTolerance,
		"an attacker in the TARGET's room (%.1f) must not cost the sniper the "+
			"bonus (%.1f): the scan reads the shooter's room", remote, clear)
	assert.False(t, remoteRes.AimedWhileEngaged)
}

// ---------------------------------------------------------------------------
// 4. It COMPOUNDS with the surprise opening shot -- a product, not a choice
// ---------------------------------------------------------------------------

// The opener is unengaged by definition (a hidden shooter is not being hit),
// so the two always meet. The product is pinned explicitly so that a later
// "simplification" into alternatives -- max(surprise, unengaged) -- is caught
// rather than passing as "still bigger than the plain shot".
//
// Would this pass with the feature absent? NO: without the unengaged factor
// the ratio is the surprise stack alone.
func TestFireUnengaged_CompoundsWithTheSurpriseOpener(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	cleanup := seedUnengagedFire(t, 1)
	defer cleanup()

	setUnengagedKnob(t, 1.0)
	plain := sampleFreshShooterDamage(t, false, unengagedSamples)
	require.Greater(t, plain, 0.0, "fixture sanity: the control shot must do real damage")

	setUnengagedKnob(t, unengagedKnob)
	ambush := sampleFreshShooterDamage(t, true, unengagedSamples)

	cfg := configs.GetBalanceConfig()
	probe := newUnengagedShooter(true)
	surpriseStack := combat.CritDamageMultiplier(surpriseRangedRank) *
		combat.OpeningStrikeMultiplier(probe, float64(cfg.SurpriseRangedStrikeMultiplier))
	wantProduct := surpriseStack * unengagedKnob
	wantEitherOr := math.Max(surpriseStack, unengagedKnob)

	require.Greater(t, wantProduct, wantEitherOr*1.3,
		"fixture sanity: product (%.2f) and either/or (%.2f) must be far enough "+
			"apart that the tolerance cannot swallow the difference",
		wantProduct, wantEitherOr)

	got := ambush / plain
	assert.InEpsilon(t, wantProduct, got, unengagedTolerance,
		"the stealth shot (%.1f) over the plain 1.0x shot (%.1f) must be the "+
			"PRODUCT of the surprise stack and RangedUnengagedDamageMultiplier",
		ambush, plain)
}

// ---------------------------------------------------------------------------
// 5. CONTROL: melee is untouched
// ---------------------------------------------------------------------------

// Would this pass with the feature absent? YES -- it is a control, and it is
// here to catch the knob being wired somewhere shared (a global damage hook, a
// SituationalAttackMult, ExecuteSkillMove itself) instead of the ranged shot
// alone. Kick is the melee sample: same channel seam, same contest stub.
func TestFireUnengaged_DoesNotTouchMelee(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)

	cfg := configs.GetConfig()
	cfg.Balance.KickDamagePercent = 0.80
	// Zero knockdown: a knocked-down target would flip the next kick to a
	// stomp (different damage percent and mitigation) and blur the means.
	cfg.Balance.KickKnockdownFactor = 0
	configs.SetConfigForTest(t, cfg)

	cleanupSpecies := species.SeedSpeciesForTest(map[int]*species.Species{
		8102: {SpeciesId: 8102, Name: "target", BodyParts: []string{"arms", "hands", "legs"}},
		8201: {SpeciesId: 8201, Name: "kicker", BodyParts: []string{"arms", "hands", "legs"}},
	})
	defer cleanupSpecies()

	actor, char, target := newSpecialMoveAdmissionActor(t, 8201, 100, 10, false)

	sample := func(n int) float64 {
		total := 0
		for i := 0; i < n; i++ {
			char.Cooldowns = characters.Cooldowns{}
			char.Stamina = char.StaminaMax.Value
			target.Health = target.HealthMax.Value
			res := ExecuteKick(actor)
			require.True(t, res.Executed, "sample %d: the kick must execute", i)
			require.True(t, res.MoveResult.Hit, "sample %d: the deterministic win must land", i)
			total += res.MoveResult.Damage
		}
		return float64(total) / float64(n)
	}

	setUnengagedKnob(t, 1.0)
	atOne := sample(unengagedSamples)
	require.Greater(t, atOne, 0.0, "fixture sanity: the kick must do real damage")

	setUnengagedKnob(t, unengagedKnob)
	atKnob := sample(unengagedSamples)

	assert.InEpsilon(t, atOne, atKnob, unengagedTolerance,
		"melee damage (%.1f at 1.0x, %.1f at %.2fx) must not move with a RANGED knob",
		atOne, atKnob, unengagedKnob)
}

// ---------------------------------------------------------------------------
// 6. CONTROL: the knob at 1.0 is a true no-op on every path
// ---------------------------------------------------------------------------

// Would this pass with the feature absent? YES -- it is a control. It exists
// because the knob's Go default IS 1.0, so any deployment that omits it from
// config.yaml must behave exactly as it did before this commit: engaged and
// unengaged identical, same-room and cross-room alike. The FLAG assertion is
// the part that does bite: the flag reports the situation, not the knob.
func TestFireUnengaged_KnobAtOneIsANoOp(t *testing.T) {
	for _, tc := range []struct {
		name       string
		defenderRm int
		watcherRm  int
		rest       string
	}{
		{"same room", 1, 1, "skeleton"},
		{"cross room", 2, 1, "skeleton north"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pinRangedSurpriseBalance(t)
			pinOrdinaryContestWin(t)
			cleanup := seedUnengagedFire(t, tc.defenderRm,
				unengagedWatcher{unengagedWatcherHere, tc.watcherRm, aggroElsewhere(), 0})
			defer cleanup()
			setUnengagedKnob(t, 1.0)

			char := newUnengagedShooter(false)

			clear, _ := sampleShotsFromOneShooter(t, char, tc.rest, unengagedSamples)
			require.Greater(t, clear, 0.0, "fixture sanity: the shot must do real damage")

			setWatcherAggro(t, unengagedWatcherHere, aggroOnShooter())
			engaged, engagedRes := sampleShotsFromOneShooter(t, char, tc.rest, unengagedSamples)

			assert.InEpsilon(t, clear, engaged, unengagedTolerance,
				"at 1.0x the engaged shot (%.1f) must be indistinguishable from the "+
					"unengaged one (%.1f)", engaged, clear)
			assert.True(t, engagedRes.AimedWhileEngaged,
				"the flag reports the SITUATION, not the knob: it must still be set "+
					"when the multiplier happens to be 1.0")
		})
	}
}

// ---------------------------------------------------------------------------
// 7. Your OWN companion aggroed on you does not count as an attacker
// ---------------------------------------------------------------------------

// Reachable, not hypothetical. Three live paths leave a mob CHARMED BY YOU,
// still in your Companions list, in your room, and aggroed on you:
//
//   - steal.go answers a failed roll with `attack @<owner>`, and deliberately
//     opts out of mobs.CheckPlayerHarm, so stealing from your own pet is
//     allowed;
//   - plant.go does the same and carries no guards at all;
//   - the behaviour-tree mob_idle attack fallback picks a random player in
//     the room with no owner exclusion, and charmed mobs do receive mob_idle.
//
// Without the charm skip, your pet nipping at you would silently cost you the
// whole ranged bonus.
//
// The control below is the half that keeps the skip honest: BETRAYAL must
// still count. Charm lapse and dismiss both RemoveCharm BEFORE they SetAggro,
// so an ex-companion turning on you arrives here uncharmed and reads as the
// genuine attacker it is. A skip written on Companions membership, or on
// IsCharmed() with no argument, would wrongly swallow that too.
//
// Would this pass with the feature absent? NO for the control half.
func TestFireUnengaged_OwnCompanionIsNotAnAttacker(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	cleanup := seedUnengagedFire(t, 1,
		unengagedWatcher{unengagedWatcherHere, 1, aggroOnShooter(), unengagedShooterUserId})
	defer cleanup()
	setUnengagedKnob(t, unengagedKnob)

	char := newUnengagedShooter(false)

	pet := mobs.GetInstance(unengagedWatcherHere)
	require.NotNil(t, pet)
	require.True(t, pet.Character.IsCharmed(unengagedShooterUserId),
		"precondition: the mob is the shooter's own companion")
	require.Equal(t, unengagedShooterUserId, pet.Character.Aggro.UserId,
		"precondition: and it is nonetheless aggroed on its owner")

	withPet, petRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)
	require.Greater(t, withPet, 0.0, "fixture sanity: the shot must do real damage")
	assert.False(t, petRes.AimedWhileEngaged,
		"your own companion snapping at you is not somebody hitting you")

	// The control: the SAME mob, same room, same aggro -- but the charm is
	// gone, exactly as charm lapse and dismiss leave it. Now it is a real
	// attacker and the bonus must go.
	pet.Character.Charmed = nil
	betrayed, betrayedRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)

	assert.InEpsilon(t, unengagedKnob, withPet/betrayed, unengagedTolerance,
		"a charmed companion (%.1f) must keep the bonus that an ex-companion "+
			"turning on you (%.1f) takes away", withPet, betrayed)
	assert.True(t, betrayedRes.AimedWhileEngaged,
		"betrayal is a real attack: the shooter is engaged")
}

// ---------------------------------------------------------------------------
// 8. The headline case: your own first shot ends your own bonus
// ---------------------------------------------------------------------------

// "Your first same-room shot makes the target engage you, so you lose the
// bonus until you break away" is the design claim this whole slice rests on,
// and cases 1-3 do NOT pin it: they move a BYSTANDER's aggro, while the
// mechanism that matters is the TARGET reciprocating.
//
// The reciprocation itself lives outside this package -- ExecuteFire's own
// doc comment lists "retaliation aggro on the target" as a caller
// responsibility, and it is applied in internal/hooks
// (NewRound_DoCombat_unified.go, PvM and MvP). So it is seeded by hand here
// rather than driven; what this test pins is that shooterIsUnengaged reads the
// resulting state correctly. The seeded shape is exactly what those two sites
// produce: the target's Aggro pointing back at the shooter, in the shooter's
// room, uncharmed.
//
// Would this pass with the feature absent? NO.
func TestFireUnengaged_TheTargetsReciprocalAggroEndsTheBonus(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	cleanup := seedUnengagedFire(t, 1)
	defer cleanup()
	setUnengagedKnob(t, unengagedKnob)

	char := newUnengagedShooter(false)
	target := mobs.GetInstance(500)
	require.NotNil(t, target)
	require.Nil(t, target.Character.Aggro,
		"precondition: the target has not noticed anyone yet")

	opener, openerRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)
	require.Greater(t, opener, 0.0, "fixture sanity: the shot must do real damage")
	assert.False(t, openerRes.AimedWhileEngaged,
		"the opening shot is taken with nothing on the shooter")

	// What internal/hooks does on the round after a same-room shot lands.
	target.Character.Aggro = &characters.Aggro{UserId: unengagedShooterUserId}

	followUp, followUpRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)

	assert.InEpsilon(t, unengagedKnob, opener/followUp, unengagedTolerance,
		"once the target is shooting back, the follow-up (%.1f) must drop a full "+
			"RangedUnengagedDamageMultiplier below the opener (%.1f)", followUp, opener)
	assert.True(t, followUpRes.AimedWhileEngaged,
		"the target is now on the shooter, so the shot was taken while engaged")
}

// ---------------------------------------------------------------------------
// 9. A mob shooter must not borrow a player's charm by numeric coincidence
// ---------------------------------------------------------------------------

// CharmInfo.UserId is ALWAYS a player id: every production Charm() caller
// passes user.UserId, and the one exception (behaviortree/actions_mob.go)
// passes literal 0. So a charm skip keyed on a `charmerKey` that falls back to
// MobInstanceId compares two different id spaces -- and they collide for real,
// because instanceCounter hands out ids from 1 upward and prod user ids are
// also small (Meirok is 3).
//
// The fixture IS the collision: a hostile archer with InstanceId 3, attacked
// by the companion of PLAYER 3. Under the fallback the archer reads that
// companion as "charmed by me" and silently keeps the full multiplier. Keyed
// on uid alone, a mob shooter (uid == 0) can never take the skip, and the
// archer is engaged like anything else.
//
// Would this pass with the feature absent? NO.
func TestFireUnengaged_MobShooterDoesNotBorrowAPlayersCharm(t *testing.T) {
	pinRangedSurpriseBalance(t)
	pinOrdinaryContestWin(t)
	cleanup := seedUnengagedFire(t, 1,
		unengagedWatcher{
			instanceId: unengagedWatcherHere,
			roomId:     1,
			// Attacking the ARCHER, and charmed by the PLAYER whose user id
			// happens to equal the archer's instance id.
			aggro:     state.ActorRef{MobInstanceId: unengagedMobShooterInstanceId},
			charmedBy: unengagedMobShooterInstanceId,
		})
	defer cleanup()
	setUnengagedKnob(t, unengagedKnob)

	char := newUnengagedMobShooter()
	require.Zero(t, char.GetUserId(), "precondition: a MOB shooter has no user id")
	require.Equal(t, unengagedMobShooterInstanceId, char.MobInstanceId)

	pet := mobs.GetInstance(unengagedWatcherHere)
	require.NotNil(t, pet)
	require.True(t, pet.Character.IsCharmed(unengagedMobShooterInstanceId),
		"fixture sanity: the id spaces really do collide, so a MobInstanceId "+
			"fallback would match this charm")

	engaged, engagedRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)
	require.Greater(t, engaged, 0.0, "fixture sanity: the shot must do real damage")
	assert.True(t, engagedRes.AimedWhileEngaged,
		"somebody else's companion is a real attacker: the archer is engaged")

	// The control that turns the flag assertion into a damage measurement:
	// same mob, same room, now fighting someone else entirely.
	setWatcherAggro(t, unengagedWatcherHere, aggroElsewhere())
	clear, clearRes := sampleShotsFromOneShooter(t, char, "skeleton", unengagedSamples)
	assert.False(t, clearRes.AimedWhileEngaged)

	assert.InEpsilon(t, unengagedKnob, clear/engaged, unengagedTolerance,
		"the archer must LOSE the bonus (%.1f) it keeps when unattacked (%.1f); "+
			"a MobInstanceId charm fallback would hand it back", engaged, clear)
}
