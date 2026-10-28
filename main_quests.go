package main

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// offerIndexDoor builds the dialogue offer index (#375 phase 5): which NPCs
// offer each quest, for `quests abandon`. Dialogue loads lazily, so this reads
// every file once.
func offerIndexDoor() {
	files, _ := dialogue.LoadAllFiles()
	dialogue.BuildOfferIndex(files)
}

// destinationDoor logs every quest step whose minimap destination is unknown
// or mistagged (#375 phase 4) as a warning, and the inferred count. It never
// refuses: the CI sweep ratchets these findings against the backlog.
func destinationDoor() int {
	return destinationDoorFrom(func() ([]questloop.Finding, []string) {
		all := quests.GetAllQuests()
		qs := make([]*quests.Quest, 0, len(all))
		for i := range all {
			qs = append(qs, &all[i])
		}
		return questengine.Destinations(qs)
	})
}

// destinationDoorFrom is destinationDoor over any source, for tests. It
// returns how many findings it logged.
func destinationDoorFrom(src func() ([]questloop.Finding, []string)) int {
	fs, inferred := src()
	for _, f := range fs {
		mudlog.Warn("quests", "destination", f.String())
	}
	mudlog.Info("quests", "inferred destinations", len(inferred))
	return len(fs)
}

// validateShippedQuests is the boot door of the quest loop validators
// (#375 phase 2): every rule over the loaded quests and dialogue, split by
// the shipped backlog. Load quests, dialogue, mobs, items, spells, recipes
// and factions first.
//
// Boot runs Validate only, never the walkthrough (questengine.Walk). The
// backlog also holds the walk's lines, which no boot finding matches; boot
// ignores stale lines, so they cannot refuse it. The CI sweep
// (quest_loop_smoke_test.go) ratchets Validate plus Walk.
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
	return questDoorFrom(isReload, validateShippedQuests)
}

// questDoorFrom is questDoor over any finding source, so its refusals are
// testable without the shipped world.
func questDoorFrom(isReload bool, src func() (fresh, known []questloop.Finding, stale []string, err error)) error {
	fresh, known, _, err := src()
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
