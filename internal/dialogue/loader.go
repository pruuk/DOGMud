package dialogue

import (
	"fmt"
	"os"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

var (
	dialogueCache = map[string]*DialogueFile{}
	// nilSentinel records mob/zone pairs that have NO dialogue file, so the
	// common "this mob never talks" case does not stat the disk on every
	// interaction.
	//
	// It records CONFIRMED absence only. A read or parse failure must never
	// land here: those are transient or fixable, and caching them meant a
	// corrected dialogue file could not load until the process restarted
	// (review finding 14). Fix a typo in an NPC's YAML and the NPC stayed mute
	// until a reboot.
	nilSentinel = map[string]bool{}
	// loadFailureLogged throttles the error log for a broken file. Without
	// caching the failure, every ask/talk re-reads it, and a busy room would
	// otherwise repeat the same parse error on every interaction. Keyed by
	// mob/zone, valued by the last error text, so a CHANGED error still logs
	// (the builder saved a different mistake) and a successful load clears it.
	loadFailureLogged = map[string]string{}
)

// logLoadFailureOnce reports a dialogue load failure at most once per distinct
// error per mob/zone. Returns nothing: the caller always returns nil.
func logLoadFailureOnce(key, msg string) {
	if prev, ok := loadFailureLogged[key]; ok && prev == msg {
		return
	}
	loadFailureLogged[key] = msg
	mudlog.Error("dialogue.Load()", "error", msg)
}

// Load lazily reads and caches a mob's dialogue file.
// Returns nil if no file exists for this mob/zone combination.
func Load(mobId int, zone string) *DialogueFile {
	key := fmt.Sprintf("%d:%s", mobId, zone)

	if nilSentinel[key] {
		return nil
	}

	if df, ok := dialogueCache[key]; ok {
		return df
	}

	sanitizedZone := zoneNameSanitize(zone)
	dataFiles := string(configs.GetFilePathsConfig().DataFiles)
	path := util.FilePath(dataFiles + `/dialogue/` + sanitizedZone + `/` + fmt.Sprintf("%d", mobId) + `.yaml`)

	if _, err := os.Stat(path); err != nil {
		nilSentinel[key] = true
		return nil
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		// NOT cached: the file exists, it just could not be read this time.
		// Caching would make the failure permanent for the process lifetime.
		logLoadFailureOnce(key, "Problem reading dialogue file "+path+": "+err.Error())
		return nil
	}

	var df DialogueFile
	if err := yaml.Unmarshal(bytes, &df); err != nil {
		// NOT cached, for the same reason: this is the case an author hits
		// while editing, and they must be able to fix the file and retry
		// without restarting the server.
		logLoadFailureOnce(key, "Problem unmarshalling dialogue file "+path+": "+err.Error())
		return nil
	}

	validateQuestExclusions(&df, path)

	delete(loadFailureLogged, key) // a good load clears the throttle
	dialogueCache[key] = &df
	return &df
}

// zoneNameSanitize converts a zone display name to the lowercase
// underscore form used in file paths (e.g. "Sanctum Basin" → "sanctum_basin").
func zoneNameSanitize(zone string) string {
	if zone == "" {
		return ""
	}
	zone = strings.ReplaceAll(zone, " ", "_")
	return strings.ToLower(zone)
}

// validateQuestExclusions warns if any grantsQuest node is missing the
// quest's end token from questExcluded. Without the end-token exclusion,
// players who have completed a quest can get it re-offered.
func validateQuestExclusions(df *DialogueFile, path string) {
	checkExclusions := func(label string, grantsQuest string, questExcluded []string) {
		if grantsQuest == "" {
			return
		}
		// Extract quest ID prefix: "10-start" → "10", "14-evidence" → "14"
		idx := strings.Index(grantsQuest, "-")
		if idx < 0 {
			return
		}
		prefix := grantsQuest[:idx]
		endToken := prefix + "-end"

		// If it already grants the end token, no exclusion needed
		if grantsQuest == endToken {
			return
		}

		for _, ex := range questExcluded {
			if ex == endToken {
				return
			}
		}
		mudlog.Warn("dialogue.validateQuestExclusions()", "warning",
			fmt.Sprintf("%s: %s grants %q but questExcluded is missing %q — completed quest can be re-offered",
				path, label, grantsQuest, endToken))
	}

	// Check patterns
	for i, p := range df.Patterns {
		checkExclusions(fmt.Sprintf("pattern[%d]", i), p.GrantsQuest, p.QuestExcluded)
	}

	// Check tree nodes
	if df.Tree != nil {
		for _, n := range df.Tree.Nodes {
			checkExclusions(fmt.Sprintf("node[%s]", n.Id), n.GrantsQuest, n.QuestExcluded)
		}
	}
}
