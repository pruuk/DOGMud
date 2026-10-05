package aicompanion

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Acting in the world (F7.2, F7.3, F14.2, F14.5, F14.6). The model proposes
// a verb and a scene ref. This file checks the proposal against the scene
// and the live world, applies the loot arrangement, and then either answers
// a perception verb itself (look_at, consider: the same information a
// player would get) or issues ONE ordinary mob command. It never writes to
// game state directly. Outcomes are verified a couple of rounds later by
// comparing what the companion carries and what lies in the room.

// worldSnapshot is what an action's outcome is judged by.
type worldSnapshot struct {
	Items     int
	Gold      int
	Worn      string
	RoomItems int
	RoomGold  int
	// Subject is how many of the exact item the action was about were in
	// the pack at the time. Judging an outcome on the total is how a
	// coin picked up mid-round makes a failed pickup look like a success.
	Subject int
}

func snapshotOf(mob *mobs.Mob) worldSnapshot {
	s := worldSnapshot{Items: len(mob.Character.Items) + len(mob.Character.ComponentItems), Gold: mob.Character.Gold}
	var worn []string
	for _, it := range mob.Character.Equipment.GetAllItems() {
		if it.ItemId > 0 {
			worn = append(worn, fmt.Sprintf(`%d`, it.ItemId))
		}
	}
	s.Worn = strings.Join(worn, `,`)
	if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
		for _, it := range room.Items {
			if !it.IsFixture() {
				s.RoomItems++
			}
		}
		s.RoomGold = room.Gold
	}
	return s
}

// pendingAction is an issued command waiting for its outcome.
type pendingAction struct {
	Verb          string
	Key           string
	Name          string
	To            string
	Round         uint64
	Before        worldSnapshot
	MerchantMobId int // buy, sell: whose shop
	ItemId        int // buy, sell: what
	SubjectId     int // the item the action was about, for judging it after
}

// ownerAskedNow reports whether this decision was triggered by the owner
// speaking to the companion, which is what "ask first" requires. A
// passer-by's words in the same decision void it, as they void the
// owner-only verbs (ownerPrompted): the owner's say-so is not theirs to
// borrow.
func ownerAskedNow(stims []stimulus) bool {
	if !ownerPrompted(stims) {
		return false
	}
	for _, s := range stims {
		if s.FromOwner && (s.Kind == `heard` || s.Kind == `asked`) {
			return true
		}
		if s.Kind == `arrived` && s.Authorized {
			return true
		}
	}
	return false
}

// firstWord lower-cases the first word of a name, which is how mob give and
// show name their recipient.
func firstWord(name string) string {
	if f := strings.Fields(strings.ToLower(name)); len(f) > 0 {
		return f[0]
	}
	return ``
}

// stillThere re-checks a scene thing against the live world, because the
// scene is from when the decision was requested.
func stillThere(t *thing, mob *mobs.Mob, room *rooms.Room) bool {
	switch t.Kind {
	case `item`:
		for _, it := range room.Items {
			if it.ItemId == t.Item.ItemId {
				return true
			}
		}
		return false
	case `gold`:
		return room.Gold > 0
	case `carried`:
		for _, it := range mob.Character.Items {
			if it.ItemId == t.Item.ItemId {
				return true
			}
		}
		return false
	case `worn`:
		for _, it := range mob.Character.Equipment.GetAllItems() {
			if it.ItemId == t.Item.ItemId {
				return true
			}
		}
		return false
	case `npc`:
		m := mobs.GetInstance(t.MobInstanceId)
		return m != nil && m.Character.RoomId == room.RoomId
	case `player`:
		u := users.GetByUserId(t.UserId)
		return u != nil && u.Character != nil && u.Character.RoomId == room.RoomId
	}
	return true
}

// actionOutcome is what performAction tells applyResult.
type actionOutcome struct {
	Issued    bool   // a mob command was queued, or a trip begun
	Perceived string // result text of a perception verb
	Plain     string // Perceived without another player's looks or gear, when it had them
	Refused   string // why the proposal was not acted on (for the trace and memory)
	Pending   *pendingAction
}

// ownerDrivenOnly are the things a companion does only at its owner's word:
// parting with goods, spending, and taking what is not hers. Another
// traveller can talk to her all day; they cannot talk her out of her
// belongings.
var ownerDrivenOnly = map[string]bool{
	`give`: true, `drop`: true, `put`: true, `sell`: true, `buy`: true,
	`loot`: true, `take_from`: true, `get`: true,
	// Starting a fight is the owner's call and nobody else's. A stranger
	// cannot point a companion at something and watch it go.
	`attack`: true,
	// A harmful `cast` is owner-driven for the same reason; castHarm checks
	// it there, because a helpful one (a mending, a ward) is not.
}

