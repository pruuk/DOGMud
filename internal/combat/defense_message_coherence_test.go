package combat

import (
	"fmt"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/items"
)

// TestSendDefenseMessagesMeleePathCoordinatesAllThreeRoles is the melee-path
// regression test for the coherence bug: authored defense-message pools pair
// up BY INDEX (variant N of todefender/toattacker/toroom describe the SAME
// event), but the melee path in sendDefenseMessages used to pick three
// INDEPENDENT random indices via three separate MessageOptions.Get() calls,
// so the defender, attacker and room text could each describe a different
// action.
//
// sendDefenseMessages has no picker override (it goes through
// items.GetDefenseMessage -> items.RenderTriad with production randomness),
// so this cannot be pinned to a single deterministic index the way the
// items-package RenderTriad test can. Instead it runs many trials and checks
// EVERY one is internally coordinated: before the fix, three independent
// picks out of a 10-variant pool land on the same index for all three roles
// only about 1 time in 100 (1/n^2), so 40 trials all matching by chance is
// effectively impossible; after the fix every trial is coordinated by
// construction, so the loop is not flaky in either direction.
func TestSendDefenseMessagesMeleePathCoordinatesAllThreeRoles(t *testing.T) {
	const n = 10
	toDefender := make(items.MessageOptions, n)
	toAttacker := make(items.MessageOptions, n)
	toRoom := make(items.MessageOptions, n)
	for i := 0; i < n; i++ {
		toDefender[i] = items.ItemMessage(fmt.Sprintf("MELEE-DEF-%d", i))
		toAttacker[i] = items.ItemMessage(fmt.Sprintf("MELEE-ATK-%d", i))
		toRoom[i] = items.ItemMessage(fmt.Sprintf("MELEE-ROOM-%d", i))
	}

	group := &items.DefenseMessageGroup{
		OptionId: items.DefenseBlock,
		Options: items.DefenseIntensity{
			items.Weak: items.DefenseOptions{Together: items.DefenseTogetherMessages{
				ToDefender: toDefender, ToAttacker: toAttacker, ToRoom: toRoom,
			}},
			// Normal/Heavy are unused by this test (zScore pins Weak) but
			// GetDefenseMessage only looks up the band it needs, so leaving
			// them absent is fine here.
		},
	}
	restore := items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{
		items.DefenseBlock: group,
	})
	defer restore()

	sourceChar := characters.New()
	sourceChar.Name = "Attacker"
	targetChar := characters.New()
	targetChar.Name = "Defender"

	best := bestDefenseResult{
		defenseType: characters.DefenseBlock,
		defRoll:     dice.RollResult{ZScore: 0.0}, // < 0.5 => Weak band
	}

	const trials = 40
	for i := 0; i < trials; i++ {
		result := &AttackResult{}
		// partial=false so the defender/attacker personal lines are sent
		// alongside the room line (mirrors the defenseCrit call site).
		sendDefenseMessages(result, best, sourceChar, targetChar, false, false)

		if len(result.MessagesToTarget) != 1 || len(result.MessagesToSource) != 1 || len(result.MessagesToSourceRoom) != 1 {
			t.Fatalf("trial %d: expected exactly one message per channel, got target=%d source=%d sourceRoom=%d",
				i, len(result.MessagesToTarget), len(result.MessagesToSource), len(result.MessagesToSourceRoom))
		}

		defText := result.MessagesToTarget[0].Text
		atkText := result.MessagesToSource[0].Text
		roomText := result.MessagesToSourceRoom[0].Text

		var defIdx, atkIdx, roomIdx int
		if _, err := fmt.Sscanf(defText, "MELEE-DEF-%d", &defIdx); err != nil {
			t.Fatalf("trial %d: could not parse defender index from %q: %v", i, defText, err)
		}
		if _, err := fmt.Sscanf(atkText, "MELEE-ATK-%d", &atkIdx); err != nil {
			t.Fatalf("trial %d: could not parse attacker index from %q: %v", i, atkText, err)
		}
		if _, err := fmt.Sscanf(roomText, "MELEE-ROOM-%d", &roomIdx); err != nil {
			t.Fatalf("trial %d: could not parse room index from %q: %v", i, roomText, err)
		}

		if defIdx != atkIdx || defIdx != roomIdx {
			t.Fatalf("trial %d: melee defense narration is NOT coordinated: defender picked variant %d, attacker picked %d, room picked %d (defender=%q attacker=%q room=%q)",
				i, defIdx, atkIdx, roomIdx, defText, atkText, roomText)
		}
	}
}
