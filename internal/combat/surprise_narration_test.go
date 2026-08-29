package combat

// U10d Task 14 — the VOICE of the surprise-attack redesign.
//
// The mechanics landed first and said nothing. These tests pin the claims the
// copy makes: exactly one swing of an ambush round is marked as the opener, an
// answered opener names the defence that answered it, the opener's lines carry
// messaging.CategorySurpriseAttack (the category's first producer), and every
// authored line fits an 80-column client.

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// ─── width ──────────────────────────────────────────────────────────────────

// renderedWidth counts the columns a client actually prints: ANSI tags are
// markup and occupy none.
func renderedWidth(line string) int {
	n, inTag := 0, false
	for _, r := range line {
		switch {
		case r == '<':
			inTag = true
		case r == '>' && inTag:
			inTag = false
		case !inTag:
			n++
		}
	}
	return n
}

// widthFixture is the worst realistic substitution these lines see.
//
// wideName is 20 characters: the p90 of the 641 authored mob names (median 13,
// max 29). widestBand is the longest string GetDamageDescription can return.
// Measuring with "Selka" would have passed the version of this copy that
// shipped at 95 to 103 columns.
const (
	wideName   = "a Carrion Highland-H" // 20 chars, p90 mob-name length
	widestBand = "devastating wounds"   // longest GetDamageDescription band
)

// assertCopyRules checks the three rules a reviewer cannot see by reading:
// width, no raw numbers, no en/em dashes.
func assertCopyRules(t *testing.T, what, line string) {
	t.Helper()
	if w := renderedWidth(line); w > 80 {
		t.Errorf("%s renders %d columns wide, want at most 80: %q", what, w, line)
	}
	for _, r := range line {
		if r >= '0' && r <= '9' {
			t.Errorf("%s leaks a raw number: %q", what, line)
			break
		}
	}
	if strings.ContainsAny(line, "–—") {
		t.Errorf("%s contains an en or em dash: %q", what, line)
	}
}

// Every composite, every defence, both damage cases, at the p90 name length and
// the widest damage band. THIS is the guard that was missing: the first version
// of assertSurpriseCopy checked digits and dashes and never measured, so copy
// running 95 to 103 columns passed it.
func TestOpeningStrikeDefendedLines_FitEightyColumns(t *testing.T) {
	for _, def := range []DefenseType{DefenseDodge, DefenseParry, DefenseBlock, DefenseNone} {
		for _, band := range []string{"", widestBand} {
			atk, dfn, room := openingStrikeDefendedLines(def, wideName, wideName, band)
			label := string(def)
			if label == "" {
				label = "(fallback)"
			}
			if band == "" {
				label += "/stopped"
			} else {
				label += "/partial"
			}
			assertCopyRules(t, label+" attacker line", string(atk))
			assertCopyRules(t, label+" defender line", string(dfn))
			assertCopyRules(t, label+" room line", string(room))
		}
	}
}

// ─── the composite builder ──────────────────────────────────────────────────

func TestOpeningStrikeDefendedLines_NameTheDefenceThatWon(t *testing.T) {
	cases := []struct {
		defense  DefenseType
		verbYou  string // second person, for the defender's own line
		verbThey string // third person, for the attacker and room lines
	}{
		{DefenseDodge, "dodge", "dodges"},
		{DefenseParry, "parry", "parries"},
		{DefenseBlock, "block", "blocks"},
		// DefenseNone is unreachable by construction on the melee path; if one
		// ever produced it the line must still read as English rather than
		// printing an empty verb.
		{DefenseNone, "deflect", "deflects"},
	}

	for _, tc := range cases {
		label := string(tc.defense)
		if label == "" {
			label = "fallback"
		}

		t.Run(label+"/stopped", func(t *testing.T) {
			atk, dfn, room := openingStrikeDefendedLines(tc.defense, "Rurik", "Selka", "")
			mustContain(t, "attacker", string(atk), "Selka "+tc.verbThey+" your opening blow")
			mustContain(t, "defender", string(dfn), "You "+tc.verbYou+" Rurik's opening blow")
			mustContain(t, "room", string(room), "Selka "+tc.verbThey+" Rurik's opening blow")
			// The stopped wording must NOT promise damage that did not land.
			for _, line := range []string{string(atk), string(dfn), string(room)} {
				if strings.Contains(line, "most of") {
					t.Errorf("a stopped opener claims part of it landed: %q", line)
				}
			}
		})

		t.Run(label+"/partial", func(t *testing.T) {
			atk, dfn, room := openingStrikeDefendedLines(tc.defense, "Rurik", "Selka", "light wounds")
			// "most of" is what keeps the line coherent: the defence won AND
			// something still landed, so neither half may be stated alone.
			mustContain(t, "attacker", string(atk), "Selka "+tc.verbThey+" most of your opening blow")
			mustContain(t, "defender", string(dfn), "You "+tc.verbYou+" most of Rurik's opening blow")
			mustContain(t, "room", string(room), "Selka "+tc.verbThey+" most of Rurik's opening blow")
			for _, line := range []string{string(atk), string(dfn)} {
				if !strings.Contains(line, "light wounds") {
					t.Errorf("partial line %q is missing the damage description", line)
				}
				// The deflection-coherence rule from U6 Task 16b: a line that
				// carries real damage must never also say the swing missed.
				assertNoContradiction(t, "opening strike", line)
			}
			// Room lines carry no damage description anywhere in this package.
			if strings.Contains(string(room), "light wounds") {
				t.Errorf("room line carries a damage description: %q", room)
			}
		})
	}
}

