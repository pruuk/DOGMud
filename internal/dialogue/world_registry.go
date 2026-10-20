package dialogue

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
)

// LoadAllFiles reads every dialogue file under {DataFiles}/dialogue
// strictly: an unknown key is a failure, so a stray old-format field or a
// typo cannot be silently ignored. It bypasses the live cache.
func LoadAllFiles() (files []*DialogueFile, failures []string) {
	base := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "dialogue")
	paths, _ := filepath.Glob(filepath.Join(base, "*", "*.yaml"))
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: unreadable: %v", filepath.ToSlash(p), err))
			continue
		}
		var df DialogueFile
		if err := yaml.UnmarshalStrict(data, &df); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", filepath.ToSlash(p), err))
			continue
		}
		files = append(files, &df)
	}
	return files, failures
}

// RegistryWorld is the loaded world: the shipped dialogue files plus the
// quest and item registries. Load quests and items first. Unreadable files
// come back as problems. behaviorTopics may be nil.
func RegistryWorld(behaviorTopics func(mobId int) []string) (World, []Problem) {
	files, failures := LoadAllFiles()
	var probs []Problem
	for _, f := range failures {
		probs = append(probs, Problem{Where: "file", Msg: f})
	}
	info := func(q *quests.Quest) (QuestInfo, bool) {
		if q == nil || len(q.Steps) == 0 {
			return QuestInfo{}, false
		}
		return QuestInfo{
			Title:      q.Name,
			StartToken: quests.PartsToToken(q.QuestId, q.Steps[0].Id),
			EndToken:   fmt.Sprintf("%d-end", q.QuestId),
		}, true
	}
	var giveGrants []string
	for _, q := range quests.GetAllQuests() {
		for i, t := range q.Triggers {
			if t.Event != "item_give" {
				continue
			}
			for _, a := range t.Actions {
				if a.Grant != "" {
					giveGrants = append(giveGrants, fmt.Sprintf("quest %d trigger %d", q.QuestId, i))
					break
				}
			}
		}
	}
	return World{
		Files:     files,
		QuestById: func(id int) (QuestInfo, bool) { return info(quests.GetQuestById(id)) },
		QuestOfToken: func(tok string) (QuestInfo, bool) {
			if !strings.Contains(tok, "-") {
				return QuestInfo{}, false
			}
			return info(quests.GetQuest(tok))
		},
		FlagDeclared:   func(k, v string) bool { return quests.ValidateFlag(k, v) == nil },
		ItemExists:     func(id int) bool { return items.GetItemSpec(id) != nil },
		BehaviorTopics: behaviorTopics,
		ItemGiveGrants: giveGrants,
	}, probs
}
