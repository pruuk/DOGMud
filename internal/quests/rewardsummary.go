package quests

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// RewardNames are the registry lookups RewardSummary needs, injected the way
// QuestValidators is so this package imports no registry. Each returns ""
// for an unknown id, and that part of the line is left out (the refs
// validator reports the bad id).
type RewardNames struct {
	Item    func(id int) string    // display name, "Healing Salve"
	Recipe  func(id string) string // recipe name
	Spell   func(id string) string // spell name
	Faction func(id string) string // faction display name
}

// RewardSummary is the offer card's reward text (#375 P2): the authored
// Rewards.Summary when set, else the completion payout in words. Gold and
// stockpile counts are the only numbers it shows. "" means nothing to show.
func RewardSummary(q *Quest, n RewardNames) string {
	if q == nil {
		return ""
	}
	if s := strings.TrimSpace(q.Rewards.Summary); s != "" {
		return s
	}
	r := q.Rewards
	var parts []string
	if r.Gold > 0 {
		parts = append(parts, fmt.Sprintf("%d gold", r.Gold))
	}
	if name := lookupInt(n.Item, r.ItemId); name != "" {
		parts = append(parts, articled(name))
	}
	for _, g := range ParseItemGrants(r.ItemInfo) {
		name := lookupInt(n.Item, g.ItemId)
		if name == "" {
			continue
		}
		if g.Qty > 1 {
			parts = append(parts, fmt.Sprintf("%s (x%d)", name, g.Qty))
		} else {
			parts = append(parts, articled(name))
		}
	}
	for _, id := range ParseRecipeGrants(r.RecipeInfo) {
		if name := lookupStr(n.Recipe, id); name != "" {
			parts = append(parts, "the recipe for "+name)
		}
	}
	if name := lookupStr(n.Spell, r.SpellId); name != "" {
		parts = append(parts, "the spell "+name)
	}
	for _, g := range ParseSkillGrants(r.SkillInfo) {
		parts = append(parts, "training in "+strings.ReplaceAll(g.Skill, "-", " "))
	}
	for _, g := range ParseStatGrants(r.StatInfo) {
		if g.Amount > 0 && g.Stat != "" {
			parts = append(parts, "more "+strings.ToUpper(g.Stat[:1])+g.Stat[1:])
		}
	}
	if r.RepAmount > 0 {
		if name := lookupStr(n.Faction, r.RepFaction); name != "" {
			parts = append(parts, "the goodwill of "+name)
		}
	}
	return joinParts(parts)
}

// RewardPaidNames are the names of every item, recipe and spell the quest
// pays on completion, in RewardSummary's order: what an authored Summary
// must mention unless the reward is a surprise.
func RewardPaidNames(q *Quest, n RewardNames) []string {
	var out []string
	if q == nil {
		return out
	}
	r := q.Rewards
	if name := lookupInt(n.Item, r.ItemId); name != "" {
		out = append(out, name)
	}
	for _, g := range ParseItemGrants(r.ItemInfo) {
		if name := lookupInt(n.Item, g.ItemId); name != "" {
			out = append(out, name)
		}
	}
	for _, id := range ParseRecipeGrants(r.RecipeInfo) {
		if name := lookupStr(n.Recipe, id); name != "" {
			out = append(out, name)
		}
	}
	if name := lookupStr(n.Spell, r.SpellId); name != "" {
		out = append(out, name)
	}
	return out
}

func lookupInt(f func(int) string, id int) string {
	if f == nil || id <= 0 {
		return ""
	}
	return f(id)
}

func lookupStr(f func(string) string, id string) string {
	if f == nil || id == "" {
		return ""
	}
	return f(id)
}

// articled puts "a", "an" or "some" before an item name, by the one rule
// the game uses (items.ArticlePrefix).
func articled(name string) string {
	return items.ArticlePrefix(name) + name
}

// joinParts joins "A", "A and B", "A, B, and C".
func joinParts(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
}
