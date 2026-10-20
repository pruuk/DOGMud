package dialogue

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"gopkg.in/yaml.v2"
)

// oneLine folds a multi-line parser error into one line. It joins with
// " | " because ';' is the command separator in player text.
func oneLine(s string) string {
	parts := strings.Split(strings.TrimSpace(s), "\n")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, " | ")
}

// mobIdFromPath reads the mob id from a file name like ".../9491.yaml".
func mobIdFromPath(path string) (int, error) {
	name := strings.TrimSuffix(filepath.Base(path), ".yaml")
	id, err := strconv.Atoi(name)
	if err != nil {
		return 0, fmt.Errorf("file name %q is not a mob id", filepath.Base(path))
	}
	return id, nil
}

// decodeFile decodes one dialogue file strictly: an unknown key is an error,
// so a stray old-format field or a typo cannot be silently ignored, and the
// mobid field must equal the file name (the live Load keys by file name).
// Errors are one line.
func decodeFile(path string, data []byte) (*DialogueFile, error) {
	var df DialogueFile
	if err := yaml.UnmarshalStrict(data, &df); err != nil {
		return nil, fmt.Errorf("%s", oneLine(err.Error()))
	}
	id, err := mobIdFromPath(path)
	if err != nil {
		return nil, err
	}
	if df.MobId != id {
		return nil, fmt.Errorf("mobid %d does not match the file name %s", df.MobId, filepath.Base(path))
	}
	return &df, nil
}

// LoadAllFiles reads every dialogue file under {DataFiles}/dialogue
// strictly (see decodeFile). It bypasses the live cache. A folder with no
// files at all is a problem, so a wrong data path cannot pass as clean.
func LoadAllFiles() (files []*DialogueFile, problems []Problem) {
	base := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "dialogue")
	paths, _ := filepath.Glob(filepath.Join(base, "*", "*.yaml"))
	if len(paths) == 0 {
		problems = append(problems, Problem{Where: filepath.ToSlash(base), Msg: "no dialogue files found"})
	}
	for _, p := range paths {
		id, _ := mobIdFromPath(p)
		where := filepath.ToSlash(p)
		data, err := os.ReadFile(p)
		if err != nil {
			problems = append(problems, Problem{MobId: id, Where: where, Msg: "unreadable: " + oneLine(err.Error())})
			continue
		}
		df, err := decodeFile(p, data)
		if err != nil {
			problems = append(problems, Problem{MobId: id, Where: where, Msg: err.Error()})
			continue
		}
		files = append(files, df)
	}
	return files, problems
}

// tokenStepValid reports whether step names one of q's steps. An empty
// step ("7-") and the wildcard ("7-all+") are not quest tokens a dialogue
// file may name.
func tokenStepValid(q *quests.Quest, step string) bool {
	if q == nil || step == "" {
		return false
	}
	for _, s := range q.Steps {
		if s.Id == step {
			return true
		}
	}
	return false
}

// grantsInActions reports whether any action, or any action run when a
// sequence completes, grants a quest token.
func grantsInActions(actions []quests.ActionDef) bool {
	for _, a := range actions {
		if a.Grant != "" {
			return true
		}
		if a.Sequence != nil && grantsInActions(a.Sequence.OnComplete) {
			return true
		}
	}
	return false
}

// RegistryWorld is the loaded world: the shipped dialogue files plus the
// quest and item registries. Load quests and items first. Unreadable files
// come back as problems. behaviorTopics (mob id to topic_is keys) may be nil.
func RegistryWorld(behaviorTopics map[int][]string) (World, []Problem) {
	files, probs := LoadAllFiles()
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
			if t.Event == "item_give" && grantsInActions(t.Actions) {
				giveGrants = append(giveGrants, fmt.Sprintf("quest %d trigger %d", q.QuestId, i))
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
			id, step := quests.TokenToParts(tok)
			q := quests.GetQuestById(id)
			if !tokenStepValid(q, step) {
				return QuestInfo{}, false
			}
			return info(q)
		},
		FlagDeclared:   func(k, v string) bool { return quests.ValidateFlag(k, v) == nil },
		ItemExists:     func(id int) bool { return items.GetItemSpec(id) != nil },
		BehaviorTopics: behaviorTopics,
		ItemGiveGrants: giveGrants,
	}, probs
}
