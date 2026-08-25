package combat

import (
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// U6 Task 16b — deflection narration coherence.
//
// The Phase B playtest caught a deflected swing (defence won, partial damage)
// narrating through TWO channels that contradicted each other: the defence
// pool's personal lines implied total avoidance ("coming up out of reach!"),
// and buildAttackMessages then bolted a glancing suffix onto a miss-band line
// ("misses you completely! The blow still catches you."). The fix gives each
// participant exactly ONE line naming the defence and the damage it let
// through; the room keeps only the defence line.

// Phrases from the old contradictory composites. None may appear on a
// deflected swing's personal lines.
var contradictionPhrases = []string{
	"miss",
	"nothing but air",
	"does nothing",
	"out of reach",
}

func assertNoContradiction(t *testing.T, viewer, line string) {
	t.Helper()
	lower := strings.ToLower(line)
	for _, phrase := range contradictionPhrases {
		if strings.Contains(lower, phrase) {
			t.Errorf("%s line contains contradictory phrase %q: %q", viewer, phrase, line)
		}
	}
}

// A non-crit defensive win (partial == true) must keep its side effects and
// room narration, but send NO personal lines — buildAttackMessages owns those.
func TestSendDefenseMessages_PartialSendsRoomLinesOnly(t *testing.T) {
	src, tgt := defenceFixture(1000)

	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2, 15)

	res := resolveDefenseOutcomeCore(result, best, src, tgt, 2.0, false, false, false)

	if !res.defended {
		t.Fatal("fixture did not produce a deflection")
	}
	if result.DefenseUsed != DefenseDodge {
		t.Errorf("DefenseUsed = %q, want %q — the partial path must keep this side effect", result.DefenseUsed, DefenseDodge)
	}
	if len(result.MessagesToSource) != 0 {
		t.Errorf("partial defence sent %d personal lines to the attacker, want 0: %v",
			len(result.MessagesToSource), result.MessagesToSource)
	}
	if len(result.MessagesToTarget) != 0 {
		t.Errorf("partial defence sent %d personal lines to the defender, want 0: %v",
			len(result.MessagesToTarget), result.MessagesToTarget)
	}
	if len(result.MessagesToSourceRoom) == 0 {
		t.Error("partial defence must still send the room line")
	}
}

// Regression guard for the new partial parameter: a defensive CRIT fully
// negates the swing and must keep its personal defence lines exactly as
// before.
func TestSendDefenseMessages_DefensiveCritKeepsPersonalLines(t *testing.T) {
	src, tgt := defenceFixture(1000)

	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2*3, 15)

	res := resolveDefenseOutcomeCore(result, best, src, tgt, 2.0, false, false, false)

	if !res.defenseCrit {
		t.Fatal("fixture did not produce a defensive crit")
	}
	if len(result.MessagesToSource) == 0 {
		t.Error("a defensive crit must still narrate to the attacker")
	}
	if len(result.MessagesToTarget) == 0 {
		t.Error("a defensive crit must still narrate to the defender")
	}
	if len(result.MessagesToSourceRoom) == 0 {
		t.Error("a defensive crit must still narrate to the room")
	}
}

// The full deflected flow: resolution (which stamps DefenseUsed and sends the
// room line) followed by buildAttackMessages. Each participant must end with
// exactly one line for the swing, naming the defence verb and a damage
// description, with none of the old contradictory phrasing; the room must get
// no second line from buildAttackMessages.
func TestBuildAttackMessages_DeflectedSwingOneCoherentLinePerViewer(t *testing.T) {
	src, tgt := defenceFixture(1000)
	src.Name = "Rurik"
	tgt.Name = "Selka"
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()

	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2, 15) // dodge, plain defensive win

	res := resolveDefenseOutcomeCore(result, best, src, tgt, 2.0, false, false, false)
	if !res.defended {
		t.Fatal("fixture did not produce a deflection")
	}

	roomLinesBefore := len(result.MessagesToSourceRoom)

	ws := weaponSetup{weaponName: "fists", weaponSubType: items.Unarmed}
	sdp := swingDamageParams{dmgMean: 20}
	const dmg = 5

	buildAttackMessages(result, src, tgt, ws, sdp,
		dmg, 0, 0, 0, User, User, "", res.defended, false)

	if len(result.MessagesToSource) != 1 {
		t.Fatalf("attacker got %d lines for the deflected swing, want exactly 1: %v",
			len(result.MessagesToSource), result.MessagesToSource)
	}
	if len(result.MessagesToTarget) != 1 {
		t.Fatalf("defender got %d lines for the deflected swing, want exactly 1: %v",
			len(result.MessagesToTarget), result.MessagesToTarget)
	}
	if got := len(result.MessagesToSourceRoom); got != roomLinesBefore {
		t.Errorf("buildAttackMessages added room lines on a deflected swing: %d -> %d (the defence line already covers the room)",
			roomLinesBefore, got)
	}

	dmgDesc := GetDamageDescription(dmg, tgt.HealthMax.Value)

	atkLine := result.MessagesToSource[0].Text
	if !strings.Contains(atkLine, "dodges") {
		t.Errorf("attacker line does not name the defence verb: %q", atkLine)
	}
	if !strings.Contains(atkLine, dmgDesc) {
		t.Errorf("attacker line does not carry the damage description %q: %q", dmgDesc, atkLine)
	}
	assertNoContradiction(t, "attacker", atkLine)

	defLine := result.MessagesToTarget[0].Text
	if !strings.Contains(defLine, "dodge") {
		t.Errorf("defender line does not name the defence verb: %q", defLine)
	}
	if !strings.Contains(defLine, dmgDesc) {
		t.Errorf("defender line does not carry the damage description %q: %q", defLine, defLine)
	}
	assertNoContradiction(t, "defender", defLine)
}

// The composite builder itself: every defence gets its verb pair, and an
// empty DefenseUsed (no path produces one today) falls back to the neutral
// "turn aside" phrasing rather than an empty verb.
func TestDeflectedSwingLines_VerbsAndFallback(t *testing.T) {
	cases := []struct {
		defense     DefenseType
		wantAtkVerb string
		wantDefVerb string
	}{
		{DefenseDodge, "dodges your swing", "You dodge"},
		{DefenseParry, "parries your swing", "You parry"},
		{DefenseBlock, "blocks your swing", "You block"},
		{DefenseNone, "turns your swing aside", "You turn"},
	}

	for _, tc := range cases {
		t.Run(string(tc.defense)+"_fallback_included", func(t *testing.T) {
			atk, def := deflectedSwingLines(tc.defense, "Rurik", "Selka", "light wounds")

			if !strings.Contains(string(atk), tc.wantAtkVerb) {
				t.Errorf("attacker line %q missing %q", atk, tc.wantAtkVerb)
			}
			if !strings.Contains(string(def), tc.wantDefVerb) {
				t.Errorf("defender line %q missing %q", def, tc.wantDefVerb)
			}
			for _, line := range []string{string(atk), string(def)} {
				if !strings.Contains(line, "light wounds") {
					t.Errorf("line %q missing the damage description", line)
				}
				assertNoContradiction(t, "composite", line)
			}
		})
	}
}
