package keywords

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/require"
)

// TestHelpListsHoldEachTopicOnceAndOnlySkillsAsSkills loads the shipped
// DOGMud keywords.yaml and holds the `help` index lists to two rules (#620:
// the Skills list showed map, sneak, track and foraging):
//
//   - a topic under `skill` is a real skill (skills.GetAllSkillNames);
//   - every topic is listed once across the whole help map. The loader keys
//     topics by name, so a second listing overwrites the first and the topic
//     lands in whichever list map iteration reaches last.
func TestHelpListsHoldEachTopicOnceAndOnlySkillsAsSkills(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)

	origKeywords := loadedKeywords
	defer func() { loadedKeywords = origKeywords }()

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadAliases()
	require.NotEmpty(t, loadedKeywords.Help["skill"], "fixture: the dogmud help map must have a skill group")

	isSkill := map[string]bool{}
	for _, s := range skills.GetAllSkillNames() {
		isSkill[string(s)] = true
	}

	seen := map[string]string{}
	var problems []string
	for group, categories := range loadedKeywords.Help {
		for category, topics := range categories {
			for _, topic := range topics {
				topic = strings.ToLower(topic)
				where := group + "/" + category
				if prev, dup := seen[topic]; dup {
					problems = append(problems, topic+" is listed in both "+prev+" and "+where)
				}
				seen[topic] = where
				if strings.ToLower(group) == "skill" && !isSkill[topic] {
					problems = append(problems, topic+" is listed as a skill but is not one")
				}
			}
		}
	}
	sort.Strings(problems)
	require.Empty(t, problems)
}

// #375: NPCs are talked to with talk, so the help index's quests list teaches
// talk, and ask (now only for a bonded companion) sits with the companion
// commands, never under quests (review M7).
func TestHelpQuestsListTeachesTalkNotAsk(t *testing.T) {
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)

	origKeywords := loadedKeywords
	defer func() { loadedKeywords = origKeywords }()

	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud"))
	configs.SetConfigForTest(t, cfg)

	LoadAliases()
	quests := loadedKeywords.Help["command"]["quests"]
	require.NotEmpty(t, quests, "fixture: the dogmud help map must have a command/quests list")
	require.Contains(t, quests, "talk")
	require.NotContains(t, quests, "ask")
	require.Contains(t, loadedKeywords.Help["command"]["character"], "ask")
}
