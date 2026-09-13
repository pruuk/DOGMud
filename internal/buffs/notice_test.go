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

	namelessAuthored := &BuffSpec{BuffId: 5, StartUserText: "Still spoken.", EndUserText: "Still ended."}
	assert.Equal(t, "Still spoken.", namelessAuthored.StartUserNotice(), "authored text does not need a name")
	assert.Equal(t, "Still ended.", namelessAuthored.EndUserNotice())
}

func TestSilentNoticeBuffsListsOnlyNonSecretBuffsRelyingOnTheFallback(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		10: {BuffId: 10, Name: "Authored", StartUserText: "a", EndUserText: "b"},
		11: {BuffId: 11, Name: "Half", StartUserText: "a"},
		12: {BuffId: 12, Name: "Bare"},
		13: {BuffId: 13, Name: "Hidden", Secret: true},
	})
	defer restore()
	assert.Equal(t, []string{"11 Half (end)", "12 Bare (start, end)"}, SilentNoticeBuffs(), "sorted by id")
}

// A silent-start buff leaves the start to whatever applies it. Warcry and
// rally bypass events.Buff entirely (Character.AddBuff); the bloom detox
// drink does reach Buff_ApplyBuffs on the unscaled path, and the flag keeps
// the drink's own purge narration from being doubled. Either way the
// resolver must say nothing at start
// even when start_user_text is (wrongly) authored, and the listing must not
// flag the missing start as a problem.
func TestSilentStartBuffHasNoStartNotice(t *testing.T) {
	noText := &BuffSpec{BuffId: 79, Name: "Warcry", EndUserText: "fades", Flags: []Flag{SilentStart}}
	assert.Equal(t, "", noText.StartUserNotice())
	assert.Equal(t, "fades", noText.EndUserNotice())

	withText := &BuffSpec{BuffId: 80, Name: "Rally", StartUserText: "never shown", EndUserText: "drains", Flags: []Flag{SilentStart}}
	assert.Equal(t, "", withText.StartUserNotice(), "silent-start wins even over authored start text")
}

func TestSilentStartBuffNotListedForMissingStart(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		20: {BuffId: 20, Name: "Warcry", EndUserText: "fades", Flags: []Flag{SilentStart}},
	})
	defer restore()
	assert.Equal(t, []string{}, SilentNoticeBuffs(), "a silent-start buff with an authored end is not silent by accident")
}

// A hidden buff must never announce its end: if you can't know who spotted
// you, you can't know you've been spotted. The flag wins even over authored
// end text, and the listing must not flag the missing end as a problem.
func TestHiddenBuffHasNoEndNotice(t *testing.T) {
	noText := &BuffSpec{BuffId: 9, Name: "Hidden", StartUserText: "sneaky", Flags: []Flag{Hidden}}
	assert.Equal(t, "sneaky", noText.StartUserNotice())
	assert.Equal(t, "", noText.EndUserNotice())

	withText := &BuffSpec{BuffId: 31, Name: "Empathic Shroud", StartUserText: "shrouded", EndUserText: "never shown", Flags: []Flag{Hidden}}
	assert.Equal(t, "", withText.EndUserNotice(), "hidden wins even over authored end text")
}

func TestHiddenBuffNotListedForMissingEnd(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		21: {BuffId: 21, Name: "Hidden", StartUserText: "sneaky", Flags: []Flag{Hidden}},
	})
	defer restore()
	assert.Equal(t, []string{}, SilentNoticeBuffs(), "a hidden buff with no end text is silent by design, not by accident")
}

// A quiet buff (prone recovery, the grapple exposure) is reapplied every
// round it persists, so it never emits a start or end line by design, not by
// missing authored text. The listing must not flag either as a problem.
func TestQuietBuffNotListedForMissingNotices(t *testing.T) {
	restore := SeedBuffsForTest(map[int]*BuffSpec{
		22: {BuffId: 22, Name: "Off Balance", Flags: []Flag{Quiet}},
	})
	defer restore()
	assert.Equal(t, []string{}, SilentNoticeBuffs(), "a quiet buff with no authored text is silent by design, not by accident")
}
