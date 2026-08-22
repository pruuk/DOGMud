package characters

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/banner"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// progressionRollDenominator is the integer resolution every progression roll
// uses. It was 10,000, which quantised any chance below 0.01% to a threshold of
// zero — a stat in that state could never progress again, and two of a live
// character's six were there (strength 3.98e-05, dexterity 9.25e-12 against
// users/3.yaml and the shipped config). util.Rand is already integer, so the
// finer resolution costs nothing.
//
// Resolution alone does not remove the seal, it only moves it further out the
// curve; Balance.ProgressionChanceFloor is what removes it. Both are needed —
// at 10,000 the shipped floor of 1e-5 would itself quantise to zero.
const progressionRollDenominator = 1000000

// skillNameMap maps progression context names to actual skill tags.
// Can be used to alias legacy names to new skill tags.
var skillNameMap = map[string]string{}

// MobStatGainMessages contains room-visible messages when a mob gains a stat
// through use-based progression. Format verbs: %s = mob name (pre-wrapped in ansi).
var MobStatGainMessages = map[string]string{
	"strength":   `<ansi fg="mobname">%s</ansi> seems to grow more powerful.`,
	"dexterity":  `<ansi fg="mobname">%s</ansi> moves with increasing swiftness.`,
	"perception": `<ansi fg="mobname">%s</ansi>'s eyes sharpen with awareness.`,
	"vitality":   `<ansi fg="mobname">%s</ansi> looks tougher than before.`,
	"willpower":  `<ansi fg="mobname">%s</ansi> radiates a more focused presence.`,
	"charisma":   `<ansi fg="mobname">%s</ansi> projects a more commanding aura.`,
}

// resolveSkillName maps a progression context name to the actual skill tag.
func resolveSkillName(name string) string {
	if mapped, ok := skillNameMap[name]; ok {
		return mapped
	}
	return name
}

// CalculateProgressionChance returns the probability (0.0–1.0) that a
// skill/stat progression event fires at the given virtual rank.
// The curve is exponential decay: ~30% at rank 0, ~1.5% near the soft cap,
// and very small above the soft cap.
func CalculateProgressionChance(currentRank int, softCap int) float64 {
	b := configs.GetBalanceConfig()
	if softCap <= 0 {
		softCap = 1
	}
	base := float64(b.BaseProgressionChance)
	decayBelow := float64(b.ProgressionDecayBelowCap)
	decayAbove := float64(b.ProgressionDecayAboveCap)
	if currentRank <= 0 {
		return base
	}
	ratio := float64(currentRank) / float64(softCap)
	if currentRank <= softCap {
		return base * math.Exp(-decayBelow*ratio)
	}
	// Above soft cap: very hard, continues exponential decay
	aboveCapFloor := base * math.Exp(-decayBelow) // value at exactly the cap
	return aboveCapFloor * math.Exp(-decayAbove*(ratio-1.0))
}

