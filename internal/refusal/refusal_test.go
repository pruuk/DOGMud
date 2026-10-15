package refusal

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	// Failed logs through mudlog, which panics until it is set up.
	mudlog.SetupLogger(nil, "", "", false)
}

type sent struct {
	cat  messaging.Category
	text string
}

// fakeRecipient records what a helper tells it.
type fakeRecipient struct{ got []sent }

func (f *fakeRecipient) SendText(cat messaging.Category, msg string) {
	f.got = append(f.got, sent{cat, msg})
}

// one asserts a helper sent exactly one line on cat and returns it.
func one(t *testing.T, f *fakeRecipient, cat messaging.Category) string {
	t.Helper()
	require.Len(t, f.got, 1, "a refusal is exactly one line")
	assert.Equal(t, cat, f.got[0].cat)
	return f.got[0].text
}

func TestEchoTrimsStripsAndCaps(t *testing.T) {
	assert.Equal(t, "rope", echo("  rope "))
	assert.Equal(t, "red rope", echo(`<ansi fg="red">red rope</ansi>`))
	assert.NotContains(t, echo(`<ansi fg="red"`), "<ansi", "a half tag is escaped")
	long := strings.Repeat("x", 40)
	assert.Equal(t, strings.Repeat("x", echoMax)+"...", echo(long))
	assert.Equal(t, "", echo("   "))
	assert.NotContains(t, echo("<an<ansi>si fg=\"red\">x"), "<ansi", "strip first, then escape")
}

func TestHintNamesACommand(t *testing.T) {
	assert.Equal(t, ` Try <ansi fg="command">kick <name></ansi>.`, hint("kick <name>"))
}

// fits80 is the player-copy width rule for every refusal line.
func fits80(t *testing.T, line string) {
	t.Helper()
	assert.LessOrEqual(t, util.VisibleWidth(line), 80, "%q", line)
}

func TestNotHereLinesWording(t *testing.T) {
	// The no-echo property comes from the signatures (these helpers take no
	// name) and is proven through a command by a later test (#454); this one
	// pins the wording and the category.
	for line, send := range map[string]func(Recipient){
		NotHereCreatureLine: NotHereCreature,
		NotHereThingLine:    NotHereThing,
		NothingToAimLine:    NothingToAim,
	} {
		f := &fakeRecipient{}
		send(f)
		assert.Equal(t, line, one(t, f, messaging.CategoryRefusal))
	}
	assert.Equal(t, "You don't see them here.", NotHereCreatureLine)
	assert.Equal(t, "You don't see that here.", NotHereThingLine)
	assert.Equal(t, "You can't see anything to aim at.", NothingToAimLine)
}

func TestNotCarrying(t *testing.T) {
	f := &fakeRecipient{}
	NotCarrying(f, " rope ")
	assert.Equal(t, `You aren't carrying 'rope'. Try <ansi fg="command">inventory</ansi>.`,
		one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, `You aren't carrying that. Try <ansi fg="command">inventory</ansi>.`, NotCarryingText(""))
	fits80(t, NotCarryingText(strings.Repeat("w", 60)))
	fits80(t, NotCarryingText(strings.Repeat("绳", 30)))
}

func TestCombatFamily(t *testing.T) {
	f := &fakeRecipient{}
	NoTarget(f, "kick")
	assert.Equal(t, `You have no target. Try <ansi fg="command">kick <name></ansi>.`, one(t, f, messaging.CategoryRefusal))

	f = &fakeRecipient{}
	Cooldown(f, "kick")
	assert.Equal(t, "You need a moment to recover before you can kick.", one(t, f, messaging.CategoryRefusal))

	f = &fakeRecipient{}
	WhileFighting(f, "steal")
	assert.Equal(t, "You can't steal while fighting.", one(t, f, messaging.CategoryRefusal))

	// The player-copy rule: a cooldown never names a duration.
	assert.NotRegexp(t, `[0-9]`, CooldownText("kick"))
	fits80(t, "No ambush ready. "+CooldownText("ambush"))
}

func TestMembershipFamily(t *testing.T) {
	f := &fakeRecipient{}
	NotInGuild(f)
	line := one(t, f, messaging.CategoryRefusal)
	assert.Equal(t, `You aren't in a guild. Try <ansi fg="command">guild create <TAG> <name></ansi>.`, line)
	fits80(t, line)

	f = &fakeRecipient{}
	NoCompanion(f)
	assert.Equal(t, "You have no companion travelling with you.", one(t, f, messaging.CategoryRefusal))

	f = &fakeRecipient{}
	NotPartyLeader(f, "set the loot mode")
	assert.Equal(t, "Only the party leader can set the loot mode.", one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, "Only the party leader can do that.", NotPartyLeaderText(""))
}

func TestFailedTellsPlainlyOnError(t *testing.T) {
	f := &fakeRecipient{}
	Failed(f, "", assert.AnError, "userId", 7)
	assert.Equal(t, "That didn't work. The error has been logged.", one(t, f, messaging.CategoryError))
	assert.Equal(t, "That salvage didn't work. The error has been logged.", FailedText("salvage"))
	assert.NotContains(t, f.got[0].text, assert.AnError.Error(), "the cause goes to the log, not the player")
}

func TestNoPartyInvite(t *testing.T) {
	f := &fakeRecipient{}
	NoPartyInvite(f)
	assert.Equal(t, "You have no party invitation to answer.", one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, NoPartyInviteLine, "You have no party invitation to answer.")
}

func TestRecipientFull(t *testing.T) {
	f := &fakeRecipient{}
	RecipientFull(f)
	assert.Equal(t, "They can't carry any more.", one(t, f, messaging.CategoryRefusal))
	assert.Equal(t, RecipientFullLine, "They can't carry any more.")
}
