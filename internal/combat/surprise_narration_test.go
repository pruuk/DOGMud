package combat

// U10d Task 14 — the VOICE of the surprise-attack redesign.
//
// The mechanics landed first and said nothing. These tests pin the three
// claims the copy makes: exactly one swing of an ambush round is bannered, an
// answered opener names the defence that answered it, and the opener's lines
// carry messaging.CategorySurpriseAttack (the category's first producer).

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state/position"
)

// ─── the composite builder ──────────────────────────────────────────────────

func TestOpeningStrikeDefendedLines_NameTheDefenceThatWon(t *testing.T) {
	cases := []struct {
		defense     DefenseType
		wantAtkVerb string
		wantDefVerb string
	}{
		{DefenseDodge, "dodges your surprise attack", "You dodge"},
		{DefenseParry, "parries your surprise attack", "You parry"},
		{DefenseBlock, "blocks your surprise attack", "You block"},
		// DefenseNone is the same unreachable-by-construction fallback
		// deflectedSwingLines carries: no melee path produces it, and if one
		// ever did the line must still read as English rather than printing an
		// empty verb.
		{DefenseNone, "turns aside your surprise attack", "You turn aside"},
	}

	for _, tc := range cases {
		t.Run(string(tc.defense)+"/stopped", func(t *testing.T) {
			atk, def := openingStrikeDefendedLines(tc.defense, "Rurik", "Selka", "")
			assertSurpriseCopy(t, string(atk), tc.wantAtkVerb)
			assertSurpriseCopy(t, string(def), tc.wantDefVerb)
			// The stopped wording must NOT promise damage that did not land.
			if strings.Contains(string(atk), "lands") || strings.Contains(string(def), "lands") {
				t.Errorf("a fully stopped opener claims damage landed: %q / %q", atk, def)
			}
		})

		t.Run(string(tc.defense)+"/partial", func(t *testing.T) {
			atk, def := openingStrikeDefendedLines(tc.defense, "Rurik", "Selka", "light wounds")
			assertSurpriseCopy(t, string(atk), tc.wantAtkVerb)
			assertSurpriseCopy(t, string(def), tc.wantDefVerb)
			for _, line := range []string{string(atk), string(def)} {
				if !strings.Contains(line, "light wounds") {
					t.Errorf("partial line %q is missing the damage description", line)
				}
				// The deflection-coherence rule from U6 Task 16b: a line that
				// carries real damage must never also say the swing missed.
				assertNoContradiction(t, "opening strike", line)
			}
		})
	}
}

// assertSurpriseCopy checks one authored line for the phrase it must carry and
// for the two project-wide copy rules that a reviewer cannot see by reading:
// no raw numbers, and no en/em dashes.
func assertSurpriseCopy(t *testing.T, line, wantPhrase string) {
	t.Helper()
	if !strings.Contains(line, wantPhrase) {
		t.Errorf("line %q is missing %q", line, wantPhrase)
	}
	for _, r := range line {
		if r >= '0' && r <= '9' {
			t.Errorf("line %q leaks a raw number", line)
			break
		}
	}
	if strings.ContainsAny(line, "–—") {
		t.Errorf("line %q contains an en or em dash", line)
	}
}

// ─── routing through buildAttackMessages ────────────────────────────────────

