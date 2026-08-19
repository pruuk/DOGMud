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

// CheckSkillProgression rolls against the progression chance for a skill.
// If the roll succeeds, the skill level is increased. Returns true if
// progression fired (skill actually gained), false otherwise.
// bonusMultiplier scales the chance (e.g. 2.0 for critical successes).
func (c *Character) CheckSkillProgression(skillName string, userId int, bonusMultiplier float64) bool {
	b := configs.GetBalanceConfig()

	// Mob-specific gating
	if c.IsMob {
		if !bool(b.MobProgressionEnabled) {
			return false
		}
		actualSkill := resolveSkillName(skillName)
		if lvl, ok := c.Skills[actualSkill]; ok && lvl >= int(b.MobSkillCap) {
			return false // hard cap
		}
		bonusMultiplier *= float64(b.MobProgressionRate)
	}

	// Normalize use count by the skill's progression multiplier so that
	// frequently-fired skills (combat) don't exhaust the progression curve
	// faster than infrequently-fired skills (utility). Without this,
	// combat skills asymptote at ~15 progs vs ~100 for utility skills.
	progressMult := skills.GetProgressionMultiplier(skillName)
	adjustedUseCount := c.GetSkillUseCount(skillName)
	if progressMult > 0 && progressMult < 1.0 {
		adjustedUseCount = int(float64(adjustedUseCount) * progressMult)
	}
	virtualRank := adjustedUseCount / int(b.UsesPerRank)
	// If the actual skill level exceeds the soft cap, use it as a floor for the virtual rank.
	// This prevents characters with artificially high skills (e.g. admin accounts) from
	// exploiting the low use-count portion of the progression curve.
	actualSkillName := resolveSkillName(skillName)
	if skillLevel, ok := c.Skills[actualSkillName]; ok && skillLevel > int(b.SkillSoftCap) && skillLevel > virtualRank {
		virtualRank = skillLevel
	}
	// Phase 24.2: Apply mutation skill progression multiplier
	mutSkillMult := 1.0 + mutations.GetSkillProgressionMultiplier(c.Mutations)
	// Phase 25.3: Skill Attunement buff doubles skill progression chance
	buffSkillMult := 1.0
	if c.HasBuffFlag(buffs.SkillProgress) {
		buffSkillMult = 2.0
	}
	chance := CalculateProgressionChance(virtualRank, int(b.SkillSoftCap)) * bonusMultiplier * skills.GetProgressionMultiplier(skillName) * mutSkillMult * buffSkillMult
	if chance > 1.0 {
		chance = 1.0
	}

	// Roll: chance is 0.0–1.0, convert to 0–10000 for integer roll
	threshold := int(chance * 10000)
	roll := util.Rand(10000)

	if roll < threshold {
		mudlog.Debug("Progression", "check", "skill", "result", "PROGRESS", "skill", skillName, "rank", virtualRank, "chance", fmt.Sprintf("%.2f%%", chance*100), "roll", roll, "threshold", threshold, "character", c.Name)
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
// against -- critReceivedChance shares it too, so OnCritReceived and its test
// compute the exact same expression instead of one drifting from the other.
//
// A mob at or past MobStatCap, or with mob progression disabled, returns 0
// (the hard-cap short-circuit CheckStatProgression used to perform itself).
func (c *Character) statProgressionChance(statName string, bonusMultiplier float64) float64 {
	b := configs.GetBalanceConfig()

	// Mob-specific gating
	if c.IsMob {
		if !bool(b.MobProgressionEnabled) {
			return 0
		}
		if c.GetStatValue(statName) >= int(b.MobStatCap) {
			return 0 // hard cap
		}
		bonusMultiplier *= float64(b.MobProgressionRate)
	}

	virtualRank := c.GetStatUseCount(statName) / int(b.UsesPerRank)
	// If the actual stat value exceeds the soft cap, use it as a floor for the virtual rank.
	// This prevents characters with artificially high stats (e.g. admin accounts) from
	// exploiting the low use-count portion of the progression curve.
	if statVal := c.GetStatValue(statName); statVal > int(b.StatProgressionSoftCap) && statVal > virtualRank {
		virtualRank = statVal
	}
	// Phase 24.2: Apply mutation stat progression multiplier
	mutStatMult := 1.0 + mutations.GetStatProgressionMultiplier(c.Mutations)
	// Phase 39.1: Per-stat progression multiplier from config
	statMult := b.GetStatProgressionMultiplier(statName)
	chance := CalculateProgressionChance(virtualRank, int(b.StatProgressionSoftCap)) * bonusMultiplier * mutStatMult * statMult * float64(b.StatProgressionRate)
	if chance > 1.0 {
		chance = 1.0
	}
	return chance
}

// CheckStatProgression rolls against the progression chance for a stat.
// If the roll succeeds, the stat's Training value is increased by 1.
// Returns true if progression fired (stat actually gained), false otherwise.
func (c *Character) CheckStatProgression(statName string, userId int, bonusMultiplier float64) bool {
	b := configs.GetBalanceConfig()

	chance := c.statProgressionChance(statName, bonusMultiplier)
	if chance <= 0 {
		return false
	}

	virtualRank := c.GetStatUseCount(statName) / int(b.UsesPerRank)
	if statVal := c.GetStatValue(statName); statVal > int(b.StatProgressionSoftCap) && statVal > virtualRank {
		virtualRank = statVal
	}

	threshold := int(chance * 10000)
	roll := util.Rand(10000)

	if roll < threshold {
		mudlog.Debug("Progression", "check", "stat", "result", "PROGRESS", "stat", statName, "rank", virtualRank, "chance", fmt.Sprintf("%.2f%%", chance*100), "roll", roll, "threshold", threshold, "character", c.Name)
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
// hardest because regen is its ONLY source of progression.
//
// A BaseProgressionChance of 0 (progression fully disabled) returns 0 rather
// than dividing by zero.
func (c *Character) regenDamperFactor(statName string) float64 {
	b := configs.GetBalanceConfig()
	base := float64(b.BaseProgressionChance)
	if base <= 0 {
		return 0
	}
	virtualRank := c.GetStatUseCount(statName) / int(b.UsesPerRank)
	if statVal := c.GetStatValue(statName); statVal > int(b.StatProgressionSoftCap) && statVal > virtualRank {
		virtualRank = statVal
	}
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
		if c.GetStatValue(statName) >= int(b.MobStatCap) {
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

	// Roll: chance is 0.0–1.0, convert to 0–10000 for integer roll
	threshold := int(chance * 10000)
	roll := util.Rand(10000)

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

// OnRegenTick is called every regen tick (every 3 rounds) for each resource
// pool. It computes a smooth chance based on how depleted the pool is and
// rolls for stat progression on each related stat.
//
// Formula: chance = RegenProgressionBase × (1 - current/max) ^ RegenProgressionCurve
//
// Resource→stat mappings:
//
//	Health    → vitality, willpower
//	Stamina   → strength, vitality
//	Conviction→ willpower, charisma
func (c *Character) OnRegenTick(current, max int, relatedStats []string, userId int) {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return
	}
	if max <= 0 {
		return
	}

	ratio := float64(current) / float64(max)
	if ratio >= 1.0 {
		return // Pool is full, no progression chance
	}
	if ratio < 0 {
		ratio = 0
	}

	b := configs.GetBalanceConfig()
	base := float64(b.RegenProgressionBase)
	curve := float64(b.RegenProgressionCurve)

	chance := base * math.Pow(1.0-ratio, curve)
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
// Bonus events are pure extra rolls: no use tracking (it would decay the curve,
// spec 5.2), no cluster drift, no quest event.
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
func (c *Character) applyBonusProgression(ev progression.Event, userId int) {
	if !configs.GetGamePlayConfig().UseSkillProgression {
		return
	}
	if ev.Multiplier <= 0 {
		return // the knob is set to its off-switch
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
