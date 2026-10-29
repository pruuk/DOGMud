package main

import (
	"slices"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
)

// The offer index door names the quests an NPC offers that cannot be
// abandoned, by reason: every offer hides on another quest's token, or a
// trigger or behaviour tree grants the quest's start by itself (owner ruling
// 6). It hands the second set to usercommands (#375 phase 5).
func TestOfferIndexDoor_NamesUnabandonableOffers(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	t.Cleanup(dialogue.SetOfferIndexForTest(nil))
	t.Cleanup(usercommands.SetAutoStartedForTest(nil))
	offer := func(key, tok string, excluded ...string) dialogue.Topic {
		return dialogue.Topic{Key: key, Kind: dialogue.KindOffer, Gates: dialogue.Gates{QuestExcluded: excluded},
			Effects: dialogue.Effects{GrantsQuest: tok}}
	}
	hides, auto := offerIndexDoorFrom(questloop.World{
		Quests: []*quests.Quest{{QuestId: 30, Triggers: []quests.TriggerDef{
			{Event: "room_enter", Room: 5200, Actions: []quests.ActionDef{{Grant: "30-start"}}},
		}}},
		Dialogue: []*dialogue.DialogueFile{{MobId: 1, Topics: []dialogue.Topic{
			offer("a", "88-start", "88-start", "31-start"),
			offer("b", "42-start", "42-start", "42-end"),
			offer("c", "30-start", "30-start"),
		}}},
		External: []questloop.ExternalGrant{
			// Granted by a tree but offered by no NPC: not abandonable anyway, not logged.
			{Where: "room 1 tree grant_quest 50-start", Token: "50-start"},
		},
	})
	if !slices.Equal(hides, []int{88}) || !slices.Equal(auto, []int{30}) {
		t.Fatalf("logged hides %v, auto %v; want [88], [30]", hides, auto)
	}
	if !dialogue.Resumable(42) || dialogue.Resumable(88) || !dialogue.Resumable(30) {
		t.Fatalf("index not built: 42 %v, 88 %v, 30 %v", dialogue.Resumable(42), dialogue.Resumable(88), dialogue.Resumable(30))
	}
	c := characters.New()
	c.QuestProgress = map[int]string{30: "start", 42: "start"}
	if usercommands.Abandonable(c, 30) || !usercommands.Abandonable(c, 42) {
		t.Fatalf("abandonable: 30 %v, 42 %v; want false, true",
			usercommands.Abandonable(c, 30), usercommands.Abandonable(c, 42))
	}
}

// On shipped data, quest 30 (The Awakening) is not abandonable: room_enter
// 5200 and the antechamber room tree grant 30-start by themselves. Quest 42
// (The Hunt) still is. The test logs every quest the rule takes out of
// abandonable (offered with a resuming offer, but auto-started), so the
// owner sees the full effect.
func TestSmoke_AutoStartedQuestsNotAbandonable(t *testing.T) {
	loadShippedQuestWorld(t)
	t.Cleanup(dialogue.SetOfferIndexForTest(nil))
	t.Cleanup(usercommands.SetAutoStartedForTest(nil))
	offerIndexDoor()

	c := characters.New()
	c.QuestProgress = map[int]string{30: "start", 42: "start"}
	if usercommands.Abandonable(c, 30) {
		t.Error("quest 30 is abandonable; its start is granted on entering room 5200")
	}
	if !usercommands.Abandonable(c, 42) {
		t.Error("quest 42 is not abandonable; it has a clean offer and no automatic start")
	}

	w := questworldForOfferDoor()
	auto := questloop.AutoStarts(w)
	ids := make([]int, 0, len(auto))
	for id := range auto {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	t.Logf("%d quests have an automatic start: %v", len(ids), ids)
	for _, id := range ids {
		if dialogue.Resumable(id) {
			name := ""
			if q := quests.GetQuestById(id); q != nil {
				name = q.Name
			}
			t.Logf("ruling 6 removes quest %d (%s): %v", id, name, auto[id])
		}
	}
}
