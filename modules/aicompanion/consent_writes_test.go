package aicompanion

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// consentWorld is her, her owner Corvin and a passer-by Bram in one room,
// with a module driving her for Corvin, who was asked and has not answered
// (or, with agreed, has said "i agree").
type consentWorld struct {
	m     *AICompanionModule
	c     *controller
	owner *users.UserRecord
	room  *rooms.Room
	her   *mobs.Mob
}

func newConsentWorld(t *testing.T, agreed bool) *consentWorld {
	t.Helper()
	owner, _, room, her := harmWorld(t, `off`)
	m, c := consentModule()
	m.cfg.Enabled = true
	m.cfg.RecoverArrows = false
	m.bonds.Users[1].Consented = agreed
	m.saveBonds()
	c.instanceId = her.InstanceId
	c.lastAttackBy = map[int]int64{}
	m.ctrls = map[int]*controller{1: c}
	m.meetingPlace = map[int]string{}
	return &consentWorld{m: m, c: c, owner: owner, room: room, her: her}
}

// namesInMind is every piece of her mind that names Corvin or Bram.
func namesInMind(c *controller) []string {
	var out []string
	has := func(s string) bool { return strings.Contains(s, `Corvin`) || strings.Contains(s, `Bram`) }
	for _, l := range c.mind.RecentLines {
		if has(l.Text) || has(l.Speaker) {
			out = append(out, `line: `+l.Text)
		}
	}
	for _, mem := range c.mind.Memories {
		if has(mem.Text) || has(strings.Join(mem.People, ` `)) {
			out = append(out, `memory: `+mem.Text)
		}
	}
	for _, cm := range c.mind.CoreMemories {
		if has(cm.Text) {
			out = append(out, `core: `+cm.Text)
		}
	}
	return out
}

func queued(c *controller, kind string) bool {
	for _, s := range c.pending {
		if s.Kind == kind {
			return true
		}
	}
	return false
}

