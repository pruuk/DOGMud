package behaviortree

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/itemlight"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/GoMudEngine/GoMud/internal/uuid"
)

const (
	lightProbeLanternId   = 9921 // a lantern: type light, worn condition 9925
	lightProbeHoodedId    = 9922 // a hooded lantern: worn condition 9927 (adjustable)
	lightProbeFixtureId   = 9923 // a light fixture
	lightProbeDarkFixture = 9924 // a darkness fixture
	lightProbeLanternCond = 9925
	lightProbeSleepCond   = 9926
	lightProbeHoodedCond  = 9927
)

var lightProbeLanternUUID = uuid.UUID{0x51}

// seedItemLightWorld seeds the lantern, a hooded lantern, two fixtures, their
// conditions, and user 1 wearing the lantern in the light slot with its
// record held at full strength, in a loaded room.
func seedItemLightWorld(t *testing.T) *users.UserRecord {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		lightProbeLanternCond: {ConditionId: lightProbeLanternCond, Name: "Probe Lantern Light", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 52}}},
		lightProbeHoodedCond: {ConditionId: lightProbeHoodedCond, Name: "Probe Hooded Light", TriggerCount: 4, RoundInterval: 1,
			Effects: map[conditions.EffectKind]conditions.EffectValue{conditions.EffectLightStrength: {Literal: 54}},
			Flags:   []conditions.Flag{conditions.Adjustable}},
		lightProbeSleepCond: {ConditionId: lightProbeSleepCond, Name: "Probe Sleeping", TriggerCount: 4, RoundInterval: 1,
			Flags: []conditions.Flag{conditions.Sleeping}},
	}))
	t.Cleanup(items.SeedItemsForTest(map[int]*items.ItemSpec{
		lightProbeLanternId: {ItemId: lightProbeLanternId, Name: "Probe Lantern", Type: items.Light, Subtype: items.Wearable,
			WornConditionIds: []int{lightProbeLanternCond}, Behavior: "light_probe"},
		lightProbeHoodedId: {ItemId: lightProbeHoodedId, Name: "Probe Hooded Lantern", Type: items.Light, Subtype: items.Wearable,
			WornConditionIds: []int{lightProbeHoodedCond}, Behavior: "light_probe"},
		lightProbeFixtureId: {ItemId: lightProbeFixtureId, Name: "Probe Lamp Post", Type: items.Object,
			Fixture: items.FixtureLight, Behavior: "light_probe"},
		lightProbeDarkFixture: {ItemId: lightProbeDarkFixture, Name: "Probe Shadow Stone", Type: items.Object,
			Fixture: items.FixtureDarkness, Behavior: "light_probe"},
	}))
	room := newProbeRoom(t)
	room.RoomId = engineProbeRoomId
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{engineProbeRoomId: room}, map[string]*rooms.ZoneConfig{}))
	u := users.NewTestUser(1, "probe", "Probe", 0)
	u.Character.RoomId = engineProbeRoomId
	u.Character.Equipment.Light = items.Item{ItemId: lightProbeLanternId, UUID: lightProbeLanternUUID}
	u.Character.Conditions.AddCondition(lightProbeLanternCond, true)
	t.Cleanup(users.SeedUsersForTest(map[int]*users.UserRecord{1: u}))
	t.Cleanup(ResetItemBTreeStatesForTest())
	t.Cleanup(itemlight.ResetForTest())
	return u
}

func lanternRecord(t *testing.T, c *characters.Character) *conditions.Condition {
	t.Helper()
	recs := c.Conditions.GetConditions(lightProbeLanternCond)
	if len(recs) != 1 {
		t.Fatalf("holder carries %d lantern records, want 1", len(recs))
	}
	return recs[0]
}

func runLightTree(t *testing.T, tree string, s ItemSubject) bool {
	t.Helper()
	LoadItemTreeForTest(t, "light_probe", tree)
	return TryItemBehavior(EventContext{EventType: "item_idle"}, s)
}

var wornLantern = ItemSubject{UUID: lightProbeLanternUUID, ItemId: lightProbeLanternId, UserId: 1, Slot: "light"}

