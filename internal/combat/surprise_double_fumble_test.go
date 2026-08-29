package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// doubleFumbleBest builds the one bestDefenseResult shape that reaches
// resolveDefenseOutcomeInner's double-fumble branch: BOTH z-scores at or under
// the -2.0 fumble threshold, with a defence actually attempted (defenseFumble
// is gated on defenseType != "").
//
// Constructed by hand rather than rolled, because the branch it targets fires
// on roughly 0.05% of swings. That rarity is exactly why the defect below
// survived: it reached CI as an occasional unexplained failure of
// TestSurpriseRound_ExactlyOneSwingIsMarkedAsTheOpener rather than as a
// reproducible bug.
func doubleFumbleBest() bestDefenseResult {
	return bestDefenseResult{
		defenseType: "dodge",
		margin:      5,
		hitRoll:     dice.RollResult{Value: 40, Mean: 100, StdDev: 15, ZScore: -3.0},
		defRoll:     dice.RollResult{Value: 40, Mean: 100, StdDev: 15, ZScore: -3.0},
	}
}

func countCategory(msgs []TaggedMessage, cat messaging.Category) int {
	n := 0
	for _, m := range msgs {
		if m.Category == cat {
			n++
		}
	}
	return n
}

// U10d invariant: an ambush round marks EXACTLY ONE line as the opener, and
// that marking is what carries the swing through medium and light verbosity
// (CategorySurpriseAttack is in neither suppression allowlist).
//
// The double fumble was the one outcome that broke it. The opener flag is
// consumed by the swing that is THROWN -- correctly, a double fumble is a
// thrown swing -- but the swing loop skips buildAttackMessages when
// res.doubleFumble is set, because handleDoubleFumble has already sent that
// swing's lines. buildAttackMessages is the ONLY place that applies
// CategorySurpriseAttack, so the ambush was consumed and narrated as an
// ordinary pratfall: no marked line at all, and at light verbosity nothing to
// tell the player an ambush had happened.
func TestOpeningStrike_DoubleFumbleStillMarksTheOpener(t *testing.T) {
	src, tgt := defenceFixture(1000)
	result := &AttackResult{}

	res := resolveDefenseOutcomeCore(result, doubleFumbleBest(), src, tgt, 2.0, false, false, true)

	if !res.doubleFumble {
		t.Fatalf("precondition: expected the double-fumble branch, got %+v", res)
	}

	if got := countCategory(result.MessagesToSource, messaging.CategorySurpriseAttack); got != 1 {
		t.Errorf("attacker saw %d opener-marked lines, want exactly 1: %v",
			got, result.MessagesToSource)
	}
	if got := countCategory(result.MessagesToTarget, messaging.CategorySurpriseAttack); got != 1 {
		t.Errorf("defender saw %d opener-marked lines, want exactly 1: %v",
			got, result.MessagesToTarget)
	}
	if got := countCategory(result.MessagesToSourceRoom, messaging.CategorySurpriseAttack); got != 1 {
		t.Errorf("the room saw %d opener-marked lines, want exactly 1: %v",
			got, result.MessagesToSourceRoom)
	}

	// The prose itself is unchanged -- only the routing category moves. The
	// double-fumble comedy line already opens and closes with !!! markers, so
	// it names itself loudly enough without the SURPRISE ATTACK banner the
	// generic weapon pool needs.
	for _, m := range result.MessagesToSource {
		if strings.Contains(m.Text, "SURPRISE ATTACK") {
			t.Errorf("a double-fumbled opener should not also carry the banner: %q", m.Text)
		}
		if !strings.Contains(m.Text, "!!!") {
			t.Errorf("expected the double-fumble comedy prose, got %q", m.Text)
		}
	}
}

// The other half of the invariant, and the reason the fix cannot simply mark
// every double fumble: a double fumble on an ORDINARY round marks nothing.
func TestDoubleFumble_OnAnOrdinaryRoundMarksNothing(t *testing.T) {
	src, tgt := defenceFixture(1000)
	result := &AttackResult{}

	res := resolveDefenseOutcomeCore(result, doubleFumbleBest(), src, tgt, 2.0, false, false, false)

	if !res.doubleFumble {
		t.Fatalf("precondition: expected the double-fumble branch, got %+v", res)
	}

	if got := countCategory(result.MessagesToSource, messaging.CategorySurpriseAttack); got != 0 {
		t.Errorf("a non-ambush double fumble marked %d lines as an opener, want 0: %v",
			got, result.MessagesToSource)
	}
	if got := countCategory(result.MessagesToSource, messaging.CategoryHitMelee); got != 1 {
		t.Errorf("expected the ordinary hit-melee routing, got %v", result.MessagesToSource)
	}
}
