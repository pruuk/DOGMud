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

	actor := &actions.MobActor{Mob: mob, Room: room}

	if len(rest) == 0 {
		actions.SendSeen(actor, messaging.CategoryMobEmote,
			fmt.Sprintf(`<ansi fg="mobname">%s</ansi> emotes.`, mob.Character.Name), false)
		return true, nil
	}

	result := actions.Emote(rest)
	emoteText := rest
	if result.IsAlias {
		emoteText = result.AliasText
	}

	actions.SendSeen(actor, messaging.CategoryMobEmote,
		actions.FormatMobEmoteText(mob.Character.Name, emoteText), false)

	return true, nil
}
