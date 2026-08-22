package characters

import "testing"

// GetStatTraining reads .Training only. Once Phase A folds authored mob
// training into base and Phase C moves spawn pools there too, Training means
// gains-since-spawn for every character type — so the progression curve keyed to
// it is gear-free and baseline-free by construction.
func TestGetStatTraining_ReadsTrainingOnly(t *testing.T) {
	c := New()
	c.Stats.Strength.Base = 115
	c.Stats.Strength.Training = 21
	c.Stats.Strength.SetMod(40)
	c.Stats.Strength.Recalculate()

	if got := c.GetStatTraining("strength"); got != 21 {
		t.Errorf("GetStatTraining(strength) = %d, want 21 (Base and Mods must not leak)", got)
	}
	if got := c.GetStatValue("strength"); got != 176 {
		t.Errorf("sanity: GetStatValue(strength) = %d, want 176", got)
	}
}

func TestGetStatTraining_AllSixStats(t *testing.T) {
	c := New()
	c.Stats.Strength.Training = 1
	c.Stats.Dexterity.Training = 2
	c.Stats.Perception.Training = 3
	c.Stats.Vitality.Training = 4
	c.Stats.Willpower.Training = 5
	c.Stats.Charisma.Training = 6

	for name, want := range map[string]int{
		"strength": 1, "dexterity": 2, "perception": 3,
		"vitality": 4, "willpower": 5, "charisma": 6,
	} {
		if got := c.GetStatTraining(name); got != want {
			t.Errorf("GetStatTraining(%q) = %d, want %d", name, got, want)
		}
	}
	if got := c.GetStatTraining("nonsense"); got != 0 {
		t.Errorf("GetStatTraining(nonsense) = %d, want 0", got)
	}
}
