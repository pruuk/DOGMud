package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
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
// move by a MOB and SENDS NOTHING. Mirror of the usercommands twin; see that
// file for why rendering and sending are separate.
//
// The identity resolution differs from the player copy and legitimately so: a
// mob attacker names itself with GetMobNameIndexed, a player attacker with
// GetPlayerName. That is not duplication and does not collapse.
func moveDefenceLines(mob *mobs.Mob, room *rooms.Room, target actions.AggroTarget,
	out combat.ChannelDefenceResult, attack string) (moveDefence, bool) {

	var targetUser *users.UserRecord
	if target.UserId > 0 {
		targetUser = users.GetByUserId(target.UserId)
	}

	identities := combat.ChannelDefenceIdentities{
		Attacker: mob.Character.GetMobNameIndexed(0, room.GetMobDuplicateIndex(mob.InstanceId)).String(),
		Defender: target.Name,
	}
	if targetUser != nil {
		identities.Defender = targetUser.Character.GetPlayerName(targetUser.UserId).String()
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

// sendMoveDefenceShortage speaks the defender's resource-shortage note. See the
// usercommands twin: it is a separate at-most-once event, not part of the
// defence narration.
func sendMoveDefenceShortage(targetUser *users.UserRecord, lines moveDefence) {
	if targetUser != nil && lines.Shortage != "" {
		targetUser.SendText(messaging.CategorySystem, lines.Shortage)
	}
}

// acteeDefenceLine renders the defender's personal defence line with the
// call-site anonymization the audio channel cannot do for itself.
//
// users.UserRecord.SendText is hardcoded to ChannelAudio and the pipeline runs
// the sight gate and anonymizer only on ChannelVisual, so this line is
// anonymized here or not at all. M4's perception verdict consolidation is where
// it moves into the pipeline; see internal/messaging/predicates.go:66.
func acteeDefenceLine(targetUser *users.UserRecord, room *rooms.Room, cat messaging.Category, text string) messaging.Line {
	if targetUser == nil || text == "" {
		return messaging.NoLine
	}
	if !canSeeInDark(targetUser, room) {
		text = messaging.Anonymize(text)
	}
	return messaging.Say(cat, text)
}
