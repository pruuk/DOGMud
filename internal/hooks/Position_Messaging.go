// Position_Messaging.go renders grapple and submission position
// messages via Position_GrappleTick.go and grapplemessaging.go:
//
//   - fireStaminaWarningIfLow: one-shot "you're getting gassed" beat
//     when stamina drops below GrappleStaminaLowThreshold. Reuses
//     the per-grapple cooldown map.
//
//   - Submission messaging: fireSubmissionOpeningMessage and
//     fireSubmissionResolutionMessage fire outcome-specific templates
//     that are registered with the combat package at init time.
//
// Templates live at <configured world>/messaging/position_control.yaml,
// which is _datafiles/world/dogmud/messaging/position_control.yaml for the
// shipped config.
//
// LOADER TIER: event narration. main.go calls LoadPositionMessages at boot and
// it PANICS on bad data, because a submission that narrates nothing leaves a
// player reading no account of an arm being snapped. See the two-tier policy in
// internal/narration/context.md.
package hooks

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state/position"
	"github.com/GoMudEngine/GoMud/internal/users"
	"gopkg.in/yaml.v3"
)

// submissionMsgTriple holds the three audience variants for a single
// submission message key. The actor and actee lines are personal messages to
// the two grapplers; the observer line goes to everyone else in the room.
//
// The authored keys were attacker/target/room until M4b-1 gave every narration
// store one role vocabulary. The Go field names keep the submission-specific
// spelling because they read better at the call sites below, where an
// attempter really is attacking; only the wire names are canonical. The
// mirror of this struct in internal/guards/shipped_narration_data_guard_test.go
// decodes STRICTLY and must be renamed in lockstep, or a tag changed here
// alone fails that guard's decode rather than silently yielding empty text.
type submissionMsgTriple struct {
	Attacker string `yaml:"actor"`
	Target   string `yaml:"actee"`
	Room     string `yaml:"observer"`
}

type submissionMessageBlock struct {
	Opening                map[string]submissionMsgTriple `yaml:"opening"`
	EscapeBad              submissionMsgTriple            `yaml:"escape_bad"`
	Neutral                submissionMsgTriple            `yaml:"neutral"`
	OutcomeMercy           submissionMsgTriple            `yaml:"outcome_mercy"`
	OutcomeSubdue          submissionMsgTriple            `yaml:"outcome_subdue"`
	OutcomeCrippleArm      submissionMsgTriple            `yaml:"outcome_cripple_arm"`
	OutcomeCrippleShoulder submissionMsgTriple            `yaml:"outcome_cripple_shoulder"`
	OutcomeLethal          submissionMsgTriple            `yaml:"outcome_lethal"`
	CritFlag               submissionMsgTriple            `yaml:"crit_flag"`
}

// positionMessageTemplates is the part of the store production reads:
// stamina_warning and submission. gradient_messages and transition_messages
// are authored in the same file and read by nobody (see the golden's note).
//
// The stamina warning's keys were self/room until M4b-1. It is the one
// asymmetric pair in the store: `actor` is the character the warning fires
// for, whichever side of the grapple they are on, which is what
// staminaWarningSubstitutions exists to arrange.
type positionMessageTemplates struct {
	StaminaWarning struct {
		Self string `yaml:"actor"`
		Room string `yaml:"observer"`
	} `yaml:"stamina_warning"`
	Submission submissionMessageBlock `yaml:"submission"`
}

var (
	posMsgOnce      sync.Once
	posMsgTemplates positionMessageTemplates
)

// positionMessagesPath returns the store's path under the CONFIGURED world.
//
// Until the two-tier loader policy landed this was the literal
// `_datafiles/messages/position_control.yaml`, outside the world tree
// entirely, which is why a server run from any other working directory read
// nothing and narrated every submission as silence with one Warn line.
func positionMessagesPath() string {
	return filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "messaging", "position_control.yaml")
}

// LoadPositionMessages loads the store from the configured world and panics on
// a read or parse error.
//
// EVENT TIER. It deliberately does NOT go through posMsgOnce: a test in this
// package that reached the store first would have spent that Once, and a boot
// check that silently becomes a no-op is not a boot check. The read and parse
// run unconditionally here; the Once is spent afterwards so a later lazy caller
// keeps what boot loaded.
func LoadPositionMessages() {
	path := positionMessagesPath()
	data, err := os.ReadFile(path)
	if err != nil {
		mudlog.Error("hooks.LoadPositionMessages: read failed", "path", path, "err", err)
		panic(fmt.Errorf("position_control: read %s: %w", path, err))
	}
	var loaded positionMessageTemplates
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		mudlog.Error("hooks.LoadPositionMessages: yaml parse failed", "path", path, "err", err)
		panic(fmt.Errorf("position_control: parse %s: %w", path, err))
	}

	posMsgTemplates = loaded
	posMsgOnce.Do(func() {})
}

