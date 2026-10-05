package aicompanion

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// The roster. Every companion is one person on the server: while Mara
// travels with one player nobody else can have her, and when she is free
// she waits in her own corner of the Waystone Hollow (hollow.go) for
// someone to win her over.
//
// A companion's growth is hers, not her owner's. What she has learned
// (skills, trained stats, spells, mutations) is kept here and goes with
// her from owner to owner. Her gear does not: what she carries when she
// leaves is handed to the person she leaves, except the few things she
// started with, which she takes back with her. That keeps every item out
// of this file, so the bauble sweep has nothing new to walk.
//
// The roster is the one source of truth for who holds whom. A character
// whose record still carries a companion the roster gives to nobody else
// (an owner away too long, an admin release, the move to one of each)
// finds her gone at their next login (reclaimFrom).

// keptProgress is what a companion has learned, carried between owners.
// It mirrors the progression half of characters.CompanionInfo.
type keptProgress struct {
	SchemaVersion    int            `yaml:"schema_version,omitempty"`
	StatTraining     map[string]int `yaml:"stat_training,omitempty"`
	Skills           map[string]int `yaml:"skills,omitempty"`
	SkillUseCount    map[string]int `yaml:"skill_use_count,omitempty"`
	Mutations        map[string]int `yaml:"mutations,omitempty"`
	SpellBook        map[string]int `yaml:"spellbook,omitempty"`
	MutationProgress float64        `yaml:"mutation_progress,omitempty"`
}

// rosterEntry is one companion's place in the world.
type rosterEntry struct {
	// Owner is the account she travels with, or 0 while she waits in the
	// Hollow.
	Owner int `yaml:"owner,omitempty"`
	// Since is when she took up with Owner, or came back to the Hollow.
	Since int64 `yaml:"since,omitempty"`
	// LastSeen is when Owner was last online with her.
	LastSeen int64 `yaml:"last_seen,omitempty"`
	// Kept is what she has learned, whoever she is with.
	Kept keptProgress `yaml:"kept,omitempty"`
	// OwnerName is Owner's character name when she took up with them, for
	// the sentence she keeps of them if they part while they are offline.
	OwnerName string `yaml:"owner_name,omitempty"`
	// Partings is the one sentence she keeps of each person she travelled
	// with and left, by account (parting.go). Read in the Hollow when they
	// come to talk to her; deleted when they win her back.
	Partings map[int]*Parting `yaml:"partings,omitempty"`
}

type rosterState struct {
	Version  int                     `yaml:"version"`
	Profiles map[string]*rosterEntry `yaml:"profiles"`
}

const (
	rosterStateId = `roster`
	rosterVersion = 1
)

// loadRoster reads the roster, or starts one. A missing roster is the move
// to one companion of each kind: every profile starts free, so every
// companion fielded before it is sent back to the Hollow at their owner's
// next login (reclaimFrom), without the progress they made with them.
func (m *AICompanionModule) loadRoster() {
	m.roster = rosterState{Version: rosterVersion, Profiles: map[string]*rosterEntry{}}
	if m.plug != nil {
		if err := m.plug.ReadIntoStruct(rosterStateId, &m.roster); err != nil && !errors.Is(err, util.ErrStateAbsent) {
			// Quarantined by ReadIntoStruct. Starting over frees every
			// companion, which is the safe failure: nobody keeps one the
			// roster cannot vouch for.
			mudlog.Error(`aicompanion`, `action`, `loadRoster`, `error`, err)
			m.roster = rosterState{Version: rosterVersion, Profiles: map[string]*rosterEntry{}}
		}
	}
	if m.roster.Profiles == nil {
		m.roster.Profiles = map[string]*rosterEntry{}
	}
	m.roster.Version = rosterVersion
	for id := range m.profiles {
		m.rosterFor(id)
	}
	m.consentHolders()
}

// consentHolders records consent for everyone the roster says travels with
// a companion (consentByCompanionship): it settles holders from before the
// board at the Hollow, and any an admin granted.
func (m *AICompanionModule) consentHolders() {
	changed := false
	for _, e := range m.roster.Profiles {
		if e != nil && e.Owner > 0 && m.consentByCompanionship(e.Owner) {
			changed = true
		}
	}
	if changed {
		m.saveBonds()
	}
}

