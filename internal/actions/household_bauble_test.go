package actions

import (
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/baubles"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// Taking a household's bauble is `steal` (stealHouseholdBauble): the steal
// checks, and the container theft's observer contest. The crime a resident
// raises is steal.go's (thiefCaught), stood in for here.

type householdHarness struct {
	room   *rooms.Room
	thief  *searchFakeActor
	itm    items.Item
	caught []string
}

// setupHousehold puts a household bauble "on the shelf" in a room and a
// skullduggery-rank thief who can carry it. watcher, when not nil, is in
// the room; resident says whether it is one of the household.
func setupHousehold(t *testing.T, roomId int, rank int, watcher *mobs.Mob, resident bool) *householdHarness {
	t.Helper()
	pinConfigForTest(t)
	pinStealLinearKnobs(t)
	seedBaubleSale(t)

	h := &householdHarness{room: newSearchTestRoom(roomId)}
	h.thief = newSearchFakeActor("Thief", h.room, true, 7700+roomId%100)
	h.thief.char = characters.New()
	h.thief.char.Name = "Thief"
	h.thief.char.Stats.Dexterity.ValueAdj = 10
	h.thief.char.Skills[string(skills.Skullduggery)] = rank
	canCarry(h.thief)

	h.itm = newBauble(t, "Child's Small Doll", "doll", 12, baubles.StatusReady)
	h.itm.LeaveBaubleAt("on the shelf", roomId, time.Now())
	h.room.AddItem(h.itm, false)

	if watcher != nil {
		mobs.SetInstanceForTest(watcher.InstanceId, watcher)
		h.room.AddMob(watcher.InstanceId)
		t.Cleanup(func() { mobs.SetInstanceForTest(watcher.InstanceId, nil) })
	}
	origCaught, origMember, origResidents := householdCaught, householdMember, findHouseholdResidents
	householdCaught = func(_ Actor, m *mobs.Mob, _ *rooms.Room) { h.caught = append(h.caught, m.Character.Name) }
	householdMember = func(*mobs.Mob, *rooms.Room) bool { return resident }
	findHouseholdResidents = func(*rooms.Room) []*mobs.Mob {
		if watcher != nil && resident {
			return []*mobs.Mob{watcher}
		}
		return nil
	}
	t.Cleanup(func() {
		householdCaught, householdMember, findHouseholdResidents = origCaught, origMember, origResidents
	})
	return h
}

// keeper is a watchful mob: nothing gets past it.
func keeper(instId int, name string) *mobs.Mob {
	m := newStealTestMob(instId, 0, 100000)
	m.Character.Name = name
	m.MobId = 42
	return m
}

func (h *householdHarness) steal() StealResult {
	delete(h.thief.char.Cooldowns, skills.Skullduggery.String("steal"))
	return Steal(h.thief, StealOptions{HouseholdItem: h.itm})
}

func (h *householdHarness) inPack() bool {
	for _, it := range h.thief.char.Items {
		if it.Bauble == h.itm.Bauble {
			return true
		}
	}
	return false
}

// hide puts the thief into the Hidden awareness state, as `sneak` does.
func hide(t *testing.T, a *searchFakeActor) {
	t.Helper()
	a.char.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "household_bauble_test"}
	if err := a.char.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing: %v", err)
	}
	a.char.Awareness.ResolveConcealment(true, reason)
	if !a.char.IsHidden() {
		t.Fatal("fixture thief is not hidden")
	}
}

func TestStealHousehold_NobodyWatchingTakesItUnseen(t *testing.T) {
	h := setupHousehold(t, 9701, 2, nil, false)
	h.room.Zone = "Ashwick"

	res := h.steal()

	if !res.Succeeded || res.Detected || !h.inPack() || len(h.room.Items) != 0 {
		t.Fatalf("unseen: %+v in pack=%v", res, h.inPack())
	}
	for _, it := range h.thief.char.Items {
		if it.Bauble == h.itm.Bauble && (it.BaubleSpot != "" || it.BaubleHousehold != 0 || it.BaubleLeftAt != 0) {
			t.Fatalf("carried, it lies nowhere and belongs to no household: %+v", it)
		}
	}
	rec, _ := baubles.Get(h.itm.Bauble)
	if !rec.Stolen || rec.StolenFromRoom != 9701 || rec.StolenByUserId != h.thief.userId || rec.StolenZone != "Ashwick" {
		t.Fatalf("still stolen: %+v", rec)
	}
	if len(h.thief.awards) != 1 || !h.thief.awards[0].won {
		t.Fatalf("an uncontested theft is a won skullduggery award, as in a container: %+v", h.thief.awards)
	}
	if _, n := h.thief.awardedCandidate(string(skills.Skullduggery)); n != 1 {
		t.Fatal("the award is skullduggery's")
	}
}

