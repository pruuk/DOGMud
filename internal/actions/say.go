package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// SayResult contains the results of a Say action for the wrapper to use.
type SayResult struct {
	Text string
}

// Say is the one say body for a player and a mob (sight gates slice 5b). It
// reveals a hidden speaker, echoes "You hear someone talking." through the
// exits, fires the Communication event, and sends the room line through
// sendSpoken: every listener hears the words, and the speaker's name follows
// that listener's sight. A player's line keeps the deafen filter; an NPC's
// does not (owner ruling 6).
//
// The wrappers keep only their own concerns: mute, drunk text, escaping and
// the speaker's own line (player), and the no-players shortcut (mob).
func Say(actor Actor, text string) SayResult {
	char := actor.GetCharacter()

	// Speaking aloud is a noisy action: reveal if hidden.
	if char.IsHidden() {
		_ = char.Awareness.TransitionToRevealing(state.TransitionReason{
			Trigger:  awareness.TriggerNoisyAction,
			Metadata: map[string]any{"command": "say"},
		})
	}

	isSneaking := char.IsHidden()

	room := actor.GetRoom()
	room.SendTextToExits(`You hear someone talking.`, true)

	events.AddToQueue(events.Communication{
		SourceUserId:        actor.GetUserId(),
		SourceMobInstanceId: actor.GetMobInstanceId(),
		CommType:            `say`,
		Name:                actor.GetName(),
		Message:             text,
		SpeakerHidden:       isSneaking,
	})

	nameColor, textColor := "mobname", "saytext-mob"
	if actor.IsPlayer() {
		nameColor, textColor = "username", "saytext"
	}
	sendSpoken(actor, room, messaging.CategorySpeech,
		FormatSayText(actor.GetName(), text, nameColor, textColor), isSneaking)

	return SayResult{
		Text: text,
	}
}

// FormatSayText formats the say message for room display.
// nameColor is "username" for players, "mobname" for mobs.
// textColor is "saytext" for players, "saytext-mob" for mobs.
func FormatSayText(name string, text string, nameColor string, textColor string) string {
	msg := fmt.Sprintf(`<ansi fg="%s">%s</ansi> says, "<ansi fg="%s">%s</ansi>"`, nameColor, name, textColor, text)
	return util.SplitStringNL(msg, 80)
}
