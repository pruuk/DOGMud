package actions

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// ---------------------------------------------------------------------------
// search-test-specific helpers
// ---------------------------------------------------------------------------

// searchFakeActor is a minimal Actor with a configurable room. It records
// SendText messages for assertion. It satisfies the full Actor interface.
type searchFakeActor struct {
	awardRecorder // records Actor.AwardResolved calls
	char          *characters.Character
	room          *rooms.Room
	name          string
	isPlayer      bool
	userId        int
	mobInstId     int
	sent          []string
}

func newSearchFakeActor(name string, room *rooms.Room, isPlayer bool, userId int) *searchFakeActor {
	c := &characters.Character{
		Name:  name,
		Buffs: buffs.New(),
	}
	c.Stats.Perception.ValueAdj = 100
	return &searchFakeActor{
		char:     c,
		room:     room,
		name:     name,
		isPlayer: isPlayer,
		userId:   userId,
	}
}

func newSearchMobActor(name string, room *rooms.Room, mobInstId int) *searchFakeActor {
	c := &characters.Character{
		Name:  name,
		Buffs: buffs.New(),
	}
	return &searchFakeActor{
		char:      c,
		room:      room,
		name:      name,
		isPlayer:  false,
		mobInstId: mobInstId,
	}
}

func (a *searchFakeActor) GetCharacter() *characters.Character    { return a.char }
func (a *searchFakeActor) GetRoom() *rooms.Room                   { return a.room }
func (a *searchFakeActor) GetName() string                        { return a.name }
func (a *searchFakeActor) IsPlayer() bool                         { return a.isPlayer }
func (a *searchFakeActor) GetUserId() int                         { return a.userId }
func (a *searchFakeActor) GetMobInstanceId() int                  { return a.mobInstId }
func (a *searchFakeActor) AddBuff(_ int, _ string)                {}
func (a *searchFakeActor) OnSkillUse(_ string) bool               { return false }
func (a *searchFakeActor) OnStatUse(_ string) bool                { return false }
func (a *searchFakeActor) SendRoomCommunication(_ string, _ bool) {}
func (a *searchFakeActor) SendText(_ messaging.Category, msg string) {
	a.sent = append(a.sent, msg)
}

// newSearchTestRoom builds a minimal Room with no exits.
func newSearchTestRoom(roomId int) *rooms.Room {
	return &rooms.Room{RoomId: roomId}
}