func mustContain(t *testing.T, who, line, phrase string) {
	t.Helper()
	if !strings.Contains(line, phrase) {
		t.Errorf("%s line %q is missing %q", who, line, phrase)
	}
}

// ─── routing through buildAttackMessages ────────────────────────────────────

// A defended opener replaces the line it would otherwise have carried rather
// than adding one, routes through CategorySurpriseAttack, and carries NO
// banner — the prose names the opening blow itself, and a 20-column marker
// would not leave room for the damage band.
func TestBuildAttackMessages_DefendedOpeningStrikeNamesTheDefence(t *testing.T) {
	src, tgt := defenceFixture(1000)
	src.Name = "Rurik"
	tgt.Name = "Selka"
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()

	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2, 15) // dodge, plain defensive win

	// critOnWin true: this IS the opening strike. The defence won it anyway,
	// which is the whole point of the redesign -- the ambush is contestable.
	res := resolveDefenseOutcomeCore(result, best, src, tgt, 2.0, false, false, true)
	if !res.defended {
		t.Fatal("fixture did not produce a defended swing")
	}
	if res.crit {
		t.Fatal("a defended swing must not take the opening-strike crit upgrade")
	}

	roomLinesBefore := len(result.MessagesToSourceRoom)

	ws := weaponSetup{weaponName: "fists", weaponSubType: items.Unarmed}
	sdp := swingDamageParams{dmgMean: 20}

	// Damage > 0, so this is the DEFLECTED sub-case: the room line stays with
	// sendDefenseMessages, exactly as an ordinary deflection does.
	buildAttackMessages(result, src, tgt, ws, sdp,
		5, 0, 0, 0, User, User, "", res.defended, true)

	if len(result.MessagesToSource) != 1 {
		t.Fatalf("attacker got %d lines for the defended opener, want exactly 1: %v",
			len(result.MessagesToSource), result.MessagesToSource)
	}
	if len(result.MessagesToTarget) != 1 {
		t.Fatalf("defender got %d lines for the defended opener, want exactly 1: %v",
			len(result.MessagesToTarget), result.MessagesToTarget)
	}
	if got := len(result.MessagesToSourceRoom); got != roomLinesBefore {
		t.Errorf("buildAttackMessages added a room line on a DEFLECTED opener: %d -> %d (sendDefenseMessages already narrated the room)",
			roomLinesBefore, got)
	}

	atk := result.MessagesToSource[0]
	if !strings.Contains(atk.Text, "dodges most of your opening blow") {
		t.Errorf("attacker line does not narrate as the defence that won: %q", atk.Text)
	}
	if strings.Contains(atk.Text, "SURPRISE ATTACK") {
		t.Errorf("the defended composite must not carry the banner: %q", atk.Text)
	}
	if atk.Category != messaging.CategorySurpriseAttack {
		t.Errorf("attacker line Category = %v, want CategorySurpriseAttack", atk.Category)
	}

	dfn := result.MessagesToTarget[0]
	if !strings.Contains(dfn.Text, "You dodge most of Rurik's opening blow") {
		t.Errorf("defender line does not narrate as the defence that won: %q", dfn.Text)
	}
	if dfn.Category != messaging.CategorySurpriseAttack {
		t.Errorf("defender line Category = %v, want CategorySurpriseAttack", dfn.Category)
	}
}

