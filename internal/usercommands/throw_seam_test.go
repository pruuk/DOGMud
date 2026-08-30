package usercommands

// U6b Task 15 — throw resolves per-target through the channel seam.
//
// Each hostile in the room contests the ONE grenade independently through
// combat.ResolveChannelAttack(ChannelRanged, ...): its own equipment-gated
// defence set (dodge for everyone, block only behind a shield), its own
// margin, its own crit-or-not. Damage gains the shared defence multiplier
// curve and the crit tier; the old resolution was a hand-rolled RunContest
// against Dex + Perception x SkillWeight x 0.5 (a stat as pseudo-skill) with
// BINARY full damage — tests here would fail against that shape.
//
// These drive the REAL Throw path (item selection, cost admission, cooldown,
// consumption, engagement) against a deterministic contest core injected via
// combat.SetChannelAttackContestRunnerForTest, mirroring the Task 8 fire seam
// tests in internal/actions.

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const throwSeamItemID = 79161

// pinThrowSeamKnobs pins every knob that can move the seam's scores or
// verdicts: ContestFloor 0, SkillWeight 5 (shipped value, explicit for the
// attack-score arithmetic), effectiveness multipliers 1.0, crit floors 0 so
// no random promotion muddies the deterministic-runner assertions.
func pinThrowSeamKnobs(t *testing.T) {
	t.Helper()
	c := configs.GetConfig()
	c.Balance.ContestFloor = 0
	c.Balance.SkillWeight = 5.0
	c.Balance.DodgeEffectiveness = 1.0
	c.Balance.BlockEffectiveness = 1.0
	c.Balance.MinAttackCritChance = 0
	c.Balance.MinDefenseCritChance = 0
	configs.SetConfigForTest(t, c)
}

// seedThrowSeamFixture seeds a thrower (Dex 100, given skullduggery rank,
// funded) holding one 5-use bomb, and mobCount fresh targets (Dex 1, Per 1,
// 500 HP) in room 1, each optionally wearing mitigation% of physical armour.
func seedThrowSeamFixture(t *testing.T, mobCount int, mitigation int, rank int) (*users.UserRecord, *rooms.Room, []*mobs.Mob) {
	t.Helper()
	pinThrowSeamKnobs(t)

	cleanupItems := items.SeedItemsForTest(map[int]*items.ItemSpec{
		throwSeamItemID: {
			ItemId:           throwSeamItemID,
			Name:             "seam bomb",
			NameSimple:       "bomb",
			Subtype:          items.Throwable,
			Uses:             5,
			DamageMultiplier: 1,
		},
	})
	t.Cleanup(cleanupItems)

	cleanupBiomes := rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"test": {BiomeId: "test", Name: "Test", LitArea: true},
	})
	t.Cleanup(cleanupBiomes)
	room := &rooms.Room{RoomId: 1, Zone: "test", Biome: "test"}
	cleanupRooms := rooms.SeedRoomsForTest(map[int]*rooms.Room{1: room}, nil)
	t.Cleanup(cleanupRooms)

	targets := make([]*mobs.Mob, 0, mobCount)
	instances := map[int]*mobs.Mob{}
	for i := 0; i < mobCount; i++ {
		targetChar := characters.New()
		targetChar.Name = "Target"
		targetChar.RoomId = 1
		targetChar.Health = 500
		targetChar.HealthMax.Value = 500
		targetChar.Stats.Dexterity.ValueAdj = 1
		targetChar.Stats.Perception.ValueAdj = 1
		if mitigation > 0 {
			targetChar.Equipment.Body = items.Item{
				ItemId: 79170,
				Spec: &items.ItemSpec{
					ItemId: 79170, Name: "test plate", Type: items.Body,
					PhysicalMitigation: mitigation,
				},
			}
		}
		m := &mobs.Mob{MobId: 1, InstanceId: 79162 + i, HomeRoomId: 1, Character: *targetChar}
		instances[m.InstanceId] = m
		targets = append(targets, m)
	}
	cleanupMobs := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{1: {MobId: 1, Character: characters.Character{Name: "Target"}}},
		instances,
	)
	t.Cleanup(cleanupMobs)
	for _, m := range targets {
		room.AddMob(m.InstanceId)
	}

	user := users.NewTestUser(79063, "thrower", "Thrower", 0)
	user.Character.RoomId = room.RoomId
	user.Character.Stats.Dexterity.ValueAdj = 100
	user.Character.Stamina = 100
	user.Character.Health = 200
	user.Character.HealthMax.Value = 200
	if rank > 0 {
		user.Character.Skills = map[string]int{string(skills.Skullduggery): rank}
	}
	cleanupUsers := users.SeedUsersForTest(map[int]*users.UserRecord{user.UserId: user})
	t.Cleanup(cleanupUsers)

	bomb := items.New(throwSeamItemID)
	bomb.Uses = 5
	user.Character.Items = []items.Item{bomb}

	return user, room, targets
}

