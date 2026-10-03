package aicompanion

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Meeting a companion. There is one of each companion on the server, and a
// free one waits in the Waystone Hollow above Pothole Coulee (hollow.go)
// until someone wins them over. Nobody is handed one: a new character is
// told, once, where the Hollow is (onCharacterCreated, hintHollow). A
// companion leaves when their owner sends them away (companion-part), when
// the relationship has gone too far wrong (abandons), or when their owner
// has been away too long (roster.go), and goes back to the Hollow each
// time.

// bondRecord is what the module remembers about one character's bond.
type bondRecord struct {
	Profile  string `yaml:"profile,omitempty"`
	Met      bool   `yaml:"met,omitempty"`
	Declined bool   `yaml:"declined,omitempty"`
	Unix     int64  `yaml:"t,omitempty"`
	// Consented is the player having agreed that talking to a companion
	// sends what they say to the model: by speaking to one in the Hollow
	// after reading its sign, by travelling with one at all (bondTo, and
	// every holder when the roster loads: consentByCompanionship), or with
	// companion-ai on. There is no typed "i agree" any more. Until then
	// nothing they say leaves the server, and Refused (companion-ai off,
	// their own later word) keeps it that way whatever else happens.
	Consented bool `yaml:"consented,omitempty"`
	Refused   bool `yaml:"refused,omitempty"`
	// StrangersOff and StrangersOn are the owner's own word on whether
	// passers-by may prompt a model call for their companion
	// (companion-ai strangers off, or on); at most one is set. With
	// strangers off she still hears them and answers with set lines.
	// Neither set, as in every record saved before the choice existed, is
	// the default for whoever pays (strangersOffOn): off on the owner's
	// own key, which is their money, and on for the server's key.
	StrangersOff bool `yaml:"strangers_off,omitempty"`
	StrangersOn  bool `yaml:"strangers_on,omitempty"`
	// Notice is a line kept for this player's next login: why the
	// companion they still had on their record has gone (reclaimFrom).
	Notice string `yaml:"notice,omitempty"`
	// HollowHinted is the new character having been told where the
	// Waystone Hollow is.
	HollowHinted bool `yaml:"hollow_hinted,omitempty"`
	// SignRead is the player having been shown the board at the mouth of
	// the Hollow, which says plainly what speaking to the travellers there
	// sends and where. Speaking to one after it is their agreement.
	SignRead bool `yaml:"sign_read,omitempty"`
}

type bondState struct {
	Users map[int]*bondRecord `yaml:"users"`
}

const bondStateId = `bond-state`

func (m *AICompanionModule) loadBonds() {
	m.bonds = bondState{Users: map[int]*bondRecord{}}
	if err := m.plug.ReadIntoStruct(bondStateId, &m.bonds); err != nil && !errors.Is(err, util.ErrStateAbsent) {
		mudlog.Error(`aicompanion`, `action`, `loadBonds`, `error`, err)
	}
	if m.bonds.Users == nil {
		m.bonds.Users = map[int]*bondRecord{}
	}
	m.syncConsent()
}

// saveBonds writes the bond records, and brings the consent door's copy up
// to date first: every change of mind about consent is saved, so this is
// the one place that sees them all.
func (m *AICompanionModule) saveBonds() {
	m.syncConsent()
	if m.plug == nil {
		return // built without storage, as the tests build her
	}
	if err := m.plug.WriteStruct(bondStateId, &m.bonds); err != nil {
		mudlog.Error(`aicompanion`, `action`, `saveBonds`, `error`, err)
	}
}

// consentByCompanionship records consent for a player who travels with a
// companion: they won her by talking to her in the Hollow after its board
// told them what that means, so having one IS their agreement. It also
// covers a companion an admin granted, and a holder from before the board.
// An explicit companion-ai off (Refused) is their own later word and is
// never overridden. Reports whether it changed anything.
func (m *AICompanionModule) consentByCompanionship(userId int) bool {
	if userId <= 0 {
		return false
	}
	rec := m.bondRecordFor(userId)
	if rec.Consented || rec.Refused {
		return false
	}
	rec.Consented = true
	mudlog.Info(`aicompanion`, `action`, `consent`, `owner`, userId, `how`, `travels with a companion`)
	return true
}

// markBond records that a character has met (or parted from) a companion.
func (m *AICompanionModule) markBond(userId int, profileId string, declined bool) {
	if m.bonds.Users == nil {
		m.bonds.Users = map[int]*bondRecord{}
	}
	rec, ok := m.bonds.Users[userId]
	if !ok {
		rec = &bondRecord{}
		m.bonds.Users[userId] = rec
	}
	rec.Profile, rec.Met, rec.Unix = profileId, true, time.Now().Unix()
	if declined {
		rec.Declined = true
	}
	m.saveBonds()
}

