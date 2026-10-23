package main

import (
	"path/filepath"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
)

// validateShippedDialogue is the boot door of the dialogue validators: the
// shipped files against the loaded quests, items and behavior trees. It
// reports ValidateWorld problems only; the dialoguecheck walk findings
// (orphan steps, unreachable cases) are a separate, advisory tool.
func validateShippedDialogue() []dialogue.Problem {
	w, probs := dialogue.RegistryWorld(behaviorTopicKeys())
	return append(probs, dialogue.ValidateWorld(w)...)
}

// behaviorTopicKeys indexes topic_is keys by mob id from
// behaviors/<zone>/<id>-*.yaml. Only that one level is read: archetypes and
// items carry no numeric prefix and are skipped, and room behaviors live two
// levels down (behaviors/rooms/<zone>/), so they are never read as a mob.
func behaviorTopicKeys() map[int][]string {
	byMob := map[int][]string{}
	root := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "behaviors")
	paths, _ := filepath.Glob(filepath.Join(root, "*", "*.yaml"))
	for _, p := range paths {
		idText, _, _ := strings.Cut(filepath.Base(p), "-")
		id, err := strconv.Atoi(idText)
		if err != nil {
			continue
		}
		if keys, err := behaviortree.TopicKeysInFile(p); err == nil && len(keys) > 0 {
			byMob[id] = append(byMob[id], keys...)
		}
	}
	return byMob
}