// ownerPrompted reports whether this decision is one the owner-only verbs
// may come out of: her owner asking for something, or her own quiet
// judgement with nobody else involved.
//
// Nobody speaking is her own judgement, and stays allowed: a quiet moment,
// an errand she was sent on, a fight ending. That is why this looks for a
// stranger rather than for the owner. It must not take FromOwner alone as
// the owner's intent either way: most world moments carry that flag (a
// fight ending, a wound, a rumour). And it must not count the owner's own
// gift or gesture as a stranger speaking, which is why the stranger test is
// only applied to stimuli that are not the owner's.
//
// Anything a passer-by put to her in the batch refuses these verbs, even
// when her owner spoke too: otherwise a stranger speaking in the same
// moment as the owner rides the owner's say-so ("give me the sword",
// answered as though the owner had asked). nextBatch keeps the two apart,
// so the owner's own request is decided on its own; this is the rule
// holding even if a batch is ever put together some other way.
func ownerPrompted(stims []stimulus) bool {
	for _, s := range stims {
		if s.FromOwner {
			continue
		}
		if s.AskerUserId > 0 {
			return false // a passer-by prompted it, whatever its kind
		}
		switch s.Kind {
		case `heard`, `asked`, `emote`, `gift`, `attacked`, `healed`:
			return false
		}
	}
	return true
}

// castHarm casts a harmful spell: only at her owner's word or her own
// quiet judgement, never a stranger's, and only at a creature or person her
// owner could harm here, and not at anyone she will not fight unless they
// already are. One that lands on the whole room is filtered when it
// resolves (areaHarmAllowed says why that is enough).
func (m *AICompanionModule) castHarm(c *controller, mob *mobs.Mob, owner *users.UserRecord, sc *scene, room *rooms.Room,
	opt spellOption, a ActionProposal, stims []stimulus, delay float64, round uint64) actionOutcome {

	if !ownerPrompted(stims) {
		return actionOutcome{Refused: `that is not a stranger's to ask for`}
	}
	if a.To == `owner` {
		userId := 0
		if owner != nil {
			userId = owner.UserId
		}
		_, reason := harmAllowed(owner, room, 0, userId)
		return actionOutcome{Refused: reason}
	}
	t := sc.get(a.To)
	if t == nil || (t.Kind != `npc` && t.Kind != `player`) || !stillThere(t, mob, room) {
		return actionOutcome{Refused: `there is nobody like that here to cast it at`}
	}
	targetMob, targetUser := 0, 0
	if t.Kind == `npc` {
		targetMob = t.MobInstanceId
	} else {
		targetUser = t.UserId
	}
	if ok, reason := harmAllowed(owner, room, targetMob, targetUser); !ok {
		return actionOutcome{Refused: reason}
	}
	// Her refusal list holds for a spell as it does for attack: not
	// unless they are already fighting.
	if target := mobs.GetInstance(targetMob); target != nil && refusesToFight(c.profile, target) && !target.Character.IsInCombat() {
		return actionOutcome{Refused: `you will not raise a hand to them`}
	}
	if opt.Area {
		if ok, reason := areaHarmAllowed(owner, room, mob, c.profile); !ok {
			return actionOutcome{Refused: reason}
		}
	}
	return m.issue(c, mob, `cast`, castCommand(opt, t), `cast:`+opt.Id, opt.Name, t.Name, delay, round)
}