// Rule 9: set_light writes the record's existing trimmed-output state, one
// mechanism with the trim: off is LightOff (fully dark, EmitsLight false), a
// number is LightTrimmed at it, full is LightFull.
func TestSetLightWritesTheWornRecord(t *testing.T) {
	u := seedItemLightWorld(t)
	rec := lanternRecord(t, u.Character)

	if !runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", wornLantern) {
		t.Fatal("set_light off on a worn lantern did not succeed")
	}
	if rec.LightTrim != conditions.LightOff || u.Character.EmitsLight() {
		t.Errorf("after off: LightTrim=%q EmitsLight=%v, want off and false", rec.LightTrim, u.Character.EmitsLight())
	}

	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: 30\n", wornLantern)
	if v, ok := rec.LightNow(conditions.GetConditionSpec(lightProbeLanternCond)); rec.LightTrim != conditions.LightTrimmed || !ok || v != 30 {
		t.Errorf("after 30: LightTrim=%q LightNow=%v,%v, want trimmed at 30", rec.LightTrim, v, ok)
	}

	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: full\n", wornLantern)
	if rec.LightTrim != conditions.LightFull || rec.LightOutput != 0 {
		t.Errorf("after full: LightTrim=%q LightOutput=%v, want full and 0", rec.LightTrim, rec.LightOutput)
	}
}

// YAML 1.1 reads an unquoted off as false; it means off all the same.
func TestSetLightReadsAnUnquotedOff(t *testing.T) {
	u := seedItemLightWorld(t)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: off\n", wornLantern)
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightOff {
		t.Errorf("unquoted off: LightTrim=%q, want off", rec.LightTrim)
	}
}

// Rule 9: TrimLightFor owns adjustable records only, so a move into a new
// room cannot relight a lantern the scheduler put out.
func TestATrimOnEntryDoesNotRelightAScheduledOffLantern(t *testing.T) {
	u := seedItemLightWorld(t)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", wornLantern)
	room := rooms.LoadRoom(engineProbeRoomId)
	room.AddPlayer(u.UserId)
	room.TrimLightFor(u.Character)
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightOff {
		t.Errorf("after an entry trim: LightTrim=%q, want still off", rec.LightTrim)
	}
}

// A5: an equip is a fresh start, full strength until the next tick.
func TestAnEquipIsFullUntilTheNextTick(t *testing.T) {
	u := seedItemLightWorld(t)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", wornLantern)
	lantern := u.Character.Equipment.Light
	u.Character.Equipment.Light = items.Item{}
	if _, worn, reason := u.Character.Wear(lantern); !worn {
		t.Fatalf("re-equipping the lantern refused: %s", reason)
	}
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightFull {
		t.Errorf("after an equip: LightTrim=%q, want full", rec.LightTrim)
	}
}

// Rule 9: on an item that is neither worn nor a fixture both actions fail
// and change nothing.
func TestLightActionsFailForABackpackItem(t *testing.T) {
	u := seedItemLightWorld(t)
	pack := ItemSubject{UUID: uuid.UUID{0x52}, ItemId: lightProbeLanternId, UserId: 1}
	if runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", pack) {
		t.Error("set_light on a backpack item succeeded")
	}
	if runLightTree(t, "tree:\n  type: action\n  do: pulse_light\n  min: 10\n  max: 20\n  period_rounds: 4\n", pack) {
		t.Error("pulse_light on a backpack item succeeded")
	}
	if rec := lanternRecord(t, u.Character); rec.LightTrim != conditions.LightFull {
		t.Errorf("the worn record moved: LightTrim=%q", rec.LightTrim)
	}
}

// Rule 10: a fixture's output lives in internal/itemlight, light or
// darkness by its kind; off records it unlit.
func TestSetLightWritesAFixtureToItemlight(t *testing.T) {
	seedItemLightWorld(t)
	post := ItemSubject{UUID: uuid.UUID{0x53}, ItemId: lightProbeFixtureId, RoomId: engineProbeRoomId, OnFloor: true}
	shadow := ItemSubject{UUID: uuid.UUID{0x54}, ItemId: lightProbeDarkFixture, RoomId: engineProbeRoomId, OnFloor: true}
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: 52\n", post)
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: 40\n", shadow)
	light, dark := itemlight.Terms(engineProbeRoomId)
	if len(light) != 1 || light[0] != 52 || len(dark) != 1 || dark[0] != 40 {
		t.Errorf("Terms = %v, %v; want [52], [40]", light, dark)
	}
	runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: \"off\"\n", post)
	if v, ok := itemlight.Get(engineProbeRoomId, post.UUID); !ok || !math.IsInf(v, -1) {
		t.Errorf("after off the fixture reads %v, %v; want recorded unlit", v, ok)
	}
	if runLightTree(t, "tree:\n  type: action\n  do: set_light\n  level: full\n", post) {
		t.Error("set_light full on a fixture succeeded: a fixture has no full strength of its own")
	}
}

