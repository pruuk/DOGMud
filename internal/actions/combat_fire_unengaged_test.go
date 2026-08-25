package actions

import (
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
// Seven cases below. Two of them (5 and 6) are controls that would pass with
// the feature absent; that is stated on each. The rest would not:
//
//	1. unengaged carries the multiplier                       -- fails at 1.0x
//	2. it drops mid-fight the moment something turns on you   -- kills "always true"
//	3. cross-room reads the SHOOTER's room, both directions   -- kills "scan targetRoom"
//	4. it COMPOUNDS with the surprise opener (product, not max)
//	5. melee is untouched                                     (control)
//	6. the knob at 1.0 is a true no-op                        (control)
//	7. your OWN companion aggroed on you is not "someone is hitting me"
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
)

// unengagedWatcher describes a bystander mob to place in the fixture.
// charmedByShooter makes it the shooter's own companion.
type unengagedWatcher struct {
	instanceId       int
	roomId           int
	aggro            *characters.Aggro
	charmedByShooter bool
}

// aggroOnShooter and aggroElsewhere are the two states a watcher can be in.
func aggroOnShooter() *characters.Aggro {
	return &characters.Aggro{UserId: unengagedShooterUserId}
}

func aggroElsewhere() *characters.Aggro {
	return &characters.Aggro{MobInstanceId: unengagedOtherVictim}
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
		if w.charmedByShooter {
			wc.Charmed = characters.NewCharm(unengagedShooterUserId, characters.CharmPermanent, "")
		}
		wc.Aggro = w.aggro
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

// setWatcherAggro re-points a seeded watcher without disturbing anything else.
func setWatcherAggro(t *testing.T, instanceId int, aggro *characters.Aggro) {
	t.Helper()
	w := mobs.GetInstance(instanceId)
	require.NotNil(t, w, "watcher %d must be seeded", instanceId)
	w.Character.Aggro = aggro
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
		unengagedWatcher{unengagedWatcherHere, 1, aggroElsewhere(), false})
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
		unengagedWatcher{unengagedWatcherHere, 1, aggroElsewhere(), false},
		unengagedWatcher{unengagedWatcherThere, 2, aggroElsewhere(), false})
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
				unengagedWatcher{unengagedWatcherHere, tc.watcherRm, aggroElsewhere(), false})
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
		unengagedWatcher{unengagedWatcherHere, 1, aggroOnShooter(), true})
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
