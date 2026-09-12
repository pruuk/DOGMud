package buffs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// One door for the player-side buff line: authored text first, a generic line
// underneath, silence for a secret buff. Forty-six dogmud buffs had neither a
// start nor an end line before slice C.
func TestBuffNotices(t *testing.T) {
	authored := &BuffSpec{BuffId: 1, Name: "Venom", StartUserText: "Venom burns.", EndUserText: "The venom subsides."}
	assert.Equal(t, "Venom burns.", authored.StartUserNotice())
	assert.Equal(t, "The venom subsides.", authored.EndUserNotice())

	silent := &BuffSpec{BuffId: 2, Name: "Warrior's Brew"}
	assert.Equal(t, "Warrior's Brew takes effect.", silent.StartUserNotice())
	assert.Equal(t, "Warrior's Brew has expired.", silent.EndUserNotice())

	secret := &BuffSpec{BuffId: 3, Name: "Respawn Grace", Secret: true, StartUserText: "never shown"}
	assert.Equal(t, "", secret.StartUserNotice(), "a secret buff says nothing even with authored text")
	assert.Equal(t, "", secret.EndUserNotice())

	nameless := &BuffSpec{BuffId: 4}
	assert.Equal(t, "", nameless.StartUserNotice(), "no name, no generic line; the guard catches this")
	assert.Equal(t, "", nameless.EndUserNotice())
}

func TestSilentNoticeBuffsListsOnlyNonSecretBuffsRelyingOnTheFallback(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		10: {BuffId: 10, Name: "Authored", StartUserText: "a", EndUserText: "b"},
		11: {BuffId: 11, Name: "Half", StartUserText: "a"},
		12: {BuffId: 12, Name: "Bare"},
		13: {BuffId: 13, Name: "Hidden", Secret: true},
	})
	defer restore()
	assert.ElementsMatch(t, []string{"11 Half (end)", "12 Bare (start, end)"}, SilentNoticeBuffs())
}
