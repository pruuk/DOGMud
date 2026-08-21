package characters

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/progression"
)

// newProgressionTestCharacter builds a character with initialised stats
// (New() rolls stats and calls Validate(), which recalculates derived
// values) for use across the ApplyProgression tests.
func newProgressionTestCharacter(t *testing.T) *Character {
	t.Helper()
	return New()
}

// Ordinary events must go through the SAME path they used before U9, which
// includes tracking the use counter.
func TestApplyProgression_OrdinaryTracksTheUse(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 1 {
		t.Errorf("skill use count = %d, want 1", got)
	}
}

// Bonus events must NOT track. The use count becomes a virtual rank and the
// progression curve DECREASES in rank, so tracking a crit would punish
// critting. Spec 5.2.
func TestApplyProgression_BonusDoesNotTrackTheUse(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassCrit, Multiplier: 2.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 0 {
		t.Errorf("bonus event tracked the use count (%d), which decays progression", got)
	}
}

// Task 15b refined the "bonus events never track" rule by Class: the party
// who DID the exceptional thing (ClassCrit / ClassFumble) must not track,
// since tracking would decay their own future progression -- but the party
// who RECEIVED it (ClassObserved) must, because for a crit-received
// toughening event, tracking is the only thing that ever moves the target
// stat's virtual rank. This test pins both halves directly against the
// applier, independent of the exercise-through-the-real-crit route.
func TestApplyProgression_ObservedBonusTracksButCritDoesNot(t *testing.T) {
	c := newProgressionTestCharacter(t)

	// applyBonusProgression's tracking branch is gated on
	// GamePlay.UseSkillProgression, same as the rest of the bonus tier
	// (mirrors the pre-seam OnCritReceived gate). A Go test binary starts
	// with the zero value (false), so it must be enabled here or the
	// tracking calls this test exists to pin are skipped entirely.
	prevGp := configs.GetGamePlayConfig()
	if err := configs.AddOverlayOverrides(map[string]any{
		"GamePlay.UseSkillProgression": true,
	}); err != nil {
		t.Fatalf("AddOverlayOverrides: %v", err)
	}
	defer func() {
		_ = configs.AddOverlayOverrides(map[string]any{
			"GamePlay.UseSkillProgression": bool(prevGp.UseSkillProgression),
		})
	}()

	observed := []progression.Event{{
		Side: progression.SideDefender, Skill: "dodge",
		Stat: "vitality", Class: progression.ClassObserved, Multiplier: 0.5,
	}}
	c.ApplyProgression(observed, progression.SideDefender, 0, 1)

	if got := c.GetSkillUseCount("dodge"); got != 1 {
		t.Errorf("ClassObserved skill use count = %d, want 1 (observed bonus events must track)", got)
	}
	if got := c.GetStatUseCount("vitality"); got != 1 {
		t.Errorf("ClassObserved stat use count = %d, want 1 (observed bonus events must track)", got)
	}

	crit := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassCrit, Multiplier: 2.0,
	}}
	c.ApplyProgression(crit, progression.SideAttacker, 0, 2)

	if got := c.GetSkillUseCount("weapon-combat"); got != 0 {
		t.Errorf("ClassCrit skill use count = %d, want 0 (the doer must not track)", got)
	}
	if got := c.GetStatUseCount("dexterity"); got != 0 {
		t.Errorf("ClassCrit stat use count = %d, want 0 (the doer must not track)", got)
	}
}

func TestApplyProgression_IgnoresTheOtherSide(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideDefender, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 0 {
		t.Errorf("applied the defender's event to the attacker")
	}
}

// Bonus events dedupe once per round per skill. Ordinary events do not.
func TestApplyProgression_BonusDedupesWithinARound(t *testing.T) {
	c := newProgressionTestCharacter(t)
	bonus := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassCrit, Multiplier: 2.0,
	}}

	ev := bonus[0]
	if !c.claimBonusProgression(ev, 7) {
		t.Fatal("first claim in round 7 was refused")
	}
	if c.claimBonusProgression(ev, 7) {
		t.Error("second claim in the same round was allowed")
	}
	if !c.claimBonusProgression(ev, 8) {
		t.Error("claim in the next round was refused")
	}

	// Same skill, DIFFERENT stat, same round: must NOT collide. A crit received
	// trains the defence skill with the toughening stat while a fumble observed
	// trains it with the defence stat, and keying on skill alone would let the
	// first consume the other's slot.
	other := ev
	other.Stat = "vitality"
	if !c.claimBonusProgression(other, 7) {
		t.Error("a same-skill different-stat event collided with an unrelated claim")
	}
}

func TestApplyProgression_OrdinaryDoesNotDedupe(t *testing.T) {
	c := newProgressionTestCharacter(t)
	ev := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(ev, progression.SideAttacker, 0, 3)
	c.ApplyProgression(ev, progression.SideAttacker, 0, 3)

	if got := c.GetSkillUseCount("weapon-combat"); got != 2 {
		t.Errorf("ordinary events deduped: use count = %d, want 2", got)
	}
}

// An empty skill or stat name must be skipped, not passed on.
// CheckSkillProgression("") takes a roll and a success banners no skill at all.
func TestApplyProgression_EmptyNamesAreSkipped(t *testing.T) {
	c := newProgressionTestCharacter(t)
	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "", Stat: "",
		Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1) // must not panic

	if got := c.GetSkillUseCount(""); got != 0 {
		t.Errorf("tracked an empty skill name")
	}
}

// Spec 5.3: mobs progress through the SAME applier, gated only by the existing
// MobProgressionEnabled / MobProgressionRate knobs. No new gate, no new branch.
// A userId of 0 (which every mob passes) must not suppress the roll -- it only
// suppresses the player-facing banner.
func TestApplyProgression_MobsUseTheSamePath(t *testing.T) {
	c := newProgressionTestCharacter(t)
	c.IsMob = true

	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	c.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if got := c.GetSkillUseCount("weapon-combat"); got != 1 {
		t.Errorf("mob skill use count = %d, want 1 -- mobs must not need a separate path", got)
	}
}

// Spec 5.1 rule 4: U8 lets an exhausted actor autoattack, defend, flee and
// maintain a grapple while DROPPING the skill term from its score. That is a
// combat-effectiveness penalty, not a progression penalty, so the event is
// unchanged.
//
// This is the current behaviour and U9 must not accidentally change it: the
// applier takes an Event, and nothing in Event or in the applier reads a cost
// result. The test pins that absence, because the natural "improvement" someone
// will later propose is to scale progression by whether the action was fully
// paid.
func TestApplyProgression_IgnoresWhetherTheActionWasFullyPaid(t *testing.T) {
	full := newProgressionTestCharacter(t)
	broke := newProgressionTestCharacter(t)
	broke.Stamina = 0

	evs := []progression.Event{{
		Side: progression.SideAttacker, Skill: "weapon-combat",
		Stat: "dexterity", Class: progression.ClassOrdinary, Multiplier: 1.0,
	}}

	full.ApplyProgression(evs, progression.SideAttacker, 0, 1)
	broke.ApplyProgression(evs, progression.SideAttacker, 0, 1)

	if full.GetSkillUseCount("weapon-combat") != broke.GetSkillUseCount("weapon-combat") {
		t.Error("an exhausted actor progressed differently; exhaustion is an effectiveness penalty, not a progression penalty")
	}
}