// The zero-damage deflection is the ONE case that lost a room line when the
// composite first landed. It gets the composite's own room line back, and that
// line is CategorySurpriseAttack, which no verbosity level suppresses — unlike
// the sendDefenseMessages line it would otherwise have depended on, which is
// suppressed at both medium and light.
func TestBuildAttackMessages_ZeroDamageDefendedOpenerKeepsARoomLine(t *testing.T) {
	src, tgt := defenceFixture(1000)
	src.Name = "Rurik"
	tgt.Name = "Selka"
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()

	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2, 15)
	res := resolveDefenseOutcomeCore(result, best, src, tgt, 2.0, false, false, true)
	if !res.defended {
		t.Fatal("fixture did not produce a defended swing")
	}

	roomLinesBefore := len(result.MessagesToSourceRoom)

	ws := weaponSetup{weaponName: "fists", weaponSubType: items.Unarmed}
	sdp := swingDamageParams{dmgMean: 20}

	buildAttackMessages(result, src, tgt, ws, sdp,
		0, 0, 0, 0, User, User, "", res.defended, true)

	if len(result.MessagesToSourceRoom) != roomLinesBefore+1 {
		t.Fatalf("a zero-damage defended opener sent %d room lines, want exactly 1 more than %d: %v",
			len(result.MessagesToSourceRoom)-roomLinesBefore, roomLinesBefore, result.MessagesToSourceRoom)
	}
	room := result.MessagesToSourceRoom[len(result.MessagesToSourceRoom)-1]
	if !strings.Contains(room.Text, "dodges Rurik's opening blow") {
		t.Errorf("room line does not name the defence and the opener: %q", room.Text)
	}
	if room.Category != messaging.CategorySurpriseAttack {
		t.Errorf("room line Category = %v, want CategorySurpriseAttack (nothing suppresses it)", room.Category)
	}
	// And the personal line must use the stopped wording, not claim a fragment
	// landed.
	if strings.Contains(result.MessagesToSource[0].Text, "most of") {
		t.Errorf("a stopped opener claims part of it landed: %q", result.MessagesToSource[0].Text)
	}
}

// A DEFENSIVE CRIT never reaches the composite: hitResolution.defended has one
// assignment site (the partial-deflection branch) and is false on that path.
// sendDefenseMessages keeps its personal AND room lines there and already names
// the defence, so nothing is lost — but the claim is load-bearing enough that
// getting it wrong would silently make the composite look like dead code.
func TestDefensiveCritOpener_IsNotTheCompositePath(t *testing.T) {
	src, tgt := defenceFixture(1000)
	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2*3, 15)

	res := resolveDefenseOutcomeCore(result, best, src, tgt, 2.0, false, false, true)
	if !res.defenseCrit {
		t.Fatal("fixture did not produce a defensive crit")
	}
	if res.defended {
		t.Fatal("a defensive crit must NOT set defended — the composite would then claim a fragment landed")
	}
	if len(result.MessagesToSource) == 0 || len(result.MessagesToTarget) == 0 {
		t.Fatal("a defensive crit must keep its own personal defence lines")
	}
	if len(result.MessagesToSourceRoom) == 0 {
		t.Fatal("a defensive crit must keep its own room line")
	}
}

// The control: an ordinary swing keeps the weapon-band Category and gains no
// banner. Deleting the `if openingStrike` arm makes the tests above fail;
// hard-wiring that arm true makes this one fail.
func TestBuildAttackMessages_OrdinarySwingIsNotBanneredOrRecategorised(t *testing.T) {
	src, tgt := defenceFixture(1000)
	src.Name = "Rurik"
	tgt.Name = "Selka"
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()

	result := &AttackResult{}
	ws := weaponSetup{weaponName: "fists", weaponSubType: items.Unarmed}
	sdp := swingDamageParams{dmgMean: 20}

	buildAttackMessages(result, src, tgt, ws, sdp,
		5, 0, 0, 0, User, User, "", false, false)

	if len(result.MessagesToSource) == 0 {
		t.Fatal("an ordinary swing must still narrate to the attacker")
	}
	for _, m := range result.MessagesToSource {
		if strings.Contains(m.Text, "SURPRISE ATTACK") {
			t.Errorf("an ordinary swing carries the ambush banner: %q", m.Text)
		}
		if m.Category == messaging.CategorySurpriseAttack {
			t.Errorf("an ordinary swing routed through CategorySurpriseAttack: %q", m.Text)
		}
	}
}

// ─── the round-level contract ───────────────────────────────────────────────