// skillProgressionChance computes the probability (0.0-1.0) that a skill
// progression roll succeeds for skillName, given a bonus multiplier. This is the
// single source of truth for the chance CheckSkillProgression rolls against,
// mirroring statProgressionChance.
//
// Extracted from CheckSkillProgression in U10b-0 Phase C. An inline copy is what
// let the admin dashboard's chance display drift from production, and Phase E
// needs to call the real expression rather than a duplicate.
//
// A mob at or past its skill cap, or with mob progression disabled, returns 0.
func (c *Character) skillProgressionChance(skillName string, bonusMultiplier float64) float64 {
	b := configs.GetBalanceConfig()
	actualSkillName := resolveSkillName(skillName)

	// Mob-specific gating
	if c.IsMob {
		if !bool(b.MobProgressionEnabled) {
			return 0
		}
		if lvl, ok := c.Skills[actualSkillName]; ok && lvl >= int(b.MobSkillTrainingCap) {
			return 0 // hard cap
		}
		bonusMultiplier *= float64(b.MobProgressionRate)
	}

	// Rank is the skill LEVEL. It used to be a use counter normalised by the
	// skill's progression multiplier, which existed to stop frequently-fired
	// skills exhausting the curve faster than rare ones. Keying on the level
	// makes difficulty depend on how far the skill has actually come, which is
	// frequency-independent by construction, so the normalisation goes with it.
	//
	// The old anti-exploit floor ("if the level exceeds the soft cap, use it as
	// the rank") is deleted for the same reason: it existed because a counter
	// could be low while the level was high, which cannot happen when the rank
	// IS the level.
	//
	// GetProgressionMultiplier survives below as a per-skill PACE dial.
	virtualRank := c.Skills[actualSkillName]

	// Phase 24.2: Apply mutation skill progression multiplier
	mutSkillMult := 1.0 + mutations.GetSkillProgressionMultiplier(c.Mutations)
	// Phase 25.3: Skill Attunement buff doubles skill progression chance
	buffSkillMult := 1.0
	if c.HasBuffFlag(buffs.SkillProgress) {
		buffSkillMult = 2.0
	}

	// Mobs decay against their own soft cap: they fight far more often than
	// players, so sharing the player curve would leave them flat for too long.
	softCap := int(b.SkillSoftCap)
	if c.IsMob {
		softCap = int(b.MobSkillSoftCap)
	}
	chance := CalculateProgressionChance(virtualRank, softCap) *
		bonusMultiplier * skills.GetProgressionMultiplier(skillName) *
		mutSkillMult * buffSkillMult
	if chance > 1.0 {
		chance = 1.0
	}
	// A skill may become vanishingly slow but never sealed. `chance > 0`
	// preserves the mob hard-cap short-circuit above, which returns a genuine
	// zero meaning "may not progress at all".
	if floor := float64(b.ProgressionChanceFloor); chance > 0 && chance < floor {
		chance = floor
	}
	return chance
}

// CheckSkillProgression rolls against the progression chance for a skill.
// If the roll succeeds, the skill level is increased. Returns true if
// progression fired (skill actually gained), false otherwise.
// bonusMultiplier scales the chance (e.g. 2.0 for critical successes).
func (c *Character) CheckSkillProgression(skillName string, userId int, bonusMultiplier float64) bool {
	chance := c.skillProgressionChance(skillName, bonusMultiplier)
	if chance <= 0 {
		return false
	}
	virtualRank := c.Skills[resolveSkillName(skillName)]

	// Roll: chance is 0.0–1.0, scaled to the integer roll resolution.
	threshold := int(chance * progressionRollDenominator)
	roll := util.Rand(progressionRollDenominator)

	if roll < threshold {
		mudlog.Debug("Progression", "check", "skill", "result", "PROGRESS", "skill", skillName, "rank", virtualRank, "chance", fmt.Sprintf("%.4g%%", chance*100), "roll", roll, "threshold", threshold, "character", c.Name)
		actualSkill := resolveSkillName(skillName)

		if skills.SkillExists(actualSkill) {
			increased := c.IncreaseSkill(actualSkill)
			if userId > 0 {
				var tier *banner.TierChange
				if increased {
					newLevel := c.Skills[actualSkill]
					prevTier := skills.GetSkillRankDescription(newLevel - 1)
					newTier := skills.GetSkillRankDescription(newLevel)
					if prevTier != newTier {
						tier = &banner.TierChange{From: prevTier, To: newTier}
					}
				}
				msg := banner.Format(banner.Skill, actualSkill, tier)
				events.AddToQueue(events.Message{UserId: userId, Text: msg + "\n"})
			}
		} else {
			if userId > 0 {
				msg := banner.Format(banner.Skill, skillName, nil)
				events.AddToQueue(events.Message{UserId: userId, Text: msg + "\n"})
			}
		}
		return true
	}
	return false
}

