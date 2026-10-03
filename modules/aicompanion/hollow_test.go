package aicompanion

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v3"
)

// hollowWorld is a module with Mara waiting in room 1 (the harm world's
// clearing, standing in for her corner of the Hollow), player keys on
// offer, and every line told to a player captured.
type hollowWorld struct {
	m     *AICompanionModule
	p     *Profile
	w     *waiter
	owner *users.UserRecord
	told  map[int][]string
}

func newHollowWorld(t *testing.T) *hollowWorld {
	t.Helper()
	owner, _, room, her := harmWorld(t, `off`)
	withWebDomain(t, `example.org`)
	m, c := consentModule()
	m.cfg.Enabled = true
	m.cfg.RequirePlayerKey = true
	m.cfg.PlayerKeys, m.cfg.RelayOrigin = true, `https://keys.example.org`
	m.relays = newRelayTable()
	m.relayCalls = newPendingRelays()
	m.minds = map[string]*Mind{}
	m.waiting = map[string]*waiter{}
	m.returning = map[string]bool{}
	m.ctrls = map[int]*controller{}
	m.meetingPlace = map[int]string{}
	delete(m.bonds.Users, 1)
	m.syncConsent()
	p := c.profile
	p.Hollow.RoomId = room.RoomId
	m.profiles = map[string]*Profile{p.Id: p}
	m.byMob = map[int]*Profile{p.MobId: p}
	m.roster = rosterState{Version: rosterVersion, Profiles: map[string]*rosterEntry{}}
	w := &waiter{profile: p, instanceId: her.InstanceId}
	m.waiting[p.Id], w.seq = w, 5
	mobs.SetInstanceForTest(her.InstanceId, her)
	hw := &hollowWorld{m: m, p: p, w: w, owner: owner, told: map[int][]string{}}
	m.tell = func(userId int, text string) { hw.told[userId] = append(hw.told[userId], text) }
	return hw
}

// There is one of each companion: while someone has her, nobody else can,
// and one account travels with one at a time.
func TestRosterIsOneOfEach(t *testing.T) {
	m := &AICompanionModule{roster: rosterState{Profiles: map[string]*rosterEntry{}}}
	mara := &Profile{Id: `mara`, Name: `Mara Venn`, MobId: 9800}
	hal := &Profile{Id: `hal`, Name: `Hal Grimsby`, MobId: 9809}
	m.profiles = map[string]*Profile{`mara`: mara, `hal`: hal}

	if err := m.claim(mara, 1); err != nil {
		t.Fatalf("a free companion can be claimed: %v", err)
	}
	if err := m.claim(mara, 2); err == nil {
		t.Fatal("a companion with someone else cannot be claimed")
	}
	if err := m.claim(hal, 1); err == nil {
		t.Fatal("an account travels with one companion at a time")
	}
	if m.heldBy(1) != mara || m.holderOf(`mara`) != 1 {
		t.Fatal("the roster says who has whom")
	}
	m.unclaim(mara, partReleased)
	if !m.returning[`mara`] {
		t.Fatal("a companion freed from someone walks back into the Hollow")
	}
	if err := m.claim(mara, 2); err != nil {
		t.Fatalf("once free she can be won by someone else: %v", err)
	}
}

