package progression_test

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/progression"
)

// Best-of mirrors combat's runBestOfAllDefense: every candidate is rolled, and
// exactly one of them earns the event. These tests pin the ordering rules, not
// the rolling -- BestOf never rolls anything, the caller fills Roll in.

func TestBestOf_HighestRollWins(t *testing.T) {
	cands := []progression.Candidate{
		{Skill: "weapon-combat", Stat: "strength", Roll: 120.0, Level: 40},
		{Skill: "skullduggery", Stat: "dexterity", Roll: 180.0, Level: 3},
		{Skill: "unarmed-combat", Stat: "dexterity", Roll: 90.0, Level: 50},
	}
	got, ok := progression.BestOf(cands)
	if !ok {
		t.Fatalf("BestOf reported false for a populated slice")
	}
	if got.Skill != "skullduggery" {
		t.Errorf("winner = %q (roll %v), want skullduggery -- highest Roll must win regardless of Level", got.Skill, got.Roll)
	}
}

func TestBestOf_EqualRollsBreakOnHigherLevel(t *testing.T) {
	cands := []progression.Candidate{
		{Skill: "weapon-combat", Roll: 100.0, Level: 7},
		{Skill: "skullduggery", Roll: 100.0, Level: 31},
		{Skill: "unarmed-combat", Roll: 100.0, Level: 12},
	}
	got, ok := progression.BestOf(cands)
	if !ok {
		t.Fatalf("BestOf reported false for a populated slice")
	}
	if got.Skill != "skullduggery" {
		t.Errorf("winner = %q (level %d), want skullduggery -- an equal roll breaks on the higher Level", got.Skill, got.Level)
	}
}

func TestBestOf_FullTieIsDeterministicAcrossRepeatedCalls(t *testing.T) {
	// Equal Roll AND equal Level. Slice order decides, so repeated calls with
	// the same slice must agree. A map iteration here would flake.
	cands := []progression.Candidate{
		{Skill: "weapon-combat", Stat: "strength", Roll: 100.0, Level: 9},
		{Skill: "skullduggery", Stat: "dexterity", Roll: 100.0, Level: 9},
		{Skill: "unarmed-combat", Stat: "dexterity", Roll: 100.0, Level: 9},
	}
	first, ok := progression.BestOf(cands)
	if !ok {
		t.Fatalf("BestOf reported false for a populated slice")
	}
	if first.Skill != "weapon-combat" {
		t.Errorf("winner = %q, want weapon-combat -- a full tie resolves on slice order", first.Skill)
	}
	for i := 0; i < 2000; i++ {
		got, ok := progression.BestOf(cands)
		if !ok || got != first {
			t.Fatalf("call %d returned (%+v, %v), want the stable (%+v, true)", i, got, ok, first)
		}
	}
}

func TestBestOf_EmptyInputAwardsNothing(t *testing.T) {
	got, ok := progression.BestOf(nil)
	if ok {
		t.Errorf("BestOf(nil) reported true; an empty Skill is NOT inert downstream")
	}
	if got != (progression.Candidate{}) {
		t.Errorf("BestOf(nil) candidate = %+v, want the zero Candidate", got)
	}

	got, ok = progression.BestOf([]progression.Candidate{})
	if ok {
		t.Errorf("BestOf(empty slice) reported true")
	}
	if got != (progression.Candidate{}) {
		t.Errorf("BestOf(empty slice) candidate = %+v, want the zero Candidate", got)
	}
}

func TestBestOf_WinnerWithNoSkillAndNoStatAwardsNothing(t *testing.T) {
	// Same reason as the empty slice: CheckSkillProgression("") takes a roll
	// and banners no skill, so a winner that names neither a skill nor a stat
	// would burn a roll and show the player nothing.
	got, ok := progression.BestOf([]progression.Candidate{
		{Roll: 200.0, Level: 4},
		{Skill: "weapon-combat", Roll: 100.0, Level: 4},
	})
	if ok {
		t.Errorf("BestOf reported true for a winner naming neither skill nor stat (got %+v)", got)
	}
}

func TestBestOf_StatOnlyWinnerIsALegitimateAward(t *testing.T) {
	// An empty Skill with a populated Stat is a stat-only award, a shape
	// OrdinaryEvents already supports. Do not filter it out.
	got, ok := progression.BestOf([]progression.Candidate{
		{Stat: "vitality", Roll: 200.0, Level: 0},
		{Skill: "weapon-combat", Roll: 100.0, Level: 4},
	})
	if !ok {
		t.Fatalf("BestOf reported false for a stat-only winner")
	}
	if got.Stat != "vitality" || got.Skill != "" {
		t.Errorf("winner = %+v, want the stat-only vitality candidate", got)
	}
}