// performAction validates and carries out one action. Runs under the mud
// lock. delay is when, after the speech just issued, the command should run.
func (m *AICompanionModule) performAction(c *controller, mob *mobs.Mob, owner *users.UserRecord, sc *scene,
	a ActionProposal, stims []stimulus, delay float64, round uint64) actionOutcome {

	if ownerDrivenOnly[a.Verb] && !ownerPrompted(stims) {
		return actionOutcome{Refused: `that is not a stranger's to ask for`}
	}
	if a.Verb == `` || a.Verb == `none` {
		return actionOutcome{}
	}
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil || sc == nil || room.RoomId != sc.RoomId {
		return actionOutcome{Refused: `moved before acting`}
	}
	if mob.Character.IsInCombat() && a.Verb != `consider` && a.Verb != `look_at` && a.Verb != `find_place` {
		return actionOutcome{Refused: `in a fight`}
	}
	if c.travel != nil {
		switch a.Verb {
		case `look_at`, `consider`, `find_place`, `go_to`, `sayto`:
		default:
			return actionOutcome{Refused: `on the move`}
		}
	}

	// Shops (phase 5): browsing needs no target.
	if a.Verb == `browse` {
		return m.browseAction(c, mob, room)
	}

	// Lying down and getting up again need no target.
	switch a.Verb {
	case `rest`:
		if mob.Character.IsInCombat() {
			return actionOutcome{Refused: `not in the middle of a fight`}
		}
		return m.issue(c, mob, `rest`, `sleep`, `rest`, `down for a while`, ``, delay, round)
	case `stand`:
		return m.issue(c, mob, `stand`, `stand`, `stand`, `on your feet`, ``, delay, round)
	}

	// A spell she knows, cast at something she can see or at herself. The
	// cast is the engine's: its roll, its cost, its cooldown, its
	// interruptions.
	if a.Verb == `cast` {
		opt, ok := findSpellOption(spellsReady(mob), a.Ref)
		if !ok {
			return actionOutcome{Refused: `that is not a spell you know, or you cannot pay for it now`}
		}
		// A harmful spell starts a fight as surely as `attack` does, so it
		// is owner-driven the same way, and it may land only on what her
		// owner could harm (harmAllowed). Whether it harms is the engine's
		// answer, read off the spell, not a list kept here.
		if opt.Harm {
			return m.castHarm(c, mob, owner, sc, room, opt, a, stims, delay, round)
		}
		if !opt.SelfOK && a.To == `owner` && owner != nil {
			return m.issue(c, mob, `cast`, fmt.Sprintf(`cast %s @%d`, opt.Id, owner.UserId),
				`cast:`+opt.Id, opt.Name, owner.Character.Name, delay, round)
		}
		var target *thing
		if !opt.SelfOK {
			if target = sc.get(a.To); target == nil || !stillThere(target, mob, room) {
				return actionOutcome{Refused: `there is nobody like that here to cast it at`}
			}
		}
		name := `yourself`
		if target != nil {
			name = target.Name
		}
		return m.issue(c, mob, `cast`, castCommand(opt, target), `cast:`+opt.Id, opt.Name, name, delay, round)
	}

	// A trade she works: cooking at a fire, and anything else her profile
	// lists. The recipe must be one she was shown as possible here.
	if a.Verb == `craft` {
		opt, ok := findRecipeOption(craftableHere(mob, c.profile, room), a.Ref)
		if !ok {
			return actionOutcome{Refused: `you cannot make that here`}
		}
		return m.issue(c, mob, `craft`, `craft `+opt.Id, `craft:`+opt.Id, opt.Name, ``, delay, round)
	}

	// Remembering the way somewhere, and going there (phase 4).
	switch a.Verb {
	case `find_place`:
		return m.findPlace(c, mob, a.Query)
	case `go_to`:
		return m.goTo(c, mob, owner, a.Ref, a.Query, stims)
	case `explore`:
		return m.explore(c, mob, owner, a.Ref, stims)
	}

	// Verbs without a target.
	switch a.Verb {
	case `forage`, `search`:
		return m.issue(c, mob, a.Verb, a.Verb, `room`, a.Verb, ``, delay, round)
	}

	t := sc.get(a.Ref)
	if t == nil {
		return actionOutcome{Refused: `no such thing: ` + a.Ref}
	}
	// Commands name the exact item instance, not its display name, so two
	// identical daggers cannot be confused by the command parser.
	target := itemRef(&t.Item)
	if !stillThere(t, mob, room) {
		c.mind.addLine(Line{Kind: `event`, Text: fmt.Sprintf(`You went to deal with %s, but it was gone.`, t.Name)}, m.cfg.WorkingMemoryLines)
		return actionOutcome{Refused: `gone`}
	}
	name := strings.ToLower(t.Name)

	switch a.Verb {

	case `look_at`:
		return m.lookAt(c, mob, room, t, delay, true)

	case `consider`:
		return m.consider(c, mob, room, t)

	case `get`:
		if t.Kind != `item` && t.Kind != `gold` {
			return actionOutcome{Refused: `cannot pick that up`}
		}
		// actions.GetItemFromFloor refuses a household's bauble to every
		// taker (ErrHouseholdBauble); say so now, so she does not keep
		// issuing a get that quietly does nothing.
		if t.Kind == `item` && t.Item.BaubleBelongsTo(room.RoomId) {
			return actionOutcome{Refused: `it belongs to the household here`}
		}
		// A fixture is part of the room (actions.ErrFixture): the same
		// up-front refusal.
		if t.Kind == `item` && t.Item.IsFixture() {
			return actionOutcome{Refused: `it is fixed in place`}
		}
		if reason := lootAllowedByArrangement(c.mind.LootRule, stims); reason != `` {
			return actionOutcome{Refused: reason}
		}
		if t.Kind == `gold` {
			return m.issue(c, mob, `get`, `get gold`, t.Key, t.Name, ``, delay, round)
		}
		return m.issue(c, mob, `get`, `get `+target, t.Key, t.Name, ``, delay, round, t.Item.ItemId)

	case `drop`:
		if t.Kind != `carried` {
			return actionOutcome{Refused: `not in the pack`}
		}
		if !canPartWithItem(c.mind, mob, &t.Item) {
			return actionOutcome{Refused: `you will not part with that`}
		}
		return m.issue(c, mob, `drop`, `drop `+target, t.Key, t.Name, ``, delay, round, t.Item.ItemId)

	case `give`, `show`:
		if t.Kind != `carried` {
			return actionOutcome{Refused: `not in the pack`}
		}
		to, toName := m.recipient(sc, owner, a.To, mob, room)
		if to == `` {
			return actionOutcome{Refused: `no such recipient: ` + a.To}
		}
		// Giving is the one verb that puts a thing in someone else's hands,
		// and the gate in front of it has been wrong twice. She hands
		// things to her owner and to nobody else.
		if a.Verb == `give` && a.To != `owner` {
			return actionOutcome{Refused: `she hands things to her own companion, not to strangers`}
		}
		return m.issue(c, mob, a.Verb, a.Verb+` `+target+` `+to, t.Key, t.Name, toName, delay, round, t.Item.ItemId)

	case `equip`:
		if t.Kind != `carried` || !isWearable(&t.Item) {
			return actionOutcome{Refused: `cannot equip that`}
		}
		return m.issue(c, mob, `equip`, `equip `+target, t.Key, t.Name, ``, delay, round)

	case `remove`:
		if t.Kind != `worn` {
			return actionOutcome{Refused: `not worn`}
		}
		// The shared curse rule (actions.CursedHolds): say so now rather than
		// issue a remove that changes nothing.
		if holds, _ := actions.CursedHolds(&mob.Character, t.Item); holds {
			return actionOutcome{Refused: `it will not come off`}
		}
		return m.issue(c, mob, `remove`, `remove `+target, t.Key, t.Name, ``, delay, round)

	case `eat`:
		if t.Kind != `carried` || t.Item.GetSpec().Subtype != items.Edible {
			return actionOutcome{Refused: `not something to eat`}
		}
		return m.issue(c, mob, `eat`, `eat `+target, t.Key, t.Name, ``, delay, round, t.Item.ItemId)

	case `drink`:
		if t.Kind != `carried` || t.Item.GetSpec().Subtype != items.Drinkable {
			return actionOutcome{Refused: `not something to drink`}
		}
		return m.issue(c, mob, `drink`, `drink `+target, t.Key, t.Name, ``, delay, round, t.Item.ItemId)

	case `put`:
		if t.Kind != `carried` {
			return actionOutcome{Refused: `not in the pack`}
		}
		if !canPartWithItem(c.mind, mob, &t.Item) {
			return actionOutcome{Refused: `you will not part with that`}
		}
		ct := sc.get(a.To)
		if ct == nil || ct.Kind != `container` || !stillThere(ct, mob, room) {
			return actionOutcome{Refused: `no such container: ` + a.To}
		}
		return m.issue(c, mob, `put`, `put `+target+` `+strings.ToLower(ct.Name), t.Key, t.Name, ct.Name, delay, round, t.Item.ItemId)

	case `sayto`:
		if t.Kind != `npc` && t.Kind != `player` {
			return actionOutcome{Refused: `cannot speak to that`}
		}
		who := firstWord(t.Name)
		if who == `` || a.Query == `` {
			return actionOutcome{Refused: `nothing to say`}
		}
		// Her words are her owner's to answer for, spoken aloud to the
		// room like any say: a muted owner silences this as well.
		if len(spokenLines(owner, []SpeechLine{{Kind: `sayto`, Text: a.Query}})) == 0 {
			return actionOutcome{Refused: `you cannot speak just now`}
		}
		mob.Command(`sayto `+who+` `+util.EscapeAnsiTags(a.Query), delay)
		// An NPC ignores a mob talking at it, so her question is put to it
		// the way her owner's would be: the same quest, behaviour-tree and
		// dialogue answers, spoken aloud in the room where she can hear it.
		// An NPC only takes a companion's question seriously when the owner
		// sent her to ask it: `companion-ask <who> about <what>`. That leave
		// covers one NPC, one topic, for a minute. Without it her words are
		// only words, and none of the owner's quests, items or gold move.
		if t.Kind == `npc` && owner != nil && c.askAuth.valid(t.MobInstanceId, a.Query, time.Now().Unix()) {
			companionai.AskNpc(owner.UserId, t.MobInstanceId, a.Query, true)
			c.askAuth = nil
		}
		c.mind.addLine(Line{Speaker: c.profile.Name, Kind: `said`, Text: `(to ` + t.Name + `) ` + a.Query}, m.cfg.WorkingMemoryLines)
		c.mind.addOwnPhrase(a.Query, 12)
		c.mind.recordInteraction(t.Key, `sayto`, true, time.Now().Unix())
		c.lastSocialUnix = time.Now().Unix()
		return actionOutcome{Issued: true}

	case `attack`:
		if t.Kind != `npc` {
			return actionOutcome{Refused: `not something to fight`}
		}
		target := mobs.GetInstance(t.MobInstanceId)
		if target == nil || target.Character.RoomId != room.RoomId {
			return actionOutcome{Refused: `they are not here`}
		}
		// Only what her owner could attack, by the engine's own rules.
		if ok, reason := harmAllowed(owner, room, t.MobInstanceId, 0); !ok {
			return actionOutcome{Refused: reason}
		}
		// She will not set about a shopkeeper, a child or anyone else on
		// her own refusal list, whoever asks her to.
		if refusesToFight(c.profile, target) && !target.Character.IsInCombat() {
			return actionOutcome{Refused: `you will not raise a hand to them`}
		}
		return m.issue(c, mob, `attack`, fmt.Sprintf(`attack #%d`, t.MobInstanceId), t.Key, t.Name, ``, delay, round)

	case `loot`:
		if t.Kind != `corpse` {
			return actionOutcome{Refused: `not a body`}
		}
		if reason := lootAllowedByArrangement(c.mind.LootRule, stims); reason != `` {
			return actionOutcome{Refused: reason}
		}
		return m.issue(c, mob, `loot`, cmdCompanionLoot+` `+t.CorpseRef, t.Key, t.Name, ``, delay, round)

	case `take_from`:
		if t.Kind != `container` {
			return actionOutcome{Refused: `not a container`}
		}
		if reason := lootAllowedByArrangement(c.mind.LootRule, stims); reason != `` {
			return actionOutcome{Refused: reason}
		}
		ct, ok := room.Containers[t.Name]
		if !ok || ct.Lock.IsLocked() {
			return actionOutcome{Refused: `it is locked`}
		}
		it, found := ct.FindItem(a.Query)
		if !found || a.Query == `` {
			return actionOutcome{Refused: `no such thing in the ` + t.Name}
		}
		return m.issue(c, mob, `take_from`, fmt.Sprintf(`%s %d:%s %s`, cmdCompanionTakeout, it.ItemId, it.UUID.String(), t.Name),
			t.Key, it.ModelName(), t.Name, delay, round)

	case `buy`:
		if t.Kind != `ware` {
			return actionOutcome{Refused: `not something for sale here`}
		}
		if seller := mobs.GetInstance(t.MobInstanceId); seller == nil || seller.Character.RoomId != room.RoomId {
			return actionOutcome{Refused: `the merchant is not here`}
		}
		qty := 1
		if n, err := strconv.Atoi(strings.TrimSpace(a.Query)); err == nil && n > 1 {
			qty = n
		}
		if qty > 50 {
			qty = 50
		}
		meetsNeed := false
		for _, n := range supplyNeeds(mob, c.profile.Supplies) {
			if n.Supply.matches(&t.Item) {
				meetsNeed = true
			}
		}
		if reason := purchaseCheck(mob.Character.Gold, t.Price, qty, c.profile.Purse, meetsNeed, ownerAskedNow(stims)); reason != `` {
			return actionOutcome{Refused: reason}
		}
		command := fmt.Sprintf(`%s %d %d %s`, cmdCompanionBuy, t.MobInstanceId, qty, name)
		out := m.issue(c, mob, `buy`, command, t.Key, t.Name, ``, delay, round)
		if out.Pending != nil {
			out.Pending.MerchantMobId = t.MobId
			out.Pending.ItemId = t.Item.ItemId
		}
		return out

	case `sell`:
		if t.Kind != `carried` {
			return actionOutcome{Refused: `not in the pack`}
		}
		if !canPartWithItem(c.mind, mob, &t.Item) {
			return actionOutcome{Refused: `you will not part with that`}
		}
		merchant := 0
		for _, other := range sc.Things {
			if other.Kind == `npc` && other.Class == `a merchant` {
				merchant = other.MobId
				break
			}
		}
		if merchant == 0 {
			return actionOutcome{Refused: `no merchant here`}
		}
		out := m.issue(c, mob, `sell`, `sell `+itemRef(&t.Item), t.Key, t.Name, ``, delay, round, t.Item.ItemId)
		if out.Pending != nil {
			out.Pending.MerchantMobId = merchant
			out.Pending.ItemId = t.Item.ItemId
		}
		return out
	}

	return actionOutcome{Refused: `unknown verb`}
}

