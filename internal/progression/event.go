// Package progression is the pure event layer of the unified contest arc:
// given the plain facts of one resolved contest, it says which progression
// events that contest implies, for whom, at what multiplier.
//
// It deliberately does NOT fire anything. It holds no *characters.Character, no
// room, and reads no config -- multipliers arrive as arguments. That is what
// makes the matrix table-testable with plain values, which matters here because
// a Go test binary never loads _datafiles/config.yaml and a package that read
// balance config would be tested against Go defaults instead of shipped values.
//
// It also does NOT decide WHEN progression fires. The caller populates only the
// sides that earned an ordinary event, exactly as that call site did before U9.
// Adding or removing a firing condition is U10b's job, not this package's.
package progression

// Side names one participant in a contest.
type Side uint8

const (
	SideAttacker Side = iota
	SideDefender
)

// Class names why an event fired. Ordinary events track a use and progress
// normally; the other three are bonus events.
type Class uint8

const (
	// ClassOrdinary is ordinary use. It tracks the use counter.
	ClassOrdinary Class = iota
	// ClassCrit is the party who landed the crit.
	ClassCrit
	// ClassFumble is the party who fumbled. Failure teaches.
	ClassFumble
	// ClassObserved is the party who received or witnessed the exceptional
	// event rather than causing it.
	ClassObserved
)

// IsBonus reports whether a class is one of the three exceptional classes.
//
// Bonus events must NOT increment the use counters. In CheckSkillProgression
// the use count becomes a virtual rank and CalculateProgressionChance is
// monotonically DECREASING in rank, so inflating the counter on a crit would
// punish critting. See spec 5.2.
func (c Class) IsBonus() bool { return c != ClassOrdinary }

// Event is one progression award for one side. An empty Skill or Stat means
// "no roll of that kind" -- never pass an empty name downstream, since
// CheckSkillProgression("") takes a roll and a success banners no skill at all.
type Event struct {
	Side       Side
	Skill      string
	Stat       string
	Class      Class
	Multiplier float64

	// Lost reports that the actor's action was RESOLVED and LOST, so this
	// event is the consolation award rather than the award for succeeding.
	// Under U10b-1's Best-of firing convention a resolved action always
	// produces an event: at full weight on a win, and at
	// ProgressionFailureFraction on a loss.
	//
	// It is a SEPARATE field and NOT inferred from Multiplier < 1.0, and that
	// distinction is load-bearing. Downstream, mutation cluster drift scales
	// by Multiplier while the SkillUsed quest event is gated on Lost -- two
	// different mechanisms on purpose. A sub-1.0 multiplier does NOT imply a
	// loss: a self-buff cast is a WINNING action that legitimately arrives at
	// SelfCastProgressionMultiplier (ships 0.5). "Simplifying" this field away
	// into Multiplier < 1.0 silently stops every self-buff cast from ticking
	// skill_use quests, with no error message anywhere -- the exact regression
	// TestOnSkillUseScaled_WinningSubOneMultiplierStillEmitsSkillUsed exists
	// to catch. Do not do it.
	Lost bool
}

// Outcome is everything about a resolved contest that the matrix needs.
//
// The four boolean outcome flags are not mutually exclusive by type, but a
// single contest produces at most one attack-side and one defence-side
// exceptional result. Callers set what happened; this package does not
// arbitrate.
type Outcome struct {
	// Populate a side's Skill/Stat only if that side earns an ordinary event
	// under the CALL SITE's existing rules. Leaving them empty suppresses that
	// side entirely.
	AttackerSkill string
	AttackerStat  string
	DefenderSkill string
	DefenderStat  string

	// ToughenStat is the stat awarded to a defender who RECEIVES a crit:
	// vitality for physical, willpower for magical, charisma for conviction.
	// Deliberately not DefenderStat -- you learn to take a hit, not to swing
	// better. Falls back to DefenderStat when empty.
	ToughenStat string

	// Exceptional names the ONE exceptional result this contest produced.
	// It is a single enum rather than four booleans on purpose: the spec's
	// matrix is five mutually exclusive rows, and four independent flags let a
	// caller pay four bonus events for one contest, or pay a fumble bonus to
	// the side that won. Use Classify to derive it.
	Exceptional Exceptional

	// Floored reports that a contest floor CHANGED the outcome. Floored
	// contests award ordinary events but never bonuses.
	Floored bool
}

// Exceptional names which single row of the spec's matrix a contest landed on.
type Exceptional uint8

const (
	ExcNone Exceptional = iota
	ExcAttackCrit
	ExcAttackFumble
	ExcDefenceCrit
	ExcDefenceFumble
)

