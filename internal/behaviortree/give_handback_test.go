package behaviortree

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func TestHandBackSafe_OnlySpeechAndReturn(t *testing.T) {
	safe := `tree:
  type: selector
  children:
    - type: sequence
      event: player_give
      children:
        - type: condition
          check: item_matches
          item_id: 1
        - type: decorator
          mod: cooldown
          rounds: 2
          child:
            type: action
            do: say
            text: "No."
        - type: action
          do: return_item
`
	node, err := LoadTreeFromBytes([]byte(safe))
	if err != nil {
		t.Fatal(err)
	}
	if !handBackSafe(node) {
		t.Fatal("speech, a condition, a decorator and return_item are hand-back safe")
	}
	for _, do := range []string{"grant_quest", "give_gold", "give_item", "take_item", "set_state", "set_quest_flag", "command"} {
		tree := strings.Replace(safe, "do: return_item", "do: "+do, 1)
		node, err := LoadTreeFromBytes([]byte(tree))
		if err != nil {
			t.Fatalf("%s: %v", do, err)
		}
		if handBackSafe(node) {
			t.Errorf("%s passed as hand-back safe", do)
		}
	}
}

// The five NPCs whose player_give branches refuse a quest item at the wrong
// step (#375 review M1). Each refusal still plays, now as a hand-back: the
// NPC says the same line and the item, still in the pack, is said to come
// back. States use exact quest steps (no quest registry is loaded).
func TestHandBack_ShippedGiveRefusalsStillPlay(t *testing.T) {
	const (
		templateId = 90700
		instanceId = 90799
		roomId     = 90790
		userId     = 907
	)
	cases := []struct {
		file     string
		itemId   int
		progress map[int]string
		says     string
	}{
		{"stillwater/342-dock_master_arn.yaml", 40054, map[int]string{19: "end"}, "The bounty is paid out."},
		{"stillwater/342-dock_master_arn.yaml", 40054, nil, "See Drunn first, then"},
		{"stillwater/335-constable_drunn.yaml", 40054, map[int]string{19: "end"}, "The bounty is closed."},
		{"stillwater/335-constable_drunn.yaml", 40054, nil, "I cannot officially accept it"},
		{"thornwall_city/99-records_clerk_pell.yaml", 31, map[int]string{6: "report"}, "Reference number 4-7-12-B."},
		{"thornwall_city/99-records_clerk_pell.yaml", 31, nil, "I do not accept unsolicited documents."},
		{"stillwater/355-mistress_vella_thorne.yaml", 40060, map[int]string{20: "vella_journal"}, "You have already brought it."},
		{"stillwater/355-mistress_vella_thorne.yaml", 40060, nil, "That carving did not come from where it was supposed to."},
		{"dustwalk_road/83-road_warden_tessara.yaml", 16, map[int]string{4: "end"}, "You have already done your part."},
		// A catch-all branch answers ordinary gifts only: it would tell a
		// player holding the purse Tessara will ask for, or Sylara's totem,
		// that the NPC has no use for it. No branch plays; give prints its
		// generic hand-back line.
		{"dustwalk_road/83-road_warden_tessara.yaml", 16, nil, ""},
		{"ironwind_steppe/241-windwarden_sylara.yaml", 40033, nil, ""},
	}
	for _, c := range cases {
		t.Run(c.file+" "+c.says, func(t *testing.T) {
			t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
				c.itemId: {ItemId: c.itemId, Name: "quest thing", Type: items.Object, Subtype: items.Mundane},
			}))
			t.Cleanup(seedTestRoom(t, roomId, "Testzone"))
			t.Cleanup(seedTestMob(t, templateId, instanceId, roomId, "Clerk"))
			t.Cleanup(seedTestUser(t, userId, "handback", "Handback", roomId))
			node, err := LoadTreeFromFile(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "behaviors", filepath.FromSlash(c.file)))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(GetEngine().SetMobTreeForTest(templateId, node))
			user := users.GetByUserId(userId)
			user.Character.QuestProgress = c.progress
			itm := items.New(c.itemId)
			if !user.Character.StoreItem(itm) {
				t.Fatal("setup: the player could not carry the item")
			}
			events.DrainQueuedMessagesForTest(userId)
			events.DrainQueuedInputsForTest(instanceId)

			hb := &GiveHandBack{}
			handled := TryMobBehavior(instanceId, EventContext{
				EventType: "player_give", UserId: userId, ItemId: c.itemId, ItemUUID: itm.UUID, HandBack: hb,
			})
			DrainAllDelayedActionsForTest(t)
			if c.says == "" {
				if handled || hb.Returned {
					t.Fatalf("a catch-all branch answered a hand-back: handled %v, returned %v", handled, hb.Returned)
				}
				if said := events.DrainQueuedInputsForTest(instanceId); len(said) != 0 {
					t.Fatalf("the NPC spoke: %q", said)
				}
				return
			}
			if !handled || !hb.Returned {
				t.Fatalf("the refusal branch did not play: handled %v, returned %v", handled, hb.Returned)
			}
			said := strings.Join(events.DrainQueuedInputsForTest(instanceId), "\n")
			if !strings.Contains(strings.Join(strings.Fields(said), " "), c.says) {
				t.Errorf("the NPC did not say %q; it queued %q", c.says, said)
			}
			told := strings.Join(events.DrainQueuedMessagesForTest(userId), "")
			// The seeded room is unlit, so the line names "A figure".
			if !strings.Contains(told, "hands back the quest thing.") {
				t.Errorf("no hand-back line: %q", told)
			}
			if n := countItemsById(user.Character.Items, c.itemId); n != 1 {
				t.Errorf("the player holds %d of the item, want 1", n)
			}
			if m := mobs.GetInstance(instanceId); countItemsById(m.Character.Items, c.itemId) != 0 {
				t.Error("the item moved to the NPC")
			}
		})
	}
}
