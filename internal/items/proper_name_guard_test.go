package items

import (
	"sort"
	"strings"
	"testing"
)

// Every shipped item whose name opens with a possessive is either named for
// one person (proper_name: true, no article: "You receive Edvar's
// Map-Fragment.") or a trade or role possessive (an article: "a Foreman's
// Tally-Book"). Nothing in the data tells the two apart, so each name is
// classified here by hand and a new one fails until it is (#799). Evidence
// for each person: a mob or dialogue of that name in _datafiles/world/dogmud.
var (
	// properNameItems are the item ids that must carry proper_name: true.
	properNameItems = map[int]string{
		40113: "Edvar's Map-Fragment",
		40109: "Ysolde's Purge",
		40120: "Ostry's Gratitude",
		40116: "Lysha's Annotated Reading",
		40060: "Elgar's Carved Kingfisher",
		40061: "Elgar's Last Journal Entry",
		10032: "Elgar's Hook-spear",
		20067: "Soren's Ironbound Buckler",
		54:    "Soren's Iron Pin",
		39:    "Tova's Lockbox Key",
		40:    "Halix's Lockbox Key",
		41:    "Kessa's Lockbox Key",
		10025: "Edrin's Gnarled Staff",
	}
	// tradePossessiveItems open with a possessive that names a trade, role,
	// kind or creature, so they keep their article.
	tradePossessiveItems = map[int]string{
		20049: "Warden's Tabard", 20068: "Bandit's Leather Vest",
		20062: "Phantom's Wraps", 20065: "Phantom's Cowl",
		20134: "Keeper's Lantern", 30034: "Artisan's Satchel",
		30035: "Master's Component Case", 30039: "Warrior's Brew",
		30040: "Preacher's Tincture", 30047: "Cat's Eye Draught",
		30054: "Savant's Infusion", 40004: "Healer's Root",
		40074: "Courier's Road-Token", 40100: "Foreman's Tally-Book",
		40165: "Sentinel's Heartstone", 40184: "Wayfarer's Bottomless Pack",
		16: "Bandit's Purse", 10023: "Smuggler's Short Blade",
		10026: "Bandit's Longsword", 10044: "Watch-Captain's Blade",
		10047: "Warden's Lash",
		// Bounty Hunter, Guard Captain, Pit Fighter: roles.
		20086: "Bounty Hunter's Cloak", 20080: "Bounty Hunter's Coat",
		20083: "Bounty Hunter's Boots", 20084: "Bounty Hunter's Gloves",
		20081: "Bounty Hunter's Hood", 20082: "Bounty Hunter's Leggings",
		20085: "Bounty Hunter's Badge", 10037: "Bounty Hunter's Blade",
		40037: "Guard Captain's Commendation",
		10085: "Pit Fighter's Knife", 10086: "Pit Fighter's Sword",
		// Old White is a creature (the mob "The Old White"), not a person:
		// left with its article until the owner rules.
		40179: "Old White's Fang",
	}
)

func TestPossessiveItemNamesAreClassified(t *testing.T) {
	loadRealItemSpecsForTest(t)

	var unclassified, wrongFlag []string
	for id, spec := range items {
		_, isPerson := properNameItems[id]
		_, isTrade := tradePossessiveItems[id]
		// The first word may sit after "Bounty Hunter's": a possessive
		// anywhere in the first three words counts as opening with one.
		words := strings.Fields(spec.Name)
		if len(words) > 3 {
			words = words[:3]
		}
		opens := false
		for _, w := range words {
			if strings.HasSuffix(w, `'s`) {
				opens = true
			}
		}
		if opens && !isPerson && !isTrade && !strings.HasPrefix(strings.ToLower(spec.Name), `a `) &&
			!strings.HasPrefix(spec.Name, `Copy of `) {
			unclassified = append(unclassified, spec.Name)
		}
		if isPerson && !spec.ProperName {
			wrongFlag = append(wrongFlag, spec.Name+" lacks proper_name")
		}
		if !isPerson && spec.ProperName {
			wrongFlag = append(wrongFlag, spec.Name+" has proper_name but is not in properNameItems")
		}
	}
	sort.Strings(unclassified)
	sort.Strings(wrongFlag)
	if len(unclassified) > 0 {
		t.Errorf("names open with a possessive and are not classified in proper_name_guard_test.go (a person: set proper_name: true and list it in properNameItems; a trade or role: list it in tradePossessiveItems): %v", unclassified)
	}
	if len(wrongFlag) > 0 {
		t.Errorf("proper_name does not match properNameItems: %v", wrongFlag)
	}
	for id, name := range properNameItems {
		if spec := GetItemSpec(id); spec == nil || !strings.EqualFold(spec.Name, name) {
			t.Errorf("properNameItems[%d] = %q does not match the shipped item", id, name)
		}
	}
	for id, name := range tradePossessiveItems {
		if spec := GetItemSpec(id); spec == nil || !strings.EqualFold(spec.Name, name) {
			t.Errorf("tradePossessiveItems[%d] = %q does not match the shipped item", id, name)
		}
	}
}
