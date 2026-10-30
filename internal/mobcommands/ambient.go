package mobcommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Ambient sends a mob's unprompted emote or say (#317), issued only by the
// idle handler and the gossip tick as `ambient <source> <emote|say> <text>`.
// Every listener over their ambient budget is left out of the room line;
// everyone else gets it exactly as a plain emote or say. A player's own
// command, a reaction, a quest line or a conversation line never comes this
// way, so none of them is ever budgeted. Content never authors it.
func Ambient(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	srcWord, after, _ := strings.Cut(strings.TrimSpace(rest), ` `)
	verb, text, _ := strings.Cut(strings.TrimSpace(after), ` `)

	src, ok := ambient.ParseSource(srcWord)
	if !ok {
		return false, fmt.Errorf(`ambient: unknown source %q`, srcWord)
	}
	verb = strings.ToLower(verb)
	if verb != `emote` && verb != `say` {
		return false, fmt.Errorf(`ambient: %q is not an ambient verb`, verb)
	}

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	if verb == `emote` {
		// A hidden actor's emote reaches no one (actions.SendSeen), so it
		// must charge no one either.
		if mob.Character.IsHidden() {
			return true, nil
		}
		plan := ambient.Plan(ambient.Line{Room: room, Source: src, Perceives: ambient.Sees(room)})
		if len(plan.Delivered) == 0 {
			return true, nil
		}
		return emoteAs(&actions.MobActor{Mob: mob, Room: room, Skip: plan.Exclude}, text)
	}

	plan := ambient.Plan(ambient.Line{Room: room, Source: src})
	if len(plan.Delivered) == 0 {
		// Nobody would hear it: no reveal, no echo next door, no
		// Communication event.
		return true, nil
	}
	return sayAs(&actions.MobActor{Mob: mob, Room: room, Skip: plan.Exclude}, text)
}
