package questengine

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/textutil"
	"gopkg.in/yaml.v2"
)

// Package-level engine instance
var globalEngine *Engine

// GetEngine returns the global quest engine instance.
func GetEngine() *Engine {
	if globalEngine == nil {
		globalEngine = NewEngine()
	}
	return globalEngine
}

// LoadDataFiles builds the trigger index from the quest definitions already
// loaded by quests.LoadDataFiles (main.go boot order guarantees it ran first).
// Since the 5c-pre unification this package performs NO file I/O — the quest
// file parse (and its Validate()-at-load enforcement, plus flag registration)
// lives entirely in internal/quests.
func LoadDataFiles() {
	start := time.Now()

	globalEngine = NewEngine()
	all := quests.GetAllQuests()
	for i := range all {
		globalEngine.RegisterQuest(&all[i])
	}

	ValidateAllFlags()
	ValidateAllRoomText()

	mudlog.Info("questengine.LoadDataFiles()", "loadedCount", len(globalEngine.quests), "Time Taken", time.Since(start))
}

// dialogueEntry pairs a parsed DialogueFile with the mob ID from the filename.
type dialogueEntry struct {
	mobId int
	df    *dialogue.DialogueFile
}

// loadAllDialogueFiles walks the dialogue directory and parses every YAML file.
//
// It returns the files it could NOT read alongside the ones it could. Skipping
// them silently is what made the flag validator dishonest: it could report
// success without having inspected files that may contain undeclared or
// misspelled flag keys, which is precisely what it exists to catch (review
// finding 16). A validator that quietly narrows its own input is worse than no
// validator, because it produces confidence rather than a warning.
//
// A missing dialogue directory is still fine and yields no failures — that is
// absence, not corruption.
func loadAllDialogueFiles(basePath string) (entries []dialogueEntry, failures []string) {

	zoneDirs, err := os.ReadDir(basePath)
	if err != nil {
		// No dialogue directory is fine (e.g., tests).
		return entries, failures
	}

	for _, zoneDir := range zoneDirs {
		if !zoneDir.IsDir() {
			continue
		}
		zonePath := basePath + "/" + zoneDir.Name()
		files, err := os.ReadDir(zonePath)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: cannot list zone directory: %s", zonePath, err))
			continue
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".yaml") {
				continue
			}
			fullPath := zonePath + "/" + f.Name()
			data, err := os.ReadFile(fullPath)
			if err != nil {
				failures = append(failures, fmt.Sprintf("%s: cannot read: %s", fullPath, err))
				continue
			}
			var df dialogue.DialogueFile
			if err := yaml.Unmarshal(data, &df); err != nil {
				failures = append(failures, fmt.Sprintf("%s: cannot parse: %s", fullPath, err))
				continue
			}
			entries = append(entries, dialogueEntry{mobId: df.MobId, df: &df})
		}
	}

	return entries, failures
}