// saveRoster writes the roster through the plugin store (durable,
// autosave-queued).
func (m *AICompanionModule) saveRoster() {
	if m.plug == nil {
		return // built without storage, as the tests build her
	}
	if err := m.plug.WriteStruct(rosterStateId, &m.roster); err != nil {
		mudlog.Error(`aicompanion`, `action`, `saveRoster`, `error`, err)
	}
}

// rosterFor is a profile's entry, made when missing.
func (m *AICompanionModule) rosterFor(profileId string) *rosterEntry {
	if m.roster.Profiles == nil {
		m.roster.Profiles = map[string]*rosterEntry{}
	}
	e := m.roster.Profiles[profileId]
	if e == nil {
		e = &rosterEntry{Since: time.Now().Unix()}
		m.roster.Profiles[profileId] = e
	}
	return e
}

// holdsHer reports whether this character is the one the roster gives her
// to: its account holds her and, when the roster knows which character of
// that account (OwnerName), it is this one. A claim made before OwnerName
// was kept is adopted by the first character of the account to carry her.
func (m *AICompanionModule) holdsHer(p *Profile, u *users.UserRecord) bool {
	if p == nil || u == nil || u.Character == nil {
		return false
	}
	e := m.roster.Profiles[p.Id]
	if e == nil || e.Owner != u.UserId {
		return false
	}
	if e.OwnerName == `` {
		e.OwnerName = u.Character.Name
		m.saveRoster()
		return true
	}
	return strings.EqualFold(e.OwnerName, u.Character.Name)
}

// holderOf is the account a companion travels with, or 0.
func (m *AICompanionModule) holderOf(profileId string) int {
	if e := m.roster.Profiles[profileId]; e != nil {
		return e.Owner
	}
	return 0
}

// heldBy is the profile an account holds, or nil.
func (m *AICompanionModule) heldBy(userId int) *Profile {
	if userId <= 0 {
		return nil
	}
	for id, e := range m.roster.Profiles {
		if e != nil && e.Owner == userId {
			if p := m.profiles[id]; p != nil {
				return p
			}
		}
	}
	return nil
}

// claim gives a companion to an account. It is refused while someone else
// has her, and an account holds one companion at a time.
func (m *AICompanionModule) claim(p *Profile, userId int) error {
	e := m.rosterFor(p.Id)
	if e.Owner != 0 && e.Owner != userId {
		return fmt.Errorf(`%s is already on the road with someone else`, p.Name)
	}
	// One account, several characters (alts): she travels with one of
	// them, not with each.
	if e.Owner == userId && e.OwnerName != `` {
		if u := users.GetByUserId(userId); u != nil && u.Character != nil && !strings.EqualFold(e.OwnerName, u.Character.Name) {
			return fmt.Errorf(`%s already travels with %s, another character on that account`, p.Name, e.OwnerName)
		}
	}
	if other := m.heldBy(userId); other != nil && other.Id != p.Id {
		return fmt.Errorf(`that account already travels with %s`, other.Name)
	}
	now := time.Now().Unix()
	e.Owner, e.Since, e.LastSeen = userId, now, now
	e.OwnerName = ``
	if u := users.GetByUserId(userId); u != nil && u.Character != nil {
		e.OwnerName = u.Character.Name
	}
	m.saveRoster()
	return nil
}

// unclaim frees a companion to wait in the Hollow again. What she had with
// the person she leaves comes down to one sentence (partWith); why is how
// it ended, as the server saw it.
func (m *AICompanionModule) unclaim(p *Profile, why string) {
	e := m.rosterFor(p.Id)
	if e.Owner != 0 {
		if m.returning == nil {
			m.returning = map[string]bool{}
		}
		m.returning[p.Id] = true // she walks back in, rather than appearing
		m.partWith(p, e, why)
	}
	e.Owner, e.Since, e.LastSeen, e.OwnerName = 0, time.Now().Unix(), 0, ``
	m.saveRoster()
}