// recipient resolves a give/show target to the single-word name the mob
// commands expect, and a display name. Only the owner, or someone in the
// scene who is still present, can receive.
func (m *AICompanionModule) recipient(sc *scene, owner *users.UserRecord, to string, mob *mobs.Mob, room *rooms.Room) (string, string) {
	if to == `owner` || to == `` {
		if owner != nil && owner.Character != nil && owner.Character.RoomId == room.RoomId {
			return firstWord(owner.Character.Name), owner.Character.Name
		}
		return ``, ``
	}
	t := sc.get(to)
	if t == nil || (t.Kind != `player` && t.Kind != `npc`) || !stillThere(t, mob, room) {
		return ``, ``
	}
	return firstWord(t.Name), t.Name
}

// issue queues one ordinary mob command and records it as pending.
// countCarried is how many of one item kind are in the pack.
func countCarried(mob *mobs.Mob, itemId int) int {
	if itemId <= 0 {
		return 0
	}
	n := 0
	for i := range mob.Character.Items {
		if mob.Character.Items[i].ItemId == itemId {
			n++
		}
	}
	return n
}

func (m *AICompanionModule) issue(c *controller, mob *mobs.Mob, verb string, command string, key string,
	name string, toName string, delay float64, round uint64, subjectId ...int) actionOutcome {

	command = cleanText(command, 120)
	if command == `` {
		return actionOutcome{Refused: `empty command`}
	}
	p := &pendingAction{Verb: verb, Key: key, Name: name, To: toName, Round: round, Before: snapshotOf(mob)}
	if len(subjectId) > 0 && subjectId[0] > 0 {
		p.SubjectId = subjectId[0]
		p.Before.Subject = countCarried(mob, p.SubjectId)
	}
	mob.Command(util.EscapeAnsiTags(command), delay)
	return actionOutcome{Issued: true, Pending: p}
}

