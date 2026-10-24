package gmcp

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// The builder's room and item deletes run the boot doors' own rules (the
// quest loop's refs rule, the dialogue item check) with the id gone, and
// refuse what the delete would add. The fixture is the clean loop world.

func deleteFixture(mod func(q10 *quests.Quest)) questloop.World {
	q10, q11 := loopFixtureQuests()
	if mod != nil {
		mod(q10)
	}
	v := newFakeQuestWorld().deps().validators
	return questloop.World{Quests: []*quests.Quest{q10, q11}, Dialogue: loopFixtureDialogue(), Refs: &v}
}

func TestQuestRefsWithoutRoom_NamesEveryReferencingField(t *testing.T) {
	w := deleteFixture(func(q *quests.Quest) {
		q.Triggers[0].Conditions.InRoom = 9001
		q.Steps[1].MapTarget = 9001
		q.Rewards.RoomId = 9001
	})
	got := questRefsWithoutRoom(w, questloop.Backlog{}, 9001)
	joined := strings.Join(got, "\n")
	for _, want := range []string{"in_room", "map_target", "rewards roomid"} {
		if !strings.Contains(joined, want) {
			t.Errorf("refusal must name %s, got %v", want, got)
		}
	}
	for _, line := range got {
		if !strings.Contains(line, "quest 90010") || !strings.Contains(line, "room 9001") {
			t.Errorf("each line must name the quest and the room, got %q", line)
		}
	}
	if other := questRefsWithoutRoom(w, questloop.Backlog{}, 9002); len(other) != 0 {
		t.Errorf("an unreferenced room must not refuse, got %v", other)
	}
}

// A finding the world already has is not the delete's doing.
func TestQuestRefsWithoutRoom_IgnoresWhatTheWorldAlreadyHas(t *testing.T) {
	w := deleteFixture(func(q *quests.Quest) { q.Triggers[0].Conditions.InRoom = 9001 })
	base := *w.Refs
	base.RoomExists = func(id int) bool { return id != 9001 }
	w.Refs = &base
	if got := questRefsWithoutRoom(w, questloop.Backlog{}, 9001); len(got) != 0 {
		t.Errorf("a room already missing adds nothing, got %v", got)
	}
}

func TestQuestRefsWithoutItem_TriggersActionsAndRewards(t *testing.T) {
	w := deleteFixture(func(q *quests.Quest) {
		q.Triggers[0].Conditions.HasItem = 40163
		q.Triggers[0].Actions = append(q.Triggers[0].Actions, quests.ActionDef{GiveItem: 40163})
		q.Rewards.ItemInfo = "40163:2"
	})
	joined := strings.Join(questRefsWithoutItem(w, questloop.Backlog{}, 40163), "\n")
	for _, want := range []string{"has_item", "give_item", "item_info"} {
		if !strings.Contains(joined, want) {
			t.Errorf("refusal must name %s, got %q", want, joined)
		}
	}
}

func TestDialogueRefsWithoutItem(t *testing.T) {
	files := loopFixtureDialogue()
	files[0].Topics[0].RequiresItem = 40163
	w := dialogue.World{Files: files, ItemExists: func(int) bool { return true }}
	got := dialogueRefsWithoutItem(w, 40163)
	if len(got) != 1 || !strings.Contains(got[0], "mob 9112") || !strings.Contains(got[0], "item 40163") {
		t.Fatalf("the gate must refuse and name the mob and item, got %v", got)
	}
	if other := dialogueRefsWithoutItem(w, 40164); len(other) != 0 {
		t.Errorf("an unreferenced item must not refuse, got %v", other)
	}
}

func TestBuildRoomDelete_RefusedWhileAQuestNamesIt(t *testing.T) {
	w := newFakeWorld()
	w.put(&rooms.Room{RoomId: 1, Zone: "Over", Exits: map[string]exit.RoomExit{"north": {RoomId: 2}}})
	w.put(&rooms.Room{RoomId: 2, Zone: "Over", Y: -1})
	d := w.deps()
	d.questRefs = func(id int) []string {
		return []string{"quest 90010 refs: step firstkill map_target: room 2 does not exist"}
	}
	res := buildRoomDelete(d, 2)
	if res.Ok {
		t.Fatal("a room a quest names must not delete")
	}
	if !strings.Contains(res.Error, "quest 90010 refs: step firstkill map_target") {
		t.Errorf("the refusal must name the quest and where, got %q", res.Error)
	}
	if len(w.deleted) != 0 || len(w.saved) != 0 {
		t.Errorf("a refused delete must change nothing (deleted %v, saved %d)", w.deleted, len(w.saved))
	}
}

func TestScanItemReferencesWith_QuestAndDialogueRefs(t *testing.T) {
	refs := scanItemReferencesWith(40163, refIterators{
		questRefs: func(id int) []itemRef {
			return []itemRef{{Kind: "quest", Id: "quest 90010 refs: trigger mob_death mob 9110 has_item: item 40163 does not exist"}}
		},
	})
	if len(refs) != 1 || refs[0].Kind != "quest" {
		t.Fatalf("quest refs must block the delete, got %+v", refs)
	}
}