// Before her owner has agreed that her mind may be sent, nothing that names
// a person is written into it, on any path, while she still reacts (the
// stimulus is queued, so dispatch answers with her set lines). Each path is
// checked against its control: the same deed, once agreed, is written.
func TestNothingNamingAnyoneIsWrittenBeforeConsent(t *testing.T) {
	now := time.Now()
	paths := []struct {
		name  string
		do    func(t *testing.T, w *consentWorld)
		react string // the stimulus she still queues, "" when the path queues none
	}{
		{`gift`, func(t *testing.T, w *consentWorld) {
			w.m.onGiftAccepted(events.GiftAccepted{UserId: 2, MobInstanceId: w.c.instanceId})
		}, `gift`},
		{`gold`, func(t *testing.T, w *consentWorld) {
			w.m.onGoldGiven(events.GoldGiven{UserId: 2, MobInstanceId: w.c.instanceId, Amount: 5})
		}, `gift`},
		{`attacked`, func(t *testing.T, w *consentWorld) {
			w.m.onPlayerAttackedMob(events.PlayerAttackedMob{UserId: 2, MobInstanceId: w.c.instanceId})
		}, `attacked`},
		{`attacked by her owner, while close`, func(t *testing.T, w *consentWorld) {
			w.c.mind.Romance.Stage = romanceCourting
			w.m.onPlayerAttackedMob(events.PlayerAttackedMob{UserId: 1, MobInstanceId: w.c.instanceId})
		}, `attacked`},
		{`healed`, func(t *testing.T, w *consentWorld) {
			w.m.onHealed(events.Healed{HealerUserId: 2, MobInstanceId: w.c.instanceId})
		}, `healed`},
		{`witnessed`, func(t *testing.T, w *consentWorld) {
			victim := harmMob(t, w.room, 77, `a merchant`)
			w.m.witnessAttack(1, victim.InstanceId)
		}, `witnessed`},
		{`calledBack`, func(t *testing.T, w *consentWorld) {
			w.m.calledBack(w.c, w.her, w.owner)
		}, ``},
		{`interruptErrand`, func(t *testing.T, w *consentWorld) {
			w.c.travel = &travelPlan{Dest: 9, DestName: `the mill`, Purpose: `errand`}
			w.m.interruptErrand(w.c, `Corvin`)
		}, ``},
		{`watchParty`, func(t *testing.T, w *consentWorld) {
			w.c.partyKnown, w.c.partyKey = true, `an old party`
			w.m.watchParty(w.c, w.owner)
		}, `party`},
		{`first meeting`, func(t *testing.T, w *consentWorld) {
			w.m.firstMet(w.c, w.owner, now.Unix())
		}, ``},
		{`fallen`, func(t *testing.T, w *consentWorld) {
			w.m.handleFallen(w.c, w.owner, w.c.profile, 5, now)
		}, ``},
		{`fight over`, func(t *testing.T, w *consentWorld) {
			w.c.fight = &fightState{Enemies: map[int]string{}, EnemyUsers: map[int]string{2: `Bram`},
				WorstSelf: 100, WorstOwner: 10}
			w.m.endFight(w.c, w.her, w.owner, 3)
		}, `fight_over`},
		{`owner fell`, func(t *testing.T, w *consentWorld) {
			w.m.onPlayerDeath(events.PlayerDeath{UserId: 1, CharacterName: `Corvin`, RoomId: 1})
		}, ``},
		{`heading back`, func(t *testing.T, w *consentWorld) {
			w.her.Character.RoomId = 2
			w.c.mind.Map = map[int]*RoomRecord{
				2: {Title: `A Lane`, Exits: map[string]*ExitRecord{`north`: {To: 1}}},
				1: {Title: `A Clearing`},
			}
			w.c.apartSince = 1
			w.m.headBack(w.c, w.her, w.owner, 50)
		}, ``},
		{`back beside her owner`, func(t *testing.T, w *consentWorld) {
			w.c.travel = &travelPlan{Purpose: `return`, DestName: `Corvin`}
			w.m.advanceTravel(w.c, w.her, w.owner, 50)
		}, ``},
		// Parting ways writes nothing into her mind of them: it is wiped,
		// and the one sentence she keeps is in the roster
		// (TestPartingKeepsOneSentenceAndWipesTheRest).
		{`asking to leave`, func(t *testing.T, w *consentWorld) {
			w.m.requestLeave(w.c, w.owner, `It was time.`)
		}, ``},
		{`a line drawn at friendship`, func(t *testing.T, w *consentWorld) {
			w.m.setBoundary(w.c, `friendship`)
		}, `romance_no`},
	}
	for _, p := range paths {
		t.Run(p.name, func(t *testing.T) {
			w := newConsentWorld(t, false)
			p.do(t, w)
			if got := namesInMind(w.c); len(got) != 0 {
				t.Fatalf("before consent nothing naming anyone is written: %q", got)
			}
			if p.react != `` && !queued(w.c, p.react) {
				t.Fatalf("she still reacts with a %q stimulus: %+v", p.react, w.c.pending)
			}

			// The control: the same deed, once agreed, is remembered.
			w = newConsentWorld(t, true)
			p.do(t, w)
			if len(namesInMind(w.c)) == 0 {
				t.Fatal("after consent the deed is remembered, so the check above can fail")
			}
		})
	}
}

// Her owner's own rules still apply before consent: a gift warms her, an
// attack costs trust, whatever has been written down.
func TestOwnersRulesApplyBeforeConsent(t *testing.T) {
	w := newConsentWorld(t, false)
	w.m.onGiftAccepted(events.GiftAccepted{UserId: 1, MobInstanceId: w.c.instanceId})
	if w.c.mind.ruleChangesSince(`gift`, 0) != 1 {
		t.Fatal("her owner's gift still warms her")
	}
	w.m.onPlayerAttackedMob(events.PlayerAttackedMob{UserId: 1, MobInstanceId: w.c.instanceId})
	if w.c.mind.ruleChangesSince(`attacked`, 0) != 1 {
		t.Fatal("her owner's attack still costs her trust")
	}
}

