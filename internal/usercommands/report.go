package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

func Report(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	c := user.Character

	hpBar := users.RenderVitalBar(c.Health, c.HealthMax.Value,
		c.GetPoolReservation("health", c.HealthMax.Value))
	spBar := users.RenderVitalBar(c.Stamina, c.StaminaMax.Value,
		c.GetPoolReservation("stamina", c.StaminaMax.Value))
	cpBar := users.RenderVitalBar(c.Conviction, c.ConvictionMax.Value,
		c.GetPoolReservation("conviction", c.ConvictionMax.Value))

	barText := fmt.Sprintf(`HP %s  SP %s  CP %s`, hpBar, spBar, cpBar)

	rest = strings.TrimSpace(strings.ToLower(rest))

	// Mode 1: party rep — send to party chat
	if rest == "party" {
		currentParty := parties.Get(user.UserId)
		if currentParty == nil {
			user.SendText(messaging.CategorySystem, "You are not in a party.")
			return true, nil
		}

		for _, uId := range currentParty.GetMembers() {
			if uId == user.UserId {
				continue
			}
			if u := users.GetByUserId(uId); u != nil {
				u.SendText(messaging.CategorySystem, fmt.Sprintf(
					`<ansi fg="magenta">(party)</ansi> <ansi fg="username">%s</ansi> reports: %s`,
					c.Name, barText))
			}
		}
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`<ansi fg="magenta">(party)</ansi> You report: %s`, barText))
		return true, nil
	}

	// Mode 2: rep <target> — whisper report to a specific player
	if rest == "me" || rest == "self" {
		user.SendText(messaging.CategorySystem, fmt.Sprintf(`You check yourself: %s`, barText))
		return true, nil
	}

	if rest != "" {
		// Find target player in room. #454, owner call 2: a typed name
		// resolves only at full sight, or a whisper would confirm who is there
		// in the dark.
		name, refusal := actions.AimBySight(user.Character, user.UserId, room, rest, `rep`)
		if refusal != `` {
			user.SendText(messaging.CategorySystem, refusal)
			return true, nil
		}
		target, err := actions.ResolveTargetActor(room, name, actions.ResolveTargetOptions{Viewer: user.Character})
		if err == actions.ErrTargetVanished {
			user.SendText(messaging.CategorySystem, "They are no longer here.")
			return true, nil
		}
		if err != nil || !target.IsPlayer() {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`You don't see "%s" here.`, rest))
			return true, nil
		}

		targetUser := target.(*actions.UserActor).User
		if targetUser.UserId == user.UserId {
			user.SendText(messaging.CategorySystem, fmt.Sprintf(`You check yourself: %s`, barText))
			return true, nil
		}

		// Each side's line hides the other's name at that reader's sight. The
		// report is a whisper, so its sender is "Someone" (HideSpeakerNames).
		targetUser.SendText(messaging.CategorySystem, messaging.HideSpeakerNames(fmt.Sprintf(
			`<ansi fg="whisper"><ansi fg="username">%s</ansi> reports to you: %s</ansi>`,
			c.Name, barText),
			[]string{c.Name}, messaging.ParticipantSight(targetUser.Character, room)))
		user.SendText(messaging.CategorySystem, messaging.HideNames(fmt.Sprintf(
			`<ansi fg="whisper">You report to <ansi fg="username">%s</ansi>: %s</ansi>`,
			targetUser.Character.Name, barText),
			[]string{targetUser.Character.Name}, messaging.ParticipantSight(c, room)))
		return true, nil
	}

	// Mode 3 (default): rep — broadcast to room
	user.SendText(messaging.CategorySystem, fmt.Sprintf(`You report: %s`, barText))
	room.SendTextVisual(messaging.CategoryMobEmote, fmt.Sprintf(
		`<ansi fg="username">%s</ansi> reports: %s`,
		c.Name, barText), user.UserId)

	return true, nil
}
