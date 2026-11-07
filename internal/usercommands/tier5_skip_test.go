package usercommands

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conversation"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

// Quest 64 walked through the real talk path, Talk and the menu, against the
// shipped files: what each ending pays, and that abandon and resume farm nothing.

const (
	skipQuest = 64
	wafer     = 40108
	horstFile = "new_plymouth_merchant/9344.yaml"
	odileFile = "new_plymouth_common/9801.yaml"
	skipFloor = 5625
)

// skipNPC stands the fixture's mob in for Horst or Odile, with quest 64's
// triggers running and the offer index knowing who offers it.
func skipNPC(t *testing.T, file string, mobId int, zone string) (*users.UserRecord, *dialogue.DialogueFile, func(string)) {
	t.Helper()
	user, mob := tier5NPC(t, file, mobId, zone, []string{"64-the_skip.yaml", "67-the_bloom_trail.yaml"}, wafer)
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(quests.GetQuestById(skipQuest))
	t.Cleanup(dialogue.SetOfferIndexForTest(map[int][]int{skipQuest: {9344}}))
	user.Character.Gold = 0
	user.Character.QuestProgress = map[int]string{}

	data, err := os.ReadFile(filepath.Join(tier5World, "dialogue", file))
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(data, &df))

	// choose plays a menu line by its key, then lands the tokens it queued.
	choose := func(key string) {
		t.Helper()
		conversation.Talk(user, mustRoom(t, user), mob)
		number := skipMenuNumber(t, user, &df, key)
		require.True(t, user.RouteMenuInput(number, true), key)
		if df.FindTopic(key).Kind == dialogue.KindOffer {
			require.True(t, user.RouteMenuInput("1", true), "1 is Accept")
		}
		for _, tok := range questTokensFor(1) {
			user.Character.GiveQuestToken(tok)
		}
		drained(1)
		drained(2)
	}
	return user, &df, choose
}

func skipMenuKeys(user *users.UserRecord, df *dialogue.DialogueFile) []string {
	var keys []string
	for _, e := range df.Menu("npc", conversation.PlayerState(user)).Entries {
		keys = append(keys, e.Id)
	}
	return keys
}

func skipMenuNumber(t *testing.T, user *users.UserRecord, df *dialogue.DialogueFile, key string) string {
	t.Helper()
	for i, k := range skipMenuKeys(user, df) {
		if k == key {
			return string(rune('1' + i))
		}
	}
	require.Failf(t, "not on the menu", "%s: %v", key, skipMenuKeys(user, df))
	return ""
}

func skipGateOpen(user *users.UserRecord) bool {
	return !actions.QuestGateBlocks(user, &rooms.Room{RoomId: 1}, &rooms.Room{RoomId: skipFloor})
}

func TestTheSkip_ReportedPaysOnceAndCloses(t *testing.T) {
	user, df, choose := skipNPC(t, horstFile, 9344, "New Plymouth Merchant")
	user.Character.SetQuestFlag("67-entry", "undercover")
	user.Character.QuestProgress[skipQuest] = "found"

	require.NotContains(t, skipMenuKeys(user, df), "q64_report", "hear the terms first")
	choose("q64_terms")
	require.Zero(t, user.Character.Gold, "the terms pay nothing")
	choose("q64_report")

	require.Equal(t, "end", user.Character.QuestProgress[skipQuest])
	require.Equal(t, "reported", user.Character.GetQuestFlag("64-outcome"))
	require.Equal(t, 150, user.Character.Gold)
	require.Equal(t, 1, countBackpack(user, wafer))

	menu := skipMenuKeys(user, df)
	require.NotContains(t, menu, "q64_offer", "no second job")
	require.NotContains(t, menu, "q64_report", "no second pay")
	ok, _ := AbandonQuest(user, skipQuest)
	require.False(t, ok, "a finished quest cannot be shelved, so the pay cannot come round again")
	require.False(t, skipGateOpen(user), "her hiding place closes with her story")
}