// statProgressionChance computes the probability (0.0-1.0) that a stat
// progression roll succeeds for statName, given a bonus multiplier. This is
// the single source of truth for the chance CheckStatProgression rolls
// against. The faucet test calls it directly too, so the test pins the exact
// expression production rolls rather than a hand-rolled duplicate that could
// silently drift from it.
//
// A mob at or past MobStatTrainingCap, or with mob progression disabled, returns 0
// (the hard-cap short-circuit CheckStatProgression used to perform itself).
func (c *Character) statProgressionChance(statName string, bonusMultiplier float64) float64 {
	b := configs.GetBalanceConfig()

	// Mob-specific gating
	if c.IsMob {
		if !bool(b.MobProgressionEnabled) {
			return 0
		}
		// Cap on GAINS, not on value. The old value cap was asymmetric: a mob
		// authored at base 250 could gain nothing while one at 180 could gain
		// 20, purely because of how big it was written. Training is
		// gains-since-spawn for mobs too since Phase C, so this is now the
		// same rule players would get if they had one.
		if c.GetStatTraining(statName) >= int(b.MobStatTrainingCap) {
			return 0 // hard cap
		}
		bonusMultiplier *= float64(b.MobProgressionRate)
	}

	// Rank is TRAINED POINTS, not uses and not the final value. Uses measured
	// how often you swung, which punished frequency; the value included Base
	// (the baseline you started with) and Mods (equipment), so gear made a stat
	// HARDER to train and a big mob was harder to train than a small one.
	// Training is exactly what progression has added, so difficulty depends only
	// on gains actually made.
	//
	// The old anti-exploit value floor is deleted along with the counter: it
	// existed because a counter could be low while the value was high, which
	// cannot happen when the rank IS the gains.
	virtualRank := c.GetStatTraining(statName)
	// Phase 24.2: Apply mutation stat progression multiplier
	mutStatMult := 1.0 + mutations.GetStatProgressionMultiplier(c.Mutations)
	// Phase 39.1: Per-stat progression multiplier from config
	statMult := b.GetStatProgressionMultiplier(statName)
	chance := CalculateProgressionChance(virtualRank, int(b.StatProgressionSoftCap)) * bonusMultiplier * mutStatMult * statMult * float64(b.StatProgressionRate)
	if chance > 1.0 {
		chance = 1.0
	}
	// Floor last, after every multiplier. Progression is meant to become
	// asymptotically slow, never impossible, and the integer roll quantises
	// anything smaller than one part in progressionRollDenominator down to
	// "never". Applied here rather than at the roll site so every caller of this
	// expression -- CheckStatProgression, OnCritReceived, the faucet test --
	// sees the same guarantee.
	//
	// `chance > 0` matters: the mob gates above return a hard 0 meaning "this
	// mob may not progress at all", and the floor must not resurrect that.
	if floor := float64(b.ProgressionChanceFloor); chance > 0 && chance < floor {
		chance = floor
	}
	return chance
}

// CheckStatProgression rolls against the progression chance for a stat.
// If the roll succeeds, the stat's Training value is increased by 1.
// Returns true if progression fired (stat actually gained), false otherwise.
func (c *Character) CheckStatProgression(statName string, userId int, bonusMultiplier float64) bool {
	chance := c.statProgressionChance(statName, bonusMultiplier)
	if chance <= 0 {
		return false
	}

	virtualRank := c.GetStatTraining(statName)

	threshold := int(chance * progressionRollDenominator)
	roll := util.Rand(progressionRollDenominator)

	if roll < threshold {
		mudlog.Debug("Progression", "check", "stat", "result", "PROGRESS", "stat", statName, "rank", virtualRank, "chance", fmt.Sprintf("%.4g%%", chance*100), "roll", roll, "threshold", threshold, "character", c.Name)
		if c.IncreaseStat(statName, 1) {
			if userId > 0 {
				msg := banner.Format(banner.Stat, statName, nil)
				events.AddToQueue(events.Message{UserId: userId, Text: msg + "\n"})
			}
			return true
		}
	}
	return false
}

// TrackSkillUse increments the usage counter for a specific skill.
func (c *Character) TrackSkillUse(skillName string) {
	if c.SkillUseCount == nil {
		c.SkillUseCount = make(map[string]int)
	}
	c.SkillUseCount[skillName]++
}

// TrackStatUse increments the usage counter for a specific stat.
func (c *Character) TrackStatUse(statName string) {
	if c.StatUseCount == nil {
		c.StatUseCount = make(map[string]int)
	}
	c.StatUseCount[statName]++
}

