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
//
// A malformed line returns handled with its error, so world.go logs it once
// as an error and not again as an unknown command. The source and the verb
// read case-blind.
func Ambient(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {
	srcWord, after, _ := strings.Cut(strings.TrimSpace(rest), ` `)
	verb, text, _ := strings.Cut(strings.TrimSpace(after), ` `)
	text = strings.TrimSpace(text)

	src, ok := ambient.ParseSource(strings.ToLower(srcWord))
	if !ok {
		return true, fmt.Errorf(`ambient: unknown source %q`, srcWord)
	}
	if src == ambient.SourceSayEcho {
		// Charged only by an ambient say's echo next door, never issued.
		return true, fmt.Errorf(`ambient: %s is not a source a line can name`, src)
	}
	verb = strings.ToLower(verb)
	if verb != `emote` && verb != `say` {
		return true, fmt.Errorf(`ambient: %q is not an ambient verb`, verb)
	}
	if text == `` {
		// Refused before the budget is asked, so nobody is charged for it.
		return true, fmt.Errorf(`ambient: %s %s has no text`, src, verb)
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
	// The echo next door is ambient too: each neighbour's listeners are
	// charged and an over-budget one is left out (owner 2026-10-30).
	echo := func(tgt *rooms.Room) []int {
		return ambient.Plan(ambient.Line{Room: tgt, Source: ambient.SourceSayEcho, Perceives: ambient.HearsQuiet}).Exclude
	}
	return sayAs(&actions.MobActor{Mob: mob, Room: room, Skip: plan.Exclude, EchoExclude: echo}, text)
}
