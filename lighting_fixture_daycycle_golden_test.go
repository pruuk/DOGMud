package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/hooks"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

var updateFixtureDaycycle = flag.Bool("update-lighting-fixture-daycycle", false,
	"re-record testdata/lighting_fixture_daycycle.golden")

// fixtureDaycycleRooms are the rooms whose fixtures slice 1 ships: 4111 (the
// North Gate's arch lantern, item 55) and 5000 (the Rift Chamber's Rift
// Stone, item 56).
var fixtureDaycycleRooms = []int{4111, 5000}

// TestLightingFixtureDayCycle is the fixture day-cycle record of lighting 5e
// (spec X8). The day-cycle golden loads no items and runs no Prepare and no
// tick, so it cannot see a fixture. This test loads items, prepares the two
// fixture rooms (spawninfo places each fixture), and runs the real item tick
// (hooks.ItemRoundTick) at the day-cycle golden's twelve samples, recording
// each room's light and its fixture's state.
//
// Expected shape: 4111 reads its sky by day and the sky combined with the
// lantern's 52 at night (midwinter midnight 34 becomes 54: shapes to
// faces); 5000 is a dungeon with no sky, so it reads 40 to 45 at every hour,
// whatever round the sample lands on in the stone's twelve-round pulse.
func TestLightingFixtureDayCycle(t *testing.T) {
	loadItemBehaviourWorld(t)
	t.Cleanup(items.ResetHolderIndexForTest())
	t.Cleanup(itemlight.ResetForTest())
	t.Cleanup(behaviortree.ResetItemBTreeStatesForTest())
	prevHook := items.OnRoomHolderIndexed
	items.OnRoomHolderIndexed = hooks.EvaluateRoomFixtures
	t.Cleanup(func() { items.OnRoomHolderIndexed = prevHook })

	prepared := map[int]*rooms.Room{}
	spawnInfo := map[int][]rooms.SpawnInfo{}
	for _, id := range fixtureDaycycleRooms {
		r := rooms.LoadRoom(id)
		if r == nil {
			t.Fatalf("room %d failed to load", id)
		}
		// Prepare stamps each item spawn's DespawnedRound, and the loaded
		// room outlives the test, so without restoring it a second run
		// (-count=2) finds the spawn on its respawn timer and places nothing.
		spawnInfo[id] = append([]rooms.SpawnInfo(nil), r.SpawnInfo...)
		r.Prepare(false)
		prepared[id] = r
	}
	// Leave the shared rooms as the other root tests expect them: no
	// fixture on the floor, nothing lit, and the spawn bookkeeping as loaded.
	t.Cleanup(func() {
		for id, r := range prepared {
			for _, it := range append([]items.Item(nil), r.Items...) {
				if it.IsFixture() {
					r.RemoveItem(it, false)
				}
			}
			r.SpawnInfo = spawnInfo[id]
		}
	})

	var b strings.Builder
	for _, s := range sampleRounds() {
		util.SetRoundCount(s.Round)
		hooks.ItemRoundTick(events.NewRound{})
		fmt.Fprintf(&b, "== %s (round %d)\n", s.Label, s.Round)
		for _, id := range fixtureDaycycleRooms {
			r := prepared[id]
			fixtures := []string{}
			for _, it := range r.Items {
				if !it.IsFixture() {
					continue
				}
				state := "unlit"
				if v, ok := itemlight.Get(id, it.UUID); ok && itemlight.Lit(id, it.UUID) {
					state = fmt.Sprintf("lit %.1f", v)
				}
				fixtures = append(fixtures, fmt.Sprintf("%d %s", it.ItemId, state))
			}
			if len(fixtures) == 0 {
				t.Fatalf("room %d has no fixture on its floor after Prepare: spawninfo did not place it", id)
			}
			fmt.Fprintf(&b, "room %d light=%d fixtures=[%s]\n", id, r.LightLevel(), strings.Join(fixtures, ", "))
		}
	}
	got := b.String()

	goldenPath := filepath.Join("testdata", "lighting_fixture_daycycle.golden")
	if *updateFixtureDaycycle {
		if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		t.Logf("recorded %s", goldenPath)
		return
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v (record it with -update-lighting-fixture-daycycle)", err)
	}
	if got != string(want) {
		t.Errorf("fixture day-cycle golden moved. Explain every changed line (which fixture, "+
			"which sample, why) before re-recording with\n"+
			"  go test . -run TestLightingFixtureDayCycle -update-lighting-fixture-daycycle -v\n\ngot:\n%s", got)
	}
}