// What she learns goes with her: read off a live companion, written into
// her next owner's record, and put back onto a fresh body.
func TestKeptProgressTravelsWithHer(t *testing.T) {
	_, _, room, her := harmWorld(t, `off`)
	_ = room
	her.Character.Skills = map[string]int{`ranged-combat`: 7, `search`: 3}
	her.Character.SkillUseCount = map[string]int{`ranged-combat`: 140}
	her.Character.SpellBook = map[string]int{`heal`: 1}
	her.Character.Stats.Dexterity.Training = 4
	k := progressOfMob(her)
	if k.empty() || k.Skills[`ranged-combat`] != 7 || k.StatTraining[`dexterity`] != 4 {
		t.Fatalf("progress read off the live companion: %+v", k)
	}

	info := characters.CompanionInfo{}
	k.toInfo(&info)
	if info.Skills[`search`] != 3 || info.SpellBook[`heal`] != 1 || info.StatTraining[`dexterity`] != 4 {
		t.Fatalf("progress written into the new owner's record: %+v", info)
	}
	if back := progressOfInfo(&info); back.Skills[`ranged-combat`] != 7 {
		t.Fatalf("and read back from it: %+v", back)
	}

	fresh := harmMob(t, room, 43, `Mara Venn`)
	applyProgress(fresh, k)
	if fresh.Character.Skills[`ranged-combat`] != 7 || fresh.Character.Stats.Dexterity.Training != 4 ||
		!fresh.Character.HasSpell(`heal`) {
		t.Fatalf("progress put back onto a fresh body: %+v", fresh.Character.Skills)
	}
	k.Skills[`search`] = 99
	if fresh.Character.Skills[`search`] == 99 {
		t.Fatal("the body gets its own copy, not the roster's map")
	}
}

// An owner away for ReleaseAfterDays loses her; one who is online, or not
// yet away that long, does not. The one who lost her is told at login.
func TestReleaseAfterDaysAway(t *testing.T) {
	hw := newHollowWorld(t)
	m, p := hw.m, hw.p
	m.cfg.ReleaseAfterDays = 60
	day := int64(86400)
	now := time.Now()

	// Away 61 days, offline (account 7 has no user record).
	m.roster.Profiles[p.Id] = &rosterEntry{Owner: 7, Since: now.Unix() - 100*day, LastSeen: now.Unix() - 61*day}
	m.releaseLapsed(now)
	if m.holderOf(p.Id) != 0 {
		t.Fatal("an owner away longer than ReleaseAfterDays loses her")
	}
	if rec := m.bonds.Users[7]; rec == nil || !strings.Contains(rec.Notice, `Waystone Hollow`) {
		t.Fatalf("they are told at their next login: %+v", rec)
	}

	// Away 59 days.
	m.roster.Profiles[p.Id] = &rosterEntry{Owner: 7, Since: now.Unix() - 100*day, LastSeen: now.Unix() - 59*day}
	m.releaseLapsed(now)
	if m.holderOf(p.Id) != 7 {
		t.Fatal("not yet away long enough")
	}

	// Online, whatever the file says.
	m.roster.Profiles[p.Id] = &rosterEntry{Owner: 1, Since: now.Unix() - 100*day, LastSeen: now.Unix() - 90*day}
	m.releaseLapsed(now)
	if m.holderOf(p.Id) != 1 || m.roster.Profiles[p.Id].LastSeen != now.Unix() {
		t.Fatal("an owner who is online is seen, not released")
	}
}

// She leaves an owner she has come to distrust and dislike badly enough,
// and will not set out with someone she distrusts or dislikes.
func TestAbandonAndTheFloorUnderJoining(t *testing.T) {
	m := &AICompanionModule{cfg: buildConfig(nil)}
	if !m.abandons(Opinion{Trust: -50, Affection: -50}) {
		t.Fatal("at AbandonBelow on both, she goes")
	}
	if m.abandons(Opinion{Trust: -50, Affection: -10}) || m.abandons(Opinion{Trust: -80, Affection: 20}) {
		t.Fatal("one bad axis alone is not enough")
	}
	if !m.wouldTravelWith(Opinion{}) {
		t.Fatal("a stranger she has nothing against is someone she could go with")
	}
	if m.wouldTravelWith(Opinion{Trust: -30}) || m.wouldTravelWith(Opinion{Affection: -30}) {
		t.Fatal("not with someone she distrusts or dislikes")
	}
}

