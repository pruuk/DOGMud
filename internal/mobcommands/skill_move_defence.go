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
// defence narration. note is messaging.Say(<the verb's category>,
// defence.Shortage), the category of the defence line sent beside it, and it
// leaves through SendTrio as the usercommands twin explains.
func sendMoveDefenceShortage(note messaging.Line, aud messaging.Audience) {
	// Private to the defender: the room is not told the defender ran short.
	messaging.SendTrio(messaging.Trio{Actor: messaging.NoLine, Actee: note, Observer: messaging.NoLine}, aud)
}

// acteeDefenceLine renders the defender's personal defence line with the
// call-site name-hiding the audio channel cannot do for itself.
//
// users.UserRecord.SendText is hardcoded to ChannelAudio and the pipeline runs
// the sight gate and hides names only on ChannelVisual, so this line is
// hidden here or not at all -- that is the reason trio.go's own docstring
// names this file as the sanctioned exception to "callers hand SendTrio
// finished text and nothing more": text handed to SendTrio here has already
// been hidden by the reader's three-tier sight verdict.
//
// sourceName is the attacker's BARE name (no ansi tag): every call site
// already has it as mob.Character.Name.
func acteeDefenceLine(targetUser *users.UserRecord, room *rooms.Room, cat messaging.Category, text string, sourceName string) messaging.Line {
	if targetUser == nil || text == "" {
		return messaging.NoLine
	}
	sight := messaging.ParticipantSight(targetUser.Character, room)
	text = messaging.HideNames(text, []string{sourceName}, sight)
	return messaging.Say(cat, text)
}
