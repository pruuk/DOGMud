package combat

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// The live contest must score each defence with defenceEntryScore, the same
// function the fight prediction calls (#301). A capturing runner sees the
// entries the loop built; each must equal defenceEntryScore's score.
func TestDefenceEntryScore_IsWhatTheLiveContestRolls(t *testing.T) {
	pinParityBalance(t)
	attacker := parityCombatant(t, "attacker")
	defender := parityCombatant(t, "defender")
	ctx := combatContext{sourceSight: messaging.SightFull, targetSight: messaging.SightFull}
	defSeq := DefenceEntriesFor(combatvocab.Melee(combatvocab.TargetSingle), defender, DefenceEntryOpts{})
	if len(defSeq) == 0 {
		t.Fatal("fixture defender has no melee defence; the test proves nothing")
	}

	var seen []contest.Entry
	capture := func(_ float64, entries []contest.Entry) contest.Result {
		seen = append([]contest.Entry(nil), entries...)
		return contest.Result{}
	}
	res := AttackResult{}
	runBestOfAllDefenseWithRunner(&res, attacker, defender, defSeq, 100, false, ctx, capture)

	if len(seen) != len(defSeq) {
		t.Fatalf("runner saw %d entries, want %d", len(seen), len(defSeq))
	}
	for i, d := range defSeq {
		want, _, _ := defenceEntryScore(defender, d, false, ctx)
		if seen[i].Score != want {
			t.Errorf("%s: live entry score %v, defenceEntryScore %v", d, seen[i].Score, want)
		}
	}
}
