package guards

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"gopkg.in/yaml.v2"
)

// possessionClaim matches a first-step description that says the player
// already holds something: "you were given", "you have a/an/the/some",
// "has given you", "gave you", "(has) handed you", "pressed ... into your
// hands", "entrusted you with", "you carry".
var possessionClaim = regexp.MustCompile(`(?i)\b(you were given|you have (a|an|the|some)\b|has given you|gave you|handed you|pressed\b.*\binto your hands|entrusted you with|you carry)`)

// offerCardPossessionProblems returns why desc cannot stand on an offer card
// whose Accept hands an item over, or "" when it can.
func offerCardPossessionProblems(desc string) string {
	flat := strings.Join(strings.Fields(desc), " ")
	if m := possessionClaim.FindString(flat); m != "" {
		return `claims the player already has the item ("` + m + `")`
	}
	return ""
}

// TestOfferCardFirstStepDoesNotClaimTheItem: an offer card (#375 phase 2a,
// internal/conversation/card.go cardBody) shows the quest's first step
// description before the player accepts, and the same text is the quest log
// line after. When the offer topic hands an item over on Accept (givesItem),
// that description must not say the player already has the item: on the card
// they do not have it yet. Name who hands it over when the player takes the
// job instead.
func TestOfferCardFirstStepDoesNotClaimTheItem(t *testing.T) {
	type step struct {
		Id          string `yaml:"id"`
		Description string `yaml:"description"`
	}
	type quest struct {
		QuestId int    `yaml:"questid"`
		Steps   []step `yaml:"steps"`
	}

	questFiles, err := filepath.Glob("_datafiles/world/dogmud/quests/*.yaml")
	if err != nil || len(questFiles) == 0 {
		t.Fatalf("found no quest files (err %v); the guard cannot run", err)
	}
	firstStep := map[int]string{}
	questFile := map[int]string{}
	for _, f := range questFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var q quest
		if err := yaml.Unmarshal(b, &q); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if len(q.Steps) > 0 {
			firstStep[q.QuestId] = q.Steps[0].Description
			questFile[q.QuestId] = f
		}
	}

	dlgFiles, err := filepath.Glob("_datafiles/world/dogmud/dialogue/*/*.yaml")
	if err != nil || len(dlgFiles) == 0 {
		t.Fatalf("found no dialogue files (err %v)", err)
	}
	checked := map[int]bool{}
	for _, f := range dlgFiles {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var df dialogue.DialogueFile
		if err := yaml.Unmarshal(b, &df); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		for _, tp := range df.Topics {
			if tp.Kind != "offer" || tp.GivesItem == 0 || tp.Quest == 0 {
				continue
			}
			desc, ok := firstStep[tp.Quest]
			if !ok {
				t.Errorf("%s topic %s offers quest %d, which has no quest file with steps", f, tp.Key, tp.Quest)
				continue
			}
			checked[tp.Quest] = true
			if why := offerCardPossessionProblems(desc); why != "" {
				t.Errorf("%s first step %s, but the offer topic %s in %s only hands item %d over on Accept; name who hands it over instead",
					questFile[tp.Quest], why, tp.Key, f, tp.GivesItem)
			}
		}
	}
	ids := make([]int, 0, len(checked))
	for id := range checked {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	t.Logf("checked the first step of %d quests whose offer gives an item: %v", len(ids), ids)
	// Floor: the nine quests this guard was written for (#375 phase 2a).
	if len(ids) < 9 {
		t.Fatalf("found %d quests whose offer topic gives an item; expected at least 9 (6, 10, 14, 32, 39, 42, 50, 60, 66)", len(ids))
	}
}

// TestOfferCardFirstStepDoesNotClaimTheItem_GuardCanFail proves the check
// rejects the shipped quest 32 first step it was written against, and the
// other old phrasings, and passes the reworded forms.
func TestOfferCardFirstStepDoesNotClaimTheItem_GuardCanFail(t *testing.T) {
	bad := []string{
		"You were given a cudgel. Hold it in your hand\n      before you fight. If you already hold it, type remove cudgel, then\n      wield it again.",
		"You have a sling and a pouch of shot. First, ready the sling.",
		"Toll collector Harn at the tollhouse has given you a maintenance report.",
		"Tavern keeper Marek at the Drowning Post gave you a protection notice.",
		"Falv has handed you a firebomb and set you loose in the marsh.",
		"Toller Garrick has entrusted you with a sealed dispatch.",
		"They pressed a spent wax wrapper\n into your hands.",
		"You carry the report to Pell.",
	}
	for _, d := range bad {
		if offerCardPossessionProblems(d) == "" {
			t.Errorf("guard passed a possession claim: %q", d)
		}
	}
	good := []string{
		"Toke hands you a cudgel when you take this on. Hold it in your hand before you fight.",
		"Marek hands you a lantern when you take this on and asks you to investigate.",
		"Iden gives you a sling when you take this on.",
	}
	for _, d := range good {
		if why := offerCardPossessionProblems(d); why != "" {
			t.Errorf("guard rejected a reworded line %q: %s", d, why)
		}
	}
}