// ValidateAllFlags scans all quest engine triggers and dialogue files for quest
// flag references and panics if any reference an undeclared flag key or value.
// This runs at startup so typos are caught before they reach players.
func ValidateAllFlags() {
	var errors []string

	// Scan quest engine triggers for flag references.
	for _, q := range globalEngine.quests {
		for i, t := range q.Triggers {
			for k, v := range t.Conditions.HasFlag {
				if err := quests.ValidateFlag(k, v); err != nil {
					errors = append(errors, fmt.Sprintf("quest %d trigger %d has_flag: %s", q.QuestId, i, err))
				}
			}
			for k, v := range t.Conditions.MissingFlag {
				if err := quests.ValidateFlag(k, v); err != nil {
					errors = append(errors, fmt.Sprintf("quest %d trigger %d missing_flag: %s", q.QuestId, i, err))
				}
			}
			for j, a := range t.Actions {
				if a.SetFlag != nil {
					if err := quests.ValidateFlag(a.SetFlag.Key, a.SetFlag.Value); err != nil {
						errors = append(errors, fmt.Sprintf("quest %d trigger %d action %d set_flag: %s", q.QuestId, i, j, err))
					}
				}
			}
		}
	}

	// Scan dialogue files for flag references.
	dialoguePath := configs.GetFilePathsConfig().DataFiles.String() + `/dialogue`
	dialogueFiles, dialogueFailures := loadAllDialogueFiles(dialoguePath)
	// A file we could not inspect is a validation failure, not a file to skip.
	// Undeclared flag references are a startup panic by design, so letting an
	// unreadable file through would hollow out that guarantee.
	for _, f := range dialogueFailures {
		errors = append(errors, fmt.Sprintf("dialogue file could not be validated: %s", f))
	}
	for _, entry := range dialogueFiles {
		refs, sets := dialogue.CollectFlagReferences(entry.df)
		for _, ref := range refs {
			if err := quests.ValidateFlag(ref.Key, ref.Value); err != nil {
				errors = append(errors, fmt.Sprintf("dialogue mob %d %s: %s", entry.mobId, ref.Source, err))
			}
		}
		for _, set := range sets {
			if err := quests.ValidateFlag(set.Key, set.Value); err != nil {
				errors = append(errors, fmt.Sprintf("dialogue mob %d %s setsQuestFlag: %s", entry.mobId, set.Source, err))
			}
		}
	}

	if len(errors) > 0 {
		panic(fmt.Sprintf("Quest flag validation failed (%d errors):\n  %s", len(errors), strings.Join(errors, "\n  ")))
	}

	mudlog.Info("ValidateAllFlags()", "msg", "all quest flag references validated")
}

// ValidateAllRoomText enforces the one convention quest room_text uses, and
// panics at startup on a line that breaks it, the same way ValidateAllFlags
// does for flag references.
//
// Every quest room_text is something the room watches the triggering player
// do, so it must name them with {source}. Until 2026-09-11 twenty-one lines
// were written as subjectless fragments, so the room read "unlocks the
// strongbox" with no one doing it, and one used {source} that nothing filled
// in. A line that fails here would otherwise fail silently in play.
func ValidateAllRoomText() {
	var problems []string
	for _, q := range globalEngine.quests {
		for i, t := range q.Triggers {
			for j, a := range t.Actions {
				if a.RoomText == "" {
					continue
				}
				prefix := fmt.Sprintf("quest %d trigger %d action %d room_text", q.QuestId, i, j)
				for _, p := range roomTextProblems(a.RoomText) {
					problems = append(problems, prefix+": "+p)
				}
			}
		}
	}

	if len(problems) > 0 {
		// The engine stores quests in a map; sort so the report is stable.
		sort.Strings(problems)
		panic(fmt.Sprintf("Quest room_text validation failed (%d errors):\n  %s", len(problems), strings.Join(problems, "\n  ")))
	}

	mudlog.Info("ValidateAllRoomText()", "msg", "all quest room_text validated")
}

// roomTextProblems returns every way text breaks the quest room_text
// convention, or nil. Split out so the rules are testable without an engine.
//
// It is stricter than buffs and spells, which only WARN on an unknown token.
// A new rule with no shipped violations can fail at boot at no cost; upgrading
// buffs and spells would change what is allowed to boot, so that is filed.
func roomTextProblems(text string) []string {
	var problems []string
	if !strings.Contains(text, "{source}") {
		problems = append(problems, "must name the acting player with {source}; the room is watching them act")
	}
	for _, token := range []string{"{target}", "{target_plain}"} {
		if strings.Contains(text, token) {
			problems = append(problems, token+" is not available: a quest has no target, so it would render empty")
		}
	}
	if strings.Contains(text, "{source_plain}") {
		problems = append(problems, "{source_plain} is an untagged name that cannot be anonymized in the dark; use {source}")
	}
	problems = append(problems, textutil.ValidateTokens(text)...)
	return problems
}
