package dialogue

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

// RegistryValidators wires DialogueValidators to the loaded quest and item
// registries. The builder's dialogue editor (modules/gmcp) validates a save
// with it, and the boot smoke sweep validates every shipped dialogue file with
// it, so the editor and CI apply one set of rules. It reads the registries at
// call time: load quests and items before validating.
func RegistryValidators() DialogueValidators {
	return DialogueValidators{
		QuestExists: func(tok string) bool { return quests.GetQuest(tok) != nil },
		QuestEndToken: func(tok string) (string, bool) {
			if q := quests.GetQuest(tok); q != nil {
				return fmt.Sprintf("%d-end", q.QuestId), true
			}
			return "", false
		},
		// The boot check (questengine.ValidateAllFlags) and the quest editor
		// both ask quests.ValidateFlag, which keys flags as "<questId>-<key>"
		// the way content writes them. Matching a quest's bare flag key here
		// rejected every correctly written flag reference.
		FlagDeclared: func(key, value string) bool {
			return quests.ValidateFlag(key, value) == nil
		},
		ItemExists: func(id int) bool { return items.GetItemSpec(id) != nil },
	}
}