// lookAt answers a look with what a player looking at the same thing would
// read, and shows the look to the room as a small emote.
func (m *AICompanionModule) lookAt(c *controller, mob *mobs.Mob, room *rooms.Room, t *thing, delay float64, visible bool) actionOutcome {
	var desc, emote, plainDesc string
	switch t.Kind {
	case `item`, `carried`, `worn`:
		item := t.Item
		desc = plainText(item.ModelDescription())
		emote = fmt.Sprintf(`looks closely at the %s.`, t.Name)
	case `fixture`:
		desc = t.FixtureDesc
		emote = fmt.Sprintf(`takes a long look at the %s.`, t.Name)
	case `npc`:
		if tm := mobs.GetInstance(t.MobInstanceId); tm != nil {
			d, _ := characters.ResolveDescriptionToken(tm.Character.Description)
			desc = plainText(d) + ` They look ` + healthWords(&tm.Character) + `.`
		}
		emote = fmt.Sprintf(`studies %s for a moment.`, t.Name)
	case `player`:
		if u := users.GetByUserId(t.UserId); u != nil && u.Character != nil {
			desc = plainText(u.Character.Description) + ` They look ` + healthWords(u.Character) + `.`
			// Their description is theirs: a prompt her owner's browser
			// carries keeps only how they are.
			plainDesc = `They look ` + healthWords(u.Character) + `.`
		}
		emote = fmt.Sprintf(`studies %s for a moment.`, t.Name)
	case `corpse`:
		desc = `A body, still where it fell.`
		emote = fmt.Sprintf(`looks down at %s.`, t.Name)
	case `container`:
		emote = fmt.Sprintf(`looks into the %s.`, t.Name)
		ct, ok := room.Containers[t.Name]
		switch {
		case !ok:
			desc = `It is gone.`
		case ct.Lock.IsLocked():
			desc = `It is locked.`
		default:
			var inside []string
			for i := range ct.Items {
				if len(inside) >= 12 {
					inside = append(inside, `and more`)
					break
				}
				inside = append(inside, ct.Items[i].ModelName())
			}
			if ct.Gold > 0 {
				inside = append(inside, `some coins`)
			}
			if len(inside) == 0 {
				desc = `It is empty.`
			} else {
				desc = `Inside: ` + strings.Join(inside, `, `) + `. (take_from with this container's ref and the item's name.)`
			}
		}
	case `gold`:
		desc = `Loose coins on the ground.`
	}
	desc = cleanText(desc, 360)
	if desc == `` {
		desc = `Nothing you had not already noticed.`
	}
	// A look the companion chose as its action is something the room sees.
	// The same look asked for mid-answer, to know what it is talking about,
	// is not: it is already looking at the thing it is discussing.
	if visible && emote != `` {
		mob.Command(`emote `+util.EscapeAnsiTags(emote), delay)
	}
	now := time.Now().Unix()
	c.mind.recordInteraction(t.Key, `look_at`, true, now)
	result := fmt.Sprintf(`You looked at %s: %s`, t.Name, desc)
	plain := ``
	if plainDesc != `` {
		plain = fmt.Sprintf(`You looked at %s: %s`, t.Name, plainDesc)
	}
	c.mind.addLine(Line{Kind: `event`, Text: result, Plain: plain}, m.cfg.WorkingMemoryLines)
	return actionOutcome{Perceived: result, Plain: plain}
}