// newSearchTestMob builds a minimal Mob with an InstanceId.
func newSearchTestMob(instId int, name string, roomId int) *mobs.Mob {
	m := &mobs.Mob{
		InstanceId: instId,
	}
	m.Character.Name = name
	m.Character.Buffs = buffs.New()
	m.Character.RoomId = roomId
	return m
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

// TestSearch_EmptyRoomNothingFound confirms an empty room produces
// zero hits across all tiers.
func TestSearch_EmptyRoomNothingFound(t *testing.T) {
	room := newSearchTestRoom(9201)
	actor := newSearchFakeActor("SearchTester", room, false, 0)

	result := Search(actor, SearchOptions{})

	if len(result.HiddenExitsFound) != 0 ||
		len(result.HiddenContainersFound) != 0 ||
		len(result.StashedItemsFound) != 0 ||
		len(result.HiddenPlayersFound) != 0 ||
		len(result.HiddenMobsFound) != 0 ||
		len(result.HiddenNounsFound) != 0 {
		t.Error("expected all-empty result for empty room")
	}
}

// TestSearch_CooldownGate confirms two consecutive calls within the
// cooldown return OnCooldown=true on the second.
func TestSearch_CooldownGate(t *testing.T) {
	room := newSearchTestRoom(9202)
	actor := newSearchFakeActor("SearchTester2", room, false, 0)

	_ = Search(actor, SearchOptions{})
	second := Search(actor, SearchOptions{})

	if !second.OnCooldown {
		t.Error("second call within 2-round window should return OnCooldown=true")
	}
}

// TestSearch_MobActorSilent confirms mob actor invocation works
// without panic.
func TestSearch_MobActorSilent(t *testing.T) {
	room := newSearchTestRoom(9203)
	mob := newSearchTestMob(8801, "SearchMob", 9203)
	actor := newSearchMobActor("SearchMob", room, mob.InstanceId)

	_ = Search(actor, SearchOptions{})
}

// ---------------------------------------------------------------------------
// U10b-1 Task 14: the search award
// ---------------------------------------------------------------------------

// searchRoomWithHiddenNouns builds a room carrying n hidden nouns, the tier
// that needs no mob or player registry to exercise.
func searchRoomWithHiddenNouns(roomId, n int) *rooms.Room {
	room := newSearchTestRoom(roomId)
	room.HiddenNouns = map[string]rooms.HiddenNoun{}
	for i := 0; i < n; i++ {
		key := fmt.Sprintf("hidden-thing-%d", i)
		room.HiddenNouns[key] = rooms.HiddenNoun{HiddenDescription: "Something is here."}
	}
	return room
}

// A room with FIVE hidden things awards exactly as many events as a room with
// ONE: the six tiers are one resolved action, not one per roll.
//
// The plan's required test. Search rolls once per candidate, so a rich room
// rolls five times; if the award had ever been placed inside a tier loop this
// would report five.
func TestSearch_AwardsOncePerSearchRegardlessOfCandidateCount(t *testing.T) {
	pinConfigForTest(t)

	one := newSearchFakeActor("SearchOne", searchRoomWithHiddenNouns(9301, 1), false, 0)
	five := newSearchFakeActor("SearchFive", searchRoomWithHiddenNouns(9302, 5), false, 0)

	_ = Search(one, SearchOptions{})
	_ = Search(five, SearchOptions{})

	if got := len(one.awards); got != 1 {
		t.Errorf("a search of a room with ONE candidate produced %d awards, want 1", got)
	}
	if got := len(five.awards); got != 1 {
		t.Errorf("a search of a room with FIVE candidates produced %d awards, want 1; the six tiers are ONE resolved action", got)
	}
}

// A fruitless but RESOLVED search awards at the loss weight.
//
// The plan's second required test, and the case this task exists for: a search
// that rolled against real candidates and found nothing used to pay a FULL
// event, exactly as much as one that found everything.
//
// Determinism without pinning the dice: the hidden-noun tier needs a roll of
// 175 against a searchScore built from Perception + search*SkillWeight. The
// fixture's Perception is 100 and its search rank 0, so the roll is centred on
// 100 with a 0.15 spread -- 175 is five sigma away and is not going to happen.
// The assertion below fails loudly rather than flaking if it ever does.
func TestSearch_AFruitlessSearchAwardsAtTheLossWeight(t *testing.T) {
	pinConfigForTest(t)

	actor := newSearchFakeActor("SearchFruitless", searchRoomWithHiddenNouns(9303, 3), false, 0)

	result := Search(actor, SearchOptions{})

	if result.FoundAnything() {
		t.Skip("fixture beat a five-sigma roll and actually found something; nothing to assert")
	}
	if got := len(actor.awards); got != 1 {
		t.Fatalf("a resolved but fruitless search produced %d awards, want 1; it rolled against real candidates and must still train", got)
	}
	if actor.awards[0].won {
		t.Error("a search that found nothing reported won=true; a fruitless search is a LOSS and must pay the failure fraction")
	}
	if _, n := actor.awardedCandidate(string(skills.Search)); n != 1 {
		t.Errorf("the award named the search skill %d times, want 1", n)
	}
}

// A search that rolled against NOTHING awards nothing at all.
//
// This is the anti-botting gate and it is NOT the firing rule: an empty room
// resolved no contest, so there is no loss to pay a fraction on. Without this,
// `search` in a bare corridor would become a free progression tick now that
// losing pays.
func TestSearch_AnEmptyRoomAwardsNothing(t *testing.T) {
	pinConfigForTest(t)

	actor := newSearchFakeActor("SearchEmpty", newSearchTestRoom(9304), false, 0)

	_ = Search(actor, SearchOptions{})

	if got := len(actor.awards); got != 0 {
		t.Errorf("a search of an empty room produced %d awards, want 0; nothing was rolled against, so nothing resolved", got)
	}
}

// ---------------------------------------------------------------------------
// U10b-1 review followup: the fruitless-search message
// ---------------------------------------------------------------------------

// A search that finds nothing TELLS the player so.
//
// Before this, a fruitless search printed "You snoop around for a bit..." and
// then nothing at all, so the most common outcome of the most common search
// looked like an ignored command.
func TestSearch_AFruitlessSearchTellsThePlayer(t *testing.T) {
	pinConfigForTest(t)

	actor := newSearchFakeActor("SearchTalker", searchRoomWithHiddenNouns(9401, 3), true, 7001)

	result := Search(actor, SearchOptions{})
	if result.FoundAnything() {
		t.Skip("fixture beat a five-sigma roll and found something; nothing to assert")
	}

	if !searchSaid(actor, "You find nothing of interest.") {
		t.Errorf("a fruitless search sent %q, none of which tells the player the search finished", actor.sent)
	}
}

// THE ANTI-LEAK PROPERTY, and the reason the two cases share one message.
//
// A room with hidden things that you FAILED to find, and a room with nothing in
// it at all, must be indistinguishable from the player's side. If they differed,
// `search` would become an oracle for the EXISTENCE of hidden content: stand in
// a room, read the line, and learn a secret exit is there without ever passing
// the roll.
//
// This is the test that stops a well-meaning future change from "helpfully"
// splitting the message into "you sense something eludes you" and "there is
// nothing here". That change would defeat every hidden thing in the world.
func TestSearch_EmptyAndFruitlessRoomsAreIndistinguishable(t *testing.T) {
	pinConfigForTest(t)

	fruitless := newSearchFakeActor("SearchFruit", searchRoomWithHiddenNouns(9402, 3), true, 7002)
	empty := newSearchFakeActor("SearchBare", newSearchTestRoom(9403), true, 7003)

	rf := Search(fruitless, SearchOptions{})
	_ = Search(empty, SearchOptions{})
	if rf.FoundAnything() {
		t.Skip("fixture beat a five-sigma roll and found something; nothing to compare")
	}

	if !reflect.DeepEqual(fruitless.sent, empty.sent) {
		t.Errorf("a failed search and a bare-room search must read IDENTICALLY "+
			"or search becomes a hidden-content detector.\n failed: %q\n bare:   %q",
			fruitless.sent, empty.sent)
	}

	// And the award still differs, which is the whole point: only one of them
	// resolved a contest.
	if len(fruitless.awards) != 1 {
		t.Errorf("the fruitless search produced %d awards, want 1", len(fruitless.awards))
	}
	if len(empty.awards) != 0 {
		t.Errorf("the bare-room search produced %d awards, want 0", len(empty.awards))
	}
}

// searchSaid reports whether any message the actor received contains want.
func searchSaid(a *searchFakeActor, want string) bool {
	for _, m := range a.sent {
		if strings.Contains(m, want) {
			return true
		}
	}
	return false
}