// throwSeamResult fabricates one contest outcome with the given normalized
// margin (defence-positive when negative) and attacker z-score — the same
// shape as internal/actions' tauntDeterministicRunner.
func throwSeamResult(atkScore float64, entries []contest.Entry, normMargin, atkZ float64) contest.Result {
	stdDev := dice.StdDevFor(atkScore)
	if stdDev <= 0 {
		stdDev = 15
	}
	margin := normMargin * stdDev * math.Sqrt2
	return contest.Result{
		Contested: true,
		Success:   margin > 0,
		Winner:    entries[0].Name,
		Margin:    margin,
		AttackRoll: dice.RollResult{
			Value: atkScore + atkZ*stdDev, Mean: atkScore,
			StdDev: stdDev, ZScore: atkZ,
		},
		DefenseRoll: dice.RollResult{
			Value: atkScore + atkZ*stdDev - margin, Mean: entries[0].Score,
			StdDev: stdDev, ZScore: 0.5,
		},
	}
}

// TestThrowSeam_PerTargetContests_OneGrenadeManyContests: N targets means N
// independent contests for the ONE thrown grenade — every target rolled, every
// target damaged, everything hit engaged, one use consumed.
func TestThrowSeam_PerTargetContests_OneGrenadeManyContests(t *testing.T) {
	user, room, targets := seedThrowSeamFixture(t, 3, 0, 0)

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		calls++
		require.NotEmpty(t, entries, "the seam must send a defence set to every contest")
		return throwSeamResult(atkScore, entries, 0.5, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 3, calls, "3 targets must mean 3 independent contests for the one grenade")
	require.Len(t, user.Character.Items, 1)
	assert.Equal(t, 4, user.Character.Items[0].Uses, "one grenade consumed once, not per target")

	for i, m := range targets {
		dmg := 500 - m.Character.Health
		// Dex 100, rank 0, itemMult 1.0 → raw mean 30 at the default
		// physical scale; a full (undefended, non-crit) hit rolls around it.
		assert.GreaterOrEqual(t, dmg, 10, "target %d must take a full hit", i)
		assert.LessOrEqual(t, dmg, 55, "target %d damage must stay in the non-crit band", i)
		assert.True(t, m.Character.IsInCombat(), "hit target %d must engage", i)
	}
	require.True(t, user.Character.IsInCombat(), "an out-of-combat thrower who connects must engage")
}

