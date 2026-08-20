package items

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// TestNewlyDefendableAttackNamesRenderTriads is the U6b Task 9 loader gate:
// every attack newly routed through the channel defence seam must render a
// non-empty defender/attacker/room triad for each defence in its channel's
// set, at each band, for every variant — with all tokens substituted and the
// attack name folded in cleanly.
func TestNewlyDefendableAttackNamesRenderTriads(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	originalItems, originalAttack, originalDefense := items, attackMessages, defenseMessages
	t.Cleanup(func() { items, attackMessages, defenseMessages = originalItems, originalAttack, originalDefense })

	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repoRoot, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)
	LoadDataFiles()

	// The melee channel's defence set answers bash/kick/trip and the beast
	// moves; the ranged set answers aimed shots and (Task 15) thrown
	// grenades; physical spells reach dodge/block; mental spells reach
	// quell; taunts reach defy. Attack names are the exact strings the
	// converted call sites pass to RenderChannelDefenceMessages.
	meleeDefences := []DefenseType{DefenseDodge, DefenseParry, DefenseBlock}
	rangedDefences := []DefenseType{DefenseDodge, DefenseBlock}

	cases := []struct {
		attack   string
		defences []DefenseType
	}{
		{"shield bash", meleeDefences},
		{"crushing slam", meleeDefences},
		{"kick", meleeDefences},
		{"stomp", meleeDefences},
		{"knee strike", meleeDefences},
		{"trip", meleeDefences},
		{"tailsweep", meleeDefences},
		{"charge", meleeDefences},
		{"goring charge", meleeDefences},
		{"hamstring slash", meleeDefences},
		{"savage bite", meleeDefences},
		{"pounce", meleeDefences},
		{"claw rake", meleeDefences},
		{"draining grasp", meleeDefences},
		{"throttle lunge", meleeDefences},
		{"aimed shot", rangedDefences},
		{"firebomb", rangedDefences},
		// Spell/taunt exemplars: a spell name must read naturally through
		// the mental and physical-spell defence pools, a taunt through defy.
		{"Mind Fog", []DefenseType{DefenseQuell, DefenseDodge, DefenseBlock}},
		{"taunt", []DefenseType{DefenseDefy}},
	}

	bands := []struct {
		name   string
		band   Intensity
		crit   bool
		margin float64
	}{
		{"weak", Weak, false, 0.1},
		{"normal", Normal, false, 0.6},
		{"heavy", Heavy, true, 0.0},
	}

	leftoverTokens := []string{"{attack}", "{attacker}", "{defender}", "{weapon}"}

	for _, tc := range cases {
		for _, defence := range tc.defences {
			group := defenseMessages[defence]
			if group == nil {
				t.Fatalf("no message pool loaded for defence %q", defence)
			}
			for _, band := range bands {
				variants := len(group.Options[band.band].Together.ToDefender)
				if variants < 5 {
					t.Fatalf("%s %s has %d variants; want at least 5", defence, band.name, variants)
				}
				for idx := 0; idx < variants; idx++ {
					triad := RenderDefenseMessage(defence, band.crit, band.margin, map[TokenName]string{
						TokenAttacker: "Rurik",
						TokenDefender: "Selka",
						TokenAttack:   tc.attack,
						TokenWeapon:   tc.attack,
					}, idx)
					for audience, msg := range map[string]ItemMessage{
						"defender": triad.ToDefender,
						"attacker": triad.ToAttacker,
						"room":     triad.ToRoom,
					} {
						text := string(msg)
						if strings.TrimSpace(text) == "" {
							t.Errorf("%s vs %s %s[%d] %s line is empty", tc.attack, defence, band.name, idx, audience)
							continue
						}
						for _, leftover := range leftoverTokens {
							if strings.Contains(text, leftover) {
								t.Errorf("%s vs %s %s[%d] %s line leaves token %s unsubstituted: %q",
									tc.attack, defence, band.name, idx, audience, leftover, text)
							}
						}
						if strings.Contains(text, "—") || strings.Contains(text, "–") {
							t.Errorf("%s vs %s %s[%d] %s line contains an em/en dash: %q",
								tc.attack, defence, band.name, idx, audience, text)
						}
					}
				}
			}
		}
	}
}

// TestQuellFizzleFlavorLivesOnlyInTheHeavyBand pins the ratified copy decision
// (U6b Assumption 1): the word "fizzle" survives ONLY inside quell's
// defensive-crit (heavy) band, where the spell truly is fully stopped. It must
// never narrate a partial hit — no weak/normal variant in any pool may use it.
func TestQuellFizzleFlavorLivesOnlyInTheHeavyBand(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	originalItems, originalAttack, originalDefense := items, attackMessages, defenseMessages
	t.Cleanup(func() { items, attackMessages, defenseMessages = originalItems, originalAttack, originalDefense })

	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repoRoot, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)
	LoadDataFiles()

	allDefences := []DefenseType{DefenseDodge, DefenseParry, DefenseBlock, DefenseQuell, DefenseDefy}
	fizzleFound := false
	for _, defence := range allDefences {
		group := defenseMessages[defence]
		if group == nil {
			t.Fatalf("no message pool loaded for defence %q", defence)
		}
		for band, options := range group.Options {
			for audience, messages := range map[string]MessageOptions{
				"defender": options.Together.ToDefender,
				"attacker": options.Together.ToAttacker,
				"room":     options.Together.ToRoom,
			} {
				for idx, msg := range messages {
					if !strings.Contains(strings.ToLower(string(msg)), "fizzle") {
						continue
					}
					if defence != DefenseQuell || band != Heavy {
						t.Errorf("%s %s %s[%d] uses 'fizzle' outside quell's heavy band: %q",
							defence, band, audience, idx, msg)
						continue
					}
					fizzleFound = true
				}
			}
		}
	}
	if !fizzleFound {
		t.Error("quell's heavy band carries no 'fizzle' variant; the ratified flavor is missing")
	}
}
