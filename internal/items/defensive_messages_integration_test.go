package items

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

func TestDefenseMessageValidRepositoryPoolsLoadThroughRealLoader(t *testing.T) {
	mudlog.SetupLogger(nil, "", "", false)
	originalItems := items
	originalAttackMessages := attackMessages
	originalDefenseMessages := defenseMessages
	t.Cleanup(func() {
		items = originalItems
		attackMessages = originalAttackMessages
		defenseMessages = originalDefenseMessages
	})
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repoRoot, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadDataFiles()
	for _, defenseType := range []DefenseType{DefenseQuell, DefenseDefy} {
		group := defenseMessages[defenseType]
		if group == nil {
			t.Fatalf("real loader did not load %q", defenseType)
		}
		if err := group.Validate(); err != nil {
			t.Fatalf("loaded %q pool invalid: %v", defenseType, err)
		}
	}
}

func TestDefenseMessageRepositoryPoolsKeepPartialAndKnockdownWordingTruthful(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(here), "..", ".."))
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(repoRoot, "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)
	mudlog.SetupLogger(nil, "", "", false)
	originalItems, originalAttack, originalDefense := items, attackMessages, defenseMessages
	t.Cleanup(func() { items, attackMessages, defenseMessages = originalItems, originalAttack, originalDefense })
	LoadDataFiles()

	for _, defenseType := range []DefenseType{DefenseQuell, DefenseDefy} {
		group := defenseMessages[defenseType]
		for _, band := range []Intensity{Weak, Normal} {
			options := group.Options[band].Together
			for audience, messages := range map[string]MessageOptions{
				"defender": options.ToDefender, "attacker": options.ToAttacker, "room": options.ToRoom,
			} {
				for index, message := range messages {
					lower := strings.ToLower(string(message))
					for _, forbidden := range []string{"no purchase", "hollow noise", "unmoved", "untouched", "no trace", "nothing behind", "breaks completely"} {
						if strings.Contains(lower, forbidden) {
							t.Errorf("%s %s %s[%d] overclaims partial defence with %q: %q", defenseType, band, audience, index, forbidden, message)
						}
					}
					if strings.Contains(lower, `<ansi fg="user">{attacker}</ansi>`) || strings.Contains(lower, `<ansi fg="mob">{defender}</ansi>`) {
						t.Errorf("%s %s %s[%d] hardcodes actor orientation: %q", defenseType, band, audience, index, message)
					}
				}
			}
		}
	}

	quellHeavy := defenseMessages[DefenseQuell].Options[Heavy].Together
	for audience, messages := range map[string]MessageOptions{
		"defender": quellHeavy.ToDefender, "attacker": quellHeavy.ToAttacker, "room": quellHeavy.ToRoom,
	} {
		for index, message := range messages {
			lower := strings.ToLower(string(message))
			if !strings.Contains(lower, "damag") && !strings.Contains(lower, "harm") && !strings.Contains(lower, "mental force") {
				t.Errorf("quell heavy %s[%d] does not limit negation claim to damage/mental force: %q", audience, index, message)
			}
		}
	}
}