// OnStatUse is called whenever a character uses a stat in gameplay.
// Tracks usage and, if progression is enabled, rolls for stat advancement.
// Returns true if the stat actually increased.
func (c *Character) OnStatUse(statName string, userId int) bool {
	c.TrackStatUse(statName)
	mudlog.Debug("Progression", "event", "stat_use", "stat", statName, "character", c.Name)

	if configs.GetGamePlayConfig().UseSkillProgression {
		return c.CheckStatProgression(statName, userId, 1.0)
	}
	return false
}

// OnSkillUse is called whenever a character uses a skill in gameplay.
// Tracks usage and, if progression is enabled, rolls for skill advancement.
// Also auto-tracks and progresses the skill's primary governing stat.
// Returns true if the skill actually increased.
func (c *Character) OnSkillUse(skillName string, userId int) bool {
	return c.OnSkillUseScaled(skillName, userId, 1.0)
}

// OnSkillUseScaled is like OnSkillUse but accepts a bonus multiplier that
// scales the progression chance. Used for difficulty-scaled progression
// where harder spells/crafts reward proportionally more skill growth.
func (c *Character) OnSkillUseScaled(skillName string, userId int, bonusMultiplier float64) bool {
	c.TrackSkillUse(skillName)
	mudlog.Debug("Progression", "event", "skill_use", "skill", skillName, "bonus", fmt.Sprintf("%.2f", bonusMultiplier), "character", c.Name)

	// Mutation-graph drift: cluster-relevant skill use nudges affinity.
	if clusters := mutations.ClustersForSkill(skillName); clusters != nil {
		amt := float64(configs.GetBalanceConfig().MutationAffinityPerSkillUse)
		for _, cl := range clusters {
			c.AddClusterAffinity(cl, amt)
		}
	}

	gained := false
	if configs.GetGamePlayConfig().UseSkillProgression {
		gained = c.CheckSkillProgression(skillName, userId, bonusMultiplier)
	}

	// Auto-track and progress the skill's primary governing stat
	if primaryStat := skills.GetSkillPrimaryStat(skillName); primaryStat != "" {
		c.OnStatUse(primaryStat, userId)
	}

	// Emit SkillUsed event for quest engine and other listeners
	if userId > 0 {
		events.AddToQueue(events.SkillUsed{
			UserId: userId,
			Skill:  skills.SkillTag(skillName),
		})
	}

	return gained
}

// AddClusterAffinity accumulates mutation-graph drift affinity toward a
// cluster. Lazily initializes the map.
func (c *Character) AddClusterAffinity(cluster string, amount float64) {
	if c.ClusterAffinity == nil {
		c.ClusterAffinity = make(map[string]float64)
	}
	c.ClusterAffinity[cluster] += amount
}

// DriftFromCombat grants drift affinity toward a cluster from a combat behavior
// (tanking, evading, controlling) — at most once per cluster per combat round,
// mirroring once-per-action skill drift so many sub-events in a round can't
// spam it. round is util.GetRoundCount(). This is what makes Ironhide/Trickster/
// Weaver reachable, since no skill maps to them.
func (c *Character) DriftFromCombat(cluster string, round uint64) {
	if c.combatDriftRound == nil {
		c.combatDriftRound = make(map[string]uint64)
	}
	if round > 0 && c.combatDriftRound[cluster] >= round {
		return // already granted this round
	}
	c.combatDriftRound[cluster] = round
	c.AddClusterAffinity(cluster, float64(configs.GetBalanceConfig().MutationAffinityPerCombatEvent))
}

// OnFirstMobKill is called when a player kills a mob type for the first time.
// Triggers a bonus combat skill progression check.
func (c *Character) OnFirstMobKill(userId int) {
	mudlog.Debug("Progression", "event", "first_mob_kill", "character", c.Name)

	if configs.GetGamePlayConfig().UseSkillProgression {
		bonus := float64(configs.GetBalanceConfig().CritProgressionBonus)
		if c.CheckSkillProgression("combat", userId, bonus) {
			msg := `<ansi fg="magenta">***</ansi> Defeating a new foe hones your combat instincts! <ansi fg="magenta">***</ansi>`
			events.AddToQueue(events.Message{UserId: userId, Text: msg + "\n"})
		}
	}
}

