package actions

import (
	"fmt"
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// TrackOptions parameterizes the trail-read.
type TrackOptions struct {
	// TargetNoun: name of target to actively track. Empty = trail-scan
	// current room mode.
	TargetNoun string

	// TargetFrom (mob path): "aggro" | "event" | "soft_target" | "none".
	// Ignored if TargetNoun is set. When set, the action resolves the
	// target's name from the named context source and treats it as
	// TargetNoun.
	TargetFrom string

	// CancelTracking: "stop" / "clear" semantics — remove buff 86 + clear
	// tracking misc data without rolling. UserActor wrapper handles
	// the keyword check.
	CancelTracking bool
}

// TrackingInfo is the per-visitor record used by the descriptions/track
// template. Lifted verbatim from usercommands/skill.track.go.
type TrackingInfo struct {
	Name            string
	Type            string // "mob" or "user"
	Strength        string
	NumericStrength float64
	ExitName        string
}

// TrackResult is the structured outcome.
type TrackResult struct {
	// Trail-scan mode populates Visitors.
	Visitors []TrackingInfo

	// Active-track mode populates these.
	ActiveTargetUserId    int    // 0 when not user target
	ActiveTargetMobInstId int    // 0 when not mob target
	ActiveTargetName      string // for caller messaging
	DirectionExit         string // best exit toward target
	BuffApplied           bool   // true when buff 86 applied

	// Common.
	// Detail is the resolved read quality. It REPLACES the old RollValue float:
	// once each band is its own contest there is no single roll whose magnitude
	// means anything, and exposing one invited exactly the decoupling that the
	// first attempt at this conversion shipped.
	Detail     trailDetail
	OnCooldown bool   // 1-round cooldown collision
	Reason     string // human-readable reason on failure
}

// activeTrackingBuffId is the buff applied when active tracking starts.
// See _datafiles/world/dogmud/buffs/86-active_tracking.yaml.
const activeTrackingBuffId = 86

// Track runs the Perception+Search trail-read. With TargetNoun set (or
// resolved via TargetFrom), attempts active-tracking: locates the trail
// across adjacent rooms, applies buff 86, and stores tracking-user or
// tracking-mob misc data on the actor's Character. Without a target,
// reports the visitor log of the current room (tiered by roll).
func Track(actor Actor, opts TrackOptions) TrackResult {
	result := TrackResult{Visitors: []TrackingInfo{}}

	char := actor.GetCharacter()
	room := actor.GetRoom()
	if char == nil || room == nil {
		result.Reason = "no character or room"
		return result
	}

	// "stop"/"clear" path — pure cleanup, no roll.
	if opts.CancelTracking {
		char.SetMiscData("tracking-mob", nil)
		char.SetMiscData("tracking-user", nil)
		char.SetMiscData("tracking-display-count", nil)
		char.RemoveBuff(activeTrackingBuffId)
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem, `You stop tracking.`)
		}
		return result
	}

	// Resolve target source if TargetNoun absent.
	// TargetFrom resolution is handled by the btree wrapper before calling;
	// this action only consumes TargetNoun.
	targetNoun := opts.TargetNoun

	// If the named target is right here in the room, there is nothing to track
	// down -- you can see it. Report a clean positive read before the skill
	// roll, so a tracker who would otherwise fail the roll is not told "you
	// don't see any tracks" about a creature standing in front of them.
	if targetNoun != "" {
		if name, isMob, found := findPresentTargetByNoun(room, targetNoun, actor.GetUserId()); found {
			result.ActiveTargetName = name
			if actor.IsPlayer() {
				nameTag := "username"
				if isMob {
					nameTag = "mobname"
				}
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf(`You read <ansi fg="%s">%s</ansi>'s sign in the scuffed ground -- close, here in the open with you.`, nameTag, name))
			}
			return result
		}
	}

	// Roll the Perception+Search score.
	searchScore := CalcSearchScore(char)
	// ⚠️ THE QUALITY BANDS ARE STILL STATIC WHILE THE SCORE IS NOT. Phase A
	// converted the 125 DETECTION GATE to a contest, so that one now compresses
	// toward 50% and a developed tracker no longer clears it almost always. The
	// 175 band here, and the 135/175 bands inside readRoomTrail, are unchanged
	// and remain flat targets read off the attacker's own roll.
	//
	// What that means in practice: CalcSearchScore is
	// Perception + SkillMultiplier(rank)*25, so a high-Perception tracker still
	// saturates the UPPER bands even though the entry gate now contests. U10b-1
	// made a failed track award ProgressionFailureFraction, and with the gate
	// contested that branch now fires meaningfully more often for developed
	// characters than it used to -- previously the cut was real only for
	// beginners.
	// U10b-1b: track resolves as TWO DIFFERENT KINDS of question, because it
	// asks two different things (owner ruling, 2026-08-28).
	//
	//   READING THE ROOM'S TRAIL is a static-difficulty read, aligned with
	//   search and forage: contest.AgainstDifficulty against 125/135/175.
	//
	//   TRACKING A NAMED TARGET is an OPPOSED contest against that target, the
	//   way go.go already resolves hidden detection.
	//
	// That split is what makes the ladder coherent. An earlier attempt contested
	// only the 125 gate and left the 135/175 bands reading the raw roll, which
	// DECOUPLED them -- a contest is won by out-rolling a ROLLED difficulty, not
	// by clearing a fixed number, so at score 100 fully 73.8% of successful reads
	// carried a roll below 125, and 0.70% of high rolls lost the gate outright.
	// Resolving the bands as a NESTED ladder of contests removes that by
	// construction: you cannot reach a finer band without winning the coarser one.
	//
	// The score split follows the convention U6b Task 16 already set:
	// CalcSearchScore for static-difficulty reads, CalcDetectionScore for opposed
	// contests.
	detail := resolveTrailDetail(searchScore)
	result.Detail = detail

	// awardTrack fires the round's ONE progression award, at a weight that
	// follows the outcome (U10b-1 Task 15). This used to be a FULL event on
	// every fired roll, so a tracker who read nothing trained exactly as much
	// as one who picked up a trail -- this site is a CUT on failure.
	//
	// CALLED AT EACH RESOLVED EXIT RATHER THAN ONCE UP HERE, and that placement
	// is the point. The cooldown checks below run AFTER the roll, so an award
	// made here would pay a track that was then REFUSED for cooldown -- a free
	// progression tick for spamming the verb, and one that got worse the moment
	// losing started paying. A cooldown refusal is not a resolved contest and
	// awards nothing.
	//
	// Note a sub-threshold roll returns WITHOUT consuming the cooldown, so the
	// failure paths below are genuinely resolved actions rather than refusals.
	awardTrack := func(won bool) {
		actor.AwardResolved(won, char.CandidateFor(string(skills.Search)))
	}

	// Saw nothing at all: the coarsest band was lost.
	if !detail.SeesAnything {
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem, "You don't see any tracks.")
		}
		result.Reason = "lost the trail-detection contest"
		awardTrack(false)
		return result
	}

	// Trail-scan mode (no-arg).
	if targetNoun == "" {
		if !char.TryCooldown(skills.Search.String(), "1 round") {
			result.OnCooldown = true
			if actor.IsPlayer() {
				actor.SendText(messaging.CategorySystem,
					fmt.Sprintf("You need to wait %d more rounds to use that skill again.",
						char.GetCooldown(skills.Search.String())))
			}
			return result
		}

		awardTrack(true)
		result.Visitors = readRoomTrail(room, detail, actor.GetUserId(), actor.GetMobInstanceId())
		if actor.IsPlayer() {
			renderTrailToPlayer(actor, result.Visitors)
		}
		return result
	}

	// Active-track mode. The 175 STATIC gate that used to sit here is gone: a
	// named target now defends the contest itself (owner ruling). The contest
	// runs after the target is identified, because there is nothing to contest
	// against until then.

	if !char.TryCooldown(skills.Search.String(), "1 round") {
		result.OnCooldown = true
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				fmt.Sprintf("You need to wait %d more rounds to use that skill again.",
					char.GetCooldown(skills.Search.String())))
		}
		return result
	}

	// ⚠️ NO AWARD HERE ANY MORE. It used to fire unconditionally the moment the
	// cooldown was consumed, because the 175 static gate above had already
	// decided the outcome. Now the outcome is the OPPOSED CONTEST further down,
	// so the award must follow it — awarding here as well would pay twice for
	// one action and would pay a WIN for a track that then loses the contest.

	// Find target in current room first (just reports "they are here").
	//
	// No contest: the quarry is standing in front of you, so there is no trail
	// to read and nothing to out-roll. Awarded as a win because the verb
	// resolved and told the player something true.
	if targetUser := findUserInRoomByName(room, targetNoun, actor.GetUserId()); targetUser != nil {
		awardTrack(true)
		result.ActiveTargetUserId = targetUser.UserId
		result.ActiveTargetName = targetUser.Character.Name
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				fmt.Sprintf(`<ansi fg="username">%s</ansi> is in the room with you!`, targetUser.Character.Name))
		}
		return result
	}
	if targetMob := findMobInRoomByName(room, targetNoun); targetMob != nil {
		awardTrack(true)
		result.ActiveTargetMobInstId = targetMob.InstanceId
		result.ActiveTargetName = targetMob.Character.Name
		if actor.IsPlayer() {
			actor.SendText(messaging.CategorySystem,
				fmt.Sprintf(`<ansi fg="mobname">%s</ansi> is in the room with you!`, targetMob.Character.Name))
		}
		return result
	}

	// Search visitor log of current room for a trail matching targetNoun;
	// if found, CONTEST against that target, and only then apply buff 86 +
	// store misc data + populate DirectionExit.
	if applied, miscKey, miscVal, dirExit, targetUserId, targetMobId, targetName :=
		lookupAdjacentTrail(room, targetNoun, actor.GetUserId()); applied {

		// 🔴 THE OPPOSED CONTEST. A named quarry defends with its own ability to
		// leave no trail, so a careful mover is genuinely harder to follow than
		// a careless one — which the old flat 175 threshold could not express at
		// all, since it never read the target's side.
		//
		// Uses combat.RunContest, NOT contest.AgainstDifficulty: there is a real
		// opponent, so this belongs on the opposed seam and takes ContestFloor
		// like every other opposed contest. Mirrors usercommands/go.go's hidden
		// detection, which resolves the same shape of question.
		if tgt := trackTargetCharacter(targetUserId, targetMobId); tgt != nil {
			won := combat.RunContest(
				CalcDetectionScore(char),
				[]contest.Entry{{Score: CalcSneakScoreVsObserver(tgt, char, room)}},
			).Success
			awardTrack(won)
			if !won {
				if actor.IsPlayer() {
					actor.SendText(messaging.CategorySystem,
						"You cast about for the trail, but lose it.")
				}
				result.Reason = "lost the opposed tracking contest"
				return result
			}
		} else {
			// No character behind the trail (logged out, despawned). Nothing to
			// contest against, so the read stands on the trail alone.
			awardTrack(true)
		}

		char.SetMiscData(miscKey, miscVal)
		char.SetMiscData("tracking-display-count", nil)
		actor.AddBuff(activeTrackingBuffId, "skill")
		result.BuffApplied = true
		result.ActiveTargetUserId = targetUserId
		result.ActiveTargetMobInstId = targetMobId
		result.ActiveTargetName = targetName
		result.DirectionExit = dirExit
		return result
	}

	// No trail matched the name at all. A resolved attempt that found nothing,
	// so it trains at the loss weight — the same treatment a fruitless search
	// gets.
	if actor.IsPlayer() {
		actor.SendText(messaging.CategorySystem, "You don't see any tracks.")
	}
	awardTrack(false)
	result.Reason = "no trail found in adjacent rooms"
	return result
}

