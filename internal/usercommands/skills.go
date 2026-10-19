package usercommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

type SkillsOptions struct {
	SkillList      map[string]int
	SkillCooldowns map[string]int
	SkillBlurbs    map[string]string
	// NameWidth is the padded width of the name column: the longest name the
	// player holds, and never under 12, so the rank column lines up even for
	// "blacksmithing" and "unarmed-combat".
	NameWidth int
}

// skillNameWidth is the name column width for a skill list.
func skillNameWidth(list map[string]int) int {
	w := 12
	for name := range list {
		if len(name) > w {
			w = len(name)
		}
	}
	return w
}

func Skills(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	allSkills := user.Character.GetSkills()
	allCooldowns := map[string]int{}
	blurbs := map[string]string{}
	for skillName := range allSkills {
		allCooldowns[skillName] = user.Character.GetCooldown(skillName)
		blurbs[skillName] = skills.SkillBlurb(skillName)
	}

	skillData := SkillsOptions{
		SkillList:      allSkills, // name to level
		SkillCooldowns: allCooldowns,
		SkillBlurbs:    blurbs,
		NameWidth:      skillNameWidth(allSkills),
	}

	// A laid-out screen: CategoryScreen, so no row gains a stop, and the
	// template's trailing newline trimmed, since SendText ends the message
	// itself and a blank line is sent on purpose or not at all (#614).
	skillTxt, _ := templates.Process("character/skills", skillData, user.UserId)
	user.SendText(messaging.CategoryScreen, strings.TrimRight(skillTxt, "\r\n"))

	if rest == `extra` {
		user.SendText(messaging.CategoryScreen, `<ansi fg="yellow">Cooldown Tracking:</ansi>`)
		for name, rnds := range user.Character.GetAllCooldowns() {
			user.SendText(messaging.CategoryScreen, fmt.Sprintf(` <ansi fg="yellow">%s</ansi>: <ansi fg="red">%d</ansi>`, name, rnds))
		}
	}

	return true, nil
}
