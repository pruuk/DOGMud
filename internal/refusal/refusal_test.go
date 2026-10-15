package refusal

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
}

func TestHintNamesACommand(t *testing.T) {
	assert.Equal(t, ` Try <ansi fg="command">kick <name></ansi>.`, hint("kick <name>"))
}

// fits80 is the player-copy width rule for every refusal line.
func fits80(t *testing.T, line string) {
	t.Helper()
	assert.LessOrEqual(t, util.VisibleWidth(line), 80, "%q", line)
}
