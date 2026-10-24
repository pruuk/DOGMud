package main

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// validateShippedQuests is the boot door of the quest loop validators
// (#375 phase 2): every rule over the loaded quests and dialogue, split by
// the shipped backlog. Load quests, dialogue, mobs, items, spells, recipes
// and factions first.
func validateShippedQuests() (fresh, known []questloop.Finding, stale []string, err error) {
	b, err := questworld.LoadBacklog()
	if err != nil {
		return nil, nil, nil, err
	}
	fresh, known, stale = questloop.Ratchet(questloop.Validate(questworld.Registry()), b)
	return fresh, known, stale, nil
}

// questDoor logs the backlog's known findings as warnings and refuses on a
// fresh one: a panic at boot, an error on a reload (the dialogueDoor rule).
// Stale backlog lines are CI's job (quest_loop_smoke_test.go), not boot's.
func questDoor(isReload bool) error {
	fresh, known, _, err := validateShippedQuests()
	if err != nil {
		mudlog.Error("quests", "backlog", err.Error())
		if !isReload {
			panic(err.Error())
		}
		return err
	}
	if len(known) > 0 {
		mudlog.Warn("quests", "loop backlog findings", len(known), "file", questworld.BacklogPath())
	}
	if len(fresh) == 0 {
		return nil
	}
	for _, f := range fresh {
		mudlog.Error("quests", "loop", f.String())
	}
	msg := fmt.Sprintf("%d quest loop problems not in the backlog; see the log above", len(fresh))
	if !isReload {
		panic(msg)
	}
	return errors.New(msg)
}
