package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/stretchr/testify/assert"
)

// TestMeleeTargetOpts_MessagesMatchPreExtraction pins the exact player-facing
// strings each of the 11 melee verbs produced before AcquireMeleeTarget was
// extracted. These were copied verbatim from the command files; if a default
// here drifts, a player sees different text than they used to.
func TestMeleeTargetOpts_MessagesMatchPreExtraction(t *testing.T) {
	tests := []struct {
		name     string
		opts     MeleeTargetOpts
		crafting string
		prompt   string
		self     string
		charmed  string
	}{
		{
			name:     "bash",
			opts:     MeleeTargetOpts{Verb: "bash"},
			crafting: refusal.WhileBusyText("bash"),
			prompt:   "Bash whom? Try <ansi fg=\"command\">bash <name></ansi>.",
			self:     "You can't bash yourself.",
			charmed:  "You can't bash a companion.",
		},
		{
			name:     "drain",
			opts:     MeleeTargetOpts{Verb: "drain"},
			crafting: refusal.WhileBusyText("drain"),
			prompt:   "Drain whom? Try <ansi fg=\"command\">drain <name></ansi>.",
			self:     "You can't drain yourself.",
			charmed:  "You can't drain a companion.",
		},
		{
			name:     "gore",
			opts:     MeleeTargetOpts{Verb: "gore"},
			crafting: refusal.WhileBusyText("gore"),
			prompt:   "Gore whom? Try <ansi fg=\"command\">gore <name></ansi>.",
			self:     "You can't gore yourself.",
			charmed:  "You can't gore a companion.",
		},
		{
			name:     "grapple",
			opts:     MeleeTargetOpts{Verb: "grapple"},
			crafting: refusal.WhileBusyText("grapple"),
			prompt:   "Grapple whom? Try <ansi fg=\"command\">grapple <name></ansi>.",
			self:     "You can't grapple yourself.",
			charmed:  "You can't grapple a companion.",
		},
		{
			name:     "kick",
			opts:     MeleeTargetOpts{Verb: "kick"},
			crafting: refusal.WhileBusyText("kick"),
			prompt:   "Kick whom? Try <ansi fg=\"command\">kick <name></ansi>.",
			self:     "You can't kick yourself.",
			charmed:  "You can't kick a companion.",
		},
		{
			name:     "maul",
			opts:     MeleeTargetOpts{Verb: "maul"},
			crafting: refusal.WhileBusyText("maul"),
			prompt:   "Maul whom? Try <ansi fg=\"command\">maul <name></ansi>.",
			self:     "You can't maul yourself.",
			charmed:  "You can't maul a companion.",
		},
		{
			// pounce differs on BOTH the prompt and the self message.
			name: "pounce",
			opts: MeleeTargetOpts{
				Verb:          "pounce",
				PromptVerb:    "pounce on",
				SelfTargetMsg: "You can't pounce on yourself.",
				CharmedMsg:    "You can't pounce on a companion.",
			},
			crafting: refusal.WhileBusyText("pounce"),
			prompt:   "Pounce on whom? Try <ansi fg=\"command\">pounce <name></ansi>.",
			self:     "You can't pounce on yourself.",
			charmed:  "You can't pounce on a companion.",
		},
		{
			name:     "rake",
			opts:     MeleeTargetOpts{Verb: "rake"},
			crafting: refusal.WhileBusyText("rake"),
			prompt:   "Rake whom? Try <ansi fg=\"command\">rake <name></ansi>.",
			self:     "You can't rake yourself.",
			charmed:  "You can't rake a companion.",
		},
		{
			name:     "taunt",
			opts:     MeleeTargetOpts{Verb: "taunt"},
			crafting: refusal.WhileBusyText("taunt"),
			prompt:   "Taunt whom? Try <ansi fg=\"command\">taunt <name></ansi>.",
			self:     "You can't taunt yourself.",
			charmed:  "You can't taunt a companion.",
		},
		{
			name:     "throttle",
			opts:     MeleeTargetOpts{Verb: "throttle"},
			crafting: refusal.WhileBusyText("throttle"),
			prompt:   "Throttle whom? Try <ansi fg=\"command\">throttle <name></ansi>.",
			self:     "You can't throttle yourself.",
			charmed:  "You can't throttle a companion.",
		},
		{
			// trip's crafting message says "trip someone", not just "trip".
			name:     "trip",
			opts:     MeleeTargetOpts{Verb: "trip", CraftingVerb: "trip someone"},
			crafting: refusal.WhileBusyText("trip someone"),
			prompt:   "Trip whom? Try <ansi fg=\"command\">trip <name></ansi>.",
			self:     "You can't trip yourself.",
			charmed:  "You can't trip a companion.",
		},
	}

	// An empty Verb asks as attack.
	assert.Equal(t, `Attack whom? Try <ansi fg="command">attack <name></ansi>.`, MeleeTargetOpts{}.promptText())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.crafting, refusal.WhileBusyText(tt.opts.craftingVerb()), "crafting-guard message changed")
			assert.Equal(t, tt.prompt, tt.opts.promptText(), "empty-target prompt changed")
			assert.Equal(t, tt.self, tt.opts.selfTargetMsg(), "self-target message changed")
			assert.Equal(t, tt.charmed, tt.opts.charmedMsg(), "charmed-target message changed")
		})
	}
}
