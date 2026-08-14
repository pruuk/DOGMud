package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/stats"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// degradationConfig builds the death knobs explicitly. Unit tests do not load
// the config file, so every Death.* value would otherwise be zero — and
// SkillRustCount's default only guards NEGATIVE values, so a zero would
// silently rust nothing and make these tests pass for the wrong reason.
func degradationConfig() configs.GamePlay {
	var cfg configs.GamePlay
	cfg.Death.StatDecayMin = configs.ConfigInt(1)
	cfg.Death.StatDecayMax = configs.ConfigInt(2)
	cfg.Death.SkillRustCount = configs.ConfigInt(1)
	cfg.Death.SkillRustAmount = configs.ConfigInt(1)
	cfg.Death.StatDecayFloor = configs.ConfigInt(80)
	cfg.Death.SkillRustFloor = configs.ConfigInt(1)
	return cfg
}

// seedRacialStats gives the character a human-baseline racial value so its
// stat Value sits above StatDecayFloor to begin with.
func seedRacialStats(u *users.UserRecord, training int) {
	for _, si := range []*stats.StatInfo{
		&u.Character.Stats.Strength,
		&u.Character.Stats.Dexterity,
		&u.Character.Stats.Perception,
		&u.Character.Stats.Vitality,
		&u.Character.Stats.Willpower,
		&u.Character.Stats.Charisma,
	} {
		si.Base = 100
		si.Training = training
		si.Recalculate()
	}
}

// A heavily used skill must be rustable.
//
// The old eligibility filter skipped any skill whose LIFETIME use count was at
// or above SkillRecencyThreshold (50), calling that "recently used". Since the
// counter is never reset, everything an established character actually used was
// protected forever, and the penalty fell every single time on the handful of
// skills they had barely touched. Reported from prod as "it is always
// skullduggery".
func TestSkillRust_HeavilyUsedSkillIsStillEligible(t *testing.T) {
	config := degradationConfig()

	u := users.NewTestUser(701, "rustplayer", "Rustplayer", 5001)
	u.Character.Skills = map[string]int{"weapon-combat": 20}
	u.Character.SkillUseCount = map[string]int{"weapon-combat": 100000}

	applyPlayerSkillRust(u, config)

	if got := u.Character.Skills["weapon-combat"]; got >= 20 {
		t.Fatalf("weapon-combat = %d, want < 20 — a lifetime-used skill was protected forever", got)
	}
}

// A skill already at the floor is left completely alone.
func TestSkillRust_SkillAtFloorIsUntouched(t *testing.T) {
	config := degradationConfig()

	u := users.NewTestUser(702, "floorplayer", "Floorplayer", 5001)
	floor := int(config.Death.SkillRustFloor)
	u.Character.Skills = map[string]int{"skullduggery": floor}

	applyPlayerSkillRust(u, config)

	if got := u.Character.Skills["skullduggery"]; got != floor {
		t.Fatalf("skullduggery = %d, want %d — a skill at the floor was degraded", got, floor)
	}
}

// Rust never drops a skill below the floor, however large the rust amount.
func TestSkillRust_NeverGoesBelowFloor(t *testing.T) {
	config := degradationConfig()
	floor := int(config.Death.SkillRustFloor)

	u := users.NewTestUser(703, "lowplayer", "Lowplayer", 5001)
	u.Character.Skills = map[string]int{"skullduggery": floor + 1}

	for range 20 {
		applyPlayerSkillRust(u, config)
	}

	if got := u.Character.Skills["skullduggery"]; got < floor {
		t.Fatalf("skullduggery = %d, want >= %d", got, floor)
	}
}

// Stat decay never drives Training negative, and never drops a stat's Value
// below StatDecayFloor.
func TestStatDecay_RespectsBothFloors(t *testing.T) {
	config := degradationConfig()
	floor := int(config.Death.StatDecayFloor)

	u := users.NewTestUser(704, "statplayer", "Statplayer", 5001)
	seedRacialStats(u, 3)

	for range 200 {
		applyPlayerStatDecay(u, config)
	}

	all := []struct {
		name  string
		train int
		value int
	}{
		{"strength", u.Character.Stats.Strength.Training, u.Character.Stats.Strength.Value},
		{"dexterity", u.Character.Stats.Dexterity.Training, u.Character.Stats.Dexterity.Value},
		{"perception", u.Character.Stats.Perception.Training, u.Character.Stats.Perception.Value},
		{"vitality", u.Character.Stats.Vitality.Training, u.Character.Stats.Vitality.Value},
		{"willpower", u.Character.Stats.Willpower.Training, u.Character.Stats.Willpower.Value},
		{"charisma", u.Character.Stats.Charisma.Training, u.Character.Stats.Charisma.Value},
	}
	for _, s := range all {
		if s.train < 0 {
			t.Errorf("%s training = %d, want >= 0", s.name, s.train)
		}
		if s.value < floor {
			t.Errorf("%s value = %d, want >= %d (StatDecayFloor)", s.name, s.value, floor)
		}
	}
}

// The stat pick is uniform across all six. Over many deaths every stat should
// take at least one hit; a filter or a biased pick would starve some of them.
func TestStatDecay_SpreadsAcrossAllSixStats(t *testing.T) {
	config := degradationConfig()

	u := users.NewTestUser(705, "spreadplayer", "Spreadplayer", 5001)
	start := 400
	seedRacialStats(u, start)

	for range 300 {
		applyPlayerStatDecay(u, config)
	}

	for _, s := range []struct {
		name  string
		train int
	}{
		{"strength", u.Character.Stats.Strength.Training},
		{"dexterity", u.Character.Stats.Dexterity.Training},
		{"perception", u.Character.Stats.Perception.Training},
		{"vitality", u.Character.Stats.Vitality.Training},
		{"willpower", u.Character.Stats.Willpower.Training},
		{"charisma", u.Character.Stats.Charisma.Training},
	} {
		if s.train >= start {
			t.Errorf("%s was never picked in 300 deaths — the pick is not uniform", s.name)
		}
	}
}
