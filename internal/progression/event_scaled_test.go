package progression_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/progression"
)

func scaledOutcome() progression.Outcome {
	return progression.Outcome{
		AttackerSkill: "weapon-combat",
		AttackerStat:  "strength",
		DefenderSkill: "unarmed-combat",
		DefenderStat:  "dexterity",
	}
}

// pick returns the single event for a side, or fails.
func pick(t *testing.T, evs []progression.Event, side progression.Side) progression.Event {
	t.Helper()
	var out []progression.Event
	for _, e := range evs {
		if e.Side == side {
			out = append(out, e)
		}
	}
	if len(out) != 1 {
		t.Fatalf("want exactly 1 event for side=%v, got %d (%+v)", side, len(out), evs)
	}
	return out[0]
}

func TestOrdinaryEventsScaled_DefendedScalesTheAttackerOnly(t *testing.T) {
	o := scaledOutcome()
	o.Defended = true

	evs := progression.OrdinaryEventsScaled(o, 0.35)
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2", len(evs))
	}

	a := pick(t, evs, progression.SideAttacker)
	if a.Multiplier != 0.35 {
		t.Errorf("attacker multiplier = %v, want 0.35 -- Defended means the ACTOR lost", a.Multiplier)
	}
	if !a.Lost {
		t.Errorf("attacker Lost = false, want true")
	}
	if a.Skill != "weapon-combat" || a.Stat != "strength" || a.Class != progression.ClassOrdinary {
		t.Errorf("attacker event = %+v, want the ordinary weapon-combat/strength pair", a)
	}

	d := pick(t, evs, progression.SideDefender)
	if d.Multiplier != 1.0 {
		t.Errorf("defender multiplier = %v, want 1.0 -- the winning side is untouched", d.Multiplier)
	}
	if d.Lost {
		t.Errorf("defender Lost = true, want false")
	}
}

func TestOrdinaryEventsScaled_NotDefendedScalesTheDefenderOnly(t *testing.T) {
	o := scaledOutcome()
	o.Defended = false

	evs := progression.OrdinaryEventsScaled(o, 0.35)
	if len(evs) != 2 {
		t.Fatalf("got %d events, want 2", len(evs))
	}

	d := pick(t, evs, progression.SideDefender)
	if d.Multiplier != 0.35 {
		t.Errorf("defender multiplier = %v, want 0.35", d.Multiplier)
	}
	if !d.Lost {
		t.Errorf("defender Lost = false, want true")
	}
	if d.Skill != "unarmed-combat" || d.Stat != "dexterity" || d.Class != progression.ClassOrdinary {
		t.Errorf("defender event = %+v, want the ordinary unarmed-combat/dexterity pair", d)
	}

	a := pick(t, evs, progression.SideAttacker)
	if a.Multiplier != 1.0 {
		t.Errorf("attacker multiplier = %v, want 1.0 -- the winning side is untouched", a.Multiplier)
	}
	if a.Lost {
		t.Errorf("attacker Lost = true, want false")
	}
}

func TestOrdinaryEventsScaled_PopulatesTheSameSidesAsOrdinaryEvents(t *testing.T) {
	// Scaling must not invent a side. A one-sided Outcome stays one-sided,
	// including when the missing side is the losing one.
	o := progression.Outcome{AttackerSkill: "skullduggery", Defended: true}
	evs := progression.OrdinaryEventsScaled(o, 0.35)
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	a := pick(t, evs, progression.SideAttacker)
	if a.Multiplier != 0.35 || !a.Lost {
		t.Errorf("attacker event = %+v, want multiplier 0.35 and Lost true", a)
	}

	o = progression.Outcome{DefenderStat: "vitality", Defended: true}
	evs = progression.OrdinaryEventsScaled(o, 0.35)
	if len(evs) != 1 {
		t.Fatalf("got %d events, want 1", len(evs))
	}
	d := pick(t, evs, progression.SideDefender)
	if d.Multiplier != 1.0 || d.Lost {
		t.Errorf("defender event = %+v, want the untouched winning side", d)
	}
}

func TestOrdinaryEventsScaled_FracBoundariesAreLegal(t *testing.T) {
	// frac 0 is the shipped off-switch (losing teaches nothing, today's
	// behaviour); frac 1.0 makes a loss worth as much as a win. Neither is an
	// error, and both must still mark the loser Lost -- the SkillUsed quest
	// gate reads Lost, not Multiplier.
	for _, frac := range []float64{0.0, 1.0} {
		o := scaledOutcome()
		o.Defended = true
		evs := progression.OrdinaryEventsScaled(o, frac)
		a := pick(t, evs, progression.SideAttacker)
		if a.Multiplier != frac {
			t.Errorf("frac %v: attacker multiplier = %v, want %v", frac, a.Multiplier, frac)
		}
		if !a.Lost {
			t.Errorf("frac %v: attacker Lost = false, want true", frac)
		}
	}
}

func TestOrdinaryEvents_UnscaledLeavesLostFalse(t *testing.T) {
	// The unscaled path is untouched by this task: full weight, no Lost.
	o := scaledOutcome()
	o.Defended = true
	for _, e := range progression.OrdinaryEvents(o) {
		if e.Multiplier != 1.0 || e.Lost {
			t.Errorf("OrdinaryEvents event = %+v, want multiplier 1.0 and Lost false", e)
		}
	}
}