// Rule 11: pulse_light is a triangle wave over period_rounds, from the round
// count alone.
func TestPulseLightIsADeterministicTriangleWave(t *testing.T) {
	for _, c := range []struct {
		round uint64
		want  float64
	}{{0, 20}, {3, 28}, {6, 36}, {9, 28}, {12, 20}, {15, 28}} {
		if got := PulseLightValue(20, 36, 12, c.round); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("round %d: PulseLightValue(20, 36, 12) = %v, want %v", c.round, got, c.want)
		}
	}
	seedItemLightWorld(t)
	stone := ItemSubject{UUID: uuid.UUID{0x55}, ItemId: lightProbeFixtureId, RoomId: engineProbeRoomId, OnFloor: true}
	prev := util.GetRoundCount()
	t.Cleanup(func() { util.SetRoundCountForTest(prev) })
	util.SetRoundCountForTest(6006) // 6006 % 12 == 6: the crest
	runLightTree(t, "tree:\n  type: action\n  do: pulse_light\n  min: 20\n  max: 36\n  period_rounds: 12\n", stone)
	if v, _ := itemlight.Get(engineProbeRoomId, stone.UUID); v != 36 {
		t.Errorf("at the crest the stone reads %v, want 36", v)
	}
}

// R2: holder_asleep reads the Sleeping flag; worn is an equipment slot;
// in_combat is the holder's combat state. A floor item has no holder.
func TestItemConditionsReadTheHolder(t *testing.T) {
	u := seedItemLightWorld(t)
	t.Cleanup(seedTestMob(t, engineProbeMobId, engineProbeInstId, engineProbeRoomId, "Probe Keeper"))
	cases := []struct {
		name string
		tree string
		s    ItemSubject
		want bool
	}{
		{"awake holder", "holder_asleep", wornLantern, false},
		{"worn", "worn", wornLantern, true},
		{"backpack", "worn", ItemSubject{UUID: uuid.UUID{0x56}, ItemId: lightProbeLanternId, UserId: 1}, false},
		{"holder at peace", "in_combat", wornLantern, false},
		{"floor has no holder", "holder_asleep", ItemSubject{UUID: uuid.UUID{0x57}, ItemId: lightProbeFixtureId, RoomId: engineProbeRoomId, OnFloor: true}, false},
	}
	for _, c := range cases {
		if got := runLightTree(t, "tree:\n  type: condition\n  check: "+c.tree+"\n", c.s); got != c.want {
			t.Errorf("%s: %s = %v, want %v", c.name, c.tree, got, c.want)
		}
	}
	u.Character.Conditions.AddCondition(lightProbeSleepCond, false)
	if !runLightTree(t, "tree:\n  type: condition\n  check: holder_asleep\n", wornLantern) {
		t.Error("holder_asleep with the Sleeping flag held: false, want true")
	}
	u.Character.SetAggro(0, engineProbeInstId, characters.DefaultAttack)
	if !runLightTree(t, "tree:\n  type: condition\n  check: in_combat\n", wornLantern) {
		t.Error("in_combat with the holder fighting: false, want true")
	}
}

// Rule 8: item-only nodes refuse in a mob or room tree.
func TestItemOnlyNodesRefuseInMobAndRoomTrees(t *testing.T) {
	for _, bad := range []string{
		"tree:\n  type: action\n  do: set_light\n  level: 52\n",
		"tree:\n  type: condition\n  check: holder_asleep\n",
	} {
		if _, err := LoadTreeFromBytes([]byte(bad)); err == nil || !strings.Contains(err.Error(), "item") {
			t.Errorf("a mob tree compiled %q: err = %v, want an item-only refusal", bad, err)
		}
	}
}