// Without a key of their own, a player gets no answer at all: nothing is
// queued and nothing is said to them. The sign at the mouth told them.
func TestHollowIgnoresPlayersWithoutAKey(t *testing.T) {
	hw := newHollowWorld(t)
	hw.m.bondRecordFor(1).SignRead = true
	hw.m.hearInHollow(hw.owner, 1, `Hello, Mara.`)
	hw.m.hearInHollow(hw.owner, 1, `Mara? Hello?`)
	if len(hw.w.pending) != 0 || len(hw.told[1]) != 0 || hw.m.consented(1) {
		t.Fatalf("no answer, no notice, no consent: pending=%+v told=%q", hw.w.pending, hw.told[1])
	}

	// The control: with the requirement off, the same words are heard.
	hw.m.cfg.RequirePlayerKey = false
	hw.m.hearInHollow(hw.owner, 1, `Hello, Mara.`)
	if len(hw.w.pending) != 1 {
		t.Fatalf("control: without the requirement she hears them: %+v", hw.w.pending)
	}
}

// The board at the mouth is shown, once, to whoever steps into its room.
func TestTheSignIsShownOnceAtTheMouth(t *testing.T) {
	hw := newHollowWorld(t)
	hw.m.cfg.HollowSignRoom = 1 // the fixture's room stands in for the Mouth
	hw.m.tendSign()
	hw.m.tendSign()
	if !hw.m.bondRecordFor(1).SignRead {
		t.Fatal("stepping into the Mouth shows the sign")
	}
	if got := strings.Count(strings.Join(hw.told[1], ``), `PLAYED BY AN AI`); got != 1 {
		t.Fatalf("once, not every round: shown %d times", got)
	}
	if hw.m.consented(1) {
		t.Fatal("reading the sign is not agreeing; speaking to them is")
	}
}

// Consent is the sign: someone who has read it and speaks to a traveller
// has agreed, and is heard. Someone who has not read it is shown it, and
// what they said is not sent. Someone who said "companion-ai off" is never
// heard.
func TestSpeakingAfterTheSignIsConsent(t *testing.T) {
	hw := newHollowWorld(t)
	hw.m.relays.ready(1, `player-model`, false)

	hw.m.hearInHollow(hw.owner, 1, `Hello, Mara.`)
	if len(hw.w.pending) != 0 || hw.m.consented(1) || !hw.m.bondRecordFor(1).SignRead ||
		!strings.Contains(strings.Join(hw.told[1], ``), `PLAYED BY AN AI`) {
		t.Fatalf("not past the sign yet: shown it, nothing sent: pending=%+v", hw.w.pending)
	}
	hw.m.hearInHollow(hw.owner, 1, `Hello, Mara. I have walked the Marches.`)
	if !hw.m.consented(1) || !hw.m.consent.allows(1) {
		t.Fatal("having read the sign, speaking to her is agreeing, and the door knows it")
	}
	if len(hw.w.pending) != 1 || hw.w.pending[0].Kind != `said` {
		t.Fatalf("and she hears them: %+v", hw.w.pending)
	}
	mind := hw.m.getMind(1, hw.p)
	if mind.Courtship.Since == 0 || len(mind.RecentLines) == 0 {
		t.Fatal("the courtship starts in the mind she would have of them")
	}

	// Refused: never heard, and not consented by speaking.
	hw2 := newHollowWorld(t)
	hw2.m.relays.ready(1, `player-model`, false)
	rec := hw2.m.bondRecordFor(1)
	rec.SignRead, rec.Refused = true, true
	hw2.m.hearInHollow(hw2.owner, 1, `Hello, Mara.`)
	if hw2.m.consented(1) || len(hw2.w.pending) != 0 {
		t.Fatal("companion-ai off is respected in the Hollow")
	}
}

