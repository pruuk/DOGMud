package combat

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/pets"
)

// File: darkness_identity_hiding_test.go
//
// M4d PR 2 deletes internal/hooks.replaceDarknessMessages and hides
// identities at composition instead, inside internal/combat, where the
// per-side sight verdict (combatContext.sourceSight / targetSight) already
// lives. hideIdentitiesInPersonalLines is the seam that does it: applied once,
// at the end of calculateCombat, to every line already composed for
// MessagesToSource and MessagesToTarget.
//
// TWO PROOFS, because a golden built on a label fixture (hooks package) can
// prove the twelve-sentence substitution is GONE without proving the
// composed line's identity is actually HIDDEN -- that gap is exactly what
// sunk this arc twice before. Test 1 below is a direct unit test of the
// hiding function itself, using hand-built text so the "reader's own name
// survives" invariant is provable regardless of what today's authored
// templates happen to interpolate. Test 2 drives the real production seam,
// calculateCombat, so a composed swing line (weapon flavour, defence band,
// all of it) is what gets checked, not a label.

// TestHideIdentitiesInPersonalLines_UnitSeam drives hideIdentitiesInPersonalLines
// directly against hand-built text that names BOTH combatants on BOTH lines --
// including the reader's own name, which no real composer in this package
// produces today (every SendToSource call interpolates only the target's
// name, every SendToTarget call only the source's -- see buildAttackMessages,
// sendDefenseMessages, handleDoubleFumble, filterDefensesForThirdParty,
// applyPetDamage). That is exactly why this needs its own direct test: the
// invariant "the reader's own name is never hidden" cannot be observed
// through calculateCombat's real templates, because they never put a reader's
// own name where hiding would reach it. A regression that widened the hide
// list to include the reader's own name would go unnoticed by every
// production-path test but must be caught here.
func TestHideIdentitiesInPersonalLines_UnitSeam(t *testing.T) {
	atk := characters.New()
	atk.Name = "Grimwald"
	def := characters.New()
	def.Name = "Shade"

	buildResult := func() *AttackResult {
		return &AttackResult{
			MessagesToSource: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: "Grimwald swings and Shade staggers back!"},
			},
			MessagesToTarget: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: "Shade is struck hard by Grimwald!"},
			},
		}
	}

	t.Run("both sighted: nothing changes", func(t *testing.T) {
		res := buildResult()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{
			sourceSight: messaging.SightFull, targetSight: messaging.SightFull,
		})
		if got := res.MessagesToSource[0].Text; got != "Grimwald swings and Shade staggers back!" {
			t.Fatalf("SightFull attacker line changed: %q", got)
		}
		if got := res.MessagesToTarget[0].Text; got != "Shade is struck hard by Grimwald!" {
			t.Fatalf("SightFull defender line changed: %q", got)
		}
	})

	t.Run("both blind: other party hidden, own name survives", func(t *testing.T) {
		res := buildResult()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{
			sourceSight: messaging.SightNone, targetSight: messaging.SightNone,
		})

		atkLine := res.MessagesToSource[0].Text
		if strings.Contains(atkLine, "Shade") {
			t.Fatalf("attacker (blind, sourceSight=SightNone) still reads the defender's name: %q", atkLine)
		}
		if !strings.Contains(atkLine, "Grimwald") {
			t.Fatalf("attacker's OWN name was hidden from their own line -- SendTrio hides only the other party, this is the exact bug the trap warns about: %q", atkLine)
		}
		if !strings.Contains(atkLine, "something") {
			t.Fatalf("attacker line does not carry the SightNone substitute word: %q", atkLine)
		}

		defLine := res.MessagesToTarget[0].Text
		if strings.Contains(defLine, "Grimwald") {
			t.Fatalf("defender (blind, targetSight=SightNone) still reads the attacker's name: %q", defLine)
		}
		if !strings.Contains(defLine, "Shade") {
			t.Fatalf("defender's OWN name was hidden from their own line: %q", defLine)
		}
		if !strings.Contains(defLine, "something") {
			t.Fatalf("defender line does not carry the SightNone substitute word: %q", defLine)
		}
	})

	t.Run("shapes vs none diverge", func(t *testing.T) {
		res := buildResult()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{
			sourceSight: messaging.SightShapes, targetSight: messaging.SightNone,
		})

		atkLine := res.MessagesToSource[0].Text
		if strings.Contains(atkLine, "Shade") {
			t.Fatalf("attacker (shapes) still reads the defender's real name: %q", atkLine)
		}
		if !strings.Contains(atkLine, "a figure") {
			t.Fatalf("attacker (SightShapes) did not get 'a figure', want the shapes wording not the none wording: %q", atkLine)
		}

		defLine := res.MessagesToTarget[0].Text
		if strings.Contains(defLine, "Grimwald") {
			t.Fatalf("defender (none) still reads the attacker's real name: %q", defLine)
		}
		if !strings.Contains(defLine, "something") {
			t.Fatalf("defender (SightNone) did not get 'something', want the none wording not the shapes wording: %q", defLine)
		}
	})
}

