package progression

import "testing"

func bonuses() Bonuses { return Bonuses{Doing: 2.0, Observing: 0.5} }

func fullOutcome() Outcome {
	return Outcome{
		AttackerSkill: "weapon-combat",
		AttackerStat:  "dexterity",
		DefenderSkill: "unarmed-combat",
		DefenderStat:  "dexterity",
		ToughenStat:   "vitality",
	}
}

// find returns the single event matching side+class, or fails.
func find(t *testing.T, evs []Event, side Side, class Class) Event {
	t.Helper()
	var out []Event
	for _, e := range evs {
		if e.Side == side && e.Class == class {
			out = append(out, e)
		}
	}
	if len(out) != 1 {
		t.Fatalf("want exactly 1 event for side=%v class=%v, got %d", side, class, len(out))
	}
	return out[0]
}

func TestOrdinary_BothSidesOneEventEach(t *testing.T) {
	evs := EventsForContest(fullOutcome(), bonuses())
	if len(evs) != 2 {
		t.Fatalf("ordinary contest produced %d events, want 2", len(evs))
	}
	a := find(t, evs, SideAttacker, ClassOrdinary)
	if a.Skill != "weapon-combat" || a.Stat != "dexterity" || a.Multiplier != 1.0 {
		t.Errorf("attacker ordinary = %+v", a)
	}
	d := find(t, evs, SideDefender, ClassOrdinary)
	if d.Skill != "unarmed-combat" || d.Multiplier != 1.0 {
		t.Errorf("defender ordinary = %+v", d)
	}
}

func TestAttackCrit_AttackerDoesDefenderToughens(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	evs := EventsForContest(o, bonuses())

	a := find(t, evs, SideAttacker, ClassCrit)
	if a.Multiplier != 2.0 || a.Stat != "dexterity" {
		t.Errorf("attacker crit = %+v, want mult 2.0 stat dexterity", a)
	}
	d := find(t, evs, SideDefender, ClassObserved)
	if d.Multiplier != 0.5 {
		t.Errorf("defender observed multiplier = %v, want 0.5", d.Multiplier)
	}
	// You learn to TAKE a hit, not to swing better.
	if d.Stat != "vitality" {
		t.Errorf("defender observed stat = %q, want vitality (the toughening stat)", d.Stat)
	}
	if d.Skill != "unarmed-combat" {
		t.Errorf("defender observed skill = %q, want the defence skill", d.Skill)
	}
}

func TestDefenceCrit_DefenderDoesAttackerObserves(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcDefenceCrit
	evs := EventsForContest(o, bonuses())

	d := find(t, evs, SideDefender, ClassCrit)
	if d.Multiplier != 2.0 || d.Stat != "dexterity" {
		t.Errorf("defender crit = %+v", d)
	}
	a := find(t, evs, SideAttacker, ClassObserved)
	if a.Multiplier != 0.5 {
		t.Errorf("attacker observed multiplier = %v, want 0.5", a.Multiplier)
	}
}

// Failure teaches: spec 5.0's matrix pays the bonus to whoever fumbled.
func TestAttackFumble_AttackerEarnsTheBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackFumble
	evs := EventsForContest(o, bonuses())

	a := find(t, evs, SideAttacker, ClassFumble)
	if a.Multiplier != 2.0 {
		t.Errorf("attacker fumble multiplier = %v, want 2.0", a.Multiplier)
	}
	// The defender OBSERVES a fumble with their own defence stat, not the
	// toughening stat -- nothing hit them.
	d := find(t, evs, SideDefender, ClassObserved)
	if d.Stat != "dexterity" {
		t.Errorf("defender observed stat on fumble = %q, want the defence stat", d.Stat)
	}
}

