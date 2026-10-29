package users

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var (
	ErrBothFilesExist = errors.New("could not migrate due to both file formats existing")
)

//
// Handles user file migration
//

func DoUserMigrations() {

	DoFilenameMigrationV1()
	DoQuestStepMigration()

}

// DoQuestStepMigration checks all players for quest progress on steps that
// no longer exist (removed during quest engine port). Players stuck on
// deleted steps are bumped to "start" so they can redo the quest cleanly.
// The shelf (#375 phase 5) is migrated the same way.
func DoQuestStepMigration() {
	migrated := 0

	SearchOfflineUsers(func(u *UserRecord) bool {
		if migrateQuestSteps(u) {
			migrated++
			SaveUser(u)
		}
		return true
	})

	if migrated > 0 {
		mudlog.Info("DoQuestStepMigration", "info",
			"Quest step migration complete", "usersUpdated", migrated)
	}
}

// migrateQuestSteps runs the step migration over one user's QuestProgress
// and ShelvedQuests, and reports whether anything changed.
func migrateQuestSteps(u *UserRecord) bool {
	changed := migrateQuestStepMap(u.Username, `progress`, u.Character.QuestProgress)
	if migrateQuestStepMap(u.Username, `shelf`, u.Character.ShelvedQuests) {
		changed = true
	}
	if len(u.Character.ShelvedQuests) == 0 {
		u.Character.ShelvedQuests = nil
	}
	return changed
}

// migrateQuestStepMap drops a deleted quest from progress and resets a
// removed step to "start". where names the map in the log.
func migrateQuestStepMap(username, where string, progress map[int]string) bool {
	changed := false
	for questId, currentStep := range progress {
		// GetQuestById, not GetQuest: GetQuest is nil for a live quest on an
		// unknown step, which would drop the progress instead of resetting it.
		q := quests.GetQuestById(questId)
		if q == nil {
			// Quest itself was deleted (e.g., Q17): remove progress
			delete(progress, questId)
			mudlog.Info("DoQuestStepMigration", "info",
				"Removed deleted quest progress",
				"user", username, "map", where, "questId", questId, "step", currentStep)
			changed = true
			continue
		}

		// Check if the step still exists in the quest definition
		stepExists := false
		for _, step := range q.Steps {
			if step.Id == currentStep {
				stepExists = true
				break
			}
		}

		if !stepExists {
			// Step was removed: reset to "start"
			progress[questId] = "start"
			mudlog.Info("DoQuestStepMigration", "info",
				"Migrated removed quest step to start",
				"user", username, "map", where, "questId", questId,
				"oldStep", currentStep, "newStep", "start")
			changed = true
		}
	}
	return changed
}

func DoFilenameMigrationV1() error {

	var errorResult error = nil

	SearchOfflineUsers(func(u *UserRecord) bool {

		oldUserPath := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/`, `users`, `/`, strings.ToLower(u.Username)+`.yaml`)
		newUserPath := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/`, `users`, `/`, strconv.Itoa(u.UserId)+`.yaml`)

		_, err := os.Stat(oldUserPath)
		oldUserPathExists := err == nil

		_, err = os.Stat(newUserPath)
		newUserPathExists := err == nil

		if oldUserPathExists && newUserPathExists {
			errorResult = ErrBothFilesExist
			return false
		}

		if oldUserPathExists {
			err := os.Rename(oldUserPath, newUserPath)
			if err != nil {
				errorResult = err
				return false
			}
		}

		oldAltsFilePath := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/users/`, strings.ToLower(u.Username)+`-alts.yaml`)
		newAltsFilePath := util.FilePath(string(configs.GetFilePathsConfig().DataFiles), `/users/`, strconv.Itoa(u.UserId)+`.alts.yaml`)

		_, err = os.Stat(oldAltsFilePath)
		oldAltsPathExists := err == nil

		_, err = os.Stat(newAltsFilePath)
		newAltsPathExists := err == nil

		if oldAltsPathExists && newAltsPathExists {
			errorResult = ErrBothFilesExist
			return false
		}

		if oldAltsPathExists {
			err := os.Rename(oldAltsFilePath, newAltsFilePath)
			if err != nil {
				errorResult = err
				return false
			}
		}

		return true
	})

	return errorResult
}