// onCharacterCreated marks a brand-new character to be told, once, where
// the companions wait (hintHollow).
func (m *AICompanionModule) onCharacterCreated(e events.Event) events.ListenerReturn {
	evt, ok := e.(events.CharacterCreated)
	if !ok || !m.cfg.Enabled {
		return events.Continue
	}
	if m.newcomers == nil {
		m.newcomers = map[int]bool{}
	}
	m.newcomers[evt.UserId] = true
	return events.Continue
}

// hintHollow tells a new character where the Waystone Hollow is, the first
// time they stand in the Hollow's own zone (the village they start near),
// once. It runs in sync.
func (m *AICompanionModule) hintHollow(u *users.UserRecord) {
	if !m.newcomers[u.UserId] || u.Character == nil {
		return
	}
	zone := m.hollowZone()
	room := rooms.LoadRoom(u.Character.RoomId)
	if zone == `` || room == nil || !strings.EqualFold(room.Zone, zone) || u.Character.IsInCombat() {
		return
	}
	delete(m.newcomers, u.UserId)
	if rec := m.bonds.Users[u.UserId]; rec != nil && rec.HollowHinted {
		return
	}
	if m.bonds.Users == nil {
		m.bonds.Users = map[int]*bondRecord{}
	}
	rec := m.bonds.Users[u.UserId]
	if rec == nil {
		rec = &bondRecord{}
		m.bonds.Users[u.UserId] = rec
	}
	rec.HollowHinted = true
	m.saveBonds()
	m.tellOwner(u.UserId, `(Word in the village is that a few travellers between roads wait in the Waystone Hollow, a cave west of Scrub Draw, for someone worth going with. Each of them chooses for themselves. See "help companion-hollow".)`)
}

// leave ends the bond at the companion's own decision: the owner clearly
// asked it to go and confirmed it, or it has come to dislike and distrust
// them so much that it will not stay (abandons). It says goodbye in its own
// words first (already spoken, or not at all), walks off, and goes back to
// wait in the Waystone Hollow, taking what it has learned with it.
func (m *AICompanionModule) leave(c *controller, owner *users.UserRecord, why string) {
	if owner == nil || owner.Character == nil {
		return
	}
	if strings.TrimSpace(why) == `` {
		why = partSentAway // her owner asked her to go, twice
	}
	p := c.profile
	profileId := p.Id
	// What she had with them comes down to one sentence (partWith, by way
	// of unclaim); the rest of her mind of them is wiped.
	if err := m.sendBack(owner, `turns and walks away without looking back, toward the Waystone Hollow.`, true, why); err != nil {
		mudlog.Error(`aicompanion`, `action`, `leave`, `owner`, owner.UserId, `error`, err)
		return
	}
	m.markBond(owner.UserId, profileId, true)
	owner.SendText(messaging.CategorySystem, fmt.Sprintf(
		`%s has gone back to the Waystone Hollow. %s will not come back on %s own; you would have to go there and win %s over again.`,
		p.Name, capitalize(subjectPronoun(p)), possessive(p), objectPronoun(p)))
}