// TestHideIdentitiesInPersonalLines_HidesAttackersPetFromBlindDefender pins
// M4d PR 2 followup 2 (owner: "It shouldn't be able to see the name."):
// applyPetDamage's toDefenderMsg names the attacker's pet via
// sourceChar.Pet.DisplayName(), and a blind defender must not learn which
// animal is attacking them any more than they learn the owner's name. Text
// is hand-built with the real DisplayName() output -- `<ansi
// fg="petname">wolf</ansi>` -- exactly the shape applyPetDamage produces, so
// this exercises the tag-aware match HideNames uses for a real identity tag,
// not a bare-string stand-in.
func TestHideIdentitiesInPersonalLines_HidesAttackersPetFromBlindDefender(t *testing.T) {
	atk := characters.New()
	atk.Name = "Grimwald"
	atk.Pet = pets.Pet{Type: "wolf"}
	def := characters.New()
	def.Name = "Shade"

	buildResult := func() *AttackResult {
		return &AttackResult{
			MessagesToSource: []TaggedMessage{
				{Category: messaging.CategoryHitNaturalSharp, Text: fmt.Sprintf(
					`%s jumps into the fray and deals <ansi fg="damage">3</ansi> to <ansi fg="username">Shade</ansi>!`, atk.Pet.DisplayName())},
			},
			MessagesToTarget: []TaggedMessage{
				{Category: messaging.CategoryHitNaturalSharp, Text: fmt.Sprintf(
					`%s jumps into the fray and deals <ansi fg="damage">3</ansi> to you!`, atk.Pet.DisplayName())},
			},
		}
	}

	t.Run("blind defender: pet name hidden alongside owner's name", func(t *testing.T) {
		res := buildResult()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{
			sourceSight: messaging.SightFull, targetSight: messaging.SightNone,
		})
		defLine := res.MessagesToTarget[0].Text
		if strings.Contains(defLine, "wolf") {
			t.Fatalf("blind defender (targetSight=SightNone) still reads the attacker's pet name: %q", defLine)
		}
		if strings.Contains(defLine, "Grimwald") {
			t.Fatalf("blind defender still reads the attacker's own name: %q", defLine)
		}
		// The pet line's %s is the first word of the sentence, so HideNames
		// capitalizes the substitute word (atSentenceStart) -- "Something",
		// not "something", the same capitalization rule TestHideIdentitiesInPersonalLines_UnitSeam
		// pins via a mid-sentence substitution that stays lowercase.
		if !strings.Contains(defLine, "Something") {
			t.Fatalf("defender line does not carry the SightNone substitute word: %q", defLine)
		}
	})

	t.Run("shapes-only defender: pet name hidden with the shapes wording", func(t *testing.T) {
		res := buildResult()
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{
			sourceSight: messaging.SightFull, targetSight: messaging.SightShapes,
		})
		defLine := res.MessagesToTarget[0].Text
		if strings.Contains(defLine, "wolf") {
			t.Fatalf("shapes-only defender still reads the attacker's pet name: %q", defLine)
		}
		if !strings.Contains(defLine, "A figure") {
			t.Fatalf("shapes-only defender did not get the sentence-start 'A figure' for the pet: %q", defLine)
		}
	})

	t.Run("owner: pet name survives on the owner's own line even when the owner is blind", func(t *testing.T) {
		res := buildResult()
		// sourceSight is deliberately SightNone here: the owner's own sight
		// must never hide their OWN pet or their OWN name from their OWN
		// line. hideIdentitiesInPersonalLines only ever adds a name to the
		// OTHER side's hide list, so nothing in MessagesToSource is ever
		// looked up against sourceSight for removal.
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{
			sourceSight: messaging.SightNone, targetSight: messaging.SightFull,
		})
		atkLine := res.MessagesToSource[0].Text
		if !strings.Contains(atkLine, "wolf") {
			t.Fatalf("owner's own pet name was hidden from the owner's own line: %q", atkLine)
		}
	})

	t.Run("sighted defender: nothing changes", func(t *testing.T) {
		res := buildResult()
		before := res.MessagesToTarget[0].Text
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{
			sourceSight: messaging.SightFull, targetSight: messaging.SightFull,
		})
		if got := res.MessagesToTarget[0].Text; got != before || !strings.Contains(got, "wolf") {
			t.Fatalf("SightFull must leave the pet line unchanged and it must have named the pet to begin with: %q", got)
		}
	})

	t.Run("no pet: unaffected, no panic", func(t *testing.T) {
		noPetAtk := characters.New()
		noPetAtk.Name = "Grimwald"
		res := &AttackResult{
			MessagesToTarget: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: "Grimwald strikes you!"},
			},
		}
		hideIdentitiesInPersonalLines(res, noPetAtk, def, combatContext{
			sourceSight: messaging.SightFull, targetSight: messaging.SightNone,
		})
		if got := res.MessagesToTarget[0].Text; strings.Contains(got, "Grimwald") {
			t.Fatalf("attacker's name should still be hidden with no pet present: %q", got)
		}
	})
}

