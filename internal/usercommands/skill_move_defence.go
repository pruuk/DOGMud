package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// moveDefence is one rendered defence outcome, ready to compose into a Trio.
// Shortage is the defender's resource note, empty when there is none.
type moveDefence struct {
	ToAttacker, ToDefender, ToRoom string
	Shortage                       string
}

// moveDefenceLines renders the channel defence triad for a DEFENDED special
// move and SENDS NOTHING.
//
// RENDER UP FRONT, COMPOSE ONCE. This is the template usercommands/shoot.go:368
// and mobcommands/taunt.go:108 already follow by calling
// combat.RenderChannelDefenceMessages directly and keeping its three lines in
// locals until they are ready to speak.
//
// sendMoveDefenceTriad was the outlier: it bundled rendering with sending, so a
// caller whose room line was CONDITIONAL on a defence having spoken had to
// split one narrated event across two sends, because asking the question also
// answered it. Separating them lets every caller build one Trio.
//
// ok is false when the outcome was not a defence (e.g. a fumbled swing that
// had actually won its roll); the caller then falls back to its plain miss
// text. It is NOT false for a missing pool: as of 2026-08-31
// combat.RenderChannelDefenceMessages substitutes generic narration, so a
// defended outcome always speaks. Do not reintroduce a pool check here.
func moveDefenceLines(user *users.UserRecord, room *rooms.Room, target actions.AggroTarget,
	out combat.ChannelDefenceResult, attack string) (moveDefence, bool) {

	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	identities := combat.ChannelDefenceIdentities{
		Attacker: user.Character.GetPlayerName(user.UserId).String(),
		Defender: target.Name,
	}
	if targetUser != nil {
		identities.Defender = targetUser.Character.GetPlayerName(user.UserId).String()
	} else if targetMob := mobs.GetInstance(target.MobInstanceId); targetMob != nil {
		identities.Defender = targetMob.Character.GetMobNameIndexed(user.UserId,
			room.GetMobDuplicateIndex(targetMob.InstanceId)).String()
	}

	triad := combat.RenderChannelDefenceMessages(out, identities, attack)
	if triad.ToRoom == "" {
		return moveDefence{}, false
	}

	lines := moveDefence{
		ToAttacker: string(triad.ToAttacker),
		ToDefender: string(triad.ToDefender),
		ToRoom:     string(triad.ToRoom),
	}
	if targetUser != nil {
		lines.Shortage = combat.ChannelDefenceShortageText(out, targetUser.Character)
	}
	return lines, true
}

// sendMoveDefenceShortage speaks the defender's resource-shortage note, which
// is a separate at-most-once event rather than part of the defence narration:
// combat.sendDefenceShortageOnce dedupes it per round on the melee path and
// positions it independently there too.
//
// It lives here rather than at the call sites so the note stays in one place
// and does not have to be repeated in every verb.
func sendMoveDefenceShortage(targetUser *users.UserRecord, lines moveDefence) {
	if targetUser != nil && lines.Shortage != "" {
		targetUser.SendText(messaging.CategorySystem, lines.Shortage)
	}
}

// canSeeInDark reports whether the user can make out who is acting on them:
// the room is lit, or they carry night vision.
//
// An unexported twin of mobcommands.canSeeInDark. The two should collapse when
// M4 consolidates darkness, blindness and sleep into one perception verdict --
// internal/messaging/predicates.go:66 already flags that seam as temporary. A
// shared helper package for two call sites would be the wrong shape today.
func canSeeInDark(u *users.UserRecord, room *rooms.Room) bool {
	return room.GetVisibility() >= 1 || u.Character.HasFlagFromAnySource(buffs.NightVision)
}

// acteeDefenceLine renders the defender's personal defence line, anonymized
// when the defender cannot see who is acting on them.
//
// WHY THIS EXISTS. users.UserRecord.SendText is hardcoded to ChannelAudio and
// the pipeline runs the sight gate and the anonymizer only on ChannelVisual
// (internal/messaging/pipeline.go:59), so this line is anonymized here or not
// at all.
//
// The mob-attacker copy of the defence helper always did this. This
// player-attacker side did not, so a player defending in the dark was told who
// hit them when the attacker was a player and was not when it was a mob. Same
// duplicated-and-drifted shape as the five defects the M1 audit found, and
// found the same way: by reading the two copies against each other.
func acteeDefenceLine(targetUser *users.UserRecord, room *rooms.Room, cat messaging.Category, text string) messaging.Line {
	if targetUser == nil || text == "" {
		return messaging.NoLine
	}
	if !canSeeInDark(targetUser, room) {
		text = messaging.Anonymize(text)
	}
	return messaging.Say(cat, text)
}
