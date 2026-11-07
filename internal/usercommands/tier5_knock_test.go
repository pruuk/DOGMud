package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
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

// Quest 102 walked through the real talk path, Talk and the menu, against the
// shipped files: the stake is charged once however often the quest is shelved, a
// shelved payer's lines read true, and the overheard line waits for the first
// time through.

const (
	knockQuest = 102
	knockDoor  = 5530
	knockFile  = "new_plymouth_docks/9305.yaml"
	knockLine  = "three months at minimum before the new one's producing at volume, if she survives the first month"
)

var knockAnsi = regexp.MustCompile(`</?ansi[^>]*>`)

// knockPlain is what the player reads, with the colour tags gone and the wrapped
// lines joined, so a phrase a test looks for, or looks for the absence of, cannot
// hide across a line break.
func knockPlain(s string) string {
	return strings.Join(strings.Fields(knockAnsi.ReplaceAllString(s, "")), " ")
}

type knockRig struct {
	t    *testing.T
	user *users.UserRecord
	df   *dialogue.DialogueFile
	talk func() string
}

func knockFixture(t *testing.T, gold int) *knockRig {
	t.Helper()
	user, mob := tier5NPC(t, knockFile, 9305, "New Plymouth Docks", []string{"102-the_knock.yaml", "64-the_skip.yaml", "67-the_bloom_trail.yaml"})
	t.Cleanup(questengine.ResetEngineForTest())
	questengine.GetEngine().RegisterQuest(quests.GetQuestById(knockQuest))
	t.Cleanup(dialogue.SetOfferIndexForTest(map[int][]int{knockQuest: {9305}}))
	user.Character.Gold = gold
	user.Character.QuestProgress = map[int]string{}
	user.Character.SetQuestFlag("64-outcome", "reported")

	data, err := os.ReadFile(filepath.Join(tier5World, "dialogue", knockFile))
	require.NoError(t, err)
	var df dialogue.DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(data, &df))

	return &knockRig{t: t, user: user, df: &df, talk: func() string {
		drained(1)
		conversation.Talk(user, mustRoom(t, user), mob)
		return knockPlain(drained(1))
	}}
}

// accept talks, plays an offer's line and accepts its card, lands the token, and
// returns everything the player read after the opener.
func (r *knockRig) accept(key string) string {
	r.t.Helper()
	r.talk()
	require.True(r.t, r.user.RouteMenuInput(skipMenuNumber(r.t, r.user, r.df, key), true), key)
	require.True(r.t, r.user.RouteMenuInput("1", true), "1 is Accept")
	out := knockPlain(drained(1))
	for _, tok := range questTokensFor(1) {
		r.user.Character.GiveQuestToken(tok)
	}
	drained(1)
	drained(2)
	return out
}

func (r *knockRig) doorOpen() bool {
	return !actions.QuestGateBlocks(r.user, &rooms.Room{RoomId: 5523}, &rooms.Room{RoomId: knockDoor})
}

// enter raises room_enter for the room behind the door, the event the hook raises
// on arrival, and returns what the player read.
func (r *knockRig) enter() string {
	r.t.Helper()
	drained(1)
	bridge := questengine.NewGameBridge(r.user, r.user.Character.RoomId)
	questengine.GetEngine().Notify("room_enter", questengine.EventDetails{UserId: r.user.UserId, RoomId: knockDoor}, bridge, bridge)
	questTokensFor(1)
	return knockPlain(drained(1))
}

func (r *knockRig) shelve() {
	r.t.Helper()
	ok, line := AbandonQuest(r.user, knockQuest)
	require.True(r.t, ok, line)
	questTokensFor(1)
	drained(1)
}

