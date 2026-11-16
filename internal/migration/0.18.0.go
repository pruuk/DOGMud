package migration

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v2"
)

// Description:
// Messaging M3 item 7 renamed the periodic gameplay broadcast from hints to
// tips, including the player's on/off setting: `set tips` now reads and writes
// configoptions.tips. A save still holding configoptions.hints would silently
// turn tips back on for a player who had turned them off, so the value moves.
//
// The setting lives on the USER record, not the character, so there is no
// alts trap. Idempotent without a marker: a save with no hints option is not
// rewritten, so a second run changes nothing. A save holding both hints and
// tips is an error, so Run restores the datafiles backup instead of guessing
// which the player meant. A save that fails to parse is logged and skipped,
// as in 0.16.0: it would not load as a player either.
func migrate_TipsConfigOption(dryRun bool) error {
	c := configs.GetConfig()
	return renameTipsConfigOptionInDir(filepath.Join(string(c.FilePaths.DataFiles), "users"), dryRun)
}

// renameTipsConfigOptionInDir is the testable core.
func renameTipsConfigOptionInDir(usersDir string, dryRun bool) error {
	mode := "APPLY"
	if dryRun {
		mode = "DRY-RUN"
	}

	matches, err := filepath.Glob(filepath.Join(usersDir, "*.yaml"))
	if err != nil {
		return err
	}

	mudlog.Info("Migration 0.18.0", "message", "Renaming the hints setting to tips", "mode", mode, "files", len(matches))

	renamed := 0
	for _, path := range matches {
		if !isUserSaveFile(path) {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", path, err)
		}

		var userMap map[string]any
		if err := yaml.Unmarshal(raw, &userMap); err != nil {
			mudlog.Warn("Migration 0.18.0", "file", filepath.Base(path), "error", err)
			continue
		}

		opts, ok := userMap["configoptions"].(map[any]any)
		if !ok {
			continue
		}
		value, hasHints := opts["hints"]
		if !hasHints {
			continue
		}
		if _, hasTips := opts["tips"]; hasTips {
			return fmt.Errorf("%s: configoptions carries both hints and tips; refusing to guess which the player meant", path)
		}

		renamed++
		if dryRun {
			continue
		}

		opts["tips"] = value
		delete(opts, "hints")

		out, err := yaml.Marshal(userMap)
		if err != nil {
			return fmt.Errorf("failed to marshal %s: %w", path, err)
		}
		// util.Save is safe by default (temp file, fsync, rename): a crash
		// mid-write cannot truncate the file it is replacing, unlike os.WriteFile.
		if err := util.Save(path, out); err != nil {
			return fmt.Errorf("failed to write %s: %w", path, err)
		}
	}

	mudlog.Info("Migration 0.18.0", "message", "hints setting renamed", "saves", renamed, "mode", mode)
	return nil
}