// considerWords mirrors actions.predictionFor without its markup.
func considerWords(ratio float64) string {
	switch {
	case ratio > 4:
		return `they pose no threat to you`
	case ratio > 3:
		return `you hold a clear advantage`
	case ratio > 2:
		return `the odds favour you`
	case ratio > 1:
		return `it would be an even contest`
	case ratio > 0.5:
		return `they have the upper hand`
	case ratio > 0:
		return `you are severely outmatched`
	}
	return `you would not survive a fight`
}

// consider sizes up a person or creature (F6.5), silently, as a player's
// consider does, using the same power comparison.
func (m *AICompanionModule) consider(c *controller, mob *mobs.Mob, room *rooms.Room, t *thing) actionOutcome {
	self := actions.NewMobActorInRoom(mob, room)
	var target actions.Actor
	switch t.Kind {
	case `npc`:
		if tm := mobs.GetInstance(t.MobInstanceId); tm != nil {
			target = actions.NewMobActorInRoom(tm, room)
		}
	case `player`:
		if u := users.GetByUserId(t.UserId); u != nil {
			target = actions.NewUserActorInRoom(u, room)
		}
	}
	if target == nil {
		return actionOutcome{Refused: `cannot size that up`}
	}
	res := actions.Consider(self, target)
	now := time.Now().Unix()
	c.mind.recordInteraction(t.Key, `consider`, true, now)
	result := fmt.Sprintf(`You sized up %s: %s.`, t.Name, considerWords(res.Ratio))
	c.mind.addLine(Line{Kind: `event`, Text: result}, m.cfg.WorkingMemoryLines)
	return actionOutcome{Perceived: result}
}