func TestTheKnock_StakeIsChargedOnceAcrossAShelf(t *testing.T) {
	r := knockFixture(t, 600)
	require.False(t, r.doorOpen(), "no knock, no door")

	// A player who has not paid hears the price and sees the offer.
	opener := r.talk()
	require.Contains(t, opener, "Five hundred gold")
	require.Contains(t, opener, "choose The Knock below")
	pitch := r.accept("q102_offer")
	require.Contains(t, pitch, "takes the gold now")
	require.Contains(t, pitch, "Five hundred gold")
	require.NotContains(t, pitch, "Three short")
	require.Equal(t, 100, r.user.Character.Gold, "the stake is taken")
	require.Equal(t, "start", r.user.Character.QuestProgress[knockQuest])
	require.Equal(t, "paid", r.user.Character.GetQuestFlag("102-stake"))
	require.True(t, r.doorOpen())

	// Shelved and broke: the lines read true, and the way back costs nothing.
	r.shelve()
	require.False(t, r.doorOpen(), "a shelved quest is not held, so the door is shut")
	require.Equal(t, 100, r.user.Character.Gold, "abandon hands nothing back")
	r.user.Character.Gold = 0
	opener = r.talk()
	require.NotContains(t, opener, "Five hundred")
	require.NotContains(t, opener, "five hundred")
	require.Contains(t, opener, "Choose The Knock below")
	require.NotContains(t, skipMenuKeys(r.user, r.df), "q102_offer")
	pitch = r.accept("q102_resume")
	require.NotContains(t, pitch, "takes the gold")
	require.NotContains(t, pitch, "ive hundred")
	require.Zero(t, r.user.Character.Gold)
	require.Equal(t, "start", r.user.Character.QuestProgress[knockQuest])
	require.True(t, r.doorOpen())

	// Shelved and rich: the paying line never comes back.
	r.shelve()
	r.user.Character.Gold = 5000
	require.NotContains(t, skipMenuKeys(r.user, r.df), "q102_offer")
	r.accept("q102_resume")
	require.Equal(t, 5000, r.user.Character.Gold, "charged once")
}

// A player who cannot pay reads the price, reaches no offer, and pays nothing.
func TestTheKnock_ShortOfTheStakeTakesNothing(t *testing.T) {
	r := knockFixture(t, 499)
	opener := r.talk()
	require.Contains(t, opener, "Five hundred gold")
	keys := skipMenuKeys(r.user, r.df)
	require.NotContains(t, keys, "q102_offer")
	require.NotContains(t, keys, "q102_resume")
	require.Equal(t, 499, r.user.Character.Gold)
	require.False(t, r.doorOpen())
}

func TestTheKnock_OverheardOnceAcrossAShelf(t *testing.T) {
	r := knockFixture(t, 600)

	// Nobody overhears without the knock.
	require.NotContains(t, r.enter(), knockLine)
	r.accept("q102_offer")

	// Shelved between paying and entering: the line waits, not lost.
	r.shelve()
	require.NotContains(t, r.enter(), knockLine, "a shelved quest overhears nothing")
	_, shelvedStep := r.user.Character.ShelvedQuests[knockQuest]
	require.True(t, shelvedStep)
	r.accept("q102_resume")
	require.Equal(t, "start", r.user.Character.QuestProgress[knockQuest])

	out := r.enter()
	require.Contains(t, out, "three months at minimum")
	require.Contains(t, out, "if she survives the first month")
	require.Equal(t, "behind", r.user.Character.QuestProgress[knockQuest])

	// Never again, through any later entry or any later shelf.
	require.NotContains(t, r.enter(), "three months")
	r.shelve()
	r.accept("q102_resume")
	require.Equal(t, "behind", r.user.Character.QuestProgress[knockQuest], "picks up where it left off")
	require.True(t, r.doorOpen())
	require.NotContains(t, r.enter(), "three months")
	require.Equal(t, 100, r.user.Character.Gold, "still charged once")
}

// The shipped refusal, through the real look and scan. The trigger's room is moved
// to the fixture's exit; its words and its conditions are the shipped ones.
func TestTheKnock_DoorReadsTheSameByLookAndStep(t *testing.T) {
	user, room := seedDarknessGateRoom(t, 90)
	t.Cleanup(questengine.ResetEngineForTest())

	data, err := os.ReadFile(filepath.Join(tier5World, "quests", "102-the_knock.yaml"))
	require.NoError(t, err)
	var q questengine.QuestDef
	require.NoError(t, yaml.Unmarshal(data, &q))
	found := false
	for i := range q.Triggers {
		if q.Triggers[i].Event == "room_gate" {
			q.Triggers[i].Room, found = 1, true
		}
	}
	require.True(t, found)
	questengine.GetEngine().RegisterQuest(&q)

	lookOut := runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
	require.Contains(t, lookOut, "barred from the other side")
	require.NotContains(t, lookOut, "You peer")

	require.True(t, actions.QuestGateRefuses(user, room, rooms.LoadRoom(1)))
	require.Equal(t, lookOut, drained(user.UserId), "a look and a step read one line")

	scanOut := runGate(t, user, func() (bool, error) { return Scan("", user, room, 0) })
	require.Contains(t, scanOut, `the exit is locked`)
	require.NotContains(t, scanOut, "barred")

	user.Character.QuestProgress = map[int]string{knockQuest: "start"}
	t.Cleanup(func() { delete(user.Character.QuestProgress, knockQuest) })
	lookOut = runGate(t, user, func() (bool, error) { return Look("south", user, room, 0) })
	require.Contains(t, lookOut, "You peer", "the door is open to the knock")
	require.NotContains(t, lookOut, "barred")
}