// capitalize upper-cases the first letter of a word.
func capitalize(s string) string {
	if s == `` {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// leaveRequestAllowed is the rule for a companion ASKING to part ways: its
// owner said something to it in this very moment (so it may have been told
// to go), or the relationship has collapsed entirely. Asking is never the
// same as going: the bond only ends when the owner confirms with
// `companion-part`, or when the companion has nothing left to stay for.
func leaveRequestAllowed(o Opinion, stims []stimulus) bool {
	if ownerAskedNow(stims) {
		return true
	}
	return o.Trust <= -70 && o.Affection <= -70
}

// collapsed reports a relationship with nothing left in it, the one case
// where a companion leaves without being told twice.
func collapsed(o Opinion) bool {
	return o.Trust <= -85 && o.Affection <= -85
}

// requestLeave is what the model's "leave" actually does: the companion has
// said its piece and now holds back, waiting for its owner to mean it. The
// owner confirms with `companion-part` (or takes it back by saying so).
func (m *AICompanionModule) requestLeave(c *controller, owner *users.UserRecord, why string) {
	c.leaveAskedAt = time.Now().Unix()
	c.leaveWhy = why
	c.mind.Autonomy = autonomyClose
	if m.mayRemember(c) {
		c.mind.addLine(Line{Kind: `event`, Text: `You told ` + owner.Character.Name + ` you would go, and hung back to see if they meant it.`},
			m.cfg.WorkingMemoryLines)
	}
	c.dirty = true
	owner.SendText(messaging.CategorySystem, fmt.Sprintf(
		`(If you truly want %s to leave for good, type <ansi fg="command">companion-part</ansi> within %d minutes. Otherwise just carry on; they will stay.)`,
		c.profile.Name, m.cfg.LeaveConfirmSeconds/60))
}

// confirmPart is the owner's side of parting ways, from the
// `companion-part` command: it ends the bond when the companion has asked
// to go, or when the owner asks twice in a row.
func (m *AICompanionModule) confirmPart(c *controller, owner *users.UserRecord) string {
	now := time.Now().Unix()
	asked := c.leaveAskedAt > 0 && now-c.leaveAskedAt <= int64(m.cfg.LeaveConfirmSeconds)
	if !asked && (c.partAskedAt == 0 || now-c.partAskedAt > 60) {
		c.partAskedAt = now
		return fmt.Sprintf(`%s looks at you and waits. Type companion-part again within a minute to part ways; they go back to the Waystone Hollow, and only come with you again if you win them over.`, c.profile.Name)
	}
	m.leave(c, owner, c.leaveWhy)
	return ``
}

// Consent. A player agrees to the model by speaking to a traveller in the
// Waystone Hollow after the board at its mouth has told them, plainly, what
// that sends and where (hollow.go, showSign and hollowMayTalk), or with
// "companion-ai on"; "companion-ai off" takes it back at any time. Nothing
// of theirs is written into a mind or sent before then.

// consented reports whether this owner has agreed to the model being used
// for their companion. Without agreement she is an ordinary companion.
func (m *AICompanionModule) consented(ownerUserId int) bool {
	if !m.cfg.RequireConsent {
		return true
	}
	rec := m.bonds.Users[ownerUserId]
	return rec != nil && rec.Consented
}

// mayRemember reports whether a deed with a person's name in it (a gift,
// an attack, healing, her owner calling her, a fight, a fall, a party) may
// be written into her mind. Her mind is what gets sent, so, as with speech,
// nothing naming anyone is written down before her owner has agreed (or
// while they have turned it off): writing first and gating the send later
// would send it the moment they agreed. She still reacts to it: the
// stimulus is queued and dispatch answers with her set lines, and the
// rules that are her owner's own relationship (a gift's warmth, an attack's
// cost) still apply. Events that name nobody ("You reached the mill") are
// written as before.
func (m *AICompanionModule) mayRemember(c *controller) bool {
	return m.consented(c.ownerUserId)
}

// consentLedger is the model door's own copy of who has agreed. The bond
// records live under the mud lock, and requests leave from goroutines that
// do not hold it, so the door in send reads this instead. It is rebuilt
// from the bond records whenever they are loaded or saved. Its zero value
// agrees to nothing, so a module that never filled it sends nothing.
type consentLedger struct {
	mu      sync.Mutex
	open    bool         // this server does not ask for consent at all
	agreed  map[int]bool // owners who have said yes
	lastLog time.Time    // last refusal written to the log
}

// allows reports whether a request carrying this owner's data may leave.
// An unknown owner never may, whatever the server's setting.
func (l *consentLedger) allows(ownerUserId int) bool {
	if l == nil || ownerUserId <= 0 {
		return false
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.open || l.agreed[ownerUserId]
}

// noteRefusal logs a request the door turned away, at most once a minute.
// Every caller checks consent before building a request, so a refusal here
// means one of them forgot: worth a line, not a flood.
func (l *consentLedger) noteRefusal(ownerUserId int, path string) {
	if l != nil {
		l.mu.Lock()
		now := time.Now()
		if now.Sub(l.lastLog) < time.Minute {
			l.mu.Unlock()
			return
		}
		l.lastLog = now
		l.mu.Unlock()
	}
	mudlog.Error(`aicompanion`, `action`, `consentDoor`, `owner`, ownerUserId, `path`, path,
		`error`, `a request for an owner who has not agreed reached the door; a caller is missing its consent check`)
}

// syncConsent rebuilds the door's copy from the bond records. Runs under
// the mud lock.
func (m *AICompanionModule) syncConsent() {
	agreed := make(map[int]bool, len(m.bonds.Users))
	for id, rec := range m.bonds.Users {
		if rec != nil && rec.Consented {
			agreed[id] = true
		}
	}
	m.consent.mu.Lock()
	m.consent.open = !m.cfg.RequireConsent
	m.consent.agreed = agreed
	m.consent.mu.Unlock()
}
