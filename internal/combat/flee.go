package combat

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// FleeBlocker identifies the first opponent that successfully blocked
// a flee attempt. Exactly one of UserId / MobInstanceId is non-zero,
// matching the standard `state.ActorRef` convention.
type FleeBlocker struct {
	Name          string
	UserId        int
	MobInstanceId int
}

// IsPlayer reports whether the blocker is a player.
func (b FleeBlocker) IsPlayer() bool { return b.UserId != 0 }

// ResolveFleeBlockers walks every combatant in `room` currently
// targeting `fleer`, rolls each one's Dex+UnarmedCombat×SkillWeight
// against the fleer's Dex+Skullduggery×SkillWeight (with a 0.5×
// prone penalty when the fleer is Prone or Supine), and returns the
// first blocker whose roll wins. Returns nil if nobody blocks.
//
// Works for player and mob fleers alike: the helper reads
// fleer.GetUserId() and fleer.GetMobInstanceId() to identify which
// combatants are actually targeting the fleer. Both player and mob
// blockers are walked.
//
// The grapple-block precondition (IsStandingGrapple || IsGroundGrapple
// on the fleer) is NOT checked here — callers handle it because the
// messaging differs (player direct vs mob room broadcast). This
// helper is purely the opposed-roll resolver.
//
// Replaces two duplicated opposed-roll loops:
//   - internal/hooks/NewRound_DoCombat_helpers.go:handlePlayerFlee
//   - internal/mobcommands/flee.go:Flee
//
// Side effect of the refactor: the player-flee path's player-blocker
// loop had a variable-shadowing typo (the inner `for _, userId := range`
// shadowed the outer fleer's userId, so the in-loop "is this player
// targeting me" check degenerated to "is this player targeting
// themselves" — always false in practice). Routing through this
// helper restores the intended PvP-blocker behavior.
// The second return value reports whether ANY opposed roll was actually made.
// Callers use it to award skullduggery practice, which is deliberately NOT done
// here: this function is named "resolve", it is called from two packages, and a
// progression write buried in it awarded practice for a contest that never
// happened -- walking out of a room where nothing was fighting you trained
// skullduggery exactly as much as breaking a grapple line did. Progression is
// the wrapper's job; see U9, which makes that split explicit everywhere.
func ResolveFleeBlockers(fleer *characters.Character, room *rooms.Room, includeSkill bool) (*FleeBlocker, bool) {
	if fleer == nil || room == nil {
		return nil, false
	}
	contested := false

	fleerUid := fleer.GetUserId()
	fleerMid := fleer.GetMobInstanceId()

	fleeScore := fleeContestScore(fleer, includeSkill)

	// Mobs first, then players. Both loops use FindFighting to scope
	// to combatants only; the in-loop targeting filter narrows to
	// "fighting the fleer specifically".
	for _, mobInstId := range room.GetMobs(rooms.FindFighting) {
		m := mobs.GetInstance(mobInstId)
		if m == nil {
			continue
		}
		if m.InstanceId == fleerMid {
			continue // a mob doesn't block its own flee
		}
		if !mobTargetsFleer(m, fleerUid, fleerMid) {
			continue
		}
		blockScore := fleeBlockScore(&m.Character)
		contested = true
		success := RunContest(fleeScore, []contest.Entry{{Score: blockScore}}).Success
		if !success {
			return &FleeBlocker{
				Name:          m.Character.Name,
				MobInstanceId: m.InstanceId,
			}, contested
		}
	}

	for _, uId := range room.GetPlayers(rooms.FindFighting) {
		if uId == fleerUid {
			continue // a player doesn't block their own flee
		}
		u := users.GetByUserId(uId)
		if u == nil {
			continue
		}
		if !playerTargetsFleer(u, fleerUid, fleerMid) {
			continue
		}
		blockScore := fleeBlockScore(u.Character)
		contested = true
		success := RunContest(fleeScore, []contest.Entry{{Score: blockScore}}).Success
		if !success {
			return &FleeBlocker{
				Name:   u.Character.Name,
				UserId: u.UserId,
			}, contested
		}
	}

	return nil, contested
}

func fleeContestScore(fleer *characters.Character, includeSkill bool) float64 {
	if fleer == nil {
		return 0
	}
	score := float64(fleer.GetEffectiveDexterity())
	if includeSkill {
		score += float64(fleer.GetSkillLevel(skills.Skullduggery)) *
			float64(configs.GetBalanceConfig().SkillWeight)
	}
	pronePenalty := 1.0
	if fleer.IsProne() || fleer.IsSupine() {
		pronePenalty = 0.5
	}
	return score * pronePenalty
}

// fleeBlockScore is a would-be blocker's side of the flee contest:
// Dex + UnarmedCombat × SkillWeight. Shared by the mob and player
// blocker loops in ResolveFleeBlockers.
func fleeBlockScore(blocker *characters.Character) float64 {
	return float64(blocker.GetEffectiveDexterity()) +
		float64(blocker.GetSkillLevel(skills.UnarmedCombat))*
			float64(configs.GetBalanceConfig().SkillWeight)
}

// mobTargetsFleer reports whether mob m's aggro points at the fleer
// (player or mob).
func mobTargetsFleer(m *mobs.Mob, fleerUid, fleerMid int) bool {
	if m == nil || m.Character.Aggro == nil {
		return false
	}
	if fleerUid > 0 && m.Character.Aggro.UserId == fleerUid {
		return true
	}
	if fleerMid > 0 && m.Character.Aggro.MobInstanceId == fleerMid {
		return true
	}
	return false
}

// playerTargetsFleer reports whether player u's current combat
// target is the fleer (player or mob).
func playerTargetsFleer(u *users.UserRecord, fleerUid, fleerMid int) bool {
	if u == nil || !u.Character.IsInCombat() {
		return false
	}
	target := u.Character.CurrentCombatTarget()
	if fleerUid > 0 && target.UserId == fleerUid {
		return true
	}
	if fleerMid > 0 && target.MobInstanceId == fleerMid {
		return true
	}
	return false
}