// keepProgress records what a companion has learned.
func (m *AICompanionModule) keepProgress(p *Profile, k keptProgress) {
	if k.empty() {
		return
	}
	m.rosterFor(p.Id).Kept = k
}

// seeOwner notes that a companion's owner is online with her. Saved with
// the roster at the next save.
func (m *AICompanionModule) seeOwner(p *Profile, now int64) {
	if e := m.roster.Profiles[p.Id]; e != nil {
		e.LastSeen = now
	}
}

// releaseLapsed sends home every companion whose owner has not been seen
// for ReleaseAfterDays. Her progress was kept the last time they were
// together; the owner learns of it at their next login (reclaimFrom).
func (m *AICompanionModule) releaseLapsed(now time.Time) {
	limit := int64(m.cfg.ReleaseAfterDays) * 86400
	ids := make([]string, 0, len(m.roster.Profiles))
	for id := range m.roster.Profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		e := m.roster.Profiles[id]
		p := m.profiles[id]
		if e == nil || p == nil || e.Owner == 0 {
			continue
		}
		if u := users.GetByUserId(e.Owner); u != nil && u.Character != nil {
			e.LastSeen = now.Unix() // online, whatever the file said
			continue
		}
		last := e.LastSeen
		if last == 0 {
			last = e.Since
		}
		if now.Unix()-last < limit {
			continue
		}
		owner := e.Owner
		m.unclaim(p, partLapsed)
		m.noteForLogin(owner, fmt.Sprintf(
			`%s waited for you for a long time, and in the end went back to the Waystone Hollow above Pothole Coulee. If you want %s company again, you will have to go and win it.`,
			p.Name, possessive(p)))
		mudlog.Info(`aicompanion`, `action`, `release`, `profile`, p.Id, `owner`, owner, `reason`, `owner away too long`)
	}
}

// noteForLogin leaves a player a line to read the next time their
// companion is taken from them at login.
func (m *AICompanionModule) noteForLogin(userId int, text string) {
	if m.bonds.Users == nil {
		m.bonds.Users = map[int]*bondRecord{}
	}
	rec := m.bonds.Users[userId]
	if rec == nil {
		rec = &bondRecord{}
		m.bonds.Users[userId] = rec
	}
	rec.Notice = text
	m.saveBonds()
}

// reclaimFrom takes back a companion the roster does not give this player:
// her owner was away too long, an admin released her, or she was fielded
// before there was one of each. She is not theirs to keep any more, and
// the progress on their record is not hers to carry (the roster already
// holds what she kept, or nothing). Her gear is handed over to them.
func (m *AICompanionModule) reclaimFrom(u *users.UserRecord, p *Profile) {
	notice := fmt.Sprintf(
		`%s is not with you any more. There is only one of %s in the world, and %s has gone back to wait in the Waystone Hollow above Pothole Coulee. If you want %s company, you will have to go and win it.`,
		p.Name, objectPronoun(p), subjectPronoun(p), possessive(p))
	if rec := m.bonds.Users[u.UserId]; rec != nil && rec.Notice != `` {
		notice, rec.Notice = rec.Notice, ``
		m.saveBonds()
	}
	// Told first, so the line about her things reads as what follows.
	m.tellOwner(u.UserId, `(`+notice+`)`)
	if err := m.sendBack(u, ``, false, ``); err != nil {
		mudlog.Error(`aicompanion`, `action`, `reclaim`, `owner`, u.UserId, `error`, err)
		return
	}
	mudlog.Info(`aicompanion`, `action`, `reclaim`, `owner`, u.UserId, `profile`, p.Id)
}

