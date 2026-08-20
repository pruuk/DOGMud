package combat

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/dice"
	"github.com/GoMudEngine/GoMud/internal/skills"
)

// side builds the canonical spellcasting AttackSide the seam tests use.
func side(stat, rank int) AttackSide {
	return AttackSide{
		Stat: stat, StatName: "willpower",
		Skill: skills.Spellcasting, SkillRank: rank, Mult: 1.0,
	}
}

// Score = (Stat + Rank x SkillWeight) x Mult. SkillWeight comes from config;
// pin it via the balance-config test idiom the package already uses.
func TestAttackSide_Score(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.SkillWeight = 5
	configs.SetConfigForTest(t, cfg)

	s := side(148, 52)
	if got, want := s.score(), 148.0+52*5; got != want {
		t.Errorf("score=%v want %v", got, want)
	}
	s.Mult = 0.75
	if got, want := s.score(), (148.0+52*5)*0.75; got != want {
		t.Errorf("mult score=%v want %v", got, want)
	}
	// The zero value is not a silent zero-out: Mult 0 means "unset", 1.0.
	s.Mult = 0
	if got, want := s.score(), 148.0+52*5; got != want {
		t.Errorf("zero-mult score=%v want %v (Mult 0 must read as 1.0)", got, want)
	}
}

// Attacker crit comes from THE contest's margin against CritBarFor, and a
// FLOORED outcome can never be a crit — the missing guard modelling found on
// the old spell/taunt call sites.
func TestResolveChannelAttack_FlooredNeverCrits(t *testing.T) {
	atk, def := newDefenceTestCharacter(t), newDefenceTestCharacter(t)
	runner := func(_ float64, entries []contest.Entry) contest.Result {
		return contest.Result{
			Contested: true, Winner: entries[0].Name,
			Floored: true, Success: true, Margin: 1, // floor-promoted "win"
		}
	}
	out := resolveChannelAttackWithRunner(ChannelSpellMental, side(148, 52), atk, def, runner)
	if out.AttackerCrit {
		t.Error("a floor-promoted win was promoted again to a crit")
	}
}

// Fumble aborts even a winning attack — Assumption 7, kept and documented.
// Fumble is self-relative: AttackRoll.ZScore <= -DefenseCritBar(). The seam
// only SURFACES the verdict; Defended keeps the runner's semantics unchanged
// and the CALLER is expected to abort on Fumble before consuming success.
func TestResolveChannelAttack_FumblePreemptsSuccess(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	atk, def := newDefenceTestCharacter(t), newDefenceTestCharacter(t)
	runner := func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Success:   true, // the roll "won" ...
			Margin:    30,
			AttackRoll: dice.RollResult{ // ... on a fumbled swing
				Value: atkScore - 2.5*stdDev, Mean: atkScore,
				StdDev: stdDev, ZScore: -2.5,
			},
			DefenseRoll: dice.RollResult{
				Value: atkScore - 2.5*stdDev - 30, Mean: entries[0].Score,
				StdDev: stdDev,
			},
		}
	}
	out := resolveChannelAttackWithRunner(ChannelSpellMental, side(148, 52), atk, def, runner)
	if !out.AttackerFumble {
		t.Error("AttackRoll.ZScore -2.5 did not surface as AttackerFumble")
	}
	if out.Defended {
		t.Error("fumble surfacing changed the runner's Defended semantics")
	}
	if out.DamageMultiplier != 1.0 {
		t.Errorf("attack-win damage multiplier = %v, want 1.0 (caller aborts on fumble, seam does not)", out.DamageMultiplier)
	}
}

