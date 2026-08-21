package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// Parity calibration (spec section 4): observed = p + F(1-2p) under
// RunWithFloors' one-symmetric-flip semantics. These are the RATIFIED
// intended rates — deliberately not the old accidental live rates (the
// pre-U10 threshold knobs claimed 50/60/35 but delivered ~50/~91/~2.3).
func TestKnockdownFactor_ParityCalibration(t *testing.T) {
	floor := float64(configs.GetBalanceConfig().ContestFloor)
	const n = 200000
	cases := []struct {
		name   string
		factor float64
		want   float64
	}{
		{"bash", 1.0, 0.50},
		{"trip", 1.057, 0.60},
		{"kick", 0.924, 0.35},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wins := 0
			for i := 0; i < n; i++ {
				if RunContest(100*tc.factor, []contest.Entry{{Score: 100}}).Success {
					wins++
				}
			}
			got := float64(wins) / n
			want := tc.want + floor*(1-2*tc.want)
			if got < want-0.02 || got > want+0.02 {
				t.Errorf("%s: parity knockdown rate = %.3f, want %.3f +-0.02", tc.name, got, want)
			}
		})
	}
}

// TestExecuteSkillMove_KnockdownResistProgression drives ExecuteSkillMove
// N=200 times through a forced clean hit (cleanWinRunner, from
// skill_moves_grapple_test.go: a contested win with ZScore 0 on both rolls —
// no crit, no fumble) so every iteration lands and the new knockdown contest
// always runs, then checks the SHAPE of the progression it fires.
//
// CRITICAL: the defender's unarmed-combat GetSkillUseCount must rise by
// N + resists, NOT by resists alone. The channel seam's
// AwardDefenceProgression (defence_multiplier.go, resolveChannelAttackWithRunner)
// already fires the winning defence's skill once per CONTESTED swing
// win-or-lose — a bare, unequipped defender's only eligible defence is dodge,
// and DefenceSkillAndStat maps dodge to unarmed-combat — so every one of the
// N forced hits contributes one ordinary unarmed-combat event from the
// channel contest alone, entirely independent of the new knockdown roll.
// cleanWinRunner deliberately rolls no crit/fumble on either side, so that
// ordinary event is the ONLY thing awardChannelDefenceBonus could have added
// and it adds nothing here. The new U10 knockdown-resist event is ADDITIONAL
// on top of that baseline: it fires once per iteration where the hit landed,
// the defender was contest-eligible (KnockdownFactor > 0, not control-immune)
// and the knockdown contest came back a resist (result.KnockedDown == false).
func TestExecuteSkillMove_KnockdownResistProgression(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	pinContestFloorOff(t)

	atk := characters.New()
	def := characters.New()
	def.HealthMax.Value = 1_000_000
	def.StaminaMax.Value = 1_000_000

	const n = 200
	usesBefore := def.GetSkillUseCount("unarmed-combat")
	resists := 0

	for i := 0; i < n; i++ {
		// Refresh the pools and position every iteration. 200 landed hits in
		// a row would otherwise run the defender out of stamina (the seam
		// charges the winning defence on every contest) or leave it Prone
		// from a prior knockdown, either of which could make the knockdown
		// FSM transition fail and desync result.KnockedDown from the
		// contest's own verdict — which is exactly the signal this test
		// counts resists by.
		def.Health = def.HealthMax.Value
		def.Stamina = def.StaminaMax.Value
		setCombatPositionParallel(def, position.Standing)

		p := SkillMoveParams{
			Attacker: atk, Defender: def,
			Channel: ChannelMelee,
			Attack: AttackSide{
				Stat: 100, StatName: "strength",
				Skill: skills.WeaponCombat, SkillRank: 0,
				Mult: 1.0,
			},
			// Factor 1.0 is the bash-parity calibration: the attacker's
			// knockdown score (100) matches a bare Dex-100 defender's, so
			// the real, unmocked knockdown contest below splits roughly
			// 50/50 across the loop rather than degenerating to all-hit or
			// all-resist — exercising both branches without requiring
			// either.
			DamagePercent: 1.0, KnockdownFactor: 1.0,
			DamageStat: 100, MitigationMultiplier: 1.0,
		}

		res := executeSkillMoveWithRunner(p, cleanWinRunner)
		if !res.Hit {
			t.Fatalf("iteration %d: the forced clean win did not land as a Hit", i)
		}
		if !res.KnockedDown {
			resists++
		}
	}

	got := def.GetSkillUseCount("unarmed-combat") - usesBefore
	want := n + resists
	if got != want {
		t.Errorf("unarmed-combat use count rose by %d, want %d (= %d ordinary defence events + %d knockdown-resist events)",
			got, want, n, resists)
	}
}
