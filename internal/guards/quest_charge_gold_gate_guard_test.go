package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"gopkg.in/yaml.v2"
)

// TestDialogueGoldPurchasesAreAtomic pins how shipped content sells a quest
// for gold through dialogue:
//   - every dialogue topic that charges gold (chargesGold) carries
//     goldRequired at exactly the same price, so the gate and the charge
//     agree, and the charge happens in the same step as the check;
//   - no dialogue entry grants a quest whose quest_granted trigger runs
//     charge_gold. That charge would run later, outside the step that
//     checked the gold, so the price belongs on the dialogue entry instead.
func TestDialogueGoldPurchasesAreAtomic(t *testing.T) {
	type action struct {
		ChargeGold int `yaml:"charge_gold"`
	}
	type trigger struct {
		Event      string   `yaml:"event"`
		QuestToken string   `yaml:"quest_token"`
		Actions    []action `yaml:"actions"`
	}
	type quest struct {
		Triggers []trigger `yaml:"triggers"`
	}

	questFiles, err := filepath.Glob("_datafiles/world/dogmud/quests/*.yaml")
	if err != nil || len(questFiles) == 0 {
		t.Fatalf("found no quest files (err %v); the guard cannot run", err)
	}
	chargedOnGrant := map[string]bool{}
	for _, f := range questFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var q quest
		if err := yaml.Unmarshal(b, &q); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, tr := range q.Triggers {
			if tr.Event != "quest_granted" || tr.QuestToken == "" {
				continue
			}
			for _, a := range tr.Actions {
				if a.ChargeGold > 0 {
					chargedOnGrant[tr.QuestToken] = true
				}
			}
		}
	}

	dlgFiles, err := filepath.Glob("_datafiles/world/dogmud/dialogue/*/*.yaml")
	if err != nil || len(dlgFiles) == 0 {
		t.Fatalf("found no dialogue files (err %v)", err)
	}
	paidGrants := 0
	// A sale (kind: sale) charges with no goldRequired by design: it stays on
	// the menu so the player can read the price on its Accept card, and the
	// charge is all-or-nothing in dialogue.Apply. It grants no quest, so it
	// counts as a paid sale where an offer counts as a paid grant.
	check := func(file, where, kind, grants string, goldRequired, chargesGold int) {
		if chargesGold > 0 || goldRequired > 0 {
			if kind != dialogue.KindSale && chargesGold > 0 && goldRequired != chargesGold {
				t.Errorf("%s %s: chargesGold %d but goldRequired %d; they must match",
					file, where, chargesGold, goldRequired)
			}
			if kind == dialogue.KindSale && goldRequired != 0 {
				t.Errorf("%s %s: a sale takes no goldRequired, Accept is its gate", file, where)
			}
			if chargesGold > 0 && (grants != "" || kind == dialogue.KindSale) {
				paidGrants++
			}
		}
		if grants != "" && chargedOnGrant[grants] {
			t.Errorf("%s %s grants %s, whose quest_granted trigger runs charge_gold; charge in the dialogue entry (chargesGold) instead",
				file, where, grants)
		}
	}
	for _, f := range dlgFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var df dialogue.DialogueFile
		if err := yaml.Unmarshal(b, &df); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		// #375: every entry is a topic; opener variants carry gates only,
		// so they can neither charge nor grant.
		for _, tp := range df.Topics {
			check(f, "topic "+tp.Key, tp.Kind, tp.GrantsQuest, tp.GoldRequired, tp.ChargesGold)
		}
		if df.Opener != nil {
			for i, v := range df.Opener.Variants {
				check(f, fmt.Sprintf("opener variant %d", i), "", "", v.GoldRequired, 0)
			}
		}
	}
	if paidGrants < 9 {
		t.Fatalf("found %d paid quest grants and sales; expected at least Veyra's nine teach_ sales", paidGrants)
	}
}