// A defended opener replaces the line it would otherwise have carried rather
// than adding one, and routes through CategorySurpriseAttack.
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

	buildAttackMessages(result, src, tgt, ws, sdp,
		5, 0, 0, 0, User, User, surpriseAttackBanner, res.defended, true)

	if len(result.MessagesToSource) != 1 {
		t.Fatalf("attacker got %d lines for the defended opener, want exactly 1: %v",
			len(result.MessagesToSource), result.MessagesToSource)
	}
	if len(result.MessagesToTarget) != 1 {
		t.Fatalf("defender got %d lines for the defended opener, want exactly 1: %v",
			len(result.MessagesToTarget), result.MessagesToTarget)
	}
	if got := len(result.MessagesToSourceRoom); got != roomLinesBefore {
		t.Errorf("buildAttackMessages added a room line on a defended opener: %d -> %d (sendDefenseMessages already narrated the room)",
			roomLinesBefore, got)
	}

	atk := result.MessagesToSource[0]
	if !strings.Contains(atk.Text, "dodges your surprise attack") {
		t.Errorf("attacker line does not narrate as the defence that won: %q", atk.Text)
	}
	if !strings.Contains(atk.Text, surpriseAttackBanner) {
		t.Errorf("attacker line is not bannered as the opening strike: %q", atk.Text)
	}
	if atk.Category != messaging.CategorySurpriseAttack {
		t.Errorf("attacker line Category = %v, want CategorySurpriseAttack", atk.Category)
	}

	def := result.MessagesToTarget[0]
	if !strings.Contains(def.Text, "You dodge Rurik's surprise attack") {
		t.Errorf("defender line does not narrate as the defence that won: %q", def.Text)
	}
	if def.Category != messaging.CategorySurpriseAttack {
		t.Errorf("defender line Category = %v, want CategorySurpriseAttack", def.Category)
	}
}

// The control: an ordinary swing keeps the weapon-band Category and gains no
// banner. Deleting the `if openingStrike` arm makes the test above fail;
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

// The banner marks ONE swing, not the round.
//
// This is the regression test for the shipped-but-unspoken state of U10d: the
// prefix was computed once per round and handed to every swing, so a four-swing
// ambush printed four identical banners and nothing told the player which line
// was the opener. Built to FAIL if the per-swing prefix is replaced by a
// round-scoped one (mutation-checked in both directions when written).
func TestSurpriseRound_ExactlyOneSwingIsBannered(t *testing.T) {
	const rounds = 40
	pinOpeningStrikeBalance(t, 8.0)

	attacker := openingStrikeCombatant(t, "Ambusher")
	defender := openingStrikeCombatant(t, "Mark")
	ctx := combatContext{sourceCanSee: true, targetCanSee: true}

	plan := buildAttackPlan(attacker, defender)
	if plan.totalSwings < 3 {
		t.Fatalf("plan throws %d swings; with fewer than 3 this test cannot tell a per-swing banner from a round-scoped one",
			plan.totalSwings)
	}

	sawMultiSwingRound := false
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
		if attacker.Aggro == nil || attacker.Aggro.Type != characters.SurpriseAttack {
			t.Fatalf("round %d: SetAggro did not take; got %+v", i, attacker.Aggro)
		}

		result, cost := resolveCombatRound(attacker, defender, User, Mob, ctx)
		if cost.Short() {
			t.Fatalf("round %d: admission ran short — the fixture pools are wrong", i)
		}
		if result.SwingsThrown > 1 {
			sawMultiSwingRound = true
		}

		bannered := 0
		for _, m := range result.MessagesToSource {
			if strings.Contains(m.Text, "SURPRISE ATTACK") {
				bannered++
				if m.Category != messaging.CategorySurpriseAttack {
					t.Fatalf("round %d: the bannered line is not routed through CategorySurpriseAttack: %q", i, m.Text)
				}
			}
		}
		if bannered != 1 {
			t.Fatalf("round %d threw %d swings and bannered %d of the attacker's lines, want exactly 1: %v",
				i, result.SwingsThrown, bannered, result.MessagesToSource)
		}
	}

	if !sawMultiSwingRound {
		t.Fatal("every round resolved a single swing — this test measured nothing")
	}
}

// The control for the round test: a DEFAULT-attack round banners nothing at
// all. Without this, hard-wiring the banner on would still pass the count above
// only by luck of the swing count, and would be invisible here.
func TestPlainRound_NoSwingIsBannered(t *testing.T) {
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
		}
	}
}