// Showing her something is evidence, kept in a few words, and says when
// they made it themselves, or that it is plain hand-made work.
func TestShowingHerSomething(t *testing.T) {
	hw := newHollowWorld(t)
	hw.m.relays.ready(1, `player-model`, false)
	hw.m.bonds.Users[1] = &bondRecord{Consented: true}
	hw.m.syncConsent()

	if !hw.m.handleShow(1, hw.w.instanceId, `a yew longbow`, `A fine bow.`, `Corvin`, true) {
		t.Fatal("a waiting companion takes the show")
	}
	if len(hw.w.pending) != 1 || hw.w.pending[0].Kind != `shown` ||
		!strings.Contains(hw.w.pending[0].Text, `made by Corvin themselves`) {
		t.Fatalf("what they showed, and that they made it: %+v", hw.w.pending)
	}
	hw.m.handleShow(1, hw.w.instanceId, `a hearty stew`, ``, ``, true)
	if !strings.Contains(hw.w.pending[1].Text, `plain hand-made work`) {
		t.Fatalf("crafted below the maker's mark: %+v", hw.w.pending[1])
	}
	hw.m.handleShow(1, hw.w.instanceId, `trail rations`, ``, ``, false)
	if strings.Contains(hw.w.pending[2].Text, `hand-made`) {
		t.Fatalf("bought goods are not hand-made: %+v", hw.w.pending[2])
	}
	if got := hw.m.getMind(1, hw.p).Courtship.Shown; len(got) != 3 || got[0] != `a yew longbow` {
		t.Fatalf("kept in a few words: %q", got)
	}
	if hw.m.handleShow(1, 999, `a stone`, ``, ``, false) {
		t.Fatal("any other mob is left to the engine")
	}
}

// interviewReply is a model reply in the Hollow.
func interviewReply(verdict string, rise int) modelResult {
	return modelResult{Content: `{"intent":"x","speech":[{"kind":"say","text":"Well."}],"mood":"calm",` +
		`"memory":{"text":"","importance":1,"emotion":"neutral"},"facts":[],` +
		`"opinion":{"trust":` + itoa(rise) + `,"respect":` + itoa(rise) + `,"affection":` + itoa(rise) + `,"reason":"r"},` +
		`"verdict":"` + verdict + `"}`, Tokens: 10}
}