// EXACTLY ONE line per ambush round is marked as the opener.
//
// The invariant is stated on the CATEGORY, not on the banner text, because the
// banner is deliberately absent from the answered-opener composite (which names
// itself in prose instead). CategorySurpriseAttack is carried by the opener's
// lines and by nothing else, whatever the outcome, so counting it covers every
// branch with one assertion.
//
// This is the regression test for the shipped-but-unspoken state of U10d, where
// the marker was computed once per round and handed to every swing. Built to
// FAIL if the per-swing flag is replaced by a round-scoped one (mutation-checked
// in both directions).
func TestSurpriseRound_ExactlyOneSwingIsMarkedAsTheOpener(t *testing.T) {
	const rounds = 60
	pinOpeningStrikeBalance(t, 8.0)

	attacker := openingStrikeCombatant(t, "Ambusher")
	defender := openingStrikeCombatant(t, "Mark")
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	plan := buildAttackPlan(attacker, defender)
	if plan.totalSwings < 3 {
		t.Fatalf("plan throws %d swings; with fewer than 3 this test cannot tell a per-swing marker from a round-scoped one",
			plan.totalSwings)
	}

	sawMultiSwingRound := false
	sawBanner := false
	for i := 0; i < rounds; i++ {
		attacker.Stamina = attacker.StaminaMax.Value
		attacker.Health = attacker.HealthMax.Value
		defender.Stamina = defender.StaminaMax.Value
		defender.Health = defender.HealthMax.Value
		if !attacker.Position.IsStanding() {
			attacker.Position = position.NewMachine()
		}
		if !defender.Position.IsStanding() {
			defender.Position = position.NewMachine()
		}

		attacker.SetAggro(0, 1, characters.SurpriseAttack)
		if !attacker.IsInCombat() || attacker.Aggro.Type != characters.SurpriseAttack {
			t.Fatalf("round %d: SetAggro did not take; got %+v", i, attacker.CurrentCombatTarget())
		}

		result, cost := resolveCombatRound(attacker, defender, User, Mob, ctx)
		if cost.Short() {
			t.Fatalf("round %d: admission ran short — the fixture pools are wrong", i)
		}
		if result.SwingsThrown > 1 {
			sawMultiSwingRound = true
		}

		marked := 0
		for _, m := range result.MessagesToSource {
			if m.Category == messaging.CategorySurpriseAttack {
				marked++
			}
			if strings.Contains(m.Text, "SURPRISE ATTACK") {
				sawBanner = true
				if m.Category != messaging.CategorySurpriseAttack {
					t.Fatalf("round %d: a bannered line is not routed through CategorySurpriseAttack: %q", i, m.Text)
				}
			}
		}
		if marked != 1 {
			t.Fatalf("round %d threw %d swings and marked %d of the attacker's lines as the opener, want exactly 1: %v",
				i, result.SwingsThrown, marked, result.MessagesToSource)
		}
	}

	if !sawMultiSwingRound {
		t.Fatal("every round resolved a single swing — this test measured nothing")
	}
	if !sawBanner {
		t.Fatal("no round produced a bannered opener — the banner arm was never exercised")
	}
}

// The control for the round test: a DEFAULT-attack round marks nothing at all.
// Without this, hard-wiring the marker on would still pass the count above only
// by luck of the swing count, and would be invisible here.
func TestPlainRound_NoSwingIsMarkedAsTheOpener(t *testing.T) {
	pinOpeningStrikeBalance(t, 8.0)

	attacker := openingStrikeCombatant(t, "Brawler")
	defender := openingStrikeCombatant(t, "Mark")
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	for i := 0; i < 20; i++ {
		attacker.Stamina = attacker.StaminaMax.Value
		defender.Stamina = defender.StaminaMax.Value
		if !attacker.Position.IsStanding() {
			attacker.Position = position.NewMachine()
		}
		if !defender.Position.IsStanding() {
			defender.Position = position.NewMachine()
		}
		attacker.SetAggro(0, 1, characters.DefaultAttack)

		result, _ := resolveCombatRound(attacker, defender, User, Mob, ctx)
		for _, m := range result.MessagesToSource {
			if strings.Contains(m.Text, "SURPRISE ATTACK") {
				t.Fatalf("round %d: an ordinary attack round bannered a swing: %q", i, m.Text)
			}
			if m.Category == messaging.CategorySurpriseAttack {
				t.Fatalf("round %d: an ordinary attack round marked a swing as the opener: %q", i, m.Text)
			}
		}
	}
}
