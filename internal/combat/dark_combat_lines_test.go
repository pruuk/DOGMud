package combat

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/darkcombat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

func loadDarkCombatForTest(t *testing.T) {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	dir := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud", "narration", "dark-combat")
	if err := darkcombat.LoadFrom(dir); err != nil {
		t.Fatalf("loading dark-combat store: %v", err)
	}
	t.Cleanup(darkcombat.UnloadForTest)
}

// The bucket order is the retired replaceDarknessMessages', except that
// "defended" reads the per-swing Defended, not the sticky DefenseUsed.
func TestDarkOutcome_Buckets(t *testing.T) {
	cases := []struct {
		name string
		se   SwingEvent
		want darkcombat.Outcome
		ok   bool
	}{
		{"double fumble says nothing", SwingEvent{DoubleFumble: true, Fumble: true}, "", false},
		{"fumble", SwingEvent{Fumble: true}, darkcombat.Fumble, true},
		{"crit", SwingEvent{Hit: true, Crit: true, Damage: 9}, darkcombat.Crit, true},
		{"deflected", SwingEvent{Defended: true, Damage: 3}, darkcombat.Deflected, true},
		{"defence crit is defended even with damage", SwingEvent{Defended: true, DefenseCrit: true, Damage: 3}, darkcombat.Defended, true},
		{"defended clean", SwingEvent{Defended: true}, darkcombat.Defended, true},
		{"hit", SwingEvent{Hit: true, Damage: 4}, darkcombat.Hit, true},
		{"miss", SwingEvent{}, darkcombat.Miss, true},
		{"sticky DefenseUsed alone is not a defence", SwingEvent{Hit: true, Damage: 4, DefenseUsed: "dodge"}, darkcombat.Hit, true},
	}
	for _, c := range cases {
		got, ok := darkOutcome(c.se)
		if got != c.want || ok != c.ok {
			t.Errorf("%s: darkOutcome = (%q, %v), want (%q, %v)", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestDarkLines_CategoriesMatchTheRetiredFunction(t *testing.T) {
	loadDarkCombatForTest(t)
	events := []SwingEvent{{Defended: true}, {Hit: true, Damage: 2}, {DoubleFumble: true, Fumble: true}}
	got := darkLines(events, darkcombat.Defender)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2 (the double fumble says nothing)", len(got))
	}
	if got[0].Category != messaging.CategoryDodge || got[1].Category != messaging.CategoryHitMelee {
		t.Fatalf("categories = %v, %v; want CategoryDodge, CategoryHitMelee", got[0].Category, got[1].Category)
	}
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Defended), got[0].Text) {
		t.Fatalf("line 0 %q is not from defender/defended", got[0].Text)
	}
}

func darkResult() *AttackResult {
	return &AttackResult{
		SwingEvents:      []SwingEvent{{Hit: true, Damage: 5}},
		MessagesToSource: []TaggedMessage{{Category: messaging.CategoryHitMelee, Text: "Reading their stance, you strike Shade with practiced precision!"}},
		MessagesToTarget: []TaggedMessage{{Category: messaging.CategoryHitMelee, Text: "Grimwald slashes you with their Rusted Cleaver!"}},
	}
}

func TestHideIdentities_BlindAttackerReadsADarkLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, true) // blind
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), false)

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if len(r.MessagesToSource) != 1 || !slices.Contains(darkcombat.Pool(darkcombat.Attacker, darkcombat.Hit), r.MessagesToSource[0].Text) {
		t.Fatalf("blind attacker read %q, want an attacker/hit dark line", r.MessagesToSource)
	}
	if strings.Contains(r.MessagesToTarget[0].Text, "something") || !strings.Contains(r.MessagesToTarget[0].Text, "Grimwald") {
		t.Fatalf("the sighted defender's line changed: %q", r.MessagesToTarget[0].Text)
	}
}

func TestHideIdentities_DarkRoomDefenderReadsADarkLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, false)
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(0), false) // both blind in the dark

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Hit), r.MessagesToTarget[0].Text) {
		t.Fatalf("defender in the dark read %q, want a defender/hit dark line", r.MessagesToTarget[0].Text)
	}
}

func TestHideIdentities_ShapesReaderKeepsTheComposedLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, true, false, false) // infrared
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(0), false)
	if ctx.sourceOptics != messaging.SightShapes {
		t.Fatalf("control: infrared attacker optics = %v, want SightShapes", ctx.sourceOptics)
	}

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if got := r.MessagesToSource[0].Text; !strings.Contains(got, "practiced precision") || strings.Contains(got, "Shade") {
		t.Fatalf("shapes attacker should read the composed line with the name hidden, got %q", got)
	}
}

func TestHideIdentities_SleeperInALitRoomKeepsTheComposedLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, false)
	def := verdictObserver(t, false, false, true, false) // asleep
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), true)

	r := darkResult()
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	got := r.MessagesToTarget[0].Text
	if slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Hit), got) {
		t.Fatalf("a sleeper in a lit room read a dark line: %q", got)
	}
	if strings.Contains(got, "Grimwald") || !strings.Contains(strings.ToLower(got), "something") {
		t.Fatalf("the sleeper should read the composed line with \"something\", got %q", got)
	}
}

func TestHideIdentities_NoSwingsKeepsTheComposedLines(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, true)
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), false)

	r := darkResult()
	r.SwingEvents = nil
	hideIdentitiesInPersonalLines(r, atk, def, ctx)
	if !strings.Contains(r.MessagesToSource[0].Text, "practiced precision") {
		t.Fatalf("with no swing to bucket the composed line must stay, got %q", r.MessagesToSource[0].Text)
	}
}

// A zero-valued combatContext must never trigger dark lines: dozens of
// existing tests build one by hand.
func TestHideIdentities_ZeroContextIsSighted(t *testing.T) {
	loadDarkCombatForTest(t)
	r := darkResult()
	hideIdentitiesInPersonalLines(r, &characters.Character{Name: "Grimwald"}, &characters.Character{Name: "Shade"}, combatContext{})
	if !strings.Contains(r.MessagesToSource[0].Text, "practiced precision") {
		t.Fatalf("zero context replaced the line: %q", r.MessagesToSource[0].Text)
	}
}
