package main

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
//   - every dialogue entry that charges gold (chargesGold) carries
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
	check := func(file, where, grants string, goldRequired, chargesGold int) {
		if chargesGold > 0 || goldRequired > 0 {
			if chargesGold > 0 && goldRequired != chargesGold {
				t.Errorf("%s %s: chargesGold %d but goldRequired %d; they must match",
					file, where, chargesGold, goldRequired)
			}
			if chargesGold > 0 && grants != "" {
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
		for i, p := range df.Patterns {
			check(f, fmt.Sprintf("pattern %d", i), p.GrantsQuest, p.GoldRequired, p.ChargesGold)
		}
		if df.Tree == nil {
			continue
		}
		for i, v := range df.Tree.Root.Variants {
			check(f, fmt.Sprintf("root variant %d", i), v.GrantsQuest, v.GoldRequired, v.ChargesGold)
		}
		for _, n := range df.Tree.Nodes {
			check(f, "node "+n.Id, n.GrantsQuest, n.GoldRequired, n.ChargesGold)
		}
	}
	if paidGrants < 9 {
		t.Fatalf("found %d paid quest grants; expected at least Veyra's nine teach_ nodes", paidGrants)
	}
}