// sendBack ends a bond and sends the companion back to the Hollow: her
// progress is kept (when kept is true, which is every time she was this
// player's to lose), her gear is handed to them, her claim is freed, and
// the waiting companion reappears in her room on the next round
// (tendHollow). departure is the line the room sees as she goes.
func (m *AICompanionModule) sendBack(owner *users.UserRecord, departure string, kept bool, why string) error {
	if owner == nil || owner.Character == nil {
		return fmt.Errorf(`no owner`)
	}
	comp, p := m.bondedCompanionOf(owner)
	if comp == nil {
		return fmt.Errorf(`%s has no bonded companion`, owner.Character.Name)
	}

	// What she knew and what she had, read before the mob is destroyed.
	var gear []items.Item
	gold := 0
	progress := progressOfInfo(comp)
	if mob := mobs.GetInstance(comp.InstanceId); comp.InstanceId > 0 && mob != nil {
		progress = progressOfMob(mob)
		gear = append(gear, mob.Character.Items...)
		gear = append(gear, mob.Character.Equipment.GetAllItems()...)
		gold = mob.Character.Gold
		mob.Character.Items = nil
		mob.Character.Equipment = characters.Worn{}
		mob.Character.Gold = 0
	} else {
		gear = append(gear, comp.Items...)
		gear = append(gear, comp.Equipment.GetAllItems()...)
		gold = comp.Gold
	}
	roomId := owner.Character.RoomId

	if _, err := m.unbond(owner, departure); err != nil {
		return err
	}
	m.handOver(owner, roomId, p, gear, gold)

	if kept {
		m.keepProgress(p, progress)
	}
	// Freed only when she was this character's to lose: a stale record on
	// an alt (reclaimFrom, kept false) must not free her from the one she
	// really travels with.
	if kept && m.holderOf(p.Id) == owner.UserId {
		m.unclaim(p, why)
	} else {
		m.saveRoster()
	}

	// Winning her back, if they ever try, starts again from nothing.
	if mind := m.minds[mindIdentifier(owner.UserId, p.MobId)]; mind != nil {
		mind.Courtship = Courtship{}
		if m.plug == nil {
			// built without storage, as the tests build her
		} else if err := saveMind(m.plug, mind); err != nil {
			mudlog.Error(`aicompanion`, `action`, `saveMind`, `owner`, owner.UserId, `error`, err)
		}
	}
	return nil
}

// handOver gives a departing companion's gear and coin to the person she
// leaves: into their pack where it fits, on the floor where it does not.
// The things she started with (the profile's starting kit, one of each
// counted) go with her, so leaving and coming back can never make more of
// them.
func (m *AICompanionModule) handOver(owner *users.UserRecord, roomId int, p *Profile, gear []items.Item, gold int) {
	hers := map[int]int{}
	for _, id := range p.StartingItems {
		hers[id]++
	}
	room := rooms.LoadRoom(roomId)
	given, dropped := 0, 0
	for _, it := range gear {
		if it.ItemId <= 0 {
			continue
		}
		if hers[it.ItemId] > 0 {
			hers[it.ItemId]--
			continue
		}
		if owner.Character.StoreItem(it) {
			given++
			events.AddToQueue(events.ItemOwnership{UserId: owner.UserId, Item: it, Gained: true})
			continue
		}
		if room != nil {
			room.AddItem(it, false)
			dropped++
		}
	}
	if gold > 0 {
		owner.Character.Gold += gold
		events.AddToQueue(events.CharacterVitalsChanged{UserId: owner.UserId})
	}
	if given > 0 || dropped > 0 || gold > 0 {
		var parts []string
		if given > 0 {
			parts = append(parts, `the things you gave or bought for `+objectPronoun(p)+` are in your pack`)
		}
		if dropped > 0 {
			parts = append(parts, `what would not fit is on the ground`)
		}
		if gold > 0 {
			parts = append(parts, fmt.Sprintf(`%d gold is back in your purse`, gold))
		}
		m.tellOwner(owner.UserId, `(`+p.Name+` left your things with you: `+strings.Join(parts, `, `)+`.)`)
	}
	if err := users.SaveUser(owner); err != nil {
		mudlog.Error(`aicompanion`, `action`, `handOver`, `owner`, owner.UserId, `error`, err)
	}
}

// abandons reports a relationship gone so far wrong that she leaves
// without being told twice: trust and affection both at or below
// AbandonBelow.
func (m *AICompanionModule) abandons(o Opinion) bool {
	return o.Trust <= m.cfg.AbandonBelow && o.Affection <= m.cfg.AbandonBelow
}