func TestTheSkip_RanPaysNothingAndCloses(t *testing.T) {
	user, df, choose := skipNPC(t, odileFile, 9801, "New Plymouth Common")
	user.Character.SetQuestFlag("67-entry", "undercover")
	user.Character.QuestProgress[skipQuest] = "start"

	require.NotContains(t, skipMenuKeys(user, df), "q64_run", "hear her first")
	choose("who")
	require.Equal(t, "start", user.Character.QuestProgress[skipQuest], "only her story moves the step")
	choose("q64_found")
	require.Equal(t, "found", user.Character.QuestProgress[skipQuest])
	choose("q64_run")

	require.Equal(t, "end", user.Character.QuestProgress[skipQuest])
	require.Equal(t, "ran", user.Character.GetQuestFlag("64-outcome"))
	require.Zero(t, user.Character.Gold)
	require.Zero(t, countBackpack(user, wafer))
	require.NotContains(t, skipMenuKeys(user, df), "q64_run")
	require.False(t, skipGateOpen(user))

	// Horst has neither the job nor the pay left for this player.
	data, err := os.ReadFile(filepath.Join(tier5World, "dialogue", horstFile))
	require.NoError(t, err)
	var horst dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(data, &horst))
	menu := skipMenuKeys(user, &horst)
	require.NotContains(t, menu, "q64_offer")
	require.NotContains(t, menu, "q64_report")
	require.NotContains(t, menu, "q64_terms")
	text, _ := horst.OpenerFor(conversation.PlayerState(user))
	require.Contains(t, text, "no more work for you")
}

// A shelved quest closes the stair and hands nothing back, a resume hands nothing
// over, and the player picks up at the step they left, able to finish, at both
// steps and through either ending.
func TestTheSkip_AbandonAndResumeFarmNothing(t *testing.T) {
	round := func(t *testing.T, user *users.UserRecord, choose func(string), wantStep string) {
		t.Helper()
		ok, line := AbandonQuest(user, skipQuest)
		require.True(t, ok, line)
		require.False(t, skipGateOpen(user), "a shelved quest is not held, so the stair is shut")
		require.Zero(t, user.Character.Gold)
		require.Zero(t, countBackpack(user, wafer))

		// Once she has told her story the offer reads true for someone who found
		// her, so it is a different line on the menu.
		offer := "q64_offer"
		if user.Character.GetQuestFlag("64-seen") == "found" {
			offer = "q64_resume"
		}
		choose(offer)
		require.Equal(t, wantStep, user.Character.QuestProgress[skipQuest])
		require.True(t, skipGateOpen(user), "the stair opens again")
		require.Zero(t, user.Character.Gold, "a resume pays nothing")
		require.Zero(t, countBackpack(user, wafer), "a resume hands over no wafer")
	}

	t.Run("reported", func(t *testing.T) {
		user, _, choose := skipNPC(t, horstFile, 9344, "New Plymouth Merchant")
		user.Character.SetQuestFlag("67-entry", "undercover")
		choose("q64_offer")
		require.Equal(t, "start", user.Character.QuestProgress[skipQuest])
		round(t, user, choose, "start")

		user.Character.QuestProgress[skipQuest] = "found"
		round(t, user, choose, "found")
		choose("q64_terms")
		round(t, user, choose, "found")
		choose("q64_report")
		require.Equal(t, 150, user.Character.Gold, "paid once")
		require.Equal(t, 1, countBackpack(user, wafer), "one wafer")
	})

	t.Run("ran", func(t *testing.T) {
		user, _, choose := skipNPC(t, odileFile, 9801, "New Plymouth Common")
		user.Character.SetQuestFlag("67-entry", "undercover")
		user.Character.QuestProgress[skipQuest] = "start"
		choose("q64_found")
		ok, line := AbandonQuest(user, skipQuest)
		require.True(t, ok, line)
		require.False(t, skipGateOpen(user))
		// Horst takes it back up; the token lands as his Accept's would.
		user.Character.GiveQuestToken("64-start")
		require.Equal(t, "found", user.Character.QuestProgress[skipQuest])
		choose("q64_run")
		require.Equal(t, "ran", user.Character.GetQuestFlag("64-outcome"))
		require.Zero(t, user.Character.Gold)
	})
}
