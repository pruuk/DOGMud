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

// shippedStepManifest reads the shipped manifest. A missing manifest is not
// an error (ok false); an unreadable one is logged (ok false).
func shippedStepManifest(caller string) (manifest.Manifest, bool) {
	path := util.FilePath(string(configs.GetFilePathsConfig().DataFiles) + `/quests/step-ids.txt`)
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest.Manifest{}, false
	}
	m, err := ParseStepManifest(data)
	if err != nil {
		mudlog.Warn(caller, "path", path, "error", err)
		return manifest.Manifest{}, false
	}
	return m, true
}

// StepRenames reads the shipped manifest's renames for the load migration.
// A missing or unreadable manifest renames nothing.
func StepRenames() map[string]string {
	m, ok := shippedStepManifest("quests.StepRenames()")
	if !ok {
		return nil
	}
	return m.Renamed
}

// StepRetired reads the shipped manifest's retired steps, the set LoadUser
// checks saved current steps against (spec 5.5: left as is and logged). A
// missing or unreadable manifest retires nothing.
func StepRetired() map[string]bool {
	m, ok := shippedStepManifest("quests.StepRetired()")
	if !ok || len(m.Retired) == 0 {
		return nil
	}
	out := make(map[string]bool, len(m.Retired))
	for _, tok := range m.Retired {
		out[tok] = true
	}
	return out
}
