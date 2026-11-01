package questengine

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/areas"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

// The Shadows line's steps (#375 phase 7), walked through the real triggers:
// each step moves on what its line names, and not on the near misses a player
// will try. The owner ruled that a plant and an ambush count on the attempt, so
// those two events carry no success and the lines claim none.

const (
	gethin      = 9125
	dace        = 9126
	pocketDummy = 9127
	sandPurse   = 60
	hoodedLamp  = 20097
	oilLantern  = 40038
)

func shadowsInstallMine(t *testing.T) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	data, err := os.ReadFile(filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "areas", "coulee_mine.yaml"))
	require.NoError(t, err)
	var a areas.Area
	require.NoError(t, yaml.Unmarshal(data, &a))
	zones := map[int]string{}
	for _, id := range a.Rooms {
		zones[id] = "Pothole Coulee"
	}
	t.Cleanup(areas.InstallForTest([]areas.Area{a}, zones))
}

func TestQuest98FirstLight_Steps(t *testing.T) {
	q := loadShippedQuest(t, "98-first_light.yaml", 98)
	w := newQuestWalker(q).at("start")

	w.notify("topic_heard", EventDetails{MobId: gethin, Topic: "night"})
	require.Equal(t, "start", w.p.current, "only the lantern topic counts")
	w.notify("topic_heard", EventDetails{MobId: gethin, Topic: "first_light_lantern"})
	require.Equal(t, "lantern", w.p.current)

	w.notify("command", EventDetails{Command: "equip", RoomId: mineHead, TargetKind: "item", ItemId: oilLantern})
	require.Equal(t, "lantern", w.p.current, "an oil lantern has no hood")
	w.notify("command", EventDetails{Command: "equip", RoomId: mineHead, TargetKind: "item", ItemId: hoodedLamp})
	require.Equal(t, "equip", w.p.current)

	w.notify("room_enter", EventDetails{RoomId: floodedSump})
	require.Equal(t, "equip", w.p.current, "only the drift counts")
	w.notify("room_enter", EventDetails{RoomId: timberedDrift})
	require.Equal(t, "drift", w.p.current)

	w.notify("command", EventDetails{Command: "hood", RoomId: mineHead, TargetKind: "item", ItemId: hoodedLamp})
	require.Equal(t, "drift", w.p.current, "the hood comes down in the drift")
	w.notify("command", EventDetails{Command: "hood", RoomId: timberedDrift, TargetKind: "item", ItemId: hoodedLamp})
	require.Equal(t, "hooded", w.p.current)
}

func TestQuest99QuietHands_Steps(t *testing.T) {
	q := loadShippedQuest(t, "99-quiet_hands.yaml", 99)
	w := newQuestWalker(q).at("start")

	w.notify("topic_heard", EventDetails{MobId: dace, Topic: "quiet_hands_lesson"})
	require.Equal(t, "lesson", w.p.current)
	w.notify("command", EventDetails{Command: "sneak", RoomId: 5201})
	require.Equal(t, "hidden", w.p.current, "a student spotted on the jetty may hide next door")

	w.notify("command", EventDetails{Command: "steal", RoomId: reedJetty, TargetKind: "mob", MobId: pocketDummy})
	require.Equal(t, "hidden", w.p.current, "a steal attempt proves nothing: only the purse landing does")
	w.notify("item_gain", EventDetails{ItemId: sandPurse, RoomId: 5201})
	require.Equal(t, "hidden", w.p.current, "only on the jetty")
	w.notify("item_gain", EventDetails{ItemId: sandPurse, RoomId: reedJetty})
	require.Equal(t, "lifted", w.p.current)

	w.notify("command", EventDetails{Command: "plant", RoomId: reedJetty, TargetKind: "mob", MobId: pocketDummy, ItemId: hoodedLamp})
	require.Equal(t, "lifted", w.p.current, "only the purse")
	w.notify("command", EventDetails{Command: "plant", RoomId: reedJetty, TargetKind: "mob", MobId: dace, ItemId: sandPurse})
	require.Equal(t, "lifted", w.p.current, "only on the dummy")
	w.notify("command", EventDetails{Command: "plant", RoomId: reedJetty, TargetKind: "mob", MobId: pocketDummy, ItemId: sandPurse})
	require.Equal(t, "planted", w.p.current)

	w.notify("command", EventDetails{Command: "picklock", RoomId: reedJetty, Noun: "north"})
	require.Equal(t, "planted", w.p.current, "an exit lock is not the lockbox")
	w.notify("command", EventDetails{Command: "picklock", RoomId: 5201, TargetKind: "container", Noun: "lockbox"})
	require.Equal(t, "planted", w.p.current, "only on the jetty")
	w.notify("command", EventDetails{Command: "picklock", RoomId: reedJetty, TargetKind: "container", Noun: "lockbox"})
	require.Equal(t, "picked", w.p.current)
}

func TestQuest100FromTheDark_Steps(t *testing.T) {
	shadowsInstallMine(t)
	q := loadShippedQuest(t, "100-from_the_dark.yaml", 100)
	w := newQuestWalker(q).at("start")

	w.notify("command", EventDetails{Command: "sneak", RoomId: reedJetty})
	require.Equal(t, "start", w.p.current, "hiding on the jetty is not hiding in the mine")
	w.notify("command", EventDetails{Command: "sneak", RoomId: timberedDrift})
	require.Equal(t, "hidden", w.p.current)

	w.notify("mob_death", EventDetails{MobId: 9121, RoomId: floodedSump})
	require.Equal(t, "hidden", w.p.current, "a kill with no strike from hiding does not count")
	w.notify("command", EventDetails{Command: "ambush", RoomId: 5249, TargetKind: "mob", MobId: 9118})
	require.Equal(t, "hidden", w.p.current, "the talus is not the mine")
	w.notify("command", EventDetails{Command: "ambush", RoomId: floodedSump, TargetKind: "mob", MobId: 9121})
	require.Equal(t, "ambush", w.p.current)

	w.notify("mob_death", EventDetails{MobId: 9118, RoomId: 5249})
	require.Equal(t, "ambush", w.p.current, "a kill on the talus does not count")
	w.notify("mob_death", EventDetails{MobId: 9121, RoomId: floodedSump})
	require.Equal(t, "kill", w.p.current)
}
