package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/progression"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The Actor seam must carry U10b-1's firing rule, so a non-combat action site
// can award progression without knowing whether it is driving a player or a
// mob. These assertions are the compile-time half: the whole point of putting
// AwardResolved on the interface is that BOTH production adapters supply the
// userId themselves.
var (
	_ Actor = (*UserActor)(nil)
	_ Actor = (*MobActor)(nil)
)

// awardResolvedWinner and awardResolvedLoser are a pair chosen so the win half
// of these tests is EXACT rather than statistical. Under
// pinCertainProgressionForTest BaseProgressionChance is 1.0, and
// weapon-combat's per-skill progression multiplier pushes the product past the
// 1.0 clamp at a fresh character's rank -- so a won award ALWAYS advances. The
// preconditions below assert that, so a moved default fails loudly here rather
// than flaking on the counts.
const (
	awardResolvedWinner = "weapon-combat"
	awardResolvedLoser  = "search"
)

// pinCertainProgressionForTest is the internal/actions twin of
// internal/characters' pinCertainStatProgressionForTest. It cannot be shared:
// Go test helpers are not visible across packages, and the characters one is
// unexported.
//
// MobProgressionRate is pinned too, which the characters helper does not need.
// The mob branch of ProgressionChanceForSkill multiplies the bonus multiplier
// by it (shipped default 0.5), so without this pin the mob half of these tests
// would be a coin flip while the player half was certain.
func pinCertainProgressionForTest(t *testing.T) {
	t.Helper()
	pinConfigForTest(t)
	cfg := configs.GetConfig()
	cfg.Balance.BaseProgressionChance = 1.0
	cfg.Balance.StatProgressionRate = 1.0
	cfg.Balance.StatProgressionMultipliers = nil
	cfg.Balance.MobProgressionRate = 1.0
	configs.SetConfigForTest(t, cfg)
}

// requireCertainSkillAward fails the test if the pinned config does not make a
// WON award on this character certain. Without it the exact assertions below
// would silently become statistical.
func requireCertainSkillAward(t *testing.T, c *characters.Character) {
	t.Helper()
	if got := c.ProgressionChanceForSkill(awardResolvedWinner, 1.0); got < 1.0 {
		t.Fatalf("precondition: a won %s roll has chance %v, not the pinned certainty of 1.0; the exact assertions below would be statistical", awardResolvedWinner, got)
	}
}

// A UserActor's AwardResolved must reach the USER's character, fire for the
// Best-of winner alone, and pass the user id -- which is observable, because
// OnSkillUseScaled only queues the SkillUsed quest event when userId > 0.
func TestUserActorAwardResolved_FiresForTheBestOfWinnerAndCarriesTheUserId(t *testing.T) {
	pinCertainProgressionForTest(t)
	events.DrainQueuedSkillUsedForTest(0) // start from a clean queue

	const userId = 77
	char := characters.New()
	requireCertainSkillAward(t, char)

	var actor Actor = &UserActor{User: &users.UserRecord{UserId: userId, Character: char}}

	before := char.Skills[awardResolvedWinner]
	actor.AwardResolved(true,
		progression.Candidate{Skill: awardResolvedLoser, Roll: 10},
		progression.Candidate{Skill: awardResolvedWinner, Roll: 200},
	)

	// Use counts are tracked unconditionally by OnSkillUseScaled, so these two
	// are exact regardless of any roll: exactly ONE event, for the winner.
	if got := char.GetSkillUseCount(awardResolvedWinner); got != 1 {
		t.Errorf("winner %s use count = %d, want 1; the award did not reach the user's character", awardResolvedWinner, got)
	}
	if got := char.GetSkillUseCount(awardResolvedLoser); got != 0 {
		t.Errorf("loser %s use count = %d, want 0; Best-of must fire for exactly one candidate", awardResolvedLoser, got)
	}
	if after := char.Skills[awardResolvedWinner]; after <= before {
		t.Errorf("%s went %d -> %d on a WON award; the chance is pinned to certainty so it must advance", awardResolvedWinner, before, after)
	}

	queued := events.DrainQueuedSkillUsedForTest(0)
	if len(queued) != 1 {
		t.Fatalf("queued %d SkillUsed events, want 1; UserActor must pass its own user id through so quest tracking still fires", len(queued))
	}
	if queued[0].UserId != userId {
		t.Errorf("SkillUsed carried user id %d, want %d", queued[0].UserId, userId)
	}
	if string(queued[0].Skill) != awardResolvedWinner {
		t.Errorf("SkillUsed named skill %q, want %q", queued[0].Skill, awardResolvedWinner)
	}
}

// A MobActor's AwardResolved must reach the MOB's own character and pass user
// id 0, exactly as OnSkillUse does. Mobs have no user record; a non-zero id
// here would queue a quest event against whichever player happens to hold it.
func TestMobActorAwardResolved_UsesTheMobsCharacterAndUserIdZero(t *testing.T) {
	pinCertainProgressionForTest(t)
	events.DrainQueuedSkillUsedForTest(0) // start from a clean queue

	c := characters.New()
	c.IsMob = true
	requireCertainSkillAward(t, c)

	m := &mobs.Mob{InstanceId: 4242, Character: *c}
	var actor Actor = &MobActor{Mob: m}

	before := m.Character.Skills[awardResolvedWinner]
	actor.AwardResolved(true,
		progression.Candidate{Skill: awardResolvedLoser, Roll: 10},
		progression.Candidate{Skill: awardResolvedWinner, Roll: 200},
	)

	if got := m.Character.GetSkillUseCount(awardResolvedWinner); got != 1 {
		t.Errorf("winner %s use count = %d on the mob's character, want 1; the award did not reach a.Mob.Character", awardResolvedWinner, got)
	}
	if got := m.Character.GetSkillUseCount(awardResolvedLoser); got != 0 {
		t.Errorf("loser %s use count = %d, want 0; Best-of must fire for exactly one candidate", awardResolvedLoser, got)
	}
	if after := m.Character.Skills[awardResolvedWinner]; after <= before {
		t.Errorf("%s went %d -> %d on a WON award; the chance is pinned to certainty so it must advance", awardResolvedWinner, before, after)
	}

	if queued := events.DrainQueuedSkillUsedForTest(0); len(queued) != 0 {
		t.Errorf("a mob award queued %d SkillUsed events, want 0; MobActor must pass user id 0", len(queued))
	}
}

// A LOST award still fires -- at ProgressionFailureFraction, not full weight.
// The use count is the deterministic witness: it is tracked before any roll,
// so this half needs no probability at all.
func TestUserActorAwardResolved_ALossStillFiresForTheWinner(t *testing.T) {
	pinCertainProgressionForTest(t)
	events.DrainQueuedSkillUsedForTest(0)

	char := characters.New()
	var actor Actor = &UserActor{User: &users.UserRecord{UserId: 78, Character: char}}

	actor.AwardResolved(false,
		progression.Candidate{Skill: awardResolvedLoser, Roll: 10},
		progression.Candidate{Skill: awardResolvedWinner, Roll: 200},
	)

	if got := char.GetSkillUseCount(awardResolvedWinner); got != 1 {
		t.Errorf("winner %s use count = %d after a LOST action, want 1; a loss awards at the failure fraction, not nothing", awardResolvedWinner, got)
	}
	// A loss must NOT tick a "use this skill N times" quest.
	if queued := events.DrainQueuedSkillUsedForTest(0); len(queued) != 0 {
		t.Errorf("a LOST award queued %d SkillUsed events, want 0", len(queued))
	}
}