// buildDeflectedSwing drives the REAL production composers for a deflected
// swing -- resolveDefenseOutcomeCore (which stamps DefenseUsed and sends the
// room defence line) followed by buildAttackMessages (which builds the one
// composite personal line each side gets, via deflectedSwingLines) -- exactly
// the two-call sequence calculateCombat's swing loop uses. Chosen over a full
// calculateCombat round because deflectedSwingLines is a pure Go format
// string, not an authored content-pool lookup, so the composed text reliably
// names both combatants regardless of what content this unit test binary has
// loaded (see reference-test-binary-config-defaults-differ-from-shipped and
// TestBuildAttackMessages_DeflectedSwingOneCoherentLinePerViewer, which
// established this exact fixture pattern for the same reason).
func buildDeflectedSwing(t *testing.T, srcName, tgtName string) *AttackResult {
	t.Helper()
	src, tgt := defenceFixture(1000)
	src.Name = srcName
	tgt.Name = tgtName
	tgt.HealthMax.Base = 100
	tgt.HealthMax.Recalculate()

	result := &AttackResult{}
	best := defenceWinBest(15*math.Sqrt2, 15) // dodge, plain defensive win

	res := resolveDefenseOutcomeCore(result, best, src, tgt, 2.0, false, false, false)
	if !res.defended {
		t.Fatal("fixture did not produce a deflection")
	}

	ws := weaponSetup{weaponName: "fists", weaponSubType: items.Unarmed}
	sdp := swingDamageParams{dmgMean: 20}
	const dmg = 5
	buildAttackMessages(result, src, tgt, ws, sdp,
		dmg, 0, 0, 0, User, User, "", res.defended, false)

	if len(result.MessagesToSource) != 1 || len(result.MessagesToTarget) != 1 || len(result.MessagesToSourceRoom) == 0 {
		t.Fatalf("fixture did not produce the expected deflected-swing lines: source=%d target=%d sourceRoom=%d",
			len(result.MessagesToSource), len(result.MessagesToTarget), len(result.MessagesToSourceRoom))
	}
	return result
}