// Rule 8 and X22: one writer per record. A light-writing tree on an item
// whose worn light is adjustable (the trim owns it) refuses at boot.
func TestValidateItemBehaviorsRefusesALightTreeOnAnAdjustableLight(t *testing.T) {
	dir := overrideBTDataFilesDir(t)
	if err := os.MkdirAll(filepath.Join(dir, "behaviors", "items"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "behaviors", "items", "light_probe.yaml"),
		[]byte("tree:\n  type: action\n  do: set_light\n  level: full\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedItemLightWorld(t)
	t.Cleanup(func() { GetEngine().EvictItemTree("light_probe") })
	err := ValidateItemBehaviors()
	if err == nil || !strings.Contains(err.Error(), "item 9922") || !strings.Contains(err.Error(), "adjustable") {
		t.Fatalf("err = %v, want item 9922 refused for an adjustable light", err)
	}
	if strings.Contains(err.Error(), "item 9921") {
		t.Errorf("the plain lantern was refused too: %v", err)
	}
}

// X22 at run time: a tree that reaches a worn item whose light is adjustable
// (the trim owns it) fails and leaves the record untouched, for both actions.
func TestLightActionsLeaveAnAdjustableRecordToTheTrim(t *testing.T) {
	u := seedItemLightWorld(t)
	hoodedUUID := uuid.UUID{0x58}
	u.Character.Equipment.Light = items.Item{ItemId: lightProbeHoodedId, UUID: hoodedUUID}
	u.Character.Conditions.AddCondition(lightProbeHoodedCond, true)
	recs := u.Character.Conditions.GetConditions(lightProbeHoodedCond)
	if len(recs) != 1 {
		t.Fatalf("holder carries %d hooded records, want 1", len(recs))
	}
	rec := recs[0]
	before := *rec
	hooded := ItemSubject{UUID: hoodedUUID, ItemId: lightProbeHoodedId, UserId: 1, Slot: "light"}
	for _, tree := range []string{
		"tree:\n  type: action\n  do: set_light\n  level: \"off\"\n",
		"tree:\n  type: action\n  do: set_light\n  level: 30\n",
		"tree:\n  type: action\n  do: pulse_light\n  min: 10\n  max: 20\n  period_rounds: 4\n",
	} {
		if runLightTree(t, tree, hooded) {
			t.Errorf("%q succeeded on an adjustable record", tree)
		}
		if rec.LightTrim != before.LightTrim || rec.LightOutput != before.LightOutput || rec.Hooded != before.Hooded {
			t.Errorf("%q moved the record: trim %q->%q output %v->%v hooded %v->%v", tree,
				before.LightTrim, rec.LightTrim, before.LightOutput, rec.LightOutput, before.Hooded, rec.Hooded)
		}
	}
}

// The one-writer boot check finds a writer wherever it sits in the tree:
// pulse_light as well as set_light, and nested under composites.
func TestValidateItemBehaviorsFindsTheLightWriterWhereverItSits(t *testing.T) {
	trees := map[string]string{
		"pulse at root": "tree:\n  type: action\n  do: pulse_light\n  min: 10\n  max: 20\n  period_rounds: 4\n",
		"set under selector": "tree:\n  type: selector\n  children:\n" +
			"    - type: condition\n      check: worn\n" +
			"    - type: action\n      do: set_light\n      level: full\n",
		"pulse under sequence under selector": "tree:\n  type: selector\n  children:\n" +
			"    - type: condition\n      check: in_combat\n" +
			"    - type: sequence\n      children:\n" +
			"        - type: condition\n          check: worn\n" +
			"        - type: action\n          do: pulse_light\n          min: 10\n          max: 20\n          period_rounds: 4\n",
	}
	for name, body := range trees {
		t.Run(name, func(t *testing.T) {
			dir := overrideBTDataFilesDir(t)
			if err := os.MkdirAll(filepath.Join(dir, "behaviors", "items"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "behaviors", "items", "light_probe.yaml"), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			seedItemLightWorld(t)
			t.Cleanup(func() { GetEngine().EvictItemTree("light_probe") })
			err := ValidateItemBehaviors()
			if err == nil || !strings.Contains(err.Error(), "item 9922") || !strings.Contains(err.Error(), "adjustable") {
				t.Fatalf("err = %v, want item 9922 refused for an adjustable light", err)
			}
			if strings.Contains(err.Error(), "item 9921") {
				t.Errorf("the plain lantern was refused too: %v", err)
			}
		})
	}
}
