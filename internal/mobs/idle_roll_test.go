package mobs

import (
	"math"
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// todayIdlePick is the single roll #317 replaced, pinned from master
// 1b3eaca3a (hooks/MobIdle_HandleIdleMobs.go:313-319 and mobs.go:1200-1216):
// one ActivityLevel roll, the 1% forced blank, then a uniform pick over the
// whole pool. "" means the mob looked for trouble instead.
func todayIdlePick(al int, pool []string, randN func(int) int) string {
	if randN(100) >= al {
		return ""
	}
	if randN(100) == 0 {
		return ""
	}
	if len(pool) == 0 {
		return ""
	}
	return pool[randN(len(pool))]
}

// todayRates is today's expected per-round chatter and movement rate.
func todayRates(al int, entries []string) (chatter, movement float64) {
	p := SplitIdlePool(entries)
	if p.Size == 0 {
		return 0, 0
	}
	base := float64(al) / 100 * 0.99
	return base * float64(len(p.Chatter)) / float64(p.Size), base * float64(len(p.Movement)) / float64(p.Size)
}

var parityPools = []struct {
	name    string
	al      int
	entries []string
}{
	{"padded: Sand Elemental 318", 25, []string{
		"emote shifts and reforms, sand cascading in slow spirals", "wander",
		"emote rises from the ground in a column of whirling grit", ""}},
	{"schedule pool, no blanks: Marek 10-22", 15, []string{
		"emote wipes down the bar with a worn rag.", "emote pulls a fresh tap.",
		"say What'll it be, friend?", "emote shouts an order back to the kitchen."}},
	{"movement only: cubegen wanderer", 10, []string{"wander", "", "wander", ""}},
}

var unitScales = IdleRollInput{ChatterScale: 1, MovementScale: 1, ZoneAmbient: 1, ZoneWander: 1, MobWander: 1}

func assertNear(t *testing.T, what string, got, want, rel float64) {
	t.Helper()
	if want == 0 {
		if got != 0 {
			t.Errorf("%s = %v, want 0", what, got)
		}
		return
	}
	if math.Abs(got-want) > rel*want {
		t.Errorf("%s = %.5f, want %.5f within %.0f%%", what, got, want, rel*100)
	}
}

func TestIdleVerb(t *testing.T) {
	for entry, want := range map[string]string{
		"emote waves.":          "emote",
		"  Say hello":           "say",
		"wander;wander;suicide": "wander",
		"portal home;go portal": "portal",
		"":                      "",
		"   ":                   "",
	} {
		if got := IdleVerb(entry); got != want {
			t.Errorf("IdleVerb(%q) = %q, want %q", entry, got, want)
		}
	}
}

func TestClassifyIdleVerb(t *testing.T) {
	if c, ok := ClassifyIdleVerb("sneak"); !ok || c != IdleMovement {
		t.Errorf("sneak = %v,%v, want movement,true", c, ok)
	}
	if c, ok := ClassifyIdleVerb("alchemy"); !ok || c != IdleChatter {
		t.Errorf("alchemy = %v,%v, want chatter,true", c, ok)
	}
	if c, ok := ClassifyIdleVerb("dance"); ok || c != IdleChatter {
		t.Errorf("an unknown verb rolls as chatter and reports unknown, got %v,%v", c, ok)
	}
}

func TestSplitIdlePool_CountsBlanksInTheSize(t *testing.T) {
	p := SplitIdlePool(parityPools[0].entries)
	if len(p.Chatter) != 2 || len(p.Movement) != 1 || p.Size != 4 {
		t.Fatalf("Sand Elemental pool split = %d chatter, %d movement, size %d; want 2, 1, 4", len(p.Chatter), len(p.Movement), p.Size)
	}
}

// At every scale 1.0 each class's chance is today's divided by 0.99: the
// only change is the dropped 1% forced blank.
func TestIdleChances_UnitScalesEqualTodayByFormula(t *testing.T) {
	for _, pc := range parityPools {
		gotC, gotM := IdleChances(pc.al, SplitIdlePool(pc.entries), unitScales)
		wantC, wantM := todayRates(pc.al, pc.entries)
		if math.Abs(gotC*0.99-wantC) > 1e-12 || math.Abs(gotM*0.99-wantM) > 1e-12 {
			t.Errorf("%s: chances %v/%v, today %v/%v (want today = 0.99 x new)", pc.name, gotC, gotM, wantC, wantM)
		}
	}
}

func TestRollIdle_UnitScalesMatchTodayOverManyRounds(t *testing.T) {
	const rounds = 1_000_000
	for _, pc := range parityPools {
		t.Run(pc.name, func(t *testing.T) {
			restore := util.SetRandForTest(317)
			var todayC, todayM int
			for i := 0; i < rounds; i++ {
				cmd := todayIdlePick(pc.al, pc.entries, util.Rand)
				if cmd == "" {
					continue
				}
				if c, _ := ClassifyIdleVerb(IdleVerb(cmd)); c == IdleMovement {
					todayM++
				} else {
					todayC++
				}
			}
			restore()

			m := &Mob{ActivityLevel: pc.al, IdleCommands: pc.entries}
			restore = util.SetRandForTest(318)
			var newC, newM int
			for i := 0; i < rounds; i++ {
				c, mv := m.RollIdle(unitScales, util.Rand)
				if c != "" {
					newC++
				}
				if mv != "" {
					newM++
				}
			}
			restore()

			wantC, wantM := todayRates(pc.al, pc.entries)
			assertNear(t, "today chatter", float64(todayC)/rounds, wantC, 0.03)
			assertNear(t, "today movement", float64(todayM)/rounds, wantM, 0.03)
			assertNear(t, "new chatter", float64(newC)/rounds, wantC/0.99, 0.03)
			assertNear(t, "new movement", float64(newM)/rounds, wantM/0.99, 0.03)
			assertNear(t, "new vs today chatter", float64(newC)/rounds, float64(todayC)/rounds, 0.04)
			assertNear(t, "new vs today movement", float64(newM)/rounds, float64(todayM)/rounds, 0.04)
		})
	}
}

func TestRollIdle_ClassOnlyPools(t *testing.T) {
	big := IdleRollInput{ChatterScale: 4, MovementScale: 4, ZoneAmbient: 1, ZoneWander: 1, MobWander: 1}
	talker := &Mob{ActivityLevel: 100, IdleCommands: []string{"emote hums.", "say Hello."}}
	walker := &Mob{ActivityLevel: 100, IdleCommands: []string{"wander", "sneak"}}
	for i := 0; i < 10000; i++ {
		if _, mv := talker.RollIdle(big, util.Rand); mv != "" {
			t.Fatalf("a mob with only chatter entries moved: %q", mv)
		}
		if c, _ := walker.RollIdle(big, util.Rand); c != "" {
			t.Fatalf("a mob with only movement entries spoke: %q", c)
		}
	}
}

func TestIdleChances_Multipliers(t *testing.T) {
	pool := SplitIdlePool([]string{"wander", "emote hums."})
	scaled := func(mod func(*IdleRollInput)) (float64, float64) {
		in := unitScales
		mod(&in)
		return IdleChances(50, pool, in)
	}
	baseC, baseM := scaled(func(*IdleRollInput) {})
	if baseC != 0.25 || baseM != 0.25 {
		t.Fatalf("AL 50, one of two each: chances %v/%v, want 0.25/0.25", baseC, baseM)
	}
	for _, c := range []struct {
		name         string
		mod          func(*IdleRollInput)
		wantC, wantM float64
	}{
		{"mob wander 0.5", func(in *IdleRollInput) { in.MobWander = 0.5 }, 0.25, 0.125},
		{"zone wander 0.5", func(in *IdleRollInput) { in.ZoneWander = 0.5 }, 0.25, 0.125},
		{"both wanders 0.5", func(in *IdleRollInput) { in.MobWander, in.ZoneWander = 0.5, 0.5 }, 0.25, 0.0625},
		{"zone wander 0", func(in *IdleRollInput) { in.ZoneWander = 0 }, 0.25, 0},
		{"zone ambient 2.0", func(in *IdleRollInput) { in.ZoneAmbient = 2 }, 0.5, 0.25},
		{"zone ambient 0", func(in *IdleRollInput) { in.ZoneAmbient = 0 }, 0, 0.25},
		{"chatter scale 0", func(in *IdleRollInput) { in.ChatterScale = 0 }, 0, 0.25},
		{"movement scale 0", func(in *IdleRollInput) { in.MovementScale = 0 }, 0.25, 0},
	} {
		gotC, gotM := scaled(c.mod)
		if gotC != c.wantC || gotM != c.wantM {
			t.Errorf("%s: chances %v/%v, want %v/%v", c.name, gotC, gotM, c.wantC, c.wantM)
		}
	}
}

func TestRollIdle_LeavesActivityLevelAlone(t *testing.T) {
	m := &Mob{ActivityLevel: 37, IdleCommands: []string{"emote hums.", "wander", ""}}
	for i := 0; i < 1000; i++ {
		m.RollIdle(unitScales, util.Rand)
	}
	if m.ActivityLevel != 37 {
		t.Fatalf("RollIdle changed ActivityLevel to %d; combat AI reads it", m.ActivityLevel)
	}
}

func TestMobWanderMultiplier_YAMLAndValidate(t *testing.T) {
	var zero Mob
	if err := yaml.Unmarshal([]byte("mobid: 1\nwandermultiplier: 0\n"), &zero); err != nil {
		t.Fatal(err)
	}
	if zero.WanderMultiplier == nil || *zero.WanderMultiplier != 0 || zero.WanderScale() != 0 {
		t.Fatalf("wandermultiplier: 0 must load as a set 0 (never), got %v", zero.WanderMultiplier)
	}
	var unset Mob
	if err := yaml.Unmarshal([]byte("mobid: 2\n"), &unset); err != nil {
		t.Fatal(err)
	}
	if unset.WanderMultiplier != nil || unset.WanderScale() != 1 {
		t.Fatalf("an absent wandermultiplier is unset and scales by 1.0, got %v", unset.WanderMultiplier)
	}
	neg := -0.5
	bad := &Mob{MobId: 3, WanderMultiplier: &neg}
	if err := bad.Validate(); err == nil {
		t.Fatal("a negative wandermultiplier must fail Validate (and so refuse boot)")
	}
}

func TestCommandNoSplit_KeepsSemicolons(t *testing.T) {
	m := &Mob{InstanceId: 31701}
	t.Cleanup(func() { events.DrainQueuedInputsForTest(31701) })
	m.CommandNoSplit("ambient mob_chatter emote plunges the iron in; steam rises.")
	if got := events.DrainQueuedInputsForTest(31701); len(got) != 1 || got[0] != "ambient mob_chatter emote plunges the iron in; steam rises." {
		t.Fatalf("CommandNoSplit queued %q, want the whole line as one input", got)
	}
	m.Command("emote plunges the iron in; steam rises.")
	if got := events.DrainQueuedInputsForTest(31701); len(got) != 2 {
		t.Fatalf("Command still splits on ';' (the loot goblin relies on it), got %q", got)
	}
}
