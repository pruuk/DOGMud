package combat

// U6b Task 11 render pins for the counter-taunt (defy carve-out) narration.
// The playtest could not observe a decisive defy exchange live (every
// eligible target died or hid), so the render contract is pinned here
// deterministically: BuildCounterTauntMessages must produce a non-empty
// defender/attacker/room triad from the counter-defy pool with all tokens
// substituted, and must never go silent when the pool is not loaded (the
// generic fallback stands in).

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/stretchr/testify/require"
)

func counterDefyMessageFixture() *items.DefenseMessageGroup {
	mk := func(band string) items.DefenseOptions {
		messages := func(audience string) items.MessageOptions {
			result := make(items.MessageOptions, 5)
			for i := range result {
				result[i] = items.ItemMessage("counterdefy-" + band + "-" + audience +
					" {defender} turns the jeer back on {attacker}")
			}
			return result
		}
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: messages("defender"),
			ToAttacker: messages("attacker"),
			ToRoom:     messages("room"),
		}}
	}
	return &items.DefenseMessageGroup{OptionId: items.DefenseCounterDefy, Options: items.DefenseIntensity{
		items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
	}}
}

// With the counter-defy pool loaded, every line renders from it: RETORT
// prefix, both names substituted, no leftover tokens, and the conviction
// damage description on the two personal lines only (room lines never carry
// damage).
func TestBuildCounterTauntMessagesRendersFromCounterDefyPool(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{
		items.DefenseCounterDefy: counterDefyMessageFixture(),
	})
	defer restore()

	tests := []struct {
		name     string
		crit     bool
		damage   int
		wantBand string
	}{
		{"turned_aside_is_weak", false, 0, "weak"},
		{"landed_is_normal", false, 40, "normal"},
		{"crit_is_heavy", true, 40, "heavy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			countererMsg, taunterMsg, roomMsg := BuildCounterTauntMessages(
				"Selka", "Rurik", tc.crit, tc.damage, 200)
			for audience, line := range map[string]string{
				"counterer": countererMsg, "taunter": taunterMsg, "room": roomMsg,
			} {
				require.NotEmpty(t, line, "%s line must never be empty", audience)
				require.Contains(t, line, "RETORT!", "%s line must carry the retort prefix", audience)
				require.Contains(t, line, "counterdefy-"+tc.wantBand,
					"%s line must render from the counter-defy pool's %s band", audience, tc.wantBand)
				require.NotContains(t, line, "{attacker}", "%s line leaves a token unsubstituted", audience)
				require.NotContains(t, line, "{defender}", "%s line leaves a token unsubstituted", audience)
				require.Contains(t, line, "Selka", "%s line must name the counterer", audience)
				require.Contains(t, line, "Rurik", "%s line must name the original taunter", audience)
			}
			if tc.damage > 0 {
				dmgDesc := GetConvictionDamageDescription(tc.damage, 200)
				require.Contains(t, countererMsg, dmgDesc)
				require.Contains(t, taunterMsg, dmgDesc)
				require.NotContains(t, roomMsg, dmgDesc, "room lines never carry damage")
			}
		})
	}
}

// Without the pool (unit-test environments, a broken data deploy), the
// generic fallback must stand in — the retort can never go silent.
func TestBuildCounterTauntMessagesFallbackNeverSilent(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(nil)
	defer restore()

	for _, damage := range []int{0, 25} {
		countererMsg, taunterMsg, roomMsg := BuildCounterTauntMessages(
			"Selka", "Rurik", false, damage, 200)
		for audience, line := range map[string]string{
			"counterer": countererMsg, "taunter": taunterMsg, "room": roomMsg,
		} {
			require.NotEmpty(t, line, "fallback %s line must never be empty (damage %d)", audience, damage)
			require.Contains(t, line, "RETORT!", "fallback %s line must carry the retort prefix", audience)
		}
	}
}