// progressOfMob reads what a live companion has learned. It mirrors the
// engine's snapshotCompanionProgression (internal/hooks) and must be kept
// in step with it.
func progressOfMob(mob *mobs.Mob) keptProgress {
	ch := &mob.Character
	return keptProgress{
		SchemaVersion: mobs.InstanceSchemaVersion,
		StatTraining: map[string]int{
			`strength`:   ch.Stats.Strength.Training,
			`dexterity`:  ch.Stats.Dexterity.Training,
			`perception`: ch.Stats.Perception.Training,
			`vitality`:   ch.Stats.Vitality.Training,
			`willpower`:  ch.Stats.Willpower.Training,
			`charisma`:   ch.Stats.Charisma.Training,
		},
		Skills:           copyInts(ch.Skills),
		SkillUseCount:    copyInts(ch.SkillUseCount),
		Mutations:        copyInts(ch.Mutations),
		SpellBook:        copyInts(ch.SpellBook),
		MutationProgress: ch.MutationProgress,
	}
}

// progressOfInfo reads what a companion's record says it has learned.
func progressOfInfo(ci *characters.CompanionInfo) keptProgress {
	return keptProgress{
		SchemaVersion:    ci.SchemaVersion,
		StatTraining:     copyInts(ci.StatTraining),
		Skills:           copyInts(ci.Skills),
		SkillUseCount:    copyInts(ci.SkillUseCount),
		Mutations:        copyInts(ci.Mutations),
		SpellBook:        copyInts(ci.SpellBook),
		MutationProgress: ci.MutationProgress,
	}
}

// empty reports a companion who has learned nothing yet.
func (k keptProgress) empty() bool {
	return len(k.StatTraining) == 0 && len(k.Skills) == 0 && len(k.SkillUseCount) == 0 &&
		len(k.Mutations) == 0 && len(k.SpellBook) == 0 && k.MutationProgress == 0
}

// toInfo copies what she has learned into a new owner's record of her.
func (k keptProgress) toInfo(ci *characters.CompanionInfo) {
	if k.empty() {
		return
	}
	ci.SchemaVersion = k.SchemaVersion
	ci.StatTraining = copyInts(k.StatTraining)
	ci.Skills = copyInts(k.Skills)
	ci.SkillUseCount = copyInts(k.SkillUseCount)
	ci.Mutations = copyInts(k.Mutations)
	ci.SpellBook = copyInts(k.SpellBook)
	ci.MutationProgress = k.MutationProgress
}

// applyProgress puts what she has learned onto a fresh mob. It mirrors the
// progression half of the engine's applyCompanionState (internal/hooks);
// a roster entry is always saved at the current schema, so no legacy
// conversion is needed.
func applyProgress(mob *mobs.Mob, k keptProgress) {
	if k.empty() {
		return
	}
	ch := &mob.Character
	if k.StatTraining != nil {
		ch.Stats.Strength.Training = k.StatTraining[`strength`]
		ch.Stats.Dexterity.Training = k.StatTraining[`dexterity`]
		ch.Stats.Perception.Training = k.StatTraining[`perception`]
		ch.Stats.Vitality.Training = k.StatTraining[`vitality`]
		ch.Stats.Willpower.Training = k.StatTraining[`willpower`]
		ch.Stats.Charisma.Training = k.StatTraining[`charisma`]
	}
	if k.Skills != nil {
		// What she learned on the road, over the trade she arrived with
		// (the template's starting ranks): never less than either.
		start := ch.Skills
		ch.Skills = copyInts(k.Skills)
		for tag, rank := range start {
			if rank > ch.Skills[tag] {
				ch.Skills[tag] = rank
			}
		}
	}
	if k.SkillUseCount != nil {
		ch.SkillUseCount = copyInts(k.SkillUseCount)
	}
	if k.Mutations != nil {
		ch.Mutations = copyInts(k.Mutations)
	}
	if k.SpellBook != nil {
		ch.SpellBook = copyInts(k.SpellBook)
	}
	ch.MutationProgress = k.MutationProgress
	ch.Validate()
}

