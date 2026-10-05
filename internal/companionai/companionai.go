// Package companionai is the seam between the engine and the aicompanion
// module. internal/ never imports modules/, so the engine calls these
// functions and the module (or internal/hooks) installs the implementations
// at boot. Every entry point is nil-safe: with nothing installed, each one
// reports "not handled" and the engine behaves exactly as it did before.
package companionai

// AskFunc handles `ask <companion> <text>` for a companion the module drives.
// It returns true when it claimed the question.
type AskFunc func(userId int, mobInstanceId int, text string) bool

// RespawnFunc brings a fallen bonded companion back into its owner's room
// mid-session. It returns the new mob instance id, or 0 when it could not.
type RespawnFunc func(userId int, mobId int) int

// IdleFunc takes over a mob's idle tick. It returns true when the mob is a
// companion the module drives, in which case the default idle behaviour
// must not run.
type IdleFunc func(mobInstanceId int) bool

// RejoinFunc brings an owner's fielded companions to the owner's room the
// way following does. It returns false when the owner is not online.
type RejoinFunc func(userId int) bool

// NpcAskFunc puts a question from a companion to an NPC as though the
// companion's owner had asked it, so the NPC answers with its ordinary
// dialogue, quest and behaviour-tree responses. It returns false when the
// question could not be delivered.
// authorized says the owner asked for this in the moment. Only an
// authorized question may touch the owner's quests; an unauthorized one is
// conversation and nothing more.
type NpcAskFunc func(ownerUserId int, mobInstanceId int, text string, authorized bool) bool

// BondedFunc reports whether a mob instance is somebody's bonded
// companion. Engine code that keeps its own per-template opinion of a mob
// asks this first: a bonded companion keeps its feelings in its own mind,
// and moving two numbers on one event leaves the admin views describing a
// score that drives nothing.
type BondedFunc func(mobInstanceId int) bool

// HoldFunc reports that one of a user's companions, the mob instance
// mobInstanceId, should not follow them just now: the usual case is an owner
// moving in secret, where a bonded companion padding along behind would give
// them away. It leaves that companion where it is until the owner comes back
// or stops sneaking. It is asked once per companion, so an ordinary companion
// the handler does not drive keeps following.
type HoldFunc func(userId int, mobInstanceId int) bool

// SnapshotFunc copies a user's fielded bonded companion's live state (gear,
// gold, progression) into its saved record without despawning it. It
// returns false when there was nothing to snapshot.
type SnapshotFunc func(userId int) bool

var (
	askFunc      AskFunc
	respawnFunc  RespawnFunc
	idleFunc     IdleFunc
	rejoinFunc   RejoinFunc
	snapshotFunc SnapshotFunc
	npcAskFunc   NpcAskFunc
	holdFunc     HoldFunc
	bondedFunc   BondedFunc
)

// SetBondedCheck installs the bonded-companion test. Called by the
// aicompanion module.
func SetBondedCheck(f BondedFunc) {
	bondedFunc = f
}

// IsBondedCompanion reports whether a mob is somebody's bonded companion.
// Nil-safe: with no module installed, nothing is bonded.
func IsBondedCompanion(mobInstanceId int) bool {
	if bondedFunc == nil {
		return false
	}
	return bondedFunc(mobInstanceId)
}

// DrivesFunc reports whether the bonded companions of a mob template are
// driven: the module is on and has that companion's profile.
type DrivesFunc func(mobId int) bool

var drivesFunc DrivesFunc

// SetDrivesCheck installs the per-companion drives check. Called by the
// aicompanion module, only while it is switched on.
func SetDrivesCheck(f DrivesFunc) {
	drivesFunc = f
}

// DrivesBonded reports whether anything drives a bonded companion of this
// mob template (CompanionInfo.MobId). With nothing installed (the module
// off), or with no profile for that companion, a bonded companion left
// over is an ordinary companion nobody drives, and the engine lets its
// owner dismiss it rather than leave them stuck with it. It asks by
// template, not by live instance, so a companion that is fallen or not yet
// taken up at login is still driven.
func DrivesBonded(mobId int) bool {
	if drivesFunc == nil {
		return false
	}
	return drivesFunc(mobId)
}

