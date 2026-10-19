package combat

import (
	"path/filepath"
	"regexp"
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
	got := darkLines(events, darkcombat.Defender, 100)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2 (the double fumble says nothing)", len(got))
	}
	if got[0].Category != messaging.CategoryDodge || got[1].Category != messaging.CategoryHitMelee {
		t.Fatalf("categories = %v, %v; want CategoryDodge, CategoryHitMelee", got[0].Category, got[1].Category)
	}
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Defended), got[0].Text) {
		t.Fatalf("line 0 %q is not from defender/defended", got[0].Text)
	}
	// A hit that dealt damage ends with its wound words (#572), after the
	// pool line.
	hitTag := ` ` + damageTag(GetDamageDescription(2, 100))
	hitLine, tagged := strings.CutSuffix(got[1].Text, hitTag)
	if !tagged {
		t.Fatalf("line 1 %q does not end with the wound words %q", got[1].Text, hitTag)
	}
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Hit), hitLine) {
		t.Fatalf("line 1 %q is not from defender/hit", hitLine)
	}
}

// #572: every incoming hit a blind defender feels carries its wound words, a
// crit's inside its banner; a clean defence carries none, and the attacker's
// dark lines are unchanged.
func TestDarkLines_DefenderHitsCarryWoundWords(t *testing.T) {
	loadDarkCombatForTest(t)
	events := []SwingEvent{
		{Hit: true, Crit: true, Damage: 40},
		{Defended: true, Damage: 3},
		{Defended: true},
	}
	got := darkLines(events, darkcombat.Defender, 100)
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3", len(got))
	}
	critTag := damageTag(GetDamageDescription(40, 100))
	if !strings.HasSuffix(got[0].Text, critTag+` `+darkCritClose) {
		t.Errorf("crit line %q lacks %q inside its banner", got[0].Text, critTag)
	}
	if !strings.HasSuffix(got[1].Text, ` `+damageTag(GetDamageDescription(3, 100))) {
		t.Errorf("deflected line %q lacks its wound words", got[1].Text)
	}
	if strings.Contains(got[2].Text, `fg="damage"`) {
		t.Errorf("clean defence line %q carries wound words", got[2].Text)
	}
	for _, l := range darkLines(events, darkcombat.Attacker, 100) {
		if strings.Contains(l.Text, `fg="damage"`) {
			t.Errorf("attacker dark line %q gained wound words", l.Text)
		}
	}
}

// darkResult builds a one-swing round through the real senders, so the swing
// lines carry the indices hideIdentitiesInPersonalLines swaps on.
func darkResult() *AttackResult {
	r := &AttackResult{SwingEvents: []SwingEvent{{Hit: true, Damage: 5}}}
	r.sendSwingToSource(messaging.CategoryHitMelee, "Reading their stance, you strike Shade with practiced precision!")
	r.sendSwingToTarget(messaging.CategoryHitMelee, "Grimwald slashes you with their Rusted Cleaver!")
	return r
}

func TestHideIdentities_BlindDefenderKeepsThePetLineNameHidden(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, false)
	def := verdictObserver(t, false, false, false, true) // blind
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), false)

	r := darkResult()
	r.SendToTarget(messaging.CategoryHitNaturalSharp, "Grimwald's wolf jumps into the fray and deals a wound to you!")
	r.SendToTarget(messaging.CategorySystem, "Grimwald watches you.")
	hideIdentitiesInPersonalLines(r, atk, def, ctx)

	if len(r.MessagesToTarget) != 3 {
		t.Fatalf("want dark line + pet line + system line, got %d: %v", len(r.MessagesToTarget), r.MessagesToTarget)
	}
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Hit), withoutWoundWords(t, r.MessagesToTarget[0].Text)) {
		t.Fatalf("line 0 %q is not a defender/hit dark line", r.MessagesToTarget[0].Text)
	}
	pet := r.MessagesToTarget[1]
	if pet.Category != messaging.CategoryHitNaturalSharp || !strings.Contains(pet.Text, "jumps into the fray") || strings.Contains(pet.Text, "Grimwald") {
		t.Fatalf("pet line should survive with the attacker hidden, got %+v", pet)
	}
	if strings.Contains(r.MessagesToTarget[2].Text, "Grimwald") {
		t.Fatalf("a kept line leaked the name: %q", r.MessagesToTarget[2].Text)
	}
}