// TestThrowSeam_AttackSideAndDefenceSets: the attack side the seam receives is
// Dex + skullduggery x SkillWeight (the defender's Perception pseudo-skill is
// gone), and the defence set is equipment-gated per target — dodge alone for a
// bare target, dodge + block behind a shield.
func TestThrowSeam_AttackSideAndDefenceSets(t *testing.T) {
	user, room, targets := seedThrowSeamFixture(t, 2, 0, 8)

	// Shield the second target: weapon + wearable offhand passes the gate.
	targets[1].Character.Equipment.Weapon = items.Item{
		ItemId: 79171,
		Spec: &items.ItemSpec{
			ItemId: 79171, Name: "sword", Type: items.Weapon,
			Subtype: items.Slashing, Hands: 1,
		},
	}
	targets[1].Character.Equipment.Offhand = items.Item{
		ItemId: 79172,
		Spec: &items.ItemSpec{
			ItemId: 79172, Name: "buckler", Type: items.Offhand,
			Subtype: items.Wearable, BlockRating: 10,
		},
	}

	var atkScores []float64
	var entrySets [][]string
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		atkScores = append(atkScores, atkScore)
		names := []string{}
		for _, e := range entries {
			names = append(names, e.Name)
		}
		entrySets = append(entrySets, names)
		return throwSeamResult(atkScore, entries, 0.5, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	require.Len(t, atkScores, 2)
	for i, score := range atkScores {
		assert.InDelta(t, 100+8*5.0, score, 0.001,
			"contest %d: attack side must be Dex + rank x SkillWeight", i)
	}

	require.Len(t, entrySets, 2)
	bare, shielded := 0, 0
	for _, names := range entrySets {
		switch {
		case len(names) == 1 && names[0] == characters.DefenseDodge:
			bare++
		case len(names) == 2 && names[0] == characters.DefenseDodge && names[1] == characters.DefenseBlock:
			shielded++
		default:
			t.Fatalf("unexpected ranged defence set %v", names)
		}
	}
	assert.Equal(t, 1, bare, "the bare target answers with dodge alone")
	assert.Equal(t, 1, shielded, "the shielded target answers with dodge + a real block entry")
}

// TestThrowSeam_DefendedTargetTakesPartialSplashDamage: a defended (non-crit)
// throw is no longer a binary miss — the splash lands partial damage along the
// shared DefenceMitigation curve, and the clipped target still engages.
func TestThrowSeam_DefendedTargetTakesPartialSplashDamage(t *testing.T) {
	user, room, targets := seedThrowSeamFixture(t, 1, 0, 0)

	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		return throwSeamResult(atkScore, entries, -0.5, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	dmg := 500 - targets[0].Character.Health
	// Normalized defence margin 0.5 → multiplier 1-(0.5+0.5x0.25) = 0.375 of
	// the ~30-mean roll ≈ 11.
	assert.GreaterOrEqual(t, dmg, 1, "a bare defensive win must still be clipped by the splash")
	assert.LessOrEqual(t, dmg, 22, "a defended throw must not land full damage")
	assert.True(t, targets[0].Character.IsInCombat(), "a clipped target engages")
}

// TestThrowSeam_DefensiveCritFullyNegatesAndDoesNotEngage: a defensive crit
// walks away clean — zero damage, no engagement on either side.
func TestThrowSeam_DefensiveCritFullyNegatesAndDoesNotEngage(t *testing.T) {
	user, room, targets := seedThrowSeamFixture(t, 1, 0, 0)

	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		return throwSeamResult(atkScore, entries, -3.0, 0.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 500, targets[0].Character.Health, "a defensive crit fully negates the splash")
	assert.False(t, targets[0].Character.IsInCombat(), "an untouched target does not engage")
	assert.False(t, user.Character.IsInCombat(), "a throw that touched nothing starts no fight")
}

// TestThrowSeam_AttackerCritBypassesMitigationAndScales: throw gains the crit
// tier — a crit bypasses the target's 75% physical mitigation and scales by
// CritDamageMultiplier, where a normal hit is mitigated to a quarter.
func TestThrowSeam_AttackerCritBypassesMitigationAndScales(t *testing.T) {
	t.Run("normal hit is mitigated", func(t *testing.T) {
		user, room, targets := seedThrowSeamFixture(t, 1, 75, 0)
		restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
			return throwSeamResult(atkScore, entries, 0.5, 0.5)
		})
		defer restore()

		handled, err := Throw("bomb", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		dmg := 500 - targets[0].Character.Health
		assert.GreaterOrEqual(t, dmg, 1)
		assert.LessOrEqual(t, dmg, 15, "a non-crit hit must respect 75%% mitigation (mean ~7.5)")
	})

	t.Run("crit bypasses and scales", func(t *testing.T) {
		user, room, targets := seedThrowSeamFixture(t, 1, 75, 0)
		restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
			return throwSeamResult(atkScore, entries, 5.0, 2.5)
		})
		defer restore()

		handled, err := Throw("bomb", user, room, 0)
		require.NoError(t, err)
		require.True(t, handled)

		dmg := 500 - targets[0].Character.Health
		assert.GreaterOrEqual(t, dmg, 30,
			"a crit must bypass mitigation and scale by CritDamageMultiplier (mean ~60)")
	})
}

// TestThrowSeam_FumbleDetonatesInHandAndEndsTheLoop: the fumble verdict now
// comes from the contest's own attack roll (self-relative, resolved before
// success); the projectile detonates on the thrower, targets are untouched,
// nobody engages. The self-damage path is deliberately unrolled
// (round(rawDmg)), so with Dex 100 it is exactly 30.
func TestThrowSeam_FumbleDetonatesInHandAndEndsTheLoop(t *testing.T) {
	user, room, targets := seedThrowSeamFixture(t, 2, 0, 0)

	calls := 0
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		calls++
		return throwSeamResult(atkScore, entries, -0.5, -2.5)
	})
	defer restore()

	handled, err := Throw("bomb", user, room, 0)
	require.NoError(t, err)
	require.True(t, handled)

	assert.Equal(t, 1, calls, "a fumble must end the AoE loop")
	assert.Equal(t, 170, user.Character.Health, "the thrower eats the un-mitigated blast")
	for i, m := range targets {
		assert.Equal(t, 500, m.Character.Health, "target %d must be untouched by a fumble", i)
		assert.False(t, m.Character.IsInCombat(), "target %d must not engage on a fumble", i)
	}
	assert.False(t, user.Character.IsInCombat(), "a fumble engages nobody")
}