func TestStealHousehold_CaughtByTheHouseholdIsTheCrime(t *testing.T) {
	h := setupHousehold(t, 9702, 2, keeper(9702, "Martha"), true)

	res := h.steal()

	if !res.Detected || res.Succeeded || h.inPack() || len(h.room.Items) != 1 {
		t.Fatalf("caught, and the bauble stays: %+v", res)
	}
	if len(h.caught) != 1 || h.caught[0] != "Martha" {
		t.Fatalf("one of the household brings down the crime: %v", h.caught)
	}
	if !searchSaid(h.thief, "spots you reaching") {
		t.Fatalf("messages: %q", h.thief.sent)
	}
	if len(h.thief.awards) != 1 || h.thief.awards[0].won {
		t.Fatalf("a lost contest: %+v", h.thief.awards)
	}
	if rec, _ := baubles.Get(h.itm.Bauble); rec.Stolen {
		t.Fatal("a caught attempt steals nothing")
	}
}

// Spotted by someone who is not of the household: revealed, as in a
// container theft, but no crime against the household.
func TestStealHousehold_SpottedByAStranger(t *testing.T) {
	h := setupHousehold(t, 9703, 2, keeper(9703, "Passer-by"), false)
	hide(t, h.thief)

	res := h.steal()

	if !res.Detected || h.inPack() || len(h.caught) != 0 {
		t.Fatalf("spotted, no household crime: %+v caught=%v", res, h.caught)
	}
	if h.thief.char.IsHidden() {
		t.Fatal("the thief is revealed")
	}
}

// The steal checks: the StealMinRank gate, and the steal cooldown.
func TestStealHousehold_UsesTheStealChecks(t *testing.T) {
	h := setupHousehold(t, 9704, 1, nil, false)
	pinStealMinRank(t, 2)
	if res := h.steal(); res.Succeeded || res.Reason != "not advanced enough" || h.inPack() {
		t.Fatalf("rank 1 cannot steal under a rank-2 gate: %+v", res)
	}

	h2 := setupHousehold(t, 9705, 2, nil, false)
	if res := Steal(h2.thief, StealOptions{HouseholdItem: h2.itm}); !res.Succeeded {
		t.Fatalf("first: %+v", res)
	}
	h2.room.AddItem(h2.itm, false) // put one back to try again at once
	if res := Steal(h2.thief, StealOptions{HouseholdItem: h2.itm}); !res.OnCooldown {
		t.Fatalf("the steal cooldown applies: %+v", res)
	}
}

func TestStealHousehold_TooHeavyIsNoCrime(t *testing.T) {
	h := setupHousehold(t, 9706, 2, keeper(9706, "Martha"), true)
	h.thief.char.Stats.Strength.ValueAdj = 0
	h.thief.char.Items = nil
	for i := 0; i < 400; i++ { // weigh the thief down
		h.thief.char.Items = append(h.thief.char.Items, h.itm)
	}

	res := h.steal()

	if res.Succeeded || res.Detected || len(h.caught) != 0 || res.Reason != "overloaded" {
		t.Fatalf("an attempt that cannot lift it is not a theft: %+v", res)
	}
}

// Only a bauble of THIS room's household is stolen this way.
func TestStealHousehold_OnlyThisHouseholds(t *testing.T) {
	h := setupHousehold(t, 9707, 2, nil, false)
	h.itm.BaubleHousehold = 1234
	if res := h.steal(); res.Succeeded {
		t.Fatalf("another household's (or nobody's) bauble is not stolen here: %+v", res)
	}
}