func TestHideIdentities_BlindAttackerKeepsTheDoubleFumbleLine(t *testing.T) {
	loadDarkCombatForTest(t)
	atk := verdictObserver(t, false, false, false, true) // blind
	def := verdictObserver(t, false, false, false, false)
	atk.Name, def.Name = "Grimwald", "Shade"
	ctx := newCombatContext(atk, def, verdictLight(100), false)

	// A double-fumble round: the composer sends the fumble line with SendToSource
	// (not a swing line) and the swing event is a double fumble.
	r := &AttackResult{SwingEvents: []SwingEvent{{Fumble: true, DoubleFumble: true}}}
	r.SendToSource(messaging.CategoryHitMelee, "!!! You trip over Shade and fall flat! !!!")
	hideIdentitiesInPersonalLines(r, atk, def, ctx)

	if len(r.MessagesToSource) != 1 {
		t.Fatalf("want exactly the double-fumble line, got %v", r.MessagesToSource)
	}
	got := r.MessagesToSource[0].Text
	if !strings.Contains(got, "fall flat") || strings.Contains(got, "Shade") {
		t.Fatalf("double-fumble line should be kept with the name hidden, got %q", got)
	}
	for _, o := range []darkcombat.Outcome{darkcombat.Fumble, darkcombat.Hit, darkcombat.Miss} {
		if slices.Contains(darkcombat.Pool(darkcombat.Attacker, o), got) {
			t.Fatalf("a dark line replaced the double-fumble line: %q", got)
		}
	}
}

func TestHideIdentities_BlindDefenderReadsDeflectedAndDefendedPools(t *testing.T) {
	loadDarkCombatForTest(t)
	cases := []struct {
		name    string
		se      SwingEvent
		outcome darkcombat.Outcome
		cat     messaging.Category
	}{
		{"deflected", SwingEvent{Defended: true, Damage: 3}, darkcombat.Deflected, messaging.CategoryHitMelee},
		{"defended", SwingEvent{Defended: true}, darkcombat.Defended, messaging.CategoryDodge},
	}
	for _, c := range cases {
		atk := verdictObserver(t, false, false, false, false)
		def := verdictObserver(t, false, false, false, true) // blind
		atk.Name, def.Name = "Grimwald", "Shade"
		ctx := newCombatContext(atk, def, verdictLight(100), false)

		r := &AttackResult{SwingEvents: []SwingEvent{c.se}}
		r.sendSwingToTarget(c.cat, "composed swing line")
		hideIdentitiesInPersonalLines(r, atk, def, ctx)
		if len(r.MessagesToTarget) != 1 {
			t.Fatalf("%s: got %d lines, want 1", c.name, len(r.MessagesToTarget))
		}
		m := r.MessagesToTarget[0]
		poolLine := m.Text
		if c.se.Damage > 0 {
			poolLine = withoutWoundWords(t, m.Text)
		}
		if !slices.Contains(darkcombat.Pool(darkcombat.Defender, c.outcome), poolLine) {
			t.Errorf("%s: %q is not from defender/%s", c.name, m.Text, c.outcome)
		}
		if m.Category != c.cat {
			t.Errorf("%s: category = %v, want %v", c.name, m.Category, c.cat)
		}
	}
}

func TestSwapSwingLines(t *testing.T) {
	m := func(s string) TaggedMessage { return TaggedMessage{Text: s} }
	texts := func(ms []TaggedMessage) []string {
		var out []string
		for _, x := range ms {
			out = append(out, x.Text)
		}
		return out
	}
	msgs := []TaggedMessage{m("pet"), m("swing1"), m("short"), m("swing2"), m("tail")}
	dark := []TaggedMessage{m("D1"), m("D2")}

	got := texts(swapSwingLines(msgs, []int{1, 3}, dark))
	want := []string{"pet", "D1", "D2", "short", "tail"}
	if !slices.Equal(got, want) {
		t.Errorf("swap = %v, want %v", got, want)
	}
	// Unsorted indices still anchor on the earliest swing line.
	got = texts(swapSwingLines(msgs, []int{3, 1}, dark))
	if !slices.Equal(got, want) {
		t.Errorf("unsorted swap = %v, want %v", got, want)
	}
	if got := swapSwingLines(msgs, nil, dark); !slices.Equal(texts(got), texts(msgs)) {
		t.Errorf("empty index changed the slice: %v", texts(got))
	}
	if got := swapSwingLines(msgs, []int{1, 3}, nil); !slices.Equal(texts(got), texts(msgs)) {
		t.Errorf("empty dark changed the slice: %v", texts(got))
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
		t.Fatalf("blind attacker read %+v, want an attacker/hit dark line", r.MessagesToSource)
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
	if !slices.Contains(darkcombat.Pool(darkcombat.Defender, darkcombat.Hit), withoutWoundWords(t, r.MessagesToTarget[0].Text)) {
		t.Fatalf("defender in the dark read %q, want a defender/hit dark line", r.MessagesToTarget[0].Text)
	}
}

// withoutWoundWords is a dark defender line with the wound words a damaging
// swing ends with (#572) taken off, so it can be found in its pool. A line
// without them fails the test: every damaging swing must carry them.
func withoutWoundWords(t *testing.T, text string) string {
	t.Helper()
	stripped := stripWoundWords(text)
	if stripped == text {
		t.Fatalf("dark defender line %q has no wound words", text)
	}
	return stripped
}

// woundWordsTag matches the wound words damageTag appends, with its space.
var woundWordsTag = regexp.MustCompile(` \(<ansi fg="damage">[^<]*</ansi>\)`)

// stripWoundWords takes the wound words off a line wherever they stand (a
// crit's sit inside its banner) and returns any other line unchanged.
func stripWoundWords(text string) string {
	return woundWordsTag.ReplaceAllString(text, ``)
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
