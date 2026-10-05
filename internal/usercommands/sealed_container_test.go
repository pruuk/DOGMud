package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// A sealed container (a housing strongbox) is shut to every command, however
// its name is spelled: the engine's own lock checks refuse it, and the paths
// that could force a lock refuse a sealed one outright. These run the real
// commands, many times over, because a container name is resolved by a fuzzy
// match over a map and so in random order.

func sealedRoom(room *rooms.Room) {
	coffer := rooms.Container{Items: []items.Item{items.New(10001)}, Gold: 50}
	coffer.Sealed = true
	coffer.Lock = gamelock.Lock{Difficulty: rooms.SealedLockDifficulty}
	room.Containers = map[string]rooms.Container{
		`mug`:    {Items: []items.Item{items.New(10001)}},
		`coffer`: coffer,
	}
}

func TestSealedContainer_NoSpellingReachesIt(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	prevContainers, prevItems, prevGold := room.Containers, user.Character.Items, user.Character.Gold
	defer func() {
		room.Containers, user.Character.Items, user.Character.Gold = prevContainers, prevItems, prevGold
	}()

	type attempt struct {
		name string
		run  func()
	}
	attempts := []attempt{}
	for _, word := range []string{`coffer`, `coff`, `of`, `off`, `e`, `o`, `COFFER`, `the coffer`} {
		w := word
		attempts = append(attempts,
			attempt{`get all ` + w, func() { Get(`all `+w, user, room, 0) }},
			attempt{`get all from ` + w, func() { Get(`all from `+w, user, room, 0) }},
			attempt{`get sword from ` + w, func() { Get(`sword from `+w, user, room, 0) }},
			attempt{`get gold from ` + w, func() { Get(`gold from `+w, user, room, 0) }},
			attempt{`remove sword from ` + w, func() { Remove(`sword from `+w, user, room, 0) }},
			attempt{`unlock ` + w, func() { Unlock(w, user, room, 0) }},
			attempt{`picklock ` + w, func() { Picklock(w, user, room, 0) }},
			attempt{`look in ` + w, func() { Look(`in `+w, user, room, 0) }},
			attempt{`put sword in ` + w, func() {
				user.Character.Items = []items.Item{items.New(10001)}
				Put(`sword in `+w, user, room, 0)
			}},
		)
	}

	for round := 0; round < 25; round++ {
		for _, a := range attempts {
			sealedRoom(room)
			user.Character.Items, user.Character.Gold = nil, 0
			a.run()
			c := room.Containers[`coffer`]
			if len(c.Items) != 1 || c.Gold != 50 {
				t.Fatalf("%q changed the sealed coffer: %d items, %d gold", a.name, len(c.Items), c.Gold)
			}
			if !c.IsSealedShut() {
				t.Fatalf("%q unlocked the sealed coffer", a.name)
			}
			if user.Character.Gold == 50 {
				t.Fatalf("%q took the coffer's gold", a.name)
			}
		}
	}
}

func TestSealedContainer_OpenForOneCommandWorksNormally(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	user, room := getTestUserAndRoom(t)
	prevContainers, prevItems, prevGold := room.Containers, user.Character.Items, user.Character.Gold
	defer func() {
		room.Containers, user.Character.Items, user.Character.Gold = prevContainers, prevItems, prevGold
	}()

	sealedRoom(room)
	user.Character.Items, user.Character.Gold = nil, 0

	// What housing.OpenStrongboxesForUser does for the owner's command.
	c := room.Containers[`coffer`]
	c.Lock = gamelock.Lock{}
	room.Containers[`coffer`] = c

	Get(`gold from coffer`, user, room, 0)
	if user.Character.Gold != 50 {
		t.Fatalf("the owner could not take gold from their open strongbox: %d", user.Character.Gold)
	}
	// "open coffer" (unlock) on an open sealed container is refused as a
	// lock, not shown: outside a unit room it answers as any unlocked one.
	Unlock(`coffer`, user, room, 0)
	if room.Containers[`coffer`].IsSealedShut() {
		t.Error("unlock changed an open strongbox")
	}
}