// loadPositionMessages reads + parses the YAML config once. Missing
// file is a Warn, not a fatal — the hook degrades to silent.
//
// This is the TEST-reachable path only, in production terms: boot calls
// LoadPositionMessages first and spends the Once. It keeps its log-and-degrade
// behaviour because it can run during test package init, before a logger
// exists.
func loadPositionMessages() positionMessageTemplates {
	posMsgOnce.Do(func() {
		path := positionMessagesPath()
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				mudlog.Warn("Position_Messaging: template file missing", "path", path)
				return
			}
			mudlog.Error("Position_Messaging: read failed", "err", err)
			return
		}
		if err := yaml.Unmarshal(data, &posMsgTemplates); err != nil {
			mudlog.Error("Position_Messaging: yaml parse failed", "err", err)
		}
	})
	return posMsgTemplates
}

// fireStaminaWarningIfLow fires a one-shot "getting gassed" beat
// when c's stamina drops below the grapple-low threshold. Reuses the
// per-grapple cooldown map so it cannot spam.
func fireStaminaWarningIfLow(c *characters.Character) {
	if c == nil || c.Position == nil {
		return
	}
	if !c.IsLowGrappleStamina() {
		return
	}
	if c.PerGrappleMessageCooldowns == nil {
		c.PerGrappleMessageCooldowns = map[string]bool{}
	}
	const cooldownKey = "stamina_low"
	if c.PerGrappleMessageCooldowns[cooldownKey] {
		return
	}
	c.PerGrappleMessageCooldowns[cooldownKey] = true

	templates := loadPositionMessages()
	subs := staminaWarningSubstitutions(c)
	sendCharacterMsg(c,
		narration.Substitute(templates.StaminaWarning.Self, subs),
		narration.Substitute(templates.StaminaWarning.Room, subs),
	)
}

// staminaWarningSubstitutions is substitutionsForCharacter with the actor slot
// forced to c.
//
// The stamina warning is the one asymmetric line in this store: its room text
// reads "{actor} looks exhausted in the {position}." and is about the
// character the warning fires for, not about the controller. Without this
// override a controlled character's warning would name the CONTROLLER.
//
// It is its own function so the override is reachable from a test:
// fireStaminaWarningIfLow itself needs a live room and user to exercise, and
// the store golden renders templates against fixed stand-ins, so neither can
// see which name lands in the actor slot.
func staminaWarningSubstitutions(c *characters.Character) map[string]string {
	subs := substitutionsForCharacter(c)
	subs[narration.TokenActor] = c.Name
	return subs
}

// substitutionsForCharacter builds the standard token map for c's
// current grapple context. Resolves the partner's display name via
// GrappleData.Partner so room broadcasts get real names instead of
// "the other grappler" filler.
func substitutionsForCharacter(c *characters.Character) map[string]string {
	partner := resolvePartner(c)
	partnerName := ""
	if partner != nil {
		partnerName = partner.Name
	}
	// The controller is the actor and the controlled is the actee, whichever
	// of the two this character is: the templates name the sides of the
	// grapple, not the reader. A line that is about the reader instead has to
	// override the actor slot itself; fireStaminaWarningIfLow is the one such
	// caller today.
	actor, actee := c.Name, partnerName
	if !c.IsController() {
		actor, actee = partnerName, c.Name
	}
	return map[string]string{
		"{position}":         c.Position.State().String(),
		narration.TokenActor: actor,
		narration.TokenActee: actee,
	}
}

