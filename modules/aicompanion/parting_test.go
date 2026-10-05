package aicompanion

import (
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// When she leaves someone, what she had with them comes down to one
// sentence she keeps in the roster, and the rest of her mind of them is
// wiped: no memories, no core memories, nothing learned about them. How she
// feels about them stays (the Hollow weighs it), and so does what she knows
// of the world.
func TestPartingKeepsOneSentenceAndWipesTheRest(t *testing.T) {
	w := newConsentWorld(t, true)
	w.m.cfg.RequirePlayerKey = false
	w.m.roster = rosterState{Profiles: map[string]*rosterEntry{w.c.profile.Id: {Owner: 1, OwnerName: `Corvin`, Since: time.Now().Unix() - 3*7*86400}}}
	w.m.profiles = map[string]*Profile{w.c.profile.Id: w.c.profile}
	w.m.byMob = map[int]*Profile{w.c.profile.MobId: w.c.profile}
	w.owner.Character.Companions = []characters.CompanionInfo{{MobId: w.c.profile.MobId, InstanceId: w.her.InstanceId,
		SourceType: characters.CompanionBonded, Name: w.c.profile.Name}}
	key := mindIdentifier(1, w.c.profile.MobId)
	w.m.minds = map[string]*Mind{key: w.c.mind}
	mind := w.c.mind
	mind.Opinion = Opinion{Trust: -60, Affection: -65}
	mind.addCore(CoreMemory{Unix: 1, Text: `Corvin left me to the wolves at Scrub Draw.`})
	mind.addMemory(Memory{Unix: 1, Kind: `event`, Text: `Corvin laughed at me.`, Importance: 8}, 100)
	mind.Facts = []Fact{{Text: `Corvin has a brother.`}}
	mind.Map = map[int]*RoomRecord{5223: {}}
	mind.FirstMetUnix = 1

	util.LockMud()
	w.m.leave(w.c, w.owner, partCantBear)
	util.UnlockMud()

	if len(mind.CoreMemories) != 0 || len(mind.Memories) != 0 || len(mind.Facts) != 0 || mind.FirstMetUnix != 0 {
		t.Fatalf("what she had with them is wiped: core=%d memories=%d facts=%d", len(mind.CoreMemories), len(mind.Memories), len(mind.Facts))
	}
	if mind.Opinion.Trust > -50 || mind.Map[5223] == nil {
		t.Fatal("how she feels about them, and what she knows of the world, stay")
	}
	pt := w.m.partingOf(w.c.profile, 1)
	if pt == nil || pt.Name != `Corvin` || pt.Why != partCantBear {
		t.Fatalf("one parting kept, of Corvin, saying why: %+v", pt)
	}
	if !strings.Contains(pt.Text, `could not bear`) || !strings.Contains(pt.Text, `could not stand them`) ||
		!strings.Contains(pt.Text, `for a few weeks`) || strings.Contains(pt.Text, `Corvin`) {
		t.Fatalf("with no model, the plain sentence, not naming them: %q", pt.Text)
	}
}

// Dismissed by her owner, the sentence says so.
func TestDismissedPartingSaysSo(t *testing.T) {
	if got := fallbackParting(Opinion{Affection: 50, Trust: 50}, partSentAway, 0); !strings.Contains(got, `They sent me away.`) ||
		!strings.Contains(got, `fond of them`) {
		t.Fatalf("%q", got)
	}
}

// Away too long, offline: she still keeps a sentence of them, by the name
// she knew them by.
func TestLapsedOwnerLeavesAParting(t *testing.T) {
	hw := newHollowWorld(t)
	m, p := hw.m, hw.p
	m.cfg.ReleaseAfterDays = 60
	now := time.Now()
	m.roster.Profiles[p.Id] = &rosterEntry{Owner: 7, OwnerName: `Wenna`, Since: now.Unix() - 100*86400, LastSeen: now.Unix() - 61*86400}
	m.releaseLapsed(now)
	pt := m.partingOf(p, 7)
	if pt == nil || pt.Name != `Wenna` || pt.Why != partLapsed {
		t.Fatalf("%+v", pt)
	}
}

// The model's sentence takes the place of the plain one, unless they have
// won her back or parted again since it was asked for.
func TestModelSentenceReplacesTheFallback(t *testing.T) {
	hw := newHollowWorld(t)
	m, p := hw.m, hw.p
	e := m.rosterFor(p.Id)
	e.Partings = map[int]*Parting{1: {Name: `Corvin`, Text: `We travelled together for a while.`, Unix: 100}}
	reply := modelResult{Content: `{"text":"He was kind to me until the night he left me to the wolves."}`, Tokens: 5}

	m.applyParting(p.Id, 1, 99, hold{}, route{}, reply)
	if e.Partings[1].Text != `We travelled together for a while.` {
		t.Fatal("a reply for an older parting changes nothing")
	}
	m.applyParting(p.Id, 1, 100, hold{}, route{}, reply)
	if e.Partings[1].Text != `He was kind to me until the night he left me to the wolves.` {
		t.Fatalf("%q", e.Partings[1].Text)
	}
	m.forgetParting(p, 1)
	m.applyParting(p.Id, 1, 100, hold{}, route{}, reply)
	if m.partingOf(p, 1) != nil {
		t.Fatal("won back: nothing is written back in")
	}
}

// In the Hollow, the sentence is what she knows of someone she travelled
// with before; someone she never did is someone she has just met.
func TestTheHollowRemembersThePartingOnly(t *testing.T) {
	profiles, _ := loadProfiles()
	p := profiles[`tobin`]
	visitor := &users.UserRecord{UserId: 1, Character: &characters.Character{Name: `Corvin`}}
	in := interviewInput{Profile: p, Visitor: visitor, Mind: newMind(1, p), Now: time.Now(),
		Parting: &Parting{Name: `Corvin`, Text: `He was kind to me until the night he left me to the wolves.`, Unix: time.Now().Unix() - 86400*3}}
	text := buildInterviewMessages(in)[1].Content
	if !strings.Contains(text, `You have travelled with Corvin before`) || !strings.Contains(text, `left me to the wolves`) {
		t.Fatalf("the parting is in front of her: %s", text)
	}
	in.Parting = nil
	text = buildInterviewMessages(in)[1].Content
	if strings.Contains(text, `travelled with Corvin before`) || !strings.Contains(text, `You have only just met Corvin`) {
		t.Fatalf("no parting, no past: %s", text)
	}
}
