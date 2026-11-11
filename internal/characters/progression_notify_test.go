package characters

import (
	"fmt"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/progression"
)

type recordedNotice struct {
	userId int
	text   string // as an 80-column reader gets it
	narrow string // as a 30-column reader gets it
}

// recordProgressionNotices installs a recording notifier for one test.
func recordProgressionNotices(t *testing.T) *[]recordedNotice {
	t.Helper()
	var got []recordedNotice
	SetProgressionNotifier(func(userId int, render func(width int) string) {
		got = append(got, recordedNotice{userId, render(80), render(30)})
	})
	t.Cleanup(func() { SetProgressionNotifier(nil) })
	return &got
}

// A rank-0 skill roll at a huge multiplier clamps to chance 1.0 under the
// pinned base of 1.0, so the gain is certain; the Fatal below proves it.
const certainSkillMultiplier = 1000.0

const notifyTestUser = 7

func assertNoRawProgressionMessages(t *testing.T) {
	t.Helper()
	if queued := events.DrainQueuedMessagesForTest(notifyTestUser); len(queued) != 0 {
		t.Errorf("progression queued %d raw events.Message %q; it must go through the notifier", len(queued), queued)
	}
}

func TestProgressionNotice_SkillBanner(t *testing.T) {
	pinCertainStatProgressionForTest(t)
	events.DrainQueuedMessagesForTest(notifyTestUser)
	got := recordProgressionNotices(t)

	c := newProgressionTestCharacter(t)
	if !c.CheckSkillProgression("weapon-combat", notifyTestUser, certainSkillMultiplier) {
		t.Fatal("pinned skill roll did not progress; the test proves nothing")
	}
	if len(*got) != 1 {
		t.Fatalf("got %d notices, want 1: %q", len(*got), *got)
	}
	n := (*got)[0]
	if n.userId != notifyTestUser || !strings.Contains(n.text, "SKILL ADVANCEMENT") {
		t.Errorf("notice = %+v, want user %d and a SKILL ADVANCEMENT banner", n, notifyTestUser)
	}
	if strings.HasSuffix(n.text, "\n") {
		t.Errorf("notice ends in a newline; UserRecord.SendText adds it")
	}
	// The banner is laid out for the reader's width (plan 3b-3): 64 glyphs
	// of rule at 80, 30 at 30.
	for _, tc := range []struct {
		text  string
		width int
	}{{n.text, 64}, {n.narrow, 30}} {
		if first := strings.SplitN(tc.text, "\n", 2)[0]; first != strings.Repeat("━", tc.width) {
			t.Errorf("banner rule is %q, want %d glyphs", first, tc.width)
		}
	}
	assertNoRawProgressionMessages(t)
}

func TestProgressionNotice_StatBanner(t *testing.T) {
	pinCertainStatProgressionForTest(t)
	events.DrainQueuedMessagesForTest(notifyTestUser)
	got := recordProgressionNotices(t)

	c := newProgressionTestCharacter(t)
	if !c.CheckStatProgression("willpower", notifyTestUser, 1.0) {
		t.Fatal("pinned stat roll did not progress; the test proves nothing")
	}
	if len(*got) != 1 || !strings.Contains((*got)[0].text, "STATISTIC INCREASED") {
		t.Fatalf("notices = %q, want one STATISTIC INCREASED banner", *got)
	}
	assertNoRawProgressionMessages(t)
}

func TestProgressionNotice_RegenLine(t *testing.T) {
	pinCertainStatProgressionForTest(t)
	events.DrainQueuedMessagesForTest(notifyTestUser)
	got := recordProgressionNotices(t)

	c := newProgressionTestCharacter(t)
	before := c.GetStatTraining("willpower")
	c.CheckRegenProgression("willpower", notifyTestUser, 1.0)
	if c.GetStatTraining("willpower") <= before {
		t.Fatal("pinned regen roll did not progress; the test proves nothing")
	}
	want := `<ansi fg="magenta">***</ansi> Your <ansi fg="yellow">willpower</ansi> grows stronger! <ansi fg="magenta">***</ansi>`
	if len(*got) != 1 || (*got)[0].text != want {
		t.Fatalf("notices = %q, want exactly [%q]", *got, want)
	}
	assertNoRawProgressionMessages(t)
}

func TestProgressionNotice_CritAndFumbleLines(t *testing.T) {
	pinCertainStatProgressionForTest(t)
	cases := []struct {
		class progression.Class
		line  string
	}{
		{progression.ClassCrit, fmt.Sprintf(`<ansi fg="magenta">***</ansi> A moment of brilliance! Your <ansi fg="yellow">%s</ansi> technique improves! <ansi fg="magenta">***</ansi>`, "weapon-combat")},
		{progression.ClassFumble, fmt.Sprintf(`<ansi fg="red">!!!</ansi> You learn from your mistake! Your <ansi fg="yellow">%s</ansi> understanding deepens. <ansi fg="red">!!!</ansi>`, "weapon-combat")},
	}
	for _, tc := range cases {
		events.DrainQueuedMessagesForTest(notifyTestUser)
		got := recordProgressionNotices(t)
		c := newProgressionTestCharacter(t)
		c.applyBonusProgression(progression.Event{Skill: "weapon-combat", Class: tc.class, Multiplier: certainSkillMultiplier}, notifyTestUser)
		// The banner first (CheckSkillProgression), then the class line.
		if len(*got) != 2 || !strings.Contains((*got)[0].text, "SKILL ADVANCEMENT") || (*got)[1].text != tc.line {
			t.Errorf("class %v notices = %q, want [banner, %q]", tc.class, *got, tc.line)
		}
		assertNoRawProgressionMessages(t)
	}
}

func TestProgressionNotice_MobAndNilNotifierSendNothing(t *testing.T) {
	pinCertainStatProgressionForTest(t)
	events.DrainQueuedMessagesForTest(notifyTestUser)

	got := recordProgressionNotices(t)
	mobLike := newProgressionTestCharacter(t)
	if !mobLike.CheckStatProgression("willpower", 0, 1.0) {
		t.Fatal("pinned stat roll did not progress; the test proves nothing")
	}
	if len(*got) != 0 {
		t.Errorf("userId 0 produced notices %q; only players are notified", *got)
	}

	SetProgressionNotifier(nil)
	c := newProgressionTestCharacter(t)
	if !c.CheckStatProgression("willpower", notifyTestUser, 1.0) {
		t.Fatal("pinned stat roll did not progress; the test proves nothing")
	}
	assertNoRawProgressionMessages(t)
}