// OnCritReceived is called when a character takes a critical hit.
// Triggers stat progression for the stat related to the damage channel:
//
//	physical crit → vitality (body toughens from surviving hard blows)
//	magical crit  → willpower (mind hardens against arcane trauma)
//	rhetoric crit → charisma (ego steels itself after being shaken)
//
// U9: this routes through CheckStatProgression, NOT CheckRegenProgression.
// CheckRegenProgression is correct for OnRegenTick, whose chance comes from
// pool depletion, but it never calls CalculateProgressionChance and never
// applies StatProgressionRate -- so as a crit handler it was a flat chance at
// every virtual rank. That is rank-independent stat progression driven by being
// hit, which is structurally the fyttyn vitality exploit (see
// internal/migration/0.16.0.go), and U6's margin-driven crit rates made it
// worse by pushing crit rates against an outclassed defender toward certainty.
//
// The multiplier is ObservedCritProgressionBonus: receiving is worth strictly
// less than doing.
func (c *Character) OnCritReceived(damageChannel string, userId int) {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return
	}

	statName := ToughenStatFor(damageChannel)
	if statName == "" {
		return
	}

	mult := float64(configs.GetBalanceConfig().ObservedCritProgressionBonus)
	if mult <= 0 {
		return
	}

	// TRACK BEFORE ROLLING. This is the second half of the faucet fix and it is
	// the load-bearing half. CheckStatProgression derives virtualRank from
	// GetStatUseCount, and NOTHING in the game tracks vitality use -- zero
	// production OnStatUse("vitality") sites, and CheckRegenProgression never
	// tracked either. So vitality's rank sat at 0 until its VALUE passed 150,
	// which meant its progression chance was constant no matter how much
	// vitality the character already had. Moving to the decayed curve without
	// this would have bought a flat 25% -> 13.5% cut and left the
	// rank-independence, which is the actual fyttyn mechanism, fully intact.
	c.TrackStatUse(statName)
	c.CheckStatProgression(statName, userId, mult)
}

// ToughenStatFor maps a damage channel to the stat that TAKING a critical hit
// in that channel trains. Exported because the contest seam needs the same
// mapping to fill Outcome.ToughenStat, and two copies would drift.
func ToughenStatFor(damageChannel string) string {
	switch damageChannel {
	case "physical":
		return "vitality"
	case "magical":
		return "willpower"
	case "conviction":
		return "charisma"
	}
	return ""
}

// regenDamperFactor is the rank-based damping factor CheckRegenProgression
// applies on top of its pool-depletion chance. It uses the SAME curve the
// rest of progression uses rather than a second one, normalised against
// BaseProgressionChance so it is exactly 1.0 at rank 0 -- a fresh character's
// passive growth is completely unchanged, and this is a pure veteran nerf.
//
// Without this, CheckRegenProgression's chance is derived from pool depletion
// alone and never falls no matter how much the stat has already been used,
// which is how a character grinds a stat at low health forever. That is the
// fyttyn mechanism (see internal/migration/0.16.0.go), and vitality is hit
// hardest because regen is its DOMINANT source of progression. Not its only
// one: ToughenStatFor("physical") routes taken-crit progression to vitality
// via CritProgressionBonus / ObservedCritProgressionBonus, both live since
// 81061c6b4 (2026-08-19). Vitality also takes TWO regen rolls per tick, not
// one, because it appears in both the Health and the Stamina stat lists in
// NewRound_AutoHeal.go.
//
// A BaseProgressionChance of 0 (progression fully disabled) returns 0 rather
// than dividing by zero.
func (c *Character) regenDamperFactor(statName string) float64 {
	b := configs.GetBalanceConfig()
	base := float64(b.BaseProgressionChance)
	if base <= 0 {
		return 0
	}
	virtualRank := c.GetStatTraining(statName)
	return CalculateProgressionChance(virtualRank, int(b.StatProgressionSoftCap)) / base
}

