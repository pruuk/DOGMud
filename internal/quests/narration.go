package quests

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/narration"
	"github.com/GoMudEngine/GoMud/internal/textutil"
)

// Narration assembles a text action's variants: send_text is the Actor line
// (the triggering player), room_text the Observer line. An action sets one or
// the other; Validate refuses both.
func (a ActionDef) Narration() narration.Variants {
	return narration.Variants{Actor: textutil.Pool(a.SendText), Observer: textutil.Pool(a.RoomText)}
}

// Narrate renders a text action with the triggering player as {source}.
// A quest has no target, so {target} renders empty; RoomTextProblems refuses
// it in room_text for that reason.
func (a ActionDef) Narrate(ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(a.Narration(), ctx)
}

// Narration assembles a reward's variants: playermessage is the Actor line,
// roommessage the Observer line.
func (r QuestReward) Narration() narration.Variants {
	return narration.Variants{Actor: textutil.Pool(r.PlayerMessage), Observer: textutil.Pool(r.RoomMessage)}
}

// Narrate renders the reward lines with the completing player as {source}.
func (r QuestReward) Narrate(ctx textutil.TokenContext) narration.Roles {
	return textutil.Narrate(r.Narration(), ctx)
}

// validateNarration refuses an action that sets both send_text and room_text
// (one narration per action; ExecuteAction used to drop the room line of such
// an action silently), and any text line that is whitespace only, in actions,
// nested sequence actions, and the rewards.
func (r *Quest) validateNarration() error {
	var problems []string
	var walk func(where string, actions []ActionDef)
	walk = func(where string, actions []ActionDef) {
		for j, a := range actions {
			aw := fmt.Sprintf("%s action %d", where, j)
			if a.SendText != "" && a.RoomText != "" {
				problems = append(problems, aw+": sets both send_text and room_text; an action narrates one line")
			}
			if a.SendText != "" || a.RoomText != "" {
				if err := narration.ValidateVariants(a.Narration(), 1); err != nil {
					key := "send_text"
					if a.SendText == "" {
						key = "room_text"
					}
					problems = append(problems, aw+" "+key+": "+err.Error())
				}
			}
			if a.Sequence != nil {
				walk(aw+" sequence on_complete", a.Sequence.OnComplete)
			}
		}
	}
	for i, t := range r.Triggers {
		walk(fmt.Sprintf("trigger %d", i), t.Actions)
	}
	if r.Rewards.PlayerMessage != "" || r.Rewards.RoomMessage != "" {
		if err := narration.ValidateVariants(r.Rewards.Narration(), 1); err != nil {
			problems = append(problems, "rewards: "+err.Error())
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("quest %d (%s) narration problems:\n%s", r.QuestId, r.Name, strings.Join(problems, "\n"))
	}
	return nil
}
