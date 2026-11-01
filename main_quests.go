package main

import (
	"errors"
	"fmt"
	"slices"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
)

// offerIndexDoor builds what `quests abandon` reads (#375 phase 5): the
// dialogue offer index (which NPCs offer each quest; dialogue loads lazily,
// so this reads every file once) and the quests whose -start a trigger or
// behaviour tree grants by itself (owner ruling 6).
func offerIndexDoor() {
	offerIndexDoorFrom(questworldForOfferDoor())
}

// questworldForOfferDoor is the quest loop world without reference checks:
// the door reads only its quests, dialogue and external grants.
func questworldForOfferDoor() questloop.World {
	return questworld.RegistryWith(quests.QuestValidators{})
}

// offerIndexDoorFrom builds the offer index from w's dialogue, hands the
// automatically started quests (questloop.AutoStarts) to usercommands, and
// logs, as information, the quests an NPC offers that cannot be abandoned,
// by reason: no offer can show again while shelved (an offer hidden by
// another quest's token or flag, like quest 88's), or a quest trigger or
// behaviour tree grants the quest's start by itself, which would take it
// back up without the player choosing to (owner ruling 6). That is
// by design, not a fault. It returns the quest ids it logged for each
// reason; a quest may appear in both.
func offerIndexDoorFrom(w questloop.World) (hides, auto []int) {
	dialogue.BuildOfferIndex(w.Dialogue)
	autoStarts := questloop.AutoStarts(w)
	all := make([]int, 0, len(autoStarts))
	for id := range autoStarts {
		all = append(all, id)
		if len(dialogue.Offerers(id)) > 0 {
			auto = append(auto, id)
		}
	}
	usercommands.SetAutoStarted(all)
	slices.Sort(auto)
	hides = dialogue.Unresumable()
	if len(hides) > 0 || len(auto) > 0 {
		mudlog.Info("quests",
			"offered but not abandonable (an offer hides on another quest)", fmt.Sprint(hides),
			"offered but not abandonable (a trigger or behaviour tree grants its start)", fmt.Sprint(auto))
	}
	return hides, auto
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
