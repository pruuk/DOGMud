package mobcommands

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// Emote sends a mob's emote through actions.SendSeen: seen by sight, a bare
// mention of its own name hidden at shapes too, and never deafen-filtered
// (NPC lines are authored content, owner ruling 6).
func Emote(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	return emoteAs(&actions.MobActor{Mob: mob, Room: room}, rest)
}

// emoteAs is Emote's body for a given actor, so an ambient emote (#317,
// ambient.go) sends the same line with its Skip list.
func emoteAs(actor *actions.MobActor, rest string) (bool, error) {
	if len(rest) == 0 {
		actions.SendSeen(actor, messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> emotes.`, actor.Mob.Character.Name), false)
		return true, nil
	}

	result := actions.Emote(rest)
	emoteText := rest
	if result.IsAlias {
		emoteText = result.AliasText
	} else if rest[0] == '@' && len(rest) > 1 {
		// The @ form, stripped as the player command strips it (#273).
		emoteText = rest[1:]
	}

	actions.SendSeen(actor, messaging.CategoryMobEmote,
		actions.FormatMobEmoteText(actor.Mob.Character.Name, emoteText), false)

	return true, nil
}
