package actions

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// CalcSneakScore computes a sneak score with light-conditional modifier.
// effectiveLit reflects the room visibility from the observer's POV:
// true if the observer makes out anything at all there (see
// CalcSneakScoreVsObserver). Caller is responsible for computing
// effectiveLit per observer.
//
// The conditional modifier:
//   - sneaker emits light, room dark:  0.5x  (beacon in darkness)
//   - sneaker emits light, room lit:   0.85x (blends in with the light)
//   - sneaker dark, room lit:          0.9x  (alert observers)
//   - sneaker dark, room dark:         1.0x  (best stealth, no penalty)
//
// Per-observer evaluation matters: the same sneaker may roll differently
// against different observers in the same room (e.g., an observer with
// night vision or infra reach may perceive a room another reads as dark).
func CalcSneakScore(c *characters.Character, effectiveLit bool) float64 {
	cfg := configs.GetBalanceConfig()

	base := float64(c.Stats.Dexterity.ValueAdj) +
		float64(c.GetSkillLevel(skills.Skullduggery))*float64(cfg.SkillWeight) +
		mutations.GetStealthBonus(c.Mutations)

	emits := c.EmitsLight()

	switch {
	case emits && !effectiveLit:
		base *= float64(cfg.SneakModEmitsLightDarkRoom) // default 0.5
	case emits && effectiveLit:
		base *= float64(cfg.SneakModEmitsLightLitRoom) // default 0.85
	case !emits && effectiveLit:
		base *= float64(cfg.SneakModNoLightLitRoom) // default 0.9
		// else: dark sneaker, dark room — baseline, no modifier applied
	}
	return base
}

// CalcSneakScoreVsObserver is a convenience for the common detection-roll
// case where the caller has sneaker + observer + room in scope. Computes
// effectiveLit per observer: the room counts as lit for an observer that
// makes out anything at all there (messaging.LightBand is not BandDark, the
// band-grained twin of ParticipantSight not being SightNone). That folds in
// blindness, night-vision strength and infra reach; it used to be the room's
// own light OR the nightvision FLAG, which treated an infravision observer in
// a faint room as blind and a nightvision holder in a pitch-dark room as
// seeing (lighting plan 5c).
//
// room is usually messaging.FixedLight(room.LightLevel()): the light is
// invariant across every observer in a room, so a caller looping over
// occupants reads it ONCE above the loop rather than recomposing the room's
// light per occupant. See internal/actions/sneak.go and
// internal/usercommands/go.go for the hoisted call sites. LightBand reads the
// narrow lighting config, not the 424-field Balance copy.
func CalcSneakScoreVsObserver(sneaker, observer *characters.Character, room messaging.RoomVisibility) float64 {
	effectiveLit := messaging.LightBand(observer, room) != messaging.BandDark
	return CalcSneakScore(sneaker, effectiveLit)
}

// CalcDetectionScore is the observer-side score for OPPOSED detection
// contests (spotting a sneaker, sensing a shadower, a player victim's
// after-the-fact notice of a mob thief or planter).
// Linear regime, matching the unified contest formula:
// Perception + rank(search)*SkillWeight.
//
// Task 16 (U6b) split the old CalcSearchScore consumers two ways: every
// opposed contest site moved here; the non-contest / flat-threshold sites
// stayed on CalcSearchScore (see below).
//
// room is the OBSERVER's room, and the score pays the observer's sight ramp
// (lighting plan 5b, messaging.SightMult): the observer's eyes count here. The
// hider's side already folds the room's light in (CalcSneakScoreVsObserver).
// Its defence twin for the theft and plant contests themselves is
// stealVictimScore (steal.go), which prices the victim's or bystander's eyes
// the same way. A nil room is unity; pass combat.SightRoom for a *rooms.Room
// that may be nil.
func CalcDetectionScore(c *characters.Character, room messaging.RoomVisibility) float64 {
	return detectionBase(c) * messaging.SightMult(c, room)
}

// CalcHearingScore is CalcDetectionScore for an observer who sees nothing
// (sight gates close-out, #333, owner 2026-10-08): no sight ramp, since there
// is no sight to price, but Balance.SneakHearingMult instead. The superhearing
// condition flag skips that multiplier. Only the sneak contests ask for it,
// through sneakObserverScore.
func CalcHearingScore(c *characters.Character) float64 {
	base := detectionBase(c)
	if c.HasConditionFlag(conditions.SuperHearing) {
		return base
	}
	return base * float64(configs.GetBalanceConfig().SneakHearingMult)
}

// detectionBase is an observer's detection before any sense prices it:
// Perception + rank(search)*SkillWeight.
func detectionBase(c *characters.Character) float64 {
	return float64(c.Stats.Perception.ValueAdj) +
		float64(c.GetSkillLevel(skills.Search))*
			float64(configs.GetBalanceConfig().SkillWeight)
}

// CalcSearchScore returns the observation score for a character detecting
// hidden things. Higher values mean better detection ability.
// Formula: Perception + SkillMultiplier(search)*25.
//
// DELIBERATELY UNCONVERTED (spec §3.2 Category B — "U6b does not silently
// absorb them"). Its remaining consumers are NOT opposed contests and their
// output rates must not move:
//   - forage.go (foragedSearchScore): forage YIELD, not even a contest
//   - search.go (Search): flat difficulty thresholds tuned x6 against this shape
//   - track.go: static difficulty roll
//
// Opposed detection sites use CalcDetectionScore instead. A regression test
// (TestCalcSearchScore_RegressionFrozen) pins this function's output
// byte-identical to the sqrt-curve values.
func CalcSearchScore(c *characters.Character) float64 {
	return float64(c.Stats.Perception.ValueAdj) +
		combat.SkillMultiplier(c.GetSkillLevel(skills.Search))*25.0
}

// awardRhetoricUse grants Rhetoric progression for a shout-style special move
// (warcry, rally). In combat it always fires; out of combat it fires 50% of the
// time, a soft incentive against spamming it for free progression.
//
// This lives in actions/ so every Actor implementation gets it. Warcry and rally
// previously left progression to their callers — the player wrappers implemented
// it and the mob wrappers did not, so mobs never built Rhetoric from either
// verb.
func awardRhetoricUse(actor Actor, c *characters.Character) {
	// U10b-1 Task 22: won is unconditionally true. A shout is not a contest --
	// warcry and rally roll against nothing and cannot fail -- so there is no
	// losing branch to pay a fraction on. Same treatment venom coat, assess and
	// an instant recipe get.
	//
	// The in-combat / 50% gate is UNCHANGED and is not the firing rule: it is
	// the anti-spam incentive described above, deciding whether the shout
	// counts as practice at all.
	if c.IsInCombat() || util.Rand(100) < 50 {
		actor.AwardResolved(true, c.CandidateFor(string(skills.Rhetoric)))
	}
}
