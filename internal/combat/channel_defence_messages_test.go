package combat

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

func channelMessageFixture() *items.DefenseMessageGroup {
	mk := func(prefix string) items.DefenseOptions {
		message := func(suffix string) items.ItemMessage { return items.ItemMessage(prefix + suffix) }
		return items.DefenseOptions{Together: items.DefenseTogetherMessages{
			ToDefender: items.MessageOptions{message("-def-0"), message("-def-1"), message("-def-2"), message("-def-3"), message("-def-4")},
			ToAttacker: items.MessageOptions{message("-atk-0"), message("-atk-1"), message("-atk-2"), message("-atk-3"), message("-atk-4")},
			ToRoom:     items.MessageOptions{message("-room-0"), message("-room-1"), message("-room-2"), message("-room-3"), message("-room-4")},
		}}
	}
	return &items.DefenseMessageGroup{OptionId: items.DefenseQuell, Options: items.DefenseIntensity{
		items.Weak: mk("weak"), items.Normal: mk("normal"), items.Heavy: mk("heavy"),
	}}
}

func TestChannelDefenceMessagesUsesCanonicalOutcomeWithoutRerolling(t *testing.T) {
	restore := items.SeedDefenseMessagesForTest(map[items.DefenseType]*items.DefenseMessageGroup{items.DefenseQuell: channelMessageFixture()})
	defer restore()

	tests := []struct {
		name string
		out  ChannelDefenceResult
		want string
	}{
		{"attack_win_has_no_false_success", ChannelDefenceResult{DefenceType: "quell", Defended: false, NormalizedDefenceMargin: 4, DefensiveCrit: true, DamageMultiplier: 1}, ""},
		{"partial_narrow_is_weak", ChannelDefenceResult{DefenceType: "quell", Defended: true, NormalizedDefenceMargin: 0.49, DamageMultiplier: 0.4}, "weak"},
		{"partial_large_margin_is_normal_not_heavy", ChannelDefenceResult{DefenceType: "quell", Defended: true, NormalizedDefenceMargin: 9, DamageMultiplier: 0.1}, "normal"},
		{"defensive_crit_is_heavy", ChannelDefenceResult{DefenceType: "quell", Defended: true, NormalizedDefenceMargin: 0.01, DefensiveCrit: true, DamageMultiplier: 0}, "heavy"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := RenderChannelDefenceMessages(tc.out, "Rurik", "Selka", "Mind Fog", 1)
			if tc.want == "" {
				if got.ToDefender != "" || got.ToAttacker != "" || got.ToRoom != "" {
					t.Fatalf("attack win rendered defence success: %+v", got)
				}
				return
			}
			for audience, line := range map[string]string{"defender": string(got.ToDefender), "attacker": string(got.ToAttacker), "room": string(got.ToRoom)} {
				if !strings.HasPrefix(line, tc.want+"-") || !strings.HasSuffix(line, "-1") {
					t.Fatalf("%s line %q does not use %s band and shared index 1", audience, line, tc.want)
				}
			}
		})
	}
}