func TestDefenceFumble_DefenderEarnsTheBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcDefenceFumble
	evs := EventsForContest(o, bonuses())

	if got := find(t, evs, SideDefender, ClassFumble).Multiplier; got != 2.0 {
		t.Errorf("defender fumble multiplier = %v, want 2.0", got)
	}
	if got := find(t, evs, SideAttacker, ClassObserved).Multiplier; got != 0.5 {
		t.Errorf("attacker observed multiplier = %v, want 0.5", got)
	}
}

// A floored outcome is the system overriding the dice. Participation still
// teaches; an exceptional event the dice did not produce does not.
func TestFloored_OrdinaryOnlyNoBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	o.Floored = true
	evs := EventsForContest(o, bonuses())

	if len(evs) != 2 {
		t.Fatalf("floored contest produced %d events, want 2 ordinary only", len(evs))
	}
	for _, e := range evs {
		if e.Class != ClassOrdinary {
			t.Errorf("floored contest emitted a %v event: %+v", e.Class, e)
		}
	}
}

// The caller decides who participates by populating the fields. An absent
// defender must not fabricate a defender event -- this is what keeps U9 from
// changing WHEN progression fires.
func TestNoDefenderFields_NoDefenderEvents(t *testing.T) {
	o := Outcome{AttackerSkill: "skullduggery", AttackerStat: "dexterity"}
	evs := EventsForContest(o, bonuses())
	if len(evs) != 1 || evs[0].Side != SideAttacker {
		t.Fatalf("got %+v, want a single attacker event", evs)
	}
}

func TestZeroBonuses_ActAsOffSwitches(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	evs := EventsForContest(o, Bonuses{Doing: 0, Observing: 0})
	for _, e := range evs {
		if e.Class != ClassOrdinary && e.Multiplier != 0 {
			t.Errorf("zeroed bonus produced multiplier %v", e.Multiplier)
		}
	}
}

// The split exists so melee can take the bonus tier WITHOUT a second ordinary
// defender event, which its per-round AwardDefenceProgression already awards.
func TestBonusEvents_EmitsNoOrdinary(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	for _, e := range BonusEvents(o, bonuses()) {
		if e.Class == ClassOrdinary {
			t.Errorf("BonusEvents emitted an ordinary event: %+v", e)
		}
	}
}

func TestOrdinaryEvents_EmitsNoBonus(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	for _, e := range OrdinaryEvents(o) {
		if e.Class != ClassOrdinary {
			t.Errorf("OrdinaryEvents emitted a bonus event: %+v", e)
		}
	}
}

// One contest lands on exactly ONE matrix row. Four independent booleans let a
// caller pay four bonus rolls for one swing, and pay a fumble bonus to the
// winner; the enum makes that unrepresentable.
func TestClassify_Precedence(t *testing.T) {
	cases := []struct {
		name                                   string
		atkCrit, defCrit, atkFumble, defFumble bool
		want                                   Exceptional
	}{
		{"nothing", false, false, false, false, ExcNone},
		{"attack crit", true, false, false, false, ExcAttackCrit},
		{"defence crit", false, true, false, false, ExcDefenceCrit},
		{"attack fumble", false, false, true, false, ExcAttackFumble},
		{"defence fumble", false, false, false, true, ExcDefenceFumble},
		// A fumble is self-relative and a crit is margin-derived, so an
		// attacker can roll terribly and still be out-rolled worse. The crit is
		// what the game narrated, so the crit is what pays.
		{"crit outranks a co-occurring fumble", false, true, true, false, ExcDefenceCrit},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Classify(tc.atkCrit, tc.defCrit, tc.atkFumble, tc.defFumble); got != tc.want {
				t.Errorf("Classify = %v, want %v", got, tc.want)
			}
		})
	}
}

// Exactly two bonus events per contest, never four.
func TestBonusEvents_NeverPaysBothAxes(t *testing.T) {
	o := fullOutcome()
	o.Exceptional = ExcAttackCrit
	if got := len(BonusEvents(o, bonuses())); got != 2 {
		t.Errorf("BonusEvents produced %d events, want exactly 2", got)
	}
}
