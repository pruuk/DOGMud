package questworld

import (
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"

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
	effects := map[string]bool{}
	for _, sp := range spells.GetAllSpells() {
		if sp.EffectType != "" {
			effects[sp.EffectType] = true
		}
	}
	return quests.QuestValidators{
		StepExists:      func(tok string) bool { return quests.GetQuest(tok) != nil },
		MobExists:       func(id int) bool { return mobs.GetMobSpec(mobs.MobId(id)) != nil },
		ItemExists:      func(id int) bool { return items.GetItemSpec(id) != nil },
		RoomExists:      func(id int) bool { return rooms.LoadRoomTemplate(id) != nil },
		ConditionExists: func(id int) bool { return conditions.GetConditionSpec(id) != nil },
		SpellExists:     func(id string) bool { return spells.GetSpell(id) != nil },
		EffectExists:    func(e string) bool { return effects[e] },
		SkillExists:     func(name string) bool { return skillNames[name] },
		StatExists:      func(name string) bool { return statNames[name] },
		RecipeExists:    func(id string) bool { return crafting.GetRecipe(id) != nil },
		FactionExists:   func(id string) bool { return factions.GetDefinition(id) != nil },
		FlagDeclared: func(key, value string) bool {
			return quests.ValidateFlag(key, value) == nil
		},
		DialogueGrants: func(token string) bool { return dlgGrants[token] },
		MobHasDialogue: mobHasDialogueFile,
		TopicExists:    topicExistsOnMob,
		TopicGrant:     topicGrantOnMob,
		HiddenNounExists: func(roomId int, key string) bool {
			r := rooms.LoadRoomTemplate(roomId)
			if r == nil {
				return false
			}
			_, ok := r.HiddenNouns[key]
			return ok
		},
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

// topicExistsOnMob reports whether the mob's dialogue file has the topic key
// (a topic_heard trigger's topic, #375 phase 3b slice 1).
func topicExistsOnMob(mobId int, key string) bool {
	m := mobs.GetMobSpec(mobs.MobId(mobId))
	if m == nil {
		return false
	}
	df := dialogue.Load(mobId, m.Zone)
	return df != nil && df.FindTopic(key) != nil
}

// topicGrantOnMob is the token the mob's topic grants through its own
// grantsQuest, or "" (none, or no such topic). A topic_heard trigger on that
// topic may not grant it again.
func topicGrantOnMob(mobId int, key string) string {
	m := mobs.GetMobSpec(mobs.MobId(mobId))
	if m == nil {
		return ""
	}
	if t := dialogue.Load(mobId, m.Zone).FindTopic(key); t != nil {
		return t.GrantsQuest
	}
	return ""
}

// Registry is the loop world over the loaded registries: every quest, every
// dialogue file (strictly decoded; their own problems are the dialogue
// door's), the live reference checks, reward names and the external grants
// (behaviour-tree grant_quest actions and item questtokens, externalGrants).
func Registry() questloop.World {
	return RegistryWith(Validators())
}

// RegistryWith is Registry with reference checks the caller already built,
// so the quest editor builds them once per save.
func RegistryWith(v quests.QuestValidators) questloop.World {
	all := quests.GetAllQuests()
	qs := make([]*quests.Quest, 0, len(all))
	for i := range all {
		qs = append(qs, &all[i])
	}
	files, _ := dialogue.LoadAllFiles()
	return questloop.World{Quests: qs, Dialogue: files, Refs: &v, Names: Names(), External: externalGrants()}
}

// WithQuest is w with q in place of the quest that has its id (or added),
// the proposed world an editor save is checked against. w is not changed;
// its dialogue, names and external grants carry over as they are.
//
// Its Refs is a copy of w's whose step and flag checks read q for q's own
// tokens and flags ("<q.QuestId>-..."), so the refs rule judges every other
// quest against the edited steps and flags rather than the loaded ones: a
// step rename that breaks another quest's trigger is a finding here, not a
// boot panic later. Every other token and flag goes to w's checks.
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
	if w.Refs != nil {
		r := *w.Refs
		r.StepExists = overlayStepExists(&q, w.Refs.StepExists)
		r.FlagDeclared = overlayFlagDeclared(&q, w.Refs.FlagDeclared)
		out.Refs = &r
	}
	return out
}

// ownPart returns the part of a "<id>-<rest>" token after q's id prefix.
func ownPart(q *quests.Quest, tok string) (string, bool) {
	return strings.CutPrefix(tok, strconv.Itoa(q.QuestId)+quests.QuestTokenSeparator)
}

// overlayStepExists resolves q's tokens the way quests.GetQuest does (a
// named step, or the "all+" wildcard) against q itself.
func overlayStepExists(q *quests.Quest, next func(string) bool) func(string) bool {
	return func(tok string) bool {
		step, own := ownPart(q, tok)
		if !own {
			return next != nil && next(tok)
		}
		step, _, _ = strings.Cut(step, quests.QuestTokenSeparator)
		if step == `all+` {
			return true
		}
		for _, s := range q.Steps {
			if s.Id == step {
				return true
			}
		}
		return false
	}
}

// overlayFlagDeclared resolves q's flags the way quests.ValidateFlag does
// (declared, and the value allowed or empty) against q's declarations.
func overlayFlagDeclared(q *quests.Quest, next func(key, value string) bool) func(key, value string) bool {
	return func(key, value string) bool {
		name, own := ownPart(q, key)
		if !own {
			return next != nil && next(key, value)
		}
		for _, f := range q.Flags {
			if f.Key != name {
				continue
			}
			return value == "" || slices.Contains(f.Values, value)
		}
		return false
	}
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