// sendCharacterMsg dispatches the self and room halves of a beat via
// messaging.SendTrio. Self goes to the player's connection (no-op for
// mobs), seated as SendTrio's Actor. Room goes to the rest of the room,
// excluding c automatically (SendTrio derives the exclusion from ActorId).
//
// c is ALWAYS the name in the room line here: the only caller,
// fireStaminaWarningIfLow, already forced that via
// staminaWarningSubstitutions before building roomMsg, so ActorName is
// simply c.Name, not a re-derivation of who controls the grapple.
func sendCharacterMsg(c *characters.Character, selfMsg, roomMsg string) {
	var actor messaging.Recipient
	var actorId int
	if u := userForCharacter(c); u != nil {
		actor = u
		actorId = u.UserId
	}
	var room messaging.Broadcaster
	if r := rooms.LoadRoom(c.RoomId); r != nil {
		room = r
	}
	// T11-followup / M4d PR3 Task 4: grapple prose names the character via
	// the canonical {actor} substitution, so route through SendTrio: a
	// shapes-only observer reads "a figure" instead of the bare name, the
	// same class of fix companion_follow.go:55 made for the follow beat.
	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategoryGrappleFlow, selfMsg),
		Actee:    messaging.NoLine,
		Observer: messaging.Say(messaging.CategoryGrappleFlow, roomMsg),
	}, messaging.Audience{
		Actor:     actor,
		ActorId:   actorId,
		ActorName: c.Name,
		ActeeName: messaging.NoName,
		Room:      room,
	})
}

// userForCharacter resolves the live user record for a player-side
// character. Mobs return nil. Uses the mobs package import indirectly
// — kept here to ensure we don't accidentally try to message a mob's
// "self" channel.
func userForCharacter(c *characters.Character) *users.UserRecord {
	if uid := c.GetUserId(); uid > 0 {
		return users.GetByUserId(uid)
	}
	return nil
}

// submissionTypeKey converts a SubmissionType to the lowercase YAML
// key used under submission.opening. Must match the keys in
// position_control.yaml exactly.
func submissionTypeKey(t position.SubmissionType) string {
	switch t {
	case position.SubArmbar:
		return "armbar"
	case position.SubRNC:
		return "rnc"
	case position.SubTriangle:
		return "triangle"
	case position.SubKimura:
		return "kimura"
	case position.SubAmericana:
		return "americana"
	case position.SubOmoplata:
		return "omoplata"
	case position.SubAnaconda:
		return "anaconda"
	default:
		return ""
	}
}

// sendSubmissionTriple dispatches a submissionMsgTriple to the
// attempter (personal), recipient (personal), and room (everyone
// else) via messaging.SendTrio. Empty string slots are silently skipped.
//
// position_control.yaml's submission templates name the OTHER grappler
// inside the actor and actee lines too, not only the observer line (e.g.
// submission.opening.armbar's actor line reads "You isolate {actee}'s
// arm...", naming the recipient in the attempter's own personal line). So
// unlike a plain self-vs-room beat, both personal lines need the other
// party's name hidden from THEIR OWN reader by that reader's own sight,
// which is exactly what SendTrio's Actor/Actee halves do; the room line
// hides both names from each observer the same way the gradient room
// broadcasts do.
func sendSubmissionTriple(
	attempter, recipient *characters.Character,
	tmpl submissionMsgTriple,
	subs map[string]string,
) {
	atkMsg := narration.Substitute(tmpl.Attacker, subs)
	tgtMsg := narration.Substitute(tmpl.Target, subs)
	roomMsg := narration.Substitute(tmpl.Room, subs)

	var actor, actee messaging.Recipient
	var actorId, acteeId int
	if ua := userForCharacter(attempter); ua != nil {
		actor = ua
		actorId = ua.UserId
	}
	if ur := userForCharacter(recipient); ur != nil {
		actee = ur
		acteeId = ur.UserId
	}
	var room messaging.Broadcaster
	if r := rooms.LoadRoom(attempter.RoomId); r != nil {
		room = r
	}

	messaging.SendTrio(messaging.Trio{
		Actor:    messaging.Say(messaging.CategorySubmission, atkMsg),
		Actee:    messaging.Say(messaging.CategorySubmission, tgtMsg),
		Observer: messaging.Say(messaging.CategorySubmission, roomMsg),
	}, messaging.Audience{
		Actor:     actor,
		ActorId:   actorId,
		ActorName: attempter.Name,
		Actee:     actee,
		ActeeId:   acteeId,
		ActeeName: recipient.Name,
		Room:      room,
	})
}

// fireSubmissionOpeningMessage sends the "opening" message for a
// submission when its window first fires, before the outcome
// resolves. Picks the phrase from position_control.yaml under
// submission.opening.<subtype-key>. Degrades gracefully when the
// key is missing (no-op).
func fireSubmissionOpeningMessage(
	attempter, recipient *characters.Character,
	subType position.SubmissionType,
) {
	if attempter == nil || recipient == nil {
		return
	}
	key := submissionTypeKey(subType)
	if key == "" {
		return
	}
	templates := loadPositionMessages()
	tmpl, ok := templates.Submission.Opening[key]
	if !ok {
		return
	}
	subs := map[string]string{
		narration.TokenActor: attempter.Name,
		narration.TokenActee: recipient.Name,
	}
	sendSubmissionTriple(attempter, recipient, tmpl, subs)
}

