package questworld

import (
	"errors"
	"os"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/crafting"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/factions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Validators is quests.QuestValidators over the live registries: the
// existence checks ValidateQuestRefs runs at the boot door, the CI sweep and
// the quest editor's save (moved here from modules/gmcp).
func Validators() quests.QuestValidators {
	statNames := map[string]bool{"strength": true, "dexterity": true, "perception": true,
		"vitality": true, "willpower": true, "charisma": true}
	skillNames := map[string]bool{}
	for _, s := range skills.GetAllSkillNames() {
		skillNames[string(s)] = true
	}
	dlgGrants := dialogueGrantTokens()
	return quests.QuestValidators{
		StepExists:      func(tok string) bool { return quests.GetQuest(tok) != nil },
		MobExists:       func(id int) bool { return mobs.GetMobSpec(mobs.MobId(id)) != nil },
		ItemExists:      func(id int) bool { return items.GetItemSpec(id) != nil },
		RoomExists:      func(id int) bool { return rooms.LoadRoomTemplate(id) != nil },
		ConditionExists: func(id int) bool { return conditions.GetConditionSpec(id) != nil },
		SpellExists:     func(id string) bool { return spells.GetSpell(id) != nil },
		SkillExists:     func(name string) bool { return skillNames[name] },
		StatExists:      func(name string) bool { return statNames[name] },
		RecipeExists:    func(id string) bool { return crafting.GetRecipe(id) != nil },
		FactionExists:   func(id string) bool { return factions.GetDefinition(id) != nil },
		FlagDeclared: func(key, value string) bool {
			return quests.ValidateFlag(key, value) == nil
		},
		DialogueGrants: func(token string) bool { return dlgGrants[token] },
		MobHasDialogue: mobHasDialogueFile,
	}
}

// dialogueGrantTokens indexes every token any dialogue file grants, the "is
// this step reachable" warning's dialogue side. Only topics carry grants
// (opener variants carry gates only).
func dialogueGrantTokens() map[string]bool {
	granted := map[string]bool{}
	dialogue.WalkAllFiles(func(mobId int, zone string, df *dialogue.DialogueFile) {
		for _, t := range df.Topics {
			if t.GrantsQuest != "" {
				granted[t.GrantsQuest] = true
			}
		}
	})
	return granted
}

// mobHasDialogueFile checks by mob template zone. Uses dialogue.Load (and so
// shares its cache behavior: fine, MainWorker only, and Load's nil-sentinel
// semantics are the live game's own).
func mobHasDialogueFile(mobId int) bool {
	m := mobs.GetMobSpec(mobs.MobId(mobId))
	if m == nil {
		return false
	}
	return dialogue.Load(mobId, m.Zone) != nil
}

// Registry is the loop world over the loaded registries: every quest, every
// dialogue file (strictly decoded; their own problems are the dialogue
// door's), the live reference checks and reward names.
func Registry() questloop.World {
	all := quests.GetAllQuests()
	qs := make([]*quests.Quest, 0, len(all))
	for i := range all {
		qs = append(qs, &all[i])
	}
	files, _ := dialogue.LoadAllFiles()
	v := Validators()
	return questloop.World{Quests: qs, Dialogue: files, Refs: &v, Names: Names()}
}

// WithQuest is w with q in place of the quest that has its id (or added),
// the proposed world an editor save is checked against.
func WithQuest(w questloop.World, q quests.Quest) questloop.World {
	out := w
	out.Quests = nil
	replaced := false
	for _, existing := range w.Quests {
		if existing != nil && existing.QuestId == q.QuestId {
			out.Quests = append(out.Quests, &q)
			replaced = true
			continue
		}
		out.Quests = append(out.Quests, existing)
	}
	if !replaced {
		out.Quests = append(out.Quests, &q)
	}
	return out
}

// BacklogPath is the shipped backlog's path.
func BacklogPath() string {
	return util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/quests/loop-backlog.txt`)
}

// LoadBacklog reads the shipped backlog; a missing file is an empty backlog.
func LoadBacklog() (questloop.Backlog, error) {
	data, err := os.ReadFile(BacklogPath())
	if errors.Is(err, os.ErrNotExist) {
		return questloop.Backlog{}, nil
	}
	if err != nil {
		return nil, err
	}
	return questloop.ParseBacklog(data)
}
