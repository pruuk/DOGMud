package refusal

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/stretchr/testify/assert"
)

func TestPromptWording(t *testing.T) {
	f := &fakeRecipient{}
	Prompt(f, "pet", What, "pet <name>")
	assert.Equal(t, `Pet what? Try <ansi fg="command">pet <name></ansi>.`, one(t, f, messaging.CategoryRefusal))

	assert.Equal(t, `Steal from whom? Try <ansi fg="command">steal <name></ansi>.`,
		PromptText("steal from", Whom, "steal <name>"))
	assert.Equal(t, `Deposit how much? Try <ansi fg="command">bank deposit <amount></ansi>.`,
		PromptText("deposit", HowMuch, "bank deposit <amount>"))
	assert.Equal(t, `Drop how much gold? Try <ansi fg="command">drop <amount> gold</ansi>.`,
		PromptText("drop", HowMuchGold, "drop <amount> gold"))
	assert.Equal(t, "Say what?", PromptText("say", What, ""), "no usage, no hint")
	assert.Equal(t, "Talk to whom?", PromptText("  talk to ", Whom, ""), "the verb is trimmed")
	assert.Equal(t, "What?", PromptText("  ", What, ""), "an empty verb asks the bare question")
}

func TestTalkToPlayerWording(t *testing.T) {
	f := &fakeRecipient{}
	TalkToPlayer(f)
	assert.Equal(t, `To speak with a player, try <ansi fg="command">say <message></ansi>.`, one(t, f, messaging.CategoryRefusal))
}

// promptRows is every Prompt call this slice makes (the plan's Wording
// table), so a long verb or usage cannot push a line past 80 columns.
var promptRows = []struct {
	verb  string
	ask   Ask
	usage string
}{
	{"get", What, "get <item>"}, {"drop", What, "drop <item>"},
	{"give", What, "give <item> <name>"}, {"show", What, "show <item> <name>"},
	{"put", What, "put <item> in <container>"}, {"loot", What, "loot <corpse>"},
	{"assess", What, "assess <corpse>"}, {"equip", What, "equip <item>"},
	{"throw", What, "throw <item>"}, {"storage add", What, "storage add <item>"},
	{"lock", What, "lock <exit or container>"}, {"unlock", What, "unlock <exit or container>"},
	{"defuse", What, "defuse <exit or container>"}, {"identify", What, "cast identify <item>"},
	{"cast", What, "cast <spell> <name>"}, {"consider", Whom, "consider <name>"},
	{"dismiss", Whom, "dismiss <name>"}, {"pet", What, "pet <name>"},
	{"talk to", Whom, "talk <name>"}, {"invite", Whom, "party invite <name>"},
	{"whisper to", Whom, "whisper <name> <message>"}, {"reply with", What, "reply <message>"},
	{"say", What, "say <message>"}, {"shout", What, "shout <message>"},
	{"follow", Whom, "follow <name>"}, {"shadow", Whom, "shadow <name>"},
	{"steal from", Whom, "steal <name>"}, {"plant", What, "plant <item> <name>"},
	{"plant on", Whom, "plant <item> <name>"}, {"withdraw", HowMuch, "bank withdraw <amount>"},
	{"bid", HowMuch, "auction bid <amount>"}, {"drop", HowMuchGold, "drop <amount> gold"},
	{"give", HowMuchGold, "give <amount> gold <name>"}, {"throttle", Whom, "throttle <name>"},
	{"pounce on", Whom, "pounce <name>"}, {"grapple", Whom, "grapple <name>"},
}

func TestPromptLinesFit80(t *testing.T) {
	for _, r := range promptRows {
		fits80(t, PromptText(r.verb, r.ask, r.usage))
	}
	fits80(t, TalkToPlayerLine)
}