// fireSubmissionResolutionMessage sends the outcome message after
// the submission resolves. Picks the key based on tier + policy +
// bodyPart:
//
//   - SubTierBad                        → escape_bad
//   - SubTierNeutral                    → neutral
//   - SubTierSuccess/Crit + mercy       → outcome_mercy
//   - SubTierSuccess/Crit + subdue      → outcome_subdue
//   - SubTierSuccess/Crit + cripple arm → outcome_cripple_arm
//   - SubTierSuccess/Crit + cripple shoulder → outcome_cripple_shoulder
//   - SubTierSuccess/Crit + lethal      → outcome_lethal
//
// On a Crit, the crit_flag attacker prefix is prepended to the
// attacker's message before dispatch.
func fireSubmissionResolutionMessage(
	attempter, recipient *characters.Character,
	subType position.SubmissionType,
	tier combat.SubmissionTier,
	policy characters.SubmissionPolicy,
	bodyPart string,
) {
	if attempter == nil || recipient == nil {
		return
	}
	templates := loadPositionMessages()
	sub := templates.Submission

	var tmpl submissionMsgTriple
	switch tier {
	case combat.SubTierBad:
		tmpl = sub.EscapeBad
	case combat.SubTierNeutral:
		tmpl = sub.Neutral
	default: // SubTierSuccess, SubTierCrit
		switch policy {
		case characters.PolicyMercy:
			tmpl = sub.OutcomeMercy
		case characters.PolicySubdue:
			tmpl = sub.OutcomeSubdue
		case characters.PolicyCripple:
			if bodyPart == "shoulder" {
				tmpl = sub.OutcomeCrippleShoulder
			} else {
				// "arm" or degraded choke (body part "" already
				// redirected to subdue by ResolveSubmissionOutcome;
				// fall back to arm bucket for safety)
				tmpl = sub.OutcomeCrippleArm
			}
		case characters.PolicyLethal:
			tmpl = sub.OutcomeLethal
		default:
			tmpl = sub.OutcomeSubdue
		}
		// Crit: prepend the crit_flag prefix to the attacker text.
		if tier == combat.SubTierCrit && sub.CritFlag.Attacker != "" {
			tmpl.Attacker = sub.CritFlag.Attacker + tmpl.Attacker
		}
	}

	subs := map[string]string{
		narration.TokenActor: attempter.Name,
		narration.TokenActee: recipient.Name,
	}
	sendSubmissionTriple(attempter, recipient, tmpl, subs)
}

// narrateSubmissionEffects tells a player victim about the conditions the
// submission outcome just applied, right after it applied them.
//
// Conditions 83 Broken Limb and 84 Stunned are applied synchronously on the
// character inside internal/combat, which sends no player text anywhere in the
// package, so their authored start_actee never travelled the condition event
// that would have narrated it: a player whose arm was just snapped read the
// submission's outcome line and nothing at all about the break. Both are
// flagged silent-start, and this is the applier's side of that bargain.
//
// The attempter and the room are already served by
// fireSubmissionResolutionMessage's outcome triple; this is only the victim's
// private consequence line. A mob victim has no client, so it gets nothing.
func narrateSubmissionEffects(effects combat.SubmissionOutcomeEffects) {
	sendSilentStartText(effects.StunnedVictim, combat.StunnedConditionId)
	sendSilentStartText(effects.BrokenLimbVictim, combat.BrokenLimbConditionId)
}

// sendSilentStartText sends a silent-start condition's authored start line to c,
// when c is a player. Reads it through AuthoredStartLine, not
// StartUserNotice(), which is empty by design for a silent-start condition.
func sendSilentStartText(c *characters.Character, conditionId int) {
	if c == nil {
		return
	}
	u := userForCharacter(c)
	if u == nil {
		return
	}
	spec := conditions.GetConditionSpec(conditionId)
	if spec == nil {
		return
	}
	line := spec.AuthoredStartLine(
		c.GetCharacterName(true),
		c.GetCharacterName(false))
	if line == "" {
		return
	}
	u.SendText(messaging.CategoryConditionApply, line)
}

func init() {
	combat.RegisterSubmissionMessaging(
		fireSubmissionOpeningMessage,
		fireSubmissionResolutionMessage,
	)
}

// Keep mobs import live for future helpers that may want to resolve
// mob names — the package is used by sibling files in this directory
// (Position_GrappleTick.go) but isn't called directly here.
var _ = mobs.GetInstance
