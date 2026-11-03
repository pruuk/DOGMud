package guards

import (
	"os"
	"strings"
	"testing"
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
