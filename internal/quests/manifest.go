package quests

import (
	"fmt"
	"os"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/manifest"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// ParseStepManifest reads quests/step-ids.txt: one "<questId>-<stepId>" per
// shipped step. Saved characters hold their current step id, so a step id
// leaves content only through a retired or renamed line (#375 phase 2).
func ParseStepManifest(data []byte) (manifest.Manifest, error) {
	return manifest.Parse("step-ids.txt", data)
}

// StepTokens lists every step of every quest as "<questId>-<stepId>".
func StepTokens(qs []Quest) []string {
	var out []string
	for _, q := range qs {
		for _, s := range q.Steps {
			out = append(out, fmt.Sprintf("%d-%s", q.QuestId, s.Id))
		}
	}
	return out
}

// StepRenames reads the shipped manifest's renames for the load migration.
// A missing or unreadable manifest renames nothing.
func StepRenames() map[string]string {
	path := util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/quests/step-ids.txt`)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	m, err := ParseStepManifest(data)
	if err != nil {
		mudlog.Warn("quests.StepRenames()", "path", path, "error", err)
		return nil
	}
	return m.Renamed
}