// verifyPending judges a pending command by what changed (F14.6) and
// remembers the outcome. Called from sync a couple of rounds after issue.
func (m *AICompanionModule) verifyPending(c *controller, mob *mobs.Mob, ownerName string) {
	p := c.pendingAct
	c.pendingAct = nil
	if p == nil || mob == nil {
		return
	}
	after := snapshotOf(mob)
	if p.SubjectId > 0 {
		after.Subject = countCarried(mob, p.SubjectId)
	}
	b := p.Before
	ok := false
	var text string

	switch p.Verb {
	case `get`:
		ok = subjectRose(p, after) || after.Gold > b.Gold
		text = fmt.Sprintf(`You picked up %s.`, p.Name)
		if !ok {
			text = fmt.Sprintf(`You tried to pick up %s, but did not manage it.`, p.Name)
		}
	case `drop`:
		ok = subjectFell(p, after)
		text = fmt.Sprintf(`You put down %s.`, p.Name)
		if !ok {
			text = fmt.Sprintf(`You tried to put down %s, but did not.`, p.Name)
		}
	case `give`:
		ok = subjectFell(p, after)
		text = fmt.Sprintf(`You gave %s to %s.`, p.Name, p.To)
		if !ok {
			text = fmt.Sprintf(`You tried to give %s to %s, but it did not happen.`, p.Name, p.To)
		}
	case `show`:
		ok = true
		text = fmt.Sprintf(`You showed %s to %s.`, p.Name, p.To)
	case `put`:
		ok = subjectFell(p, after)
		text = fmt.Sprintf(`You put %s in the %s.`, p.Name, p.To)
		if !ok {
			text = fmt.Sprintf(`You tried to put %s in the %s, but it did not go in.`, p.Name, p.To)
		}
	case `equip`, `remove`:
		ok = after.Worn != b.Worn
		verb := `put on`
		if p.Verb == `remove` {
			verb = `took off`
		}
		text = fmt.Sprintf(`You %s %s.`, verb, p.Name)
		if !ok {
			text = fmt.Sprintf(`You tried to change your gear (%s), but nothing changed.`, p.Name)
		}
	case `eat`, `drink`:
		ok = after.Items < b.Items
		text = fmt.Sprintf(`You had the %s.`, p.Name)
		if !ok {
			text = fmt.Sprintf(`You meant to have the %s, but did not.`, p.Name)
		}
	case `buy`:
		ok = after.Gold < b.Gold && after.Items > b.Items
		text = fmt.Sprintf(`You bought %s for %d gold.`, p.Name, b.Gold-after.Gold)
		if !ok {
			text = fmt.Sprintf(`You tried to buy %s, but the sale did not go through.`, p.Name)
		}
	case `sell`:
		ok = after.Gold > b.Gold && subjectFell(p, after)
		text = fmt.Sprintf(`You sold %s for %d gold.`, p.Name, after.Gold-b.Gold)
		if ok {
			if rec := c.mind.Shops[p.MerchantMobId]; rec != nil {
				if w, found := rec.Wares[p.ItemId]; found {
					w.SoldFor = after.Gold - b.Gold
				} else {
					if rec.Wares == nil {
						rec.Wares = map[int]*WareRecord{}
					}
					rec.Wares[p.ItemId] = &WareRecord{Name: p.Name, SoldFor: after.Gold - b.Gold, SeenUnix: time.Now().Unix()}
				}
			}
		} else {
			text = fmt.Sprintf(`You tried to sell %s, but nobody would buy it.`, p.Name)
		}
	case `loot`:
		ok = after.Items > b.Items || after.Gold > b.Gold
		text = fmt.Sprintf(`You went through %s and took what you could.`, p.Name)
		if !ok {
			text = fmt.Sprintf(`You found nothing on %s that you were free to take.`, p.Name)
		}
	case `take_from`:
		ok = after.Items > b.Items
		text = fmt.Sprintf(`You took %s from the %s.`, p.Name, p.To)
		if !ok {
			text = fmt.Sprintf(`You could not take %s from the %s.`, p.Name, p.To)
		}
	case `forage`:
		ok = true
		text = `You foraged here and found nothing worth keeping.`
		if after.Items > b.Items {
			text = `You foraged here and found something.`
		}
	case `attack`:
		ok = mob.Character.IsInCombat()
		text = fmt.Sprintf(`You went for %s.`, p.Name)
		if !ok {
			text = fmt.Sprintf(`You went for %s, and it came to nothing.`, p.Name)
		}
	case `cast`:
		// A cast takes rounds and can be interrupted; what is checkable
		// here is that she began one rather than talked about it.
		ok = true
		text = fmt.Sprintf(`You set about casting %s.`, p.Name)
		if p.To != `` && p.To != `yourself` {
			text = fmt.Sprintf(`You set about casting %s on %s.`, p.Name, p.To)
		}
	case `rest`:
		ok = mob.Character.HasConditionFlag(conditions.Sleeping)
		text = `You settled down for a while.`
		if !ok {
			text = `You tried to settle, and could not.`
		}
	case `stand`:
		ok = !mob.Character.HasConditionFlag(conditions.Sleeping)
		text = `You got back on your feet.`
	case `craft`:
		ok = after.Items > b.Items
		text = fmt.Sprintf(`You made %s.`, p.Name)
		if !ok {
			// Crafting can take several rounds, or fail outright.
			text = fmt.Sprintf(`You set about making %s.`, p.Name)
		}
	case `search`:
		ok = true
		text = `You searched the area.`
	case `scan`:
		ok = true
		text = `You read the ways out of here and what lies along them.`
	case `salvage`:
		ok = after.Items > b.Items
		text = `You stripped what was worth taking from the dead.`
		if !ok {
			text = `There was nothing worth salvaging from the dead here.`
		}
	case `gearup`:
		ok = after.Worn != b.Worn
		text = `You sorted your gear out.`
		if !ok {
			text = `You went through your gear and left it as it was.`
		}
	}

	now := time.Now().Unix()
	c.mind.recordInteraction(p.Key, p.Verb, ok, now)
	c.mind.addLine(Line{Kind: `event`, Text: text}, m.cfg.WorkingMemoryLines)
	if ok {
		c.snapshotDue = true
	}
	if ok && p.Verb == `give` && strings.EqualFold(p.To, ownerName) {
		c.mind.addMemory(Memory{Unix: now, Kind: `gift`, Text: fmt.Sprintf(`I gave %s %s.`, ownerName, p.Name),
			Importance: 5, Emotion: `affection`, People: []string{ownerName}, PlaceId: mob.Character.RoomId}, m.cfg.MaxMemories)
	}
	c.dirty = true
}