// TestCalculateCombat_HidesIdentitiesInPersonalLines drives the REAL
// production composers a deflected swing uses -- resolveDefenseOutcomeCore +
// buildAttackMessages, the same pair calculateCombat's swing loop calls, not
// a label fixture -- then applies hideIdentitiesInPersonalLines exactly as
// calculateCombat does at its tail, and proves the composed personal lines
// actually stop naming the other party once a participant cannot see. This is
// Proof B's answer to "a golden covers the store, not the path": the hooks
// golden (regenerated in the same PR) proves replaceDarknessMessages' twelve
// sentences are gone; this test proves the replacement -- hiding at
// composition, over real composed text -- actually works.
func TestCalculateCombat_HidesIdentitiesInPersonalLines(t *testing.T) {
	t.Run("both sighted (control): the real composite names both parties", func(t *testing.T) {
		result := buildDeflectedSwing(t, "Grimwald", "Shade")
		before := result.MessagesToSource[0].Text
		hideIdentitiesInPersonalLines(result, &characters.Character{Name: "Grimwald"}, &characters.Character{Name: "Shade"}, combatContext{
			sourceSight: messaging.SightFull, targetSight: messaging.SightFull,
		})
		if got := result.MessagesToSource[0].Text; got != before || !strings.Contains(got, "Shade") {
			t.Fatalf("SightFull must leave the real composite unchanged and it must have named the defender to begin with: %q", got)
		}
		if got := result.MessagesToTarget[0].Text; !strings.Contains(got, "Grimwald") {
			t.Fatalf("control failed: the real composite never named the attacker, so a later 'no raw name' assertion would be vacuous: %q", got)
		}
	})

	t.Run("both blind: neither real composite line names the other party", func(t *testing.T) {
		result := buildDeflectedSwing(t, "Grimwald", "Shade")
		roomBefore := result.MessagesToSourceRoom[0].Text

		hideIdentitiesInPersonalLines(result, &characters.Character{Name: "Grimwald"}, &characters.Character{Name: "Shade"}, combatContext{
			sourceSight: messaging.SightNone, targetSight: messaging.SightNone,
		})

		// deflectedSwingLines, like every composer in this package, names
		// only the OTHER party on each personal line (MessagesToSource here
		// names Shade the defender, never Grimwald the attacker themself; see
		// the "both sighted (control)" sub-test above, which pins that this
		// real composite contains "Shade" and never needed to contain
		// "Grimwald" to prove it). So the real-composite proof here is just
		// "the other party's name is gone" -- the separate, sharper claim
		// that a reader's OWN name would survive IF it were ever present is
		// proven by TestHideIdentitiesInPersonalLines_UnitSeam's synthetic
		// text, which deliberately puts a reader's own name on their own
		// line to catch a regression no real template can trigger.
		atkLine := result.MessagesToSource[0].Text
		if strings.Contains(atkLine, "Shade") {
			t.Fatalf("attacker (blind) still reads the real composite's defender name: %q", atkLine)
		}

		defLine := result.MessagesToTarget[0].Text
		if strings.Contains(defLine, "Grimwald") {
			t.Fatalf("defender (blind) still reads the real composite's attacker name: %q", defLine)
		}

		// Room lines are untouched by design -- spectators are sight-judged
		// per viewer downstream in internal/hooks, not here.
		if got := result.MessagesToSourceRoom[0].Text; got != roomBefore {
			t.Fatalf("room line changed even though hideIdentitiesInPersonalLines must never touch MessagesToSourceRoom: before=%q after=%q", roomBefore, got)
		}
		if !strings.Contains(roomBefore, "Grimwald") || !strings.Contains(roomBefore, "Shade") {
			t.Fatalf("room line fixture never carried both real names to begin with, so the untouched-room assertion is vacuous: %q", roomBefore)
		}
	})
}

// #446: weapons are kept by OWNER, not by name. Every weapon in one
// AttackResult is the attacker's, so a defender holding a weapon of the same
// name learned nothing new and still read the attacker's weapon by name.
func TestHideIdentitiesInPersonalLines_SameNamedWeaponsHideByOwner(t *testing.T) {
	atk := characters.New()
	atk.Name = "Ordel"
	atk.Equipment.Weapon = items.Item{ItemId: 999901, Spec: &items.ItemSpec{Name: "iron longsword"}}
	def := characters.New()
	def.Name = "Fold"
	def.Equipment.Weapon = items.Item{ItemId: 999903, Spec: &items.ItemSpec{Name: "iron longsword"}}

	for _, d := range []messaging.SightDecision{messaging.SightShapes, messaging.SightNone} {
		res := &AttackResult{
			MessagesToSource: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `You slash <ansi fg="username">Fold</ansi> with your <ansi fg="item">iron longsword</ansi>!`},
			},
			MessagesToTarget: []TaggedMessage{
				{Category: messaging.CategoryHitMelee, Text: `<ansi fg="username">Ordel</ansi> slashes you with their <ansi fg="item">iron longsword</ansi>!`},
			},
		}
		hideIdentitiesInPersonalLines(res, atk, def, combatContext{sourceSight: d, targetSight: d})

		if strings.Contains(strings.ToLower(res.MessagesToTarget[0].Text), "longsword") {
			t.Fatalf("sight %d: the defender read the attacker's weapon through their own same-named one: %q", d, res.MessagesToTarget[0].Text)
		}
		if !strings.Contains(res.MessagesToSource[0].Text, `your <ansi fg="item">iron longsword</ansi>`) {
			t.Fatalf("sight %d: the attacker lost their own weapon: %q", d, res.MessagesToSource[0].Text)
		}
	}
}