// CheckRegenProgression rolls against a given chance for stat progression,
// applying mob gating, per-stat multipliers, mutation bonuses, and the
// rank-based regen damper (regenDamperFactor).
// Used by OnRegenTick for regen-based stat progression.
func (c *Character) CheckRegenProgression(statName string, userId int, chance float64) {
	b := configs.GetBalanceConfig()

	// Mob-specific gating
	if c.IsMob {
		if !bool(b.MobProgressionEnabled) {
			return
		}
		// Gains cap, same rule as statProgressionChance. Missing this site
		// would leave the regen faucet -- which is what actually drives a
		// mob's vitality -- capped by value while everything else moved.
		if c.GetStatTraining(statName) >= int(b.MobStatTrainingCap) {
			return
		}
		chance *= float64(b.MobProgressionRate)
	}

	// Per-stat multiplier from config
	chance *= b.GetStatProgressionMultiplier(statName)

	// Mutation stat progression multiplier
	mutStatMult := 1.0 + mutations.GetStatProgressionMultiplier(c.Mutations)
	chance *= mutStatMult

	// Damp by rank. See regenDamperFactor's doc comment for why this exists.
	chance *= c.regenDamperFactor(statName)

	if chance <= 0 {
		return
	}
	if chance > 1.0 {
		chance = 1.0
	}

	// Roll: chance is 0.0–1.0, scaled to the integer roll resolution.
	threshold := int(chance * progressionRollDenominator)
	roll := util.Rand(progressionRollDenominator)

	if roll < threshold {
		mudlog.Debug("Progression", "check", "regen_stat", "result", "PROGRESS", "stat", statName,
			"chance", fmt.Sprintf("%.4f%%", chance*100),
			"roll", roll, "threshold", threshold, "character", c.Name)
		if c.IncreaseStat(statName, 1) {
			if userId > 0 {
				msg := fmt.Sprintf(`<ansi fg="magenta">***</ansi> Your <ansi fg="yellow">%s</ansi> grows stronger! <ansi fg="magenta">***</ansi>`, statName)
				events.AddToQueue(events.Message{UserId: userId, Text: msg + "\n"})
			}
		}
	}
}

// regenTickChance is the progression chance one regen tick presents for a pool,
// or 0 when no roll should happen. Extracted from OnRegenTick so it can be
// tested without waiting on a probabilistic roll, and so the reserved-pool rule
// lives in exactly one place.
//
// Formula: chance = RegenProgressionBase × (1 - current/effectiveMax) ^ RegenProgressionCurve
//
// The ratio measures the pool the character can actually REACH — raw max minus
// reservation — because a character sitting at their reserved cap is not
// depleted, they are full. Reading the raw max there turns a large reservation
// into a permanent progression farm (the fyttyn vitality exploit, 2026-04-16).
//
// Note this is the progression RATIO only. The regen AMOUNT deliberately keeps
// the raw max; see EffectivePoolMax's doc comment and HealthPerRound and its
// siblings in resources.go.
func (c *Character) regenTickChance(p Pool) float64 {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return 0
	}

	rawMax := c.poolMax(p)
	if rawMax <= 0 {
		return 0
	}
	// Deliberately NOT EffectivePoolMax: that is floored at 1 and never returns
	// 0, so a fully reserved pool would present max=1, current=0, ratio=0 — the
	// maximum chance, permanently. That is the same faucet inverted. A pool with
	// nothing reachable in it offers no progression at all.
	effMax := rawMax - c.GetPoolReservation(string(p), rawMax)
	if effMax <= 0 {
		return 0
	}

	ratio := float64(c.PoolValue(p)) / float64(effMax)
	if ratio >= 1.0 {
		return 0 // at the top of what they can reach: not depleted
	}
	if ratio < 0 {
		ratio = 0
	}

	b := configs.GetBalanceConfig()
	chance := float64(b.RegenProgressionBase) *
		math.Pow(1.0-ratio, float64(b.RegenProgressionCurve))
	if chance <= 0 {
		return 0
	}
	return chance
}

