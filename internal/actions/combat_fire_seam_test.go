package actions

// U6b Task 8 — ranged onto the seam. ExecuteFire routes through
// combat.ExecuteSkillMove with Channel: ChannelRanged + an AttackSide
// (Perception + ranged-combat rank), so the defender's answer is a defence SET
// from DefenceEntriesFor, not the deleted folded defence scalar:
//
//   - a SHIELDED defender (weapon + shield) enters a real BLOCK entry into the
//     contest — worth −41…−46% expected damage per the Task 8 modelling,
//     against −16…−27% from the old flat shield-bonus knob;
//   - a SHIELDLESS defender answers with dodge ALONE (no parry — you cannot
//     parry a bolt — and no phantom block);
//   - a shot can now CRIT, decided once inside the seam against CritBarFor's
//     skill-pair bar (there was no ranged crit tier before);
//   - the flat 15 is GONE: the shield's entire contribution is the block
//     CONTEST entry, never an addend on any score in the path.
//
// These drive the REAL ExecuteFire path (weapon gating, unload writeback, cost
// admission) against a deterministic contest core injected via
// combat.SetChannelAttackContestRunnerForTest, mirroring the Task 5 taunt
// collapse tests in this package.

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/require"
)

// pinFireSeamKnobs pins every knob that can move a ranged contest's scores or
// verdicts, so the closed-form arithmetic below stays true in a test binary:
// ContestFloor 0 (no ±1 sentinel outcomes), effectiveness multipliers 1.0
// (entry score == GetDefenseScoreFor exactly), crit floors 0 (no random
// promotion muddying the crit assertions), SkillWeight 5 (the shipped value,
// explicit for the attack-score arithmetic).
func pinFireSeamKnobs(t *testing.T) {
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

// fireSeamDefender seeds the standard fire-test defender (mob instance 500 in
// room 1) and, when shielded, wields a sword + a wearable shield so
// DefenceEntriesFor's equipment gate grants block. Returns the LIVE character
// so expected scores are computed through the same API the seam reads.
func fireSeamDefender(t *testing.T, dex int, shielded bool) (*characters.Character, func()) {
	t.Helper()
	instanceId, cleanup := seedFireMobInRoom(t, 1, dex)
	mob := mobs.GetInstance(instanceId)
	require.NotNil(t, mob, "seeded fire defender must be retrievable")
	if shielded {
		mob.Character.Equipment.Weapon = items.Item{
			ItemId: 7,
			Spec: &items.ItemSpec{
				ItemId: 7, Name: "sword", Type: items.Weapon,
				Subtype: items.Slashing, Hands: 1,
			},
		}
		mob.Character.Equipment.Offhand = items.Item{
			ItemId: 8,
			Spec: &items.ItemSpec{
				ItemId: 8, Name: "buckler", Type: items.Offhand,
				Subtype: items.Wearable, BlockRating: 10,
			},
		}
	}
	return &mob.Character, cleanup
}

// fireSeamContest fires one shot through an interceptor and returns the
// attack score and defence entries the seam sent to the ONE contest.
func fireSeamContest(t *testing.T, runner func(float64, []contest.Entry) contest.Result) (FireResult, *characters.Character, float64, []contest.Entry) {
	t.Helper()

	calls := 0
	var gotAtkScore float64
	var gotEntries []contest.Entry
	restore := combat.SetChannelAttackContestRunnerForTest(func(atkScore float64, entries []contest.Entry) contest.Result {
		calls++
		gotAtkScore = atkScore
		gotEntries = append([]contest.Entry(nil), entries...)
		return runner(atkScore, entries)
	})
	defer restore()

	char := fireAttacker()
	char.Equipment.Weapon = fireRangedWeapon(1, 1.0, true)
	actor := newStubActor(char, rooms.LoadRoom(1))

	res := ExecuteFire(actor, "skeleton")
	require.True(t, res.Executed, "the shot must execute (cost %+v)", res.Cost)
	require.Equal(t, 1, calls,
		"ExecuteFire must run exactly ONE contest through the channel seam")
	return res, char, gotAtkScore, gotEntries
}

// TestFireSeam_ShieldedDefenderGetsBlockEntry: a shielded defender answers a
// shot with dodge AND a real block CONTEST entry, each scored by
// GetDefenseScoreFor exactly — no flat shield addend anywhere in the path.
func TestFireSeam_ShieldedDefenderGetsBlockEntry(t *testing.T) {
	pinFireSeamKnobs(t)

	defChar, cleanup := fireSeamDefender(t, 50, true)
	defer cleanup()

	_, atkChar, atkScore, entries := fireSeamContest(t, tauntDeterministicRunner(t, 0.5, 0.5, -0.5))

	// The attack side is Perception + ranged rank × SkillWeight (rank 1 —
	// characters.New seeds every skill at 1), NOT a score with the defender
	// folded into it.
	cfg := configs.GetBalanceConfig()
	wantAtk := float64(atkChar.GetEffectivePerception()) +
		float64(atkChar.GetSkillLevel(skills.RangedCombat))*float64(cfg.SkillWeight)
	require.InDelta(t, wantAtk, atkScore, 1e-9,
		"ranged attack score must be Perception + ranged rank x SkillWeight, with no defender term")

	require.Len(t, entries, 2, "shielded defender vs a shot: dodge + block, no parry")
	require.Equal(t, characters.DefenseDodge, entries[0].Name)
	require.Equal(t, characters.DefenseBlock, entries[1].Name)

	// Scores come from the canonical per-defence formulas, exactly. The block
	// entry equalling GetDefenseScoreFor(block) IS the no-flat-15 proof for
	// this path: any surviving addend would break the equality.
	wantDodge := defChar.GetDefenseScoreFor(characters.DefenseDodge, true)
	wantBlock := defChar.GetDefenseScoreFor(characters.DefenseBlock, true)
	require.InDelta(t, wantDodge, entries[0].Score, 1e-9)
	require.InDelta(t, wantBlock, entries[1].Score, 1e-9,
		"block must be scored by GetDefenseScoreFor — (Str+Dex)/2 + skill + BlockRating — not by an addend")

	// And no entry carries the deleted scalar's shape: Dex + combat skill +
	// the old flat 15 (= 65 with this fixture).
	oldScalar := float64(defChar.GetEffectiveDexterity()) + float64(defChar.GetCombatSkillLevel()) + 15
	for _, e := range entries {
		require.NotEqual(t, oldScalar, e.Score,
			"no defence entry may reproduce the deleted flat-bonus defence scalar")
	}
}

// TestFireSeam_ShieldlessDefenderDodgeOnly: without a shield the defence set
// against a shot is dodge alone — no parry (can't parry a bolt), no block —
// and the dodge score is identical to the shielded case's dodge entry: the
// shield changes the SET, never a score.
func TestFireSeam_ShieldlessDefenderDodgeOnly(t *testing.T) {
	pinFireSeamKnobs(t)

	defChar, cleanup := fireSeamDefender(t, 50, false)
	defer cleanup()

	_, _, _, entries := fireSeamContest(t, tauntDeterministicRunner(t, 0.5, 0.5, -0.5))

	require.Len(t, entries, 1, "shieldless defender vs a shot: dodge alone")
	require.Equal(t, characters.DefenseDodge, entries[0].Name)
	require.InDelta(t, defChar.GetDefenseScoreFor(characters.DefenseDodge, true),
		entries[0].Score, 1e-9,
		"dodge must be scored identically shielded or not — the shield adds an ENTRY, not an addend")
}

// TestFireSeam_ShotCanCrit: the ranged channel now has a crit tier. A contest
// won by 4σ normalized margin clears CritBarFor's rank-parity bar (2.0), so
// the seam stamps AttackerCrit and ExecuteSkillMove surfaces it as Crit —
// something the pre-seam fire path could never produce.
func TestFireSeam_ShotCanCrit(t *testing.T) {
	pinFireSeamKnobs(t)

	_, cleanup := fireSeamDefender(t, 50, false)
	defer cleanup()

	res, _, _, _ := fireSeamContest(t, tauntDeterministicRunner(t, 4.0, 1.5, -1.5))

	require.True(t, res.MoveResult.Hit, "a 4-sigma attack win is a hit")
	require.True(t, res.MoveResult.Crit,
		"a shot must be able to CRIT: 4-sigma margin clears the rank-parity pair bar")
	require.False(t, res.MoveResult.Fumble)
	require.Greater(t, res.MoveResult.Damage, 0, "a critting shot deals damage")
}

// TestFireSeam_DefendedShotUsesContestNotAddend: when the defence wins the
// contest, the outcome carries the winning defence's identity and the shared
// mitigation curve's multiplier — the defended path is the same seam every
// other channel uses, not a scalar comparison.
func TestFireSeam_DefendedShotUsesContestNotAddend(t *testing.T) {
	pinFireSeamKnobs(t)

	_, cleanup := fireSeamDefender(t, 50, true)
	defer cleanup()

	// Defence wins by 1σ (attack margin -1.0), cleanly (no defensive crit).
	res, _, _, _ := fireSeamContest(t, tauntDeterministicRunner(t, -1.0, -0.5, 0.5))

	require.False(t, res.MoveResult.Hit)
	require.True(t, res.MoveResult.Defence.Defended)
	require.Equal(t, characters.DefenseDodge, res.MoveResult.Defence.DefenceType,
		"the defended outcome names the WINNING defence from the contest")
	require.Greater(t, res.MoveResult.Defence.DamageMultiplier, 0.0)
	require.Less(t, res.MoveResult.Defence.DamageMultiplier, 1.0,
		"a rolled defensive win lands on the shared DefenceMitigation curve, not a binary miss")
}