// firstMetMemories counts the "started travelling with" memory.
func firstMetMemories(c *controller) int {
	n := 0
	for _, mem := range c.mind.Memories {
		if strings.HasPrefix(mem.Text, `I started travelling with `) {
			n++
		}
	}
	return n
}

// The first meeting happened before her owner agreed, so nothing naming
// them was written and she did not introduce herself. When they agree,
// with companion-ai on, both happen then, once.
func TestConsentLaterKeepsTheFirstMeeting(t *testing.T) {
	agree := map[string]func(w *consentWorld){
		`companion-ai on`: func(w *consentWorld) {
			if _, err := w.m.cmdAI(`on`, w.owner, w.room, 0); err != nil {
				t.Fatal(err)
			}
		},
	}
	for name, do := range agree {
		t.Run(name, func(t *testing.T) {
			w := newConsentWorld(t, false)
			w.m.firstMet(w.c, w.owner, time.Now().Unix())
			if firstMetMemories(w.c) != 0 || queued(w.c, `first_meeting`) {
				t.Fatal("control: before consent the meeting is neither written nor introduced")
			}
			do(w)
			if firstMetMemories(w.c) != 1 {
				t.Fatalf("agreeing writes the first meeting: %+v", w.c.mind.Memories)
			}
			if !queued(w.c, `first_meeting`) {
				t.Fatalf("and she introduces herself: %+v", w.c.pending)
			}

			// Off and on again is not a second first meeting.
			if _, err := w.m.cmdAI(`off`, w.owner, w.room, 0); err != nil {
				t.Fatal(err)
			}
			w.c.pending = nil
			if _, err := w.m.cmdAI(`on`, w.owner, w.room, 0); err != nil {
				t.Fatal(err)
			}
			if firstMetMemories(w.c) != 1 || queued(w.c, `first_meeting`) {
				t.Fatalf("agreeing again changes nothing: %d memories, %+v", firstMetMemories(w.c), w.c.pending)
			}
		})
	}

	// Agreeing before she ever met them in a session leaves it to firstMet.
	w := newConsentWorld(t, false)
	if _, err := w.m.cmdAI(`on`, w.owner, w.room, 0); err != nil {
		t.Fatal(err)
	}
	if firstMetMemories(w.c) != 0 || queued(w.c, `first_meeting`) {
		t.Fatal("no meeting has happened yet: nothing to write")
	}
	w.m.firstMet(w.c, w.owner, time.Now().Unix())
	if firstMetMemories(w.c) != 1 || countKind(w.c.pending, `first_meeting`) != 1 {
		t.Fatalf("firstMet then writes it and introduces her, once: %+v", w.c.pending)
	}
}

// Owner decision: nothing about a romance moves before consent. Courting
// her is answered plainly: she is a plain companion for now, and
// companion-ai on changes that.
func TestRomanceWaitsForConsent(t *testing.T) {
	w := newConsentWorld(t, false)
	w.c.profile.Romance.Romanceable = true
	msg := w.m.courtStep(w.c, w.owner)
	if !strings.Contains(msg, `plain companion`) || !strings.Contains(msg, `"companion-ai on"`) {
		t.Fatalf("before consent courting is refused plainly: %q", msg)
	}
	if w.c.mind.Romance.Stage != `` && w.c.mind.Romance.Stage != romanceNone {
		t.Fatalf("and nothing moves: %q", w.c.mind.Romance.Stage)
	}
	for _, line := range strings.Split(messaging.WrapAnsi(msg, 80), "\n") {
		if n := len([]rune(line)); n > 80 {
			t.Fatalf("wrapped at 80 columns: %d in %q", n, line)
		}
	}
	w.m.tendRomance(w.c, w.owner, time.Now().Unix())
	if queued(w.c, `romance`) || queued(w.c, `night`) {
		t.Fatalf("she raises nothing about it either: %+v", w.c.pending)
	}

	w = newConsentWorld(t, true)
	w.c.profile.Romance.Romanceable = true
	if msg := w.m.courtStep(w.c, w.owner); strings.Contains(msg, `plain companion`) {
		t.Fatalf("control: after consent it is her answer, not the refusal: %q", msg)
	}
}
