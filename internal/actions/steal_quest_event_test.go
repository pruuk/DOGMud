package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// Steal and plant raise a `command` quest event aimed at what they were
// aimed at: a mob, or (since #375 phase 3b slice 2) another player. These
// tests register a one-trigger probe quest and look for its grant on the
// thief's user record.

const (
	stealEventProbeToken = "98-start" // a fresh character may only start a quest
	stealEventProbeQuest = 98
	stealEventThiefId    = 1001 // newStealPlayerActor's user id
	stealEventPlanterId  = 1002 // newPlantPlayerActor's user id
	stealEventVictimId   = 2501
)

// stealEventProbe seeds the thief and victim users and an engine whose only
// trigger is a `command` trigger for the given command and target kind. It
// returns the thief's user record and a cleanup.
func stealEventProbe(t *testing.T, thiefId int, command, targetKind string) (*users.UserRecord, func()) {
	t.Helper()
	thief := users.NewTestUser(thiefId, "thief", "Thief", 0)
	victim := users.NewTestUser(stealEventVictimId, "victim", "Victim", 0)
	victim.Character.Stats.Perception.ValueAdj = 1 // nearly blind
	undoUsers := users.SeedUsersForTest(map[int]*users.UserRecord{
		thiefId:            thief,
		stealEventVictimId: victim,
	})
	undoEngine := questengine.ResetEngineForTest()
	questengine.GetEngine().RegisterQuest(&questengine.QuestDef{
		QuestId: stealEventProbeQuest,
		Name:    "Steal Event Probe",
		Triggers: []quests.TriggerDef{{
			Event:   "command",
			Command: command,
			Target:  targetKind,
			Actions: []quests.ActionDef{{Grant: stealEventProbeToken}},
		}},
	})
	return thief, func() {
		undoEngine()
		undoUsers()
	}
}

func TestSteal_OnPlayerRaisesPlayerTargetEvent(t *testing.T) {
	thief, done := stealEventProbe(t, stealEventThiefId, "steal", quests.TargetPlayer)
	defer done()

	actor := newStealPlayerActor(200, 8)
	Steal(actor, StealOptions{TargetUserId: stealEventVictimId})

	assert.True(t, thief.Character.HasQuest(stealEventProbeToken),
		"a steal aimed at a player must raise a command event of kind player")
}

func TestSteal_OnMobDoesNotFirePlayerTrigger(t *testing.T) {
	thief, done := stealEventProbe(t, stealEventThiefId, "steal", quests.TargetPlayer)
	defer done()

	mob := newStealTestMob(testMobInstId, 50, 1)
	mobs.SetInstanceForTest(testMobInstId, mob)
	defer mobs.SetInstanceForTest(testMobInstId, nil)

	actor := newStealPlayerActor(200, 8)
	Steal(actor, StealOptions{TargetMobInstanceId: testMobInstId})

	assert.False(t, thief.Character.HasQuest(stealEventProbeToken),
		"a steal aimed at a mob must not satisfy a target: player trigger")
}

func TestSteal_OnMobFiresMobTrigger(t *testing.T) {
	thief, done := stealEventProbe(t, stealEventThiefId, "steal", quests.TargetMob)
	defer done()

	mob := newStealTestMob(testMobInstId, 50, 1)
	mobs.SetInstanceForTest(testMobInstId, mob)
	defer mobs.SetInstanceForTest(testMobInstId, nil)

	actor := newStealPlayerActor(200, 8)
	Steal(actor, StealOptions{TargetMobInstanceId: testMobInstId})

	assert.True(t, thief.Character.HasQuest(stealEventProbeToken),
		"a steal aimed at a mob must raise a command event of kind mob")
}

func TestPlant_OnPlayerRaisesPlayerTargetEvent(t *testing.T) {
	thief, done := stealEventProbe(t, stealEventPlanterId, "plant", quests.TargetPlayer)
	defer done()

	actor := newPlantPlayerActor(200, 8)
	seedPlantItem(actor)
	delete(actor.char.Cooldowns, skills.Skullduggery.String("steal"))
	Plant(actor, PlantOptions{TargetUserId: stealEventVictimId, ItemNoun: "!1"})

	assert.True(t, thief.Character.HasQuest(stealEventProbeToken),
		"a plant aimed at a player must raise a command event of kind player")
}

func TestPlant_OnMobDoesNotFirePlayerTrigger(t *testing.T) {
	thief, done := stealEventProbe(t, stealEventPlanterId, "plant", quests.TargetPlayer)
	defer done()

	mob := newPlantTestMob(testPlantMobInstId, 1)
	mobs.SetInstanceForTest(testPlantMobInstId, mob)
	defer mobs.SetInstanceForTest(testPlantMobInstId, nil)

	actor := newPlantPlayerActor(200, 8)
	seedPlantItem(actor)
	Plant(actor, PlantOptions{TargetMobInstanceId: testPlantMobInstId, ItemNoun: "!1"})

	assert.False(t, thief.Character.HasQuest(stealEventProbeToken),
		"a plant aimed at a mob must not satisfy a target: player trigger")
}

func TestPlant_OnMobFiresMobTrigger(t *testing.T) {
	thief, done := stealEventProbe(t, stealEventPlanterId, "plant", quests.TargetMob)
	defer done()

	mob := newPlantTestMob(testPlantMobInstId, 1)
	mobs.SetInstanceForTest(testPlantMobInstId, mob)
	defer mobs.SetInstanceForTest(testPlantMobInstId, nil)

	actor := newPlantPlayerActor(200, 8)
	seedPlantItem(actor)
	Plant(actor, PlantOptions{TargetMobInstanceId: testPlantMobInstId, ItemNoun: "!1"})

	assert.True(t, thief.Character.HasQuest(stealEventProbeToken),
		"a plant aimed at a mob must raise a command event of kind mob")
}