// learnStartingSpells puts the profile's spells into her spellbook.
func learnStartingSpells(mob *mobs.Mob, p *Profile) {
	for _, id := range p.StartingSpells {
		if !mob.Character.HasSpell(id) {
			mob.Character.LearnSpell(id)
		}
	}
}

func copyInts(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// The profile's pronouns, read from its "she/her" style field, for the
// few authored lines that speak of a companion in the third person.
func pronounParts(p *Profile) (subject string, object string, possessive string) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(p.Pronouns)), `/`)
	switch strings.TrimSpace(parts[0]) {
	case `she`:
		return `she`, `her`, `her`
	case `he`:
		return `he`, `him`, `his`
	}
	return `they`, `them`, `their`
}

func subjectPronoun(p *Profile) string { s, _, _ := pronounParts(p); return s }
func objectPronoun(p *Profile) string  { _, o, _ := pronounParts(p); return o }
func possessive(p *Profile) string     { _, _, w := pronounParts(p); return w }

// rosterLines describe the roster for the admin `aicompanion claims`.
func (m *AICompanionModule) rosterLines(now time.Time) []string {
	ids := make([]string, 0, len(m.profiles))
	for id := range m.profiles {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out []string
	for _, id := range ids {
		p := m.profiles[id]
		e := m.rosterFor(id)
		learned := `nothing learned yet`
		if !e.Kept.empty() {
			n := 0
			for _, lvl := range e.Kept.Skills {
				if lvl > 0 {
					n++
				}
			}
			learned = fmt.Sprintf(`progress kept (%d skills trained)`, n)
		}
		if e.Owner == 0 {
			out = append(out, fmt.Sprintf(`  %s (%s): waiting in the Hollow (room %d) since %s ago; %s`,
				id, p.Name, p.Hollow.RoomId, humanizeElapsed(now.Unix()-e.Since), learned))
			continue
		}
		who := fmt.Sprintf(`account %d`, e.Owner)
		if u := users.GetByUserId(e.Owner); u != nil && u.Character != nil {
			who = u.Character.Name + ` (online)`
		}
		seen := `never`
		if e.LastSeen > 0 {
			seen = humanizeElapsed(now.Unix()-e.LastSeen) + ` ago`
		}
		out = append(out, fmt.Sprintf(`  %s (%s): with %s since %s ago, last seen together %s; %s`,
			id, p.Name, who, humanizeElapsed(now.Unix()-e.Since), seen, learned))
	}
	return out
}

// release is the admin's way to free a companion: sent back now when the
// owner is online, at their next login otherwise.
func (m *AICompanionModule) release(profileId string) string {
	p, ok := m.profiles[strings.ToLower(strings.TrimSpace(profileId))]
	if !ok {
		return fmt.Sprintf(`No profile %q. Try: aicompanion profiles`, profileId)
	}
	owner := m.holderOf(p.Id)
	if owner == 0 {
		return fmt.Sprintf(`%s is already waiting in the Hollow.`, p.Name)
	}
	if u := users.GetByUserId(owner); u != nil && u.Character != nil {
		if comp, held := m.bondedCompanionOf(u); comp != nil && held.Id == p.Id {
			if err := m.sendBack(u, `turns away and takes the road back to the Waystone Hollow.`, true, partReleased); err != nil {
				return err.Error()
			}
			m.tellOwner(u.UserId, fmt.Sprintf(`(%s has gone back to the Waystone Hollow.)`, p.Name))
			return fmt.Sprintf(`Released %s from %s.`, p.Name, u.Character.Name)
		}
	}
	m.unclaim(p, partReleased)
	m.noteForLogin(owner, fmt.Sprintf(`%s has gone back to the Waystone Hollow above Pothole Coulee.`, p.Name))
	return fmt.Sprintf(`Released %s; their owner (account %d) learns of it at their next login.`, p.Name, owner)
}

// tellRoom sends a visual line to a room, when it can be loaded.
func tellRoom(roomId int, text string, exclude ...int) {
	if r := rooms.LoadRoom(roomId); r != nil {
		r.SendTextVisual(messaging.CategoryMobEmote, text, exclude...)
	}
}
