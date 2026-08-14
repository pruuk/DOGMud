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
	cfg.Death.StatDecayFloor = configs.ConfigInt(100)
	cfg.Death.SkillRustFloor = configs.ConfigInt(1)
	return cfg
}

// seedRacialStats gives the character a racial roll plus training so the
// PERMANENT part of each stat (Racial + Training) sits above StatDecayFloor.
// Racial is a gaussian roll in the real game, not a constant.
func seedRacialStats(u *users.UserRecord, training int) {
	seedRacialStatsAt(u, 100, training)
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

// Stat decay never drives Training negative, and never drops a stat's PERMANENT
// part (Racial + Training) below StatDecayFloor.
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
		{"strength", u.Character.Stats.Strength.Training, u.Character.Stats.Strength.Racial + u.Character.Stats.Strength.Training},
		{"dexterity", u.Character.Stats.Dexterity.Training, u.Character.Stats.Dexterity.Racial + u.Character.Stats.Dexterity.Training},
		{"perception", u.Character.Stats.Perception.Training, u.Character.Stats.Perception.Racial + u.Character.Stats.Perception.Training},
		{"vitality", u.Character.Stats.Vitality.Training, u.Character.Stats.Vitality.Racial + u.Character.Stats.Vitality.Training},
		{"willpower", u.Character.Stats.Willpower.Training, u.Character.Stats.Willpower.Racial + u.Character.Stats.Willpower.Training},
		{"charisma", u.Character.Stats.Charisma.Training, u.Character.Stats.Charisma.Racial + u.Character.Stats.Charisma.Training},
	}
	for _, s := range all {
		if s.train < 0 {
			t.Errorf("%s training = %d, want >= 0", s.name, s.train)
		}
		if s.value < floor {
			t.Errorf("%s permanent value = %d, want >= %d (StatDecayFloor)", s.name, s.value, floor)
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

// A character whose PERMANENT stat sits at or below the floor is left entirely
// alone, however many times they die. Racial is a gaussian roll, so a new or
// unlucky character legitimately starts here and must not be ground down.
func TestStatDecay_AtOrBelowFloorIsUntouched(t *testing.T) {
	config := degradationConfig()
	floor := int(config.Death.StatDecayFloor)

	u := users.NewTestUser(706, "newplayer", "Newplayer", 5001)
	// Rolled slightly under the floor with nothing trained yet.
	seedRacialStatsAt(u, floor-8, 0)

	for range 100 {
		applyPlayerStatDecay(u, config)
	}

	if got := u.Character.Stats.Strength.Training; got != 0 {
		t.Errorf("strength training = %d, want 0 — a below-floor character was degraded", got)
	}
	if got := u.Character.Stats.Strength.Racial; got != floor-8 {
		t.Errorf("strength racial = %d, want %d — racial must never be touched", got, floor-8)
	}
}

// Mods come from equipment and buffs. They must NOT count toward the floor, or
// a permanent penalty would hinge on what someone happened to be wearing when
// they died: put a +20 ring on and the floor silently stops protecting you.
func TestStatDecay_ModsDoNotCountTowardTheFloor(t *testing.T) {
	config := degradationConfig()
	floor := int(config.Death.StatDecayFloor)

	u := users.NewTestUser(707, "geared", "Geared", 5001)
	// Permanent part sits ON the floor, but a big item bonus lifts Value well
	// above it. The floor must still refuse to decay.
	seedRacialStatsAt(u, floor, 0)
	for _, si := range allStatInfos(u) {
		si.Mods = 40
		si.Recalculate()
	}

	for range 100 {
		applyPlayerStatDecay(u, config)
	}

	for _, si := range allStatInfos(u) {
		if si.Training != 0 {
			t.Fatalf("training = %d, want 0 — equipment Mods lifted a stat past its floor", si.Training)
		}
	}
}

func allStatInfos(u *users.UserRecord) []*stats.StatInfo {
	return []*stats.StatInfo{
		&u.Character.Stats.Strength,
		&u.Character.Stats.Dexterity,
		&u.Character.Stats.Perception,
		&u.Character.Stats.Vitality,
		&u.Character.Stats.Willpower,
		&u.Character.Stats.Charisma,
	}
}

func seedRacialStatsAt(u *users.UserRecord, racial, training int) {
	for _, si := range allStatInfos(u) {
		si.Base = racial
		si.Training = training
		si.Recalculate()
	}
}