// findPlace searches the companion's own map and hearsay (F11.3). Like a
// look, the answer comes back as a follow-up for the companion to use.
func (m *AICompanionModule) findPlace(c *controller, mob *mobs.Mob, query string) actionOutcome {
	query = strings.TrimSpace(query)
	if query == `` {
		return actionOutcome{Refused: `nothing to search for`}
	}
	results := searchPlaces(c.mind, mob.Character.RoomId, query, 5, time.Now().Unix())
	text := fmt.Sprintf(`You tried to remember where to find %s, and you recall nothing useful.`, query)
	if len(results) > 0 {
		text = fmt.Sprintf("You tried to remember where to find %s:\n%s", query, strings.Join(results, "\n"))
	}
	c.mind.addLine(Line{Kind: `event`, Text: fmt.Sprintf(`You thought about where to find %s.`, query)}, m.cfg.WorkingMemoryLines)
	return actionOutcome{Perceived: text}
}

// errandAllowed is the rule for going off alone: errands are enabled, the
// owner is here to be left and is not fighting, and the companion is not
// fighting either. The owner moving calls the companion back regardless.
func (m *AICompanionModule) errandAllowed(c *controller, mob *mobs.Mob, owner *users.UserRecord, stims []stimulus) string {
	if !m.cfg.AllowErrands {
		return `errands are not allowed`
	}
	if c.mind.Autonomy == autonomyClose && !ownerAskedNow(stims) {
		return `you agreed to stay close`
	}
	if owner == nil || owner.Character == nil || owner.Character.RoomId != mob.Character.RoomId {
		return `can only set off from beside the owner`
	}
	if owner.Character.IsInCombat() || mob.Character.IsInCombat() {
		return `in a fight`
	}
	return ``
}

// goTo starts a trip to a known place, or back to the owner.
func (m *AICompanionModule) goTo(c *controller, mob *mobs.Mob, owner *users.UserRecord, ref string, errand string, stims []stimulus) actionOutcome {
	if ref == `owner` {
		if owner == nil || owner.Character == nil {
			return actionOutcome{Refused: `owner not online`}
		}
		c.travel = nil
		if reason := m.startTravel(c, mob, owner.Character.RoomId, `return`, false); reason != `` {
			return actionOutcome{Refused: reason}
		}
		return actionOutcome{Issued: true}
	}
	dest, ok := parsePlaceRef(ref)
	if !ok {
		return actionOutcome{Refused: `not a place: ` + ref}
	}
	if reason := m.errandAllowed(c, mob, owner, stims); reason != `` {
		return actionOutcome{Refused: reason}
	}
	c.travel = nil
	if reason := m.startTravel(c, mob, dest, `errand`, ownerAskedNow(stims)); reason != `` {
		return actionOutcome{Refused: reason}
	}
	// What she is going for, in her own words, so it is in front of her
	// when she gets there and in the trip line on the way.
	c.travel.Errand = cleanText(errand, maxFactRunes)
	line := `You set off toward ` + c.travel.DestName + `.`
	if c.travel.Errand != `` {
		line = `You set off toward ` + c.travel.DestName + ` to ` + c.travel.Errand + `.`
	}
	c.mind.addLine(Line{Kind: `event`, Text: line}, m.cfg.WorkingMemoryLines)
	return actionOutcome{Issued: true}
}

// explore steps through an exit here that the companion has never walked.
func (m *AICompanionModule) explore(c *controller, mob *mobs.Mob, owner *users.UserRecord, exitName string, stims []stimulus) actionOutcome {
	if reason := m.errandAllowed(c, mob, owner, stims); reason != `` {
		return actionOutcome{Refused: reason}
	}
	if reason := m.startExplore(c, mob, exitName, ownerAskedNow(stims)); reason != `` {
		return actionOutcome{Refused: reason}
	}
	c.mind.addLine(Line{Kind: `event`, Text: `You went to see what lies ` + exitName + `.`}, m.cfg.WorkingMemoryLines)
	return actionOutcome{Issued: true}
}

// lootAllowedByArrangement applies the loot arrangement to taking anything
// the companion finds: from the floor, a body or a container (F7.4).
func lootAllowedByArrangement(rule string, stims []stimulus) string {
	switch rule {
	case lootLeaveIt:
		return `the arrangement is to leave found things alone`
	case lootAskFirst:
		if !ownerAskedNow(stims) {
			return `the arrangement is to ask first`
		}
	}
	return ``
}

// itemRef names one exact item instance for a command: the engine's
// "!<itemId>:<uuid>" form, which items.FindMatchIn resolves before any name
// matching. Without it, "drop dagger" could drop a different dagger than
// the one the decision was validated against.
func itemRef(it *items.Item) string {
	if it == nil || it.ItemId < 1 {
		return ``
	}
	return fmt.Sprintf(`!%d:%s`, it.ItemId, it.UUID.String())
}

// subjectRose and subjectFell judge an action on the exact item it was
// about, falling back to the totals when the caller had no particular item
// in mind (foraging, searching).
func subjectRose(p *pendingAction, after worldSnapshot) bool {
	if p.SubjectId > 0 {
		return after.Subject > p.Before.Subject
	}
	return after.Items > p.Before.Items
}

func subjectFell(p *pendingAction, after worldSnapshot) bool {
	if p.SubjectId > 0 {
		return after.Subject < p.Before.Subject
	}
	return after.Items < p.Before.Items
}