// Classify reduces the engine's crit and fumble signals to the one row that
// fired, in a fixed precedence.
//
// Crits cannot collide: since 5.11d a contest crit is derived from the
// NORMALIZED MARGIN, and one margin cannot be both strongly positive and
// strongly negative, so attackCrit and defenceCrit are mutually exclusive by
// construction.
//
// Fumbles CAN collide with a crit, because a fumble is self-relative (the
// z-score of one roll) rather than margin-derived: an attacker can roll
// terribly and still be out-rolled worse. When that happens the CRIT wins,
// because the crit is the outcome the game narrated to both players, and
// paying a bonus for an event nobody was told about is how progression stops
// being legible.
func Classify(attackCrit, defenceCrit, attackFumble, defenceFumble bool) Exceptional {
	switch {
	case attackCrit:
		return ExcAttackCrit
	case defenceCrit:
		return ExcDefenceCrit
	case attackFumble:
		return ExcAttackFumble
	case defenceFumble:
		return ExcDefenceFumble
	}
	return ExcNone
}

// Bonuses carries the two config-driven multipliers. They are arguments rather
// than config reads so this package stays pure. Zero is a legal off-switch.
type Bonuses struct {
	Doing     float64 // CritProgressionBonus
	Observing float64 // ObservedCritProgressionBonus
}

// OrdinaryEvents returns only the ordinary-use events an Outcome implies: one
// per side whose Skill or Stat is populated.
//
// It is SEPARATE from BonusEvents because callers genuinely need one without
// the other. Melee awards its defender's ordinary event once per round through
// AwardDefenceProgression, but evaluates its bonus tier from the same Outcome;
// asking for both there would award the defender an extra ordinary event per
// weapon hit. Making the caller filter a combined slice is how that bug gets
// written, so the package does the split.
func OrdinaryEvents(o Outcome) []Event {
	evs := make([]Event, 0, 2)
	if o.AttackerSkill != "" || o.AttackerStat != "" {
		evs = append(evs, Event{
			Side: SideAttacker, Skill: o.AttackerSkill, Stat: o.AttackerStat,
			Class: ClassOrdinary, Multiplier: 1.0,
		})
	}
	if o.DefenderSkill != "" || o.DefenderStat != "" {
		evs = append(evs, Event{
			Side: SideDefender, Skill: o.DefenderSkill, Stat: o.DefenderStat,
			Class: ClassOrdinary, Multiplier: 1.0,
		})
	}
	return evs
}

// BonusEvents returns only the crit/fumble tier: the pair of events the one
// exceptional result implies, or nothing.
//
// A floored outcome returns nothing. A floor overrode the dice, and an
// exceptional event that did not actually happen teaches nobody.
func BonusEvents(o Outcome, b Bonuses) []Event {
	if o.Floored || o.Exceptional == ExcNone {
		return nil
	}

	toughen := o.ToughenStat
	if toughen == "" {
		toughen = o.DefenderStat
	}

	// Keyed literals, not positional: Event grew a Lost field and positional
	// literals would either break the build or, worse, quietly bind a new
	// field to the wrong value. Bonus events leave Lost at its false zero --
	// they are extra rolls layered on top of an ordinary event, and they never
	// reach the SkillUsed emit at all (see applyBonusProgression).
	switch o.Exceptional {
	case ExcAttackCrit:
		return []Event{
			{Side: SideAttacker, Skill: o.AttackerSkill, Stat: o.AttackerStat, Class: ClassCrit, Multiplier: b.Doing},
			// The one cell that swaps the stat: a crit RECEIVED toughens.
			{Side: SideDefender, Skill: o.DefenderSkill, Stat: toughen, Class: ClassObserved, Multiplier: b.Observing},
		}
	case ExcAttackFumble:
		return []Event{
			{Side: SideAttacker, Skill: o.AttackerSkill, Stat: o.AttackerStat, Class: ClassFumble, Multiplier: b.Doing},
			{Side: SideDefender, Skill: o.DefenderSkill, Stat: o.DefenderStat, Class: ClassObserved, Multiplier: b.Observing},
		}
	case ExcDefenceCrit:
		return []Event{
			{Side: SideDefender, Skill: o.DefenderSkill, Stat: o.DefenderStat, Class: ClassCrit, Multiplier: b.Doing},
			{Side: SideAttacker, Skill: o.AttackerSkill, Stat: o.AttackerStat, Class: ClassObserved, Multiplier: b.Observing},
		}
	case ExcDefenceFumble:
		return []Event{
			{Side: SideDefender, Skill: o.DefenderSkill, Stat: o.DefenderStat, Class: ClassFumble, Multiplier: b.Doing},
			{Side: SideAttacker, Skill: o.AttackerSkill, Stat: o.AttackerStat, Class: ClassObserved, Multiplier: b.Observing},
		}
	}
	return nil
}

// EventsForContest is OrdinaryEvents followed by BonusEvents, for the callers
// that want both. Ordinary events come first so a caller applying them in order
// tracks the use before rolling the bonus.
func EventsForContest(o Outcome, b Bonuses) []Event {
	return append(OrdinaryEvents(o), BonusEvents(o, b)...)
}
