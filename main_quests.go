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

// destinationDoor refuses on any quest step whose minimap destination is
// unknown or mistagged (#375 phase 4; a refusal since the closing step): a
// panic at boot, an error on a reload. It logs the inferred count.
func destinationDoor(isReload bool) error {
	return destinationDoorFrom(isReload, func() ([]questloop.Finding, []string) {
		all := quests.GetAllQuests()
		qs := make([]*quests.Quest, 0, len(all))
		for i := range all {
			qs = append(qs, &all[i])
		}
		return questengine.Destinations(qs)
	})
}

// destinationDoorFrom is destinationDoor over any source, for tests.
func destinationDoorFrom(isReload bool, src func() ([]questloop.Finding, []string)) error {
	fs, inferred := src()
	mudlog.Info("quests", "inferred destinations", len(inferred))
	return refuseFindings(isReload, "quest destination", fs)
}

// questDoor is the boot door of the quest loop validators (#375 phase 2):
// every rule over the loaded quests and dialogue. Any finding refuses (the
// backlog that once parked known findings is retired). Load quests,
// dialogue, mobs, items, spells, recipes and factions first.
//
// Boot runs Validate only, never the walkthrough (questengine.Walk), which
// is the CI sweep's job (quest_loop_smoke_test.go).
func questDoor(isReload bool) error {
	return questDoorFrom(isReload, func() []questloop.Finding {
		return questloop.Validate(questworld.Registry())
	})
}

// questDoorFrom is questDoor over any finding source, so its refusals are
// testable without the shipped world.
func questDoorFrom(isReload bool, src func() []questloop.Finding) error {
	return refuseFindings(isReload, "quest loop", src())
}

// refuseFindings logs each finding and refuses: a panic at boot, an error
// on a reload (the dialogueDoor rule). No findings, no refusal.
func refuseFindings(isReload bool, what string, fs []questloop.Finding) error {
	if len(fs) == 0 {
		return nil
	}
	for _, f := range fs {
		mudlog.Error("quests", what, f.String())
	}
	msg := fmt.Sprintf("%d %s problems; see the log above", len(fs), what)
	if !isReload {
		panic(msg)
	}
	return errors.New(msg)
}
