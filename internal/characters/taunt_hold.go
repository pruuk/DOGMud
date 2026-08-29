package characters

import "github.com/GoMudEngine/GoMud/internal/util"

// This file owns the taunt-hold LOCK STATE and nothing else.
//
// It used to also own ForceTauntAggro, which set the lock and then engaged.
// That engage made this file a commit site inside internal/characters, which
// internal/targeting imports and therefore can never be imported BY. Rather
// than exempt the most frequent retargeting mechanic in the game from the
// targeting seam, the two halves were split: the lock is state and stays here,
// while "pin then engage" moved to targeting.CommitTaunt.

// SetTauntHold applies a taunt-hold lock onto the given taunter for
// holdRounds. It does NOT engage: committing is targeting.CommitTaunt's job,
// and the split is what keeps internal/characters free of targeting logic.
//
// Callers must set the lock BEFORE committing, so TauntHoldBlocks sees the new
// taunter as the locked target and lets that very commit through. That
// ordering is also why a newer taunt cleanly overrides an active hold from a
// previous taunter.
func (c *Character) SetTauntHold(userId, mobInstanceId, holdRounds int) {
	if holdRounds < 1 {
		holdRounds = 1
	}
	c.tauntHoldUserId = userId
	c.tauntHoldMobInstanceId = mobInstanceId
	c.tauntHoldUntilRound = util.GetRoundCount() + uint64(holdRounds)
}

// tauntHoldActive reports whether a taunt-hold lock is currently in force.
func (c *Character) tauntHoldActive() bool {
	if c.tauntHoldUserId == 0 && c.tauntHoldMobInstanceId == 0 {
		return false
	}
	return c.tauntHoldUntilRound > util.GetRoundCount()
}

// TauntHoldBlocks reports whether an incoming target set should be ignored
// because a taunt hold pins this character onto a different taunter. Only
// basic attack-type aggro (DefaultAttack/Shooting/SurpriseAttack) is pinned;
// SpellCast and Flee (self/room-directed) always pass, as does a set that
// matches the locked taunter.
//
// Exported so targeting.Commit can consult it once U12c deletes SetAggro and
// the guard bodies move out of this package.
func (c *Character) TauntHoldBlocks(userId, mobInstanceId int, aggroType AggroType) bool {
	if !c.tauntHoldActive() {
		return false
	}
	switch aggroType {
	case DefaultAttack, Shooting, SurpriseAttack:
		return userId != c.tauntHoldUserId || mobInstanceId != c.tauntHoldMobInstanceId
	default:
		return false
	}
}

// ClearTauntHold drops any active taunt-hold lock. Called from EndAggro so a
// dead/fled taunter doesn't leave the enemy pinned and unable to re-acquire.
func (c *Character) ClearTauntHold() {
	c.tauntHoldUntilRound = 0
	c.tauntHoldUserId = 0
	c.tauntHoldMobInstanceId = 0
}
