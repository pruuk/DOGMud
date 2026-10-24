// Package questworld builds the quest validators' inputs from the live
// registries: reward names, quest reference checks and the proposed world.
// It is the one place the offer card, the boot door and the quest editor
// get them (#375 phase 2).
package questworld

import (
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// Names is quests.RewardNames over the live registries.
func Names() quests.RewardNames {
	return quests.RewardNames{
		Item: func(id int) string {
			if s := items.GetItemSpec(id); s != nil {
				return s.Name
			}
			return ""
		},
		Recipe: func(id string) string {
			if r := crafting.GetRecipe(id); r != nil {
				return r.Name
			}
			return ""
		},
		Spell: func(id string) string {
			if s := spells.GetSpell(id); s != nil {
				return s.Name
			}
			return ""
		},
		Faction: func(id string) string {
			if f := factions.GetDefinition(id); f != nil {
				return f.DisplayName
			}
			return ""
		},
	}
}
