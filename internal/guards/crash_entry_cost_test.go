package guards

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/achievements"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// The door's price is said in three places a player reads or pays through.
func TestCrashEntryCostAgrees(t *testing.T) {
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		return strings.ReplaceAll(string(b), "\r\n", "\n")
	}
	beh := read(`_datafiles/world/dogmud/behaviors/eastern_highlands/9553-the_threshold_keeper.yaml`)
	if !strings.Contains(beh, `min_gold: 400`) {
		t.Error(`the Keeper's min_gold is not 400`)
	}
	if !strings.Contains(beh, `I take no less than four hundred.`) {
		t.Error(`the Keeper's guidance does not say four hundred`)
	}
	if strings.Contains(beh, `Two hundred to four hundred`) {
		t.Error(`the Keeper's guidance still has the old lowest band`)
	}
	if help := read(`_datafiles/world/dogmud/templates/help/instances.template`); !strings.Contains(help, `no less than 400 gold`) {
		t.Error(`help instances does not say 400 gold`)
	}
}

func TestTheLastWardenNamesARealMob(t *testing.T) {
	if _, err := os.Stat(`_datafiles/world/dogmud/mobs/crash_site_interior/9562-the_core_guardian.yaml`); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(`_datafiles/world/dogmud/achievements/the-last-warden.yaml`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `mob: 9562`) {
		t.Error(`the-last-warden does not name the Core Guardian`)
	}
}

// Key in the Dark reads the token 76-attune, and HasQuest counts every step
// before the current one, so it unlocks for anyone past attune too: a player who
// walks the new steps, and a save that finished the quest before they existed.
// A Swallowed Stone waits for 77-end, not for the reading before it.
func TestDiscLineQuestAchievementsUnlockFromTheirStepOn(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}
	quests.LoadDataFiles()
	achievements.LoadDataFiles()

	for _, c := range []struct {
		id      string
		questId int
		want    map[string]bool
	}{
		{"key-in-the-dark", 76, map[string]bool{
			"start": false, "journal": false, "shelter": false, "cairn": false, "camp": false,
			"alpha": false, "remains": false, "attune": true, "sentinel": true, "end": true,
		}},
		{"a-swallowed-stone", 77, map[string]bool{"start": false, "read": false, "end": true}},
	} {
		def, ok := achievements.Get(c.id)
		if !ok {
			t.Fatalf("achievement %s did not load", c.id)
		}
		q := quests.GetQuest(fmt.Sprintf("%d-start", c.questId))
		if q == nil {
			t.Fatalf("quest %d did not load", c.questId)
		}
		if len(q.Steps) != len(c.want) {
			t.Fatalf("quest %d has %d steps, this test names %d", c.questId, len(q.Steps), len(c.want))
		}
		for _, s := range q.Steps {
			want, named := c.want[s.Id]
			if !named {
				t.Fatalf("quest %d step %s is not named in this test", c.questId, s.Id)
			}
			char := &characters.Character{QuestProgress: map[int]string{c.questId: s.Id}}
			if got := achievements.Evaluate(def.Trigger, char, 0); got != want {
				t.Errorf("%s on a player at %d-%s: got %v, want %v", c.id, c.questId, s.Id, got, want)
			}
		}
		if achievements.Evaluate(def.Trigger, &characters.Character{}, 0) {
			t.Errorf("%s unlocked for a player who never took the quest", c.id)
		}
	}
}