// SetHolder installs the follow-hold handler. Called by the aicompanion
// module.
func SetHolder(f HoldFunc) {
	holdFunc = f
}

// HoldPosition asks whether one of a user's companions (the mob instance
// mobInstanceId) should stay where it is rather than follow. Nil-safe: with
// nothing installed, every companion always follows.
func HoldPosition(userId int, mobInstanceId int) bool {
	if holdFunc == nil {
		return false
	}
	return holdFunc(userId, mobInstanceId)
}

// SetNpcAsker installs the NPC question handler. Called by
// internal/usercommands, which owns the dialogue chain.
func SetNpcAsker(f NpcAskFunc) {
	npcAskFunc = f
}

// AskNpc delivers a companion's question to an NPC on its owner's behalf.
func AskNpc(ownerUserId int, mobInstanceId int, text string, authorized bool) bool {
	if npcAskFunc == nil {
		return false
	}
	return npcAskFunc(ownerUserId, mobInstanceId, text, authorized)
}

// SetSnapshotter installs the snapshot implementation. Called by
// internal/hooks, which owns the companion save code.
func SetSnapshotter(f SnapshotFunc) {
	snapshotFunc = f
}

// Snapshot asks the installed snapshotter to copy a bonded companion's live
// state into its record, so a crash or shutdown loses at most what happened
// since the last snapshot. The engine otherwise only saves companion state
// at logout.
func Snapshot(userId int) bool {
	if snapshotFunc == nil {
		return false
	}
	return snapshotFunc(userId)
}

// SetRejoiner installs the rejoin implementation. Called by internal/hooks,
// which owns companion transport.
func SetRejoiner(f RejoinFunc) {
	rejoinFunc = f
}

// Rejoin asks the installed rejoiner to bring a user's companions to them.
// The aicompanion module uses it only as a last resort, when a companion
// that went off on its own cannot find a known way back.
func Rejoin(userId int) bool {
	if rejoinFunc == nil {
		return false
	}
	return rejoinFunc(userId)
}

// SetIdleHandler installs the idle handler. Called by the aicompanion module.
func SetIdleHandler(f IdleFunc) {
	idleFunc = f
}

// RouteIdle offers a mob's idle tick to the installed handler.
func RouteIdle(mobInstanceId int) bool {
	if idleFunc == nil {
		return false
	}
	return idleFunc(mobInstanceId)
}

// SetAskHandler installs the ask handler. Called by the aicompanion module.
func SetAskHandler(f AskFunc) {
	askFunc = f
}

// RouteAsk offers an ask to the installed handler.
func RouteAsk(userId int, mobInstanceId int, text string) bool {
	if askFunc == nil {
		return false
	}
	return askFunc(userId, mobInstanceId, text)
}

// ShowFunc is told about an item a player shows to a mob: its name and
// description as a model may read them (items.Item ModelName and
// ModelDescription, never player-written bauble text), who made it when the
// item carries a maker's mark, and whether it was crafted at all (handMade:
// a crafter below the maker's-mark skill leaves no name). It returns true
// when the mob is one the handler speaks for. The item never leaves the
// player.
type ShowFunc func(userId int, mobInstanceId int, name string, description string, madeBy string, handMade bool) bool

var showFunc ShowFunc

// SetShowHandler installs the show handler. Called by the aicompanion
// module, which lets a companion waiting in the Waystone Hollow see what a
// player brings to show them.
func SetShowHandler(f ShowFunc) {
	showFunc = f
}

// RouteShow offers a shown item to the installed handler.
func RouteShow(userId int, mobInstanceId int, name string, description string, madeBy string, handMade bool) bool {
	if showFunc == nil {
		return false
	}
	return showFunc(userId, mobInstanceId, name, description, madeBy, handMade)
}

// SetRespawner installs the respawn implementation. Called by internal/hooks,
// which owns the companion spawn and state-restore code.
func SetRespawner(f RespawnFunc) {
	respawnFunc = f
}

// RespawnBonded asks the installed respawner to bring a fallen bonded
// companion back. Returns 0 when nothing is installed or the respawn failed.
func RespawnBonded(userId int, mobId int) int {
	if respawnFunc == nil {
		return 0
	}
	return respawnFunc(userId, mobId)
}
