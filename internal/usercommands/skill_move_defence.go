package usercommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/movenarration"
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
// and does not have to be repeated in every verb. note is
// messaging.Say(<the verb's category>, defence.Shortage): the note rides the
// category of the defence line sent beside it, since it reaches the defender
// and not the player who typed the command (M7 slice 4, #371). It leaves
// through SendTrio as the actee's line because Kick, Trip and Bash leave only
// through SendTrio (send_trio_only_guard_test.go). An empty Shortage (no
// player defender, or no shortage) sends nothing.
func sendMoveDefenceShortage(note messaging.Line, aud messaging.Audience) {
	// Private to the defender: the attacker and the room are not told that
	// the defender ran short.
	messaging.SendTrio(messaging.Trio{Actor: messaging.NoLine, Actee: note, Observer: messaging.NoLine}, aud)
}

// sendPartialMove narrates a special move that a defence blunted but did not
// stop: it still dealt damage. THE RULE (#631, owner ruling 2026-10-19): one
// outcome is told in one tier, so every role (attacker, defender, room)
// reads the move store's partial event, the tier the attacker's and the
// defender's damage lines come from. The defence triad narrates only a
// defence that stopped the move outright. Swapping the room line for the
// triad's (as every move did until #631) told the room the defence swept the
// move aside while both fighters read that it still landed.
//
// shortage is the defender's resource note from moveDefenceLines, empty when
// no defence fired or nothing ran short. The note leaves before the event,
// as it did before the rule. The mobcommands twin applies the same rule.
func sendPartialMove(verb string, event movenarration.EventKey, ids moveIdentities, aud messaging.Audience, cats moveCategories, tokens map[string]string, shortage string) {
	if shortage != "" {
		sendMoveDefenceShortage(messaging.Say(cats.Actee, shortage), aud)
	}
	sendMoveEvent(verb, event, ids, aud, cats, tokens)
}

// acteeDefenceLine renders the defender's personal defence line with the
// call-site name-hiding the audio channel cannot do for itself.
//
// WHY THIS EXISTS. users.UserRecord.SendText is hardcoded to ChannelAudio and
// the pipeline runs the sight gate and hides names only on ChannelVisual
// (internal/messaging/pipeline.go:59), so this line is hidden here or not at
// all -- that is the reason trio.go's own docstring names this file as the
// sanctioned exception to "callers hand SendTrio finished text and nothing
// more": text handed to SendTrio here has already been hidden by the reader's
// three-tier sight verdict.
//
// This reads the same shared messaging.ParticipantSight verdict as the
// mob-attacker copy of this helper (internal/mobcommands/skill_move_defence.go);
// the two used to diverge on a binary canSeeInDark predicate until this task
// collapsed them onto the shared verdict.
//
// sourceName is the attacker's BARE name (no ansi tag): every call site
// already has it as user.Character.Name.
func acteeDefenceLine(targetUser *users.UserRecord, room *rooms.Room, cat messaging.Category, text string, sourceName string) messaging.Line {
	if targetUser == nil || text == "" {
		return messaging.NoLine
	}
	sight := messaging.ParticipantSight(targetUser.Character, room)
	text = messaging.HideNames(text, []string{sourceName}, sight)
	return messaging.Say(cat, text)
}
