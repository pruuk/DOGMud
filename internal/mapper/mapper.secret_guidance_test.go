package mapper

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// markSecret marks the exits of loaded fixture rooms secret, before any map
// is built from them.
func markSecret(t *testing.T, exits ...struct {
	room int
	dir  string
}) {
	t.Helper()
	for _, e := range exits {
		r := rooms.LoadRoom(e.room)
		if r == nil {
			t.Fatalf("fixture room %d must load", e.room)
		}
		x, ok := r.Exits[e.dir]
		if !ok {
			t.Fatalf("fixture room %d has no %s exit", e.room, e.dir)
		}
		x.Secret = true
		r.Exits[e.dir] = x
	}
}

type secretExit = struct {
	room int
	dir  string
}

// knowsOnly lets guidance through the listed secret exits only.
func knowsOnly(exits ...secretExit) SecretExitKnown {
	return func(roomId int, exitName string, _ int) bool {
		for _, e := range exits {
			if e.room == roomId && e.dir == exitName {
				return true
			}
		}
		return false
	}
}

// secretRingWorld is one zone, 48001 48002 over 48003 48004, a ring of exits;
// the 48001 - 48002 pair is secret both ways, so the open way from 48001 to
// 48002 is south, east, north.
func secretRingWorld(t *testing.T) {
	t.Helper()
	isolateMapperCaches(t)
	writeFixtureWorld(t, []fixtureRoom{
		{id: 48001, x: 0, y: 0, zone: "zs_a", exits: map[string]int{"east": 48002, "south": 48003}},
		{id: 48002, x: 1, y: 0, zone: "zs_a", exits: map[string]int{"west": 48001, "south": 48004}},
		{id: 48003, x: 0, y: 1, zone: "zs_a", exits: map[string]int{"north": 48001, "east": 48004}},
		{id: 48004, x: 1, y: 1, zone: "zs_a", exits: map[string]int{"west": 48003, "north": 48002}},
	})
	markSecret(t, secretExit{48001, "east"}, secretExit{48002, "west"})
	GetMapper(48001)
	zoneGraphFromFixture(t)
}

// Quest guidance takes a secret exit only once the player knows it: until
// then it goes the open way round; mob paths (GetPath, `pathto`) still take
// it, as they always have.
func TestGuidanceSkipsAnUnknownSecretExit(t *testing.T) {
	secretRingWorld(t)
	to := inSet(48002)

	pin, next, exit, ok := NearestOf(48001, to, []string{"zs_a"}, nil)
	if !ok || pin != 48002 || next != 48003 || exit != "south" {
		t.Errorf("unknown secret exit: pin %d next %d %q ok %v; want pin 48002 via south to 48003", pin, next, exit, ok)
	}

	pin, next, exit, ok = NearestOf(48001, to, []string{"zs_a"}, knowsOnly(secretExit{48001, "east"}))
	if !ok || pin != 48002 || next != 48002 || exit != "east" {
		t.Errorf("known secret exit: pin %d next %d %q ok %v; want pin 48002 via east", pin, next, exit, ok)
	}

	p, err := GetPath(48001, 48002)
	if err != nil || len(p) != 1 || p[0].exitName != "east" {
		t.Errorf("mob GetPath 48001 -> 48002: %v, %v; want the one secret step east", p, err)
	}
}

// secretBorderWorld is three zones:
//
//	48021 [zs_b]  48022 [zs_b]
//	48011 [zs_a]  48012 [zs_a]  48031 [zs_c]
//
// 48011 - 48021 is a secret border, 48012 - 48022 an open one, and 48012 - 48031
// (zs_c's only border) is secret.
func secretBorderWorld(t *testing.T) {
	t.Helper()
	isolateMapperCaches(t)
	writeFixtureWorld(t, []fixtureRoom{
		{id: 48011, x: 0, y: 0, zone: "zs_a", exits: map[string]int{"east": 48012, "north": 48021}},
		{id: 48012, x: 1, y: 0, zone: "zs_a", exits: map[string]int{"west": 48011, "north": 48022, "east": 48031}},
		{id: 48021, x: 0, y: -1, zone: "zs_b", exits: map[string]int{"east": 48022, "south": 48011}},
		{id: 48022, x: 1, y: -1, zone: "zs_b", exits: map[string]int{"west": 48021, "south": 48012}},
		{id: 48031, x: 2, y: 0, zone: "zs_c", exits: map[string]int{"west": 48012}},
	})
	markSecret(t, secretExit{48011, "north"}, secretExit{48021, "south"},
		secretExit{48012, "east"}, secretExit{48031, "west"})
	GetMapper(48011)
	GetMapper(48021)
	GetMapper(48031)
	zoneGraphFromFixture(t)
}

// The zone graph guidance reads leaves out a secret border the player does
// not know: it heads for the open border, and a zone reached only through an
// unknown secret border gives no direction.
func TestGuidanceSkipsAnUnknownSecretBorder(t *testing.T) {
	secretBorderWorld(t)

	_, next, exit, ok := NearestOf(48011, inSet(48021), []string{"zs_b"}, nil)
	if !ok || next != 48012 || exit != "east" {
		t.Errorf("toward zs_b, secret border unknown: next %d %q ok %v; want east to 48012, the open border", next, exit, ok)
	}
	_, next, exit, ok = NearestOf(48011, inSet(48021), []string{"zs_b"}, knowsOnly(secretExit{48011, "north"}))
	if !ok || next != 48021 || exit != "north" {
		t.Errorf("toward zs_b, secret border known: next %d %q ok %v; want north to 48021", next, exit, ok)
	}

	if _, next, exit, ok = NearestOf(48011, inSet(48031), []string{"zs_c"}, nil); ok {
		t.Errorf("toward zs_c through an unknown secret border only: next %d %q; want no direction", next, exit)
	}
	_, next, exit, ok = NearestOf(48011, inSet(48031), []string{"zs_c"}, knowsOnly(secretExit{48012, "east"}))
	if !ok || next != 48012 || exit != "east" {
		t.Errorf("toward zs_c, secret border known: next %d %q ok %v; want east to 48012", next, exit, ok)
	}

	if p, err := GetPath(48011, 48031); err != nil || len(p) != 2 {
		t.Errorf("mob GetPath 48011 -> 48031: %v, %v; want 2 steps through the secret border", p, err)
	}
}