func itoa(n int) string {
	if n < 0 {
		return `-` + itoa(-n)
	}
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

// Nobody signs her on. She decides, and asks them herself whether they
// would like her company (offer); only their yes to that (join) takes her
// out of the Hollow. Her offer counts only once her opinion of them has
// risen; a join with no standing offer, or with no rise before it, is a
// hesitation in the world, with no system line.
func TestSheAsksAndTheirYesMakesIt(t *testing.T) {
	hw := newHollowWorld(t)
	m, p, w := hw.m, hw.p, hw.w
	m.relays.ready(1, `player-model`, false)
	m.bonds.Users[1] = &bondRecord{Consented: true}
	m.syncConsent()
	said := func(text string) []hollowStim {
		return []hollowStim{{UserId: 1, Kind: `said`, Speaker: `Corvin`, Text: text}}
	}
	w.seq = 5
	mind := m.getMind(1, p)
	her := mobs.GetInstance(w.instanceId)

	// "Come with me", and a yes in the same breath: there was no offer.
	m.applyInterview(p.Id, 5, 1, said(`Come with me, Mara.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`join`, 0))
	if m.holderOf(p.Id) != 0 || len(hw.told[1]) != 0 {
		t.Fatalf("no offer, so no going, and nothing said out of the world: %q", hw.told[1])
	}

	// She offers before any rise: the offer does not stand.
	m.applyInterview(p.Id, 5, 1, said(`I am Corvin.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`offer`, 0))
	if mind.Courtship.OfferedAt != 0 {
		t.Fatal("an offer with no reason to think well of them does not stand")
	}

	// A rise, and in the same moment she asks: it stands.
	m.applyInterview(p.Id, 5, 1, said(`I walked the Stillwater road.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`offer`, 2))
	if mind.Courtship.Gains != 1 || !mind.Courtship.offerStanding(time.Now().Unix()) {
		t.Fatalf("a rise and her asking: gains=%d offered=%d", mind.Courtship.Gains, mind.Courtship.OfferedAt)
	}

	// A stale reply changes nothing.
	m.applyInterview(p.Id, 4, 1, said(`Yes.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`join`, 0))
	if len(hw.told[1]) != 0 {
		t.Fatal("a reply for a call she no longer waits on is dropped")
	}

	// An offer gone stale is no offer.
	mind.Courtship.OfferedAt = time.Now().Unix() - offerStandsFor - 1
	m.applyInterview(p.Id, 5, 1, said(`Yes, come.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`join`, 0))
	if len(hw.told[1]) != 0 || mind.Courtship.OfferedAt != 0 {
		t.Fatal("an answer to an offer long gone is no answer")
	}

	// She asks again, and they say yes: she sets out with them (the fixture
	// has no mob template, so bonding stops at the spawn and the claim is
	// given straight back).
	m.applyInterview(p.Id, 5, 1, said(`Tell me about the road.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`offer`, 0))
	m.applyInterview(p.Id, 5, 1, said(`Yes. I would like that.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`join`, 0))
	tried := false
	for _, l := range hw.told[1] {
		if strings.Contains(l, `could not be spawned`) {
			tried = true
		}
	}
	if !tried {
		t.Fatalf("their yes to her offer sets her out with them: %q", hw.told[1])
	}
	if m.holderOf(p.Id) != 0 {
		t.Fatal("a bond that could not be made gives the claim back")
	}

	// not_them takes an offer back. (The failed setting-out took her out of
	// her room; the round tick would put her back.)
	m.waiting[p.Id], w.seq = w, 5
	mobs.SetInstanceForTest(her.InstanceId, her)
	mind.Courtship.OfferedAt = time.Now().Unix()
	m.applyInterview(p.Id, 5, 1, said(`No.`), hold{}, route{kind: routeRelay}, `m`, interviewReply(`not_them`, 0))
	if mind.Courtship.OfferedAt != 0 {
		t.Fatal("a no ends the offer")
	}
}

// The move to one of each: a companion on a record the roster does not
// give to that player is taken back at their login, her gear is handed to
// them except her own starting kit, and the progress on their record is
// not hers to carry.
func TestReclaimHandsBackGearButNotHerKit(t *testing.T) {
	hw := newHollowWorld(t)
	m, p := hw.m, hw.p
	her := hw.w.instanceId
	delete(m.waiting, p.Id) // she is out with them, not waiting

	mob := mobs.GetInstance(her)
	kit := items.Item{ItemId: p.StartingItems[0]}
	gift := items.Item{ItemId: 10004}
	mob.Character.Items = []items.Item{kit, gift}
	mob.Character.Gold = 12
	mob.Character.Skills = map[string]int{`ranged-combat`: 30}
	hw.owner.Character.Companions = []characters.CompanionInfo{{MobId: p.MobId, InstanceId: her, SourceType: characters.CompanionBonded, Name: p.Name}}
	gold := hw.owner.Character.Gold

	m.reclaimFrom(hw.owner, p)
	if comp, _ := m.bondedCompanionOf(hw.owner); comp != nil {
		t.Fatal("she is gone from their record")
	}
	if hw.owner.Character.Gold != gold+12 {
		t.Fatalf("her coin is handed back: %d", hw.owner.Character.Gold)
	}
	gotGift := false
	for _, it := range hw.owner.Character.Items {
		if it.ItemId == kit.ItemId {
			t.Fatal("her own starting kit goes with her")
		}
		if it.ItemId == gift.ItemId {
			gotGift = true
		}
	}
	if room := rooms.LoadRoom(1); room != nil {
		for _, it := range room.Items {
			if it.ItemId == kit.ItemId {
				t.Fatal("her own starting kit is not left on the floor either")
			}
			if it.ItemId == gift.ItemId {
				gotGift = true
			}
		}
	}
	if !gotGift {
		t.Fatalf("what she carried is handed over, to their pack or the floor: %+v", hw.owner.Character.Items)
	}
	if !m.rosterFor(p.Id).Kept.empty() {
		t.Fatal("progress on a record the roster did not give them is not carried")
	}
	if len(hw.told[1]) == 0 || !strings.Contains(hw.told[1][0], `Waystone Hollow`) {
		t.Fatalf("they are told where she went: %q", hw.told[1])
	}
}

// With RequirePlayerKey the server's key is never a route, and to a player
// without a key of their own she says nothing at all, not set lines. The
// control is the same drive with the requirement off.
func TestCompanionsSpeakOnlyToPlayersWithAKey(t *testing.T) {
	_, _, _, her := harmWorld(t, `off`)
	srv, hits := countingServer(t)
	for _, required := range []bool{false, true} {
		m, c := senderModule(srv.URL, true)
		m.cfg.RequirePlayerKey = required
		c.instanceId = her.InstanceId
		m.ctrls = map[int]*controller{1: c}
		m.minds = map[string]*Mind{mindIdentifier(c.mind.OwnerUserId, c.mind.MobId): c.mind}
		hits.Store(0)

		if got := m.route(1).kind; (got == routeServer) == required {
			t.Fatalf("required=%v: route %v", required, got)
		}
		util.LockMud()
		c.push(stimulus{Kind: `heard`, Speaker: `Corvin`, Text: `Mara, hello`, FromOwner: true})
		m.dispatch(c)
		inFlight, said := c.inFlight, len(c.mind.RecentLines)
		c.cancelInFlight()
		util.UnlockMud()
		m.decisions.Wait()
		if required && (inFlight || said != 0) {
			t.Fatalf("without a key she neither calls nor answers: inFlight=%v lines=%d", inFlight, said)
		}
		if !required && !inFlight {
			t.Fatal("control: on the server's key the same words start a call")
		}
	}
}

// Trust and affection both at the bottom: she goes back to the Hollow on
// her own, a couple of rounds later, without asking to.
func TestSheAbandonsAnOwnerSheCannotBear(t *testing.T) {
	w := newConsentWorld(t, true)
	w.m.cfg.RequirePlayerKey = false
	w.m.roster = rosterState{Profiles: map[string]*rosterEntry{w.c.profile.Id: {Owner: 1}}}
	w.m.profiles = map[string]*Profile{w.c.profile.Id: w.c.profile}
	w.m.byMob = map[int]*Profile{w.c.profile.MobId: w.c.profile}
	w.owner.Character.Companions = []characters.CompanionInfo{{MobId: w.c.profile.MobId, InstanceId: w.her.InstanceId,
		SourceType: characters.CompanionBonded, Name: w.c.profile.Name}}
	w.m.minds = map[string]*Mind{mindIdentifier(1, w.c.profile.MobId): w.c.mind}
	w.c.greeted = true
	w.c.mind.Opinion = Opinion{Trust: -60, Respect: 0, Affection: -55}
	w.her.Character.Skills = map[string]int{`ranged-combat`: 12}

	util.LockMud()
	w.m.sync(100)
	at := w.c.leaveAt
	util.UnlockMud()
	if at == 0 {
		t.Fatal("she means to go")
	}
	util.LockMud()
	w.m.sync(at)
	util.UnlockMud()
	if comp, _ := w.m.bondedCompanionOf(w.owner); comp != nil {
		t.Fatal("she has gone")
	}
	if w.m.holderOf(w.c.profile.Id) != 0 {
		t.Fatal("and is free again, waiting in the Hollow")
	}
	if got := w.m.rosterFor(w.c.profile.Id).Kept.Skills[`ranged-combat`]; got != 12 {
		t.Fatalf("taking what she learned with her: %d", got)
	}
}

// Every shipped companion has their world: a mob template of their own, a
// room of their own in the Hollow, the spells and starting things they are
// given, and romance. Read from the shipped files, since the test binary
// loads no world.
func TestShippedCompanionsHaveTheirWorld(t *testing.T) {
	profiles, errs := loadProfiles()
	if len(errs) > 0 {
		t.Fatalf("profiles: %v", errs)
	}
	if len(profiles) != 6 {
		t.Fatalf("six companions ship, got %d", len(profiles))
	}
	_, here, _, _ := runtime.Caller(0)
	world := filepath.Join(filepath.Dir(here), `..`, `..`, `_datafiles`, `world`, `dogmud`)
	exists := func(pattern string) bool {
		m, _ := filepath.Glob(filepath.Join(world, pattern))
		return len(m) == 1
	}
	rooms := map[int]string{}
	firstNames := map[string]string{}
	for id, p := range profiles {
		if !exists(fmt.Sprintf(`mobs/summons/%d-*.yaml`, p.MobId)) {
			t.Errorf("%s: no mob template %d", id, p.MobId)
		}
		if !exists(fmt.Sprintf(`rooms/*/%d.yaml`, p.Hollow.RoomId)) {
			t.Errorf("%s: no Hollow room %d", id, p.Hollow.RoomId)
		}
		if other, dup := rooms[p.Hollow.RoomId]; dup {
			t.Errorf("%s and %s share a Hollow room", id, other)
		}
		rooms[p.Hollow.RoomId] = id
		if other, dup := firstNames[strings.ToLower(p.FirstName())]; dup {
			t.Errorf("%s and %s share a first name", id, other)
		}
		firstNames[strings.ToLower(p.FirstName())] = id
		for _, s := range p.StartingSpells {
			if !exists(`spells/` + s + `.yaml`) {
				t.Errorf("%s: no spell %s", id, s)
			}
		}
		for _, it := range p.StartingItems {
			if !exists(fmt.Sprintf(`items/*/%d-*.yaml`, it)) && !exists(fmt.Sprintf(`items/*/*/%d-*.yaml`, it)) {
				t.Errorf("%s: no item %d", id, it)
			}
		}
		if !p.Romance.Romanceable {
			t.Errorf("%s: every companion can be romanced", id)
		}
		if strings.TrimSpace(p.Specialty) == `` || len(p.Hollow.Waiting) == 0 || len(p.Hollow.WaryOf) == 0 ||
			strings.TrimSpace(p.Hollow.Returns) == `` {
			t.Errorf("%s: specialty, and the Hollow's waiting, wary_of and returns, are all authored", id)
		}
	}
}

// The whole round trip on a visitor's own key: what they said goes to
// THEIR browser under the interview schema, carrying who she is, what wins
// her over and what she can see of them; her answer comes back as her
// words and her opinion of them.
func TestInterviewTravelsOnTheVisitorsOwnKey(t *testing.T) {
	hw := newHollowWorld(t)
	m, p, w := hw.m, hw.p, hw.w
	m.cfg.Model, m.cfg.FastModel, m.cfg.DeepModel = `m`, `m`, `m`
	m.cfg.RelayTimeoutSeconds = 5
	f := newFakeRelay()
	m.relaySend = f.send
	m.relays.ready(1, `player-model`, false)
	m.bonds.Users[1] = &bondRecord{Consented: true}
	m.syncConsent()

	util.LockMud()
	m.hearInHollow(hw.owner, 1, `Mara, I walked the Stillwater road last month.`)
	if len(w.pending) != 1 {
		util.UnlockMud()
		t.Fatalf("queued: %+v", w.pending)
	}
	m.tendHollow(1)
	inFlight := w.inFlight
	util.UnlockMud()
	if !inFlight {
		t.Fatal("the moment starts a call")
	}

	reply := `{"choices":[{"finish_reason":"stop","message":{"content":` + jsonString(
		`{"intent":"x","speech":[{"kind":"say","text":"Did you now. Tell me about the markers."}],"mood":"curious",`+
			`"memory":{"text":"They say they walked the Stillwater road.","importance":5,"emotion":"curiosity"},"facts":[],`+
			`"opinion":{"trust":1,"respect":2,"affection":0,"reason":"knows the road"},"verdict":"undecided"}`) +
		`}}],"usage":{"total_tokens":42}}`
	r := f.next(t)
	body := string(r.Body)
	for _, want := range []string{`companion_interview`, `player-model`, `Things that would win you over`,
		p.Hollow.Appreciates[0], `WHO IS IN FRONT OF YOU`, `Corvin`, `Stillwater road last month`} {
		if !strings.Contains(body, want) {
			t.Errorf("the request carries %q", want)
		}
	}
	if !m.relayCalls.deliver(1, relayResponse{Id: r.Id, Status: 200, Body: reply}) {
		t.Fatal("the visitor's browser answers")
	}
	m.decisions.Wait()

	util.LockMud()
	defer util.UnlockMud()
	mind := m.getMind(1, p)
	if mind.Opinion.Respect <= p.OpinionBaseline().Respect || mind.Courtship.Gains != 1 {
		t.Fatalf("her opinion of them rose, and it counts: %+v gains=%d", mind.Opinion, mind.Courtship.Gains)
	}
	said := false
	for _, l := range mind.RecentLines {
		if l.Speaker == p.Name && strings.Contains(l.Text, `markers`) {
			said = true
		}
	}
	if !said || m.holderOf(p.Id) != 0 || w.inFlight {
		t.Fatalf("she answered in her own words and stays where she is: said=%v holder=%d", said, m.holderOf(p.Id))
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// The board's words in the shipped room are the words the code falls back
// on, and say what they must.
func TestTheShippedSignSaysWhatItMust(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(here), `..`, `..`, `_datafiles`, `world`, `dogmud`, `rooms`, `pothole_coulee`, `6880.yaml`))
	if err != nil {
		t.Fatal(err)
	}
	var room struct {
		Nouns map[string]string `yaml:"nouns"`
	}
	if err := yaml.Unmarshal(b, &room); err != nil {
		t.Fatal(err)
	}
	squash := func(s string) string { return strings.Join(strings.Fields(s), ` `) }
	if squash(room.Nouns[hollowSignNoun]) != squash(hollowSignText) {
		t.Fatalf("the shipped board and hollowSignText have drifted apart:\n%s\n---\n%s", room.Nouns[hollowSignNoun], hollowSignText)
	}
	if buildConfig(nil).HollowSignRoom != 6880 {
		t.Fatal("the board hangs in the Waystone Mouth")
	}
}

// Travelling with a companion is agreement: the "i agree" question is gone,
// and a holder from before the board, or one an admin granted, is not left
// in a half state where she is fielded but treated as unconsented. Their
// own later companion-ai off is never overridden.
func TestHavingACompanionIsConsent(t *testing.T) {
	hw := newHollowWorld(t)
	m, p := hw.m, hw.p
	m.roster.Profiles[p.Id] = &rosterEntry{Owner: 1}
	m.roster.Profiles[`other`] = &rosterEntry{Owner: 2}
	m.bonds.Users[2] = &bondRecord{Refused: true}
	m.syncConsent()
	if m.consented(1) {
		t.Fatal("fixture: not yet")
	}
	m.consentHolders()
	if !m.consented(1) {
		t.Fatal("a holder has agreed by having her")
	}
	if m.consented(2) || !m.bonds.Users[2].Refused {
		t.Fatal("companion-ai off stands")
	}
	if !m.consent.allows(1) {
		t.Fatal("and the door to the model knows it")
	}
}