// The seam's crit bar is the CHANNEL's skill PAIR, not the constant. Rank 52
// against a rank-0 quell clamps the bar to the 1.5 floor, so a normalized
// margin of 1.8 crits here and would NOT have crit at the const 2.0 bar.
func TestResolveChannelAttack_CritUsesThePairBar(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	cfg := configs.GetConfig()
	cfg.Balance.CritBarSkillSlope = 0.05
	cfg.Balance.CritBarFloor = 1.5
	cfg.Balance.CritBarCeiling = 3.0
	configs.SetConfigForTest(t, cfg)

	atk, def := newDefenceTestCharacter(t), newDefenceTestCharacter(t)
	runner := func(atkScore float64, entries []contest.Entry) contest.Result {
		stdDev := dice.StdDevFor(atkScore)
		margin := 1.8 * stdDev * math.Sqrt2
		return contest.Result{
			Contested: true,
			Winner:    entries[0].Name,
			Success:   true,
			Margin:    margin,
			AttackRoll: dice.RollResult{
				Value: atkScore + 1.8*stdDev, Mean: atkScore,
				StdDev: stdDev, ZScore: 1.8,
			},
			DefenseRoll: dice.RollResult{
				Value: atkScore, Mean: entries[0].Score, StdDev: stdDev,
			},
		}
	}
	out := resolveChannelAttackWithRunner(ChannelSpellMental, side(148, 52), atk, def, runner)
	if !out.AttackerCrit {
		t.Error("normalized margin 1.8 vs pair bar 1.5 (rank 52 vs 0) must crit; the const 2.0 bar leaked back in")
	}
	if out.AttackerFumble {
		t.Error("a +1.8 z attack roll surfaced as a fumble")
	}
}

// The bonus-tier progression events must name the skill and stat the CALLER
// passed, not a per-channel hardcode — this deletes the drift risk of the U9
// channel-to-skill switch the bonus tier used to carry.
func TestResolveChannelAttack_ProgressionNamesTheCallersSkill(t *testing.T) {
	pinDefenceAdmissionConfig(t)
	attacker, defender := defenceAdmissionCharacters()
	defender.Conviction = 100

	manifestSide := AttackSide{
		Stat: 100, StatName: "charisma",
		Skill: skills.Manifestation, SkillRank: 30, Mult: 1.0,
	}
	out := resolveChannelAttackWithRunner(ChannelSocial, manifestSide, attacker, defender,
		func(atkScore float64, entries []contest.Entry) contest.Result {
			if atkScore != 160 {
				t.Fatalf("attack score = %.2f, want 160 from the caller's side (100 + 30x2)", atkScore)
			}
			// 72 / (24*sqrt(2)) = 2.1213: a defensive crit, which pays the
			// defender and has the ATTACKER observe it.
			return deterministicDefenceResult(t, atkScore, entries, characters.DefenseDefy, 160, 232)
		})

	if !out.Defended || !out.DefensiveCrit {
		t.Fatalf("outcome = %+v, want a defensive crit to drive the observed bonus", out)
	}
	if !attacker.ClaimedBonusThisRound(string(skills.Manifestation)) {
		t.Error("attacker-side observed bonus did not carry the AttackSide's skill (manifestation)")
	}
	if attacker.ClaimedBonusThisRound(string(skills.Rhetoric)) {
		t.Error("a per-channel hardcode (social -> rhetoric) named the attacker's skill instead of the caller")
	}
}

// channelDamageChannel's default-"" premise is dead: melee and ranged crits
// resolved through the seam must toughen VITALITY, and ToughenStatFor("")
// would silently toughen dexterity's wrong-stat neighbour instead.
func TestChannelDamageChannel_PhysicalRows(t *testing.T) {
	for _, ch := range []AttackChannel{ChannelMelee, ChannelRanged} {
		if got := channelDamageChannel(ch); got != "physical" {
			t.Errorf("channelDamageChannel(%s) = %q, want physical", ch, got)
		}
	}
	if got := characters.ToughenStatFor(channelDamageChannel(ChannelMelee)); got != "vitality" {
		t.Errorf("melee crit toughen stat = %q, want vitality", got)
	}
}

// AttackContestCritAt is the bar-parameterised form; AttackContestCrit must
// remain exactly the const-threshold call.
func TestAttackContestCritAt_BarParameterised(t *testing.T) {
	cfg := configs.GetConfig()
	cfg.Balance.MinAttackCritChance = 0
	configs.SetConfigForTest(t, cfg)

	roll := dice.RollResult{StdDev: 10}
	margin := 1.8 * 10 * math.Sqrt2
	if AttackContestCritAt(margin, roll, 2.0) {
		t.Error("normalized 1.8 crossed a 2.0 bar")
	}
	if !AttackContestCritAt(margin, roll, 1.5) {
		t.Error("normalized 1.8 failed a 1.5 bar")
	}
	if AttackContestCrit(margin, roll) {
		t.Error("AttackContestCrit stopped being the ContestCritThreshold call")
	}
}