// OnRegenTick is called every regen tick (every 3 rounds) for each resource
// pool, and rolls regen-driven stat progression for the stats that pool
// exercises.
//
// It takes a Pool rather than a (current, max) pair on purpose. Six call sites
// used to compute the reservation-adjusted max by hand — correctly, but with
// nothing pinning it — and the next caller to get it wrong reopens a
// progression faucet. There is no longer a max to pass.
//
// Resource→stat mappings:
//
//	Health    → vitality, willpower
//	Stamina   → strength, vitality
//	Conviction→ willpower, charisma
func (c *Character) OnRegenTick(p Pool, relatedStats []string, userId int) {
	chance := c.regenTickChance(p)
	if chance <= 0 {
		return
	}

	for _, statName := range relatedStats {
		// Same reason as OnCritReceived: CheckRegenProgression rolls but never
		// records that the stat was exercised, so the rank that decays the
		// curve never moved. Health/stamina/conviction regen is the largest
		// source of vitality progression in the game and was entirely
		// rank-free.
		c.TrackStatUse(statName)
		c.CheckRegenProgression(statName, userId, chance)
	}
}

// GetSkillUseCount returns how many times a skill has been used.
func (c *Character) GetSkillUseCount(skillName string) int {
	if c.SkillUseCount == nil {
		return 0
	}
	return c.SkillUseCount[skillName]
}

// GetStatUseCount returns how many times a stat has been checked.
func (c *Character) GetStatUseCount(statName string) int {
	if c.StatUseCount == nil {
		return 0
	}
	return c.StatUseCount[statName]
}

// claimBonusProgression enforces the once-per-round bonus rule: at most one
// bonus event per skill per class per round, per character.
//
// Ordinary per-swing events are deliberately NOT deduped, so ordinary melee
// progression rates do not move. It is only the bonus that a margin-driven crit
// rate can fire on nearly every swing of a lopsided fight.
//
// A round of 0 means "no round context" and always claims, so non-combat
// callers are never silently suppressed.
func (c *Character) claimBonusProgression(ev progression.Event, round uint64) bool {
	if round == 0 {
		return true
	}
	if c.bonusProgressionRound == nil {
		c.bonusProgressionRound = make(map[string]uint64)
	}
	skillName, class := ev.Skill, ev.Class
	// The stat is part of the key, not just the skill. Observed events can
	// carry the SAME skill with DIFFERENT stats -- a crit received trains the
	// defence skill with the TOUGHENING stat, while a fumble observed trains it
	// with the defence stat. Keying on skill alone would let the first of those
	// in a round consume the slot for the other. Events with an empty skill
	// name (which the unarmed and no-defence paths can produce) would otherwise
	// all collapse onto one key.
	key := skillName + "|" + ev.Stat + "|" + strconv.Itoa(int(class))
	if c.bonusProgressionRound[key] >= round {
		return false
	}
	c.bonusProgressionRound[key] = round
	return true
}

