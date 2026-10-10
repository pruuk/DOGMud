package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// #481: a knockdown move that lands on a defender already off their feet
// runs no knockdown contest (and so no resist award), because the Position
// table only knocks a Standing body down. It used to run the contest, win it,
// fail the Prone->Prone transition and log "transition not allowed by table"
// at warn on every such hit. The move still deals its damage.
func TestExecuteSkillMove_DownedDefenderSkipsTheKnockdownContest(t *testing.T) {
	pinDefenceAdmissionConfig(t)

	orig := knockdownContestRunner
	t.Cleanup(func() { knockdownContestRunner = orig })
	calls := 0
	knockdownContestRunner = func(_ float64, _ []contest.Entry) contest.Result {
		calls++
		return contest.Result{Contested: true, Success: true}
	}

	for _, supine := range []bool{false, true} {
		atk := characters.New()
		def := characters.New()
		def.HealthMax.Value = 1_000_000
		def.Health = def.HealthMax.Value
		def.Position = position.NewMachine()
		var err error
		if supine {
			err = def.Position.TransitionToSupine(position.SupineData{MinRecoveryRounds: 2},
				state.TransitionReason{Trigger: position.TriggerKnockdownFaceBackward})
		} else {
			err = def.Position.TransitionToProne(position.ProneData{MinRecoveryRounds: 2},
				state.TransitionReason{Trigger: position.TriggerKnockdownFaceForward})
		}
		if err != nil {
			t.Fatalf("fixture: could not knock the defender down: %v", err)
		}
		before := def.Position.State()
		usesBefore := def.GetSkillUseCount("unarmed-combat")
		calls = 0

		res := executeSkillMoveWithRunner(SkillMoveParams{
			Attacker:             atk,
			Defender:             def,
			Shape:                combatvocab.Melee(combatvocab.TargetSingle),
			Attack:               side(148, 52),
			DamagePercent:        1.0,
			KnockdownFactor:      1.0,
			DamageStat:           100,
			MitigationMultiplier: 1.0,
		}, cleanWinRunner)

		if !res.Hit || res.Damage <= 0 {
			t.Fatalf("supine=%v: the move must still land and deal damage, got hit=%v damage=%d", supine, res.Hit, res.Damage)
		}
		if calls != 0 {
			t.Errorf("supine=%v: knockdown contest ran %d times against a downed defender, want 0", supine, calls)
		}
		if res.KnockedDown {
			t.Errorf("supine=%v: reported a knockdown of a defender already down", supine)
		}
		if got := def.Position.State(); got != before {
			t.Errorf("supine=%v: position changed from %v to %v", supine, before, got)
		}
		// The channel seam's ordinary per-contest defence award (one event)
		// is unchanged; the knockdown resist award on top of it must not fire.
		if got := def.GetSkillUseCount("unarmed-combat") - usesBefore; got > 1 {
			t.Errorf("supine=%v: unarmed-combat rose by %d, want at most the one channel award", supine, got)
		}
	}
}