// Trail-read difficulty targets. Static, because reading a room's trail is a
// read of the ROOM, not a contest with anybody.
const (
	trailDetectTarget      = 125.0 // see that there are tracks at all
	trailAllVisitorsTarget = 135.0 // see every visitor, not just the strongest
	trailExitsTarget       = 175.0 // see which way they went
)

// trailDetail is the resolved quality of one trail read.
//
// The three bands are NESTED BY CONSTRUCTION: SeesExits implies SeesAll implies
// SeesAnything. That is the whole point of resolving them here rather than
// comparing a raw roll at three separate sites, which is what decoupled them in
// the reverted first attempt.
type trailDetail struct {
	SeesAnything bool
	SeesAll      bool
	SeesExits    bool
}

// resolveTrailDetail runs the nested static-difficulty ladder, aligned with
// search and forage (owner ruling 2026-08-28).
//
// Each band is its own contest.AgainstDifficulty, and a finer band is only
// attempted once the coarser one is won. So a tracker can never know which exit
// a visitor took while failing to see that anyone passed at all — an outcome
// that independent per-band contests would produce roughly 1 time in 140 at
// high scores.
func resolveTrailDetail(searchScore float64) trailDetail {
	d := trailDetail{}
	if !contest.AgainstDifficulty(searchScore, trailDetectTarget).Success {
		return d
	}
	d.SeesAnything = true
	if !contest.AgainstDifficulty(searchScore, trailAllVisitorsTarget).Success {
		return d
	}
	d.SeesAll = true
	d.SeesExits = contest.AgainstDifficulty(searchScore, trailExitsTarget).Success
	return d
}