// ClaimedBonusThisRound reports whether a bonus progression event has already
// fired for this skill this round, for any class.
//
// Exported deliberately. It is the only observable the bonus tier leaves behind
// -- bonus events do not track use counts by design -- so tests in OTHER
// packages (internal/combat, internal/hooks) need it to assert that the tier
// ran at all. A _test.go helper cannot serve them: test files compile only into
// their own package's test binary.
func (c *Character) ClaimedBonusThisRound(skillName string) bool {
	if c == nil || c.bonusProgressionRound == nil {
		return false
	}
	round := util.GetRoundCount()
	// Keys are "skill|stat|class" and the caller does not know the stat, so
	// match on the skill segment.
	prefix := skillName + "|"
	for key, claimed := range c.bonusProgressionRound {
		if claimed >= round && strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}

// ApplyProgression applies every event belonging to one side to this character.
// Callers invoke it twice per contest, once per participant, so this function
// never needs to know who the other party is.
//
// Ordinary events route through the pre-U9 entry points unchanged, which is
// what keeps the ordinary rates identical: OnSkillUseScaled tracks the use,
// grants mutation cluster drift, emits the SkillUsed quest event and rolls the
// skill's primary stat.
//
// Bonus events are pure extra rolls with no cluster drift and no quest event.
// Whether they ALSO track the use counter depends on the Class -- see
// applyBonusProgression for the refined rule (it is not a blanket "bonus
// events never track" any more).
func (c *Character) ApplyProgression(events []progression.Event, side progression.Side, userId int, round uint64) {
	if c == nil {
		return
	}
	for _, ev := range events {
		if ev.Side != side {
			continue
		}
		if ev.Class.IsBonus() {
			if !c.claimBonusProgression(ev, round) {
				continue
			}
			c.applyBonusProgression(ev, userId)
			continue
		}

		if ev.Skill != "" {
			c.OnSkillUseScaled(ev.Skill, userId, ev.Multiplier)
		}
		// OnSkillUseScaled already rolled the skill's primary stat. Roll again
		// only when the event names a DIFFERENT stat, which is the spell
		// primarystat override and the crit-received toughening stat.
		if ev.Stat != "" && ev.Stat != skills.GetSkillPrimaryStat(ev.Skill) {
			c.OnStatUse(ev.Stat, userId)
		}
	}
}

// applyBonusProgression takes the extra skill and stat rolls a crit, fumble or
// observed exceptional event earns, and speaks the flavour line on a success.
//
// Refined tracking rule (the "bonus events never track" rule was too broad):
//
//	ClassCrit / ClassFumble (the party who DID it) -- do NOT track. The use
//	count becomes a virtual rank and CalculateProgressionChance is
//	monotonically DECREASING in rank, so tracking the doer would punish the
//	very achievement (crit, or the lesson learned from a fumble) the bonus
//	roll exists to reward.
//
//	ClassObserved (the party who RECEIVED it) -- DO track. Nobody chooses to
//	be crit, so there is no achievement here for tracking to punish, and for
//	the crit-received toughening event specifically, tracking is the ONLY
//	thing that moves the target stat's virtual rank at all: nothing else in
//	the game calls OnStatUse/TrackStatUse for e.g. vitality. Leaving it
//	untracked means the stat's rank sits at 0 forever regardless of its
//	VALUE, which buys a flat, rank-independent chance no matter how much the
//	stat has already grown -- the fyttyn vitality exploit (see
//	internal/migration/0.16.0.go). Tracking before rolling (as below) is the
//	fix, mirroring the pre-seam OnCritReceived behaviour this replaces.
func (c *Character) applyBonusProgression(ev progression.Event, userId int) {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return
	}
	if ev.Multiplier <= 0 {
		return // the knob is set to its off-switch
	}

	if ev.Class == progression.ClassObserved {
		// TRACK BEFORE ROLLING, so this event's own use counts toward the
		// virtual rank the roll below is judged against.
		if ev.Skill != "" {
			c.TrackSkillUse(ev.Skill)
		}
		if ev.Stat != "" {
			c.TrackStatUse(ev.Stat)
		}
	}

	if ev.Skill != "" && c.CheckSkillProgression(ev.Skill, userId, ev.Multiplier) && userId > 0 {
		switch ev.Class {
		case progression.ClassCrit:
			events.AddToQueue(events.Message{UserId: userId, Text: fmt.Sprintf(
				`<ansi fg="magenta">***</ansi> A moment of brilliance! Your <ansi fg="yellow">%s</ansi> technique improves! <ansi fg="magenta">***</ansi>`,
				ev.Skill) + "\n"})
		case progression.ClassFumble:
			events.AddToQueue(events.Message{UserId: userId, Text: fmt.Sprintf(
				`<ansi fg="red">!!!</ansi> You learn from your mistake! Your <ansi fg="yellow">%s</ansi> understanding deepens. <ansi fg="red">!!!</ansi>`,
				ev.Skill) + "\n"})
			// ClassObserved is deliberately silent. Watching someone else's
			// brilliance is not your moment of brilliance.
		}
	}

	if ev.Stat != "" {
		c.CheckStatProgression(ev.Stat, userId, ev.Multiplier)
	}
}