// trackTargetCharacter resolves the character behind a trail so the active
// track has something to contest against. Returns nil when the quarry is gone
// (logged out, despawned), which the caller treats as an uncontested read.
func trackTargetCharacter(targetUserId, targetMobId int) *characters.Character {
	if targetUserId > 0 {
		if u := users.GetByUserId(targetUserId); u != nil {
			return u.Character
		}
	}
	if targetMobId > 0 {
		if m := mobs.GetInstance(targetMobId); m != nil {
			return &m.Character
		}
	}
	return nil
}

// readRoomTrail returns the visitor list for the current room, filtered
// by the resolved detail: without SeesAll only the strongest visitor is
// returned; with it, all of them; SeesExits additionally populates ExitName via
// an adjacent-room scan.
//
// Takes the RESOLVED bands rather than a roll value on purpose — see trailDetail.
func readRoomTrail(room *rooms.Room, detail trailDetail, excludeUserId int, excludeMobInstId int) []TrackingInfo {
	out := []TrackingInfo{}
	currentMobs := room.GetMobs()
	currentUsers := room.GetPlayers()

	// Mob trails.
	for mId, timeLeft := range room.Visitors(rooms.VisitorMob) {
		if mId == excludeMobInstId {
			continue
		}
		skip := false
		for _, curId := range currentMobs {
			if mId == curId {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		m := mobs.GetInstance(mId)
		if m == nil {
			continue
		}
		info := TrackingInfo{
			Name:            m.Character.Name,
			Type:            "mob",
			Strength:        trailStrengthToString(timeLeft),
			NumericStrength: timeLeft,
		}
		if detail.SeesExits {
			info.ExitName = findExitedTrack(room, mId, rooms.VisitorMob)
		}
		if !detail.SeesAll {
			if len(out) == 0 {
				out = append(out, info)
			} else if out[0].NumericStrength < timeLeft {
				out[0] = info
			}
			continue
		}
		out = append(out, info)
	}

	// User trails.
	for uId, timeLeft := range room.Visitors(rooms.VisitorUser) {
		if uId == excludeUserId {
			continue
		}
		skip := false
		for _, curId := range currentUsers {
			if uId == curId {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		u := users.GetByUserId(uId)
		if u == nil {
			continue
		}
		info := TrackingInfo{
			Name:            u.Character.Name,
			Type:            "user",
			Strength:        trailStrengthToString(timeLeft),
			NumericStrength: timeLeft,
		}
		if detail.SeesExits {
			info.ExitName = findExitedTrack(room, uId, rooms.VisitorUser)
		}
		if !detail.SeesAll {
			if len(out) == 0 {
				out = append(out, info)
			} else if out[0].NumericStrength < timeLeft {
				out[0] = info
			}
			continue
		}
		out = append(out, info)
	}

	return out
}

// renderTrailToPlayer emits the descriptions/track template to the player.
// No-op for mob actors (caller gates on IsPlayer).
func renderTrailToPlayer(actor Actor, visitors []TrackingInfo) {
	if len(visitors) == 0 {
		actor.SendText(messaging.CategorySystem, "You don't see any tracks.")
		return
	}
	uid := actor.GetUserId()
	trackTxt, _ := templates.Process("descriptions/track", visitors, uid)
	actor.SendText(messaging.CategorySystem, trackTxt)
}

// findUserInRoomByName looks up a user in the room by prefix match.
// Returns nil if no match.
func findUserInRoomByName(room *rooms.Room, name string, excludeUserId int) *users.UserRecord {
	for _, pId := range room.GetPlayers() {
		if pId == excludeUserId {
			continue
		}
		u := users.GetByUserId(pId)
		if u == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(u.Character.Name), strings.ToLower(name)) {
			return u
		}
	}
	return nil
}

// findMobInRoomByName looks up a mob in the room by prefix match.
func findMobInRoomByName(room *rooms.Room, name string) *mobs.Mob {
	for _, mId := range room.GetMobs() {
		m := mobs.GetInstance(mId)
		if m == nil {
			continue
		}
		if strings.HasPrefix(strings.ToLower(m.Character.Name), strings.ToLower(name)) {
			return m
		}
	}
	return nil
}

// findPresentTargetByNoun looks for a mob or user currently in the room whose
// name matches the noun via keyword match (like combat targeting, so "hare"
// matches "Steppe Hare" -- broader than the prefix match above). Mobs are
// checked before users. Returns the matched display name, whether it is a mob,
// and whether anything matched.
func findPresentTargetByNoun(room *rooms.Room, noun string, excludeUserId int) (name string, isMob bool, found bool) {
	mobNames := []string{}
	for _, mId := range room.GetMobs() {
		if m := mobs.GetInstance(mId); m != nil {
			mobNames = append(mobNames, m.Character.Name)
		}
	}
	if match, closeMatch := util.FindMatchIn(noun, mobNames...); match != "" || closeMatch != "" {
		if match != "" {
			return match, true, true
		}
		return closeMatch, true, true
	}

	userNames := []string{}
	for _, uId := range room.GetPlayers() {
		if uId == excludeUserId {
			continue
		}
		if u := users.GetByUserId(uId); u != nil {
			userNames = append(userNames, u.Character.Name)
		}
	}
	if match, closeMatch := util.FindMatchIn(noun, userNames...); match != "" || closeMatch != "" {
		if match != "" {
			return match, false, true
		}
		return closeMatch, false, true
	}

	return "", false, false
}

// lookupAdjacentTrail searches the current room's visitor log for a target
// matching targetNoun. Returns (applied=true) on first hit with the
// misc-data key/value to set, the exit direction toward the target, and
// target ids. Exit direction is found by scanning adjacent rooms for the
// strongest trail.
func lookupAdjacentTrail(room *rooms.Room, targetNoun string, excludeUserId int) (
	applied bool, miscKey string, miscVal interface{}, dirExit string, targetUserId int, targetMobId int, targetName string,
) {
	// First try users in current room's visitor log.
	allUserNames := []string{}
	for uId := range room.Visitors(rooms.VisitorUser) {
		if uId == excludeUserId {
			continue
		}
		if vu := users.GetByUserId(uId); vu != nil {
			allUserNames = append(allUserNames, vu.Character.Name)
		}
	}
	if len(allUserNames) > 0 {
		if match, closeMatch := util.FindMatchIn(targetNoun, allUserNames...); match != "" || closeMatch != "" {
			pick := match
			if pick == "" {
				pick = closeMatch
			}
			for uId := range room.Visitors(rooms.VisitorUser) {
				u := users.GetByUserId(uId)
				if u == nil || u.Character.Name != pick {
					continue
				}
				return true, "tracking-user", pick, findExitedTrack(room, uId, rooms.VisitorUser), u.UserId, 0, pick
			}
		}
	}

	// Then mobs in current room's visitor log.
	allMobNames := []string{}
	for mId := range room.Visitors(rooms.VisitorMob) {
		if vm := mobs.GetInstance(mId); vm != nil {
			allMobNames = append(allMobNames, vm.Character.Name)
		}
	}
	if len(allMobNames) > 0 {
		if match, closeMatch := util.FindMatchIn(targetNoun, allMobNames...); match != "" || closeMatch != "" {
			pick := match
			if pick == "" {
				pick = closeMatch
			}
			for mId := range room.Visitors(rooms.VisitorMob) {
				m := mobs.GetInstance(mId)
				if m == nil || m.Character.Name != pick {
					continue
				}
				return true, "tracking-mob", pick, findExitedTrack(room, mId, rooms.VisitorMob), 0, m.InstanceId, pick
			}
		}
	}

	return false, "", nil, "", 0, 0, ""
}

// trailStrengthToString converts a float64 trail strength to a human-readable
// tier name. Lifted from usercommands/skill.track.go.
func trailStrengthToString(trailStrength float64) string {
	strength := int(math.Round(trailStrength * 100))
	switch {
	case strength < 15:
		return "Dead"
	case strength < 50:
		return "Weak"
	case strength < 70:
		return "Good"
	case strength < 90:
		return "Warm"
	default:
		return "Hot"
	}
}

// findExitedTrack scans adjacent rooms for the strongest trail of the given
// target, returning the exit name that leads toward it. Lifted from
// usercommands/skill.track.go (renamed to avoid conflict with the existing
// package-level function of the same name).
func findExitedTrack(room *rooms.Room, targetId int, targetType rooms.VisitorType) string {
	var bestExit string
	var bestStrength float64

	for exitName, exitInfo := range room.Exits {
		if exitInfo.Secret {
			continue
		}
		testRoom := rooms.LoadRoom(exitInfo.RoomId)
		if testRoom == nil {
			continue
		}
		for vId, vStr := range testRoom.Visitors(targetType) {
			if vId != targetId {
				continue
			}
			if vStr < bestStrength {
				continue
			}
			bestExit = exitName
			bestStrength = vStr
		}
	}
	return bestExit
}
